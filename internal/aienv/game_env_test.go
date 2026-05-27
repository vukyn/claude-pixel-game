package aienv

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Tests depend on .env and data/game.db at the project root.
	if err := os.Chdir("../.."); err != nil {
		panic("chdir to project root: " + err.Error())
	}
	os.Exit(m.Run())
}

func TestGameEnv_Reset(t *testing.T) {
	env, err := NewGameEnv(EnvConfig{})
	if err != nil {
		t.Fatalf("NewGameEnv: %v", err)
	}
	obs := env.Reset()
	if len(obs) != ObsSize {
		t.Fatalf("obs length = %d, want %d", len(obs), ObsSize)
	}
	if obs[6] != 1.0 {
		t.Errorf("initial lives should be full (1.0), got %f", obs[6])
	}
	if obs[9] != 1.0 {
		t.Errorf("initial time_remaining should be 1.0, got %f", obs[9])
	}
}

func TestGameEnv_StepIdle(t *testing.T) {
	env, err := NewGameEnv(EnvConfig{})
	if err != nil {
		t.Fatalf("NewGameEnv: %v", err)
	}
	env.Reset()
	obs, reward, done, info := env.Step(0)
	if len(obs) != ObsSize {
		t.Fatalf("obs length = %d, want %d", len(obs), ObsSize)
	}
	if done {
		t.Error("should not be done after 1 idle step")
	}
	if reward == 0 {
		t.Error("reward should include survival bonus")
	}
	_ = info
}

func TestGameEnv_StepSequence(t *testing.T) {
	env, err := NewGameEnv(EnvConfig{Seed: 42})
	if err != nil {
		t.Fatalf("NewGameEnv: %v", err)
	}
	env.Reset()
	steps := 0
	done := false
	for !done && steps < 5000 {
		_, _, done, _ = env.Step(2)
		steps++
	}
	if !done {
		t.Error("episode should end by timeout or death within 5000 steps")
	}
}

func TestGameEnv_ResetClears(t *testing.T) {
	env, err := NewGameEnv(EnvConfig{Seed: 42})
	if err != nil {
		t.Fatalf("NewGameEnv: %v", err)
	}
	env.Reset()
	for i := 0; i < 100; i++ {
		env.Step(2)
	}
	obs := env.Reset()
	if obs[9] != 1.0 {
		t.Errorf("after reset, time_remaining should be 1.0, got %f", obs[9])
	}
}
