package enemy

import (
	"math/rand"
	"testing"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
)

func newTestEnemy() *Enemy {
	stub := func(id string, frames, durMs int, loop bool) *anim.Animation {
		spec := &anim.AnimationSpec{ID: id, FrameCount: frames, DurationMs: durMs, Loop: loop}
		return anim.NewAnimation(spec, nil)
	}
	anims := map[string]*anim.Animation{
		"orc_idle":    stub("orc_idle", 6, 900, true),
		"orc_run":     stub("orc_run", 8, 700, true),
		"orc_attack":  stub("orc_attack", 6, 600, false),
		"orc_attack2": stub("orc_attack2", 6, 700, false),
		"orc_hurt":    stub("orc_hurt", 4, 400, false),
		"orc_death":   stub("orc_death", 4, 500, false),
	}
	boxes := map[string]combat.Box{
		"body":    {OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1},
		"attack":  {OffsetX: 25, OffsetY: -70, W: 60, H: 60, FrameStart: 2, FrameEnd: 3},
		"attack2": {OffsetX: 25, OffsetY: -70, W: 70, H: 60, FrameStart: 3, FrameEnd: 4},
	}
	ph := &player.Physics{Gravity: 1800, MaxFallSpeed: 900}
	tn := &Tuning{MaxLives: 2, RunSpeed: 80, IntentTickS: 2, HurtBounceVX: 120, HurtBounceVY: -180}
	return New(Config{
		StartX: 400, StartY: -100,
		Physics: ph, Tuning: tn,
		Anims: anims, Boxes: boxes,
		RNG: rand.New(rand.NewSource(1)),
	})
}

func TestEnemyStartsInFall(t *testing.T) {
	e := newTestEnemy()
	if e.FSM.CurrentID() != StateFall {
		t.Errorf("want fall, got %q", e.FSM.CurrentID())
	}
}

func TestEnemyFallToRunOnGrounded(t *testing.T) {
	e := newTestEnemy()
	e.Grounded = true
	e.FSM.Handle(e, 16*time.Millisecond)
	if e.FSM.CurrentID() != StateRun {
		t.Errorf("want run, got %q", e.FSM.CurrentID())
	}
}

func TestEnemyHurtOnDamage(t *testing.T) {
	e := newTestEnemy()
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
	e := newTestEnemy()
	e.Grounded = true
	e.FSM.Handle(e, 16*time.Millisecond)
	e.Lives = 1
	e.OnHit(e.X + 10)
	if e.FSM.CurrentID() != StateDeath {
		t.Errorf("want death, got %q", e.FSM.CurrentID())
	}
}
