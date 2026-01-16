package fsm

import (
	"fmt"
	"sync"
)

// State 定义状态类型
type State string

// Event 定义事件类型
type Event string

const (
	StateCreated      State = "CREATED"
	StateProcessing   State = "PROCESSING"
	StateQualityCheck State = "QUALITY_CHECK"
	StateCompleted    State = "COMPLETED"
	StateFailed       State = "FAILED"
	StateCompensating State = "COMPENSATING"
	StateCompensated  State = "COMPENSATED"
)

const (
	EventStart      Event = "START"
	EventEnterQC    Event = "ENTER_QC"
	EventPassQC     Event = "PASS_QC"
	EventFinish     Event = "FINISH"
	EventFail       Event = "FAIL"
	EventCompensate Event = "COMPENSATE"
	EventRollback   Event = "ROLLBACK_DONE"
)

// FSM 有限状态机
type FSM struct {
	Current State
	mu      sync.Mutex
	// transitions 定义状态转移表: CurrentState -> Event -> NextState
	transitions map[State]map[Event]State
	// callbacks 定义状态变更后的回调: State -> func()
	callbacks map[State]func(targetID string)
	TargetID  string // 关联的目标对象ID（如工件ID）
}

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

func (f *FSM) initTransitions() {
	f.addTransition(StateCreated, EventStart, StateProcessing)
	f.addTransition(StateProcessing, EventEnterQC, StateQualityCheck)
	f.addTransition(StateProcessing, EventFinish, StateCompleted) // 无质检流程
	f.addTransition(StateProcessing, EventFail, StateFailed)

	f.addTransition(StateQualityCheck, EventPassQC, StateProcessing) // 质检通过继续加工
	f.addTransition(StateQualityCheck, EventFinish, StateCompleted)  // 质检是最后一步
	f.addTransition(StateQualityCheck, EventFail, StateFailed)

	f.addTransition(StateFailed, EventCompensate, StateCompensating)
	f.addTransition(StateCompensating, EventRollback, StateCompensated)
}

func (f *FSM) addTransition(from State, event Event, to State) {
	if _, ok := f.transitions[from]; !ok {
		f.transitions[from] = make(map[Event]State)
	}
	f.transitions[from][event] = to
}

// RegisterCallback 注册状态进入时的回调
func (f *FSM) RegisterCallback(state State, callback func(targetID string)) {
	f.callbacks[state] = callback
}

// Fire 触发事件
func (f *FSM) Fire(event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 查找合法的转移
	nextState, ok := f.transitions[f.Current][event]
	if !ok {
		return fmt.Errorf("invalid transition: cannot fire event %s from state %s", event, f.Current)
	}

	prevState := f.Current
	f.Current = nextState

	fmt.Printf("[FSM] %s: %s -> %s (Event: %s)\n", f.TargetID, prevState, nextState, event)

	// 触发回调
	if cb, exists := f.callbacks[nextState]; exists {
		// 异步执行回调，避免阻塞 FSM 锁？视情况而定，这里为了简单同步执行
		// 但要注意死锁风险，回调中不要再调用 Fire
		cb(f.TargetID)
	}

	return nil
}
