package player

import (
	"testing"
	"time"

	"claude-pixel/internal/input"
)

type countingState struct {
	id      StateID
	next    StateID
	enters  int
	exits   int
	updates int
}

func (c *countingState) ID() StateID                                                  { return c.id }
func (c *countingState) Enter(p *Player)                                              { c.enters++ }
func (c *countingState) Exit(p *Player)                                               { c.exits++ }
func (c *countingState) Update(p *Player, in input.Intent, dt time.Duration) StateID { c.updates++; return c.next }

func TestFSMTransitions(t *testing.T) {
	a := &countingState{id: "A", next: "A"}
	b := &countingState{id: "B", next: "B"}

	fsm := NewFSM("A")
	fsm.Register(a)
	fsm.Register(b)
	fsm.Start(&Player{})

	if a.enters != 1 {
		t.Fatal("initial Enter not called")
	}

	fsm.Handle(&Player{}, input.Intent{}, time.Millisecond)
	if a.updates != 1 || fsm.CurrentID() != "A" {
		t.Fatal("no-op transition failed")
	}

	a.next = "B"
	fsm.Handle(&Player{}, input.Intent{}, time.Millisecond)
	if a.exits != 1 || b.enters != 1 || fsm.CurrentID() != "B" {
		t.Fatalf("transition A->B failed, exits=%d enters(b)=%d current=%s", a.exits, b.enters, fsm.CurrentID())
	}
}
