package behavior

import "testing"

func TestActionNodeCallsRegistry(t *testing.T) {
	e := &stubEnemy{facing: 1}
	ctx := newCtx(e)
	n := &Action{Name: "flip_facing"}
	if got := n.Tick(ctx); got != StatusSuccess {
		t.Fatalf("Tick = %v", got)
	}
	if e.facing != -1 {
		t.Fatalf("facing = %d", e.facing)
	}
}

func TestActionNodePropagatesArgs(t *testing.T) {
	ctx := newCtx(&stubEnemy{})
	n := &Action{Name: "goto", Args: map[string]any{"state": "attack"}}
	n.Tick(ctx)
	if ctx.PendingGoto != "attack" {
		t.Fatalf("PendingGoto = %q", ctx.PendingGoto)
	}
}

func TestConditionNodeTrueIsSuccess(t *testing.T) {
	e := &stubEnemy{grounded: true}
	n := &Condition{Name: "grounded"}
	if got := n.Tick(newCtx(e)); got != StatusSuccess {
		t.Fatalf("Tick = %v, want success", got)
	}
}

func TestConditionNodeFalseIsFailure(t *testing.T) {
	e := &stubEnemy{grounded: false}
	n := &Condition{Name: "grounded"}
	if got := n.Tick(newCtx(e)); got != StatusFailure {
		t.Fatalf("Tick = %v, want failure", got)
	}
}
