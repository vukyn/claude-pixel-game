package enemy

import (
	"math/rand"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
	"claude-pixel/internal/world"
)

type Config struct {
	StartX, StartY float64
	Physics        *player.Physics
	Tuning         *Tuning
	Anims          map[string]*anim.Animation
	Boxes          map[string]combat.Box
	RNG            *rand.Rand
}

type Enemy struct {
	X, Y, VX, VY float64
	Facing       int
	Grounded     bool
	Lives        int
	RunSpeed     float64
	Physics      *player.Physics
	Tuning       *Tuning
	Anims        map[string]*anim.Animation
	Boxes        map[string]combat.Box
	FSM          *FSM
	Current      *anim.Animation
	CurrentAnim  string
	IntentTimer  float64
	HitSet       map[combat.Fighter]bool
	Dead         bool
	rng          *rand.Rand
}

func New(cfg Config) *Enemy {
	e := &Enemy{
		X:        cfg.StartX,
		Y:        cfg.StartY,
		Facing:   1,
		Lives:    int(cfg.Tuning.MaxLives),
		RunSpeed: cfg.Tuning.RunSpeed,
		Physics:  cfg.Physics,
		Tuning:   cfg.Tuning,
		Anims:    cfg.Anims,
		Boxes:    cfg.Boxes,
		HitSet:   map[combat.Fighter]bool{},
		rng:      cfg.RNG,
	}
	e.FSM = NewFSM(StateFall)
	e.FSM.Register(&fallState{})
	e.FSM.Register(&runState{})
	e.FSM.Register(&attackState{})
	e.FSM.Register(&attack2State{})
	e.FSM.Register(&hurtState{})
	e.FSM.Register(&deathState{})
	e.FSM.Start(e)
	return e
}

func (e *Enemy) PlayAnim(id string) {
	a, ok := e.Anims[id]
	if !ok {
		return
	}
	a.Reset()
	e.Current = a
	e.CurrentAnim = id
}

func (e *Enemy) ApplyPhysics(w *world.World, dt time.Duration) {
	dtS := dt.Seconds()
	e.VY += e.Physics.Gravity * dtS
	if e.VY > e.Physics.MaxFallSpeed {
		e.VY = e.Physics.MaxFallSpeed
	}
	e.X += e.VX * dtS
	e.Y += e.VY * dtS
	if e.Y >= w.GroundY {
		e.Y = w.GroundY
		e.VY = 0
		e.Grounded = true
	} else {
		e.Grounded = false
	}
}

func (e *Enemy) OnHit(attackerX float64) {
	e.Lives--
	if e.Lives <= 0 {
		e.FSM.Transition(e, StateDeath)
		return
	}
	dir := 1.0
	if attackerX > e.X {
		dir = -1.0
	}
	e.VX = dir * e.Tuning.HurtBounceVX
	e.VY = e.Tuning.HurtBounceVY
	e.Grounded = false
	e.FSM.Transition(e, StateHurt)
}
