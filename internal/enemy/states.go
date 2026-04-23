package enemy

import (
	"time"

	"claude-pixel/internal/combat"
)

// applyMotion applies the per-frame horizontal displacement configured for
// an attack kind ("attack" | "attack2"). If no motion is configured, VX is
// left at whatever the Enter handler set (typically 0).
func applyMotion(e *Enemy, kind string) {
	m, ok := e.Kind.Motions[kind]
	if !ok {
		return
	}
	f := e.CurrentFrame()
	if f >= m.FrameStart && f <= m.FrameEnd {
		e.VX = float64(e.Facing) * m.VX
	} else {
		e.VX = 0
	}
}

type fallState struct{}

func (fallState) ID() StateID { return StateFall }
func (fallState) Enter(e *Enemy) {
	e.PlayAnim("idle")
	e.VX = 0
}
func (fallState) Exit(e *Enemy) {}
func (fallState) Update(e *Enemy, dt time.Duration) StateID {
	if e.Grounded {
		if e.rng.Intn(2) == 0 {
			e.Facing = 1
		} else {
			e.Facing = -1
		}
		return StateRun
	}
	return StateFall
}

type runState struct{}

func (runState) ID() StateID { return StateRun }
func (runState) Enter(e *Enemy) {
	e.PlayAnim("run")
	e.IntentTimer = e.Kind.Tuning.IntentTickS
}
func (runState) Exit(e *Enemy) {}
func (runState) Update(e *Enemy, dt time.Duration) StateID {
	e.VX = float64(e.Facing) * e.Kind.Tuning.RunSpeed

	e.IntentTimer -= dt.Seconds()
	if e.IntentTimer <= 0 {
		e.IntentTimer = e.Kind.Tuning.IntentTickS
		if e.rng.Float64() < 0.5 {
			if e.rng.Float64() < 0.5 {
				return StateAttack
			}
			return StateAttack2
		}
	}
	return StateRun
}

type attackState struct{}

func (attackState) ID() StateID { return StateAttack }
func (attackState) Enter(e *Enemy) {
	e.PlayAnim("attack")
	e.VX = 0
	e.HitSet = map[combat.Fighter]bool{}
}
func (attackState) Exit(e *Enemy) {}
func (attackState) Update(e *Enemy, dt time.Duration) StateID {
	applyMotion(e, "attack")
	if e.Current != nil && e.Current.Done() {
		return StateRun
	}
	return StateAttack
}

type attack2State struct{}

func (attack2State) ID() StateID { return StateAttack2 }
func (attack2State) Enter(e *Enemy) {
	e.PlayAnim("attack2")
	e.VX = 0
	e.HitSet = map[combat.Fighter]bool{}
}
func (attack2State) Exit(e *Enemy) {}
func (attack2State) Update(e *Enemy, dt time.Duration) StateID {
	applyMotion(e, "attack2")
	if e.Current != nil && e.Current.Done() {
		return StateRun
	}
	return StateAttack2
}

type hurtState struct{}

func (hurtState) ID() StateID { return StateHurt }
func (hurtState) Enter(e *Enemy) {
	e.PlayAnim("hurt")
}
func (hurtState) Exit(e *Enemy) {}
func (hurtState) Update(e *Enemy, dt time.Duration) StateID {
	if e.Current != nil && e.Current.Done() && e.Grounded {
		if e.rng.Intn(2) == 0 {
			e.Facing = 1
		} else {
			e.Facing = -1
		}
		return StateRun
	}
	return StateHurt
}

type deathState struct{}

func (deathState) ID() StateID { return StateDeath }
func (deathState) Enter(e *Enemy) {
	e.PlayAnim("death")
	e.VX = 0
}
func (deathState) Exit(e *Enemy) {}
func (deathState) Update(e *Enemy, dt time.Duration) StateID {
	if e.Current != nil && e.Current.Done() {
		e.Dead = true
	}
	return StateDeath
}
