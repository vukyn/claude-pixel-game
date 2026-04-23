package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
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

// OrcBoxes filters HitboxSpec list down to orc boxes keyed by kind ("body","attack","attack2").
// Offsets and dims are multiplied by `scale` so boxes match the rendered sprite scale.
func OrcBoxes(specs []combat.HitboxSpec, scale int) (map[string]combat.Box, error) {
	out := make(map[string]combat.Box, 3)
	for _, s := range specs {
		if s.Owner != "orc" {
			continue
		}
		out[s.Kind] = s.ToBox().Scale(scale)
	}
	if _, ok := out["body"]; !ok {
		return nil, fmt.Errorf("orc hitboxes: missing body")
	}
	return out, nil
}
