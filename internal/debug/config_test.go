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

func TestAllowSpawn(t *testing.T) {
	cases := []struct {
		name    string
		filter  []string
		kind    string
		allowed bool
	}{
		{"empty allows all", nil, "orc", true},
		{"all keyword", []string{"all"}, "slime", true},
		{"specific match", []string{"slime"}, "slime", true},
		{"specific miss", []string{"slime"}, "orc", false},
		{"multi match", []string{"orc", "slime"}, "orc", true},
		{"multi miss", []string{"orc"}, "goblin", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{SpawnEnemies: tc.filter}
			if got := c.AllowSpawn(tc.kind); got != tc.allowed {
				t.Fatalf("AllowSpawn(%q) filter=%v = %v, want %v", tc.kind, tc.filter, got, tc.allowed)
			}
		})
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
