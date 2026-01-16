package station

import (
	"fmt"
	"industrial-4.0-demo/internal/types"
	"math/rand"
	"time"
)

type Station struct {
	ID types.StationID
}

func NewStation(id types.StationID) *Station {
	return &Station{ID: id}
}

// Execute 模拟物理工站的动作执行
func (s *Station) Execute(p *types.Product) types.Result {
	fmt.Printf("[工站 %s] 正在处理工件: %s...\n", s.ID, p.ID)

	// 模拟工业加工耗时
	time.Sleep(time.Duration(rand.Intn(500)+500) * time.Millisecond)

	// 模拟质检环节可能出现的失败
	if s.ID == types.StationQC {
		if rand.Float32() < 0.3 { // 30% 概率不合格
			return types.Result{ProductID: p.ID, Success: false, Error: fmt.Errorf("质检未通过")}
		}
	}

	p.History = append(p.History, string(s.ID))
	return types.Result{ProductID: p.ID, Success: true}
}

// Compensate 补偿逻辑（回滚动作）
func (s *Station) Compensate(p *types.Product) {
	fmt.Printf("<!> [工站 %s] 执行补偿：正在拆卸/退回工件 %s\n", s.ID, p.ID)
	time.Sleep(300 * time.Millisecond)
}
