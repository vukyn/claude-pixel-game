package combat

import "strings"

// Resolve checks each attacker's active hits against each victim's body.
// Emits one HitEvent per (attacker, victim) first-time overlap.
func Resolve(attackers, victims []Fighter) []HitEvent {
	var out []HitEvent
	for _, a := range attackers {
		if !a.Alive() || a.IsInvulnerable() {
			continue
		}
		hits := a.ActiveHits()
		if len(hits) == 0 {
			continue
		}
		kind := attackKindFromAnim(a.CurrentAnimID())
		if kind == "" {
			continue
		}

		ax, ay := a.Pos()
		for _, v := range victims {
			if v == a || !v.Alive() || v.IsInvulnerable() {
				continue
			}
			if a.AlreadyHit(v) {
				continue
			}
			vx, vy := v.Pos()
			vb := worldRect(vx, vy, v.FacingDir(), v.Body())
			for _, h := range hits {
				ab := worldRect(ax, ay, a.FacingDir(), h)
				if overlap(ab, vb) {
					out = append(out, HitEvent{Attacker: a, Victim: v, AttackKind: kind})
					a.MarkHit(v)
					break
				}
			}
		}
	}
	return out
}

type rect struct {
	MinX, MinY, MaxX, MaxY float64
}

func worldRect(anchorX, anchorY float64, facing int, b Box) rect {
	var minX float64
	if facing >= 0 {
		minX = anchorX + float64(b.OffsetX)
	} else {
		minX = anchorX - float64(b.OffsetX) - float64(b.W)
	}
	minY := anchorY + float64(b.OffsetY)
	return rect{
		MinX: minX,
		MinY: minY,
		MaxX: minX + float64(b.W),
		MaxY: minY + float64(b.H),
	}
}

func overlap(a, b rect) bool {
	if a.MaxX <= b.MinX || b.MaxX <= a.MinX {
		return false
	}
	if a.MaxY <= b.MinY || b.MaxY <= a.MinY {
		return false
	}
	return true
}

func attackKindFromAnim(id string) string {
	switch {
	case id == "attack2" || strings.HasSuffix(id, "_attack2"):
		return "attack2"
	case id == "attack" || strings.HasSuffix(id, "_attack"):
		return "attack"
	}
	return ""
}
