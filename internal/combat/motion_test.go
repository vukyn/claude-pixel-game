package combat

import (
	"context"
	"testing"

	"claude-pixel/internal/config"
	"claude-pixel/internal/storage"
)

func TestAttackMotionRepositoryRoundtrip(t *testing.T) {
	cfg := &config.Config{DBPath: ":memory:"}
	db, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	repo := storage.NewRepository(db, AttackMotionMapper{})
	in := AttackMotionSpec{
		ID:         "test_motion",
		Owner:      "slime",
		Kind:       "attack2",
		VX:         -60,
		FrameStart: 3,
		FrameEnd:   5,
	}
	if err := repo.Upsert(context.Background(), in); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	out, err := repo.Get(context.Background(), "test_motion")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: got %+v want %+v", out, in)
	}
}
