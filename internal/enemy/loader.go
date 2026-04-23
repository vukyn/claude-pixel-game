package enemy

import (
	"context"
	"fmt"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/storage"
)

// OrcAnims extracts the 6 orc anims from a loaded library.
func OrcAnims(lib map[string]*anim.Animation) (map[string]*anim.Animation, error) {
	want := []string{"orc_idle", "orc_run", "orc_attack", "orc_attack2", "orc_hurt", "orc_death"}
	out := make(map[string]*anim.Animation, len(want))
	for _, k := range want {
		a, ok := lib[k]
		if !ok {
			return nil, fmt.Errorf("orc anims: missing %q", k)
		}
		out[k] = a
	}
	return out, nil
}

// OrcBoxes loads orc hitboxes keyed by kind ("body","attack","attack2").
func OrcBoxes(repo *storage.Repository[combat.HitboxSpec]) (map[string]combat.Box, error) {
	all, err := repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	out := make(map[string]combat.Box, 3)
	for _, s := range all {
		if s.Owner != "orc" {
			continue
		}
		out[s.Kind] = s.ToBox()
	}
	if _, ok := out["body"]; !ok {
		return nil, fmt.Errorf("orc hitboxes: missing body")
	}
	return out, nil
}
