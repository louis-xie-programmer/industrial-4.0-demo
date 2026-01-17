package engine

import (
	"context"
	"fmt"
	"github.com/antonmedv/expr"
	"industrial-4.0-demo/internal/event"
	"industrial-4.0-demo/internal/fsm"
	"industrial-4.0-demo/internal/station"
	"industrial-4.0-demo/internal/types"
	"industrial-4.0-demo/internal/util"
	"log/slog"
	"sync"
	"time"
)

// WorkflowEngine 负责编排和执行生产流程
// 它管理工站、工作流定义、资源池，并协调工件在各个工站间的流转
type WorkflowEngine struct {
	stations      map[types.StationID]station.Station // 已注册的工站映射
	workflows     map[string][]types.WorkflowStep     // 工作流定义，Key 为产品类型
	resourcePools map[types.StationID]chan struct{}   // 资源池，用于限制特定工站的并发数
	logger        *slog.Logger                        // 结构化日志记录器
	eventBus      *event.Bus                          // 事件总线，用于发布业务事件
}

// NewWorkflowEngine 创建一个新的 WorkflowEngine 实例
func NewWorkflowEngine(
	workflows map[string][]types.WorkflowStep,
	pools map[types.StationID]int,
	logger *slog.Logger,
	bus *event.Bus,
) *WorkflowEngine {
	engine := &WorkflowEngine{
		stations:      make(map[types.StationID]station.Station),
		workflows:     workflows,
		resourcePools: make(map[types.StationID]chan struct{}),
		logger:        logger,
		eventBus:      bus,
	}
	// 初始化资源池
	for id, size := range pools {
		engine.resourcePools[id] = make(chan struct{}, size)
	}
	return engine
}

// RegisterStation 注册一个工站到引擎中
func (e *WorkflowEngine) RegisterStation(s station.Station) {
	e.stations[s.GetID()] = s
}

// Process 执行工件的生产流程
// 这是核心业务逻辑，包含 Saga 事务管理、规则引擎评估和并行工序执行
func (e *WorkflowEngine) Process(ctx context.Context, p *types.Product) {
	// 创建带有上下文信息的 Logger
	logger := e.logger.With("product_id", p.ID, "product_type", p.Type)
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		logger = logger.With("trace_id", traceID)
	}

	// 初始化工件的 FSM 状态机
	productFSM := fsm.NewFSM(p.ID)
	p.FSM = productFSM

	// 发布工件开始生产事件
	e.eventBus.Publish(event.Event{Type: event.ProductStarted, ProductID: p.ID, Product: p})
	logger.Info("开始生产工件", "attributes", p.Attrs)

	// 获取对应产品类型的工作流，默认为双面板流程
	sequence, ok := e.workflows[p.Type]
	if !ok {
		sequence = e.workflows["PCB_DOUBLE_LAYER"]
	}

	executedStations := []station.Station{}
	for i, step := range sequence {
		p.Step = i
		// 规则引擎评估：判断是否需要跳过当前步骤
		if shouldSkip, err := e.evaluateRule(step.Rule, p); err != nil {
			logger.Error("规则引擎评估失败", "error", err, "rule", step.Rule)
			continue
		} else if shouldSkip {
			logger.Info("跳过步骤", "rule", step.Rule)
			continue
		}

		// 执行当前步骤（可能包含并行工站）
		stepResults, stepStations := e.executeStep(ctx, step, p, logger)

		// 检查步骤执行结果，如果有失败则触发 Saga 回滚
		if failed, err := e.checkStepFailure(stepResults); failed {
			e.eventBus.Publish(event.Event{Type: event.ProductFailed, ProductID: p.ID, Product: p, Error: err})
			e.rollback(ctx, executedStations, p, logger)
			return
		}
		executedStations = append(executedStations, stepStations...)
	}

	// 流程成功完成
	e.eventBus.Publish(event.Event{Type: event.ProductCompleted, ProductID: p.ID, Product: p})
	logger.Info("工件顺利下线")
}

// evaluateRule 使用 expr 引擎评估规则表达式
func (e *WorkflowEngine) evaluateRule(rule string, p *types.Product) (bool, error) {
	if rule == "" {
		return false, nil // 没有规则则默认执行
	}
	env := map[string]interface{}{"product": p}
	program, err := expr.Compile(rule, expr.Env(env))
	if err != nil {
		return true, fmt.Errorf("rule compilation failed: %w", err)
	}
	result, err := expr.Run(program, env)
	if err != nil {
		return true, fmt.Errorf("rule execution failed: %w", err)
	}
	shouldExecute, ok := result.(bool)
	if !ok {
		return true, fmt.Errorf("rule result is not a boolean")
	}
	return !shouldExecute, nil // 返回是否跳过 (shouldSkip)
}

// executeStep 执行单个工作流步骤，支持并行执行多个工站
func (e *WorkflowEngine) executeStep(ctx context.Context, step types.WorkflowStep, p *types.Product, logger *slog.Logger) ([]types.Result, []station.Station) {
	var wg sync.WaitGroup
	results := make([]types.Result, len(step.StationIDs))
	stations := make([]station.Station, len(step.StationIDs))

	for i, sID := range step.StationIDs {
		st, exists := e.stations[sID]
		if !exists {
			results[i] = types.Result{ProductID: p.ID, Success: false, Error: fmt.Errorf("station %s not found", sID)}
			continue
		}
		stations[i] = st
		wg.Add(1)
		go func(index int, s station.Station) {
			defer wg.Done()
			stationLogger := logger.With("station_id", s.GetID())

			// 资源申请逻辑
			pool, hasPool := e.resourcePools[s.GetID()]
			if hasPool {
				stationLogger.Info("等待资源")
				pool <- struct{}{} // 获取资源凭证
				stationLogger.Info("获得资源")
				defer func() {
					<-pool // 释放资源凭证
					stationLogger.Info("释放资源")
				}()
			}

			e.eventBus.Publish(event.Event{Type: event.StepStarted, ProductID: p.ID, StationID: s.GetID()})
			start := time.Now()
			results[index] = s.Execute(ctx, p)
			duration := time.Since(start).Seconds()
			e.eventBus.Publish(event.Event{Type: event.StepCompleted, ProductID: p.ID, StationID: s.GetID(), Product: &types.Product{Attrs: map[string]interface{}{"duration": duration}}})

		}(i, st)
	}
	wg.Wait()
	return results, stations
}

// checkStepFailure 检查步骤执行结果中是否有失败
func (e *WorkflowEngine) checkStepFailure(results []types.Result) (bool, error) {
	for _, res := range results {
		if !res.Success {
			return true, res.Error
		}
	}
	return false, nil
}

// rollback 执行 Saga 补偿流程，逆序执行已完成工站的补偿操作
func (e *WorkflowEngine) rollback(ctx context.Context, stations []station.Station, p *types.Product, logger *slog.Logger) {
	logger.Warn("启动 SAGA 补偿流程")
	for i := len(stations) - 1; i >= 0; i-- {
		stations[i].Compensate(ctx, p)
	}
	e.eventBus.Publish(event.Event{Type: event.ProductCompensated, ProductID: p.ID, Product: p})
	logger.Info("工件补偿完成")
}
