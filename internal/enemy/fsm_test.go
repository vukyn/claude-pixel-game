package enemy

import (
	"math/rand"
	"testing"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/behavior"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
)

func stubAnim(id string, frames, durMs int, loop bool) *anim.Animation {
	spec := &anim.AnimationSpec{ID: id, FrameCount: frames, DurationMs: durMs, Loop: loop}
	return anim.NewAnimation(spec, nil)
}

func makeTestKind() *Kind {
	idle := stubAnim("t_idle", 2, 200, true)
	run := stubAnim("t_run", 2, 200, true)
	attack := stubAnim("t_attack", 2, 200, false)
	hurt := stubAnim("t_hurt", 2, 200, false)
	death := stubAnim("t_death", 2, 200, false)
	states := map[string]*StateDecl{
		"fall": {ID: "fall", Anim: idle, AnimKey: "idle", Decision: false, ExitOn: "grounded", Next: "run", OnExitActions: []string{"randomize_facing"}},
		"run": {ID: "run", Anim: run, AnimKey: "run", Decision: true,
			BT: &behavior.Tree{Root: &behavior.Action{Name: "goto", Args: map[string]any{"state": "attack"}}}},
		"attack": {ID: "attack", Anim: attack, AnimKey: "attack", Decision: false, ExitOn: "anim_done", Next: "run"},
		"hurt":   {ID: "hurt", Anim: hurt, AnimKey: "hurt", Decision: false, ExitOn: "anim_done_and_grounded", Next: "run"},
		"death":  {ID: "death", Anim: death, AnimKey: "death", Decision: false, ExitOn: "anim_done", Next: "__dead"},
	}
	return &Kind{
		Name: "test", AnimPrefix: "t", FrameW: 10, FrameH: 10,
		Tuning: &Tuning{MaxLives: 2, HurtBounceVX: 100, HurtBounceVY: -50, FootPadding: 0, Points: 0},
		Boxes:  map[string]combat.Box{"body": {W: 10, H: 10}},
		Anims: map[string]*anim.Animation{
			"idle": idle, "run": run, "attack": attack, "hurt": hurt, "death": death,
		},
		States:       states,
		InitialState: "fall",
	}
}

func newTestEnemy() *Enemy {
	return New(Config{
		StartX: 0, StartY: 0,
		Physics: &player.Physics{Gravity: 0, MaxFallSpeed: 0},
		Kind:    makeTestKind(),
		RNG:     rand.New(rand.NewSource(1)),
	})
}

func TestEnemyStartsInFall(t *testing.T) {
	e := newTestEnemy()
	if e.CurrentState != "fall" {
		t.Fatalf("initial = %q", e.CurrentState)
	}
}

func TestFallTransitionsToRunOnGrounded(t *testing.T) {
	e := newTestEnemy()
	e.Grounded = true
	e.Tick(16 * time.Millisecond)
	if e.CurrentState != "run" {
		t.Fatalf("after grounded, state = %q", e.CurrentState)
	}
}

func TestDecisionStateRunsBT(t *testing.T) {
	e := newTestEnemy()
	e.CurrentState = "run"
	e.Grounded = true
	e.Tick(16 * time.Millisecond)
	if e.CurrentState != "attack" {
		t.Fatalf("BT goto didn't fire: state = %q", e.CurrentState)
	}
}

func TestAnimDoneTransitionsToNext(t *testing.T) {
	e := newTestEnemy()
	e.CurrentState = "attack"
	e.Current = e.states["attack"].Anim
	e.CurrentAnim = "attack"
	e.Grounded = true
	for i := 0; i < 20 && e.CurrentState == "attack"; i++ {
		e.Tick(60 * time.Millisecond)
	}
	if e.CurrentState != "run" {
		t.Fatalf("after anim_done: state = %q", e.CurrentState)
	}
}

func TestOnHitGoesToHurtBypassingBT(t *testing.T) {
	e := newTestEnemy()
	e.CurrentState = "run"
	e.Current = e.states["run"].Anim
	e.Grounded = true
	e.OnHit(10)
	e.Tick(16 * time.Millisecond)
	if e.CurrentState != "hurt" {
		t.Fatalf("state = %q, want hurt", e.CurrentState)
	}
}

func TestLivesZeroGoesToDeath(t *testing.T) {
	e := newTestEnemy()
	e.Lives = 0
	e.CurrentState = "run"
	e.Grounded = true
	e.Tick(16 * time.Millisecond)
	if e.CurrentState != "death" {
		t.Fatalf("state = %q, want death", e.CurrentState)
	}
}

func TestDeathAnimDoneSetsDead(t *testing.T) {
	e := newTestEnemy()
	e.CurrentState = "death"
	e.Current = e.states["death"].Anim
	e.Grounded = true
	for i := 0; i < 20 && !e.Dead; i++ {
		e.Tick(60 * time.Millisecond)
	}
	if !e.Dead {
		t.Fatal("Dead still false after death anim")
	}
}
