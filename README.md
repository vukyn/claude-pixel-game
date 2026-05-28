# claude-pixel-game

2D pixel-art action game in Go + Ebiten. Soldier vs orcs/slimes on a flat ground arena. SQLite-backed tuning, JSON-driven enemy behavior trees, reloadable HUD, and PPO reinforcement-learning hooks for player + orc agents.

## Features

- **Engine**: Ebitengine renderer, fixed-tick FSM player + enemies, AABB hitbox combat with per-frame attack windows.
- **Soldier** (8 states): Idle / Run / Jump / Fall / Attack / Attack2 / Hit / Death. Stamina-gated sprint, attack-cancel-by-jump.
- **Enemies**: Orc (2 HP, BT-driven) and Slime (slower, attack2 backstep). FSM shape shared; logic from `assets/behaviors/<kind>.json`.
- **Spawner**: Multi-kind, weighted; interval + concurrent-alive cap from `tuning` table.
- **HUD**: Animated heart + lives + stamina bar + score. Layout in `hud_layout` table, tunable via CLI.
- **Tuning CLI** (`cmd/tune`): inspect/update tuning + HUD without SQL. Validates ranges. Agent-mode TSV output (`--agent-mode`).
- **Behavior editor**: Go Fiber backend (`cmd/editor`) + React/Vite/shadcn/React Flow frontend (`tools/editor-web`) for editing BT JSON live.
- **Debug overlay** (F3): 25 fields. Hitbox draw (F4). Behavior reload (F5). Pause (Esc). Restart on death (R).
- **AI training**: Headless game servers (`cmd/train`, `cmd/train-orc`) + Python PPO pipeline (`ai/`) using stable-baselines3.

## Prerequisites

- Go 1.26.2+
- Python 3 + pip (for AI training only)
- Node 18+ + npm (for editor frontend only)
- `.env` populated from `.env.example` (missing keys panic on boot)

## Quick start

```bash
cp .env.example .env
go mod tidy
make run
```

Fresh DB: `rm -rf data/` then `make run` re-applies migrations + seed.

## Make targets

### Game

| Target | Description |
|---|---|
| `make run` | Launch game (`./cmd/game`) |
| `make test` | Run all unit tests |
| `make tidy` | `go mod tidy` |
| `make tune ARGS="..."` | Tuning CLI passthrough |

### Editor

| Target | Description |
|---|---|
| `make editor` | Start Fiber backend on `EDITOR_PORT` (default 8080) |
| `make web-install` | npm install for `tools/editor-web` |
| `make web` | Vite dev server on :5173 (proxies `/api` → :8080) |
| `make web-build` | Production bundle |

### AI training

| Target | Description |
|---|---|
| `make ai-setup` | Install Python deps from `ai/requirements.txt` |
| `make train-server ENVS=4` | Headless game server on :9876 for player RL |
| `make train STEPS=1000000 ENVS=4` | Train player PPO agent |
| `make train-resume STEPS=500000` | Resume from `checkpoints/ppo_latest.zip` |
| `make train-visual STEPS=50000` | Train with game window visible |
| `make train-eval EPISODES=100` | Evaluate `checkpoints/ppo_final.zip` |
| `make train-play MODEL=...` | Watch player AI play with full rendering |
| `make train-tensorboard` | TensorBoard on `ai/logs/` |
| `make train-clean` | Wipe `ai/checkpoints/` + `ai/logs/` |
| `make train-orc-server` | Headless server for orc RL |
| `make train-orc STEPS=500000 PLAYER_MODEL=...` | Train orc vs frozen player model |
| `make train-orc-visual STEPS=50000` | Orc training with visual |
| `make play-both PLAYER_MODEL=... ORC_MODEL=...` | Watch player AI vs orc AI |

## Controls

| Action | Keys |
|---|---|
| Move | `A`/`D`, arrows (held) |
| Jump | `Space` (edge, grounded only) |
| Sprint | `Shift` + direction |
| Attack | `J` or `X` |
| Attack2 | `K` or `C` |
| Debug overlay | `F3` |
| Hitbox debug | `F4` |
| Reload behavior JSON | `F5` |
| Pause | `Esc` |
| Resume | Any key (while paused) |
| Restart on GAME OVER | `R` |

## Layout

```
cmd/
  game/        Ebiten entry point (-ai, -ai-both, -ai-orc flags)
  tune/        Tuning + HUD CLI
  editor/      Fiber HTTP server for BT editor FE
  train/       Headless server for player RL
  train-orc/   Headless server for orc RL
internal/
  config/      godotenv loader
  storage/     SQLite + migrations + Repository[T]
  anim/        Sprite-sheet slicer + library
  combat/      Hitboxes + Fighter + Resolve
  behavior/    JSON BT runtime (Selector/Sequence/Chance/Wait/Action/Condition)
  enemy/       Generic Kind-driven enemy; orc + slime registered
  spawner/     Interval + cap roll
  stamina/     Sprint stamina pool
  score/       Kill scoring
  hud/         Heart + stamina + score draw, hud_layout loader, pause overlay
  input/       Keyboard intent
  player/      Soldier FSM + physics
  world/       Ground + clamp
  debug/       F3 overlay catalog + JSON config
  game/        Ebiten Game wiring; ModePlaying/Paused/GameOver
  editor/      Hexagonal http/service/port/adapter for editor backend
  aienv/       RL env, observation/action spec, reward shaping
assets/
  behaviors/   Per-kind BT JSON (orc.json, slime.json)
  ...          Sprite sheets, HUDs, font
config/
  debug.json   Debug overlay layout (F3)
data/
  game.db      SQLite (regenerable from migrations)
ai/
  train.py, train_orc.py, eval.py, play.py, play_both.py — PPO pipeline
tools/
  editor-web/  React + Vite + Tailwind + shadcn + React Flow BT editor
docs/
  superpowers/specs/  Design + plan docs
```

## Tuning CLI

```bash
make tune ARGS="list"                       # all 27 tuning keys
make tune ARGS="set run_speed 320"
make tune ARGS="hud list"
make tune ARGS="hud set heart x 12"
```

Agent invocations: pass `--agent-mode` or `TUNE_AGENT_MODE=1` for compact TSV.

Full key list + ranges in [CLAUDE.md](CLAUDE.md). Output schema in [docs/tune-cli.md](docs/tune-cli.md).

## Editor

Backend serves under `/api/behaviors`, `/api/tuning`, `/api/registry/...`. Reuses `internal/behavior` validator + `internal/storage` repo as single source of truth — FE pre-validates with same rules; both must pass for save.

See [tools/editor-web/README.md](tools/editor-web/README.md).

## Migrations

Schema → edit `internal/storage/migrations/001_init_schema.sql` in place. Seed → edit `002_seed_data.sql` in place. Never add `003_*.sql`. Wipe `data/` between test runs.

## Tests

```bash
go test ./...
```

Covers config loader, Repository CRUD, animation math, FSM transitions (sprint + attack-cancel), tuning validator, debug-config unknown-field rejection.

## License

MIT.
