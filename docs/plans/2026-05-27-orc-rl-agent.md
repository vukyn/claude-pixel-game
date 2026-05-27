# Orc RL Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Train a PPO-based RL agent to control orc enemies via self-play against the trained player agent. Replaces behavior tree entirely.

**Architecture:** New `OrcTrainEnv` in Go handles multi-agent step: player action (from fixed model) + per-orc actions (training). Python side manages both models and talks to Go via extended JSON-line protocol. Separate `cmd/train-orc/` binary.

**Tech Stack:** Go (headless multi-agent env), Python 3.10+ (PyTorch, SB3, Gymnasium)

**Design Spec:** `docs/superpowers/specs/2026-05-27-orc-rl-agent-design.md`

---

## File Structure

### New Go files

| File | Responsibility |
|------|----------------|
| `internal/aienv/orc_action.go` | Orc action ID (0-5) → VX/state changes |
| `internal/aienv/orc_action_test.go` | Tests |
| `internal/aienv/orc_obs.go` | 16-float observation from orc's perspective |
| `internal/aienv/orc_obs_test.go` | Tests |
| `internal/aienv/orc_reward.go` | Orc per-step reward function |
| `internal/aienv/orc_reward_test.go` | Tests |
| `internal/aienv/orc_env.go` | OrcTrainEnv: multi-agent Step/Reset |
| `internal/aienv/orc_env_test.go` | Integration tests |
| `cmd/train-orc/main.go` | TCP server for orc training |
| `cmd/train-orc/protocol.go` | Multi-agent JSON-line messages |
| `cmd/train-orc/protocol_test.go` | Tests |

### New Python files

| File | Responsibility |
|------|----------------|
| `ai/orc_config.py` | Orc obs/action/PPO constants |
| `ai/orc_env.py` | OrcGymEnv: multi-agent Gymnasium wrapper |
| `ai/train_orc.py` | Orc PPO training script |

### Modified files

| File | Change |
|------|--------|
| `Makefile` | Add orc training targets |

---

## Task 1: Orc Action Mapping

**Files:**
- Create: `internal/aienv/orc_action.go`
- Create: `internal/aienv/orc_action_test.go`

6 discrete actions for orc. Unlike player actions (which map to Intent), orc actions directly set VX and trigger state transitions on the Enemy struct.

- [ ] **Step 1: Write failing tests**

```go
// internal/aienv/orc_action_test.go
package aienv

import (
	"testing"
)

func TestOrcNumActions(t *testing.T) {
	if OrcNumActions != 6 {
		t.Errorf("OrcNumActions = %d, want 6", OrcNumActions)
	}
}

func TestOrcActionIdle(t *testing.T) {
	a := OrcAction(0)
	if a.VXMode != OrcVXStop {
		t.Errorf("action 0: VXMode = %d, want OrcVXStop", a.VXMode)
	}
	if a.Transition != "" {
		t.Errorf("action 0: Transition = %q, want empty", a.Transition)
	}
}

func TestOrcActionToward(t *testing.T) {
	a := OrcAction(1)
	if a.VXMode != OrcVXToward {
		t.Errorf("action 1: VXMode = %d, want OrcVXToward", a.VXMode)
	}
}

func TestOrcActionAway(t *testing.T) {
	a := OrcAction(2)
	if a.VXMode != OrcVXAway {
		t.Errorf("action 2: VXMode = %d, want OrcVXAway", a.VXMode)
	}
}

func TestOrcActionAttack1(t *testing.T) {
	a := OrcAction(3)
	if a.Transition != "attack" {
		t.Errorf("action 3: Transition = %q, want attack", a.Transition)
	}
}

func TestOrcActionAttack2(t *testing.T) {
	a := OrcAction(4)
	if a.Transition != "attack2" {
		t.Errorf("action 4: Transition = %q, want attack2", a.Transition)
	}
}

func TestOrcActionFlip(t *testing.T) {
	a := OrcAction(5)
	if a.VXMode != OrcVXStop {
		t.Errorf("action 5: VXMode = %d, want OrcVXStop", a.VXMode)
	}
	if !a.Flip {
		t.Error("action 5: Flip should be true")
	}
}

func TestOrcActionOutOfRange(t *testing.T) {
	a := OrcAction(99)
	if a.VXMode != OrcVXStop {
		t.Errorf("out of range: VXMode = %d, want OrcVXStop", a.VXMode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/aienv/ -v -run TestOrcAction`
Expected: compilation error — types not defined

- [ ] **Step 3: Implement orc action mapping**

```go
// internal/aienv/orc_action.go
package aienv

const OrcNumActions = 6

type OrcVXMode int

const (
	OrcVXStop   OrcVXMode = iota
	OrcVXToward
	OrcVXAway
)

type OrcActionResult struct {
	VXMode     OrcVXMode
	Transition string
	Flip       bool
}

func OrcAction(action int) OrcActionResult {
	switch action {
	case 0:
		return OrcActionResult{VXMode: OrcVXStop}
	case 1:
		return OrcActionResult{VXMode: OrcVXToward}
	case 2:
		return OrcActionResult{VXMode: OrcVXAway}
	case 3:
		return OrcActionResult{Transition: "attack"}
	case 4:
		return OrcActionResult{Transition: "attack2"}
	case 5:
		return OrcActionResult{VXMode: OrcVXStop, Flip: true}
	default:
		return OrcActionResult{VXMode: OrcVXStop}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/aienv/ -v -run TestOrcAction`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aienv/orc_action.go internal/aienv/orc_action_test.go
git commit -m "feat(ai): add orc action mapping for RL agent"
```

---

## Task 2: Orc Observation Vector

**Files:**
- Create: `internal/aienv/orc_obs.go`
- Create: `internal/aienv/orc_obs_test.go`

16-float observation from orc's perspective. Pure function, no game dependencies.

- [ ] **Step 1: Write failing tests**

```go
// internal/aienv/orc_obs_test.go
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
	// player_rel_x = (800-400)/1280 shifted to [0,1] → (0.3125 + 1)/2 = 0.656
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/aienv/ -v -run TestOrcObs`
Expected: compilation error

- [ ] **Step 3: Implement orc observation**

```go
// internal/aienv/orc_obs.go
package aienv

