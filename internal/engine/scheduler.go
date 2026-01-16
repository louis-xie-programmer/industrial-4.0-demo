package engine

import (
	"container/heap"
	"context"
	"fmt"
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
}

func NewScheduler(engine *WorkflowEngine, maxWorkers int) *Scheduler {
	s := &Scheduler{
		pq:         make(PriorityQueue, 0),
		engine:     engine,
		maxWorkers: maxWorkers,
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// SubmitTask 提交新订单
func (s *Scheduler) SubmitTask(p *types.Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("[调度器] 接收到工件 %s (优先级: %d)\n", p.ID, p.Priority)
	heap.Push(&s.pq, &Item{Product: p})
	s.cond.Signal() // 唤醒调度协程
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
		s.mu.Unlock()

		// 占用工作位
		workerPool <- struct{}{}
		s.wg.Add(1)

		go func(p *types.Product) {
			defer s.wg.Done()
			s.engine.Process(p)
			<-workerPool // 释放工作位
		}(item.Product)
	}
}

// WaitForCompletion 等待所有任务完成
func (s *Scheduler) WaitForCompletion() {
	s.wg.Wait()
}
