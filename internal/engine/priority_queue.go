package engine

import (
	"industrial-4.0-demo/internal/types"
)

// Item 包装产品，增加在堆中的索引
type Item struct {
	Product *types.Product
	index   int
}

// PriorityQueue 实现 heap.Interface
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

// 注意：我们要的是高优先级先出，所以用 >
func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Product.Priority > pq[j].Product.Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // 避免内存泄漏
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
