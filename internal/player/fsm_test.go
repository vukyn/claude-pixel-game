package player

import (
	"testing"
	"time"

	"claude-pixel/internal/anim"
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

func newTestPlayer(t *testing.T) *Player {
	t.Helper()
	specs := map[string]*anim.AnimationSpec{
		"idle":    {ID: "idle", FrameCount: 1, DurationMs: 100, Loop: true},
		"run":     {ID: "run", FrameCount: 1, DurationMs: 100, Loop: true},
		"jump":    {ID: "jump", FrameCount: 1, DurationMs: 100, Loop: false},
		"fall":    {ID: "fall", FrameCount: 1, DurationMs: 100, Loop: false},
		"dash":    {ID: "dash", FrameCount: 1, DurationMs: 100, Loop: false},
		"attack":  {ID: "attack", FrameCount: 1, DurationMs: 100, Loop: false},
		"attack2": {ID: "attack2", FrameCount: 1, DurationMs: 100, Loop: false},
	}
	anims := map[string]*anim.Animation{}
	for k, s := range specs {
		anims[k] = anim.NewAnimation(s, nil)
	}
	return New(Config{
		StartX: 0, StartY: 0,
		Physics: &Physics{RunSpeed: 100, AirControl: 1, JumpVelocity: -100, Gravity: 500, MaxFallSpeed: 500, SprintSpeed: 200},
		Anims:   anims,
	})
}

func TestIdleToRunToJumpToFallToIdle(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = true

	p.FSM.Handle(p, input.Intent{Right: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateRun {
		t.Fatalf("expected Run, got %s", p.FSM.CurrentID())
	}

	p.FSM.Handle(p, input.Intent{Right: true, JumpPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateJump {
		t.Fatalf("expected Jump, got %s", p.FSM.CurrentID())
	}
	if p.VY != -100 {
		t.Fatalf("jump impulse not applied, VY=%v", p.VY)
	}

	p.VY = 50
	p.FSM.Handle(p, input.Intent{Right: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateFall {
		t.Fatalf("expected Fall, got %s", p.FSM.CurrentID())
	}

	p.Grounded = true
	p.FSM.Handle(p, input.Intent{}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateIdle {
		t.Fatalf("expected Idle, got %s", p.FSM.CurrentID())
	}
}

func TestAttackCancelByJump(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = true
	p.FSM.Handle(p, input.Intent{AttackPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateAttack {
		t.Fatalf("expected Attack, got %s", p.FSM.CurrentID())
	}
	p.FSM.Handle(p, input.Intent{JumpPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateJump {
		t.Fatalf("jump cancel failed: %s", p.FSM.CurrentID())
	}
}

func TestSprintIncreasesSpeed(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = true

	// First tick: idle → run (Enter runs, Update does not yet).
	p.FSM.Handle(p, input.Intent{Right: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateRun {
		t.Fatalf("expected Run, got %s", p.FSM.CurrentID())
	}

	// Second tick in Run without sprint: VX = RunSpeed * direction.
	p.FSM.Handle(p, input.Intent{Right: true}, 16*time.Millisecond)
	if p.VX != 100 {
		t.Fatalf("expected RunSpeed 100, got VX=%v", p.VX)
	}

	// Third tick with SprintHeld: VX = SprintSpeed * direction.
	p.FSM.Handle(p, input.Intent{Right: true, SprintHeld: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateRun {
		t.Fatalf("expected to stay in Run while sprinting, got %s", p.FSM.CurrentID())
	}
	if p.VX != 200 {
		t.Fatalf("expected SprintSpeed 200, got VX=%v", p.VX)
	}
}

func TestShiftAloneDoesNotMove(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = true

	p.FSM.Handle(p, input.Intent{SprintHeld: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateIdle {
		t.Fatalf("expected Idle with Shift alone, got %s", p.FSM.CurrentID())
	}
	if p.VX != 0 {
		t.Fatalf("expected VX=0 with Shift alone, got %v", p.VX)
	}
}
