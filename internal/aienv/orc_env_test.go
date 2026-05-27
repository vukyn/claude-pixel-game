package aienv

import (
	"os"
	"testing"
)

func TestOrcEnv_Reset(t *testing.T) {
	if err := os.Chdir(findProjectRoot()); err != nil {
		t.Skip("cannot chdir to project root")
	}
	env, err := NewOrcTrainEnv(OrcEnvConfig{Seed: 42})
	if err != nil {
		t.Fatalf("NewOrcTrainEnv: %v", err)
	}
	result := env.Reset()
	if len(result.PlayerObs) != ObsSize {
		t.Fatalf("player obs = %d, want %d", len(result.PlayerObs), ObsSize)
	}
	if len(result.OrcObs) == 0 {
		t.Fatal("should have at least 1 orc obs after reset")
	}
	if len(result.OrcObs[0]) != OrcObsSize {
		t.Fatalf("orc obs[0] = %d, want %d", len(result.OrcObs[0]), OrcObsSize)
	}
}

func TestOrcEnv_StepIdle(t *testing.T) {
	if err := os.Chdir(findProjectRoot()); err != nil {
		t.Skip("cannot chdir to project root")
	}
	env, err := NewOrcTrainEnv(OrcEnvConfig{Seed: 42})
	if err != nil {
		t.Fatalf("NewOrcTrainEnv: %v", err)
	}
	res := env.Reset()
	orcActions := make([]int, len(res.OrcObs))
	result := env.Step(0, orcActions)
	if len(result.PlayerObs) != ObsSize {
		t.Fatalf("player obs = %d, want %d", len(result.PlayerObs), ObsSize)
	}
	if result.Done {
		t.Error("should not be done after 1 step")
	}
}

func TestOrcEnv_StepSequence(t *testing.T) {
	if err := os.Chdir(findProjectRoot()); err != nil {
		t.Skip("cannot chdir to project root")
	}
	env, err := NewOrcTrainEnv(OrcEnvConfig{Seed: 42})
	if err != nil {
		t.Fatalf("NewOrcTrainEnv: %v", err)
	}
	res := env.Reset()
	steps := 0
	done := false
	for !done && steps < 5000 {
		orcActions := make([]int, len(res.OrcObs))
		for i := range orcActions {
			orcActions[i] = 1
		}
		res = env.Step(0, orcActions)
		done = res.Done
		steps++
	}
	if !done {
		t.Error("episode should end by timeout or death within 5000 steps")
	}
}

func TestOrcEnv_OrcRewards(t *testing.T) {
	if err := os.Chdir(findProjectRoot()); err != nil {
		t.Skip("cannot chdir to project root")
	}
	env, err := NewOrcTrainEnv(OrcEnvConfig{Seed: 42})
	if err != nil {
		t.Fatalf("NewOrcTrainEnv: %v", err)
	}
	res := env.Reset()
	orcActions := make([]int, len(res.OrcObs))
	result := env.Step(0, orcActions)
	if len(result.OrcRewards) != len(result.OrcObs) {
		t.Errorf("rewards count %d != obs count %d", len(result.OrcRewards), len(result.OrcObs))
	}
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(dir + "/.env"); err == nil {
			return dir
		}
		parent := dir[:max(0, len(dir)-1)]
		for parent != "" && parent[len(parent)-1] != '/' {
			parent = parent[:len(parent)-1]
		}
		if parent == "" || parent == dir {
			return "."
		}
		dir = parent[:len(parent)-1]
	}
}
