package player

import "claude-pixel/internal/combat"

func (p *Player) Pos() (float64, float64) { return p.X, p.Y }

func (p *Player) FacingDir() int { return p.Facing }

func (p *Player) CurrentAnimID() string { return p.CurrentAnim }

func (p *Player) CurrentFrame() int {
	if p.Current == nil {
		return 0
	}
	return p.Current.FrameIndex()
}

func (p *Player) Body() combat.Box { return p.Boxes["body"] }

func (p *Player) ActiveHits() []combat.Box {
	switch p.CurrentAnim {
	case "soldier_attack":
		b := p.Boxes["attack"]
		if b.Active(p.CurrentFrame()) {
			return []combat.Box{b}
		}
	case "soldier_attack2":
		b := p.Boxes["attack2"]
		if b.Active(p.CurrentFrame()) {
			return []combat.Box{b}
		}
	}
	return nil
}

func (p *Player) IsInvulnerable() bool {
	return p.HitFlag || p.FSM.CurrentID() == StateDeath
}

func (p *Player) Alive() bool {
	return p.Lives > 0 && p.FSM.CurrentID() != StateDeath
}

func (p *Player) AlreadyHit(t combat.Fighter) bool {
	if p.HitSet == nil {
		return false
	}
	return p.HitSet[t]
}

func (p *Player) MarkHit(t combat.Fighter) {
	if p.HitSet == nil {
		p.HitSet = map[combat.Fighter]bool{}
	}
	p.HitSet[t] = true
}
