package enemy

import (
	"context"
	"testing"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/config"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
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

func TestLoadTuningForPoints(t *testing.T) {
	cfg := &config.Config{DBPath: ":memory:"}
	db, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	repo := storage.NewRepository(db, player.TuningMapper{})
	ctx := context.Background()

	seed := func(key string, value float64) {
		t.Helper()
		if err := repo.Upsert(ctx, player.TuningParam{Key: key, Value: value}); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	seed("orc_max_lives", 2)
	seed("orc_run_speed", 80)
	seed("orc_intent_tick_s", 1.5)
	seed("orc_hurt_bounce_vx", 120)
	seed("orc_hurt_bounce_vy", -200)
	seed("orc_foot_padding", 4)
	seed("orc_points", 10)

	tun, err := LoadTuningFor(repo, "orc")
	if err != nil {
		t.Fatalf("LoadTuningFor: %v", err)
	}
	if tun.Points != 10 {
		t.Errorf("Points: got %d, want 10", tun.Points)
	}
}
