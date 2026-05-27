# AI Reinforcement Learning Agent — Design Spec

**Date**: 2026-05-27
**Goal**: Train an RL agent to autonomously control the soldier, kill enemies, and maximize score within the game timeout.
**Target**: ≥80% reasonable actions after training.

---

## 1. Approach

**Algorithm**: PPO (Proximal Policy Optimization) via Stable-Baselines3
**Observation**: Feature vector (~25 normalized floats extracted from game state)
**Bridge**: Go headless game server ↔ Python agent via TCP socket (JSON protocol)
**Hardware**: CPU only (MacBook Apple Silicon)
**Training estimate**: 3–6 hours for 80% performance

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Training Pipeline                     │
│                                                         │
│  ┌──────────────┐   TCP Socket        ┌──────────────┐  │
│  │  Go Process  │ ◄────────────────► │ Python Agent │  │
│  │              │   JSON messages     │              │  │
│  │ cmd/train/   │                    │ ai/           │  │
│  │  main.go     │                    │  train.py     │  │
│  │              │                    │  model.py     │  │
│  │ Headless     │                    │  env.py       │  │
│  │ GameEnv x N  │                    │  (Gymnasium)  │  │
│  └──────────────┘                    └──────────────┘  │
│                                                         │
│  Inference (play mode):                                 │
│  ┌──────────────────────────────────────────────────┐   │
│  │ cmd/game --ai ←socket→ ai/play.py (loads model)  │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### Components

1. **Go Headless Game Server** (`cmd/train/main.go`): Game logic without rendering. Exposes `Step(action) → (obs, reward, done, info)` via TCP socket. Supports N parallel envs on N goroutines, one port per env.

2. **Python Gymnasium Env** (`ai/env.py`): Socket client wrapping the standard Gymnasium interface. Sends actions, receives observations.

3. **Training Script** (`ai/train.py`): SB3 PPO training loop with SubprocVecEnv for parallelism. Logging via TensorBoard. Auto-checkpoint every 50K steps.

4. **Inference Runner** (`ai/play.py`): Loads trained model, connects to game running with `--ai` flag, sends actions at 60Hz to match game TPS.

---

## 3. Go Headless Game Environment

New package `internal/aienv/` — reuses existing game logic packages without Ebiten dependency.

### GameEnv struct

```go
type GameEnv struct {
    player   *player.Player
    enemies  []*enemy.Enemy
    spawner  *spawner.Spawner
    score    *score.Counter
    combat   *combat.Tuning
    world    *world.World
    elapsed  float64
    timeout  float64
    dt       time.Duration // fixed 1/60s
    prevScore int
    prevLives int
}

func (e *GameEnv) Step(action int) (obs []float64, reward float64, done bool, info map[string]any)
func (e *GameEnv) Reset() []float64
func (e *GameEnv) GetObservation() []float64
```

### Key differences vs `game.Game`

- No Ebiten dependency (no Draw, no Layout, no image/sprite loading)
- No animation frames — state tracked by ID only
- Fixed `dt = 1/60s` per Step() call
- No input polling — action injected directly as Intent
- Deterministic given same seed

---

## 4. Action Space

Discrete action space with 10 actions covering all meaningful combinations:

| ID | Action | Intent Fields |
|----|--------|---------------|
| 0 | Idle | all false |
| 1 | Move Left | Left=true |
| 2 | Move Right | Right=true |
| 3 | Jump | JumpPressed=true |
| 4 | Move Left + Jump | Left=true, JumpPressed=true |
| 5 | Move Right + Jump | Right=true, JumpPressed=true |
| 6 | Attack1 | AttackPressed=true |
| 7 | Attack2 | Attack2Pressed=true |
| 8 | Sprint Left | Left=true, SprintHeld=true |
| 9 | Sprint Right | Right=true, SprintHeld=true |

---

## 5. Observation Space

25 floats, all normalized to [0, 1] range:

| Index | Feature | Normalization |
|-------|---------|---------------|
| 0 | player_x | / window_w |
| 1 | player_y | / window_h |
| 2 | player_vx | / max_speed, shifted to [0,1] |
| 3 | player_vy | / max_fall, shifted to [0,1] |
| 4 | player_facing | 0 (left) or 1 (right) |
| 5 | player_grounded | 0 or 1 |
| 6 | player_lives | / max_lives |
| 7 | player_stamina | stamina.Fraction() |
| 8 | player_state_id | / num_states |
| 9 | time_remaining | / timeout |
| 10–12 | enemy1 (rel_x, rel_y, lives) | normalized relative to player |
| 13–15 | enemy2 (rel_x, rel_y, lives) | same |
| 16–18 | enemy3 (rel_x, rel_y, lives) | same |
| 19 | num_enemies_alive | / max_alive |
| 20 | score | / theoretical_max |
| 21 | nearest_enemy_dist | / screen_diagonal |
| 22 | nearest_enemy_angle | / 2π, shifted |
| 23 | nearest_enemy_state | encoded |
| 24 | nearest_enemy_attacking | 0 or 1 |

