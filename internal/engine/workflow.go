package engine

import (
	"fmt"
	"industrial-4.0-demo/internal/station"
	"industrial-4.0-demo/internal/types"
)

type WorkflowEngine struct {
	stations map[types.StationID]*station.Station
	sequence []types.StationID
}

func NewWorkflowEngine(sequence []types.StationID) *WorkflowEngine {
	return &WorkflowEngine{
		stations: make(map[types.StationID]*station.Station),
		sequence: sequence,
	}
}

func (e *WorkflowEngine) RegisterStation(s *station.Station) {
	e.stations[s.ID] = s
}

// Process 执行完整的编排流程（Saga 模式）
func (e *WorkflowEngine) Process(p *types.Product) {
	executedStations := []*station.Station{}

	fmt.Printf("\n>>> 开始生产工件: %s\n", p.ID)

	for i, stepID := range e.sequence {
		st := e.stations[stepID]
		p.Step = i // 更新当前步骤索引
		res := st.Execute(p)

		if !res.Success {
			fmt.Printf("ERR: 工件 %s 在 %s 失败: %v\n", p.ID, stepID, res.Error)
			p.Status = "FAILED" // 更新状态
			// 触发回滚补偿流程
			e.rollback(executedStations, p)
			return
		}

		executedStations = append(executedStations, st)
	}

	p.Status = "COMPLETED" // 更新状态
	fmt.Printf("SUCCESS: 工件 %s 已顺利下线！历史: %v\n", p.ID, p.History)
}

func (e *WorkflowEngine) rollback(stations []*station.Station, p *types.Product) {
	fmt.Printf("--- 启动 SAGA 补偿流程 ---\n")
	for i := len(stations) - 1; i >= 0; i-- {
		stations[i].Compensate(p)
	}
	fmt.Printf("--- 工件 %s 补偿完成，已安全剔除 ---\n", p.ID)
}
