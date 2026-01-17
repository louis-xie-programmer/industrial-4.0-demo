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
	GetID() types.StationID
	Execute(ctx context.Context, p *types.Product) types.Result
	Compensate(ctx context.Context, p *types.Product)
}

// LocalStation 代表一个在本地模拟的工站
type LocalStation struct {
	ID      types.StationID
	logger  *slog.Logger
	delayMs int
}

// NewStation 创建一个新的本地工站实例
func NewStation(id types.StationID, logger *slog.Logger, delayMs int) Station {
	return &LocalStation{
		ID:      id,
		logger:  logger.With("station_id", id),
		delayMs: delayMs,
	}
}

func (s *LocalStation) GetID() types.StationID {
	return s.ID
}

// Execute 模拟物理工站的动作执行
func (s *LocalStation) Execute(ctx context.Context, p *types.Product) types.Result {
	logger := s.logger
	if traceID, ok := util.TraceIDFromContext(ctx); ok {
		logger = logger.With("trace_id", traceID)
	}

	logger.Info("开始处理工件", "product_id", p.ID)

	var processTime time.Duration
	if s.delayMs <= 1 {
		processTime = time.Duration(s.delayMs) * time.Millisecond
	} else {
		processTime = time.Duration(s.delayMs+rand.Intn(s.delayMs/2)) * time.Millisecond
	}
	time.Sleep(processTime)

	if s.ID == types.StationETest {
		if rand.Float32() < 0.05 {
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

	// *** BUG FIX: 补偿延时也使用配置，并确保测试时延时足够短 ***
	var compensateTime time.Duration
	if s.delayMs <= 1 {
		compensateTime = time.Duration(s.delayMs) * time.Millisecond
	} else {
		compensateTime = 1500 * time.Millisecond // 生产演示时保持 1.5s
	}
	time.Sleep(compensateTime)
}
