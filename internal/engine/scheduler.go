package engine

import (
	"container/heap"
	"context"
	"fmt"
	"industrial-4.0-demo/internal/metrics"
	"industrial-4.0-demo/internal/persistence"
	"industrial-4.0-demo/internal/types"
	"sync"
)

type Scheduler struct {
	pq         PriorityQueue
	engine     *WorkflowEngine
	mu         sync.Mutex
	cond       *sync.Cond // 用于通知调度协程有新任务
	maxWorkers int        // 生产线并行能力（同时加工的工件数）
	wg         sync.WaitGroup
	wal        *persistence.WAL // 预写日志
}

func NewScheduler(engine *WorkflowEngine, maxWorkers int, wal *persistence.WAL) *Scheduler {
	s := &Scheduler{
		pq:         make(PriorityQueue, 0),
		engine:     engine,
		maxWorkers: maxWorkers,
		wal:        wal,
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// RecoverTasks 从 WAL 恢复未完成的任务
func (s *Scheduler) RecoverTasks() error {
	if s.wal == nil {
		return nil
	}

	tasks, err := s.wal.Recover()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range tasks {
		fmt.Printf("[恢复] 重新加载未完成工件: %s\n", p.ID)
		heap.Push(&s.pq, &Item{Product: p})
		metrics.TasksInQueue.Inc()
	}

	if len(tasks) > 0 {
		s.cond.Signal()
	}
	return nil
}

// SubmitTask 提交新订单
func (s *Scheduler) SubmitTask(p *types.Product) {
	// 1. 先写入 WAL
	if s.wal != nil {
		if err := s.wal.Append(p); err != nil {
			fmt.Printf("ERR: 写入 WAL 失败: %v\n", err)
			// 在生产环境中，这里可能需要拒绝任务或重试
		}
	}

	// 2. 再更新内存状态
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("[调度器] 接收到工件 %s (类型: %s, 优先级: %d)\n", p.ID, p.Type, p.Priority)
	heap.Push(&s.pq, &Item{Product: p})
	metrics.TasksInQueue.Inc() // 指标：队列长度 +1
	s.cond.Signal()            // 唤醒调度协程
}

// Start 启动调度引擎
func (s *Scheduler) Start(ctx context.Context) {
	workerPool := make(chan struct{}, s.maxWorkers)

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		s.cond.Broadcast() // 唤醒所有等待的 goroutine 以便退出
		s.mu.Unlock()
	}()

	for {
		s.mu.Lock()
		// 如果队列为空，则等待
		for s.pq.Len() == 0 {
			if ctx.Err() != nil {
				s.mu.Unlock()
				return
			}
			s.cond.Wait()
		}

		if ctx.Err() != nil {
			s.mu.Unlock()
			return
		}

		// 取出优先级最高的任务
		item := heap.Pop(&s.pq).(*Item)
		metrics.TasksInQueue.Dec() // 指标：队列长度 -1
		s.mu.Unlock()

		// 占用工作位
		workerPool <- struct{}{}
		s.wg.Add(1)

		go func(p *types.Product) {
			defer s.wg.Done()
			s.engine.Process(p)

			// 任务完成后，标记 WAL
			if s.wal != nil {
				// 注意：这里简单地认为 Process 结束就是任务完成（无论成功失败）
				// 实际场景可能需要区分状态
				_ = s.wal.Complete(p.ID)
			}

			<-workerPool // 释放工作位
		}(item.Product)
	}
}

// WaitForCompletion 等待所有任务完成
func (s *Scheduler) WaitForCompletion() {
	s.wg.Wait()
}
