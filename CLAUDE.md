# claude-pixel — Claude-facing project guide

Notes for future Claude sessions. Designs/plans:
- `docs/superpowers/specs/2026-04-21-char1-controller-design.md` + plan
- `docs/superpowers/specs/2026-04-23-combat-and-enemy-design.md` + plan

## Layout

```
cmd/
  game/main.go         # ebiten entry point
  tune/main.go         # CLI to read / update tuning values
config/debug.json      # debug overlay layout (F3 toggle)
data/game.db           # SQLite; regenerable from migrations
internal/
  config/              # godotenv loader (panics on missing env)
  storage/             # sqlite Open + migrations/*.sql + Repository[T]; includes hud_layout table
  anim/                # AnimationSpec (per-spec frame dims + path + grid), sheet slicer, LoadLibrary
  combat/              # Box + HitboxSpec mapper, Fighter, Resolve, Tuning, AttackMotionSpec (motion.go)
  enemy/               # Enemy + Kind (kind.go) with AnimPrefix/FrameW/FrameH/Tuning/Boxes/Anims/Motions; FSM (fall/run/attack/attack2/hurt/death), generic AnimsFor/BoxesFor/MotionsFor (loader.go), LoadTuningFor, LoadSpawnTuning
  spawner/             # timer + interval roll + cap enforcement
  stamina/             # stamina pool; drain while sprinting, regen while not; gates sprint
  score/               # score accumulator; incremented on enemy kill by kind-specific points value
  hud/                 # monogram font loader, HUD (heart+lives+stamina+score), layout.go (hud_layout loader), pause.go (pause overlay)
  input/               # Poll() -> Intent (held + edge keys)
  player/              # Physics, Player, FSM with Hit+Death states, combat.Fighter impl
  world/               # flat ground, Clamp helper
  debug/               # field catalog, JSON config, overlay
  game/                # ebiten Game: wires enemies + spawner + combat dispatch + HUD + state; modes: ModePlaying, ModePaused, ModeGameOver
```

## Run

```bash
make run          # launch game
make test         # all unit tests
make tune ARGS="list"
make tune ARGS="set run_speed 320"
```

Env from `.env` (template `.env.example`). Missing keys panic at boot. Fresh DB: `rm -rf data/` then `make run` re-runs all migrations.

## Tuning CLI (`cmd/tune`)

Inspect/adjust physics params without editing SQL. Values in `tuning` table. `set` validates vs row `min_value`/`max_value`, rejects unknown keys.

### List every parameter

```bash
go run ./cmd/tune list
# or
make tune ARGS="list"
```

Output: tabwriter table `KEY VALUE MIN MAX UNIT DESCRIPTION`.

Current keys (26):

Physics (6):
| Key | Unit | Effect |
|---|---|---|
| `run_speed` | px/s | Horizontal ground speed |
| `sprint_speed` | px/s | Speed when Shift held (≈1.5× run_speed) |
| `air_control` | — | Air horizontal multiplier (0..1) |
| `jump_velocity` | px/s | Jump impulse (negative = up) |
| `gravity` | px/s² | Downward accel per tick |
| `max_fall_speed` | px/s | Terminal vertical velocity |

Stamina (3):
| Key | Unit | Effect |
|---|---|---|
| `stamina_max` | — | Max stamina pool (default 100) |
| `stamina_drain_per_s` | /s | Drain rate while sprinting |
| `stamina_regen_per_s` | /s | Regen rate while not sprinting |

Combat + enemy (17):
| Key | Unit | Effect |
|---|---|---|
| `soldier_max_lives` | — | Starting soldier lives (default 10) |
| `soldier_knockback_vx` | px/s | Horizontal bounce when hit |
| `soldier_knockback_vy` | px/s | Upward pop when hit (airborne i-frame) |
| `soldier_foot_padding` | px | Transparent px at soldier sprite frame bottom |
| `orc_max_lives` | — | Starting orc lives (default 2) |
| `orc_hurt_bounce_vx` | px/s | Horizontal bounce on hurt |
| `orc_hurt_bounce_vy` | px/s | Upward pop on hurt |
| `orc_foot_padding` | px | Transparent px at orc sprite frame bottom |
| `orc_points` | — | Points awarded on orc kill (default 10) |
| `slime_max_lives` | — | Starting slime lives (default 2) |
| `slime_hurt_bounce_vx` | px/s | Horizontal bounce on hurt |
| `slime_hurt_bounce_vy` | px/s | Upward pop on hurt |
| `slime_foot_padding` | px | Transparent px at slime sprite frame bottom |
| `slime_points` | — | Points awarded on slime kill (default 15) |
| `enemy_spawn_min_s` | s | Minimum enemy spawn interval (all kinds) |
| `enemy_spawn_max_s` | s | Maximum enemy spawn interval (all kinds) |
| `enemy_max_alive` | — | Concurrent enemy cap across all kinds (default 3) |

### Update one parameter

```bash
go run ./cmd/tune set <key> <value>
```

Exit 0: `OK: <key> = <new> <unit> (was <old>)`.
Exit 1, one of:
- `unknown tuning key "..."` (key not in DB)
- `value "..." is not a number` (bad parse)
- `value out of range: X not in [min, max] unit` (validator)

