package spawner

import (
	"math/rand"
	"testing"
	"time"

	"claude-pixel/internal/enemy"
)

func fakeFactory(calls *int) func(x, y float64) *enemy.Enemy {
	return func(x, y float64) *enemy.Enemy {
		*calls++
		return &enemy.Enemy{X: x, Y: y}
	}
}

func TestSpawnerRespectsInterval(t *testing.T) {
	calls := 0
	s := New(Config{
		MinIntervalS: 2, MaxIntervalS: 2, MaxAlive: 5,
		SpawnXMin: 100, SpawnXMax: 200, SpawnY: -300,
		RNG:      rand.New(rand.NewSource(1)),
		NewEnemy: fakeFactory(&calls),
	})
	if got := s.Tick(time.Second, 0); got != nil {
		t.Errorf("at t=1s, should not spawn yet")
	}
	if got := s.Tick(time.Second, 0); got == nil {
		t.Errorf("at t=2s, should spawn")
	}
	if calls != 1 {
		t.Errorf("want 1 factory call, got %d", calls)
	}
}

func TestSpawnerSkipsWhenAtCap(t *testing.T) {
	calls := 0
	s := New(Config{
		MinIntervalS: 1, MaxIntervalS: 1, MaxAlive: 2,
		SpawnXMin: 100, SpawnXMax: 200, SpawnY: -300,
		RNG:      rand.New(rand.NewSource(1)),
		NewEnemy: fakeFactory(&calls),
	})
	if got := s.Tick(time.Second, 2); got != nil {
		t.Errorf("at cap=2 alive=2, should skip")
	}
	if calls != 0 {
		t.Errorf("want 0 factory calls, got %d", calls)
	}
	if got := s.Tick(time.Second, 2); got != nil {
		t.Errorf("still at cap, should skip")
	}
	if got := s.Tick(time.Second, 1); got == nil {
		t.Errorf("under cap, should spawn")
	}
	if calls != 1 {
		t.Errorf("want 1 factory call, got %d", calls)
	}
}

func TestSpawnerIntervalWithinRange(t *testing.T) {
	s := New(Config{
		MinIntervalS: 3, MaxIntervalS: 10, MaxAlive: 5,
		SpawnXMin: 100, SpawnXMax: 200, SpawnY: -300,
		RNG:      rand.New(rand.NewSource(42)),
		NewEnemy: func(x, y float64) *enemy.Enemy { return &enemy.Enemy{} },
	})
	for i := 0; i < 50; i++ {
		iv := s.rollInterval()
		if iv < 3 || iv > 10 {
			t.Fatalf("interval out of range: %v", iv)
		}
	}
}
