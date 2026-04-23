package player

import (
	"time"

	"claude-pixel/internal/combat"
	"claude-pixel/internal/input"
)

func moveDir(in input.Intent) int {
	switch {
	case in.Right && !in.Left:
		return 1
	case in.Left && !in.Right:
		return -1
	default:
		return 0
	}
}

func groundSpeed(p *Player, in input.Intent) float64 {
	if in.SprintHeld && p.Stamina != nil && p.Stamina.CanSprint() {
		return p.Physics.SprintSpeed
	}
	return p.Physics.RunSpeed
}

func airSpeed(p *Player, in input.Intent) float64 {
	return groundSpeed(p, in) * p.Physics.AirControl
}

type idleState struct{}

func (idleState) ID() StateID     { return StateIdle }
func (idleState) Enter(p *Player) { p.PlayAnim("soldier_idle"); p.VX = 0 }
func (idleState) Exit(p *Player)  {}
func (idleState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if !p.Grounded {
		return StateFall
	}
	if in.AttackPressed {
		return StateAttack
	}
	if in.Attack2Pressed {
		return StateAttack2
	}
	if in.JumpPressed {
		return StateJump
	}
	if moveDir(in) != 0 {
		return StateRun
	}
	return StateIdle
}

type runState struct{}

func (runState) ID() StateID     { return StateRun }
func (runState) Enter(p *Player) { p.PlayAnim("soldier_run") }
func (runState) Exit(p *Player)  {}
func (runState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * groundSpeed(p, in)

	if !p.Grounded {
		return StateFall
	}
	if in.AttackPressed {
		return StateAttack
	}
	if in.Attack2Pressed {
		return StateAttack2
	}
	if in.JumpPressed {
		return StateJump
	}
	if d == 0 {
		return StateIdle
	}
	return StateRun
}

type jumpState struct{}

func (jumpState) ID() StateID     { return StateJump }
func (jumpState) Enter(p *Player) { p.PlayAnim("soldier_jump"); p.VY = p.Physics.JumpVelocity }
func (jumpState) Exit(p *Player)  {}
func (jumpState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * airSpeed(p, in)

	if in.AttackPressed {
		return StateAttack
	}
	if in.Attack2Pressed {
		return StateAttack2
	}
	if p.VY >= 0 {
		return StateFall
	}
	return StateJump
}

type fallState struct{}

func (fallState) ID() StateID     { return StateFall }
func (fallState) Enter(p *Player) { p.PlayAnim("soldier_fall") }
func (fallState) Exit(p *Player)  {}
func (fallState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * airSpeed(p, in)

	if in.AttackPressed {
		return StateAttack
	}
	if in.Attack2Pressed {
		return StateAttack2
	}
	if p.Grounded {
		if d == 0 {
			return StateIdle
		}
		return StateRun
	}
	return StateFall
}

type attackState struct{}

func (attackState) ID() StateID { return StateAttack }
func (attackState) Enter(p *Player) {
	p.PlayAnim("soldier_attack")
	if p.Grounded {
		p.VX = 0
	}
	p.HitSet = map[combat.Fighter]bool{}
}
func (attackState) Exit(p *Player) {}
func (attackState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if in.JumpPressed && p.Grounded {
		return StateJump
	}
	if p.Current != nil && p.Current.Done() {
		if p.Grounded {
			if moveDir(in) == 0 {
				return StateIdle
			}
			return StateRun
		}
		return StateFall
	}
	return StateAttack
}

type attack2State struct{}

func (attack2State) ID() StateID { return StateAttack2 }
func (attack2State) Enter(p *Player) {
	p.PlayAnim("soldier_attack2")
	if p.Grounded {
		p.VX = 0
	}
	p.HitSet = map[combat.Fighter]bool{}
}
func (attack2State) Exit(p *Player) {}
func (attack2State) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if in.JumpPressed && p.Grounded {
		return StateJump
	}
	if p.Current != nil && p.Current.Done() {
		if p.Grounded {
			if moveDir(in) == 0 {
				return StateIdle
			}
			return StateRun
		}
		return StateFall
	}
	return StateAttack2
}

type hitState struct{}

func (hitState) ID() StateID { return StateHit }
func (hitState) Enter(p *Player) {
	p.PlayAnim("soldier_hit")
	p.HitFlag = true
}
func (hitState) Exit(p *Player) {
	p.HitFlag = false
}
func (hitState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	_ = in
	if p.Grounded {
		return StateIdle
	}
	return StateHit
}

type deathState struct{}

func (deathState) ID() StateID { return StateDeath }
func (deathState) Enter(p *Player) {
	p.PlayAnim("soldier_death")
	p.VX = 0
}
func (deathState) Exit(p *Player) {}
func (deathState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	_ = in
	return StateDeath
}
