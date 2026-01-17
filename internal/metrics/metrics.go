package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 定义 Prometheus 监控指标
var (
	// TasksInQueue 仪表盘：当前队列中的任务数量
	// 用于监控系统积压情况
	TasksInQueue = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_tasks_in_queue",
		Help: "The number of tasks currently waiting in the priority queue",
	})

	// TasksProcessedTotal 计数器：处理完成的任务总数
	// 按状态 (success/failed) 和产品类型分类
	TasksProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_tasks_processed_total",
		Help: "The total number of processed tasks",
	}, []string{"status", "type"})

	// StationProcessingDuration 直方图：工站处理耗时分布
	// 用于分析各工站的性能瓶颈
	StationProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "station_processing_duration_seconds",
		Help:    "Time spent in each station",
		Buckets: prometheus.DefBuckets,
	}, []string{"station_id"})
)
