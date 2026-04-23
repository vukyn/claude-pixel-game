package player

import (
	"testing"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/input"
	"claude-pixel/internal/stamina"
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

func stubAnim(id string, frames, durMs int, loop bool) *anim.Animation {
	spec := &anim.AnimationSpec{ID: id, FrameCount: frames, DurationMs: durMs, Loop: loop}
	return anim.NewAnimation(spec, nil)
}

func newTestPlayer(t *testing.T) *Player {
	t.Helper()
	anims := map[string]*anim.Animation{
		"soldier_idle":    stubAnim("soldier_idle", 10, 1000, true),
		"soldier_run":     stubAnim("soldier_run", 10, 1000, true),
		"soldier_jump":    stubAnim("soldier_jump", 3, 500, false),
		"soldier_fall":    stubAnim("soldier_fall", 3, 500, false),
		"soldier_attack":  stubAnim("soldier_attack", 4, 500, false),
		"soldier_attack2": stubAnim("soldier_attack2", 6, 750, false),
		"soldier_hit":     stubAnim("soldier_hit", 1, 200, false),
		"soldier_death":   stubAnim("soldier_death", 10, 1000, false),
	}
	boxes := map[string]combat.Box{
		"body":    {OffsetX: -20, OffsetY: -70, W: 40, H: 70, FrameStart: -1, FrameEnd: -1},
		"attack":  {OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2},
		"attack2": {OffsetX: 20, OffsetY: -60, W: 80, H: 60, FrameStart: 2, FrameEnd: 4},
	}
	return New(Config{
		StartX:     400,
		StartY:     600,
		Physics:    &Physics{RunSpeed: 100, SprintSpeed: 200, AirControl: 1, JumpVelocity: -100, Gravity: 500, MaxFallSpeed: 500},
		Anims:      anims,
		Boxes:      boxes,
		StartLives: 10,
		Stamina:    stamina.NewPool(100, 20, 20),
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

func TestPlayerOnHitEntersHit(t *testing.T) {
	p := newTestPlayer(t)
	p.Lives = 3
	p.OnHit(200, -300, p.X+10)
	if p.FSM.CurrentID() != StateHit {
		t.Errorf("want hit, got %q", p.FSM.CurrentID())
	}
	if p.Lives != 2 {
		t.Errorf("want 2 lives, got %d", p.Lives)
	}
	if p.Grounded {
		t.Errorf("expected airborne after knockback")
	}
	if p.VX >= 0 {
		t.Errorf("expected leftward bounce, got VX=%v", p.VX)
	}
}

func TestPlayerHitToIdleOnGround(t *testing.T) {
	p := newTestPlayer(t)
	p.Lives = 3
	p.OnHit(200, -300, p.X+10)
	p.Grounded = true
	p.FSM.Handle(p, input.Intent{}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateIdle {
		t.Errorf("want idle after land, got %q", p.FSM.CurrentID())
	}
	if p.HitFlag {
		t.Errorf("HitFlag should be cleared on Exit")
	}
}

func TestPlayerDeathOnZeroLives(t *testing.T) {
	p := newTestPlayer(t)
	p.Lives = 1
	p.OnHit(200, -300, p.X+10)
	if p.FSM.CurrentID() != StateDeath {
		t.Errorf("want death, got %q", p.FSM.CurrentID())
	}
}

func TestRunUsesRunSpeedWhenStaminaEmpty(t *testing.T) {
	pool := stamina.NewPool(100, 20, 20)
	pool.Cur = 0
	p := newTestPlayer(t)
	p.Stamina = pool
	p.Grounded = true
	p.FSM.Transition(p, StateRun)
	in := input.Intent{Right: true, SprintHeld: true}
	p.FSM.Handle(p, in, time.Second/60)
	if p.VX != p.Physics.RunSpeed {
		t.Fatalf("want VX=RunSpeed=%f when stamina empty, got %f", p.Physics.RunSpeed, p.VX)
	}
}

func TestRunUsesSprintSpeedWhenStaminaAvailable(t *testing.T) {
	pool := stamina.NewPool(100, 20, 20)
	p := newTestPlayer(t)
	p.Stamina = pool
	p.Grounded = true
	p.FSM.Transition(p, StateRun)
	in := input.Intent{Right: true, SprintHeld: true}
	p.FSM.Handle(p, in, time.Second/60)
	if p.VX != p.Physics.SprintSpeed {
		t.Fatalf("want VX=SprintSpeed=%f, got %f", p.Physics.SprintSpeed, p.VX)
	}
}
