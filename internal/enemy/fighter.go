package enemy

import "claude-pixel/internal/combat"

func (e *Enemy) Pos() (float64, float64) { return e.X, e.Y }

func (e *Enemy) FacingDir() int { return e.Facing }

func (e *Enemy) CurrentAnimID() string { return e.CurrentAnim }

func (e *Enemy) CurrentFrame() int {
	if e.Current == nil {
		return 0
	}
	return e.Current.FrameIndex()
}

func (e *Enemy) Body() combat.Box { return e.Boxes["body"] }

func (e *Enemy) ActiveHits() []combat.Box {
	switch e.CurrentAnim {
	case "orc_attack":
		box := e.Boxes["attack"]
		if box.Active(e.CurrentFrame()) {
			return []combat.Box{box}
		}
	case "orc_attack2":
		box := e.Boxes["attack2"]
		if box.Active(e.CurrentFrame()) {
			return []combat.Box{box}
		}
	}
	return nil
}

func (e *Enemy) IsInvulnerable() bool {
	id := e.FSM.CurrentID()
	return id == StateHurt || id == StateDeath
}

func (e *Enemy) Alive() bool {
	return !e.Dead && e.FSM.CurrentID() != StateDeath
}

func (e *Enemy) AlreadyHit(t combat.Fighter) bool {
	if e.HitSet == nil {
		return false
	}
	return e.HitSet[t]
}

func (e *Enemy) MarkHit(t combat.Fighter) {
	if e.HitSet == nil {
		e.HitSet = map[combat.Fighter]bool{}
	}
	e.HitSet[t] = true
}
