package player

import (
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/world"
)

type Player struct {
	X, Y     float64
	VX, VY   float64
	Facing   int
	Grounded bool
	FSM      *FSM
	Physics  *Physics
	Anims    map[string]*anim.Animation
	Current  *anim.Animation

	Lives       int
	HitFlag     bool
	Boxes       map[string]combat.Box
	HitSet      map[combat.Fighter]bool
	CurrentAnim string
}

type Config struct {
	StartX, StartY float64
	Physics        *Physics
	Anims          map[string]*anim.Animation
	Boxes          map[string]combat.Box
	StartLives     int
}

func (p *Player) PlayAnim(id string) {
	a, ok := p.Anims[id]
	if !ok {
		return
	}
	a.Reset()
	p.Current = a
	p.CurrentAnim = id
}

func (p *Player) ApplyPhysics(w *world.World, dt time.Duration) {
	dtS := dt.Seconds()

	p.VY += p.Physics.Gravity * dtS
	if p.VY > p.Physics.MaxFallSpeed {
		p.VY = p.Physics.MaxFallSpeed
	}
	p.X += p.VX * dtS
	p.Y += p.VY * dtS

	if p.Y >= w.GroundY {
		p.Y = w.GroundY
		p.VY = 0
		p.Grounded = true
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
		Boxes:   cfg.Boxes,
		Lives:   cfg.StartLives,
		HitSet:  map[combat.Fighter]bool{},
	}
	p.FSM = NewFSM(StateIdle)
	p.FSM.Register(&idleState{})
	p.FSM.Register(&runState{})
	p.FSM.Register(&jumpState{})
	p.FSM.Register(&fallState{})
	p.FSM.Register(&attackState{})
	p.FSM.Register(&attack2State{})
	p.FSM.Register(&hitState{})
	p.FSM.Register(&deathState{})
	p.FSM.Start(p)
	return p
}

func (p *Player) OnHit(knockbackVX, knockbackVY, attackerX float64) {
	p.Lives--
	if p.Lives <= 0 {
		p.FSM.Transition(p, StateDeath)
		return
	}
	dir := 1.0
	if attackerX > p.X {
		dir = -1.0
	}
	p.VX = dir * knockbackVX
	p.VY = knockbackVY
	p.Grounded = false
	p.FSM.Transition(p, StateHit)
}
