package debug

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigValid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "debug.json")
	if err := os.WriteFile(p, []byte(`{"sections":[{"title":"A","fields":["state","x","y"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sections) != 1 || cfg.Sections[0].Title != "A" {
		t.Fatalf("bad cfg: %+v", cfg)
	}
}

func TestLoadConfigRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "debug.json")
	if err := os.WriteFile(p, []byte(`{"sections":[{"title":"A","fields":["not_a_real_field"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error on unknown field")
	}
}
