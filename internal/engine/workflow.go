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
	"strings"
	"sync"
	"time"
)

// WorkflowEngine 负责编排和执行生产流程
type WorkflowEngine struct {
	stations      map[types.StationID]station.Station
	workflows     map[string][]types.WorkflowStep
	resourcePools map[types.StationID]chan struct{}
	logger        *slog.Logger
	eventBus      *event.Bus
	stepDelay     time.Duration
}

// NewWorkflowEngine 创建一个新的 WorkflowEngine 实例
func NewWorkflowEngine(
	workflows map[string][]types.WorkflowStep,
	pools map[types.StationID]int,
	logger *slog.Logger,
	bus *event.Bus,
	stepDelayMs int,
) *WorkflowEngine {
	engine := &WorkflowEngine{
		stations:      make(map[types.StationID]station.Station),
		workflows:     workflows,
		resourcePools: make(map[types.StationID]chan struct{}),
		logger:        logger,
		eventBus:      bus,
		stepDelay:     time.Duration(stepDelayMs) * time.Millisecond,
	}
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
func (e *WorkflowEngine) Process(ctx context.Context, p *types.Product) {
	logger := e.logger.With("product_id", p.ID, "product_type", p.Type)
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		logger = logger.With("trace_id", traceID)
	}

	productFSM := fsm.NewFSM(p.ID)
	p.FSM = productFSM

	e.eventBus.Publish(event.Event{Type: event.ProductStarted, ProductID: p.ID, Product: p})
	logger.Info("开始生产工件", "attributes", p.Attrs)

	// *** BUG FIX: Viper 将 key 转换为小写，所以查找时需要转换为小写 ***
	sequence, ok := e.workflows[strings.ToLower(p.Type)]
	if !ok {
		logger.Warn("未找到指定的工作流，将使用默认流程", "requested_type", p.Type)
		sequence = e.workflows["pcb_double_layer"] // 使用小写 key
	}

	executedStations := []station.Station{}
	for i, step := range sequence {
		p.Step = i
		if shouldSkip, err := e.evaluateRule(step.Rule, p); err != nil {
			logger.Error("规则引擎评估失败", "error", err, "rule", step.Rule)
			continue
		} else if shouldSkip {
			logger.Info("跳过步骤", "rule", step.Rule)
			continue
		}

		if i > 0 {
			time.Sleep(e.stepDelay)
		}

		stepResults, stepStations := e.executeStep(ctx, step, p, logger)

		if failed, err := e.checkStepFailure(stepResults); failed {
			e.eventBus.Publish(event.Event{Type: event.ProductFailed, ProductID: p.ID, Product: p, Error: err})
			e.rollback(ctx, executedStations, p, logger)
			return
		}
		executedStations = append(executedStations, stepStations...)
	}

	e.eventBus.Publish(event.Event{Type: event.ProductCompleted, ProductID: p.ID, Product: p})
	logger.Info("工件顺利下线")
}

func (e *WorkflowEngine) evaluateRule(rule string, p *types.Product) (bool, error) {
	if rule == "" {
		return false, nil
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
	return !shouldExecute, nil
}

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
			pool, hasPool := e.resourcePools[s.GetID()]
			if hasPool {
				stationLogger.Info("等待资源")
				pool <- struct{}{}
				stationLogger.Info("获得资源")
				defer func() {
					<-pool
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

func (e *WorkflowEngine) checkStepFailure(results []types.Result) (bool, error) {
	for _, res := range results {
		if !res.Success {
			return true, res.Error
		}
	}
	return false, nil
}

func (e *WorkflowEngine) rollback(ctx context.Context, stations []station.Station, p *types.Product, logger *slog.Logger) {
	logger.Warn("启动 SAGA 补偿流程")
	for i := len(stations) - 1; i >= 0; i-- {
		stations[i].Compensate(ctx, p)
	}
	e.eventBus.Publish(event.Event{Type: event.ProductCompensated, ProductID: p.ID, Product: p})
	logger.Info("工件补偿完成")
}
