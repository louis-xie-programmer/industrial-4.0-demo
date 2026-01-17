package engine

import (
	"container/heap"
	"context"
	"industrial-4.0-demo/internal/metrics"
	"industrial-4.0-demo/internal/persistence"
	"industrial-4.0-demo/internal/types"
	"industrial-4.0-demo/internal/util"
	"industrial-4.0-demo/internal/web"
	"log/slog"
	"sync"
)

// Scheduler 负责任务的调度和分发
// 它维护一个优先级队列，并控制并发执行的 worker 数量
type Scheduler struct {
	pq           PriorityQueue     // 优先级队列，存储待处理的任务
	engine       *WorkflowEngine   // 工作流引擎，用于执行任务
	mu           sync.Mutex        // 互斥锁，保护队列并发访问
	cond         *sync.Cond        // 条件变量，用于通知 worker 有新任务
	maxWorkers   int               // 最大并发 worker 数
	wg           sync.WaitGroup    // 等待组，用于优雅停机
	wal          *persistence.WAL  // 预写日志，用于持久化任务
	stateTracker *web.StateTracker // 状态追踪器，用于更新前端状态
	logger       *slog.Logger      // 结构化日志记录器
}

// NewScheduler 创建一个新的 Scheduler 实例
func NewScheduler(engine *WorkflowEngine, maxWorkers int, wal *persistence.WAL, st *web.StateTracker, logger *slog.Logger) *Scheduler {
	s := &Scheduler{
		pq:           make(PriorityQueue, 0),
		engine:       engine,
		maxWorkers:   maxWorkers,
		wal:          wal,
		stateTracker: st,
		logger:       logger.With("component", "scheduler"),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// RecoverTasks 从 WAL 日志中恢复未完成的任务
// 在系统启动时调用，确保任务不丢失
func (s *Scheduler) RecoverTasks() error {
	if s.wal == nil {
		return nil
	}
	tasks, err := s.wal.Recover()
	if err != nil {
		return err
	}
	for _, p := range tasks {
		s.logger.Info("重新加载未完成的工件", "product_id", p.ID)
		s.submit(p) // 内部提交，不重复写 WAL
	}
	return nil
}

// SubmitTask 提交一个新任务到调度器
// 先写入 WAL 持久化，再放入内存队列
func (s *Scheduler) SubmitTask(p *types.Product) {
	if s.wal != nil {
		if err := s.wal.Append(p); err != nil {
			s.logger.Error("写入 WAL 失败", "error", err, "product_id", p.ID)
			// 注意：生产环境中这里可能需要返回错误或重试
		}
	}
	s.submit(p)
}

// submit 将任务放入优先级队列并唤醒 worker
func (s *Scheduler) submit(p *types.Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger.Info("接收到工件", "product_id", p.ID, "type", p.Type, "priority", p.Priority)
	heap.Push(&s.pq, &Item{Product: p})
	metrics.TasksInQueue.Inc()
	s.stateTracker.AddProduct(p)
	s.cond.Signal() // 唤醒一个等待的 worker
}

// Start 启动调度循环
// 启动 worker 池来并发处理任务
func (s *Scheduler) Start(ctx context.Context) {
	workerPool := make(chan struct{}, s.maxWorkers)

	// 监听上下文取消信号，用于优雅停机
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		s.cond.Broadcast() // 唤醒所有 worker 以便它们退出
		s.mu.Unlock()
	}()

	for {
		s.mu.Lock()
		// 如果队列为空，等待新任务
		for s.pq.Len() == 0 {
			if ctx.Err() != nil {
				s.mu.Unlock()
				return
			}
			s.cond.Wait()
		}

		// 再次检查是否需要退出
		if ctx.Err() != nil {
			s.mu.Unlock()
			return
		}

		// 取出优先级最高的任务
		item := heap.Pop(&s.pq).(*Item)
		metrics.TasksInQueue.Dec()
		s.mu.Unlock()

		// 获取 worker 凭证（控制并发数）
		workerPool <- struct{}{}
		s.wg.Add(1)

		// 启动 goroutine 执行任务
		go func(p *types.Product) {
			defer s.wg.Done()

			// 生成 Trace ID 并注入 Context，用于全链路追踪
			traceID := util.NewTraceID()
			taskCtx := util.ContextWithTraceID(ctx, traceID)

			s.engine.Process(taskCtx, p)

			// 任务完成后标记 WAL
			if s.wal != nil {
				_ = s.wal.Complete(p.ID)
			}
			<-workerPool // 释放 worker 凭证
		}(item.Product)
	}
}

// WaitForCompletion 等待所有正在执行的任务完成
// 用于优雅停机
func (s *Scheduler) WaitForCompletion() {
	s.wg.Wait()
}
