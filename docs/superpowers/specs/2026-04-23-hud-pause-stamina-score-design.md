# HUD Overhaul: Pause, Stamina, Score, Data-Driven Layout

**Date:** 2026-04-23
**Status:** Design approved — awaiting implementation plan

## Purpose

Add four player-facing systems and one infrastructure change:

1. **Pause** — Esc suspends simulation; any key resumes (action swallowed on that tick).
2. **Stamina** — sprint drains a 100%→0% pool in 5s; hard cut to run_speed at 0; symmetric regen (5s full refill) whenever not sprinting.
3. **Score** — top-left counter; per-kind points awarded on enemy death; reset on restart.
4. **Heart asset path** — move from `assets/sprites/heart/` (deleted) to `assets/huds/healthbar/heartbeat.png`.
5. **HUD layout in storage** — x/y/w/h/anchor/scale per HUD element, tunable via CLI.

## Non-Goals

- No stamina for enemies. No stamina cost for attacks (sprint-only).
- No score persistence across sessions. In-memory only.
- No HUD redesign beyond placement of new elements.
- No hot reload — layout changes apply next `make run` (matches existing tuning flow).

## Architecture

```
internal/
  stamina/         NEW  Pool{max, cur, drainRate, regenRate} + Update(dt, sprinting bool)
  score/           NEW  Counter{total int} + Add(int) + Reset() + Total() int
  hud/             EXT  layout loader, stamina bar, score text, pause overlay
    layout.go      NEW  HUDLayout map[string]Element{X, Y, W, H, Anchor, Scale}
    pause.go       NEW  draw pause overlay
  player/          EXT  stamina field on Player; sprint gated by stamina>0
  game/            EXT  Mode enum (Playing/Paused/GameOver); pause input; score dispatch on enemy death
  enemy/           EXT  Kind.Points field, loaded from tuning
cmd/tune/          EXT  new `hud list|get|set` subcommand
```

### Control flow (per tick)

```
Update(dt):
  switch game.mode:
    ModePaused:
      if any key edge -> mode = ModePlaying; swallow edge this tick (see Pause Input)
      return  // no sim
    ModeGameOver:
      existing behavior
    ModePlaying:
      intent = input.Poll()
      if intent.EscEdge: mode = ModePaused; return
      player.Update(intent, dt)        // drains stamina if sprinting
      spawner.Update(dt); enemies.Update(dt)
      combat.Resolve(...)              // on enemy death -> score.Add(kind.Points)
      hud.Update(dt)                   // heart anim ticks
```

## Components

### 1. Pause (`internal/game`)

**Mode enum** replaces `gameOver bool`:

```go
type Mode int
const (
    ModePlaying Mode = iota
    ModePaused
    ModeGameOver
)
```

**Esc handling:** `input.Intent` gains `PauseEdge bool` (bound to `KeyEscape` edge). Toggle only `Playing → Paused` (not the reverse — any key handles that).

**Resume handling:** while `ModePaused`, poll raw ebiten keys (not Intent, which bundles gameplay keys). If any key just-pressed (`inpututil.IsKeyJustPressed` over all keys, or simpler: non-empty `inpututil.AppendJustPressedKeys`), set `mode = ModePlaying` AND mark the Intent frame as "swallow" so player doesn't jump/attack on resume.

**Swallow mechanism:** `Game.swallowNextIntent bool`. Set true on resume. Next `Update` tick, after polling intent, zero out the intent edges if flag set, then clear flag. Held keys continue normally — only edges (Jump, Attack, Attack2) are swallowed.

**Overlay:** `internal/hud/pause.go` — semi-transparent dim + "PAUSED" @96 + "Press any key to resume" @32, centered. Reuse font face from GameOver.

**Frozen during pause:** physics, anim.Update, spawner timer, enemy FSM, stamina regen, heart beat. Only the mode-check + input poll run.

### 2. Stamina (`internal/stamina`)

**Pool:**

```go
type Pool struct {
    Max, Cur         float64  // 0..Max
    DrainPerSec      float64  // Max / 5 = 20 if Max=100
    RegenPerSec      float64  // Max / 5 = 20
}
func (p *Pool) Update(dt time.Duration, sprinting bool)
func (p *Pool) Fraction() float64   // Cur/Max, 0..1
func (p *Pool) CanSprint() bool     // Cur > 0
```

**Tuning keys** (new migration):

| Key | Unit | Default | Effect |
|---|---|---|---|
| `stamina_max` | — | 100 | Max pool value |
| `stamina_drain_per_s` | /s | 20 | Drain rate while sprinting (100/5s=20) |
| `stamina_regen_per_s` | /s | 20 | Regen rate while not sprinting |

