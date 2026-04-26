package enemy

import (
	"testing"

	"claude-pixel/internal/anim"
)

func TestLoadBehaviorResolvesStates(t *testing.T) {
	lib := map[string]*anim.Animation{
		"orc_idle": {}, "orc_run": {}, "orc_attack": {}, "orc_attack2": {},
		"orc_hurt": {}, "orc_death": {},
	}
	states, initial, err := LoadBehavior("../../assets/behaviors/orc.json", "orc_", lib)
	if err != nil {
		t.Fatalf("LoadBehavior: %v", err)
	}
	if initial != "fall" {
		t.Fatalf("initial = %q, want fall", initial)
	}
	for _, id := range []string{"fall", "run", "attack", "attack2", "hurt", "death"} {
		if _, ok := states[id]; !ok {
			t.Fatalf("missing state %q", id)
		}
	}
	if states["run"].BT == nil {
		t.Fatal("run state missing BT")
	}
}
