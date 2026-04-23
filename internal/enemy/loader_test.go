package enemy

import (
	"testing"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
)

func TestAnimsForReturnsUnprefixedMap(t *testing.T) {
	stub := func(id string) *anim.Animation {
		return anim.NewAnimation(&anim.AnimationSpec{ID: id, FrameCount: 1, DurationMs: 100}, nil)
	}
	lib := map[string]*anim.Animation{
		"orc_idle":     stub("orc_idle"),
		"orc_run":      stub("orc_run"),
		"orc_attack":   stub("orc_attack"),
		"orc_attack2":  stub("orc_attack2"),
		"orc_hurt":     stub("orc_hurt"),
		"orc_death":    stub("orc_death"),
		"soldier_idle": stub("soldier_idle"),
	}
	out, err := AnimsFor(lib, "orc")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	for _, k := range []string{"idle", "run", "attack", "attack2", "hurt", "death"} {
		if _, ok := out[k]; !ok {
			t.Errorf("missing unprefixed key %q", k)
		}
	}
	if _, ok := out["soldier_idle"]; ok {
		t.Errorf("should not leak soldier_idle into orc map")
	}
}

func TestAnimsForErrorsOnMissing(t *testing.T) {
	lib := map[string]*anim.Animation{"orc_idle": nil}
	_, err := AnimsFor(lib, "orc")
	if err == nil {
		t.Errorf("expected error for missing keys")
	}
}

func TestBoxesForFiltersByOwnerAndScales(t *testing.T) {
	specs := []combat.HitboxSpec{
		{ID: "orc_body", Owner: "orc", Kind: "body", Width: 50, Height: 80, FrameStart: -1, FrameEnd: -1},
		{ID: "orc_attack", Owner: "orc", Kind: "attack", Width: 60, Height: 60, FrameStart: 2, FrameEnd: 3},
		{ID: "slime_body", Owner: "slime", Kind: "body", Width: 40, Height: 40, FrameStart: -1, FrameEnd: -1},
	}
	out, err := BoxesFor(specs, "orc", 2)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out["body"].W != 100 || out["body"].H != 160 {
		t.Errorf("body not scaled: %+v", out["body"])
	}
	if _, ok := out["attack"]; !ok {
		t.Errorf("missing attack box")
	}
	if _, ok := out["slime_body"]; ok {
		t.Errorf("slime leaked into orc boxes")
	}
}

func TestBoxesForMissingBody(t *testing.T) {
	specs := []combat.HitboxSpec{
		{ID: "orc_attack", Owner: "orc", Kind: "attack", Width: 60, Height: 60, FrameStart: 2, FrameEnd: 3},
	}
	_, err := BoxesFor(specs, "orc", 1)
	if err == nil {
		t.Errorf("expected missing-body error")
	}
}

func TestMotionsForFiltersByOwner(t *testing.T) {
	specs := []combat.AttackMotionSpec{
		{ID: "slime_attack2_motion", Owner: "slime", Kind: "attack2", VX: -60, FrameStart: 3, FrameEnd: 5},
		{ID: "orc_attack_motion", Owner: "orc", Kind: "attack", VX: 30, FrameStart: 1, FrameEnd: 2},
	}
	out := MotionsFor(specs, "slime")
	if len(out) != 1 {
		t.Fatalf("want 1 motion, got %d", len(out))
	}
	m, ok := out["attack2"]
	if !ok {
		t.Fatalf("missing attack2")
	}
	if m.VX != -60 || m.FrameStart != 3 || m.FrameEnd != 5 {
		t.Errorf("wrong motion: %+v", m)
	}
}

func TestMotionsForEmptyWhenNoMatch(t *testing.T) {
	out := MotionsFor(nil, "orc")
	if len(out) != 0 {
		t.Errorf("want empty, got %d", len(out))
	}
}