Empty enemy slots filled with zeros (fixed-size vector).

Enemies sorted by distance to player (nearest first).

---

## 6. Reward Function

### Per-step rewards

| Event | Reward | Rationale |
|-------|--------|-----------|
| Orc killed | +10.0 | Primary objective |
| Slime killed | +15.0 | Higher difficulty enemy |
| Life lost | -5.0 | Discourage reckless play |
| Player death | -50.0 | Heavy penalty, episode ends |
| Survival per step | +0.01 | Encourage staying alive |

### Shaped rewards (guide early learning)

| Event | Reward | Rationale |
|-------|--------|-----------|
| Move toward nearest enemy | +0.5 | Encourage engagement |
| Move away from all enemies | -0.2 | Discourage passive play |
| Attack connected (hit landed) | +2.0 | Reward accuracy |
| Attack whiffed (wrong range) | -0.1 | Discourage spam |

### Episode termination

- **Player death**: `done=True`, final reward = -50
- **Timeout**: `done=True`, final reward = `+score / 10.0` (survived = good, higher score = better)

### Curriculum (phased difficulty)

| Phase | Steps | Max Enemies | Timeout | Notes |
|-------|-------|-------------|---------|-------|
| 1 | 0–200K | 1 | 60s | Learn movement + basic attack |
| 2 | 200K–500K | 2 | 30s | Multi-enemy engagement |
| 3 | 500K–1M+ | 3 | 30s | Full difficulty, reduce shaped rewards |

Shaped rewards (move toward, whiff penalty) are scaled down 50% per phase to avoid reward hacking.

---

## 7. Communication Protocol

### Transport

TCP socket, one port per env: `localhost:(9876 + env_id)`.

### Message format

JSON-line protocol (one JSON object per line, terminated by `\n`).

**Go → Python (observation):**
```json
{
  "type": "obs",
  "obs": [0.5, 0.83, 0.0, ...],
  "reward": 10.0,
  "done": false,
  "info": {
    "score": 25,
    "lives": 8,
    "elapsed": 12.3,
    "kills": {"orc": 1, "slime": 1}
  }
}
```

**Python → Go (action):**
```json
{"type": "action", "action": 3}
```

**Python → Go (reset):**
```json
{"type": "reset"}
```

**Go → Python (reset response):**
```json
{"type": "obs", "obs": [...], "reward": 0.0, "done": false, "info": {}}
```

### Sequence

```
Python              Go
  |--- reset ------->|
  |<-- obs ----------|
  |--- action 3 ---->|
  |<-- obs+rew+done -|
  |--- action 7 ---->|
  |<-- obs+rew+done -|  (done=true)
  |--- reset ------->|
  |<-- obs ----------|
  ...
```

---

## 8. PPO Hyperparameters

```python
PPO(
    policy="MlpPolicy",
    env=vec_env,
    learning_rate=3e-4,
    n_steps=2048,          # steps per rollout
    batch_size=64,
    n_epochs=10,
    gamma=0.99,            # discount factor
    gae_lambda=0.95,       # GAE lambda
    clip_range=0.2,        # PPO clip
    ent_coef=0.01,         # entropy bonus for exploration
    vf_coef=0.5,           # value function loss weight
    max_grad_norm=0.5,
    policy_kwargs=dict(
        net_arch=[128, 128],       # 2 hidden layers
        activation_fn=torch.nn.ReLU,
    ),
    verbose=1,
    tensorboard_log="logs/",
)
```

### Network architecture

```
Input (25) → FC(128) → ReLU → FC(128) → ReLU → Policy Head (10 actions)
                                               → Value Head (1 scalar)
```

Small network — fast inference, trains on CPU without issue.

---

## 9. Checkpointing & Resume

### Auto-checkpoint

Every 50K steps, save to `ai/checkpoints/ppo_step_{N}.zip`.
Keep last 5 checkpoints + best (by mean reward over 100 episodes).

### Graceful shutdown

SIGINT (Ctrl+C) handler saves current model before exit:
```
ai/checkpoints/
  ppo_step_50000.zip
  ppo_step_100000.zip
  ...
  ppo_latest.zip      # symlink to most recent
  ppo_best.zip        # best mean reward
```

### Resume training

```bash
python ai/train.py --resume ai/checkpoints/ppo_latest.zip --timesteps 500000
```

Loads model weights + optimizer state. Training continues seamlessly.

---

## 10. Inference (Watch AI Play)

### v1: External Python process

```
cmd/game --ai --port 9877
    ↕ TCP socket
ai/play.py --model checkpoints/ppo_best.zip --port 9877
```

