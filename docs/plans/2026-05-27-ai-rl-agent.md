# AI RL Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Train a PPO-based RL agent to autonomously control the soldier, kill enemies, and maximize score within game timeout.

**Architecture:** Go headless game server exposes Step/Reset via TCP socket (JSON-line protocol). Python side wraps it as a Gymnasium env and trains with Stable-Baselines3 PPO. Headless env reuses existing `internal/` game logic packages without any Ebiten rendering dependency.

**Tech Stack:** Go (headless game env + TCP server), Python 3.10+ (PyTorch, Stable-Baselines3, Gymnasium)

**Design Spec:** `docs/superpowers/specs/2026-05-27-ai-rl-agent-design.md`

---

## File Structure

### New Go files

| File | Responsibility |
|------|----------------|
| `internal/aienv/action.go` | Action ID → `input.Intent` mapping (10 discrete actions) |
| `internal/aienv/action_test.go` | Tests for action mapping |
| `internal/aienv/observation.go` | Extract + normalize 25-float observation vector from game state |
| `internal/aienv/observation_test.go` | Tests for observation extraction |
| `internal/aienv/reward.go` | Per-step reward calculation with shaped rewards |
| `internal/aienv/reward_test.go` | Tests for reward function |
| `internal/aienv/game_env.go` | Headless GameEnv: Step, Reset, wiring player/enemy/spawner/combat |
| `internal/aienv/game_env_test.go` | Integration tests for Step/Reset cycle |
| `cmd/train/main.go` | TCP server entry point, N parallel envs |
| `cmd/train/protocol.go` | JSON-line message encode/decode |
| `cmd/train/protocol_test.go` | Tests for protocol serialization |

### New Python files

| File | Responsibility |
|------|----------------|
| `ai/requirements.txt` | Python dependencies |
| `ai/config.py` | Hyperparameters, curriculum config |
| `ai/env.py` | Gymnasium wrapper (TCP socket client) |
| `ai/train.py` | SB3 PPO training entry point with checkpointing |
| `ai/eval.py` | Evaluate trained model over N episodes |
| `ai/play.py` | Watch AI play real game via socket |

### Modified files

| File | Change |
|------|--------|
| `Makefile` | Add AI training targets |
| `.gitignore` | Add `ai/checkpoints/`, `ai/logs/` |

---

## Task 1: Action Mapping

**Files:**
- Create: `internal/aienv/action.go`
- Create: `internal/aienv/action_test.go`

Maps integer action IDs (0–9) to `input.Intent` structs. This is the decoder that converts agent outputs into game inputs.

- [ ] **Step 1: Write failing tests for action mapping**

```go
// internal/aienv/action_test.go
package aienv

import (
	"testing"

	"claude-pixel/internal/input"
)

func TestToIntent_Idle(t *testing.T) {
	intent := ToIntent(0)
	want := input.Intent{}
	if intent != want {
		t.Errorf("action 0: got %+v, want %+v", intent, want)
	}
}

func TestToIntent_MoveLeft(t *testing.T) {
	intent := ToIntent(1)
	if !intent.Left || intent.Right {
		t.Errorf("action 1: got Left=%v Right=%v", intent.Left, intent.Right)
	}
}

func TestToIntent_MoveRight(t *testing.T) {
	intent := ToIntent(2)
	if intent.Left || !intent.Right {
		t.Errorf("action 2: got Left=%v Right=%v", intent.Left, intent.Right)
	}
}

func TestToIntent_Jump(t *testing.T) {
	intent := ToIntent(3)
	if !intent.JumpPressed {
		t.Errorf("action 3: JumpPressed should be true")
	}
}

func TestToIntent_MoveLeftJump(t *testing.T) {
	intent := ToIntent(4)
	if !intent.Left || !intent.JumpPressed {
		t.Errorf("action 4: Left=%v JumpPressed=%v", intent.Left, intent.JumpPressed)
	}
}

func TestToIntent_MoveRightJump(t *testing.T) {
	intent := ToIntent(5)
	if !intent.Right || !intent.JumpPressed {
		t.Errorf("action 5: Right=%v JumpPressed=%v", intent.Right, intent.JumpPressed)
	}
}

func TestToIntent_Attack1(t *testing.T) {
	intent := ToIntent(6)
	if !intent.AttackPressed {
		t.Errorf("action 6: AttackPressed should be true")
	}
}

func TestToIntent_Attack2(t *testing.T) {
	intent := ToIntent(7)
	if !intent.Attack2Pressed {
		t.Errorf("action 7: Attack2Pressed should be true")
	}
}

func TestToIntent_SprintLeft(t *testing.T) {
	intent := ToIntent(8)
	if !intent.Left || !intent.SprintHeld {
		t.Errorf("action 8: Left=%v SprintHeld=%v", intent.Left, intent.SprintHeld)
	}
}

func TestToIntent_SprintRight(t *testing.T) {
	intent := ToIntent(9)
	if !intent.Right || !intent.SprintHeld {
		t.Errorf("action 9: Right=%v SprintHeld=%v", intent.Right, intent.SprintHeld)
	}
}

func TestToIntent_OutOfRange(t *testing.T) {
	intent := ToIntent(99)
	want := input.Intent{}
	if intent != want {
		t.Errorf("out of range action: got %+v, want idle", intent)
	}
}

func TestNumActions(t *testing.T) {
	if NumActions != 10 {
		t.Errorf("NumActions = %d, want 10", NumActions)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/aienv/ -v -run TestToIntent`
Expected: compilation error — package `aienv` does not exist

- [ ] **Step 3: Implement action mapping**

```go
// internal/aienv/action.go
package aienv

import "claude-pixel/internal/input"

const NumActions = 10

func ToIntent(action int) input.Intent {
	switch action {
	case 0:
		return input.Intent{}
	case 1:
		return input.Intent{Left: true}
	case 2:
		return input.Intent{Right: true}
	case 3:
		return input.Intent{JumpPressed: true}
	case 4:
		return input.Intent{Left: true, JumpPressed: true}
	case 5:
		return input.Intent{Right: true, JumpPressed: true}
	case 6:
		return input.Intent{AttackPressed: true}
	case 7:
		return input.Intent{Attack2Pressed: true}
	case 8:
		return input.Intent{Left: true, SprintHeld: true}
	case 9:
		return input.Intent{Right: true, SprintHeld: true}
	default:
		return input.Intent{}
	}
}
```

**Important:** The `input` package imports `ebiten` in `Poll()`, but `input.Intent` is a plain struct with no Ebiten dependency. Importing `input` for the struct only is safe — the headless env never calls `input.Poll()`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/aienv/ -v -run TestToIntent`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aienv/action.go internal/aienv/action_test.go
git commit -m "feat(ai): add action ID to Intent mapping for RL agent"
```

