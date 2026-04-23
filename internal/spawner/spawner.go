package spawner

import (
	"math/rand"
	"time"

	"claude-pixel/internal/enemy"
)

type KindFactory struct {
	Name     string
	Weight   int
	NewEnemy func(x, y float64) *enemy.Enemy
}

type Spawner struct {
	MinIntervalS float64
	MaxIntervalS float64
	MaxAlive     int
	nextSpawn    float64
	spawnXMin    float64
	spawnXMax    float64
	rng          *rand.Rand
	kinds        []KindFactory
	totalWeight  int
}

type Config struct {
	MinIntervalS float64
	MaxIntervalS float64
	MaxAlive     int
	SpawnXMin    float64
	SpawnXMax    float64
	RNG          *rand.Rand
	Kinds        []KindFactory
}

func New(cfg Config) *Spawner {
	tw := 0
	for _, k := range cfg.Kinds {
		if k.Weight <= 0 {
			continue
		}
		tw += k.Weight
	}
	s := &Spawner{
		MinIntervalS: cfg.MinIntervalS,
		MaxIntervalS: cfg.MaxIntervalS,
		MaxAlive:     cfg.MaxAlive,
		spawnXMin:    cfg.SpawnXMin,
		spawnXMax:    cfg.SpawnXMax,
		rng:          cfg.RNG,
		kinds:        cfg.Kinds,
		totalWeight:  tw,
	}
	s.nextSpawn = s.rollInterval()
	return s
}

func (s *Spawner) NextSpawnS() float64 { return s.nextSpawn }

func (s *Spawner) Reset() {
	s.nextSpawn = s.rollInterval()
}

func (s *Spawner) Tick(dt time.Duration, alive int) *enemy.Enemy {
	s.nextSpawn -= dt.Seconds()
	if s.nextSpawn > 0 {
		return nil
	}
	s.nextSpawn = s.rollInterval()
	if alive >= s.MaxAlive {
		return nil
	}
	k := s.pickKind()
	if k.NewEnemy == nil {
		return nil
	}
	return k.NewEnemy(s.rollSpawnX(), 0)
}

func (s *Spawner) pickKind() KindFactory {
	switch len(s.kinds) {
	case 0:
		return KindFactory{}
	case 1:
		return s.kinds[0]
	}
	if s.totalWeight <= 0 {
		return s.kinds[0]
	}
	r := s.rng.Intn(s.totalWeight)
	for _, k := range s.kinds {
		if k.Weight <= 0 {
			continue
		}
		if r < k.Weight {
			return k
		}
		r -= k.Weight
	}
	return s.kinds[len(s.kinds)-1]
}

func (s *Spawner) rollInterval() float64 {
	return s.MinIntervalS + s.rng.Float64()*(s.MaxIntervalS-s.MinIntervalS)
}

func (s *Spawner) rollSpawnX() float64 {
	return s.spawnXMin + s.rng.Float64()*(s.spawnXMax-s.spawnXMin)
}