import "math"

const OrcObsSize = 16

type OrcGameState struct {
	OrcX, OrcY         float64
	OrcVX, OrcVY       float64
	OrcFacing          int
	OrcGrounded        bool
	OrcLives           int
	OrcMaxLives        int
	OrcStateID         int
	OrcNumStates       int
	PlayerX, PlayerY   float64
	PlayerLives        int
	PlayerMaxLives     int
	PlayerStateID      int
	PlayerNumStates    int
	PlayerAttacking    bool
	PlayerFacing       int
	OrcMaxSpeed        float64
	OrcMaxFall         float64
	TimeoutS           float64
	ElapsedS           float64
	WindowW, WindowH   float64
}

func OrcObserve(gs OrcGameState) [OrcObsSize]float64 {
	var obs [OrcObsSize]float64

	obs[0] = clamp01(gs.OrcX / gs.WindowW)
	obs[1] = clamp01(gs.OrcY / gs.WindowH)
	obs[2] = clamp01((gs.OrcVX/safeDivisor(gs.OrcMaxSpeed) + 1) / 2)
	obs[3] = clamp01((gs.OrcVY/safeDivisor(gs.OrcMaxFall) + 1) / 2)
	if gs.OrcFacing >= 0 {
		obs[4] = 1
	}
	if gs.OrcGrounded {
		obs[5] = 1
	}
	obs[6] = clamp01(float64(gs.OrcLives) / safeDivisor(float64(gs.OrcMaxLives)))
	obs[7] = clamp01(float64(gs.OrcStateID) / safeDivisor(float64(gs.OrcNumStates)))

	relX := gs.PlayerX - gs.OrcX
	relY := gs.PlayerY - gs.OrcY
	obs[8] = clamp01((relX/gs.WindowW + 1) / 2)
	obs[9] = clamp01((relY/gs.WindowH + 1) / 2)

	obs[10] = clamp01(float64(gs.PlayerLives) / safeDivisor(float64(gs.PlayerMaxLives)))
	obs[11] = clamp01(float64(gs.PlayerStateID) / safeDivisor(float64(gs.PlayerNumStates)))
	if gs.PlayerAttacking {
		obs[12] = 1
	}

	playerFacingToward := false
	if gs.PlayerX > gs.OrcX && gs.PlayerFacing < 0 {
		playerFacingToward = true
	}
	if gs.PlayerX < gs.OrcX && gs.PlayerFacing > 0 {
		playerFacingToward = true
	}
	if playerFacingToward {
		obs[13] = 1
	}

	diag := math.Sqrt(gs.WindowW*gs.WindowW + gs.WindowH*gs.WindowH)
	dist := math.Sqrt(relX*relX + relY*relY)
	obs[14] = clamp01(dist / diag)

	remaining := gs.TimeoutS - gs.ElapsedS
	if remaining < 0 {
		remaining = 0
	}
	obs[15] = clamp01(remaining / safeDivisor(gs.TimeoutS))

	return obs
}
```

Note: `clamp01` and `safeDivisor` already exist in `observation.go` (same package).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/aienv/ -v -run TestOrcObs`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aienv/orc_obs.go internal/aienv/orc_obs_test.go
git commit -m "feat(ai): add orc observation vector for RL agent"
```

---

## Task 3: Orc Reward Function

**Files:**
- Create: `internal/aienv/orc_reward.go`
- Create: `internal/aienv/orc_reward_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/aienv/orc_reward_test.go
package aienv

import (
	"math"
	"testing"
)

func TestOrcReward_Survival(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{})
	if math.Abs(r-0.01) > 1e-6 {
		t.Errorf("idle step = %f, want 0.01", r)
	}
}

func TestOrcReward_HitPlayer(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{HitPlayer: 1})
	if r < 8 {
		t.Errorf("hit player = %f, want >= 8", r)
	}
}

func TestOrcReward_PlayerDied(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{PlayerDied: true})
	if r < 19 {
		t.Errorf("player died = %f, want >= 19", r)
	}
}

func TestOrcReward_OrcDied(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{OrcDied: true})
	if r > -14 {
		t.Errorf("orc died = %f, want <= -14", r)
	}
}

func TestOrcReward_LifeLost(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{OrcLivesLost: 1})
	if r > -4 {
		t.Errorf("life lost = %f, want <= -4", r)
	}
}

func TestOrcReward_DodgeSuccess(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{DodgeSuccess: true})
	base := OrcCalcReward(OrcRewardInput{})
	if r <= base {
		t.Errorf("dodge should increase reward: got %f, base %f", r, base)
	}
}

func TestOrcReward_MoveToward(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{DistDelta: -50})
	base := OrcCalcReward(OrcRewardInput{})
	if r <= base {
		t.Errorf("move toward should increase reward: got %f, base %f", r, base)
	}
}

