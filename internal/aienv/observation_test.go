package aienv

import (
	"math"
	"testing"
)

func TestObsSize(t *testing.T) {
	if ObsSize != 31 {
		t.Fatalf("ObsSize = %d, want 31", ObsSize)
	}
}

func TestObserve_PlayerPosition(t *testing.T) {
	gs := GameState{
		PlayerX: 640, PlayerY: 360,
		WindowW: 1280, WindowH: 720,
	}
	obs := Observe(gs)
	if obs[0] != 0.5 {
		t.Errorf("player_x norm = %f, want 0.5", obs[0])
	}
	if obs[1] != 0.5 {
		t.Errorf("player_y norm = %f, want 0.5", obs[1])
	}
}

func TestObserve_PlayerFacing(t *testing.T) {
	gs := GameState{Facing: 1, WindowW: 1280, WindowH: 720}
	obs := Observe(gs)
	if obs[4] != 1.0 {
		t.Errorf("facing right = %f, want 1.0", obs[4])
	}
	gs.Facing = -1
	obs = Observe(gs)
	if obs[4] != 0.0 {
		t.Errorf("facing left = %f, want 0.0", obs[4])
	}
}

func TestObserve_Grounded(t *testing.T) {
	gs := GameState{Grounded: true, WindowW: 1280, WindowH: 720}
	obs := Observe(gs)
	if obs[5] != 1.0 {
		t.Errorf("grounded = %f, want 1.0", obs[5])
	}
}

func TestObserve_NoEnemies(t *testing.T) {
	gs := GameState{WindowW: 1280, WindowH: 720, MaxAlive: 5}
	obs := Observe(gs)
	for i := 10; i <= 24; i++ {
		if obs[i] != 0.0 {
			t.Errorf("obs[%d] = %f, want 0.0 (no enemies)", i, obs[i])
		}
	}
	if obs[25] != 0.0 {
		t.Errorf("num_enemies = %f, want 0.0", obs[25])
	}
}

func TestObserve_WithEnemy(t *testing.T) {
	gs := GameState{
		PlayerX: 640, PlayerY: 600,
		WindowW: 1280, WindowH: 720,
		MaxAlive: 5, MaxLives: 10,
		TimeoutS: 30, ElapsedS: 15,
		Enemies: []EnemyState{
			{RelX: 100, RelY: 0, Lives: 2, MaxLives: 2, State: 1, Attacking: false},
		},
	}
	obs := Observe(gs)
	if obs[10] < 0.07 || obs[10] > 0.09 {
		t.Errorf("enemy1 rel_x = %f, want ~0.078", obs[10])
	}
	if math.Abs(obs[25]-1.0/5.0) > 0.01 {
		t.Errorf("num_enemies = %f, want ~0.2", obs[25])
	}
	if math.Abs(obs[9]-0.5) > 0.01 {
		t.Errorf("time_remaining = %f, want 0.5", obs[9])
	}
}

func TestObserve_Clamped(t *testing.T) {
	gs := GameState{
		PlayerX: -100, PlayerY: 9999,
		WindowW: 1280, WindowH: 720,
	}
	obs := Observe(gs)
	if obs[0] < 0 || obs[0] > 1 {
		t.Errorf("player_x should be clamped [0,1], got %f", obs[0])
	}
	if obs[1] < 0 || obs[1] > 1 {
		t.Errorf("player_y should be clamped [0,1], got %f", obs[1])
	}
}
