package enemy

import (
	"time"

	"claude-pixel/internal/combat"
)

type fallState struct{}

func (fallState) ID() StateID { return StateFall }
func (fallState) Enter(e *Enemy) {
	e.PlayAnim("orc_idle")
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
	e.PlayAnim("orc_run")
	e.IntentTimer = e.Tuning.IntentTickS
}
func (runState) Exit(e *Enemy) {}
func (runState) Update(e *Enemy, dt time.Duration) StateID {
	e.VX = float64(e.Facing) * e.RunSpeed

	e.IntentTimer -= dt.Seconds()
	if e.IntentTimer <= 0 {
		e.IntentTimer = e.Tuning.IntentTickS
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
	e.PlayAnim("orc_attack")
	e.VX = 0
	e.HitSet = map[combat.Fighter]bool{}
}
func (attackState) Exit(e *Enemy) {}
func (attackState) Update(e *Enemy, dt time.Duration) StateID {
	if e.Current != nil && e.Current.Done() {
		return StateRun
	}
	return StateAttack
}

type attack2State struct{}

func (attack2State) ID() StateID { return StateAttack2 }
func (attack2State) Enter(e *Enemy) {
	e.PlayAnim("orc_attack2")
	e.VX = 0
	e.HitSet = map[combat.Fighter]bool{}
}
func (attack2State) Exit(e *Enemy) {}
func (attack2State) Update(e *Enemy, dt time.Duration) StateID {
	if e.Current != nil && e.Current.Done() {
		return StateRun
	}
	return StateAttack2
}

type hurtState struct{}

func (hurtState) ID() StateID { return StateHurt }
func (hurtState) Enter(e *Enemy) {
	e.PlayAnim("orc_hurt")
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
	e.PlayAnim("orc_death")
	e.VX = 0
}
func (deathState) Exit(e *Enemy) {}
func (deathState) Update(e *Enemy, dt time.Duration) StateID {
	if e.Current != nil && e.Current.Done() {
		e.Dead = true
	}
	return StateDeath
}