func TestOrcReward_Stagnant(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{Stagnant: true})
	base := OrcCalcReward(OrcRewardInput{})
	if r >= base {
		t.Errorf("stagnant should decrease reward: got %f, base %f", r, base)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/aienv/ -v -run TestOrcReward`
Expected: compilation error

- [ ] **Step 3: Implement orc reward function**

```go
// internal/aienv/orc_reward.go
package aienv

type OrcRewardInput struct {
	HitPlayer    int
	PlayerDied   bool
	OrcLivesLost int
	OrcDied      bool
	DodgeSuccess bool
	Stagnant     bool
	DistDelta    float64
}

func OrcCalcReward(in OrcRewardInput) float64 {
	reward := 0.0

	reward += float64(in.HitPlayer) * 8.0

	if in.PlayerDied {
		reward += 20.0
		return reward
	}

	reward += float64(in.OrcLivesLost) * -5.0

	if in.OrcDied {
		reward += -15.0
		return reward
	}

	reward += 0.01

	if in.DodgeSuccess {
		reward += 3.0
	}
	if in.Stagnant {
		reward += -0.3
	}
	if in.DistDelta < 0 {
		reward += 0.1
	}

	return reward
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/aienv/ -v -run TestOrcReward`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aienv/orc_reward.go internal/aienv/orc_reward_test.go
git commit -m "feat(ai): add orc reward function for RL training"
```

---

## Task 4: OrcTrainEnv (Multi-Agent Headless Env)

**Files:**
- Create: `internal/aienv/orc_env.go`
- Create: `internal/aienv/orc_env_test.go`

The core multi-agent environment. Player uses a fixed action each step (provided externally). Orcs are RL-controlled. Replaces `enemy.Tick()` with RL action application for orcs.

- [ ] **Step 1: Write failing tests**

```go
// internal/aienv/orc_env_test.go
package aienv

import (
	"testing"
)

func TestOrcEnv_Reset(t *testing.T) {
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
	env, err := NewOrcTrainEnv(OrcEnvConfig{Seed: 42})
	if err != nil {
		t.Fatalf("NewOrcTrainEnv: %v", err)
	}
	res := env.Reset()
	orcActions := make([]int, len(res.OrcObs))
	result := env.Step(0, orcActions) // player idle, all orcs idle
	if len(result.PlayerObs) != ObsSize {
		t.Fatalf("player obs = %d, want %d", len(result.PlayerObs), ObsSize)
	}
	if result.Done {
		t.Error("should not be done after 1 step")
	}
}

func TestOrcEnv_StepSequence(t *testing.T) {
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
			orcActions[i] = 1 // all orcs move toward
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/aienv/ -v -run TestOrcEnv`
Expected: compilation error

- [ ] **Step 3: Implement OrcTrainEnv**

This is the biggest piece. Key differences from GameEnv:
- `Step(playerAction, orcActions[])` — two sets of actions
- Player action → Intent via `ToIntent()` (same as before)
- Orc actions → bypass `enemy.Tick()`, apply VX/transitions directly
- Returns `OrcStepResult` with per-orc observations and rewards
- Spawner still runs to maintain orc count

```go
// internal/aienv/orc_env.go
package aienv

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/config"
	"claude-pixel/internal/enemy"
	"claude-pixel/internal/player"
	"claude-pixel/internal/score"
	"claude-pixel/internal/spawner"
	"claude-pixel/internal/stamina"
	"claude-pixel/internal/storage"
	"claude-pixel/internal/world"
)

type OrcEnvConfig struct {
	Seed int64
}

type OrcStepResult struct {
	PlayerObs  []float64
	OrcObs     [][]float64
	OrcRewards []float64
	OrcDones   []bool
	Done       bool
	Info       map[string]any
}

type orcPrevState struct {
	lives int
	dist  float64
}

type OrcTrainEnv struct {
	cfg          *config.Config
	w            *world.World
	p            *player.Player
	enemies      []*enemy.Enemy
	sp           *spawner.Spawner
	sc           *score.Counter
	combatTuning *combat.Tuning
	orcKind      *enemy.Kind
	physics      *player.Physics
	staminaT     *player.StaminaTuning
	spawnTuning  *enemy.SpawnTuning
	timeoutS     float64
	elapsedS     float64
	rng          *rand.Rand
	anims        map[string]*anim.Animation
	soldierBoxes map[string]combat.Box
	prevPlayerLives int
	prevOrcStates   map[*enemy.Enemy]*orcPrevState
	stepsSinceHit   int
}

func NewOrcTrainEnv(ecfg OrcEnvConfig) (*OrcTrainEnv, error) {
	cfg := config.Load()
	db := storage.MustOpen(cfg)
	defer db.Close()

	animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
	hitboxRepo := storage.NewRepository[combat.HitboxSpec](db, combat.HitboxMapper{})

	animSpecs, err := animRepo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load anim specs: %w", err)
	}
	anims := buildHeadlessAnims(animSpecs)

	physics, err := player.LoadPhysics(tuneRepo)
	if err != nil {
		return nil, fmt.Errorf("load physics: %w", err)
	}
	staminaT, err := player.LoadStaminaTuning(tuneRepo)
	if err != nil {
		return nil, fmt.Errorf("load stamina: %w", err)
	}
	tuneParams, err := tuneRepo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list tuning: %w", err)
	}
	tuneMap := make(map[string]float64, len(tuneParams))
	for _, p := range tuneParams {
		tuneMap[p.Key] = p.Value
	}
	combatTuning, err := combat.LoadTuning(tuneMap)
	if err != nil {
		return nil, fmt.Errorf("load combat tuning: %w", err)
	}
	spawnTuning, err := enemy.LoadSpawnTuning(tuneRepo)
	if err != nil {
		return nil, fmt.Errorf("load spawn tuning: %w", err)
	}
	hitboxSpecs, err := hitboxRepo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list hitboxes: %w", err)
	}
	soldierBoxes, err := combat.SoldierBoxes(hitboxSpecs, cfg.RenderScale)
	if err != nil {
		return nil, fmt.Errorf("load soldier boxes: %w", err)
	}
	orcKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "orc", Prefix: "orc", FrameW: 100, FrameH: 100,
		AnimLib: anims, HitboxSpecs: hitboxSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
		BehaviorPath: cfg.AssetsDir + "/behaviors/orc.json",
	})
	if err != nil {
		return nil, fmt.Errorf("build orc kind: %w", err)
	}

	seed := ecfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &OrcTrainEnv{
		cfg:          cfg,
		physics:      physics,
		staminaT:     staminaT,
		combatTuning: combatTuning,
		spawnTuning:  spawnTuning,
		orcKind:      orcKind,
		anims:        anims,
		soldierBoxes: soldierBoxes,
		timeoutS:     tuneMap["game_timeout_s"],
		rng:          rand.New(rand.NewSource(seed)),
	}, nil
}

func (env *OrcTrainEnv) Reset() OrcStepResult {
	pool := stamina.NewPool(env.staminaT.Max, env.staminaT.DrainPerSec, env.staminaT.RegenPerSec)
	env.w = world.New(env.cfg, env.physics.Gravity)
	env.p = player.New(player.Config{
		StartX:     float64(env.cfg.WindowW) / 2,
		StartY:     env.w.GroundY,
		Physics:    env.physics,
		Anims:      env.anims,
		Boxes:      env.soldierBoxes,
		StartLives: env.combatTuning.SoldierMaxLives,
		Stamina:    pool,
	})
	env.p.Grounded = true
	env.sc = &score.Counter{}
	env.elapsedS = 0
	env.prevPlayerLives = env.combatTuning.SoldierMaxLives
	env.stepsSinceHit = 0

	env.enemies = nil
	env.prevOrcStates = make(map[*enemy.Enemy]*orcPrevState)

	kindFactories := []spawner.KindFactory{{
		Name: env.orcKind.Name, Weight: 1,
		NewEnemy: func(x, _ float64) *enemy.Enemy {
			halfW := float64(env.orcKind.Boxes["body"].W) / 2
			spriteH := float64(env.orcKind.FrameH * env.cfg.RenderScale)
			if x < halfW { x = halfW }
			if maxX := float64(env.cfg.WindowW) - halfW; x > maxX { x = maxX }
			return enemy.New(enemy.Config{
				StartX: x, StartY: -spriteH,
				Physics: env.physics, Kind: env.orcKind, RNG: env.rng,
			})
		},
	}}

	env.sp = spawner.New(spawner.Config{
		MinIntervalS: env.spawnTuning.MinS,
		MaxIntervalS: env.spawnTuning.MaxS,
		MaxAlive:     env.spawnTuning.MaxAlive,
		SpawnXMin:    0,
		SpawnXMax:    float64(env.cfg.WindowW),
		RNG:          env.rng,
		Kinds:        kindFactories,
	})

	// Force-spawn first orc immediately
	firstOrc := kindFactories[0].NewEnemy(float64(env.cfg.WindowW)/4, 0)
	firstOrc.Y = env.w.GroundY
	firstOrc.Grounded = true
	env.enemies = append(env.enemies, firstOrc)
	env.prevOrcStates[firstOrc] = &orcPrevState{
		lives: firstOrc.Lives,
		dist:  math.Abs(firstOrc.X - env.p.X),
	}

	return env.buildResult(nil, nil)
}

func (env *OrcTrainEnv) Step(playerAction int, orcActions []int) OrcStepResult {
	dt := fixedDT
	intent := ToIntent(playerAction)

	// 1. Player stamina + FSM
	if env.p.Stamina != nil {
		env.p.Stamina.Update(dt, env.p.IsSprinting(intent))
	}
	env.p.FSM.Handle(env.p, intent, dt)

	// 2. Apply orc RL actions (replace enemy.Tick)
	for i, e := range env.enemies {
		if e.Dead || e.CurrentState == "death" || e.CurrentState == "hurt" {
			// Let FSM-driven states play out (anim timing)
			e.Tick(dt)
			continue
		}
		if e.CurrentState == "fall" {
			e.Tick(dt)
			continue
		}
		if e.CurrentState == "attack" || e.CurrentState == "attack2" {
			// Let attack animation play out, don't interrupt
			e.Tick(dt)
			continue
		}

		// In "run" state: apply RL action
		action := 0
		if i < len(orcActions) {
			action = orcActions[i]
		}
		env.applyOrcAction(e, action)

		// Still need anim update for run state
		if e.Current != nil {
			e.Current.Update(dt)
		}
	}

	// Handle event transitions for orcs (hit/death bypass)
	for _, e := range env.enemies {
		if e.Lives <= 0 && e.CurrentState != "death" {
			env.orcTransition(e, "death")
		}
		if e.OnHitPending {
			e.OnHitPending = false
			env.orcTransition(e, "hurt")
		}
	}

	// 3. Physics
	env.p.ApplyPhysics(env.w, dt)
	for _, e := range env.enemies {
		e.ApplyPhysics(env.w, dt)
	}

	// 4. Clamp positions
	soldierBodyHalfW := float64(env.p.Boxes["body"].W) / 2
	env.p.X = world.Clamp(env.p.X, soldierBodyHalfW, float64(env.cfg.WindowW)-soldierBodyHalfW)
	for _, e := range env.enemies {
		bodyHalfW := float64(e.Kind.Boxes["body"].W) / 2
		e.X = world.Clamp(e.X, bodyHalfW, float64(env.cfg.WindowW)-bodyHalfW)
	}

	// 5. Player anim update
	if env.p.Current != nil {
		env.p.Current.Update(dt)
	}

	// 6. Spawner
	if spawned := env.sp.Tick(dt, len(env.enemies)); spawned != nil {
		env.enemies = append(env.enemies, spawned)
		env.prevOrcStates[spawned] = &orcPrevState{
			lives: spawned.Lives,
			dist:  math.Abs(spawned.X - env.p.X),
		}
	}

	// 7. Combat resolution
	orcHitsOnPlayer := env.dispatchOrcHits()
	playerHitsOnOrcs := env.dispatchPlayerHits()
	_ = playerHitsOnOrcs

	// 8. Compute per-orc rewards BEFORE cleanup
	orcRewards := make([]float64, len(env.enemies))
	orcDones := make([]bool, len(env.enemies))
	playerAttacking := env.p.CurrentAnim == "soldier_attack" || env.p.CurrentAnim == "soldier_attack2"

	for i, e := range env.enemies {
		prev, ok := env.prevOrcStates[e]
		if !ok {
			prev = &orcPrevState{lives: int(env.orcKind.Tuning.MaxLives)}
			env.prevOrcStates[e] = prev
		}
		livesLost := prev.lives - e.Lives
		currDist := math.Abs(e.X - env.p.X)
		distDelta := currDist - prev.dist

		hitsLanded := 0
		for _, ev := range orcHitsOnPlayer {
			if ev == e {
				hitsLanded++
			}
		}

		dodged := false
		if playerAttacking && distDelta > 0 && livesLost == 0 {
			dodged = true
		}

		if hitsLanded > 0 {
			env.stepsSinceHit = 0
		} else {
			env.stepsSinceHit++
		}

		playerDied := env.p.FSM.CurrentID() == player.StateDeath

		orcRewards[i] = OrcCalcReward(OrcRewardInput{
			HitPlayer:    hitsLanded,
			PlayerDied:   playerDied,
			OrcLivesLost: livesLost,
			OrcDied:      e.Dead || e.CurrentState == "death",
			DodgeSuccess: dodged,
			Stagnant:     env.stepsSinceHit > 180,
			DistDelta:    distDelta,
		})

		orcDones[i] = e.Dead || e.CurrentState == "death"
		prev.lives = e.Lives
		prev.dist = currDist
	}

	// 9. Clean up dead enemies
	alive := env.enemies[:0]
	for _, e := range env.enemies {
		if e.Dead {
			delete(env.prevOrcStates, e)
			continue
		}
		alive = append(alive, e)
	}
	env.enemies = alive

	// 10. Time + termination
	env.elapsedS += dt.Seconds()
	playerDied := env.p.FSM.CurrentID() == player.StateDeath && env.p.Current != nil && env.p.Current.Done()
	timedOut := env.timeoutS > 0 && env.elapsedS >= env.timeoutS
	done := playerDied || timedOut

	env.prevPlayerLives = env.p.Lives

	return env.buildResult(orcRewards, orcDones)
}

func (env *OrcTrainEnv) applyOrcAction(e *enemy.Enemy, action int) {
	ar := OrcAction(action)

	if ar.Transition != "" && e.CurrentState == "run" {
		env.orcTransition(e, ar.Transition)
		return
	}

	if ar.Flip {
		e.Facing = -e.Facing
		e.VX = 0
		return
	}

	speed := 120.0 // orc run speed (matches behavior json)
	switch ar.VXMode {
	case OrcVXStop:
		e.VX = 0
	case OrcVXToward:
		if env.p.X > e.X {
			e.Facing = 1
		} else {
			e.Facing = -1
		}
		e.VX = float64(e.Facing) * speed
	case OrcVXAway:
		if env.p.X > e.X {
			e.Facing = -1
		} else {
			e.Facing = 1
		}
		e.VX = float64(e.Facing) * speed
	}
}

func (env *OrcTrainEnv) orcTransition(e *enemy.Enemy, to string) {
	e.CurrentState = to
	e.HitSet = map[combat.Fighter]bool{}
	e.VX = 0
	if anim, ok := e.Kind.Anims[to]; ok {
		anim.Reset()
		e.Current = anim
		e.CurrentAnim = to
	}
}

func (env *OrcTrainEnv) dispatchOrcHits() []*enemy.Enemy {
	attackers := make([]combat.Fighter, 0, len(env.enemies))
	for _, e := range env.enemies {
		attackers = append(attackers, e)
	}
	victims := []combat.Fighter{env.p}
	hits := combat.Resolve(attackers, victims)
	var hitOrcs []*enemy.Enemy
	for _, ev := range hits {
		orc := ev.Attacker.(*enemy.Enemy)
		env.p.OnHit(env.combatTuning.SoldierKnockbackVX, env.combatTuning.SoldierKnockbackVY, orc.X)
		hitOrcs = append(hitOrcs, orc)
	}
	return hitOrcs
}

func (env *OrcTrainEnv) dispatchPlayerHits() int {
	attackers := []combat.Fighter{env.p}
	victims := make([]combat.Fighter, 0, len(env.enemies))
	for _, e := range env.enemies {
		victims = append(victims, e)
	}
	hits := combat.Resolve(attackers, victims)
	for _, ev := range hits {
		orc := ev.Victim.(*enemy.Enemy)
		orc.OnHit(env.p.X)
	}
	return len(hits)
}

func (env *OrcTrainEnv) buildResult(orcRewards []float64, orcDones []bool) OrcStepResult {
	// Player obs
	playerObs := env.playerObserve()

	// Per-orc obs
	orcObs := make([][]float64, len(env.enemies))
	for i, e := range env.enemies {
		obs := env.orcObserve(e)
		orcObs[i] = obs[:]
	}

	if orcRewards == nil {
		orcRewards = make([]float64, len(env.enemies))
	}
	if orcDones == nil {
		orcDones = make([]bool, len(env.enemies))
	}

	playerDied := env.p.FSM.CurrentID() == player.StateDeath
	timedOut := env.timeoutS > 0 && env.elapsedS >= env.timeoutS

	return OrcStepResult{
		PlayerObs:  playerObs[:],
		OrcObs:     orcObs,
		OrcRewards: orcRewards,
		OrcDones:   orcDones,
		Done:       playerDied || timedOut,
		Info: map[string]any{
			"player_lives": env.p.Lives,
			"orc_count":    len(env.enemies),
			"elapsed":      env.elapsedS,
		},
	}
}

func (env *OrcTrainEnv) playerObserve() [ObsSize]float64 {
	enemies := make([]EnemyState, 0, len(env.enemies))
	for _, e := range env.enemies {
		stateIdx := 0
		attacking := false
		switch e.CurrentState {
		case "run":     stateIdx = 1
		case "attack":  stateIdx = 2; attacking = true
		case "attack2": stateIdx = 3; attacking = true
		case "hurt":    stateIdx = 4
		case "death":   stateIdx = 5
		}
		enemies = append(enemies, EnemyState{
			RelX: e.X - env.p.X, RelY: e.Y - env.p.Y,
			Lives: e.Lives, MaxLives: int(env.orcKind.Tuning.MaxLives),
			State: stateIdx, Attacking: attacking,
		})
	}
	stateIdx := 0
	switch env.p.FSM.CurrentID() {
	case player.StateRun:     stateIdx = 1
	case player.StateJump:    stateIdx = 2
	case player.StateFall:    stateIdx = 3
	case player.StateAttack:  stateIdx = 4
	case player.StateAttack2: stateIdx = 5
	case player.StateHit:     stateIdx = 6
	case player.StateDeath:   stateIdx = 7
	}
	staminaFrac := 0.0
	if env.p.Stamina != nil {
		staminaFrac = env.p.Stamina.Fraction()
	}
	return Observe(GameState{
		PlayerX: env.p.X, PlayerY: env.p.Y,
		PlayerVX: env.p.VX, PlayerVY: env.p.VY,
		Facing: env.p.Facing, Grounded: env.p.Grounded,
		Lives: env.p.Lives, MaxLives: env.combatTuning.SoldierMaxLives,
		Stamina: staminaFrac, StateID: stateIdx, NumStates: 8,
		TimeoutS: env.timeoutS, ElapsedS: env.elapsedS,
		Enemies: enemies, MaxAlive: env.spawnTuning.MaxAlive,
		Score: env.sc.Total(),
		MaxSpeed: env.physics.SprintSpeed, MaxFall: env.physics.MaxFallSpeed,
		WindowW: float64(env.cfg.WindowW), WindowH: float64(env.cfg.WindowH),
	})
}

func (env *OrcTrainEnv) orcObserve(e *enemy.Enemy) [OrcObsSize]float64 {
	orcStateIdx := 0
	switch e.CurrentState {
	case "run":     orcStateIdx = 1
	case "attack":  orcStateIdx = 2
	case "attack2": orcStateIdx = 3
	case "hurt":    orcStateIdx = 4
	case "death":   orcStateIdx = 5
	}
	playerStateIdx := 0
	switch env.p.FSM.CurrentID() {
	case player.StateRun:     playerStateIdx = 1
	case player.StateJump:    playerStateIdx = 2
	case player.StateFall:    playerStateIdx = 3
	case player.StateAttack:  playerStateIdx = 4
	case player.StateAttack2: playerStateIdx = 5
	case player.StateHit:     playerStateIdx = 6
	case player.StateDeath:   playerStateIdx = 7
	}
	playerAttacking := env.p.CurrentAnim == "soldier_attack" || env.p.CurrentAnim == "soldier_attack2"

	return OrcObserve(OrcGameState{
		OrcX: e.X, OrcY: e.Y,
		OrcVX: e.VX, OrcVY: e.VY,
		OrcFacing: e.Facing, OrcGrounded: e.Grounded,
		OrcLives: e.Lives, OrcMaxLives: int(env.orcKind.Tuning.MaxLives),
		OrcStateID: orcStateIdx, OrcNumStates: 6,
		PlayerX: env.p.X, PlayerY: env.p.Y,
		PlayerLives: env.p.Lives, PlayerMaxLives: env.combatTuning.SoldierMaxLives,
		PlayerStateID: playerStateIdx, PlayerNumStates: 8,
		PlayerAttacking: playerAttacking, PlayerFacing: env.p.Facing,
		OrcMaxSpeed: 120, OrcMaxFall: env.physics.MaxFallSpeed,
		TimeoutS: env.timeoutS, ElapsedS: env.elapsedS,
		WindowW: float64(env.cfg.WindowW), WindowH: float64(env.cfg.WindowH),
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/aienv/ -v -run TestOrcEnv -timeout 30s`
Expected: all PASS

Note: Tests require `.env` and `data/game.db`. If DB doesn't exist, run `make run` briefly then Ctrl+C.

- [ ] **Step 5: Commit**

```bash
git add internal/aienv/orc_env.go internal/aienv/orc_env_test.go
git commit -m "feat(ai): add OrcTrainEnv multi-agent headless environment"
```

---

## Task 5: Orc Training Server + Protocol

**Files:**
- Create: `cmd/train-orc/main.go`
- Create: `cmd/train-orc/protocol.go`
- Create: `cmd/train-orc/protocol_test.go`

Separate binary from `cmd/train`. Uses multi-agent protocol.

- [ ] **Step 1: Write protocol tests**

```go
// cmd/train-orc/protocol_test.go
package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeOrcObsMsg(t *testing.T) {
	msg := OrcObsMsg{
		Type:       "obs",
		PlayerObs:  []float64{0.5, 0.3},
		OrcObs:     [][]float64{{0.1, 0.2}, {0.3, 0.4}},
		OrcRewards: []float64{1.0, -0.5},
		OrcDones:   []bool{false, true},
		Done:       false,
	}
	var buf bytes.Buffer
	if err := writeOrcMsg(&buf, msg); err != nil {
		t.Fatalf("writeOrcMsg: %v", err)
	}
	line := buf.String()
	if line[len(line)-1] != '\n' {
		t.Error("should end with newline")
	}
	var decoded OrcObsMsg
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.OrcObs) != 2 {
		t.Errorf("orc obs count = %d, want 2", len(decoded.OrcObs))
	}
}

func TestDecodeOrcActionMsg(t *testing.T) {
	input := `{"type":"action","player_action":2,"orc_actions":[1,3]}` + "\n"
	msg, err := readOrcMsg(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("readOrcMsg: %v", err)
	}
	if msg.PlayerAction != 2 {
		t.Errorf("player_action = %d, want 2", msg.PlayerAction)
	}
	if len(msg.OrcActions) != 2 {
		t.Errorf("orc_actions len = %d, want 2", len(msg.OrcActions))
	}
}
```

- [ ] **Step 2: Implement protocol**

```go
// cmd/train-orc/protocol.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

type OrcObsMsg struct {
	Type       string         `json:"type"`
	PlayerObs  []float64      `json:"player_obs"`
	OrcObs     [][]float64    `json:"orc_obs"`
	OrcRewards []float64      `json:"orc_rewards"`
	OrcDones   []bool         `json:"orc_dones"`
	Done       bool           `json:"done"`
	Info       map[string]any `json:"info,omitempty"`
}

type OrcClientMsg struct {
	Type         string `json:"type"`
	PlayerAction int    `json:"player_action,omitempty"`
	OrcActions   []int  `json:"orc_actions,omitempty"`
}

func writeOrcMsg(w io.Writer, msg OrcObsMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func readOrcMsg(r io.Reader) (OrcClientMsg, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return OrcClientMsg{}, err
		}
		return OrcClientMsg{}, io.EOF
	}
	var msg OrcClientMsg
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		return OrcClientMsg{}, err
	}
	return msg, nil
}
```

- [ ] **Step 3: Implement training server**

```go
// cmd/train-orc/main.go
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"claude-pixel/internal/aienv"
)

func main() {
	port := flag.Int("port", 9876, "listen port")
	flag.Parse()

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("listen on port %d: %v", *port, err)
	}
	log.Printf("orc training server listening on :%d", *port)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("shutting down...")
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			return
		}
		log.Println("client connected")
		handleOrcConn(conn)
		conn.Close()
		log.Println("client disconnected")
	}
}

func handleOrcConn(conn net.Conn) {
	env, err := aienv.NewOrcTrainEnv(aienv.OrcEnvConfig{Seed: 1})
	if err != nil {
		log.Printf("create OrcTrainEnv: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("read: %v", err)
			}
			return
		}

		var msg OrcClientMsg
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("unmarshal: %v", err)
			continue
		}

		var resp OrcObsMsg
		switch msg.Type {
		case "reset":
			result := env.Reset()
			resp = OrcObsMsg{
				Type: "obs", PlayerObs: result.PlayerObs,
				OrcObs: result.OrcObs, OrcRewards: result.OrcRewards,
				OrcDones: result.OrcDones, Done: result.Done, Info: result.Info,
			}
		case "action":
			result := env.Step(msg.PlayerAction, msg.OrcActions)
			resp = OrcObsMsg{
				Type: "obs", PlayerObs: result.PlayerObs,
				OrcObs: result.OrcObs, OrcRewards: result.OrcRewards,
				OrcDones: result.OrcDones, Done: result.Done, Info: result.Info,
			}
		default:
			log.Printf("unknown message type: %q", msg.Type)
			continue
		}

		data, _ := json.Marshal(resp)
		writer.Write(data)
		writer.WriteByte('\n')
		writer.Flush()
	}
}
```

- [ ] **Step 4: Run tests and verify compilation**

Run: `go test ./cmd/train-orc/ -v && go build ./cmd/train-orc/`
Expected: all PASS, build succeeds

- [ ] **Step 5: Commit**

```bash
git add cmd/train-orc/
git commit -m "feat(ai): add orc training server with multi-agent protocol"
```

---

## Task 6: Python Orc Training Pipeline

**Files:**
- Create: `ai/orc_config.py`
- Create: `ai/orc_env.py`
- Create: `ai/train_orc.py`

Python side manages both player (fixed model) and orc (training) agents.

- [ ] **Step 1: Create orc_config.py**

```python
# ai/orc_config.py

ORC_OBS_SIZE = 16
ORC_NUM_ACTIONS = 6

ORC_PPO_PARAMS = dict(
    learning_rate=3e-4,
    n_steps=2048,
    batch_size=64,
    n_epochs=10,
    gamma=0.99,
    gae_lambda=0.95,
    clip_range=0.2,
    ent_coef=0.01,
    vf_coef=0.5,
    max_grad_norm=0.5,
    policy_kwargs=dict(
        net_arch=[128, 128],
    ),
)

ORC_CHECKPOINT_INTERVAL = 50_000
ORC_BASE_PORT = 9876
```

- [ ] **Step 2: Create orc_env.py**

The key challenge: multiple orcs per game tick but SB3 expects single-agent env. Solution: `OrcGymEnv` manages one game connection and exposes the first alive orc as the "agent". When it dies, the env auto-resets.

```python
# ai/orc_env.py
import json
import socket

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from orc_config import ORC_OBS_SIZE, ORC_NUM_ACTIONS
from config import OBS_SIZE, NUM_ACTIONS


class OrcGymEnv(gym.Env):
    """Multi-agent env for orc training. Manages player (fixed) + orcs."""

    metadata = {"render_modes": []}

    def __init__(self, player_model=None, host="localhost", port=9876):
        super().__init__()
        self.host = host
        self.port = port
        self.player_model = player_model
        self.action_space = spaces.Discrete(ORC_NUM_ACTIONS)
        self.observation_space = spaces.Box(
            low=0.0, high=1.0, shape=(ORC_OBS_SIZE,), dtype=np.float32
        )
        self._sock = None
        self._rfile = None
        self._player_obs = None

    def _connect(self):
        if self._sock is not None:
            return
        self._sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._sock.connect((self.host, self.port))
        self._rfile = self._sock.makefile("r")

    def _send(self, msg: dict):
        data = json.dumps(msg) + "\n"
        self._sock.sendall(data.encode())

    def _recv(self) -> dict:
        line = self._rfile.readline()
        if not line:
            raise ConnectionError("Server closed connection")
        return json.loads(line)

    def _get_player_action(self, player_obs):
        if self.player_model is None:
            return 0
        obs = np.array(player_obs, dtype=np.float32)
        action, _ = self.player_model.predict(obs, deterministic=False)
        return int(action)

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        self._connect()
        self._send({"type": "reset"})
        resp = self._recv()
        self._player_obs = resp["player_obs"]
        orc_obs = resp["orc_obs"]
        if len(orc_obs) == 0:
            return np.zeros(ORC_OBS_SIZE, dtype=np.float32), {}
        return np.array(orc_obs[0], dtype=np.float32), {}

    def step(self, action: int):
        player_action = self._get_player_action(self._player_obs)

        orc_obs_count = max(1, len(self._player_obs) if hasattr(self, '_last_orc_count') else 1)
        if hasattr(self, '_last_orc_count'):
            orc_obs_count = self._last_orc_count

        orc_actions = [int(action)] * orc_obs_count

        self._send({
            "type": "action",
            "player_action": player_action,
            "orc_actions": orc_actions,
        })
        resp = self._recv()
        self._player_obs = resp["player_obs"]
        orc_obs_list = resp["orc_obs"]
        orc_rewards = resp["orc_rewards"]
        orc_dones = resp["orc_dones"]
        done = resp["done"]

        self._last_orc_count = len(orc_obs_list)

        if len(orc_obs_list) == 0:
            obs = np.zeros(ORC_OBS_SIZE, dtype=np.float32)
            reward = 0.0
            terminated = done
        else:
            obs = np.array(orc_obs_list[0], dtype=np.float32)
            reward = float(orc_rewards[0]) if orc_rewards else 0.0
            terminated = done or (orc_dones[0] if orc_dones else False)

        if terminated:
            obs, _ = self.reset()
            return obs, reward, False, False, resp.get("info", {})

        return obs, reward, terminated, False, resp.get("info", {})

    def close(self):
        if self._sock is not None:
            self._sock.close()
            self._sock = None
            self._rfile = None
```

- [ ] **Step 3: Create train_orc.py**

```python
# ai/train_orc.py
import argparse
import os
import signal

from stable_baselines3 import PPO
from stable_baselines3.common.callbacks import CheckpointCallback, BaseCallback
from stable_baselines3.common.vec_env import DummyVecEnv

from orc_config import ORC_PPO_PARAMS, ORC_CHECKPOINT_INTERVAL, ORC_BASE_PORT
from orc_env import OrcGymEnv


class GracefulShutdown(BaseCallback):
    def __init__(self, save_path: str):
        super().__init__()
        self.save_path = save_path
        self._shutdown = False
        signal.signal(signal.SIGINT, self._handler)

    def _handler(self, signum, frame):
        print("\nGraceful shutdown — saving checkpoint...")
        self._shutdown = True

    def _on_step(self) -> bool:
        if self._shutdown:
            path = os.path.join(self.save_path, "orc_interrupted")
            self.model.save(path)
            print(f"Saved to {path}.zip")
            return False
        return True


def main():
    parser = argparse.ArgumentParser(description="Train orc RL agent")
    parser.add_argument("--timesteps", type=int, default=500_000)
    parser.add_argument("--player-model", type=str, default="checkpoints/ppo_final.zip",
                        help="Path to trained player model")
    parser.add_argument("--resume", type=str, default=None)
    args = parser.parse_args()

    os.makedirs("checkpoints", exist_ok=True)
    os.makedirs("logs", exist_ok=True)

    player_model = None
    if os.path.exists(args.player_model):
        print(f"Loading player model from {args.player_model}")
        player_model = PPO.load(args.player_model)
    else:
        print(f"WARNING: Player model not found at {args.player_model}, using idle player")

    vec_env = None
    try:
        vec_env = DummyVecEnv([lambda: OrcGymEnv(player_model=player_model, port=ORC_BASE_PORT)])

        if args.resume:
            print(f"Resuming from {args.resume}")
            model = PPO.load(args.resume, env=vec_env, tensorboard_log="logs/")
        else:
            model = PPO(
                "MlpPolicy",
                vec_env,
                verbose=1,
                tensorboard_log="logs/",
                **ORC_PPO_PARAMS,
            )

        checkpoint_cb = CheckpointCallback(
            save_freq=ORC_CHECKPOINT_INTERVAL,
            save_path="checkpoints/",
            name_prefix="orc",
        )
        shutdown_cb = GracefulShutdown("checkpoints/")

        print(f"Training orc for {args.timesteps} timesteps...")
        model.learn(
            total_timesteps=args.timesteps,
            callback=[checkpoint_cb, shutdown_cb],
            reset_num_timesteps=args.resume is None,
        )

        model.save("checkpoints/orc_final")
        print("Orc training complete. Saved to checkpoints/orc_final.zip")

    finally:
        if vec_env is not None:
            vec_env.close()


if __name__ == "__main__":
    main()
```

- [ ] **Step 4: Verify Python syntax**

Run: `python3 -c "import ast; [ast.parse(open(f'ai/{f}').read()) for f in ('orc_config.py','orc_env.py','train_orc.py')]; print('OK')"`
Expected: `OK`

- [ ] **Step 5: Commit**

```bash
git add ai/orc_config.py ai/orc_env.py ai/train_orc.py
git commit -m "feat(ai): add Python orc training pipeline with self-play"
```

---

## Task 7: Makefile Targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add orc training targets**

Append to Makefile after existing AI targets:

```makefile

# === Orc AI Training ===

.PHONY: train-orc-server train-orc train-orc-visual

train-orc-server:      ## Start headless server for orc RL training
	go run ./cmd/train-orc -port=9876

train-orc:             ## Train orc RL agent vs player model (STEPS=500000 default)
	cd ai && python3 train_orc.py --timesteps=$(or $(STEPS),500000) --player-model=$(or $(PLAYER_MODEL),checkpoints/ppo_final.zip)

train-orc-visual:      ## Train orc with game window visible (STEPS=50000 default)
	@echo "Training orc with visual — game window will open..."
	go run ./cmd/game -ai-orc 9876 &
	@sleep 2
	cd ai && python3 train_orc.py --timesteps=$(or $(STEPS),50000) --player-model=$(or $(PLAYER_MODEL),checkpoints/ppo_final.zip)
```

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "feat(ai): add orc training Makefile targets"
```

---

## Task 8: Integration Test

End-to-end smoke test.

- [ ] **Step 1: Run full integration**

Terminal 1:
```bash
make train-orc-server
```

Terminal 2:
```bash
make train-orc STEPS=8192
```

Expected:
- Server prints "orc training server listening on :9876"
- Python connects, trains 4 rollouts of 2048 steps
- Checkpoint saved, no crashes

- [ ] **Step 2: Commit any fixes**

```bash
git add -A
git commit -m "fix(ai): integration fixes from orc training test"
```

---

## Summary

| Task | Component | Est. Time |
|------|-----------|-----------|
| 1 | Orc action mapping | 5 min |
| 2 | Orc observation vector | 10 min |
| 3 | Orc reward function | 10 min |
| 4 | OrcTrainEnv (multi-agent) | 40 min |
| 5 | TCP server + protocol | 15 min |
| 6 | Python training pipeline | 15 min |
| 7 | Makefile targets | 5 min |
| 8 | Integration test | 15 min |
| **Total** | | **~2 hours** |

After completing all tasks:
```bash
# Terminal 1
make train-orc-server

# Terminal 2
make train-orc STEPS=500000

# Visual training
make train-orc-visual STEPS=50000
```
