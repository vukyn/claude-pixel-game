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
	Kind           *Kind
	RNG            *rand.Rand
}

type Enemy struct {
	X, Y, VX, VY float64
	Facing        int
	Grounded      bool
	Lives         int
	Physics       *player.Physics
	Kind          *Kind
	Current       *anim.Animation
	CurrentAnim   string
	CurrentState  string
	BranchTag     string
	HitSet        map[combat.Fighter]bool
	Dead          bool
	OnHitPending  bool
	states        map[string]*StateDecl
	rng           *rand.Rand
}

func New(cfg Config) *Enemy {
	e := &Enemy{
		X:       cfg.StartX,
		Y:       cfg.StartY,
		Facing:  1,
		Lives:   int(cfg.Kind.Tuning.MaxLives),
		Physics: cfg.Physics,
		Kind:    cfg.Kind,
		HitSet:  map[combat.Fighter]bool{},
		rng:     cfg.RNG,
	}
	e.states = CloneStates(cfg.Kind.States)
	e.CurrentState = cfg.Kind.InitialState
	if st := e.states[e.CurrentState]; st != nil && st.Anim != nil {
		st.Anim.Reset()
		e.Current = st.Anim
		e.CurrentAnim = st.AnimKey
	}
	return e
}

func (e *Enemy) PlayAnim(id string) {
	a, ok := e.Kind.Anims[id]
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

// OnHit is called by combat resolution. Decrements lives, applies knockback,
// marks a pending Hurt transition. The FSM driver picks up OnHitPending on
// the next Tick and bypasses the BT. When lives hit 0, the hurt transition
// is skipped so the driver can route directly to death on the next Tick.
func (e *Enemy) OnHit(attackerX float64) {
	e.Lives--
	if e.Lives <= 0 {
		return
	}
	dir := 1.0
	if attackerX > e.X {
		dir = -1.0
	}
	e.VX = dir * e.Kind.Tuning.HurtBounceVX
	e.VY = e.Kind.Tuning.HurtBounceVY
	e.Grounded = false
	e.OnHitPending = true
}
