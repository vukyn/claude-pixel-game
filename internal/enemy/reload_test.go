package enemy

import (
	"os"
	"path/filepath"
	"testing"

	"claude-pixel/internal/anim"
)

func TestReloadBehaviorSwapsStates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "orc.json")
	os.WriteFile(p, []byte(`{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":true,
         "bt":{"type":"action","name":"goto","args":{"state":"attack"}}},
        {"id":"attack","anim":"attack","decision":false,"exit_on":"anim_done","next":"run"}
      ]}`), 0o600)

	lib := map[string]*anim.Animation{"orc_run": {}, "orc_attack": {}}
	k := &Kind{
		Name: "orc", AnimPrefix: "orc",
		BehaviorPath: p,
	}

	if err := ReloadBehavior(k, lib); err != nil {
		t.Fatalf("ReloadBehavior: %v", err)
	}
	if len(k.States) != 2 {
		t.Fatalf("states = %d", len(k.States))
	}

	// Rewrite file with a different shape, reload, expect updated count.
	os.WriteFile(p, []byte(`{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":true,
         "bt":{"type":"action","name":"stop"}}
      ]}`), 0o600)
	if err := ReloadBehavior(k, lib); err != nil {
		t.Fatalf("ReloadBehavior (2): %v", err)
	}
	if len(k.States) != 1 {
		t.Fatalf("states after reload = %d", len(k.States))
	}
}

func TestReloadBehaviorBadFileKeepsOldState(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "orc.json")
	os.WriteFile(p, []byte(`{
      "kind":"orc",
      "states":[{"id":"run","anim":"run","decision":true,
                "bt":{"type":"action","name":"stop"}}]}`), 0o600)
	lib := map[string]*anim.Animation{"orc_run": {}}
	k := &Kind{Name: "orc", AnimPrefix: "orc", BehaviorPath: p}
	if err := ReloadBehavior(k, lib); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	prev := k.States

	// Corrupt the file.
	os.WriteFile(p, []byte(`{ not json`), 0o600)
	if err := ReloadBehavior(k, lib); err == nil {
		t.Fatal("expected error from malformed JSON")
	}
	if len(k.States) != len(prev) {
		t.Fatalf("states replaced despite error: now %d", len(k.States))
	}
}