---

## Task 2: Observation Vector

**Files:**
- Create: `internal/aienv/observation.go`
- Create: `internal/aienv/observation_test.go`

Extracts a 25-float normalized observation vector from game state. Pure function — no side effects, no Ebiten dependency.

- [ ] **Step 1: Write failing tests for observation extraction**

```go
// internal/aienv/observation_test.go
package aienv

import (
	"math"
	"testing"
)

func TestObsSize(t *testing.T) {
	if ObsSize != 25 {
		t.Fatalf("ObsSize = %d, want 25", ObsSize)
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
	gs := GameState{WindowW: 1280, WindowH: 720, MaxAlive: 3}
	obs := Observe(gs)
	// enemy slots 10-18 should be zero
	for i := 10; i <= 18; i++ {
		if obs[i] != 0.0 {
			t.Errorf("obs[%d] = %f, want 0.0 (no enemies)", i, obs[i])
		}
	}
	if obs[19] != 0.0 {
		t.Errorf("num_enemies = %f, want 0.0", obs[19])
	}
}

func TestObserve_WithEnemy(t *testing.T) {
	gs := GameState{
		PlayerX:   640,
		PlayerY:   600,
		WindowW:   1280,
		WindowH:   720,
		MaxAlive:  3,
		MaxLives:  10,
		TimeoutS:  30,
		ElapsedS:  15,
		Enemies: []EnemyState{
			{RelX: 100, RelY: 0, Lives: 2, MaxLives: 2, State: 1, Attacking: false},
		},
	}
	obs := Observe(gs)
	// enemy1 rel_x = 100 / 1280 ≈ 0.078
	if obs[10] < 0.07 || obs[10] > 0.09 {
		t.Errorf("enemy1 rel_x = %f, want ~0.078", obs[10])
	}
	// num_enemies = 1/3
	if math.Abs(obs[19]-1.0/3.0) > 0.01 {
		t.Errorf("num_enemies = %f, want ~0.333", obs[19])
	}
	// time_remaining = 15/30 = 0.5
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/aienv/ -v -run TestObs`
Expected: compilation error — `Observe`, `GameState` not defined

- [ ] **Step 3: Implement observation extraction**

```go
// internal/aienv/observation.go
package aienv

import "math"

const ObsSize = 25

type EnemyState struct {
	RelX      float64
	RelY      float64
	Lives     int
	MaxLives  int
	State     int
	Attacking bool
}

type GameState struct {
	PlayerX, PlayerY   float64
	PlayerVX, PlayerVY float64
	Facing             int
	Grounded           bool
	Lives, MaxLives    int
	Stamina            float64
	StateID            int
	NumStates          int
	TimeoutS           float64
	ElapsedS           float64
	Enemies            []EnemyState
	MaxAlive           int
	Score              int
	MaxSpeed           float64
	MaxFall            float64
	WindowW, WindowH   float64
}

func Observe(gs GameState) [ObsSize]float64 {
	var obs [ObsSize]float64

	obs[0] = clamp01(gs.PlayerX / gs.WindowW)
	obs[1] = clamp01(gs.PlayerY / gs.WindowH)
	obs[2] = clamp01((gs.PlayerVX/safeDivisor(gs.MaxSpeed) + 1) / 2)
	obs[3] = clamp01((gs.PlayerVY/safeDivisor(gs.MaxFall) + 1) / 2)

	if gs.Facing >= 0 {
		obs[4] = 1
	}
	if gs.Grounded {
		obs[5] = 1
	}
	obs[6] = clamp01(float64(gs.Lives) / safeDivisor(float64(gs.MaxLives)))
	obs[7] = clamp01(gs.Stamina)

	obs[8] = clamp01(float64(gs.StateID) / safeDivisor(float64(gs.NumStates)))

	remaining := gs.TimeoutS - gs.ElapsedS
	if remaining < 0 {
		remaining = 0
	}
	obs[9] = clamp01(remaining / safeDivisor(gs.TimeoutS))

	sorted := sortByDistance(gs.Enemies)
	for i := 0; i < 3; i++ {
		base := 10 + i*3
		if i < len(sorted) {
			e := sorted[i]
			obs[base+0] = clamp01(math.Abs(e.RelX) / gs.WindowW)
			obs[base+1] = clamp01((e.RelY/gs.WindowH + 1) / 2)
			obs[base+2] = clamp01(float64(e.Lives) / safeDivisor(float64(e.MaxLives)))
		}
	}

	obs[19] = clamp01(float64(len(gs.Enemies)) / safeDivisor(float64(gs.MaxAlive)))

	maxScore := 1000.0
	obs[20] = clamp01(float64(gs.Score) / maxScore)

	if len(sorted) > 0 {
		nearest := sorted[0]
		diag := math.Sqrt(gs.WindowW*gs.WindowW + gs.WindowH*gs.WindowH)
		dist := math.Sqrt(nearest.RelX*nearest.RelX + nearest.RelY*nearest.RelY)
		obs[21] = clamp01(dist / diag)
		obs[22] = clamp01((math.Atan2(nearest.RelY, nearest.RelX)/math.Pi + 1) / 2)
		obs[23] = clamp01(float64(nearest.State) / 6.0)
		if nearest.Attacking {
			obs[24] = 1
		}
	}

	return obs
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	if math.IsNaN(v) {
		return 0
	}
	return v
}

func safeDivisor(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}

func sortByDistance(enemies []EnemyState) []EnemyState {
	if len(enemies) == 0 {
		return nil
	}
	sorted := make([]EnemyState, len(enemies))
	copy(sorted, enemies)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			di := sorted[j].RelX*sorted[j].RelX + sorted[j].RelY*sorted[j].RelY
			dj := sorted[j-1].RelX*sorted[j-1].RelX + sorted[j-1].RelY*sorted[j-1].RelY
			if di < dj {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			}
		}
	}
	return sorted
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/aienv/ -v -run TestObs`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aienv/observation.go internal/aienv/observation_test.go
git commit -m "feat(ai): add observation vector extraction for RL agent"
```

---

## Task 3: Reward Function

**Files:**
- Create: `internal/aienv/reward.go`
- Create: `internal/aienv/reward_test.go`

Computes per-step reward from state deltas. Pure function — takes before/after snapshots and events, returns reward float.

- [ ] **Step 1: Write failing tests**

```go
// internal/aienv/reward_test.go
package aienv

import (
	"math"
	"testing"
)

