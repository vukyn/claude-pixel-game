package behavior

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildNodeSelector(t *testing.T) {
	raw := map[string]any{
		"type": "selector",
		"children": []any{
			map[string]any{"type": "action", "name": "flip_facing"},
			map[string]any{"type": "action", "name": "stop"},
		},
	}
	n, err := buildNode(raw)
	if err != nil {
		t.Fatalf("buildNode: %v", err)
	}
	s, ok := n.(*Selector)
	if !ok {
		t.Fatalf("type = %T, want *Selector", n)
	}
	if len(s.Children) != 2 {
		t.Fatalf("children = %d", len(s.Children))
	}
}

func TestBuildNodeChanceTree(t *testing.T) {
	raw := map[string]any{
		"type": "chance",
		"branches": []any{
			map[string]any{"weight": 30.0, "node": map[string]any{"type": "action", "name": "flip_facing"}},
			map[string]any{"weight": 70.0, "node": map[string]any{"type": "action", "name": "stop"}},
		},
	}
	n, err := buildNode(raw)
	if err != nil {
		t.Fatalf("buildNode: %v", err)
	}
	ch, ok := n.(*Chance)
	if !ok {
		t.Fatalf("type = %T", n)
	}
	if len(ch.Branches) != 2 || ch.Branches[0].Weight != 30 {
		t.Fatalf("branches = %+v", ch.Branches)
	}
}

func TestBuildNodeWait(t *testing.T) {
	raw := map[string]any{"type": "wait", "seconds": 2.5}
	n, err := buildNode(raw)
	if err != nil {
		t.Fatalf("buildNode: %v", err)
	}
	w := n.(*Wait)
	if w.Seconds != 2.5 {
		t.Fatalf("seconds = %f", w.Seconds)
	}
}

func TestBuildNodeActionUnknownRejected(t *testing.T) {
	raw := map[string]any{"type": "action", "name": "nope_nada"}
	_, err := buildNode(raw)
	if err == nil || !strings.Contains(err.Error(), "nope_nada") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildNodeConditionUnknownRejected(t *testing.T) {
	raw := map[string]any{"type": "condition", "name": "nope_nada"}
	_, err := buildNode(raw)
	if err == nil || !strings.Contains(err.Error(), "nope_nada") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildNodeUnknownTypeRejected(t *testing.T) {
	raw := map[string]any{"type": "spline"}
	_, err := buildNode(raw)
	if err == nil || !strings.Contains(err.Error(), "spline") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildNodeChanceEmptyBranchesRejected(t *testing.T) {
	raw := map[string]any{"type": "chance", "branches": []any{}}
	_, err := buildNode(raw)
	if err == nil || !strings.Contains(err.Error(), "chance") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildNodeChanceNonPositiveWeightRejected(t *testing.T) {
	raw := map[string]any{
		"type": "chance",
		"branches": []any{
			map[string]any{"weight": 0.0, "node": map[string]any{"type": "action", "name": "stop"}},
		},
	}
	_, err := buildNode(raw)
	if err == nil || !strings.Contains(err.Error(), "weight") {
		t.Fatalf("err = %v", err)
	}
}

// ------ end-to-end LoadFile tests (Task 7) ------

func writeTmp(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "k.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadFileMinimalValid(t *testing.T) {
	p := writeTmp(t, `{
      "kind": "orc",
      "states": [
        { "id":"run","anim":"run","decision":true,
          "bt": { "type":"action","name":"goto","args":{"state":"attack"} } },
        { "id":"attack","anim":"attack","decision":false,"exit_on":"anim_done","next":"run" }
      ]
    }`)
	f, err := LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if f.Kind != "orc" {
		t.Fatalf("kind = %q", f.Kind)
	}
	if len(f.States) != 2 {
		t.Fatalf("states = %d", len(f.States))
	}
	if !f.States[0].Decision || f.States[0].BT == nil {
		t.Fatalf("run state missing BT")
	}
	if f.States[1].Decision || f.States[1].BT != nil {
		t.Fatalf("attack state should be non-decision")
	}
}

func TestLoadFileDuplicateStateIDRejected(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":false,"exit_on":"anim_done","next":"run"},
        {"id":"run","anim":"run","decision":false,"exit_on":"anim_done","next":"run"}
      ]
    }`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "duplicate state id") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileGotoUndeclaredRejected(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":true,
         "bt":{"type":"action","name":"goto","args":{"state":"somewhere"}}}
      ]
    }`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "somewhere") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileNextUndeclaredRejected(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"attack","anim":"attack","decision":false,"exit_on":"anim_done","next":"nope"}
      ]
    }`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileNextDeadAllowed(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"death","anim":"death","decision":false,"exit_on":"anim_done","next":"__dead"}
      ]
    }`)
	if _, err := LoadFile(p); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
}

func TestLoadFileUnknownOnExitActionRejected(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":false,"exit_on":"grounded","next":"__dead",
         "on_exit_actions":["nope_nada"]}
      ]
    }`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "nope_nada") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileDecisionWithoutBTRejected(t *testing.T) {
	p := writeTmp(t, `{"kind":"orc","states":[{"id":"run","anim":"run","decision":true}]}`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "missing bt") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileNonDecisionWithBTRejected(t *testing.T) {
	p := writeTmp(t, `{"kind":"orc","states":[
      {"id":"run","anim":"run","decision":false,"exit_on":"anim_done","next":"__dead",
       "bt":{"type":"action","name":"stop"}}
    ]}`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "must not have bt") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileMissingKindRejected(t *testing.T) {
	p := writeTmp(t, `{"states":[{"id":"run","anim":"run","decision":false,"exit_on":"anim_done","next":"__dead"}]}`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileEmptyStatesRejected(t *testing.T) {
	p := writeTmp(t, `{"kind":"orc","states":[]}`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "states is empty") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileNonexistentPathError(t *testing.T) {
	_, err := LoadFile("/nonexistent/totally-not-there/orc.json")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestBuildNodeChanceFractionalWeightRejected(t *testing.T) {
	raw := map[string]any{
		"type": "chance",
		"branches": []any{
			map[string]any{"weight": 1.5, "node": map[string]any{"type": "action", "name": "stop"}},
		},
	}
	_, err := buildNode(raw)
	if err == nil || !strings.Contains(err.Error(), "integer") {
		t.Fatalf("err = %v", err)
	}
}
