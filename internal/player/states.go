package player

import (
	"time"

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

type idleState struct{}

func (idleState) ID() StateID     { return StateIdle }
func (idleState) Enter(p *Player) { p.PlayAnim("idle"); p.VX = 0 }
func (idleState) Exit(p *Player)  {}
func (idleState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if !p.Grounded {
		return StateFall
	}
	if in.DashPressed {
		return StateDash
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
func (runState) Enter(p *Player) { p.PlayAnim("run") }
func (runState) Exit(p *Player)  {}
func (runState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * p.Physics.RunSpeed

	if !p.Grounded {
		return StateFall
	}
	if in.DashPressed {
		return StateDash
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
func (jumpState) Enter(p *Player) { p.PlayAnim("jump"); p.VY = p.Physics.JumpVelocity }
func (jumpState) Exit(p *Player)  {}
func (jumpState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * p.Physics.RunSpeed * p.Physics.AirControl

	if in.DashPressed && p.HasAirDash {
		return StateDash
	}
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
func (fallState) Enter(p *Player) { p.PlayAnim("fall") }
func (fallState) Exit(p *Player)  {}
func (fallState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * p.Physics.RunSpeed * p.Physics.AirControl

	if in.DashPressed && p.HasAirDash {
		return StateDash
	}
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

type dashState struct{}

func (dashState) ID() StateID { return StateDash }
func (dashState) Enter(p *Player) {
	p.PlayAnim("dash")
	p.DashTimer = 0
	p.VX = 0
	p.VY = 0
	if !p.Grounded {
		p.HasAirDash = false
	}
}
func (dashState) Exit(p *Player) {}
func (dashState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	p.DashTimer += dt
	if p.DashTimer >= p.Physics.DashDuration {
		if p.Grounded {
			return StateIdle
		}
		return StateFall
	}
	return StateDash
}

type attackState struct{}

func (attackState) ID() StateID { return StateAttack }
func (attackState) Enter(p *Player) {
	p.PlayAnim("attack")
	if p.Grounded {
		p.VX = 0
	}
}
func (attackState) Exit(p *Player) {}
func (attackState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if in.DashPressed {
		return StateDash
	}
	if in.JumpPressed && p.Grounded {
		return StateJump
	}
	if p.Current.Done() {
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
	p.PlayAnim("attack2")
	if p.Grounded {
		p.VX = 0
	}
}
func (attack2State) Exit(p *Player) {}
func (attack2State) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if in.DashPressed {
		return StateDash
	}
	if in.JumpPressed && p.Grounded {
		return StateJump
	}
	if p.Current.Done() {
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
