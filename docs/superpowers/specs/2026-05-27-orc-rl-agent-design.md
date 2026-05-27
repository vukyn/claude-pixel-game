# Orc RL Agent — Design Spec

**Date**: 2026-05-27
**Goal**: Train an RL agent to control orc enemies — approach player, attack, and dodge player attacks. Replaces behavior tree entirely.
**Training**: Self-play against trained player RL agent. Shared policy across all orc instances.

---

## 1. Approach

**Algorithm**: PPO via Stable-Baselines3 (same as player agent)
**Observation**: 16-float vector from orc's perspective
**Action**: 6 discrete actions (move toward/away, attack1/2, idle, flip)
**Opponent**: Fixed player PPO model (loaded from checkpoint)
**Multi-orc**: Shared policy — one model, each orc observes independently

---

## 2. Architecture

```
┌──────────────────────────────────────────────────────┐
│                   Training Pipeline                   │
│                                                       │
│  Go Headless Server (cmd/train)                       │
│    OrcTrainEnv:                                       │
│      Step(playerAction, orcActions[]) →                │
│        per-orc obs[], rewards[], dones[]               │
│        + player obs (for fixed model inference)        │
│                                                       │
│  Python (ai/)                                         │
│    player_model = PPO.load("player_checkpoint.zip")   │
│    orc_model = PPO("MlpPolicy", orc_env)  # training │
│                                                       │
│    Each step:                                         │
│      1. Receive game state from Go                    │
│      2. player_action = player_model.predict(p_obs)   │
│      3. orc_actions = orc_model.predict(orc_obs)      │
│      4. Send {player_action, orc_actions} to Go       │
│      5. Go processes tick → returns new state          │
└──────────────────────────────────────────────────────┘
```

### Components

1. **Go `OrcTrainEnv`** (`internal/aienv/orc_env.go`): Headless game env that accepts both player and orc actions per step. Returns separate observations and rewards for player and each orc.

2. **Go TCP protocol extension**: Messages now carry player action + orc actions array, and responses include per-orc observations/rewards.

3. **Python `OrcTrainWrapper`** (`ai/orc_env.py`): Gymnasium env that manages both agents. Loads fixed player model, trains orc model. Treats N orcs as a batch for SB3.

4. **Python `train_orc.py`**: Training script similar to `train.py` but for orc agent.

---

## 3. Orc Observation Space

16 floats, normalized to [0, 1]:

| Index | Feature | Normalization |
|-------|---------|---------------|
| 0 | orc_x | / window_w |
| 1 | orc_y | / window_h |
| 2 | orc_vx | / max_speed, shifted to [0,1] |
| 3 | orc_vy | / max_fall, shifted to [0,1] |
| 4 | orc_facing | 0 (left) or 1 (right) |
| 5 | orc_grounded | 0 or 1 |
| 6 | orc_lives | / max_lives |
| 7 | orc_state | / num_states |
| 8 | player_rel_x | signed distance / window_w, shifted to [0,1] |
| 9 | player_rel_y | / window_h, shifted |
| 10 | player_lives | / max_lives |
| 11 | player_state | / num_player_states |
| 12 | player_attacking | 0 or 1 |
| 13 | player_facing_toward_orc | 0 or 1 |
| 14 | distance_to_player | / screen_diagonal |
| 15 | time_remaining | / timeout |

---

## 4. Orc Action Space

6 discrete actions:

| ID | Action | Effect |
|----|--------|--------|
| 0 | Idle | VX = 0, stay in current state |
| 1 | Move toward player | VX = run_speed × sign(player_x - orc_x) |
| 2 | Move away from player | VX = run_speed × -sign(player_x - orc_x) |
| 3 | Attack1 | Transition to attack state (if in run state) |
| 4 | Attack2 | Transition to attack2 state (if in run state) |
| 5 | Stop + flip | VX = 0, reverse facing direction |

Actions 3-4 only take effect when orc is in "run" state. During attack/hurt/death animations, the action is ignored and the state plays out according to existing FSM exit rules.

---

## 5. Orc Reward Function

### Per-step rewards

| Event | Reward | Rationale |
|-------|--------|-----------|
| Hit player (attack connected) | +8.0 | Primary objective |
| Player died | +20.0 | Ultimate success |
| Orc life lost | -5.0 | Discourage reckless approach |
| Orc died | -15.0 | Heavy penalty |
| Survival per step | +0.01 | Stay alive |

### Shaped rewards

| Event | Reward | Rationale |
|-------|--------|-----------|
| Move toward player | +0.1 | Encourage engagement |
| Dodge success | +3.0 | Reward evasion skill |
| Stagnation (>180 steps no hit landed) | -0.3/step | Pressure to attack |

### Dodge detection

A "dodge success" is detected when ALL of these are true in a single step:
- Player is in attack animation (`soldier_attack` or `soldier_attack2`)
- Orc moved away from player this step (DistDelta > 0 relative to player)
- Orc was NOT hit this step

### Episode termination

- **Player death**: `done=True` for all orcs, reward +20
- **All orcs dead**: `done=True`, reward -15 for the dying orc
- **Timeout**: `done=True`, reward = 0

---

## 6. Multi-Agent Protocol

### Go → Python message

```json
{
  "type": "obs",
  "player_obs": [0.5, 0.83, ...],
  "orc_obs": [
    [0.1, 0.7, ...],
    [0.8, 0.7, ...]
  ],
  "orc_rewards": [0.5, -0.1],
  "orc_dones": [false, false],
  "done": false,
  "info": {
    "player_score": 25,
    "player_lives": 8,
    "orc_count": 2
  }
}
```

