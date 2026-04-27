package adapter_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"claude-pixel/internal/config"
	"claude-pixel/internal/editor/adapter"
	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func newTuningRepo(t *testing.T) *storage.Repository[player.TuningParam] {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := &config.Config{DBPath: dbPath}
	db, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
}

func TestSQLiteTuning_ListWithPrefix(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	rows, err := a.List("orc")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if !strings.HasPrefix(r.Key, "orc_") {
			t.Fatalf("row %q does not match prefix orc_", r.Key)
		}
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one orc_* row from seed")
	}
}

func TestSQLiteTuning_ListNoPrefixReturnsAll(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	rows, err := a.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 10 {
		t.Fatalf("expected many rows, got %d", len(rows))
	}
}

func TestSQLiteTuning_UpdateWithinRange(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	old, err := a.Update("orc_max_lives", 5)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(context.Background(), "orc_max_lives")
	if err != nil {
		t.Fatalf("re-get: %v", err)
	}
	if got.Value != 5 {
		t.Fatalf("value not persisted, got %v", got.Value)
	}
	if old == 5 {
		t.Fatalf("old value should differ from new (was %v)", old)
	}
}

func TestSQLiteTuning_UpdateOutOfRangeRejected(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	_, err := a.Update("orc_max_lives", 9_999_999)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("err should mention out of range, got %v", err)
	}
}

func TestSQLiteTuning_UpdateUnknownKey(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	_, err := a.Update("ghost_key", 1)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

var _ port.TuningStore = (*adapter.SQLiteTuning)(nil)
