package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
)

var kindAnimKeys = []string{"idle", "run", "attack", "attack2", "hurt", "death"}

// AnimsFor picks the 6 animations belonging to `prefix` out of a loaded
// library. The returned map is keyed by unprefixed state name so FSM states
// are owner-agnostic.
func AnimsFor(lib map[string]*anim.Animation, prefix string) (map[string]*anim.Animation, error) {
	out := make(map[string]*anim.Animation, len(kindAnimKeys))
	for _, k := range kindAnimKeys {
		id := prefix + "_" + k
		a, ok := lib[id]
		if !ok {
			return nil, fmt.Errorf("%s anims: missing %q", prefix, id)
		}
		out[k] = a
	}
	return out, nil
}

// BoxesFor filters HitboxSpec list by owner and multiplies offsets/dims by
// scale. Requires a "body" box; "attack"/"attack2" are optional.
func BoxesFor(specs []combat.HitboxSpec, owner string, scale int) (map[string]combat.Box, error) {
	out := make(map[string]combat.Box, 3)
	for _, s := range specs {
		if s.Owner != owner {
			continue
		}
		out[s.Kind] = s.ToBox().Scale(scale)
	}
	if _, ok := out["body"]; !ok {
		return nil, fmt.Errorf("%s hitboxes: missing body", owner)
	}
	return out, nil
}

// MotionsFor filters AttackMotionSpec list by owner and returns a map keyed
// by kind ("attack" | "attack2"). Empty map (not error) if owner has none.
func MotionsFor(specs []combat.AttackMotionSpec, owner string) map[string]AttackMotion {
	out := map[string]AttackMotion{}
	for _, s := range specs {
		if s.Owner != owner {
			continue
		}
		out[s.Kind] = AttackMotion{VX: s.VX, FrameStart: s.FrameStart, FrameEnd: s.FrameEnd}
	}
	return out
}
