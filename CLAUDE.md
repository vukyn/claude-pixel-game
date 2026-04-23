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
  storage/             # sqlite Open + migrations/*.sql + Repository[T]
  anim/                # AnimationSpec (per-spec frame dims + path + grid), sheet slicer, LoadLibrary
  combat/              # Box + HitboxSpec mapper, Fighter interface, Resolve, Tuning
  enemy/               # Enemy + FSM (fall/run/attack/attack2/hurt/death), Tuning, OrcAnims/OrcBoxes
  spawner/             # timer + interval roll + cap enforcement
  hud/                 # monogram font loader, HUD (heart+lives), GameOver overlay
  input/               # Poll() -> Intent (held + edge keys)
  player/              # Physics, Player, FSM with Hit+Death states, combat.Fighter impl
  world/               # flat ground, Clamp helper
  debug/               # field catalog, JSON config, overlay
  game/                # ebiten Game: wires enemies + spawner + combat dispatch + HUD + state
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

Current keys (17):

Physics (6):
| Key | Unit | Effect |
|---|---|---|
| `run_speed` | px/s | Horizontal ground speed |
| `sprint_speed` | px/s | Speed when Shift held (≈1.5× run_speed) |
| `air_control` | — | Air horizontal multiplier (0..1) |
| `jump_velocity` | px/s | Jump impulse (negative = up) |
| `gravity` | px/s² | Downward accel per tick |
| `max_fall_speed` | px/s | Terminal vertical velocity |

Combat + enemy (11):
| Key | Unit | Effect |
|---|---|---|
| `soldier_max_lives` | — | Starting soldier lives (default 10) |
| `soldier_knockback_vx` | px/s | Horizontal bounce when hit |
| `soldier_knockback_vy` | px/s | Upward pop when hit (airborne i-frame) |
| `orc_max_lives` | — | Starting orc lives (default 2) |
| `orc_run_speed` | px/s | Orc ground speed |
| `orc_hurt_bounce_vx` | px/s | Horizontal bounce on hurt |
| `orc_hurt_bounce_vy` | px/s | Upward pop on hurt |
| `orc_intent_tick_s` | s | Interval for run-vs-attack reroll |
| `orc_spawn_min_s` | s | Minimum spawn interval |
| `orc_spawn_max_s` | s | Maximum spawn interval |
| `orc_max_alive` | — | Concurrent orc cap (default 3) |

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

## Debug overlay

Toggle **F3** in-game. Layout `config/debug.json` — edit, restart. Unknown field keys = boot-time error listing valid keys. Catalog: `internal/debug/fields.go` (23 fields: 19 player/engine + 4 orc/lives). **F4** toggles hitbox debug draw (green = body, red = active attack box).

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
| Restart (on GAME OVER) | `R` (edge) |

Shift alone = no-op. No double-jump. Attacks cancelable by Jump only (grounded).

## State machines

**Soldier** (8 states): `Idle`, `Run`, `Jump`, `Fall`, `Attack`, `Attack2`, `Hit`, `Death`. `Hit` = bounced back + airborne i-frame until grounded. `Death` = 10 lives consumed, terminal.

**Orc** (6 states): `Fall` (from spawn), `Run`, `Attack`, `Attack2`, `Hurt`, `Death`. Every `orc_intent_tick_s`, Run either stays or 50/50 picks `Attack`/`Attack2`. 2 lives — second hit kills. Hurt anim = i-frame window. `internal/enemy/states.go`.

## Combat + hitboxes

Hitbox table seeded by migration 012. Each fighter has a body box (always-on) and attack/attack2 boxes (frame-windowed). `combat.Resolve(attackers, victims)` returns `HitEvent`s via AABB overlap, respecting facing flip, invulnerability, and per-swing dedup. Soldier attack → orc.OnHit (decrement, bounce or die). Orc attack → player.OnHit (knockback + airborne i-frame until land).

Hitbox dims stored in `hitboxes` table (not in `tuning`). Retune via new migration.

## Spawner

`internal/spawner` rolls interval uniformly from `[orc_spawn_min_s, orc_spawn_max_s]`, caps at `orc_max_alive`, spawns at random X above screen (`Y = -sprite_h*renderScale`). Orc enters `fall` → `run` on land.

## HUD + font

Top-right: heart anim (row 3 of 4×6 grid, 4 frames, 400ms loop) + monogram-font `xN` lives counter. GAME OVER overlay (dim + "GAME OVER" @96 + "Press R to restart" @32) on soldier death. Font loaded from `FONT_PATH` env (`./assets/fonts/monogram/ttf/monogram.ttf`) via `text/v2`.

## Migrations

`internal/storage/migrations/*.sql` embedded via `//go:embed`, applied in order by `internal/storage/migrations.go`. Tracked in `schema_migrations`. Never edit applied migration — add new numbered file.

## Tests

```bash
go test ./...
```

Covers: config loader, Repository CRUD, Animation math, FSM transitions (incl. sprint + attack-cancel-by-jump), tuning validator, debug config unknown-field rejection.

Ebiten rendering + sprite slicing verified manually — see T18 checklist in plan doc.