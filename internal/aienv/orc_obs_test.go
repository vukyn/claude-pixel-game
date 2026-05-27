package aienv

import (
	"math"
	"testing"
)

func TestOrcObsSize(t *testing.T) {
	if OrcObsSize != 16 {
		t.Fatalf("OrcObsSize = %d, want 16", OrcObsSize)
	}
}

func TestOrcObserve_Position(t *testing.T) {
	os := OrcGameState{
		OrcX: 640, OrcY: 600,
		WindowW: 1280, WindowH: 720,
	}
	obs := OrcObserve(os)
	if obs[0] != 0.5 {
		t.Errorf("orc_x = %f, want 0.5", obs[0])
	}
}

func TestOrcObserve_PlayerRel(t *testing.T) {
	os := OrcGameState{
		OrcX: 400, OrcY: 600,
		PlayerX: 800, PlayerY: 600,
		WindowW: 1280, WindowH: 720,
	}
	obs := OrcObserve(os)
	if obs[8] < 0.6 || obs[8] > 0.7 {
		t.Errorf("player_rel_x = %f, want ~0.656", obs[8])
	}
}

func TestOrcObserve_PlayerAttacking(t *testing.T) {
	os := OrcGameState{
		PlayerAttacking: true,
		WindowW: 1280, WindowH: 720,
	}
	obs := OrcObserve(os)
	if obs[12] != 1.0 {
		t.Errorf("player_attacking = %f, want 1.0", obs[12])
	}
}

func TestOrcObserve_FacingToward(t *testing.T) {
	os := OrcGameState{
		OrcX: 400, PlayerX: 800,
		PlayerFacing: -1,
		WindowW: 1280, WindowH: 720,
	}
	obs := OrcObserve(os)
	if obs[13] != 1.0 {
		t.Errorf("player facing toward orc should be 1.0, got %f", obs[13])
	}

	os.PlayerFacing = 1
	obs = OrcObserve(os)
	if obs[13] != 0.0 {
		t.Errorf("player facing away should be 0.0, got %f", obs[13])
	}
}

func TestOrcObserve_Distance(t *testing.T) {
	os := OrcGameState{
		OrcX: 0, OrcY: 0,
		PlayerX: 1280, PlayerY: 720,
		WindowW: 1280, WindowH: 720,
	}
	obs := OrcObserve(os)
	if math.Abs(obs[14]-1.0) > 0.01 {
		t.Errorf("max distance should be ~1.0, got %f", obs[14])
	}
}
