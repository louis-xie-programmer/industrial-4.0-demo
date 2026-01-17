package web

import (
	"industrial-4.0-demo/internal/types"
	"sync"
)

// ProductState 定义了用于 UI 展示的工件状态
// 这是一个简化的视图，只包含前端需要的数据
type ProductState struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Priority int                    `json:"priority"`
	Station  types.StationID        `json:"station"`
	Status   string                 `json:"status"`
	Attrs    map[string]interface{} `json:"attrs,omitempty"`
}

// GlobalState 代表整个工厂车间的实时状态快照
type GlobalState struct {
	Products map[string]ProductState `json:"products"`
}

// StateTracker 负责追踪所有工件的实时状态，并通知前端更新
type StateTracker struct {
	mu    sync.RWMutex
	state GlobalState
	hub   *Hub
}

// NewStateTracker 创建一个新的 StateTracker 实例
func NewStateTracker(hub *Hub) *StateTracker {
	return &StateTracker{
		state: GlobalState{Products: make(map[string]ProductState)},
		hub:   hub,
	}
}

// UpdateProductState 更新单个工件的状态，并向所有客户端广播最新的全局状态
func (st *StateTracker) UpdateProductState(id string, station types.StationID, status string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if product, ok := st.state.Products[id]; ok {
		product.Station = station
		product.Status = status
		st.state.Products[id] = product
	}
	// 注意：如果工件不存在，这里不会创建。新工件通过 AddProduct 添加。

	st.hub.BroadcastState(st.state)
}

// AddProduct 将一个新产品添加到状态追踪器中，并广播
func (st *StateTracker) AddProduct(p *types.Product) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.state.Products[p.ID] = ProductState{
		ID:       p.ID,
		Type:     p.Type,
		Priority: p.Priority,
		Station:  "", // 初始状态在队列中，不在任何工站
		Status:   "QUEUED",
		Attrs:    p.Attrs,
	}
	st.hub.BroadcastState(st.state)
}

// GetStateSnapshot 返回当前全局状态的一个深拷贝副本
// 用于新客户端连接时获取一次全量数据
func (st *StateTracker) GetStateSnapshot() GlobalState {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// 创建深拷贝以避免并发问题
	newState := GlobalState{Products: make(map[string]ProductState)}
	for id, p := range st.state.Products {
		newState.Products[id] = p
	}
	return newState
}
