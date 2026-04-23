package combat

import "fmt"

type Tuning struct {
	SoldierKnockbackVX float64
	SoldierKnockbackVY float64
	SoldierMaxLives    int
	SoldierFootPadding int
}

// LoadTuning extracts the combat tuning values from a tuning map.
func LoadTuning(tuning map[string]float64) (*Tuning, error) {
	get := func(k string) (float64, error) {
		v, ok := tuning[k]
		if !ok {
			return 0, fmt.Errorf("missing tuning key %q", k)
		}
		return v, nil
	}
	t := &Tuning{}
	var e error
	if t.SoldierKnockbackVX, e = get("soldier_knockback_vx"); e != nil {
		return nil, e
	}
	if t.SoldierKnockbackVY, e = get("soldier_knockback_vy"); e != nil {
		return nil, e
	}
	var maxL float64
	if maxL, e = get("soldier_max_lives"); e != nil {
		return nil, e
	}
	t.SoldierMaxLives = int(maxL)
	var pad float64
	if pad, e = get("soldier_foot_padding"); e != nil {
		return nil, e
	}
	t.SoldierFootPadding = int(pad)
	return t, nil
}

// SoldierBoxes filters HitboxSpec list down to soldier boxes keyed by kind.
// Offsets and dims are multiplied by `scale` so boxes match the rendered sprite scale.
func SoldierBoxes(specs []HitboxSpec, scale int) (map[string]Box, error) {
	out := make(map[string]Box, 3)
	for _, s := range specs {
		if s.Owner != "soldier" {
			continue
		}
		out[s.Kind] = s.ToBox().Scale(scale)
	}
	if _, ok := out["body"]; !ok {
		return nil, fmt.Errorf("soldier hitboxes: missing body")
	}
	return out, nil
}
