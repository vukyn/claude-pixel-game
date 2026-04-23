package spawner

import (
	"math/rand"
	"time"

	"claude-pixel/internal/enemy"
)

type Spawner struct {
	MinIntervalS float64
	MaxIntervalS float64
	MaxAlive     int
	nextSpawn    float64
	spawnXMin    float64
	spawnXMax    float64
	spawnY       float64
	rng          *rand.Rand
	newEnemy     func(x, y float64) *enemy.Enemy
}

type Config struct {
	MinIntervalS float64
	MaxIntervalS float64
	MaxAlive     int
	SpawnXMin    float64
	SpawnXMax    float64
	SpawnY       float64
	RNG          *rand.Rand
	NewEnemy     func(x, y float64) *enemy.Enemy
}

func New(cfg Config) *Spawner {
	s := &Spawner{
		MinIntervalS: cfg.MinIntervalS,
		MaxIntervalS: cfg.MaxIntervalS,
		MaxAlive:     cfg.MaxAlive,
		spawnXMin:    cfg.SpawnXMin,
		spawnXMax:    cfg.SpawnXMax,
		spawnY:       cfg.SpawnY,
		rng:          cfg.RNG,
		newEnemy:     cfg.NewEnemy,
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
	return s.newEnemy(s.rollSpawnX(), s.spawnY)
}

func (s *Spawner) rollInterval() float64 {
	return s.MinIntervalS + s.rng.Float64()*(s.MaxIntervalS-s.MinIntervalS)
}

func (s *Spawner) rollSpawnX() float64 {
	return s.spawnXMin + s.rng.Float64()*(s.spawnXMax-s.spawnXMin)
}