func TestReward_Survival(t *testing.T) {
	r := CalcReward(RewardInput{})
	if math.Abs(r-0.01) > 1e-6 {
		t.Errorf("idle step reward = %f, want 0.01", r)
	}
}

func TestReward_EnemyKilled(t *testing.T) {
	r := CalcReward(RewardInput{EnemyKilledPoints: 10})
	if r < 10 {
		t.Errorf("enemy killed reward = %f, want >= 10", r)
	}
}

func TestReward_LifeLost(t *testing.T) {
	r := CalcReward(RewardInput{LivesLost: 1})
	if r > -4 {
		t.Errorf("life lost reward = %f, want <= -4", r)
	}
}

func TestReward_Death(t *testing.T) {
	r := CalcReward(RewardInput{Died: true})
	if r > -49 {
		t.Errorf("death reward = %f, want <= -49", r)
	}
}

func TestReward_HitLanded(t *testing.T) {
	r := CalcReward(RewardInput{HitsLanded: 1})
	if r < 2 {
		t.Errorf("hit landed reward = %f, want >= 2", r)
	}
}

func TestReward_MovedToward(t *testing.T) {
	r := CalcReward(RewardInput{DistDelta: -50})
	base := CalcReward(RewardInput{})
	if r <= base {
		t.Errorf("moving toward enemy should increase reward: got %f, baseline %f", r, base)
	}
}

func TestReward_MovedAway(t *testing.T) {
	r := CalcReward(RewardInput{DistDelta: 50})
	base := CalcReward(RewardInput{})
	if r >= base {
		t.Errorf("moving away should decrease reward: got %f, baseline %f", r, base)
	}
}

func TestReward_ShapedScale(t *testing.T) {
	full := CalcReward(RewardInput{DistDelta: -50})
	half := CalcRewardScaled(RewardInput{DistDelta: -50}, 0.5)
	base := CalcReward(RewardInput{})
	baseHalf := CalcRewardScaled(RewardInput{}, 0.5)
	shapedFull := full - base
	shapedHalf := half - baseHalf
	if math.Abs(shapedHalf-shapedFull*0.5) > 0.01 {
		t.Errorf("shaped scale 0.5: got delta %f, want %f", shapedHalf, shapedFull*0.5)
	}
}

