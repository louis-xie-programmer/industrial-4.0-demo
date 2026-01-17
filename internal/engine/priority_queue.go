package engine

import (
	"industrial-4.0-demo/internal/types"
)

// Item 是优先级队列中的元素，包装了 Product
type Item struct {
	Product *types.Product // 实际的工件数据
	index   int            // 元素在堆中的索引，用于 update 操作（虽然本项目未用到）
}

// PriorityQueue 实现了 heap.Interface 接口，是一个基于最小堆的优先级队列
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

// Less 定义了元素的排序规则
// 注意：我们要实现最大堆（高优先级先出），所以这里使用 >
func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Product.Priority > pq[j].Product.Priority
}

// Swap 交换两个元素的位置
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push 向队列中添加元素
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

// Pop 从队列中移除并返回优先级最高的元素
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // 避免内存泄漏
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
