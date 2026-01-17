package event

import (
	"industrial-4.0-demo/internal/types"
	"sync"
)

// EventType 定义事件的类型
type EventType string

// 定义所有业务事件类型
const (
	ProductStarted     EventType = "ProductStarted"     // 产品开始生产
	ProductCompleted   EventType = "ProductCompleted"   // 产品成功完成
	ProductFailed      EventType = "ProductFailed"      // 产品生产失败
	ProductCompensated EventType = "ProductCompensated" // 产品补偿完成
	StepStarted        EventType = "StepStarted"        // 步骤开始执行
	StepCompleted      EventType = "StepCompleted"      // 步骤执行完成
)

// Event 结构体定义了事件的数据负载
type Event struct {
	Type      EventType       // 事件类型
	ProductID string          // 关联的产品 ID
	Product   *types.Product  // 完整的产品数据
	StationID types.StationID // 关联的工站 ID (仅步骤相关事件)
	Error     error           // 错误信息 (仅失败事件)
}

// Handler 是事件处理函数的签名
type Handler func(e Event)

// Bus 是一个简单的内存事件总线
type Bus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler // 存储事件类型到多个处理函数的映射
}

// NewBus 创建一个新的事件总线实例
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[EventType][]Handler),
	}
}

// Subscribe 订阅一个特定类型的事件
func (b *Bus) Subscribe(eventType EventType, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Publish 发布一个事件，所有订阅了该事件类型的处理器都将被调用
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if handlers, ok := b.handlers[e.Type]; ok {
		// 遍历所有处理器并异步执行
		// 使用 goroutine 避免单个处理器的阻塞影响其他处理器
		for _, handler := range handlers {
			go handler(e)
		}
	}
}
