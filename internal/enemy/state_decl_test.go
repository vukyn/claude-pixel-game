package enemy

import (
	"testing"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/behavior"
)

func TestConvertStatesResolvesAnims(t *testing.T) {
	anims := map[string]*anim.Animation{
		"run":    {},
		"attack": {},
	}
	bStates := []behavior.State{
		{ID: "run", Anim: "run", Decision: true, BT: &behavior.Tree{}},
		{ID: "attack", Anim: "attack", Decision: false, ExitOn: "anim_done", Next: "run"},
	}
	decls, err := ConvertStates(bStates, anims)
	if err != nil {
		t.Fatalf("ConvertStates: %v", err)
	}
	if decls["run"].Anim != anims["run"] {
		t.Fatalf("run anim not resolved")
	}
	if decls["attack"].Next != "run" {
		t.Fatalf("attack next = %q", decls["attack"].Next)
	}
}

func TestConvertStatesMissingAnimError(t *testing.T) {
	bStates := []behavior.State{
		{ID: "run", Anim: "run", Decision: true, BT: &behavior.Tree{}},
	}
	_, err := ConvertStates(bStates, map[string]*anim.Animation{})
	if err == nil {
		t.Fatal("expected error for missing anim")
	}
}
