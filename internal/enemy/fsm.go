package enemy

import "time"

type StateID string

const (
	StateFall    StateID = "fall"
	StateRun     StateID = "run"
	StateAttack  StateID = "attack"
	StateAttack2 StateID = "attack2"
	StateHurt    StateID = "hurt"
	StateDeath   StateID = "death"
)

type State interface {
	ID() StateID
	Enter(e *Enemy)
	Update(e *Enemy, dt time.Duration) StateID
	Exit(e *Enemy)
}

type FSM struct {
	states    map[StateID]State
	initialID StateID
	current   State
}

func NewFSM(initial StateID) *FSM {
	return &FSM{states: map[StateID]State{}, initialID: initial}
}

func (f *FSM) Register(s State) { f.states[s.ID()] = s }

func (f *FSM) Start(e *Enemy) {
	f.current = f.states[f.initialID]
	if f.current != nil {
		f.current.Enter(e)
	}
}

func (f *FSM) CurrentID() StateID {
	if f.current == nil {
		return ""
	}
	return f.current.ID()
}

func (f *FSM) Handle(e *Enemy, dt time.Duration) {
	if f.current == nil {
		return
	}
	next := f.current.Update(e, dt)
	if next != f.current.ID() {
		f.current.Exit(e)
		ns, ok := f.states[next]
		if !ok {
			return
		}
		f.current = ns
		f.current.Enter(e)
	}
}

// Transition forces a state change (used by OnHit).
func (f *FSM) Transition(e *Enemy, to StateID) {
	if f.current != nil && f.current.ID() == to {
		return
	}
	if f.current != nil {
		f.current.Exit(e)
	}
	ns, ok := f.states[to]
	if !ok {
		return
	}
	f.current = ns
	f.current.Enter(e)
}