Changes apply next `make run`. No hot reload.

### HUD layout (`hud_layout` table)

```bash
go run ./cmd/tune hud list
go run ./cmd/tune hud get <key>
go run ./cmd/tune hud set <key> <field> <value>   # fields: x, y, w, h, anchor, scale
```

Keys: `heart`, `lives_text`, `score_text`, `stamina_bar`.
Anchors: `top_left`, `top_right`, `bottom_left`, `bottom_right`.
`x/y` = offset of the element's nearest corner from the screen anchor corner.
For text elements, stored `w/h=0` → width is measured at draw time.

## Debug overlay

Toggle **F3** in-game. Layout `config/debug.json` — edit, restart. Unknown field keys = boot-time error listing valid keys. Catalog: `internal/debug/fields.go` (25 fields: 19 player/engine + 4 orc/lives + 2 behavior). **F4** toggles hitbox debug draw (green = body, red = active attack box).

## Controls

| Action | Keys |
|---|---|
| Move | `A`/`D`, arrows (held) |
| Jump | `Space` (edge, grounded only) |
| Sprint | `Shift` held + direction |
| Attack | `J` or `X` (edge) |
| Attack2 | `K` or `C` (edge) |
| Debug overlay | `F3` (edge) |
| Hitbox debug | `F4` (edge) |
| Pause | `Esc` (edge) |
| Resume (while paused) | Any key (edge, action swallowed that tick) |
| Restart (on GAME OVER) | `R` (edge) |

Shift alone = no-op. No double-jump. Attacks cancelable by Jump only (grounded).

## State machines

**Soldier** (8 states): `Idle`, `Run`, `Jump`, `Fall`, `Attack`, `Attack2`, `Hit`, `Death`. `Hit` = bounced back + airborne i-frame until grounded. `Death` = 10 lives consumed, terminal. Sprint is gated by stamina — depletes while sprinting, regenerates otherwise.

**Orc** (6 states): `Fall` (from spawn), `Run`, `Attack`, `Attack2`, `Hurt`, `Death`. Every `orc_intent_tick_s`, Run either stays or 50/50 picks `Attack`/`Attack2`. 2 lives — second hit kills. Hurt anim = i-frame window. `internal/enemy/states.go`.

**Slime** (6 states): identical FSM to orc — `Fall` → `Run` → `Attack`/`Attack2` → `Hurt`/`Death`, using `slime_intent_tick_s`. Attack2 applies a backward VX slide on frames 3–5 (configurable via `attack_motions` table → row `slime_attack2_motion`).

## Combat + hitboxes

Hitbox table seeded by migration 012. Each fighter has a body box (always-on) and attack/attack2 boxes (frame-windowed). `combat.Resolve(attackers, victims)` returns `HitEvent`s via AABB overlap, respecting facing flip, invulnerability, and per-swing dedup. Soldier attack → enemy.OnHit (decrement, bounce or die). Enemy attack → player.OnHit (knockback + airborne i-frame until land).

Hitbox dims stored in `hitboxes` table (not in `tuning`). Retune via new migration.

**Attack motions** (`attack_motions` table, migration 019) optionally apply a per-frame-window VX slide during an attack state. VX is signed: positive = forward along facing, negative = backward. Seeded for slime Attack2 only (`slime_attack2_motion`: vx=-60, frames 3–5). Retune via `tune motions set <id> <field> <value>`. Orcs have no motion rows; their attacks keep VX=0.

## Spawner

`internal/spawner` is multi-kind: rolls interval uniformly from `[enemy_spawn_min_s, enemy_spawn_max_s]`, caps at `enemy_max_alive` across all kinds combined, then weighted-rolls which `Kind` to spawn (currently orc + slime). Spawn position = random X above screen (`Y = -kind.FrameH*renderScale`). Enemy enters `fall` → `run` on land.

## HUD + font

Heart anim from `assets/huds/healthbar/heartbeat.png` (row 3 of 4×6 grid, 4 frames, 400ms loop) + monogram-font `xN` lives counter. Stamina bar drawn from `assets/huds/healthbar/healthbar.png`. Score text shown top-left. Element positions loaded from `hud_layout` table via `internal/hud/layout.go`. GAME OVER overlay (dim + "GAME OVER" @96 + "Press R to restart" @32) on soldier death. Pause overlay (dim + "PAUSED" + "Press any key to resume") on `ModePaused`. Font loaded from `FONT_PATH` env (`./assets/fonts/monogram/ttf/monogram.ttf`) via `text/v2`.

## Migrations

`internal/storage/migrations/*.sql` embedded via `//go:embed`, applied in order by `internal/storage/migrations.go`. Tracked in `schema_migrations`. Never edit applied migration — add new numbered file.

## Tests

```bash
go test ./...
```

Covers: config loader, Repository CRUD, Animation math, FSM transitions (incl. sprint + attack-cancel-by-jump), tuning validator, debug config unknown-field rejection.

Ebiten rendering + sprite slicing verified manually — see T18 checklist in plan doc.