package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// TasksInQueue 当前队列中的任务数量
	TasksInQueue = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_tasks_in_queue",
		Help: "The number of tasks currently waiting in the priority queue",
	})

	// TasksProcessedTotal 处理完成的任务总数（按状态分类）
	TasksProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_tasks_processed_total",
		Help: "The total number of processed tasks",
	}, []string{"status", "type"})

	// StationProcessingDuration 工站处理耗时直方图
	StationProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "station_processing_duration_seconds",
		Help:    "Time spent in each station",
		Buckets: prometheus.DefBuckets,
	}, []string{"station_id"})
)
