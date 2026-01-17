package station

import (
	"context"
	"fmt"
	"industrial-4.0-demo/internal/types"
	"industrial-4.0-demo/internal/util"
	"log/slog"
	"math/rand"
	"time"
)

// Station 定义了所有工站必须实现的接口
type Station interface {
	GetID() types.StationID                                     // 获取工站的唯一 ID
	Execute(ctx context.Context, p *types.Product) types.Result // 执行正向生产逻辑
	Compensate(ctx context.Context, p *types.Product)           // 执行逆向补偿逻辑 (Saga)
}

// LocalStation 代表一个在本地模拟的工站
type LocalStation struct {
	ID     types.StationID // 工站 ID
	logger *slog.Logger    // 日志记录器
}

// NewStation 创建一个新的本地工站实例
func NewStation(id types.StationID, logger *slog.Logger) Station {
	return &LocalStation{
		ID:     id,
		logger: logger.With("station_id", id), // 为该工站的日志添加固定字段
	}
}

func (s *LocalStation) GetID() types.StationID {
	return s.ID
}

// Execute 模拟物理工站的动作执行
func (s *LocalStation) Execute(ctx context.Context, p *types.Product) types.Result {
	// 从 Context 中提取 Trace ID 并添加到日志中
	logger := s.logger
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		logger = logger.With("trace_id", traceID)
	}

	logger.Info("开始处理工件", "product_id", p.ID)

	// 模拟工业加工耗时，增加到 2-4 秒，以便前端观察
	processTime := time.Duration(rand.Intn(2000)+2000) * time.Millisecond
	time.Sleep(processTime)

	// 模拟电测环节可能出现的失败
	if s.ID == types.StationETest {
		if rand.Float32() < 0.05 { // 5% 概率电测不通过
			logger.Warn("工件电测失败", "product_id", p.ID)
			return types.Result{ProductID: p.ID, Success: false, Error: fmt.Errorf("电测未通过")}
		}
	}

	p.History = append(p.History, string(s.ID))
	logger.Info("工件处理完成", "product_id", p.ID, "duration", processTime.Seconds())
	return types.Result{ProductID: p.ID, Success: true}
}

// Compensate 模拟补偿逻辑（回滚动作）
func (s *LocalStation) Compensate(ctx context.Context, p *types.Product) {
	logger := s.logger
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		logger = logger.With("trace_id", traceID)
	}
	logger.Warn("执行补偿逻辑", "product_id", p.ID)
	time.Sleep(1000 * time.Millisecond) // 补偿也增加一点延时
}