**Player integration** (`internal/player/player.go`):
- `Player.Stamina *stamina.Pool` field
- In `Update`: determine `sprinting := intent.SprintHeld && intent.Moving && pool.CanSprint() && grounded`. If sprinting, use `sprint_speed`; else `run_speed`. Call `pool.Update(dt, sprinting)`.
- Hard cut: if `pool.Cur == 0` mid-sprint, velocity clamps to run_speed that tick (not sprint_speed).
- Airborne: sprint does not drain stamina (sprint only applies on ground in current physics — preserve).

**StaminaProvider interface** (HUD-facing):

```go
type StaminaProvider interface { StaminaFraction() float64 }
```

### 3. Score (`internal/score`)

```go
type Counter struct { total int }
func (c *Counter) Add(n int)
func (c *Counter) Total() int
func (c *Counter) Reset()
```

**Per-kind points** — new tuning keys `orc_points=10`, `slime_points=15`. Loaded into `enemy.Kind.Points` via existing `LoadTuningFor` path (add one field).

**Dispatch** (`internal/game`):
- Track enemy deaths. When an enemy FSM enters `Death` state and anim finishes (existing cleanup), call `score.Add(kind.Points)`.
- On soldier restart (R key in GameOver): `score.Reset()` alongside existing state reset.

**ScoreProvider interface:**

```go
type ScoreProvider interface { Score() int }
```

### 4. Heart asset migration

New migration `025_move_heart_to_huds.sql`:

```sql
UPDATE animations
   SET path = 'huds/healthbar/heartbeat.png',
       file = 'heartbeat.png'
 WHERE id = 'heart_beat';
```

No dimension change — still 16×16, 4×6 grid, pick_row=3 (row 4 visually), 4 frames, 400ms.

Asset already at `assets/huds/healthbar/heartbeat.png` (verified 64×96). Old folder `assets/sprites/heart/` already deleted in working tree.

### 5. Stamina bar rendering

**Sheet:** `assets/huds/healthbar/healthbar.png` (192×160, 4 cols × 10 rows, frame 48×16).

**Frames used:** col index 2 (0-indexed, 3rd from left), 10 frames top→bottom. Frame 0 = full, frame 9 = empty.

**Schema extension** — migration `026_animations_add_pick_col.sql`:

```sql
ALTER TABLE animations ADD COLUMN pick_col INTEGER NOT NULL DEFAULT -1;
-- -1 = use pick_row (existing behavior). >=0 = slice single column.
```

**Loader change** (`internal/anim/spec.go` + slicer): if `pick_col >= 0`, slice a single column (N frames vertically) instead of a row. Both exclusive.

**Seed** — `027_seed_stamina_bar_animation.sql`:

```sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path,
     is_player, is_enemy, grid_cols, grid_rows, pick_row, pick_col)
VALUES
    ('stamina_bar', 'healthbar.png', 10, 0, 0, 48, 16,
     'huds/healthbar/healthbar.png', 0, 0, 4, 10, 0, 2);
```

`duration_ms=0` + `loop=0` — not a time-driven animation. HUD picks frame manually:

```go
idx := int((1.0 - pool.Fraction()) * float64(frameCount-1) + 0.5)  // round
if idx < 0 { idx = 0 } else if idx > 9 { idx = 9 }
```

### 6. HUD layout in storage

**Schema** — migration `028_hud_layout_schema.sql`:

```sql
CREATE TABLE hud_layout (
    key     TEXT PRIMARY KEY,
    x       INTEGER NOT NULL,
    y       INTEGER NOT NULL,
    w       INTEGER NOT NULL,
    h       INTEGER NOT NULL,
    anchor  TEXT NOT NULL CHECK(anchor IN ('top_left','top_right','bottom_left','bottom_right')),
    scale   REAL NOT NULL DEFAULT 1.0
);
```

**Seed** — migration `029_seed_hud_layout.sql`:

| key | x | y | w | h | anchor | scale | notes |
|---|---|---|---|---|---|---|---|
| `heart` | 48 | 16 | 16 | 16 | top_right | 2.0 | Offset x includes text gap |
| `lives_text` | 16 | 16 | 0 | 0 | top_right | 1.0 | w/h=0 → measure at draw |
| `score_text` | 16 | 16 | 0 | 0 | top_left | 1.0 | |
| `stamina_bar` | 16 | 48 | 48 | 16 | top_left | 2.0 | Below score |

**x/y interpretation:** offset from anchor corner of the element's nearest edge to that corner.

- `top_left`: (x, y) = offset of element's top-left from screen top-left
- `top_right`: (x, y) = offset of element's top-**right** from screen top-right (grows left/down)
- `bottom_left`: (x, y) = offset of element's bottom-left from screen bottom-left (grows right/up)
- `bottom_right`: (x, y) = offset of element's bottom-right from screen bottom-right (grows left/up)

For text elements (stored W=0), width is measured at draw time and used in place of W.

**Loader** — `internal/hud/layout.go`:

