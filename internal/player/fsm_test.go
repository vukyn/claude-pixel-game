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
		Physics: &Physics{RunSpeed: 100, AirControl: 1, JumpVelocity: -100, Gravity: 500, MaxFallSpeed: 500, DashSpeed: 200, DashDuration: 50 * time.Millisecond},
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

func TestAttackCancelByDash(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = true
	p.FSM.Handle(p, input.Intent{AttackPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateAttack {
		t.Fatalf("expected Attack, got %s", p.FSM.CurrentID())
	}
	p.FSM.Handle(p, input.Intent{DashPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateDash {
		t.Fatalf("dash cancel failed: %s", p.FSM.CurrentID())
	}
}

func TestAirDashConsumedOnce(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = false
	p.HasAirDash = true
	p.FSM.Handle(p, input.Intent{}, 16*time.Millisecond) // should land at Fall
	if p.FSM.CurrentID() != StateFall {
		t.Fatalf("expected Fall, got %s", p.FSM.CurrentID())
	}
	p.FSM.Handle(p, input.Intent{DashPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateDash {
		t.Fatalf("air-dash not triggered: %s", p.FSM.CurrentID())
	}
	for i := 0; i < 10; i++ {
		p.FSM.Handle(p, input.Intent{}, 10*time.Millisecond)
	}
	if p.HasAirDash {
		t.Fatal("HasAirDash should be false after air-dash")
	}
	p.FSM.Handle(p, input.Intent{DashPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() == StateDash {
		t.Fatal("second air-dash should be refused")
	}
}
