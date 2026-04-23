package enemy

import (
	"context"
	"fmt"

	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

// Tuning holds per-kind physics/AI knobs read from the tuning table with
// a prefix (e.g. "orc_run_speed", "slime_run_speed").
type Tuning struct {
	MaxLives     float64
	RunSpeed     float64
	IntentTickS  float64
	HurtBounceVX float64
	HurtBounceVY float64
	FootPadding  int
}

// SpawnTuning is global (all kinds) and lives under the enemy_* key prefix.
type SpawnTuning struct {
	MinS     float64
	MaxS     float64
	MaxAlive int
}

func loadTuneMap(repo *storage.Repository[player.TuningParam]) (map[string]float64, error) {
	params, err := repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	m := make(map[string]float64, len(params))
	for _, p := range params {
		m[p.Key] = p.Value
	}
	return m, nil
}

// LoadTuningFor reads six per-kind keys: <prefix>_max_lives, <prefix>_run_speed,
// <prefix>_intent_tick_s, <prefix>_hurt_bounce_vx, <prefix>_hurt_bounce_vy,
// <prefix>_foot_padding.
func LoadTuningFor(repo *storage.Repository[player.TuningParam], prefix string) (*Tuning, error) {
	m, err := loadTuneMap(repo)
	if err != nil {
		return nil, err
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
		{prefix + "_max_lives", &t.MaxLives},
		{prefix + "_run_speed", &t.RunSpeed},
		{prefix + "_intent_tick_s", &t.IntentTickS},
		{prefix + "_hurt_bounce_vx", &t.HurtBounceVX},
		{prefix + "_hurt_bounce_vy", &t.HurtBounceVY},
	}
	for _, k := range keys {
		v, err := pick(k.k)
		if err != nil {
			return nil, err
		}
		*k.p = v
	}
	pad, err := pick(prefix + "_foot_padding")
	if err != nil {
		return nil, err
	}
	t.FootPadding = int(pad)
	return t, nil
}

// LoadSpawnTuning reads the three global spawn keys: enemy_spawn_min_s,
// enemy_spawn_max_s, enemy_max_alive.
func LoadSpawnTuning(repo *storage.Repository[player.TuningParam]) (*SpawnTuning, error) {
	m, err := loadTuneMap(repo)
	if err != nil {
		return nil, err
	}
	pick := func(k string) (float64, error) {
		v, ok := m[k]
		if !ok {
			return 0, fmt.Errorf("missing tuning key %q", k)
		}
		return v, nil
	}
	st := &SpawnTuning{}
	if v, err := pick("enemy_spawn_min_s"); err != nil {
		return nil, err
	} else {
		st.MinS = v
	}
	if v, err := pick("enemy_spawn_max_s"); err != nil {
		return nil, err
	} else {
		st.MaxS = v
	}
	if v, err := pick("enemy_max_alive"); err != nil {
		return nil, err
	} else {
		st.MaxAlive = int(v)
	}
	return st, nil
}
