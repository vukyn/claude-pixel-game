package player

import (
	"time"

	"claude-pixel/internal/input"
)

type StateID string

const (
	StateIdle    StateID = "idle"
	StateRun     StateID = "run"
	StateJump    StateID = "jump"
	StateFall    StateID = "fall"
	StateDash    StateID = "dash"
	StateAttack  StateID = "attack"
	StateAttack2 StateID = "attack2"
)

type State interface {
	ID() StateID
	Enter(p *Player)
	Update(p *Player, in input.Intent, dt time.Duration) StateID
	Exit(p *Player)
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

func (f *FSM) Start(p *Player) {
	f.current = f.states[f.initialID]
	if f.current != nil {
		f.current.Enter(p)
	}
}

func (f *FSM) CurrentID() StateID {
	if f.current == nil {
		return ""
	}
	return f.current.ID()
}

func (f *FSM) Handle(p *Player, in input.Intent, dt time.Duration) {
	if f.current == nil {
		return
	}
	next := f.current.Update(p, in, dt)
	if next != f.current.ID() {
		f.current.Exit(p)
		ns, ok := f.states[next]
		if !ok {
			return
		}
		f.current = ns
		f.current.Enter(p)
	}
}
