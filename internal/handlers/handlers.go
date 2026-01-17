package handlers

import (
	"industrial-4.0-demo/internal/event"
	"industrial-4.0-demo/internal/fsm"
	"industrial-4.0-demo/internal/metrics"
	"industrial-4.0-demo/internal/types"
	"industrial-4.0-demo/internal/web"
	"log/slog"
)

// RegisterEventHandlers 将所有事件处理器注册到事件总线
// 这是事件驱动架构的核心，将不同的业务关注点（监控、UI、日志）解耦
func RegisterEventHandlers(bus *event.Bus, st *web.StateTracker, logger *slog.Logger) {
	// --- 指标处理器 (Metrics Handler) ---
	// 订阅产品完成事件，增加成功计数器
	bus.Subscribe(event.ProductCompleted, func(e event.Event) {
		metrics.TasksProcessedTotal.WithLabelValues("success", e.Product.Type).Inc()
	})
	// 订阅产品失败事件，增加失败计数器
	bus.Subscribe(event.ProductFailed, func(e event.Event) {
		metrics.TasksProcessedTotal.WithLabelValues("failed", e.Product.Type).Inc()
	})
	// 订阅步骤完成事件，记录工站处理耗时
	bus.Subscribe(event.StepCompleted, func(e event.Event) {
		if duration, ok := e.Product.Attrs["duration"].(float64); ok {
			metrics.StationProcessingDuration.WithLabelValues(string(e.StationID)).Observe(duration)
		}
	})

	// --- Web UI 处理器 (Web UI Handler) ---
	// 订阅产品开始事件，更新 UI 状态
	bus.Subscribe(event.ProductStarted, func(e event.Event) {
		st.UpdateProductState(e.ProductID, types.StationCAM, string(fsm.StateProcessing))
	})
	// 订阅步骤开始事件，更新 UI 中工件的位置
	bus.Subscribe(event.StepStarted, func(e event.Event) {
		st.UpdateProductState(e.ProductID, e.StationID, string(fsm.StateProcessing))
	})
	// 订阅产品完成事件，将工件移动到出货区
	bus.Subscribe(event.ProductCompleted, func(e event.Event) {
		st.UpdateProductState(e.ProductID, types.StationPack, string(fsm.StateCompleted))
	})
	// 订阅产品失败事件，更新 UI 状态
	bus.Subscribe(event.ProductFailed, func(e event.Event) {
		st.UpdateProductState(e.ProductID, "", string(fsm.StateFailed))
	})
	// 订阅产品补偿完成事件，更新 UI 状态
	bus.Subscribe(event.ProductCompensated, func(e event.Event) {
		st.UpdateProductState(e.ProductID, "", string(fsm.StateCompensated))
	})

	// --- 日志处理器 (Logging Handler) ---
	// 订阅关键业务事件，记录审计日志
	bus.Subscribe(event.ProductFailed, func(e event.Event) {
		logger.Error("产品处理失败", "product_id", e.ProductID, "error", e.Error)
	})
	bus.Subscribe(event.ProductCompleted, func(e event.Event) {
		logger.Info("产品处理成功", "product_id", e.ProductID)
	})
}