Game runs with full rendering. Instead of polling keyboard, reads action from socket each frame. `--ai` flag in `cmd/game`:
- Starts socket listener on specified port
- Each Update() tick: send observation → receive action → apply as Intent
- Player still renders normally, just AI-controlled
- Human can watch in real-time

### Future: ONNX embedding (v2, optional)

Export PyTorch → ONNX → load in Go via `onnxruntime-go`. No Python at runtime. Deferred — not in scope for v1.

---

## 11. File Structure

```
cmd/train/
  main.go              # Headless game server entry point
  protocol.go          # TCP socket JSON protocol
  env.go               # Port-per-env listener setup

internal/aienv/
  game_env.go          # GameEnv: Step, Reset, observation extraction
  reward.go            # Reward calculation logic
  action.go            # Action ID → Intent mapping
  curriculum.go        # Phase-based difficulty scaling

ai/
  requirements.txt     # Python dependencies
  train.py             # Training entry point (SB3 PPO)
  env.py               # Gymnasium wrapper (socket client)
  model.py             # Network architecture config
  eval.py              # Evaluate model over N episodes
  play.py              # Inference: watch AI play
  config.py            # Hyperparameters + curriculum config
  checkpoints/         # Saved models (.zip)
  logs/                # TensorBoard logs
```

---

## 12. Makefile Targets

```makefile
# === AI Training ===

train-server:          ## Start headless Go game server for RL training (ENVS=4 default)
	go run ./cmd/train -envs=$(or $(ENVS),4) -port=9876

train:                 ## Train PPO agent (STEPS=1000000 default, ENVS=4 default)
	cd ai && python train.py --timesteps=$(or $(STEPS),1000000) --envs=$(or $(ENVS),4)

train-resume:          ## Resume training from latest checkpoint (STEPS=500000 default)
	cd ai && python train.py --resume checkpoints/ppo_latest.zip --timesteps=$(or $(STEPS),500000)

train-eval:            ## Evaluate trained model over 100 episodes
	cd ai && python eval.py --model checkpoints/ppo_best.zip --episodes=100

train-play:            ## Watch AI play game with full rendering
	@echo "Starting game server and AI player..."
	go run ./cmd/game -ai -port=9877 &
	sleep 2 && cd ai && python play.py --model checkpoints/ppo_best.zip --port 9877

train-tensorboard:     ## Open TensorBoard dashboard for training metrics
	tensorboard --logdir ai/logs/

train-clean:           ## Remove all checkpoints and training logs
	rm -rf ai/checkpoints/* ai/logs/*

ai-setup:              ## Install Python dependencies for AI training
	cd ai && pip install -r requirements.txt
```

---

## 13. Training Time Estimates

On MacBook Apple Silicon (CPU only, 4 parallel envs):

| Phase | Steps | Time | Expected Behavior |
|-------|-------|------|-------------------|
| 1 | 0–200K | 30–60 min | Learns to move, face enemies, basic attacks |
| 2 | 200K–500K | 1–2 hours | Kills enemies consistently, some dodging |
| 3 | 500K–1M | 1–3 hours | 80%+ reasonable actions, strategic play |
| 4 (optional) | 1M–2M | 2–4 hours | 90%+ optimized, efficient combos |

**Total for 80% target: ~3–6 hours.**

Bottleneck is Go game simulation speed (~3000–5000 steps/s headless). PPO network inference trivial on CPU for 25-input MLP.

---

## 14. Dependencies

### Python (`ai/requirements.txt`)

```
stable-baselines3>=2.3.0
gymnasium>=0.29.0
torch>=2.0.0
tensorboard>=2.14.0
numpy>=1.24.0
```

### Go

No new Go dependencies. Headless env reuses existing `internal/` packages:
- `internal/player` (physics, FSM)
- `internal/enemy` (kind, tick)
- `internal/spawner` (timer)
- `internal/combat` (resolve, tuning)
- `internal/score` (counter)
- `internal/world` (ground, clamp)
- `internal/behavior` (BT runtime)
- `internal/stamina` (pool)
- `internal/storage` (tuning values from DB)

---

## 15. Success Criteria

| Metric | Target |
|--------|--------|
| Mean episode score | ≥ 80% of theoretical max (kills × points within timeout) |
| Action reasonableness | ≥ 80% of actions are contextually appropriate |
| Survival rate | Agent survives to timeout in ≥ 60% of episodes |
| Training time | ≤ 6 hours on MacBook CPU |
| Checkpoint/resume | Training resumable after stop without performance loss |

### What "80% reasonable actions" means

- Moves toward enemies (not wandering aimlessly)
- Attacks when in range (not spamming attack at distance)
- Jumps to avoid enemy attacks occasionally
- Uses sprint to close distance
- Manages stamina (doesn't sprint to depletion constantly)
- Engages new enemies after kills
