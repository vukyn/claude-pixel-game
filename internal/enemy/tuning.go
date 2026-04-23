package enemy

import (
	"context"
	"fmt"

	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

type Tuning struct {
	MaxLives     float64
	RunSpeed     float64
	IntentTickS  float64
	HurtBounceVX float64
	HurtBounceVY float64
	SpawnMinS    float64
	SpawnMaxS    float64
	MaxAlive     float64
}

func LoadTuning(repo *storage.Repository[player.TuningParam]) (*Tuning, error) {
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
	t := &Tuning{}
	keys := []struct {
		k string
		p *float64
	}{
		{"orc_max_lives", &t.MaxLives},
		{"orc_run_speed", &t.RunSpeed},
		{"orc_intent_tick_s", &t.IntentTickS},
		{"orc_hurt_bounce_vx", &t.HurtBounceVX},
		{"orc_hurt_bounce_vy", &t.HurtBounceVY},
		{"orc_spawn_min_s", &t.SpawnMinS},
		{"orc_spawn_max_s", &t.SpawnMaxS},
		{"orc_max_alive", &t.MaxAlive},
	}
	for _, k := range keys {
		v, err := pick(k.k)
		if err != nil {
			return nil, err
		}
		*k.p = v
	}
	return t, nil
}
