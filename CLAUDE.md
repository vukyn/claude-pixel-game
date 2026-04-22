# claude-pixel — Claude-facing project guide

Short notes for future Claude sessions. Full design lives in
`docs/superpowers/specs/2026-04-21-char1-controller-design.md` and
`docs/superpowers/plans/2026-04-21-char1-controller.md`.

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
  anim/                # AnimationSpec, sheet slicer, runtime, LoadLibrary
  input/               # Poll() -> Intent (held + edge keys)
  player/              # Physics, Player, FSM, 6 states, tuning validator
  world/               # flat ground at WindowH - 120
  debug/               # field catalog, JSON config, overlay
  game/                # ebiten Game: wires everything
```

## Run

```bash
make run          # launch game
make test         # all unit tests
make tune ARGS="list"
make tune ARGS="set run_speed 320"
```

Env loaded from `.env` (template in `.env.example`). Missing keys panic
at boot. Fresh DB: `rm -rf data/` then `make run` re-runs all migrations.

## Tuning CLI (`cmd/tune`)

Use this to inspect or adjust physics parameters without editing SQL
directly. Values live in the `tuning` table; `set` validates against
each row's `min_value`/`max_value` and rejects unknown keys.

### List every parameter

```bash
go run ./cmd/tune list
# or
make tune ARGS="list"
```

Output: tabwriter table with `KEY VALUE MIN MAX UNIT DESCRIPTION`.

Current keys (6):

| Key | Unit | Effect |
|---|---|---|
| `run_speed` | px/s | Horizontal ground speed |
| `sprint_speed` | px/s | Speed when Shift held (≈1.5× run_speed) |
| `air_control` | — | Air horizontal multiplier (0..1) |
| `jump_velocity` | px/s | Jump impulse (negative = up) |
| `gravity` | px/s² | Downward accel per tick |
| `max_fall_speed` | px/s | Terminal vertical velocity |

### Update one parameter

```bash
go run ./cmd/tune set <key> <value>
```

Exits 0 with `OK: <key> = <new> <unit> (was <old>)` on success.
Exits 1 with one of:
- `unknown tuning key "..."` (key not in DB)
- `value "..." is not a number` (bad parse)
- `value out of range: X not in [min, max] unit` (validator)

Tuning changes take effect on next `make run` (no hot reload).

## Debug overlay

Toggle with **F3** in-game. Layout is `config/debug.json` — edit,
restart. Unknown field keys cause boot-time error listing all valid
keys. Source catalog: `internal/debug/fields.go` (19 fields total).

## Controls

| Action | Keys |
|---|---|
| Move | `A`/`D`, arrows (held) |
| Jump | `Space` (edge, grounded only) |
| Sprint | `Shift` held + direction |
| Attack | `J` or `X` (edge) |
| Attack2 | `K` or `C` (edge) |
| Debug | `F3` (edge) |

Shift alone is a no-op. No double-jump. Attacks cancelable by Jump only
(grounded).

## State machine

6 states: `Idle`, `Run`, `Jump`, `Fall`, `Attack`, `Attack2`. See
`internal/player/states.go` and the mermaid diagram in the design spec
(section 7). Each state is a `struct{}` with value-receiver methods
registered pointerwise in `player.New`.

## Migrations

All `internal/storage/migrations/*.sql` embedded via `//go:embed`,
applied in order by `internal/storage/migrations.go`. Tracked in
`schema_migrations`. Never edit an already-applied migration — add a
new numbered file.

## Tests

```bash
go test ./...
```

Covers: config loader, Repository CRUD, Animation math, FSM
transitions (incl. sprint + attack-cancel-by-jump), tuning validator,
debug config unknown-field rejection.

Ebiten rendering and sprite slicing verified manually — see T18
checklist in the plan doc.
