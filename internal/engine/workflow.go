package engine

import (
	"fmt"
	"industrial-4.0-demo/internal/fsm"
	"industrial-4.0-demo/internal/metrics"
	"industrial-4.0-demo/internal/station"
	"industrial-4.0-demo/internal/types"
	"sync"
	"time"
)

type WorkflowEngine struct {
	stations  map[types.StationID]station.Station
	workflows map[string][]types.WorkflowStep
}

func NewWorkflowEngine(workflows map[string][]types.WorkflowStep) *WorkflowEngine {
	return &WorkflowEngine{
		stations:  make(map[types.StationID]station.Station),
		workflows: workflows,
	}
}

func (e *WorkflowEngine) RegisterStation(s station.Station) {
	e.stations[s.GetID()] = s
}

// Process 执行完整的编排流程（Saga 模式）
func (e *WorkflowEngine) Process(p *types.Product) {
	// 初始化 FSM
	productFSM := fsm.NewFSM(p.ID)
	p.FSM = productFSM

	// 注册状态回调（可选，用于监控或副作用）
	productFSM.RegisterCallback(fsm.StateFailed, func(id string) {
		fmt.Printf("!!! 警报：工件 %s 进入失败状态，准备回滚 !!!\n", id)
	})

	executedStations := []station.Station{}

	// 获取产品对应的工艺流程，默认为 STANDARD
	sequence, ok := e.workflows[p.Type]
	if !ok {
		sequence = e.workflows["STANDARD"]
	}

	fmt.Printf("\n>>> 开始生产工件: %s (类型: %s)\n", p.ID, p.Type)
	_ = productFSM.Fire(fsm.EventStart) // CREATED -> PROCESSING

	for i, step := range sequence {
		p.Step = i // 更新当前步骤索引

		// 执行当前步骤（可能是并行的）
		stepResults, stepStations := e.executeStep(step, p, productFSM)

		// 检查是否有失败
		failed := false
		for _, res := range stepResults {
			if !res.Success {
				fmt.Printf("ERR: 工件 %s 在 %s 失败: %v\n", p.ID, res.ProductID, res.Error)
				failed = true
				break
			}
		}

		if failed {
			_ = productFSM.Fire(fsm.EventFail) // -> FAILED
			// 指标：记录失败任务
			metrics.TasksProcessedTotal.WithLabelValues("failed", p.Type).Inc()
			// 触发回滚补偿流程
			// 注意：这里需要回滚之前所有成功的步骤，以及当前步骤中可能已经成功的部分（如果需要更精细的控制）
			// 简化起见，我们只回滚之前完全成功的步骤
			e.rollback(executedStations, p, productFSM)
			return
		}

		// 将当前步骤的所有工站加入已执行列表
		executedStations = append(executedStations, stepStations...)
	}

	_ = productFSM.Fire(fsm.EventFinish)  // -> COMPLETED
	p.Status = string(fsm.StateCompleted) // 同步最终状态到 Status 字段以便查看

	// 指标：记录成功任务
	metrics.TasksProcessedTotal.WithLabelValues("success", p.Type).Inc()

	fmt.Printf("SUCCESS: 工件 %s 已顺利下线！历史: %v\n", p.ID, p.History)
}

// executeStep 执行单个步骤，支持并行工站
func (e *WorkflowEngine) executeStep(step types.WorkflowStep, p *types.Product, productFSM *fsm.FSM) ([]types.Result, []station.Station) {
	var wg sync.WaitGroup
	results := make([]types.Result, len(step.StationIDs))
	stations := make([]station.Station, len(step.StationIDs))

	// 如果是 QC 站，触发 FSM 事件
	// 注意：并行步骤中如果有 QC，逻辑可能需要调整，这里假设 QC 是单独的步骤
	if len(step.StationIDs) == 1 && step.StationIDs[0] == types.StationQC {
		_ = productFSM.Fire(fsm.EventEnterQC)
	}

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
			start := time.Now()
			results[index] = s.Execute(p)
			duration := time.Since(start).Seconds()
			metrics.StationProcessingDuration.WithLabelValues(string(s.GetID())).Observe(duration)
		}(i, st)
	}

	wg.Wait()

	// QC 成功后切回 Processing
	if len(step.StationIDs) == 1 && step.StationIDs[0] == types.StationQC {
		// 检查 QC 结果
		if results[0].Success {
			_ = productFSM.Fire(fsm.EventPassQC)
		}
	}

	return results, stations
}

func (e *WorkflowEngine) rollback(stations []station.Station, p *types.Product, productFSM *fsm.FSM) {
	_ = productFSM.Fire(fsm.EventCompensate) // -> COMPENSATING
	fmt.Printf("--- 启动 SAGA 补偿流程 ---\n")

	for i := len(stations) - 1; i >= 0; i-- {
		stations[i].Compensate(p)
	}

	_ = productFSM.Fire(fsm.EventRollback) // -> COMPENSATED
	p.Status = string(fsm.StateCompensated)
	fmt.Printf("--- 工件 %s 补偿完成，已安全剔除 ---\n", p.ID)
}