func TestReward_Timeout(t *testing.T) {
	r := CalcReward(RewardInput{TimedOut: true, FinalScore: 50})
	if r < 4 {
		t.Errorf("timeout reward = %f, want >= 4 (score/10)", r)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/aienv/ -v -run TestReward`
Expected: compilation error — `CalcReward`, `RewardInput` not defined

- [ ] **Step 3: Implement reward function**

```go
// internal/aienv/reward.go
package aienv

type RewardInput struct {
	EnemyKilledPoints int
	LivesLost         int
	Died              bool
	TimedOut          bool
	FinalScore        int
	HitsLanded        int
	AttackWhiffed     bool
	DistDelta         float64
}

func CalcReward(in RewardInput) float64 {
	return CalcRewardScaled(in, 1.0)
}

func CalcRewardScaled(in RewardInput, shapedScale float64) float64 {
	reward := 0.0

	reward += float64(in.EnemyKilledPoints)
	reward += float64(in.LivesLost) * -5.0

	if in.Died {
		reward += -50.0
		return reward
	}

	if in.TimedOut {
		reward += float64(in.FinalScore) / 10.0
		return reward
	}

	reward += 0.01

	shaped := 0.0
	if in.HitsLanded > 0 {
		shaped += float64(in.HitsLanded) * 2.0
	}
	if in.AttackWhiffed {
		shaped += -0.1
	}
	if in.DistDelta < 0 {
		shaped += 0.5
	} else if in.DistDelta > 0 {
		shaped += -0.2
	}

	reward += shaped * shapedScale

	return reward
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/aienv/ -v -run TestReward`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/aienv/reward.go internal/aienv/reward_test.go
git commit -m "feat(ai): add reward calculation for RL training"
```

---

## Task 4: Headless GameEnv

**Files:**
- Create: `internal/aienv/game_env.go`
- Create: `internal/aienv/game_env_test.go`

The core headless game environment. Wraps player, enemies, spawner, combat into a Step/Reset interface without any Ebiten dependency.

**Key challenge:** The existing `player.Player` and `enemy.Enemy` structs depend on `anim.Animation` which imports `ebiten.Image`. The headless env needs a strategy to handle this.

**Strategy:** Create stub animations — `anim.NewAnimation` accepts `*AnimationSpec` and `[]*ebiten.Image`. For headless mode, we pass `nil` frames. `Animation.Update()`, `FrameIndex()`, and `Done()` work purely on elapsed time + spec metadata — they never touch the image frames. `CurrentFrame()` returns nil but the headless env never calls it. The player FSM calls `PlayAnim()` which needs animations in the map, but they work with nil-frame animations for timing.

- [ ] **Step 1: Write failing test for GameEnv**

```go
// internal/aienv/game_env_test.go
package aienv

import (
	"testing"
)

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
		_, _, done, _ = env.Step(2) // move right
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/aienv/ -v -run TestGameEnv`
Expected: compilation error — `NewGameEnv`, `EnvConfig` not defined

- [ ] **Step 3: Implement headless GameEnv**

This is the largest implementation unit. Key points:
- Creates player/enemies/spawner using existing constructors
- Uses stub animations (nil frames) for headless operation
- Step() applies action as Intent → runs one tick of game logic → returns observation + reward + done + info

```go
// internal/aienv/game_env.go
package aienv

import (
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

const fixedDT = time.Second / 60

type EnvConfig struct {
	Seed int64
}

type GameEnv struct {
	cfg          *config.Config
	w            *world.World
	p            *player.Player
	enemies      []*enemy.Enemy
	sp           *spawner.Spawner
	sc           *score.Counter
	combatTuning *combat.Tuning
	kinds        []*enemy.Kind
	physics      *player.Physics
	staminaT     *player.StaminaTuning
	spawnTuning  *enemy.SpawnTuning
	timeoutS     float64
	elapsedS     float64
	rng          *rand.Rand
	prevScore    int
	prevLives    int
	prevDist     float64
	anims        map[string]*anim.Animation
	soldierBoxes map[string]combat.Box
	hitEvents    int
	shapedScale  float64
}

func NewGameEnv(ecfg EnvConfig) (*GameEnv, error) {
	cfg := config.Load()
	db := storage.MustOpen(cfg)
	defer db.Close()

	animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
	hitboxRepo := storage.NewRepository[combat.HitboxSpec](db, combat.HitboxMapper{})

	animSpecs, err := anim.LoadSpecs(animRepo)
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
	tuneParams, err := tuneRepo.List(nil)
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
	hitboxSpecs, err := hitboxRepo.List(nil)
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
	slimeKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "slime", Prefix: "slime", FrameW: 96, FrameH: 96,
		AnimLib: anims, HitboxSpecs: hitboxSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
		BehaviorPath: cfg.AssetsDir + "/behaviors/slime.json",
	})
	if err != nil {
		return nil, fmt.Errorf("build slime kind: %w", err)
	}

	seed := ecfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &GameEnv{
		cfg:          cfg,
		physics:      physics,
		staminaT:     staminaT,
		combatTuning: combatTuning,
		spawnTuning:  spawnTuning,
		kinds:        []*enemy.Kind{orcKind, slimeKind},
		anims:        anims,
		soldierBoxes: soldierBoxes,
		timeoutS:     tuneMap["game_timeout_s"],
		rng:          rand.New(rand.NewSource(seed)),
		shapedScale:  1.0,
	}, nil
}

func (env *GameEnv) SetShapedScale(s float64) { env.shapedScale = s }

func (env *GameEnv) Reset() []float64 {
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
	env.enemies = nil
	env.sc = &score.Counter{}
	env.elapsedS = 0
	env.prevScore = 0
	env.prevLives = env.combatTuning.SoldierMaxLives
	env.prevDist = 0
	env.hitEvents = 0

	env.sp = spawner.New(spawner.Config{
		MinIntervalS: env.spawnTuning.MinS,
		MaxIntervalS: env.spawnTuning.MaxS,
		MaxAlive:     env.spawnTuning.MaxAlive,
		SpawnXMin:    0,
		SpawnXMax:    float64(env.cfg.WindowW),
		RNG:          env.rng,
		Kinds:        env.buildKindFactories(),
	})

	obs := env.observe()
	return obs[:]
}

func (env *GameEnv) Step(action int) (obs []float64, reward float64, done bool, info map[string]interface{}) {
	intent := ToIntent(action)
	dt := fixedDT

	if env.p.Stamina != nil {
		env.p.Stamina.Update(dt, env.p.IsSprinting(intent))
	}
	env.p.FSM.Handle(env.p, intent, dt)

	for _, e := range env.enemies {
		e.Tick(dt)
	}

	env.p.ApplyPhysics(env.w, dt)
	for _, e := range env.enemies {
		e.ApplyPhysics(env.w, dt)
	}

	soldierBodyHalfW := float64(env.p.Boxes["body"].W) / 2
	env.p.X = world.Clamp(env.p.X, soldierBodyHalfW, float64(env.cfg.WindowW)-soldierBodyHalfW)

	for _, e := range env.enemies {
		bodyHalfW := float64(e.Kind.Boxes["body"].W) / 2
		clamped := world.Clamp(e.X, bodyHalfW, float64(env.cfg.WindowW)-bodyHalfW)
		if clamped != e.X && e.CurrentState == "run" {
			if e.X <= bodyHalfW {
				e.Facing = 1
			} else {
				e.Facing = -1
			}
		}
		e.X = clamped
	}

	if env.p.Current != nil {
		env.p.Current.Update(dt)
	}

	if spawned := env.sp.Tick(dt, len(env.enemies)); spawned != nil {
		env.enemies = append(env.enemies, spawned)
	}

	soldierHits := env.dispatchSoldierHits()
	env.dispatchEnemyHits()

	alive := env.enemies[:0]
	for _, e := range env.enemies {
		if e.Dead {
			env.sc.Add(e.Kind.Tuning.Points)
			continue
		}
		alive = append(alive, e)
	}
	env.enemies = alive

	env.elapsedS += dt.Seconds()

	died := env.p.FSM.CurrentID() == player.StateDeath
	timedOut := env.timeoutS > 0 && env.elapsedS >= env.timeoutS
	done = died || timedOut

	livesLost := env.prevLives - env.p.Lives
	killedPoints := env.sc.Total() - env.prevScore
	currDist := env.nearestEnemyDist()
	distDelta := currDist - env.prevDist

	whiffed := false
	isAttacking := env.p.CurrentAnim == "soldier_attack" || env.p.CurrentAnim == "soldier_attack2"
	if isAttacking && soldierHits == 0 {
		whiffed = true
	}

	reward = CalcRewardScaled(RewardInput{
		EnemyKilledPoints: killedPoints,
		LivesLost:         livesLost,
		Died:              died,
		TimedOut:          timedOut,
		FinalScore:        env.sc.Total(),
		HitsLanded:        soldierHits,
		AttackWhiffed:     whiffed,
		DistDelta:         distDelta,
	}, env.shapedScale)

	env.prevScore = env.sc.Total()
	env.prevLives = env.p.Lives
	env.prevDist = currDist

	obsArr := env.observe()
	info = map[string]interface{}{
		"score":   env.sc.Total(),
		"lives":   env.p.Lives,
		"elapsed": env.elapsedS,
	}

	return obsArr[:], reward, done, info
}

func (env *GameEnv) dispatchSoldierHits() int {
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

func (env *GameEnv) dispatchEnemyHits() {
	attackers := make([]combat.Fighter, 0, len(env.enemies))
	for _, e := range env.enemies {
		attackers = append(attackers, e)
	}
	victims := []combat.Fighter{env.p}
	for _, ev := range combat.Resolve(attackers, victims) {
		orc := ev.Attacker.(*enemy.Enemy)
		env.p.OnHit(env.combatTuning.SoldierKnockbackVX, env.combatTuning.SoldierKnockbackVY, orc.X)
	}
}

func (env *GameEnv) nearestEnemyDist() float64 {
	if len(env.enemies) == 0 {
		return 0
	}
	best := math.MaxFloat64
	for _, e := range env.enemies {
		d := math.Abs(e.X - env.p.X)
		if d < best {
			best = d
		}
	}
	return best
}

func (env *GameEnv) observe() [ObsSize]float64 {
	enemies := make([]EnemyState, 0, len(env.enemies))
	for _, e := range env.enemies {
		stateIdx := 0
		attacking := false
		switch e.CurrentState {
		case "run":
			stateIdx = 1
		case "attack":
			stateIdx = 2
			attacking = true
		case "attack2":
			stateIdx = 3
			attacking = true
		case "hurt":
			stateIdx = 4
		case "death":
			stateIdx = 5
		}
		enemies = append(enemies, EnemyState{
			RelX:      e.X - env.p.X,
			RelY:      e.Y - env.p.Y,
			Lives:     e.Lives,
			MaxLives:  int(e.Kind.Tuning.MaxLives),
			State:     stateIdx,
			Attacking: attacking,
		})
	}

	stateIdx := 0
	switch env.p.FSM.CurrentID() {
	case player.StateIdle:
		stateIdx = 0
	case player.StateRun:
		stateIdx = 1
	case player.StateJump:
		stateIdx = 2
	case player.StateFall:
		stateIdx = 3
	case player.StateAttack:
		stateIdx = 4
	case player.StateAttack2:
		stateIdx = 5
	case player.StateHit:
		stateIdx = 6
	case player.StateDeath:
		stateIdx = 7
	}

	staminaFrac := 0.0
	if env.p.Stamina != nil {
		staminaFrac = env.p.Stamina.Fraction()
	}

	return Observe(GameState{
		PlayerX:  env.p.X,
		PlayerY:  env.p.Y,
		PlayerVX: env.p.VX,
		PlayerVY: env.p.VY,
		Facing:   env.p.Facing,
		Grounded: env.p.Grounded,
		Lives:    env.p.Lives,
		MaxLives: env.combatTuning.SoldierMaxLives,
		Stamina:  staminaFrac,
		StateID:  stateIdx,
		NumStates: 8,
		TimeoutS: env.timeoutS,
		ElapsedS: env.elapsedS,
		Enemies:  enemies,
		MaxAlive: env.spawnTuning.MaxAlive,
		Score:    env.sc.Total(),
		MaxSpeed: env.physics.SprintSpeed,
		MaxFall:  env.physics.MaxFallSpeed,
		WindowW:  float64(env.cfg.WindowW),
		WindowH:  float64(env.cfg.WindowH),
	})
}

func (env *GameEnv) buildKindFactories() []spawner.KindFactory {
	factories := make([]spawner.KindFactory, 0, len(env.kinds))
	for _, k := range env.kinds {
		k := k
		halfW := float64(k.Boxes["body"].W) / 2
		spriteH := float64(k.FrameH * env.cfg.RenderScale)
		factories = append(factories, spawner.KindFactory{
			Name:   k.Name,
			Weight: 1,
			NewEnemy: func(x, _ float64) *enemy.Enemy {
				if x < halfW {
					x = halfW
				}
				if maxX := float64(env.cfg.WindowW) - halfW; x > maxX {
					x = maxX
				}
				return enemy.New(enemy.Config{
					StartX: x, StartY: -spriteH,
					Physics: env.physics,
					Kind:    k,
					RNG:     env.rng,
				})
			},
		})
	}
	return factories
}
```

**Note on `buildHeadlessAnims` and `LoadSpecs`:** These need helper functions. `buildHeadlessAnims` creates `anim.Animation` objects from specs with nil image frames. `LoadSpecs` is a new exported function in the `anim` package that returns raw specs without loading images.

- [ ] **Step 4: Add `LoadSpecs` to anim package**

The `anim` package has `LoadLibrary` which loads images. We need a function that returns only specs (no image loading). Check if `animRepo.List()` already works for this — it does, since `Repository.List()` returns `[]AnimationSpec`. The headless env can call `animRepo.List()` directly and build nil-frame animations.

Add this helper to `internal/aienv/game_env.go` (it stays in the aienv package, not anim):

```go
func buildHeadlessAnims(specs []anim.AnimationSpec) map[string]*anim.Animation {
	m := make(map[string]*anim.Animation, len(specs))
	for i := range specs {
		s := &specs[i]
		m[s.ID] = anim.NewAnimation(s, nil)
	}
	return m
}
```

And add `LoadSpecs` as a helper in `game_env.go`:

```go
func loadAnimSpecs(repo *storage.Repository[anim.AnimationSpec]) ([]anim.AnimationSpec, error) {
	return repo.List(context.Background())
}
```

Update `NewGameEnv` to use `loadAnimSpecs` instead of `anim.LoadSpecs`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/aienv/ -v -run TestGameEnv -timeout 30s`
Expected: all PASS

Note: Tests require `.env` file and `data/game.db` to exist. Run `make run` once first to generate the database, then Ctrl+C.

- [ ] **Step 6: Commit**

```bash
git add internal/aienv/game_env.go internal/aienv/game_env_test.go
git commit -m "feat(ai): add headless GameEnv with Step/Reset for RL training"
```

---

## Task 5: TCP Protocol

**Files:**
- Create: `cmd/train/protocol.go`
- Create: `cmd/train/protocol_test.go`

JSON-line encode/decode for the socket bridge between Go and Python.

- [ ] **Step 1: Write failing tests**

```go
// cmd/train/protocol_test.go
package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeObsMsg(t *testing.T) {
	msg := ObsMsg{
		Type:   "obs",
		Obs:    []float64{0.5, 0.3},
		Reward: 1.5,
		Done:   false,
		Info:   map[string]interface{}{"score": 10},
	}
	var buf bytes.Buffer
	if err := writeMsg(&buf, msg); err != nil {
		t.Fatalf("writeMsg: %v", err)
	}
	line := buf.String()
	if line[len(line)-1] != '\n' {
		t.Error("message should end with newline")
	}
	var decoded ObsMsg
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "obs" {
		t.Errorf("type = %q, want obs", decoded.Type)
	}
	if len(decoded.Obs) != 2 {
		t.Errorf("obs len = %d, want 2", len(decoded.Obs))
	}
}

func TestDecodeActionMsg(t *testing.T) {
	input := `{"type":"action","action":3}` + "\n"
	msg, err := readMsg(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("readMsg: %v", err)
	}
	if msg.Type != "action" {
		t.Errorf("type = %q, want action", msg.Type)
	}
	if msg.Action != 3 {
		t.Errorf("action = %d, want 3", msg.Action)
	}
}

func TestDecodeResetMsg(t *testing.T) {
	input := `{"type":"reset"}` + "\n"
	msg, err := readMsg(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("readMsg: %v", err)
	}
	if msg.Type != "reset" {
		t.Errorf("type = %q, want reset", msg.Type)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/train/ -v -run TestEncode`
Expected: compilation error — types not defined

- [ ] **Step 3: Implement protocol**

```go
// cmd/train/protocol.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

type ObsMsg struct {
	Type   string                 `json:"type"`
	Obs    []float64              `json:"obs"`
	Reward float64                `json:"reward"`
	Done   bool                   `json:"done"`
	Info   map[string]interface{} `json:"info,omitempty"`
}

type ClientMsg struct {
	Type   string `json:"type"`
	Action int    `json:"action,omitempty"`
}

func writeMsg(w io.Writer, msg ObsMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func readMsg(r io.Reader) (ClientMsg, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return ClientMsg{}, err
		}
		return ClientMsg{}, io.EOF
	}
	var msg ClientMsg
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		return ClientMsg{}, err
	}
	return msg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/train/ -v -run TestEncode`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/train/protocol.go cmd/train/protocol_test.go
git commit -m "feat(ai): add JSON-line protocol for Go↔Python socket bridge"
```

---

## Task 6: TCP Training Server

**Files:**
- Create: `cmd/train/main.go`

TCP server that spawns N parallel GameEnvs, one per port. Each connection runs a request-response loop: wait for action/reset → Step/Reset → send observation.

- [ ] **Step 1: Implement training server**

```go
// cmd/train/main.go
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
	"sync"
	"syscall"

	"claude-pixel/internal/aienv"
)

func main() {
	numEnvs := flag.Int("envs", 4, "number of parallel environments")
	basePort := flag.Int("port", 9876, "base port (env i listens on port+i)")
	flag.Parse()

	var wg sync.WaitGroup
	listeners := make([]net.Listener, *numEnvs)

	for i := 0; i < *numEnvs; i++ {
		port := *basePort + i
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			log.Fatalf("env %d: listen on port %d: %v", i, port, err)
		}
		listeners[i] = ln
		log.Printf("env %d listening on :%d", i, port)

		wg.Add(1)
		go func(id int, ln net.Listener) {
			defer wg.Done()
			serveEnv(id, ln)
		}(i, ln)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down...")
	for _, ln := range listeners {
		ln.Close()
	}
	wg.Wait()
}

func serveEnv(id int, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("env %d: accept: %v", id, err)
			return
		}
		log.Printf("env %d: client connected", id)
		handleConn(id, conn)
		conn.Close()
		log.Printf("env %d: client disconnected", id)
	}
}

func handleConn(id int, conn net.Conn) {
	env, err := aienv.NewGameEnv(aienv.EnvConfig{Seed: int64(id + 1)})
	if err != nil {
		log.Printf("env %d: create GameEnv: %v", id, err)
		return
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("env %d: read: %v", id, err)
			}
			return
		}

		var msg ClientMsg
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("env %d: unmarshal: %v", id, err)
			continue
		}

		var resp ObsMsg
		switch msg.Type {
		case "reset":
			obs := env.Reset()
			resp = ObsMsg{Type: "obs", Obs: obs, Reward: 0, Done: false}

		case "action":
			obs, reward, done, info := env.Step(msg.Action)
			resp = ObsMsg{Type: "obs", Obs: obs, Reward: reward, Done: done, Info: info}

		default:
			log.Printf("env %d: unknown message type: %q", id, msg.Type)
			continue
		}

		data, _ := json.Marshal(resp)
		writer.Write(data)
		writer.WriteByte('\n')
		writer.Flush()
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/train/`
Expected: compiles without error

- [ ] **Step 3: Quick smoke test**

Run in one terminal: `go run ./cmd/train -envs=1 -port=9876`

In another terminal:
```bash
echo '{"type":"reset"}' | nc localhost 9876
```
Expected: JSON response with `"type":"obs"` and 25-float obs array.

Ctrl+C both processes.

- [ ] **Step 4: Commit**

```bash
git add cmd/train/main.go
git commit -m "feat(ai): add TCP training server for parallel headless game envs"
```

---

## Task 7: Python Gymnasium Wrapper

**Files:**
- Create: `ai/requirements.txt`
- Create: `ai/config.py`
- Create: `ai/env.py`

Python side of the bridge. Connects to Go server via TCP socket, wraps as standard Gymnasium environment.

- [ ] **Step 1: Create requirements.txt**

```
stable-baselines3>=2.3.0
gymnasium>=0.29.0
torch>=2.0.0
tensorboard>=2.14.0
numpy>=1.24.0
```

- [ ] **Step 2: Create config.py**

```python
# ai/config.py

OBS_SIZE = 25
NUM_ACTIONS = 10

PPO_PARAMS = dict(
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

CHECKPOINT_INTERVAL = 50_000
BASE_PORT = 9876
```

- [ ] **Step 3: Create env.py**

```python
# ai/env.py
import json
import socket

import gymnasium as gym
import numpy as np
from gymnasium import spaces

from config import OBS_SIZE, NUM_ACTIONS


class PixelGameEnv(gym.Env):
    """Gymnasium wrapper for the Go headless game server."""

    metadata = {"render_modes": []}

    def __init__(self, host: str = "localhost", port: int = 9876):
        super().__init__()
        self.host = host
        self.port = port
        self.action_space = spaces.Discrete(NUM_ACTIONS)
        self.observation_space = spaces.Box(
            low=0.0, high=1.0, shape=(OBS_SIZE,), dtype=np.float32
        )
        self._sock = None
        self._rfile = None

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

    def reset(self, seed=None, options=None):
        super().reset(seed=seed)
        self._connect()
        self._send({"type": "reset"})
        resp = self._recv()
        obs = np.array(resp["obs"], dtype=np.float32)
        return obs, resp.get("info", {})

    def step(self, action: int):
        self._send({"type": "action", "action": int(action)})
        resp = self._recv()
        obs = np.array(resp["obs"], dtype=np.float32)
        reward = float(resp["reward"])
        terminated = bool(resp["done"])
        truncated = False
        info = resp.get("info", {})
        return obs, reward, terminated, truncated, info

    def close(self):
        if self._sock is not None:
            self._sock.close()
            self._sock = None
            self._rfile = None
```

- [ ] **Step 4: Test env manually**

Start Go server: `go run ./cmd/train -envs=1 -port=9876`

Run in Python:
```bash
cd ai && pip install -r requirements.txt
python -c "
from env import PixelGameEnv
e = PixelGameEnv()
obs, info = e.reset()
print(f'obs shape: {obs.shape}, obs range: [{obs.min():.3f}, {obs.max():.3f}]')
for i in range(10):
    obs, r, done, trunc, info = e.step(2)
    print(f'step {i}: reward={r:.3f}, done={done}, score={info.get(\"score\",0)}')
    if done:
        obs, info = e.reset()
e.close()
print('OK')
"
```

Expected: 10 steps of output with obs shape (25,), increasing elapsed time.

- [ ] **Step 5: Commit**

```bash
git add ai/requirements.txt ai/config.py ai/env.py
git commit -m "feat(ai): add Python Gymnasium env wrapper for socket bridge"
```

---

## Task 8: Training Script

**Files:**
- Create: `ai/train.py`

SB3 PPO training with auto-checkpoint, graceful shutdown, and resume support.

- [ ] **Step 1: Create train.py**

```python
# ai/train.py
import argparse
import os
import signal
import subprocess
import sys
import time

from stable_baselines3 import PPO
from stable_baselines3.common.callbacks import CheckpointCallback, BaseCallback
from stable_baselines3.common.env_util import make_vec_env

from config import PPO_PARAMS, CHECKPOINT_INTERVAL, BASE_PORT
from env import PixelGameEnv


class GracefulShutdown(BaseCallback):
    """Save model on SIGINT."""

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
            path = os.path.join(self.save_path, "ppo_interrupted")
            self.model.save(path)
            print(f"Saved to {path}.zip")
            return False
        return True


def make_env(port: int):
    def _init():
        return PixelGameEnv(port=port)
    return _init


def main():
    parser = argparse.ArgumentParser(description="Train PPO agent")
    parser.add_argument("--timesteps", type=int, default=1_000_000)
    parser.add_argument("--envs", type=int, default=4)
    parser.add_argument("--resume", type=str, default=None, help="Path to checkpoint .zip to resume from")
    parser.add_argument("--start-server", action="store_true", help="Auto-start Go training server")
    args = parser.parse_args()

    os.makedirs("checkpoints", exist_ok=True)
    os.makedirs("logs", exist_ok=True)

    server_proc = None
    if args.start_server:
        print(f"Starting Go training server with {args.envs} envs...")
        server_proc = subprocess.Popen(
            ["go", "run", "../cmd/train", f"-envs={args.envs}", f"-port={BASE_PORT}"],
            cwd=os.path.dirname(os.path.abspath(__file__)),
        )
        time.sleep(3)

    try:
        env_fns = [make_env(BASE_PORT + i) for i in range(args.envs)]
        vec_env = make_vec_env(PixelGameEnv, n_envs=args.envs, env_kwargs=[
            {"port": BASE_PORT + i} for i in range(args.envs)
        ])

        if args.resume:
            print(f"Resuming from {args.resume}")
            model = PPO.load(args.resume, env=vec_env, tensorboard_log="logs/")
        else:
            model = PPO(
                "MlpPolicy",
                vec_env,
                verbose=1,
                tensorboard_log="logs/",
                **PPO_PARAMS,
            )

        checkpoint_cb = CheckpointCallback(
            save_freq=max(CHECKPOINT_INTERVAL // args.envs, 1),
            save_path="checkpoints/",
            name_prefix="ppo",
        )
        shutdown_cb = GracefulShutdown("checkpoints/")

        print(f"Training for {args.timesteps} timesteps with {args.envs} envs...")
        model.learn(
            total_timesteps=args.timesteps,
            callback=[checkpoint_cb, shutdown_cb],
            reset_num_timesteps=args.resume is None,
        )

        model.save("checkpoints/ppo_final")
        print("Training complete. Saved to checkpoints/ppo_final.zip")

    finally:
        vec_env.close()
        if server_proc:
            server_proc.terminate()
            server_proc.wait()


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Verify train.py imports work**

Run: `cd ai && python -c "import train; print('imports OK')"`
Expected: `imports OK`

- [ ] **Step 3: Short training smoke test**

Start Go server: `go run ./cmd/train -envs=2 -port=9876 &`

Run: `cd ai && python train.py --timesteps 4096 --envs 2`

Expected: PPO trains for 4096 steps (2 rollouts of 2048), saves checkpoint, exits cleanly. TensorBoard logs appear in `ai/logs/`.

Ctrl+C the Go server.

- [ ] **Step 4: Commit**

```bash
git add ai/train.py
git commit -m "feat(ai): add PPO training script with checkpointing and resume"
```

---

## Task 9: Evaluation Script

**Files:**
- Create: `ai/eval.py`

Evaluate a trained model over N episodes, report mean/std score and survival rate.

- [ ] **Step 1: Create eval.py**

```python
# ai/eval.py
import argparse

import numpy as np
from stable_baselines3 import PPO

from config import BASE_PORT
from env import PixelGameEnv


def main():
    parser = argparse.ArgumentParser(description="Evaluate trained model")
    parser.add_argument("--model", type=str, required=True, help="Path to model .zip")
    parser.add_argument("--episodes", type=int, default=100)
    parser.add_argument("--port", type=int, default=BASE_PORT)
    args = parser.parse_args()

    model = PPO.load(args.model)
    env = PixelGameEnv(port=args.port)

    scores = []
    survived = 0

    for ep in range(args.episodes):
        obs, _ = env.reset()
        total_reward = 0
        done = False
        while not done:
            action, _ = model.predict(obs, deterministic=True)
            obs, reward, done, _, info = env.step(action)
            total_reward += reward

        score = info.get("score", 0)
        scores.append(score)
        lives = info.get("lives", 0)
        if lives > 0:
            survived += 1

        if (ep + 1) % 10 == 0:
            print(f"Episode {ep+1}/{args.episodes}: score={score}, lives={lives}")

    env.close()

    scores = np.array(scores)
    print(f"\n{'='*40}")
    print(f"Episodes: {args.episodes}")
    print(f"Mean score: {scores.mean():.1f} ± {scores.std():.1f}")
    print(f"Max score: {scores.max()}")
    print(f"Min score: {scores.min()}")
    print(f"Survival rate: {survived}/{args.episodes} ({100*survived/args.episodes:.0f}%)")


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Commit**

```bash
git add ai/eval.py
git commit -m "feat(ai): add evaluation script for trained model"
```

---

## Task 10: Inference / Play Script

**Files:**
- Create: `ai/play.py`

Watch trained AI play the real game with rendering.

- [ ] **Step 1: Create play.py**

```python
# ai/play.py
import argparse
import time

from stable_baselines3 import PPO

from config import BASE_PORT
from env import PixelGameEnv


def main():
    parser = argparse.ArgumentParser(description="Watch AI play")
    parser.add_argument("--model", type=str, required=True, help="Path to model .zip")
    parser.add_argument("--port", type=int, default=BASE_PORT + 1)
    parser.add_argument("--episodes", type=int, default=0, help="0 = infinite")
    args = parser.parse_args()

    model = PPO.load(args.model)
    env = PixelGameEnv(port=args.port)

    ep = 0
    try:
        while args.episodes == 0 or ep < args.episodes:
            obs, _ = env.reset()
            done = False
            step = 0
            while not done:
                action, _ = model.predict(obs, deterministic=True)
                obs, reward, done, _, info = env.step(action)
                step += 1
                time.sleep(1 / 60)

            ep += 1
            print(f"Episode {ep}: score={info.get('score', 0)}, "
                  f"lives={info.get('lives', 0)}, steps={step}")
    except KeyboardInterrupt:
        pass
    finally:
        env.close()


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Commit**

```bash
git add ai/play.py
git commit -m "feat(ai): add inference script to watch AI play"
```

---

## Task 11: Makefile & .gitignore

**Files:**
- Modify: `Makefile`
- Modify: `.gitignore`

- [ ] **Step 1: Add AI targets to Makefile**

Append to existing Makefile:

```makefile

# === AI Training ===

.PHONY: ai-setup train-server train train-resume train-eval train-play train-tensorboard train-clean

ai-setup:              ## Install Python dependencies for AI training
	cd ai && pip install -r requirements.txt

train-server:          ## Start headless Go game server for RL training (ENVS=4 default)
	go run ./cmd/train -envs=$(or $(ENVS),4) -port=9876

train:                 ## Train PPO agent (STEPS=1000000 default, ENVS=4 default)
	cd ai && python train.py --timesteps=$(or $(STEPS),1000000) --envs=$(or $(ENVS),4)

train-resume:          ## Resume training from latest checkpoint (STEPS=500000 default)
	cd ai && python train.py --resume checkpoints/ppo_latest.zip --timesteps=$(or $(STEPS),500000)

train-eval:            ## Evaluate trained model over 100 episodes
	cd ai && python eval.py --model checkpoints/ppo_final.zip --episodes=$(or $(EPISODES),100)

train-play:            ## Watch AI play game with full rendering
	cd ai && python play.py --model checkpoints/ppo_final.zip

train-tensorboard:     ## Open TensorBoard dashboard for training metrics
	tensorboard --logdir ai/logs/

train-clean:           ## Remove all checkpoints and training logs
	rm -rf ai/checkpoints/ ai/logs/
```

- [ ] **Step 2: Add AI directories to .gitignore**

Append to `.gitignore`:

```
# AI training artifacts
ai/checkpoints/
ai/logs/
__pycache__/
*.pyc
```

- [ ] **Step 3: Commit**

```bash
git add Makefile .gitignore
git commit -m "feat(ai): add Makefile targets and gitignore for AI training"
```

---

## Task 12: Integration Test — Full Training Loop

End-to-end smoke test: Go server + Python training for a few thousand steps.

- [ ] **Step 1: Run full integration**

Terminal 1:
```bash
make train-server ENVS=2
```

Terminal 2:
```bash
make ai-setup
make train STEPS=8192 ENVS=2
```

Expected:
- Go server prints "env 0 listening on :9876", "env 1 listening on :9877"
- Python connects, trains 4 rollouts of 2048 steps each
- Checkpoint saved to `ai/checkpoints/ppo_2048_steps.zip` and `ppo_final.zip`
- TensorBoard logs in `ai/logs/`

- [ ] **Step 2: Run evaluation**

```bash
make train-eval EPISODES=10
```

Expected: prints 10 episode scores (likely low — only 8K steps of training). No crashes.

- [ ] **Step 3: Verify TensorBoard**

```bash
make train-tensorboard
```

Open browser to `localhost:6006`. Should see reward curve, episode length, loss metrics.

- [ ] **Step 4: Clean up**

```bash
make train-clean
```

Ctrl+C the Go server.

- [ ] **Step 5: Commit any fixes from integration**

```bash
git add -A
git commit -m "fix(ai): integration fixes from end-to-end training test"
```

---

## Task 13: AI Mode for Real Game (Inference)

**Files:**
- Modify: `cmd/game/main.go`
- Modify: `internal/game/game.go`

Add `--ai` flag to `cmd/game` that replaces keyboard input with socket-based AI control. Game renders normally but actions come from the Python agent.

- [ ] **Step 1: Add AI socket client to game.go**

Add a new field `aiConn` to `Game` struct and modify `Update()` to read actions from socket instead of `input.Poll()` when in AI mode.

In `internal/game/game.go`, add:

```go
// Near top of file, add imports:
import (
    "bufio"
    "encoding/json"
    "net"
    // ... existing imports
)

// Add to Game struct:
type Game struct {
    // ... existing fields
    aiConn   net.Conn
    aiReader *bufio.Reader
    aiWriter *bufio.Writer
}

// Add AI message types:
type aiObsMsg struct {
    Type   string                 `json:"type"`
    Obs    []float64              `json:"obs"`
    Reward float64                `json:"reward"`
    Done   bool                   `json:"done"`
    Info   map[string]interface{} `json:"info,omitempty"`
}

type aiActionMsg struct {
    Type   string `json:"type"`
    Action int    `json:"action"`
}
```

Modify `Deps` to add `AIPort int`. In `New()`, if `AIPort > 0`, listen for connection.

In `Update()`, replace `intent := input.Poll()` with:
```go
var intent input.Intent
if g.aiConn != nil {
    intent = g.aiReadAction()
} else {
    intent = input.Poll()
}
```

The `aiReadAction` method sends current observation, then reads the action response.

This is a substantial change — implement carefully to avoid breaking the normal game flow.

- [ ] **Step 2: Add --ai flag to cmd/game/main.go**

```go
// Add flag parsing at top of main():
aiPort := flag.Int("ai", 0, "AI control port (0 = keyboard)")
flag.Parse()

// Pass to Deps:
g := game.New(game.Deps{
    // ... existing fields
    AIPort: *aiPort,
})
```

- [ ] **Step 3: Test AI mode**

Terminal 1: `go run ./cmd/game -ai 9877`

Wait for "AI: listening on :9877" log message.

Terminal 2:
```bash
cd ai && python play.py --model checkpoints/ppo_final.zip --port 9877
```

Expected: Game window shows soldier being controlled by AI. Player moves, attacks enemies.

- [ ] **Step 4: Commit**

```bash
git add internal/game/game.go cmd/game/main.go
git commit -m "feat(ai): add --ai flag for socket-based AI control in real game"
```

---

## Summary

| Task | Component | Est. Time |
|------|-----------|-----------|
| 1 | Action mapping | 5 min |
| 2 | Observation vector | 10 min |
| 3 | Reward function | 10 min |
| 4 | Headless GameEnv | 30 min |
| 5 | TCP protocol | 10 min |
| 6 | TCP training server | 15 min |
| 7 | Python Gym wrapper | 15 min |
| 8 | Training script | 15 min |
| 9 | Evaluation script | 5 min |
| 10 | Play/inference script | 5 min |
| 11 | Makefile & gitignore | 5 min |
| 12 | Integration test | 15 min |
| 13 | AI mode for real game | 25 min |
| **Total** | | **~2.5 hours** |

After completing all tasks, train for real:
```bash
# Terminal 1
make train-server ENVS=4

# Terminal 2
make train STEPS=1000000 ENVS=4
# ~3-6 hours later...

# Evaluate
make train-eval EPISODES=100

# Watch it play
make train-play
```
