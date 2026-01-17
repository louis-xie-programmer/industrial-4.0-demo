package fsm

import (
	"fmt"
	"sync"
)

// State 定义状态的类型
type State string

// Event 定义事件的类型
type Event string

// 定义所有可能的状态
const (
	StateCreated      State = "CREATED"       // 已创建
	StateProcessing   State = "PROCESSING"    // 处理中
	StateQualityCheck State = "QUALITY_CHECK" // 质检中
	StateCompleted    State = "COMPLETED"     // 已完成
	StateFailed       State = "FAILED"        // 已失败
	StateCompensating State = "COMPENSATING"  // 补偿中
	StateCompensated  State = "COMPENSATED"   // 已补偿
)

// 定义所有可能触发状态转移的事件
const (
	EventStart      Event = "START"         // 开始处理
	EventEnterQC    Event = "ENTER_QC"      // 进入质检
	EventPassQC     Event = "PASS_QC"       // 质检通过
	EventFinish     Event = "FINISH"        // 完成处理
	EventFail       Event = "FAIL"          // 处理失败
	EventCompensate Event = "COMPENSATE"    // 开始补偿
	EventRollback   Event = "ROLLBACK_DONE" // 补偿完成
)

// FSM 是一个简单的有限状态机实现
type FSM struct {
	Current     State                           // 当前状态
	mu          sync.Mutex                      // 互斥锁，保证并发安全
	transitions map[State]map[Event]State       // 状态转移表: map[当前状态]map[事件]下一个状态
	callbacks   map[State]func(targetID string) // 状态进入时的回调函数
	TargetID    string                          // 状态机关联的目标对象 ID (如工件 ID)
}

// NewFSM 创建一个新的 FSM 实例
func NewFSM(targetID string) *FSM {
	fsm := &FSM{
		Current:     StateCreated,
		TargetID:    targetID,
		transitions: make(map[State]map[Event]State),
		callbacks:   make(map[State]func(string)),
	}
	fsm.initTransitions()
	return fsm
}

// initTransitions 初始化状态转移表
func (f *FSM) initTransitions() {
	f.addTransition(StateCreated, EventStart, StateProcessing)
	f.addTransition(StateProcessing, EventEnterQC, StateQualityCheck)
	f.addTransition(StateProcessing, EventFinish, StateCompleted)
	f.addTransition(StateProcessing, EventFail, StateFailed)

	f.addTransition(StateQualityCheck, EventPassQC, StateProcessing)
	f.addTransition(StateQualityCheck, EventFinish, StateCompleted)
	f.addTransition(StateQualityCheck, EventFail, StateFailed)

	f.addTransition(StateFailed, EventCompensate, StateCompensating)
	f.addTransition(StateCompensating, EventRollback, StateCompensated)
}

// addTransition 添加一条状态转移规则
func (f *FSM) addTransition(from State, event Event, to State) {
	if _, ok := f.transitions[from]; !ok {
		f.transitions[from] = make(map[Event]State)
	}
	f.transitions[from][event] = to
}

// RegisterCallback 注册进入某个状态时的回调函数
func (f *FSM) RegisterCallback(state State, callback func(targetID string)) {
	f.callbacks[state] = callback
}

// Fire 触发一个事件，尝试进行状态转移
func (f *FSM) Fire(event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 查找当前状态下，该事件是否能触发合法的转移
	nextState, ok := f.transitions[f.Current][event]
	if !ok {
		return fmt.Errorf("invalid transition: cannot fire event '%s' from state '%s'", event, f.Current)
	}

	prevState := f.Current
	f.Current = nextState

	// 触发回调（如果已注册）
	if cb, exists := f.callbacks[nextState]; exists {
		// 注意：回调是同步执行的，应避免在回调中执行耗时操作或再次调用 Fire 导致死锁
		cb(f.TargetID)
	}

	return nil
}
