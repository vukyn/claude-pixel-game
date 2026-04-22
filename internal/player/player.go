package player

import (
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/world"
)

type Player struct {
	X, Y       float64
	VX, VY     float64
	Facing     int
	Grounded   bool
	HasAirDash bool
	DashTimer  time.Duration
	FSM        *FSM
	Physics    *Physics
	Anims      map[string]*anim.Animation
	Current    *anim.Animation
}

type Config struct {
	StartX, StartY float64
	Physics        *Physics
	Anims          map[string]*anim.Animation
}

func (p *Player) PlayAnim(id string) {
	a, ok := p.Anims[id]
	if !ok {
		return
	}
	a.Reset()
	p.Current = a
}

func (p *Player) ApplyPhysics(w *world.World, dt time.Duration) {
	dtS := dt.Seconds()

	if p.FSM != nil && p.FSM.CurrentID() == StateDash {
		p.X += float64(p.Facing) * p.Physics.DashSpeed * dtS
	} else {
		p.VY += p.Physics.Gravity * dtS
		if p.VY > p.Physics.MaxFallSpeed {
			p.VY = p.Physics.MaxFallSpeed
		}
		p.X += p.VX * dtS
		p.Y += p.VY * dtS
	}

	if p.Y >= w.GroundY {
		p.Y = w.GroundY
		p.VY = 0
		p.Grounded = true
		p.HasAirDash = true
	} else {
		p.Grounded = false
	}
}

func New(cfg Config) *Player {
	p := &Player{
		X:       cfg.StartX,
		Y:       cfg.StartY,
		Facing:  1,
		Physics: cfg.Physics,
		Anims:   cfg.Anims,
	}
	p.FSM = NewFSM(StateIdle)
	p.FSM.Register(&idleState{})
	p.FSM.Register(&runState{})
	p.FSM.Register(&jumpState{})
	p.FSM.Register(&fallState{})
	p.FSM.Register(&dashState{})
	p.FSM.Register(&attackState{})
	p.FSM.Register(&attack2State{})
	p.FSM.Start(p)
	return p
}