### Python → Go message

```json
{
  "type": "action",
  "player_action": 2,
  "orc_actions": [1, 3]
}
```

### Reset

```json
{"type": "reset"}
```

Response is the initial obs message with zero rewards and done=false.

### Handling variable orc count

Orc count changes during an episode (spawning, dying). The protocol sends observations only for alive orcs. Python side pads/truncates to fixed max_orcs for SB3 compatibility.

When an orc dies mid-episode, its final observation is sent with `orc_dones[i]=true` and death reward. New orcs spawned get their first observation on the next step.

---

## 7. OrcTrainEnv (Go side)

New struct `internal/aienv/orc_env.go`:

```go
type OrcTrainEnv struct {
    // Same base as GameEnv (player, enemies, spawner, combat, world, etc.)
    // But Step() accepts both player and orc actions
}

type OrcStepResult struct {
    PlayerObs  []float64
    OrcObs     [][]float64
    OrcRewards []float64
    OrcDones   []bool
    Done       bool
    Info       map[string]any
}

func (env *OrcTrainEnv) Step(playerAction int, orcActions []int) OrcStepResult
func (env *OrcTrainEnv) Reset() OrcStepResult
```

### Orc action application

In `Step()`, for each alive orc, apply its action:
- Actions 0-2, 5: Set VX and facing directly (bypass BT)
- Actions 3-4: Trigger FSM transition to attack/attack2 state
- During non-run states (attack/hurt/death/fall): action is ignored, existing FSM exit rules apply

This replaces `enemy.Tick()` for RL-controlled orcs. The orc still uses its existing FSM states and animations, but decisions come from RL instead of behavior tree.

---

## 8. Python Training Setup

### OrcGymEnv (`ai/orc_env.py`)

```python
class OrcGymEnv(gym.Env):
    """Wraps Go server for orc RL training."""
    
    def __init__(self, player_model_path, port, max_orcs=5):
        self.player_model = PPO.load(player_model_path)
        self.max_orcs = max_orcs
        self.observation_space = spaces.Box(0, 1, shape=(ORC_OBS_SIZE,))
        self.action_space = spaces.Discrete(ORC_NUM_ACTIONS)
        # Connect to Go server
```

### VecEnv trick for multi-orc

Each alive orc is treated as a separate "environment" in SB3's perspective. Use a custom VecEnv that:
1. Collects observations from all alive orcs
2. Gets actions for all orcs from the training model
3. Sends all actions to Go in one message
4. Returns per-orc obs/reward/done

### train_orc.py

```python
def main():
    player_model = PPO.load("checkpoints/ppo_final.zip")
    orc_env = OrcVecEnv(player_model, port=9876, max_orcs=5)
    
    orc_model = PPO("MlpPolicy", orc_env, **ORC_PPO_PARAMS)
    orc_model.learn(total_timesteps=500_000)
    orc_model.save("checkpoints/orc_final")
```

---

## 9. Makefile Targets

```makefile
train-orc-server:      ## Start headless server for orc RL training
    go run ./cmd/train -mode=orc -envs=1 -port=9876

train-orc:             ## Train orc RL agent against player model
    cd ai && python3 train_orc.py --timesteps=$(or $(STEPS),500000) --player-model=checkpoints/ppo_final.zip

train-orc-visual:      ## Train orc with game window visible
    go run ./cmd/game -ai-orc 9876 &
    @sleep 2
    cd ai && python3 train_orc.py --timesteps=$(or $(STEPS),50000) --player-model=checkpoints/ppo_final.zip
```

---

## 10. File Structure

### New Go files

| File | Responsibility |
|------|----------------|
| `internal/aienv/orc_obs.go` | Orc observation vector (16 floats) |
| `internal/aienv/orc_obs_test.go` | Tests |
| `internal/aienv/orc_action.go` | Orc action ID → FSM/VX mapping |
| `internal/aienv/orc_action_test.go` | Tests |
| `internal/aienv/orc_reward.go` | Orc reward function |
| `internal/aienv/orc_reward_test.go` | Tests |
| `internal/aienv/orc_env.go` | OrcTrainEnv: multi-agent Step/Reset |
| `internal/aienv/orc_env_test.go` | Tests |

### New Python files

| File | Responsibility |
|------|----------------|
| `ai/orc_config.py` | Orc hyperparameters |
| `ai/orc_env.py` | OrcGymEnv + OrcVecEnv wrapper |
| `ai/train_orc.py` | Orc training script |
| `ai/eval_orc.py` | Evaluate orc model |

### Modified files

| File | Change |
|------|--------|
| `cmd/train/main.go` | Add `-mode=orc` flag for orc training server |
| `cmd/train/protocol.go` | Multi-agent message types |
| `cmd/game/main.go` | Add `-ai-orc` flag |
| `internal/game/game.go` | Orc AI mode support |
| `Makefile` | Orc training targets |

---

## 11. Training Pipeline

### Phase 1: Train player (done ✓)
```bash
make train STEPS=200000
```

### Phase 2: Train orc vs fixed player
```bash
make train-orc STEPS=500000
```

### Phase 3 (optional): Retrain player vs trained orc
```bash
# Future: iterative self-play
make train STEPS=200000 ORC_MODEL=checkpoints/orc_final.zip
```

---

## 12. Success Criteria

| Metric | Target |
|--------|--------|
| Orc approaches player | > 80% of time moving toward player |
| Orc attacks when in range | > 60% attack rate when distance < 100px |
| Orc dodges player attacks | > 30% dodge rate when player attacks |
| Training time | ≤ 4 hours on MacBook CPU |