```go
type Anchor int
const ( AnchorTopLeft Anchor = iota; AnchorTopRight; AnchorBottomLeft; AnchorBottomRight )

type Element struct { X, Y, W, H int; Anchor Anchor; Scale float64 }

type Layout map[string]Element

func LoadLayout(db *sql.DB) (Layout, error)
func (l Layout) Resolve(key string, screenW, screenH int) (px, py float64)  // returns absolute top-left in screen px
```

Boot-time load. Missing required key (`heart`, `lives_text`, `score_text`, `stamina_bar`) → panic with key name, same pattern as `debug` config.

**HUD struct** — extend:

```go
type HUD struct {
    Heart          *anim.Animation
    StaminaBar     *anim.Animation  // frame-indexed, no autoplay
    Face           *text.GoTextFace
    Lives          LivesProvider
    Stamina        StaminaProvider
    Score          ScoreProvider
    Layout         Layout
    WindowW, WindowH int
}
```

Draw method uses `Layout.Resolve` per element. No hardcoded `padding`/`gap` constants remain.

**Tune CLI** — `cmd/tune/main.go` adds `hud` subcommand mirroring `motions`:

```bash
go run ./cmd/tune hud list
go run ./cmd/tune hud get <key>
go run ./cmd/tune hud set <key> <field> <value>   # fields: x, y, w, h, anchor, scale
```

Validator: anchor ∈ enum, scale > 0, coords integer.

## Data Flow

```
player sprint (held) ────┐
                         ├──► stamina.Pool.Update ──► StaminaProvider.Fraction ──► HUD frame index ──► stamina_bar draw
game tick (dt) ──────────┘

enemy.FSM Death ──► game dispatch ──► score.Counter.Add(kind.Points) ──► ScoreProvider.Score ──► HUD score text

Esc edge ──► game.mode = Paused ──► Update skip sim ──► pause overlay draw
any key edge (paused) ──► game.mode = Playing + swallowNextIntent=true ──► next tick zero edges
```

## Error Handling

- Missing HUD layout key → boot panic listing expected keys (`heart`, `lives_text`, `score_text`, `stamina_bar`) + found keys.
- Unknown anchor in DB → migration CHECK constraint rejects at insert; loader treats this as "should never happen" (panic).
- `pick_col` + `pick_row` both set (≥0) on same animation row → loader error: "animation X: pick_row and pick_col are mutually exclusive".
- Stamina pool drains below 0 due to rounding → clamped to 0 in `Pool.Update`.
- Score overflow (int) → not handled; unreachable in practice.
- Pause overlap with GameOver: Esc ignored while `ModeGameOver` (only R resumes via restart).

## Testing

Unit tests:

- `internal/stamina/pool_test.go` — drain to 0 in 5s at 20/s; regen to max in 5s; CanSprint gating; clamp.
- `internal/score/counter_test.go` — Add, Reset, Total.
- `internal/hud/layout_test.go` — Resolve for each anchor; missing-key panic; unknown-anchor rejection.
- `internal/anim` — pick_col slicing produces N frames of correct pixel bounds; pick_row vs pick_col mutual exclusion.
- `internal/game` — mode transitions: Playing→Paused on Esc edge; Paused→Playing on any key; swallow flag zeros next-tick edges; Paused ignored under GameOver.
- `cmd/tune` — `hud set` validates anchor enum, scale > 0; `hud get` unknown key errors.

Manual verification (ebiten can't unit-test render):

- Sprint 5s → bar drains frame-by-frame → hits empty → sprint hard-cuts to run.
- Release Shift → bar fills in 5s.
- Kill orc → score += 10. Kill slime → score += 15. Restart → score = 0.
- Heart anim still plays post-migration (asset loads from new path).
- Stamina bar visually below score text, top-left anchored.
- Resize window (if applicable) — top-right elements stay anchored right.
- Esc pauses; any key resumes without triggering jump/attack.
- `tune hud set heart x 64` → next run, heart shifts left by 16px (2x scale applied).

## Migrations Summary

| # | File | Purpose |
|---|---|---|
| 025 | `025_move_heart_to_huds.sql` | UPDATE path/file for `heart_beat` animation |
| 026 | `026_animations_add_pick_col.sql` | ALTER TABLE animations ADD pick_col |
| 027 | `027_seed_stamina_bar_animation.sql` | INSERT stamina_bar animation row |
| 028 | `028_hud_layout_schema.sql` | CREATE TABLE hud_layout |
| 029 | `029_seed_hud_layout.sql` | Seed 4 HUD element rows |
| 030 | `030_seed_stamina_tuning.sql` | stamina_max, stamina_drain_per_s, stamina_regen_per_s |
| 031 | `031_seed_enemy_points.sql` | orc_points=10, slime_points=15 |

## Open Questions

None — all resolved during brainstorming.

## Out-of-Scope Follow-Ups

- Stamina cost per attack / per jump (future combat balance pass).
- Score multiplier on combo kills.
- Persistent high score (requires save-state subsystem).
- HUD resize responsiveness (current design uses fixed scale; fine for single window size).
- Animated stamina bar border / glow effects.
