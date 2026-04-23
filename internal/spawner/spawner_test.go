package spawner

import (
	"math/rand"
	"testing"
	"time"

	"claude-pixel/internal/enemy"
)

func fakeFactory(name string, calls *int) KindFactory {
	return KindFactory{
		Name:   name,
		Weight: 1,
		NewEnemy: func(x, y float64) *enemy.Enemy {
			*calls++
			return &enemy.Enemy{X: x, Y: y}
		},
	}
}

func TestSpawnerRespectsInterval(t *testing.T) {
	calls := 0
	s := New(Config{
		MinIntervalS: 2, MaxIntervalS: 2, MaxAlive: 5,
		SpawnXMin: 100, SpawnXMax: 200,
		RNG:   rand.New(rand.NewSource(1)),
		Kinds: []KindFactory{fakeFactory("test", &calls)},
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
		SpawnXMin: 100, SpawnXMax: 200,
		RNG:   rand.New(rand.NewSource(1)),
		Kinds: []KindFactory{fakeFactory("test", &calls)},
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
		SpawnXMin: 100, SpawnXMax: 200,
		RNG: rand.New(rand.NewSource(42)),
		Kinds: []KindFactory{{Name: "test", Weight: 1,
			NewEnemy: func(x, y float64) *enemy.Enemy { return &enemy.Enemy{} }}},
	})
	for i := 0; i < 50; i++ {
		iv := s.rollInterval()
		if iv < 3 || iv > 10 {
			t.Fatalf("interval out of range: %v", iv)
		}
	}
}

func TestSpawnerPicksAmongKindsUniformly(t *testing.T) {
	orcCalls := 0
	slimeCalls := 0
	s := New(Config{
		MinIntervalS: 0, MaxIntervalS: 0, MaxAlive: 10000,
		SpawnXMin: 100, SpawnXMax: 200,
		RNG: rand.New(rand.NewSource(1)),
		Kinds: []KindFactory{
			fakeFactory("orc", &orcCalls),
			fakeFactory("slime", &slimeCalls),
		},
	})
	const N = 2000
	for i := 0; i < N; i++ {
		s.Tick(time.Second, 0)
	}
	if orcCalls+slimeCalls != N {
		t.Fatalf("expected %d total spawns, got %d orc + %d slime", N, orcCalls, slimeCalls)
	}
	ratio := float64(orcCalls) / float64(N)
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("ratio %f outside [0.4, 0.6] — distribution skewed", ratio)
	}
}
