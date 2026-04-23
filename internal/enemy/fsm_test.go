package enemy

import (
	"math/rand"
	"testing"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
)

func stubAnim(id string, frames, durMs int, loop bool) *anim.Animation {
	spec := &anim.AnimationSpec{ID: id, FrameCount: frames, DurationMs: durMs, Loop: loop}
	return anim.NewAnimation(spec, nil)
}

func newOrcKind() *Kind {
	return &Kind{
		Name:       "orc",
		AnimPrefix: "orc",
		FrameW:     100,
		FrameH:     100,
		Tuning:     &Tuning{MaxLives: 2, RunSpeed: 80, IntentTickS: 2, HurtBounceVX: 120, HurtBounceVY: -180},
		Anims: map[string]*anim.Animation{
			"idle":    stubAnim("orc_idle", 6, 900, true),
			"run":     stubAnim("orc_run", 8, 700, true),
			"attack":  stubAnim("orc_attack", 6, 600, false),
			"attack2": stubAnim("orc_attack2", 6, 700, false),
			"hurt":    stubAnim("orc_hurt", 4, 400, false),
			"death":   stubAnim("orc_death", 4, 500, false),
		},
		Boxes: map[string]combat.Box{
			"body":    {OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1},
			"attack":  {OffsetX: 25, OffsetY: -70, W: 60, H: 60, FrameStart: 2, FrameEnd: 3},
			"attack2": {OffsetX: 25, OffsetY: -70, W: 70, H: 60, FrameStart: 3, FrameEnd: 4},
		},
		Motions: nil,
	}
}

func newTestEnemyOrc() *Enemy {
	ph := &player.Physics{Gravity: 1800, MaxFallSpeed: 900}
	return New(Config{
		StartX: 400, StartY: -100,
		Physics: ph,
		Kind:    newOrcKind(),
		RNG:     rand.New(rand.NewSource(1)),
	})
}

func TestEnemyStartsInFall(t *testing.T) {
	e := newTestEnemyOrc()
	if e.FSM.CurrentID() != StateFall {
		t.Errorf("want fall, got %q", e.FSM.CurrentID())
	}
}

func TestEnemyFallToRunOnGrounded(t *testing.T) {
	e := newTestEnemyOrc()
	e.Grounded = true
	e.FSM.Handle(e, 16*time.Millisecond)
	if e.FSM.CurrentID() != StateRun {
		t.Errorf("want run, got %q", e.FSM.CurrentID())
	}
}

func TestEnemyHurtOnDamage(t *testing.T) {
	e := newTestEnemyOrc()
	e.Grounded = true
	e.FSM.Handle(e, 16*time.Millisecond)
	e.OnHit(e.X + 10)
	if e.FSM.CurrentID() != StateHurt {
		t.Errorf("want hurt, got %q", e.FSM.CurrentID())
	}
	if e.VX >= 0 {
		t.Errorf("expected leftward bounce, got VX=%v", e.VX)
	}
}

func TestEnemyDiesOnFatalHit(t *testing.T) {
	e := newTestEnemyOrc()
	e.Grounded = true
	e.FSM.Handle(e, 16*time.Millisecond)
	e.Lives = 1
	e.OnHit(e.X + 10)
	if e.FSM.CurrentID() != StateDeath {
		t.Errorf("want death, got %q", e.FSM.CurrentID())
	}
}

func newSlimeKind() *Kind {
	return &Kind{
		Name:       "slime",
		AnimPrefix: "slime",
		FrameW:     96,
		FrameH:     96,
		Tuning:     &Tuning{MaxLives: 2, RunSpeed: 60, IntentTickS: 2, HurtBounceVX: 120, HurtBounceVY: -180},
		Anims: map[string]*anim.Animation{
			"idle":    stubAnim("slime_idle", 6, 900, true),
			"run":     stubAnim("slime_run", 8, 700, true),
			"attack":  stubAnim("slime_attack", 8, 650, false),
			"attack2": stubAnim("slime_attack2", 8, 700, false),
			"hurt":    stubAnim("slime_hurt", 4, 400, false),
			"death":   stubAnim("slime_death", 10, 800, false),
		},
		Boxes: map[string]combat.Box{
			"body":    {OffsetX: -20, OffsetY: -40, W: 40, H: 40, FrameStart: -1, FrameEnd: -1},
			"attack":  {OffsetX: 15, OffsetY: -35, W: 45, H: 35, FrameStart: 4, FrameEnd: 5},
			"attack2": {OffsetX: 15, OffsetY: -35, W: 55, H: 40, FrameStart: 3, FrameEnd: 5},
		},
		Motions: map[string]AttackMotion{
			"attack2": {VX: -60, FrameStart: 3, FrameEnd: 5},
		},
	}
}

func newTestEnemySlime() *Enemy {
	ph := &player.Physics{Gravity: 1800, MaxFallSpeed: 900}
	return New(Config{
		StartX: 400, StartY: -100,
		Physics: ph,
		Kind:    newSlimeKind(),
		RNG:     rand.New(rand.NewSource(1)),
	})
}

func TestSlimeAttack2BackstepAppliesVXOnActiveFrames(t *testing.T) {
	e := newTestEnemySlime()
	e.Facing = 1
	e.Grounded = true
	// force into attack2
	e.FSM.Transition(e, StateAttack2)
	if e.FSM.CurrentID() != StateAttack2 {
		t.Fatalf("failed to enter attack2, got %q", e.FSM.CurrentID())
	}

	// Attack2 anim: 8 frames over 700ms => ~87.5ms per frame.
	// Advance to frame 3 = 3 * 87.5ms = 262.5ms.
	e.Current.Update(263 * time.Millisecond)
	e.FSM.Handle(e, 16*time.Millisecond)
	if f := e.CurrentFrame(); f != 3 {
		t.Fatalf("expected frame 3, got %d", f)
	}
	wantVX := float64(e.Facing) * -60
	if e.VX != wantVX {
		t.Errorf("frame %d: want VX=%v, got %v", e.CurrentFrame(), wantVX, e.VX)
	}

	// Advance to frame 6 (past FrameEnd=5). 6 * 87.5ms = 525ms total.
	e.Current.Update(262 * time.Millisecond)
	e.FSM.Handle(e, 16*time.Millisecond)
	if f := e.CurrentFrame(); f != 6 {
		t.Fatalf("expected frame 6, got %d", f)
	}
	if e.VX != 0 {
		t.Errorf("frame %d (past window): want VX=0, got %v", e.CurrentFrame(), e.VX)
	}
}

func TestSlimeAttack2BackstepFacingLeftReversesDirection(t *testing.T) {
	e := newTestEnemySlime()
	e.Facing = -1
	e.Grounded = true
	e.FSM.Transition(e, StateAttack2)
	e.Current.Update(263 * time.Millisecond)
	e.FSM.Handle(e, 16*time.Millisecond)
	// facing=-1, motion.VX=-60, so e.VX = -1 * -60 = +60 (slime slides right while facing left = retreat)
	if e.VX != 60 {
		t.Errorf("want VX=60 (retreat while facing left), got %v", e.VX)
	}
}

func TestOrcAttack2NoMotionKeepsVXZero(t *testing.T) {
	e := newTestEnemyOrc()
	e.Facing = 1
	e.Grounded = true
	e.FSM.Transition(e, StateAttack2)
	e.Current.Update(300 * time.Millisecond)
	e.FSM.Handle(e, 16*time.Millisecond)
	if e.VX != 0 {
		t.Errorf("orc has no motion configured; VX should stay 0, got %v", e.VX)
	}
}
