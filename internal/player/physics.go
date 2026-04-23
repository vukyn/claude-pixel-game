package player

import (
	"context"
	"fmt"

	"claude-pixel/internal/storage"
)

type StaminaTuning struct {
	Max, DrainPerSec, RegenPerSec float64
}

func LoadStaminaTuning(repo *storage.Repository[TuningParam]) (*StaminaTuning, error) {
	params, err := repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	m := make(map[string]float64, len(params))
	for _, p := range params {
		m[p.Key] = p.Value
	}
	pick := func(k string) (float64, error) {
		v, ok := m[k]
		if !ok {
			return 0, fmt.Errorf("missing tuning key %q", k)
		}
		return v, nil
	}
	st := &StaminaTuning{}
	var e error
	if st.Max, e = pick("stamina_max"); e != nil {
		return nil, e
	}
	if st.DrainPerSec, e = pick("stamina_drain_per_s"); e != nil {
		return nil, e
	}
	if st.RegenPerSec, e = pick("stamina_regen_per_s"); e != nil {
		return nil, e
	}
	return st, nil
}

type Physics struct {
	RunSpeed     float64
	AirControl   float64
	JumpVelocity float64
	Gravity      float64
	MaxFallSpeed float64
	SprintSpeed  float64
}

func LoadPhysics(repo *storage.Repository[TuningParam]) (*Physics, error) {
	params, err := repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	m := make(map[string]float64, len(params))
	for _, p := range params {
		m[p.Key] = p.Value
	}
	pick := func(k string) (float64, error) {
		v, ok := m[k]
		if !ok {
			return 0, fmt.Errorf("missing tuning key %q", k)
		}
		return v, nil
	}
	ph := &Physics{}
	var e error
	if ph.RunSpeed, e = pick("run_speed"); e != nil {
		return nil, e
	}
	if ph.AirControl, e = pick("air_control"); e != nil {
		return nil, e
	}
	if ph.JumpVelocity, e = pick("jump_velocity"); e != nil {
		return nil, e
	}
	if ph.Gravity, e = pick("gravity"); e != nil {
		return nil, e
	}
	if ph.MaxFallSpeed, e = pick("max_fall_speed"); e != nil {
		return nil, e
	}
	if ph.SprintSpeed, e = pick("sprint_speed"); e != nil {
		return nil, e
	}
	return ph, nil
}
