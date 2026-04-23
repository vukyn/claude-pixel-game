# Combat & Enemy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `claude-pixel` from solo-soldier demo → stage with auto-spawning orc enemies, hitbox-based combat, soldier Hit/Death states with 10 lives, a monogram-font heart HUD, and a Game Over / restart loop. All within the existing ebiten + SQLite + Go architecture.

**Architecture:** Keep existing pkg layout; add siblings `internal/enemy`, `internal/combat`, `internal/spawner`, `internal/hud`. Extend schema with per-spec frame size/path/flags, a `hitboxes` table, and combat tuning rows. Soldier and orc both implement a `combat.Fighter` interface; the resolver is the only cross-package coupling point.

**Tech Stack:** Go 1.26 · `github.com/hajimehoshi/ebiten/v2 v2.9.9` (uses `text/v2` for font rendering) · `modernc.org/sqlite` · existing `Repository[T]` generic storage.

**Spec:** `docs/superpowers/specs/2026-04-23-combat-and-enemy-design.md`

---

## File Structure

**Created:**

```
internal/combat/
  box.go                  # Box type + Active(frame)
  box_mapper.go           # HitboxSpec + Mapper for storage
  fighter.go              # Fighter interface + HitEvent
  resolve.go              # Resolve(attackers, victims) []HitEvent
  resolve_test.go

internal/enemy/
  enemy.go                # Enemy struct + Config + constructor + Fighter impl
  fsm.go                  # FSM (analogous to player.FSM) + StateID consts
  states.go               # 6 states: fall, run, attack, attack2, hurt, death
  tuning.go               # Load enemy-specific tuning
  fsm_test.go

internal/spawner/
  spawner.go              # Spawner struct + Tick
  spawner_test.go

internal/hud/
  font.go                 # GoTextFaceSource loader, NewFace(size)
  hud.go                  # HUD struct + Draw (heart + lives text)
  gameover.go             # GameOver overlay draw
  hud_test.go

internal/storage/migrations/
  007_rename_char1_to_soldier.sql
  008_animations_schema_v2.sql
  009_seed_orc_animations.sql
  010_seed_heart_animation.sql
  011_hitboxes_schema.sql
  012_seed_hitboxes.sql
  013_seed_combat_tuning.sql
```

**Modified:**

```
.env                                 # ASSETS_DIR changes, drop SPRITE_FRAME_W/H
.env.example                         # same
internal/config/config.go            # drop SpriteFrameW/H fields
internal/anim/animation.go           # add FrameW, FrameH, Path, IsPlayer, IsEnemy, GridCols, GridRows, PickRow to AnimationSpec
internal/anim/spec.go                # extend SpecMapper columns
internal/anim/sheet.go               # add SliceGrid
internal/anim/library.go             # read per-spec dims/path, branch strip vs grid
internal/world/world.go              # add Clamp(x, min, max)
internal/player/player.go            # add Lives, HitFlag, Boxes, HitSet, CurrentAnim, ApplyPhysics semantics unchanged; New signature
internal/player/states.go            # add hitState, deathState; clear HitSet on attack/attack2 Enter; set CurrentAnim
internal/player/fsm.go               # add StateHit, StateDeath
internal/player/fsm_test.go          # add hit/death transition tests
internal/debug/fields.go             # 4 new fields + broaden FieldSource
internal/game/game.go                # enemies slice, spawner, hud, state, combat dispatch, F4, R-restart
cmd/game/main.go                     # load hitboxes repo, build enemies tuning, spawner, hud font
Makefile                             # (if any new make target needed) — probably unchanged
```

---

## Phase 0 — Prework

### Task 0: Verify ebiten/v2/text/v2 availability

**Files:**
- Check: `go.mod`, import test in a scratch file

- [ ] **Step 1: Confirm ebiten version includes text/v2**

Run: `go doc github.com/hajimehoshi/ebiten/v2/text/v2`

Expected: docs print describing `NewGoTextFaceSource`, `GoTextFace`, `Draw`. If not available, stop and escalate — spec assumes `text/v2`.

- [ ] **Step 2: Confirm monogram font file is present**

Run: `ls -la assets/fonts/monogram/ttf/monogram.ttf`

Expected: file exists, ~10KB.

No commit for this task (verification only).

---

## Phase 1 — Schema + asset loader

### Task 1: Migration 007 (rename char1 rows to soldier_*)

**Files:**
- Create: `internal/storage/migrations/007_rename_char1_to_soldier.sql`

- [ ] **Step 1: Write migration SQL**

```sql
-- 007_rename_char1_to_soldier.sql
UPDATE animations SET id = 'soldier_idle'    WHERE id = 'idle';
UPDATE animations SET id = 'soldier_run'     WHERE id = 'run';
UPDATE animations SET id = 'soldier_jump'    WHERE id = 'jump';
UPDATE animations SET id = 'soldier_fall'    WHERE id = 'fall';
UPDATE animations SET id = 'soldier_dash'    WHERE id = 'dash';
UPDATE animations SET id = 'soldier_attack'  WHERE id = 'attack';
UPDATE animations SET id = 'soldier_attack2' WHERE id = 'attack2';
```

- [ ] **Step 2: Apply migrations by wiping and regenerating DB**

Run: `rm -rf data/ && go run ./cmd/game -help 2>/dev/null || true`

Note: the game normally doesn't accept flags — this just exercises DB open + migrations, then may fail on missing asset path. That's fine; we're only verifying migrations run.

Actual verification:

```bash
rm -rf data/
go run ./cmd/tune list 2>&1 | head -1
```

Expected: `OK` output or at minimum no migration error. Then:

```bash
sqlite3 data/game.db "SELECT id FROM animations ORDER BY id;"
```

Expected: 7 rows, all prefixed `soldier_`.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/007_rename_char1_to_soldier.sql
git commit -m "feat(storage): migrate animation IDs to soldier_ namespace"
```

---

### Task 2: Migration 008 (animations schema v2 — add columns)

**Files:**
- Create: `internal/storage/migrations/008_animations_schema_v2.sql`

- [ ] **Step 1: Write migration SQL**

```sql
-- 008_animations_schema_v2.sql
ALTER TABLE animations ADD COLUMN frame_w   INTEGER NOT NULL DEFAULT 120;
ALTER TABLE animations ADD COLUMN frame_h   INTEGER NOT NULL DEFAULT 80;
ALTER TABLE animations ADD COLUMN path      TEXT    NOT NULL DEFAULT '';
ALTER TABLE animations ADD COLUMN is_player INTEGER NOT NULL DEFAULT 0;
ALTER TABLE animations ADD COLUMN is_enemy  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE animations ADD COLUMN grid_cols INTEGER NOT NULL DEFAULT 0;
ALTER TABLE animations ADD COLUMN grid_rows INTEGER NOT NULL DEFAULT 0;
ALTER TABLE animations ADD COLUMN pick_row  INTEGER NOT NULL DEFAULT 0;

UPDATE animations
   SET path      = 'soldier/' || file,
       is_player = 1,
       frame_w   = 120,
       frame_h   = 80
 WHERE id LIKE 'soldier_%';
```

- [ ] **Step 2: Apply and verify**

```bash
rm -rf data/
go run ./cmd/tune list >/dev/null
sqlite3 data/game.db "SELECT id, path, frame_w, frame_h, is_player FROM animations;"
```

Expected: 7 soldier rows with path like `soldier/_Idle.png`, `frame_w=120`, `frame_h=80`, `is_player=1`.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/008_animations_schema_v2.sql
git commit -m "feat(storage): extend animations schema with frame dims, path, role flags, grid"
```

---

### Task 3: Migration 009 (seed orc animations)

**Files:**
- Create: `internal/storage/migrations/009_seed_orc_animations.sql`

- [ ] **Step 1: Write SQL**

```sql
-- 009_seed_orc_animations.sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path, is_player, is_enemy)
VALUES
    ('orc_idle',    'Idle.png',    6, 900, 1, 100, 100, 'orc/Idle.png',    0, 1),
    ('orc_run',     'Run.png',     8, 700, 1, 100, 100, 'orc/Run.png',     0, 1),
    ('orc_attack',  'Attack.png',  6, 600, 0, 100, 100, 'orc/Attack.png',  0, 1),
    ('orc_attack2', 'Attack2.png', 6, 700, 0, 100, 100, 'orc/Attack2.png', 0, 1),
    ('orc_hurt',    'Hurt.png',    4, 400, 0, 100, 100, 'orc/Hurt.png',    0, 1),
    ('orc_death',   'Death.png',   4, 500, 0, 100, 100, 'orc/Death.png',   0, 1);
```

- [ ] **Step 2: Apply and verify**

```bash
rm -rf data/
go run ./cmd/tune list >/dev/null
sqlite3 data/game.db "SELECT id, frame_count FROM animations WHERE is_enemy=1 ORDER BY id;"
```

Expected: 6 orc rows, correct frame counts (6, 4, 6, 6, 8, 4 in id-sort order).

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/009_seed_orc_animations.sql
git commit -m "feat(storage): seed orc animations"
```

---

### Task 4: Migration 010 (seed heart_beat animation, grid form)

**Files:**
- Create: `internal/storage/migrations/010_seed_heart_animation.sql`

- [ ] **Step 1: Write SQL**

```sql
-- 010_seed_heart_animation.sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path, is_player, is_enemy, grid_cols, grid_rows, pick_row)
VALUES
    ('heart_beat', 'HeartsBeat.png', 4, 400, 1, 16, 16, 'heart/HeartsBeat.png', 0, 0, 4, 6, 3);
```

- [ ] **Step 2: Apply and verify**

```bash
rm -rf data/
go run ./cmd/tune list >/dev/null
sqlite3 data/game.db "SELECT id, grid_cols, grid_rows, pick_row FROM animations WHERE id='heart_beat';"
```

Expected: `heart_beat|4|6|3`.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/010_seed_heart_animation.sql
git commit -m "feat(storage): seed heart_beat animation (grid sheet)"
```

---

### Task 5: Extend `AnimationSpec` struct + SpecMapper

**Files:**
- Modify: `internal/anim/animation.go`
- Modify: `internal/anim/spec.go`

- [ ] **Step 1: Extend struct**

In `internal/anim/animation.go`, replace the `AnimationSpec` struct:

```go
type AnimationSpec struct {
    ID         string
    File       string
    FrameCount int
    DurationMs int
    Loop       bool
    FrameW     int
    FrameH     int
    Path       string
    IsPlayer   bool
    IsEnemy    bool
    GridCols   int
    GridRows   int
    PickRow    int
}
```

- [ ] **Step 2: Extend mapper**

Replace body of `internal/anim/spec.go`:

```go
package anim

import "claude-pixel/internal/storage"

type SpecMapper struct{}

func (SpecMapper) Table() string { return "animations" }

func (SpecMapper) Columns() []string {
    return []string{
        "id", "file", "frame_count", "duration_ms", "loop",
        "frame_w", "frame_h", "path", "is_player", "is_enemy",
        "grid_cols", "grid_rows", "pick_row",
    }
}

func (SpecMapper) Scan(row storage.Scanner) (AnimationSpec, error) {
    var s AnimationSpec
    var loopInt, isPlayerInt, isEnemyInt int
    err := row.Scan(
        &s.ID, &s.File, &s.FrameCount, &s.DurationMs, &loopInt,
        &s.FrameW, &s.FrameH, &s.Path, &isPlayerInt, &isEnemyInt,
        &s.GridCols, &s.GridRows, &s.PickRow,
    )
    s.Loop = loopInt != 0
    s.IsPlayer = isPlayerInt != 0
    s.IsEnemy = isEnemyInt != 0
    return s, err
}

func (SpecMapper) Values(s AnimationSpec) []any {
    b := func(v bool) int {
        if v {
            return 1
        }
        return 0
    }
    return []any{
        s.ID, s.File, s.FrameCount, s.DurationMs, b(s.Loop),
        s.FrameW, s.FrameH, s.Path, b(s.IsPlayer), b(s.IsEnemy),
        s.GridCols, s.GridRows, s.PickRow,
    }
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

Expected: no errors. (Library loader will break until Task 7 — proceed.)

- [ ] **Step 4: Commit**

```bash
git add internal/anim/animation.go internal/anim/spec.go
git commit -m "feat(anim): extend AnimationSpec with frame dims, path, flags, grid fields"
```

---

### Task 6: Add `SliceGrid` helper

**Files:**
- Modify: `internal/anim/sheet.go`
- Create: `internal/anim/sheet_test.go`

- [ ] **Step 1: Write failing test**

`internal/anim/sheet_test.go`:

```go
package anim

import (
    "testing"

    "github.com/hajimehoshi/ebiten/v2"
)

func TestSliceGridPicksCorrectRow(t *testing.T) {
    // 4 cols x 6 rows of 16x16 => 64x96
    img := ebiten.NewImage(64, 96)
    frames := SliceGrid(img, 16, 16, 4, 6, 3, 4)
    if len(frames) != 4 {
        t.Fatalf("want 4 frames, got %d", len(frames))
    }
    for i, f := range frames {
        b := f.Bounds()
        if b.Dx() != 16 || b.Dy() != 16 {
            t.Errorf("frame %d: want 16x16, got %dx%d", i, b.Dx(), b.Dy())
        }
        // pick_row=3, col=i -> origin (i*16, 48)
        if b.Min.X != i*16 || b.Min.Y != 48 {
            t.Errorf("frame %d: want origin (%d,48), got (%d,%d)", i, i*16, b.Min.X, b.Min.Y)
        }
    }
}
```

- [ ] **Step 2: Run test, verify FAIL**

Run: `go test ./internal/anim -run TestSliceGrid -v`

Expected: FAIL — `undefined: SliceGrid`.

- [ ] **Step 3: Implement SliceGrid**

Append to `internal/anim/sheet.go`:

```go
// SliceGrid slices a 2D grid sheet (cols x rows of frameW x frameH), picking
// `count` consecutive frames from row `pickRow` (0-indexed).
func SliceGrid(img *ebiten.Image, frameW, frameH, cols, rows, pickRow, count int) []*ebiten.Image {
    _ = rows
    frames := make([]*ebiten.Image, count)
    for i := 0; i < count; i++ {
        col := i % cols
        x0 := col * frameW
        y0 := pickRow * frameH
        r := image.Rect(x0, y0, x0+frameW, y0+frameH)
        frames[i] = img.SubImage(r).(*ebiten.Image)
    }
    return frames
}
```

- [ ] **Step 4: Run test, verify PASS**

Run: `go test ./internal/anim -run TestSliceGrid -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anim/sheet.go internal/anim/sheet_test.go
git commit -m "feat(anim): add SliceGrid for 2D grid sheets"
```

---

### Task 7: Update `anim.LoadLibrary` to use per-spec dims + path + grid branch

**Files:**
- Modify: `internal/anim/library.go`

- [ ] **Step 1: Rewrite library loader**

Replace file contents:

```go
package anim

import (
    "context"
    "fmt"
    "path/filepath"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/ebitenutil"

    "claude-pixel/internal/config"
    "claude-pixel/internal/storage"
)

func LoadLibrary(cfg *config.Config, repo *storage.Repository[AnimationSpec]) (map[string]*Animation, error) {
    specs, err := repo.List(context.Background())
    if err != nil {
        return nil, fmt.Errorf("list specs: %w", err)
    }
    out := make(map[string]*Animation, len(specs))
    for i := range specs {
        spec := specs[i]
        path := filepath.Join(cfg.AssetsDir, spec.Path)
        img, _, err := ebitenutil.NewImageFromFile(path)
        if err != nil {
            return nil, fmt.Errorf("load %s: %w", path, err)
        }

        w, h := img.Bounds().Dx(), img.Bounds().Dy()
        var frames []*ebiten.Image

        if spec.GridCols > 0 {
            wantW := spec.FrameW * spec.GridCols
            wantH := spec.FrameH * spec.GridRows
            if w != wantW || h != wantH {
                return nil, fmt.Errorf("sheet %s (grid): got %dx%d, want %dx%d", spec.Path, w, h, wantW, wantH)
            }
            frames = SliceGrid(img, spec.FrameW, spec.FrameH, spec.GridCols, spec.GridRows, spec.PickRow, spec.FrameCount)
        } else {
            wantW := spec.FrameW * spec.FrameCount
            if w != wantW || h != spec.FrameH {
                return nil, fmt.Errorf("sheet %s (strip): got %dx%d, want %dx%d", spec.Path, w, h, wantW, spec.FrameH)
            }
            frames = Slice(img, spec.FrameW, spec.FrameH, spec.FrameCount)
        }
        out[spec.ID] = NewAnimation(&spec, frames)
    }
    return out, nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

Expected: `internal/config` compile break (still references `SpriteFrameW`/`H` removal pending). That's handled in Task 8. Temporarily the build may still succeed if `cfg.SpriteFrameW` is unused here — confirm no reference remains in the new library.go.

Search for lingering refs:

```bash
grep -rn "SpriteFrameW\|SpriteFrameH" internal/ cmd/
```

Expected: only in `internal/config/config.go` (to be removed in Task 8).

- [ ] **Step 3: Commit**

```bash
git add internal/anim/library.go
git commit -m "feat(anim): load per-spec frame dims/path; branch strip vs grid"
```

---

### Task 8: Update `.env` + `config.Config` (drop SPRITE_FRAME_W/H; change ASSETS_DIR)

**Files:**
- Modify: `.env`, `.env.example`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go` (if it references the removed fields)

- [ ] **Step 1: Update `.env.example`**

Replace contents:

```
DB_PATH=./data/game.db
ASSETS_DIR=./assets/sprites
WINDOW_WIDTH=1280
WINDOW_HEIGHT=720
RENDER_SCALE=3
DEBUG_CONFIG_PATH=./config/debug.json
FONT_PATH=./assets/fonts/monogram/ttf/monogram.ttf
```

- [ ] **Step 2: Update `.env` to match**

Same contents as `.env.example`.

- [ ] **Step 3: Update `config.Config` struct + Load()**

`internal/config/config.go`:

```go
package config

import (
    "fmt"
    "os"
    "strconv"

    "github.com/joho/godotenv"
)

type Config struct {
    DBPath          string
    AssetsDir       string
    WindowW         int
    WindowH         int
    RenderScale     int
    DebugConfigPath string
    FontPath        string
}

func Load() *Config {
    _ = godotenv.Load()
    return &Config{
        DBPath:          mustString("DB_PATH"),
        AssetsDir:       mustString("ASSETS_DIR"),
        WindowW:         mustInt("WINDOW_WIDTH"),
        WindowH:         mustInt("WINDOW_HEIGHT"),
        RenderScale:     mustInt("RENDER_SCALE"),
        DebugConfigPath: mustString("DEBUG_CONFIG_PATH"),
        FontPath:        mustString("FONT_PATH"),
    }
}

func mustString(key string) string {
    v := os.Getenv(key)
    if v == "" {
        panic(fmt.Sprintf("config: required env %q is empty or missing", key))
    }
    return v
}

func mustInt(key string) int {
    s := mustString(key)
    n, err := strconv.Atoi(s)
    if err != nil {
        panic(fmt.Sprintf("config: env %q = %q is not an integer: %v", key, s, err))
    }
    return n
}
```

- [ ] **Step 4: Fix `config_test.go` if it references removed fields**

```bash
grep -n "SpriteFrame" internal/config/config_test.go || echo "no refs"
```

If present, delete/rewrite those assertions. Add assertion for `FontPath`:

```go
if cfg.FontPath != "/fonts/m.ttf" {
    t.Errorf("FontPath: want %q got %q", "/fonts/m.ttf", cfg.FontPath)
}
```

Update `TestLoadReadsEnvVars` setup to set `FONT_PATH=/fonts/m.ttf` and remove SPRITE_FRAME_W/H setup.

Update `TestLoadPanicsOnMissingKey` loop to include `"FONT_PATH"` and drop `"SPRITE_FRAME_W"`, `"SPRITE_FRAME_H"`.

- [ ] **Step 5: Run config tests**

Run: `go test ./internal/config -v`

Expected: PASS.

- [ ] **Step 6: Full build**

Run: `go build ./...`

Expected: compile succeeds. (Other pkgs don't reference the removed fields.)

- [ ] **Step 7: Manual smoke — game still launches soldier**

Run: `rm -rf data/ && go run ./cmd/game`

Expected: window opens, soldier sprites load from new `./assets/sprites/soldier/_*.png` path, no orc yet. Close window.

- [ ] **Step 8: Commit**

```bash
git add .env .env.example internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): ASSETS_DIR is base dir; drop SPRITE_FRAME_W/H; add FONT_PATH"
```

---

## Phase 2 — Combat primitives

### Task 9: Migration 011 (create hitboxes table)

**Files:**
- Create: `internal/storage/migrations/011_hitboxes_schema.sql`

- [ ] **Step 1: Write SQL**

```sql
-- 011_hitboxes_schema.sql
CREATE TABLE hitboxes (
    id                 TEXT    PRIMARY KEY,
    owner              TEXT    NOT NULL,
    kind               TEXT    NOT NULL,
    offset_x           INTEGER NOT NULL,
    offset_y           INTEGER NOT NULL,
    width              INTEGER NOT NULL,
    height             INTEGER NOT NULL,
    active_frame_start INTEGER NOT NULL DEFAULT -1,
    active_frame_end   INTEGER NOT NULL DEFAULT -1
);
```

- [ ] **Step 2: Verify**

```bash
rm -rf data/
go run ./cmd/tune list >/dev/null
sqlite3 data/game.db ".schema hitboxes"
```

Expected: table definition printed.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/011_hitboxes_schema.sql
git commit -m "feat(storage): add hitboxes table"
```

---

### Task 10: Migration 012 (seed hitboxes)

**Files:**
- Create: `internal/storage/migrations/012_seed_hitboxes.sql`

- [ ] **Step 1: Write SQL**

```sql
-- 012_seed_hitboxes.sql
INSERT OR IGNORE INTO hitboxes
    (id, owner, kind, offset_x, offset_y, width, height, active_frame_start, active_frame_end)
VALUES
    ('soldier_body',    'soldier', 'body',    -20, -70, 40, 70, -1, -1),
    ('soldier_attack',  'soldier', 'attack',   20, -60, 60, 50,  1,  2),
    ('soldier_attack2', 'soldier', 'attack2',  20, -60, 80, 60,  2,  4),
    ('orc_body',        'orc',     'body',    -25, -80, 50, 80, -1, -1),
    ('orc_attack',      'orc',     'attack',   25, -70, 60, 60,  2,  3),
    ('orc_attack2',     'orc',     'attack2',  25, -70, 70, 60,  3,  4);
```

- [ ] **Step 2: Verify**

```bash
rm -rf data/
go run ./cmd/tune list >/dev/null
sqlite3 data/game.db "SELECT owner, kind FROM hitboxes ORDER BY owner, kind;"
```

Expected: 6 rows.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/012_seed_hitboxes.sql
git commit -m "feat(storage): seed initial hitboxes for soldier + orc"
```

---

### Task 11: Migration 013 (seed combat tuning)

**Files:**
- Create: `internal/storage/migrations/013_seed_combat_tuning.sql`

- [ ] **Step 1: Write SQL**

```sql
-- 013_seed_combat_tuning.sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('orc_hurt_bounce_vx',    120,    0, 500, 'px/s', 'horizontal bounce away from attacker when orc is hurt'),
    ('orc_hurt_bounce_vy',   -180, -500,   0, 'px/s', 'vertical pop applied on orc hurt'),
    ('soldier_knockback_vx',  200,    0, 500, 'px/s', 'horizontal knockback away when soldier is hit'),
    ('soldier_knockback_vy', -300, -600,   0, 'px/s', 'upward pop when soldier is hit (airborne i-frame)'),
    ('soldier_max_lives',      10,    1,  99, '',     'starting soldier lives'),
    ('orc_max_lives',           2,    1,  10, '',     'starting orc lives'),
    ('orc_spawn_min_s',         3,    1,  60, 's',    'minimum orc spawn interval'),
    ('orc_spawn_max_s',        10,    1,  60, 's',    'maximum orc spawn interval'),
    ('orc_max_alive',           3,    1,  10, '',     'max concurrent orcs'),
    ('orc_intent_tick_s',       2,  0.5,  10, 's',    'orc intent reroll period'),
    ('orc_run_speed',          80,    0, 500, 'px/s', 'orc ground speed');
```

- [ ] **Step 2: Verify via tune CLI**

```bash
rm -rf data/
go run ./cmd/tune list
```

Expected: 11 new keys listed alongside existing 6 physics keys.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/013_seed_combat_tuning.sql
git commit -m "feat(storage): seed combat + spawner tuning keys"
```

---

### Task 12: `internal/combat` — Box type + mapper

**Files:**
- Create: `internal/combat/box.go`
- Create: `internal/combat/box_mapper.go`

- [ ] **Step 1: Write `box.go`**

```go
package combat

type Box struct {
    OffsetX, OffsetY int
    W, H             int
    FrameStart       int
    FrameEnd         int
}

// Active reports whether this box is live on the given 0-indexed frame.
// FrameStart = -1 means the box is always active (used for body boxes).
func (b Box) Active(frame int) bool {
    if b.FrameStart < 0 {
        return true
    }
    return frame >= b.FrameStart && frame <= b.FrameEnd
}
```

- [ ] **Step 2: Write `box_mapper.go`**

```go
package combat

import "claude-pixel/internal/storage"

type HitboxSpec struct {
    ID         string
    Owner      string
    Kind       string
    OffsetX    int
    OffsetY    int
    Width      int
    Height     int
    FrameStart int
    FrameEnd   int
}

func (h HitboxSpec) GetID() string { return h.ID }

func (h HitboxSpec) ToBox() Box {
    return Box{
        OffsetX:    h.OffsetX,
        OffsetY:    h.OffsetY,
        W:          h.Width,
        H:          h.Height,
        FrameStart: h.FrameStart,
        FrameEnd:   h.FrameEnd,
    }
}

type HitboxMapper struct{}

func (HitboxMapper) Table() string { return "hitboxes" }

func (HitboxMapper) Columns() []string {
    return []string{"id", "owner", "kind", "offset_x", "offset_y", "width", "height", "active_frame_start", "active_frame_end"}
}

func (HitboxMapper) Scan(row storage.Scanner) (HitboxSpec, error) {
    var s HitboxSpec
    err := row.Scan(&s.ID, &s.Owner, &s.Kind, &s.OffsetX, &s.OffsetY, &s.Width, &s.Height, &s.FrameStart, &s.FrameEnd)
    return s, err
}

func (HitboxMapper) Values(s HitboxSpec) []any {
    return []any{s.ID, s.Owner, s.Kind, s.OffsetX, s.OffsetY, s.Width, s.Height, s.FrameStart, s.FrameEnd}
}
```

- [ ] **Step 3: Build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/combat/box.go internal/combat/box_mapper.go
git commit -m "feat(combat): Box + HitboxSpec storage mapper"
```

---

### Task 13: `combat.Fighter` interface + `HitEvent`

**Files:**
- Create: `internal/combat/fighter.go`

- [ ] **Step 1: Write the file**

```go
package combat

type Fighter interface {
    Pos() (x, y float64)
    FacingDir() int
    CurrentAnimID() string
    CurrentFrame() int
    Body() Box
    ActiveHits() []Box
    IsInvulnerable() bool
    Alive() bool

    // HitSet accessors for dedup inside one attack swing.
    AlreadyHit(target Fighter) bool
    MarkHit(target Fighter)
}

type HitEvent struct {
    Attacker   Fighter
    Victim     Fighter
    AttackKind string // "attack" | "attack2"
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/combat/fighter.go
git commit -m "feat(combat): Fighter interface + HitEvent"
```

---

### Task 14: `combat.Resolve` with AABB overlap test (TDD)

**Files:**
- Create: `internal/combat/resolve.go`
- Create: `internal/combat/resolve_test.go`

- [ ] **Step 1: Write failing test**

```go
package combat

import (
    "testing"
)

type fakeFighter struct {
    x, y       float64
    facing     int
    anim       string
    frame      int
    body       Box
    hits       []Box
    invul      bool
    alive      bool
    hitSet     map[Fighter]bool
}

func (f *fakeFighter) Pos() (float64, float64) { return f.x, f.y }
func (f *fakeFighter) FacingDir() int          { return f.facing }
func (f *fakeFighter) CurrentAnimID() string   { return f.anim }
func (f *fakeFighter) CurrentFrame() int       { return f.frame }
func (f *fakeFighter) Body() Box               { return f.body }
func (f *fakeFighter) ActiveHits() []Box       { return f.hits }
func (f *fakeFighter) IsInvulnerable() bool    { return f.invul }
func (f *fakeFighter) Alive() bool             { return f.alive }
func (f *fakeFighter) AlreadyHit(t Fighter) bool {
    if f.hitSet == nil { return false }
    return f.hitSet[t]
}
func (f *fakeFighter) MarkHit(t Fighter) {
    if f.hitSet == nil { f.hitSet = map[Fighter]bool{} }
    f.hitSet[t] = true
}

func newFake() *fakeFighter {
    return &fakeFighter{facing: 1, alive: true}
}

func TestResolveEmitsEventOnOverlap(t *testing.T) {
    att := newFake()
    att.x, att.y = 100, 100
    att.anim = "soldier_attack"
    att.hits = []Box{{OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2}}
    att.frame = 2

    vic := newFake()
    vic.x, vic.y = 140, 100
    vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}

    events := Resolve([]Fighter{att}, []Fighter{vic})
    if len(events) != 1 {
        t.Fatalf("want 1 event, got %d", len(events))
    }
    if events[0].AttackKind != "attack" {
        t.Errorf("want AttackKind=attack, got %q", events[0].AttackKind)
    }
}

func TestResolveSkipsInvulnerableVictim(t *testing.T) {
    att := newFake()
    att.x, att.y = 100, 100
    att.anim = "soldier_attack"
    att.hits = []Box{{OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2}}
    att.frame = 2

    vic := newFake()
    vic.x, vic.y = 140, 100
    vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}
    vic.invul = true

    events := Resolve([]Fighter{att}, []Fighter{vic})
    if len(events) != 0 {
        t.Fatalf("want 0 events, got %d", len(events))
    }
}

func TestResolveSkipsFrameOutsideWindow(t *testing.T) {
    att := newFake()
    att.x, att.y = 100, 100
    att.anim = "soldier_attack"
    att.hits = []Box{} // caller filters by ActiveHits(); empty = no hit
    att.frame = 0

    vic := newFake()
    vic.x, vic.y = 140, 100
    vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}

    events := Resolve([]Fighter{att}, []Fighter{vic})
    if len(events) != 0 {
        t.Fatalf("want 0 events, got %d", len(events))
    }
}

func TestResolveDedupsWithinAttackWindow(t *testing.T) {
    att := newFake()
    att.x, att.y = 100, 100
    att.anim = "soldier_attack"
    att.hits = []Box{{OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2}}
    att.frame = 2

    vic := newFake()
    vic.x, vic.y = 140, 100
    vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}

    // First resolve: 1 event. Mark hit.
    e1 := Resolve([]Fighter{att}, []Fighter{vic})
    if len(e1) != 1 { t.Fatalf("first resolve: want 1, got %d", len(e1)) }

    // Second resolve same frame: 0 events (already hit).
    e2 := Resolve([]Fighter{att}, []Fighter{vic})
    if len(e2) != 0 { t.Fatalf("second resolve: want 0, got %d", len(e2)) }
}

func TestResolveFlipsBoxForFacingMinus1(t *testing.T) {
    att := newFake()
    att.x, att.y = 100, 100
    att.facing = -1
    att.anim = "soldier_attack"
    att.hits = []Box{{OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2}}
    att.frame = 2

    // Facing=-1 mirrors box to the LEFT of anchor.
    // World rect: (x - OffsetX - W, y + OffsetY, W, H) = (100 - 20 - 60, 40, 60, 50) = (20..80, 40..90).
    // Victim to the LEFT should be hit:
    vic := newFake()
    vic.x, vic.y = 60, 100
    vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}
    // Victim world: (60-25, 20, 50, 80) = (35..85, 20..100). Overlaps attacker box.

    events := Resolve([]Fighter{att}, []Fighter{vic})
    if len(events) != 1 {
        t.Fatalf("facing=-1 should hit left victim; got %d events", len(events))
    }
}
```

- [ ] **Step 2: Run, verify FAIL**

Run: `go test ./internal/combat -run TestResolve -v`

Expected: FAIL — `undefined: Resolve`.

- [ ] **Step 3: Implement `resolve.go`**

```go
package combat

import "strings"

// Resolve checks each attacker's active hits against each victim's body.
// Emits one HitEvent per (attacker, victim) first-time overlap.
func Resolve(attackers, victims []Fighter) []HitEvent {
    var out []HitEvent
    for _, a := range attackers {
        if !a.Alive() || a.IsInvulnerable() {
            continue
        }
        hits := a.ActiveHits()
        if len(hits) == 0 {
            continue
        }
        kind := attackKindFromAnim(a.CurrentAnimID())
        if kind == "" {
            continue
        }

        ax, ay := a.Pos()
        for _, v := range victims {
            if v == a || !v.Alive() || v.IsInvulnerable() {
                continue
            }
            if a.AlreadyHit(v) {
                continue
            }
            vx, vy := v.Pos()
            vb := worldRect(vx, vy, v.FacingDir(), v.Body())
            for _, h := range hits {
                ab := worldRect(ax, ay, a.FacingDir(), h)
                if overlap(ab, vb) {
                    out = append(out, HitEvent{Attacker: a, Victim: v, AttackKind: kind})
                    a.MarkHit(v)
                    break
                }
            }
        }
    }
    return out
}

type rect struct {
    MinX, MinY, MaxX, MaxY float64
}

func worldRect(anchorX, anchorY float64, facing int, b Box) rect {
    var minX float64
    if facing >= 0 {
        minX = anchorX + float64(b.OffsetX)
    } else {
        minX = anchorX - float64(b.OffsetX) - float64(b.W)
    }
    minY := anchorY + float64(b.OffsetY)
    return rect{
        MinX: minX,
        MinY: minY,
        MaxX: minX + float64(b.W),
        MaxY: minY + float64(b.H),
    }
}

func overlap(a, b rect) bool {
    if a.MaxX <= b.MinX || b.MaxX <= a.MinX {
        return false
    }
    if a.MaxY <= b.MinY || b.MaxY <= a.MinY {
        return false
    }
    return true
}

// attackKindFromAnim maps anim IDs like "soldier_attack", "orc_attack2" -> "attack"/"attack2".
func attackKindFromAnim(id string) string {
    switch {
    case strings.HasSuffix(id, "_attack2"):
        return "attack2"
    case strings.HasSuffix(id, "_attack"):
        return "attack"
    }
    return ""
}
```

- [ ] **Step 4: Run, verify PASS**

Run: `go test ./internal/combat -v`

Expected: PASS on all 5 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/combat/resolve.go internal/combat/resolve_test.go
git commit -m "feat(combat): Resolve emits HitEvents with facing-flip, dedup, i-frame skip"
```

---

### Task 15: `world.Clamp` helper

**Files:**
- Modify: `internal/world/world.go`
- Create: `internal/world/world_test.go`

- [ ] **Step 1: Write test**

```go
package world

import "testing"

func TestClamp(t *testing.T) {
    cases := []struct {
        in, min, max, want float64
    }{
        {50, 100, 200, 100},
        {150, 100, 200, 150},
        {250, 100, 200, 200},
        {100, 100, 200, 100},
        {200, 100, 200, 200},
    }
    for _, c := range cases {
        got := Clamp(c.in, c.min, c.max)
        if got != c.want {
            t.Errorf("Clamp(%v, %v, %v) = %v, want %v", c.in, c.min, c.max, got, c.want)
        }
    }
}
```

- [ ] **Step 2: Verify FAIL**

Run: `go test ./internal/world -v`

Expected: FAIL — `undefined: Clamp`.

- [ ] **Step 3: Implement**

Append to `internal/world/world.go`:

```go
func Clamp(x, min, max float64) float64 {
    if x < min {
        return min
    }
    if x > max {
        return max
    }
    return x
}
```

- [ ] **Step 4: Verify PASS**

Run: `go test ./internal/world -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/world/world.go internal/world/world_test.go
git commit -m "feat(world): add Clamp(x, min, max)"
```

---

## Phase 3 — Enemy package

### Task 16: `internal/enemy/enemy.go` — struct + constructor skeleton

**Files:**
- Create: `internal/enemy/enemy.go`

- [ ] **Step 1: Write file**

```go
package enemy

import (
    "math/rand"
    "time"

    "claude-pixel/internal/anim"
    "claude-pixel/internal/combat"
    "claude-pixel/internal/player"
    "claude-pixel/internal/world"
)

type Config struct {
    StartX, StartY float64
    Physics        *player.Physics
    Tuning         *Tuning
    Anims          map[string]*anim.Animation
    Boxes          map[string]combat.Box // keys: "body", "attack", "attack2"
    RNG            *rand.Rand
}

type Enemy struct {
    X, Y, VX, VY float64
    Facing       int
    Grounded     bool
    Lives        int
    RunSpeed     float64
    Physics      *player.Physics
    Tuning       *Tuning
    Anims        map[string]*anim.Animation
    Boxes        map[string]combat.Box
    FSM          *FSM
    Current      *anim.Animation
    CurrentAnim  string
    IntentTimer  float64
    HitSet       map[combat.Fighter]bool
    Dead         bool
    rng          *rand.Rand
}

func New(cfg Config) *Enemy {
    e := &Enemy{
        X:        cfg.StartX,
        Y:        cfg.StartY,
        Facing:   1,
        Lives:    int(cfg.Tuning.MaxLives),
        RunSpeed: cfg.Tuning.RunSpeed,
        Physics:  cfg.Physics,
        Tuning:   cfg.Tuning,
        Anims:    cfg.Anims,
        Boxes:    cfg.Boxes,
        HitSet:   map[combat.Fighter]bool{},
        rng:      cfg.RNG,
    }
    e.FSM = NewFSM(StateFall)
    e.FSM.Register(&fallState{})
    e.FSM.Register(&runState{})
    e.FSM.Register(&attackState{})
    e.FSM.Register(&attack2State{})
    e.FSM.Register(&hurtState{})
    e.FSM.Register(&deathState{})
    e.FSM.Start(e)
    return e
}

func (e *Enemy) PlayAnim(id string) {
    a, ok := e.Anims[id]
    if !ok {
        return
    }
    a.Reset()
    e.Current = a
    e.CurrentAnim = id
}

func (e *Enemy) ApplyPhysics(w *world.World, dt time.Duration) {
    dtS := dt.Seconds()
    e.VY += e.Physics.Gravity * dtS
    if e.VY > e.Physics.MaxFallSpeed {
        e.VY = e.Physics.MaxFallSpeed
    }
    e.X += e.VX * dtS
    e.Y += e.VY * dtS
    if e.Y >= w.GroundY {
        e.Y = w.GroundY
        e.VY = 0
        e.Grounded = true
    } else {
        e.Grounded = false
    }
}
```

- [ ] **Step 2: Build — expect unresolved symbols**

Run: `go build ./internal/enemy`

Expected: FAIL — `undefined: FSM`, `StateFall`, `fallState`, etc. These are filled in Tasks 17–18.

Leave partially failing; we'll complete incrementally. Do not commit yet.

---

### Task 17: `internal/enemy/fsm.go` + `internal/enemy/tuning.go`

**Files:**
- Create: `internal/enemy/fsm.go`
- Create: `internal/enemy/tuning.go`

- [ ] **Step 1: Write `fsm.go`**

```go
package enemy

import "time"

type StateID string

const (
    StateFall    StateID = "fall"
    StateRun     StateID = "run"
    StateAttack  StateID = "attack"
    StateAttack2 StateID = "attack2"
    StateHurt    StateID = "hurt"
    StateDeath   StateID = "death"
)

type State interface {
    ID() StateID
    Enter(e *Enemy)
    Update(e *Enemy, dt time.Duration) StateID
    Exit(e *Enemy)
}

type FSM struct {
    states    map[StateID]State
    initialID StateID
    current   State
}

func NewFSM(initial StateID) *FSM {
    return &FSM{states: map[StateID]State{}, initialID: initial}
}

func (f *FSM) Register(s State) { f.states[s.ID()] = s }

func (f *FSM) Start(e *Enemy) {
    f.current = f.states[f.initialID]
    if f.current != nil {
        f.current.Enter(e)
    }
}

func (f *FSM) CurrentID() StateID {
    if f.current == nil {
        return ""
    }
    return f.current.ID()
}

func (f *FSM) Handle(e *Enemy, dt time.Duration) {
    if f.current == nil {
        return
    }
    next := f.current.Update(e, dt)
    if next != f.current.ID() {
        f.current.Exit(e)
        ns, ok := f.states[next]
        if !ok {
            return
        }
        f.current = ns
        f.current.Enter(e)
    }
}

// Transition forces a state change (used by OnHit).
func (f *FSM) Transition(e *Enemy, to StateID) {
    if f.current != nil && f.current.ID() == to {
        return
    }
    if f.current != nil {
        f.current.Exit(e)
    }
    ns, ok := f.states[to]
    if !ok {
        return
    }
    f.current = ns
    f.current.Enter(e)
}
```

- [ ] **Step 2: Write `tuning.go`**

```go
package enemy

import (
    "context"
    "fmt"

    "claude-pixel/internal/player"
    "claude-pixel/internal/storage"
)

type Tuning struct {
    MaxLives       float64
    RunSpeed       float64
    IntentTickS    float64
    HurtBounceVX   float64
    HurtBounceVY   float64
    SpawnMinS      float64
    SpawnMaxS      float64
    MaxAlive       float64
}

func LoadTuning(repo *storage.Repository[player.TuningParam]) (*Tuning, error) {
    params, err := repo.List(context.Background())
    if err != nil {
        return nil, err
    }
    m := make(map[string]float64, len(params))
    for _, p := range params {
        m[p.Key] = p.Value
    }
    pick := func(k string) (float64, error) {
        v, ok := m[k]
        if !ok {
            return 0, fmt.Errorf("missing tuning key %q", k)
        }
        return v, nil
    }
    t := &Tuning{}
    keys := []struct {
        k string
        p *float64
    }{
        {"orc_max_lives", &t.MaxLives},
        {"orc_run_speed", &t.RunSpeed},
        {"orc_intent_tick_s", &t.IntentTickS},
        {"orc_hurt_bounce_vx", &t.HurtBounceVX},
        {"orc_hurt_bounce_vy", &t.HurtBounceVY},
        {"orc_spawn_min_s", &t.SpawnMinS},
        {"orc_spawn_max_s", &t.SpawnMaxS},
        {"orc_max_alive", &t.MaxAlive},
    }
    for _, k := range keys {
        v, err := pick(k.k)
        if err != nil {
            return nil, err
        }
        *k.p = v
    }
    return t, nil
}
```

- [ ] **Step 3: Build — expect states still missing**

Run: `go build ./internal/enemy`

Expected: FAIL — undefined state types. Fixed in Task 18.

---

### Task 18: Enemy states (`states.go`) + OnHit

**Files:**
- Create: `internal/enemy/states.go`

- [ ] **Step 1: Write file**

```go
package enemy

import (
    "time"

    "claude-pixel/internal/combat"
)

// fallState: after spawn, show idle frame while falling.
type fallState struct{}

func (fallState) ID() StateID { return StateFall }
func (fallState) Enter(e *Enemy) {
    e.PlayAnim("orc_idle")
    e.VX = 0
}
func (fallState) Exit(e *Enemy) {}
func (fallState) Update(e *Enemy, dt time.Duration) StateID {
    // Freeze anim on frame 0 by NOT updating; fall state is visually static.
    if e.Grounded {
        // Randomize initial facing on land.
        if e.rng.Intn(2) == 0 {
            e.Facing = 1
        } else {
            e.Facing = -1
        }
        return StateRun
    }
    return StateFall
}

// runState: move L/R, flip on boundary, tick intent.
type runState struct{}

func (runState) ID() StateID { return StateRun }
func (runState) Enter(e *Enemy) {
    e.PlayAnim("orc_run")
    e.IntentTimer = e.Tuning.IntentTickS
}
func (runState) Exit(e *Enemy) {}
func (runState) Update(e *Enemy, dt time.Duration) StateID {
    e.VX = float64(e.Facing) * e.RunSpeed

    e.IntentTimer -= dt.Seconds()
    if e.IntentTimer <= 0 {
        e.IntentTimer = e.Tuning.IntentTickS
        if e.rng.Float64() < 0.5 {
            // 50% pick attack kind
            if e.rng.Float64() < 0.5 {
                return StateAttack
            }
            return StateAttack2
        }
        // else: stay run
    }
    return StateRun
}

type attackState struct{}

func (attackState) ID() StateID { return StateAttack }
func (attackState) Enter(e *Enemy) {
    e.PlayAnim("orc_attack")
    e.VX = 0
    // clear dedup at attack entry
    e.HitSet = map[combat.Fighter]bool{}
}
func (attackState) Exit(e *Enemy) {}
func (attackState) Update(e *Enemy, dt time.Duration) StateID {
    if e.Current != nil && e.Current.Done() {
        return StateRun
    }
    return StateAttack
}

type attack2State struct{}

func (attack2State) ID() StateID { return StateAttack2 }
func (attack2State) Enter(e *Enemy) {
    e.PlayAnim("orc_attack2")
    e.VX = 0
    e.HitSet = map[combat.Fighter]bool{}
}
func (attack2State) Exit(e *Enemy) {}
func (attack2State) Update(e *Enemy, dt time.Duration) StateID {
    if e.Current != nil && e.Current.Done() {
        return StateRun
    }
    return StateAttack2
}

type hurtState struct{}

func (hurtState) ID() StateID { return StateHurt }
func (hurtState) Enter(e *Enemy) {
    e.PlayAnim("orc_hurt")
    // bounce away: direction set by OnHit via e.Facing sign convention;
    // here we assume OnHit already set VX sign via attacker delta.
    // See Enemy.OnHit below.
}
func (hurtState) Exit(e *Enemy) {}
func (hurtState) Update(e *Enemy, dt time.Duration) StateID {
    if e.Current != nil && e.Current.Done() && e.Grounded {
        // new random facing on exit
        if e.rng.Intn(2) == 0 {
            e.Facing = 1
        } else {
            e.Facing = -1
        }
        return StateRun
    }
    return StateHurt
}

type deathState struct{}

func (deathState) ID() StateID { return StateDeath }
func (deathState) Enter(e *Enemy) {
    e.PlayAnim("orc_death")
    e.VX = 0
}
func (deathState) Exit(e *Enemy) {}
func (deathState) Update(e *Enemy, dt time.Duration) StateID {
    if e.Current != nil && e.Current.Done() {
        e.Dead = true
    }
    return StateDeath
}
```

- [ ] **Step 2: Add `OnHit` to enemy**

Append to `internal/enemy/enemy.go`:

```go
func (e *Enemy) OnHit(attackerX float64) {
    e.Lives--
    if e.Lives <= 0 {
        e.FSM.Transition(e, StateDeath)
        return
    }
    // bounce direction: away from attacker
    dir := 1.0
    if attackerX > e.X {
        dir = -1.0
    }
    e.VX = dir * e.Tuning.HurtBounceVX
    e.VY = e.Tuning.HurtBounceVY
    e.Grounded = false
    e.FSM.Transition(e, StateHurt)
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/enemy`

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/enemy/enemy.go internal/enemy/fsm.go internal/enemy/states.go internal/enemy/tuning.go
git commit -m "feat(enemy): orc FSM with 6 states, physics, OnHit bounce"
```

---

### Task 19: Enemy — `combat.Fighter` impl

**Files:**
- Create: `internal/enemy/fighter.go`

> **Context:** `combat.Fighter` requires `FacingDir() int` (NOT `Facing()`) — this avoids collision with the `Facing` struct field on `Player` and `Enemy`. The interface method was named `FacingDir` in Task 13 intentionally. The resolver in Task 14 already calls `.FacingDir()`.

- [ ] **Step 1: Write file**

```go
package enemy

import "claude-pixel/internal/combat"

func (e *Enemy) Pos() (float64, float64) { return e.X, e.Y }

func (e *Enemy) FacingDir() int { return e.Facing }

func (e *Enemy) CurrentAnimID() string { return e.CurrentAnim }

func (e *Enemy) CurrentFrame() int {
    if e.Current == nil {
        return 0
    }
    return e.Current.FrameIndex()
}

func (e *Enemy) Body() combat.Box { return e.Boxes["body"] }

func (e *Enemy) ActiveHits() []combat.Box {
    switch e.CurrentAnim {
    case "orc_attack":
        box := e.Boxes["attack"]
        if box.Active(e.CurrentFrame()) {
            return []combat.Box{box}
        }
    case "orc_attack2":
        box := e.Boxes["attack2"]
        if box.Active(e.CurrentFrame()) {
            return []combat.Box{box}
        }
    }
    return nil
}

func (e *Enemy) IsInvulnerable() bool {
    id := e.FSM.CurrentID()
    return id == StateHurt || id == StateDeath
}

func (e *Enemy) Alive() bool {
    return !e.Dead && e.FSM.CurrentID() != StateDeath
}

func (e *Enemy) AlreadyHit(t combat.Fighter) bool {
    if e.HitSet == nil {
        return false
    }
    return e.HitSet[t]
}

func (e *Enemy) MarkHit(t combat.Fighter) {
    if e.HitSet == nil {
        e.HitSet = map[combat.Fighter]bool{}
    }
    e.HitSet[t] = true
}
```

- [ ] **Step 2: Build + combat tests still green**

Run: `go test ./internal/combat -v && go build ./...`

Expected: combat tests PASS, build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/enemy/fighter.go
git commit -m "feat(enemy): implement combat.Fighter"
```

---

### Task 20: Load enemy anims + hitboxes helpers

**Files:**
- Create: `internal/enemy/loader.go`

- [ ] **Step 1: Write file**

```go
package enemy

import (
    "context"
    "fmt"

    "claude-pixel/internal/anim"
    "claude-pixel/internal/combat"
    "claude-pixel/internal/storage"
)

// OrcAnims extracts the 6 orc anims from a loaded library.
func OrcAnims(lib map[string]*anim.Animation) (map[string]*anim.Animation, error) {
    want := []string{"orc_idle", "orc_run", "orc_attack", "orc_attack2", "orc_hurt", "orc_death"}
    out := make(map[string]*anim.Animation, len(want))
    for _, k := range want {
        a, ok := lib[k]
        if !ok {
            return nil, fmt.Errorf("orc anims: missing %q", k)
        }
        out[k] = a
    }
    return out, nil
}

// OrcBoxes loads orc hitboxes keyed by kind ("body","attack","attack2").
func OrcBoxes(repo *storage.Repository[combat.HitboxSpec]) (map[string]combat.Box, error) {
    all, err := repo.List(context.Background())
    if err != nil {
        return nil, err
    }
    out := make(map[string]combat.Box, 3)
    for _, s := range all {
        if s.Owner != "orc" {
            continue
        }
        out[s.Kind] = s.ToBox()
    }
    if _, ok := out["body"]; !ok {
        return nil, fmt.Errorf("orc hitboxes: missing body")
    }
    return out, nil
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/enemy/loader.go
git commit -m "feat(enemy): OrcAnims + OrcBoxes loader helpers"
```

---

### Task 21: Enemy FSM test — fall→run on ground

**Files:**
- Create: `internal/enemy/fsm_test.go`

- [ ] **Step 1: Write failing test**

```go
package enemy

import (
    "math/rand"
    "testing"
    "time"

    "claude-pixel/internal/anim"
    "claude-pixel/internal/combat"
    "claude-pixel/internal/player"
)

func newTestEnemy() *Enemy {
    // Stub anims: use zero-frame specs with Done()=false immediately.
    stub := func(id string, frames, durMs int, loop bool) *anim.Animation {
        spec := &anim.AnimationSpec{ID: id, FrameCount: frames, DurationMs: durMs, Loop: loop}
        return anim.NewAnimation(spec, nil)
    }
    anims := map[string]*anim.Animation{
        "orc_idle":    stub("orc_idle", 6, 900, true),
        "orc_run":     stub("orc_run", 8, 700, true),
        "orc_attack":  stub("orc_attack", 6, 600, false),
        "orc_attack2": stub("orc_attack2", 6, 700, false),
        "orc_hurt":    stub("orc_hurt", 4, 400, false),
        "orc_death":   stub("orc_death", 4, 500, false),
    }
    boxes := map[string]combat.Box{
        "body":    {OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1},
        "attack":  {OffsetX: 25, OffsetY: -70, W: 60, H: 60, FrameStart: 2, FrameEnd: 3},
        "attack2": {OffsetX: 25, OffsetY: -70, W: 70, H: 60, FrameStart: 3, FrameEnd: 4},
    }
    ph := &player.Physics{Gravity: 1800, MaxFallSpeed: 900}
    tn := &Tuning{MaxLives: 2, RunSpeed: 80, IntentTickS: 2, HurtBounceVX: 120, HurtBounceVY: -180}
    return New(Config{
        StartX: 400, StartY: -100,
        Physics: ph, Tuning: tn,
        Anims: anims, Boxes: boxes,
        RNG: rand.New(rand.NewSource(1)),
    })
}

func TestEnemyStartsInFall(t *testing.T) {
    e := newTestEnemy()
    if e.FSM.CurrentID() != StateFall {
        t.Errorf("want fall, got %q", e.FSM.CurrentID())
    }
}

func TestEnemyFallToRunOnGrounded(t *testing.T) {
    e := newTestEnemy()
    e.Grounded = true
    e.FSM.Handle(e, 16*time.Millisecond)
    if e.FSM.CurrentID() != StateRun {
        t.Errorf("want run, got %q", e.FSM.CurrentID())
    }
}

func TestEnemyHurtOnDamage(t *testing.T) {
    e := newTestEnemy()
    e.Grounded = true
    e.FSM.Handle(e, 16*time.Millisecond) // -> run
    e.OnHit(e.X + 10)                    // attacker to the right
    if e.FSM.CurrentID() != StateHurt {
        t.Errorf("want hurt, got %q", e.FSM.CurrentID())
    }
    if e.VX >= 0 {
        t.Errorf("expected leftward bounce, got VX=%v", e.VX)
    }
}

func TestEnemyDiesOnFatalHit(t *testing.T) {
    e := newTestEnemy()
    e.Grounded = true
    e.FSM.Handle(e, 16*time.Millisecond)
    e.Lives = 1
    e.OnHit(e.X + 10)
    if e.FSM.CurrentID() != StateDeath {
        t.Errorf("want death, got %q", e.FSM.CurrentID())
    }
}
```

- [ ] **Step 2: Run — some should pass (construction), Update-driven ones rely on stub frame counts**

Run: `go test ./internal/enemy -v`

Expected: All 4 PASS. (Animations with nil frames are safe as long as `CurrentFrame()` isn't rendered — we're only testing FSM transitions.)

If any fail, fix stubs or state logic until green.

- [ ] **Step 3: Commit**

```bash
git add internal/enemy/fsm_test.go
git commit -m "test(enemy): FSM transitions (fall->run, hurt, death)"
```

---

## Phase 4 — Spawner

### Task 22: `internal/spawner/spawner.go`

**Files:**
- Create: `internal/spawner/spawner.go`

- [ ] **Step 1: Write file**

```go
package spawner

import (
    "math/rand"
    "time"

    "claude-pixel/internal/enemy"
)

type Spawner struct {
    MinIntervalS float64
    MaxIntervalS float64
    MaxAlive     int
    nextSpawn    float64 // seconds until next roll
    spawnXMin    float64
    spawnXMax    float64
    spawnY       float64
    rng          *rand.Rand
    newEnemy     func(x, y float64) *enemy.Enemy
}

type Config struct {
    MinIntervalS float64
    MaxIntervalS float64
    MaxAlive     int
    SpawnXMin    float64
    SpawnXMax    float64
    SpawnY       float64
    RNG          *rand.Rand
    NewEnemy     func(x, y float64) *enemy.Enemy
}

func New(cfg Config) *Spawner {
    s := &Spawner{
        MinIntervalS: cfg.MinIntervalS,
        MaxIntervalS: cfg.MaxIntervalS,
        MaxAlive:     cfg.MaxAlive,
        spawnXMin:    cfg.SpawnXMin,
        spawnXMax:    cfg.SpawnXMax,
        spawnY:       cfg.SpawnY,
        rng:          cfg.RNG,
        newEnemy:     cfg.NewEnemy,
    }
    s.nextSpawn = s.rollInterval()
    return s
}

func (s *Spawner) NextSpawnS() float64 { return s.nextSpawn }

func (s *Spawner) Reset() {
    s.nextSpawn = s.rollInterval()
}

func (s *Spawner) Tick(dt time.Duration, alive int) *enemy.Enemy {
    s.nextSpawn -= dt.Seconds()
    if s.nextSpawn > 0 {
        return nil
    }
    s.nextSpawn = s.rollInterval()
    if alive >= s.MaxAlive {
        return nil
    }
    return s.newEnemy(s.rollSpawnX(), s.spawnY)
}

func (s *Spawner) rollInterval() float64 {
    return s.MinIntervalS + s.rng.Float64()*(s.MaxIntervalS-s.MinIntervalS)
}

func (s *Spawner) rollSpawnX() float64 {
    return s.spawnXMin + s.rng.Float64()*(s.spawnXMax-s.spawnXMin)
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/spawner/spawner.go
git commit -m "feat(spawner): timer + interval roll + cap enforcement"
```

---

### Task 23: Spawner tests

**Files:**
- Create: `internal/spawner/spawner_test.go`

- [ ] **Step 1: Write tests**

```go
package spawner

import (
    "math/rand"
    "testing"
    "time"

    "claude-pixel/internal/enemy"
)

func fakeFactory(calls *int) func(x, y float64) *enemy.Enemy {
    return func(x, y float64) *enemy.Enemy {
        *calls++
        return &enemy.Enemy{X: x, Y: y}
    }
}

func TestSpawnerRespectsInterval(t *testing.T) {
    calls := 0
    s := New(Config{
        MinIntervalS: 2, MaxIntervalS: 2, MaxAlive: 5,
        SpawnXMin: 100, SpawnXMax: 200, SpawnY: -300,
        RNG:      rand.New(rand.NewSource(1)),
        NewEnemy: fakeFactory(&calls),
    })
    if got := s.Tick(time.Second, 0); got != nil {
        t.Errorf("at t=1s, should not spawn yet")
    }
    if got := s.Tick(time.Second, 0); got == nil {
        t.Errorf("at t=2s, should spawn")
    }
    if calls != 1 {
        t.Errorf("want 1 factory call, got %d", calls)
    }
}

func TestSpawnerSkipsWhenAtCap(t *testing.T) {
    calls := 0
    s := New(Config{
        MinIntervalS: 1, MaxIntervalS: 1, MaxAlive: 2,
        SpawnXMin: 100, SpawnXMax: 200, SpawnY: -300,
        RNG:      rand.New(rand.NewSource(1)),
        NewEnemy: fakeFactory(&calls),
    })
    if got := s.Tick(time.Second, 2); got != nil {
        t.Errorf("at cap=2 alive=2, should skip")
    }
    if calls != 0 {
        t.Errorf("want 0 factory calls, got %d", calls)
    }
    // timer re-rolled; next tick after 1s should also roll but be skipped
    if got := s.Tick(time.Second, 2); got != nil {
        t.Errorf("still at cap, should skip")
    }
    // drop below cap
    if got := s.Tick(time.Second, 1); got == nil {
        t.Errorf("under cap, should spawn")
    }
    if calls != 1 {
        t.Errorf("want 1 factory call, got %d", calls)
    }
}

func TestSpawnerIntervalWithinRange(t *testing.T) {
    s := New(Config{
        MinIntervalS: 3, MaxIntervalS: 10, MaxAlive: 5,
        SpawnXMin: 100, SpawnXMax: 200, SpawnY: -300,
        RNG:      rand.New(rand.NewSource(42)),
        NewEnemy: func(x, y float64) *enemy.Enemy { return &enemy.Enemy{} },
    })
    for i := 0; i < 50; i++ {
        iv := s.rollInterval()
        if iv < 3 || iv > 10 {
            t.Fatalf("interval out of range: %v", iv)
        }
    }
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/spawner -v`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/spawner/spawner_test.go
git commit -m "test(spawner): interval range, cap enforcement"
```

---

## Phase 5 — Player extensions

### Task 24: Extend `Player` struct + state IDs

**Files:**
- Modify: `internal/player/player.go`
- Modify: `internal/player/fsm.go`

- [ ] **Step 1: Extend `Player` struct**

In `internal/player/player.go`, replace the `Player` struct and `Config`:

```go
package player

import (
    "time"

    "claude-pixel/internal/anim"
    "claude-pixel/internal/combat"
    "claude-pixel/internal/world"
)

type Player struct {
    X, Y     float64
    VX, VY   float64
    Facing   int
    Grounded bool
    FSM      *FSM
    Physics  *Physics
    Anims    map[string]*anim.Animation
    Current  *anim.Animation

    // combat extensions
    Lives       int
    HitFlag     bool
    Boxes       map[string]combat.Box
    HitSet      map[combat.Fighter]bool
    CurrentAnim string
}

type Config struct {
    StartX, StartY float64
    Physics        *Physics
    Anims          map[string]*anim.Animation
    Boxes          map[string]combat.Box
    StartLives     int
}

func (p *Player) PlayAnim(id string) {
    a, ok := p.Anims[id]
    if !ok {
        return
    }
    a.Reset()
    p.Current = a
    p.CurrentAnim = id
}

func (p *Player) ApplyPhysics(w *world.World, dt time.Duration) {
    dtS := dt.Seconds()
    p.VY += p.Physics.Gravity * dtS
    if p.VY > p.Physics.MaxFallSpeed {
        p.VY = p.Physics.MaxFallSpeed
    }
    p.X += p.VX * dtS
    p.Y += p.VY * dtS
    if p.Y >= w.GroundY {
        p.Y = w.GroundY
        p.VY = 0
        p.Grounded = true
    } else {
        p.Grounded = false
    }
}

func New(cfg Config) *Player {
    p := &Player{
        X:       cfg.StartX,
        Y:       cfg.StartY,
        Facing:  1,
        Physics: cfg.Physics,
        Anims:   cfg.Anims,
        Boxes:   cfg.Boxes,
        Lives:   cfg.StartLives,
        HitSet:  map[combat.Fighter]bool{},
    }
    p.FSM = NewFSM(StateIdle)
    p.FSM.Register(&idleState{})
    p.FSM.Register(&runState{})
    p.FSM.Register(&jumpState{})
    p.FSM.Register(&fallState{})
    p.FSM.Register(&attackState{})
    p.FSM.Register(&attack2State{})
    p.FSM.Register(&hitState{})
    p.FSM.Register(&deathState{})
    p.FSM.Start(p)
    return p
}
```

- [ ] **Step 2: Add StateID constants**

In `internal/player/fsm.go`, append to the const block:

```go
const (
    // ... existing
    StateHit   StateID = "hit"
    StateDeath StateID = "death"
)
```

Also add a `Transition` method to FSM (same as enemy version):

```go
func (f *FSM) Transition(p *Player, to StateID) {
    if f.current != nil && f.current.ID() == to {
        return
    }
    if f.current != nil {
        f.current.Exit(p)
    }
    ns, ok := f.states[to]
    if !ok {
        return
    }
    f.current = ns
    f.current.Enter(p)
}
```

- [ ] **Step 3: Build — expect state registration refs to fail**

Run: `go build ./internal/player`

Expected: FAIL — `undefined: hitState`, `deathState`. Fix in Task 25.

---

### Task 25: Player `hitState` + `deathState` + attack HitSet clearing + animation IDs

**Files:**
- Modify: `internal/player/states.go`

- [ ] **Step 1: Update existing state `Enter` hooks to set `CurrentAnim` via `PlayAnim`**

`PlayAnim` already sets `CurrentAnim`. Existing states call `p.PlayAnim("idle")` etc. **Those anim IDs no longer exist** (renamed to `soldier_idle` etc. in Task 1). Fix all `PlayAnim` calls:

Replace in `internal/player/states.go`:
- `p.PlayAnim("idle")` → `p.PlayAnim("soldier_idle")`
- `p.PlayAnim("run")` → `p.PlayAnim("soldier_run")`
- `p.PlayAnim("jump")` → `p.PlayAnim("soldier_jump")`
- `p.PlayAnim("fall")` → `p.PlayAnim("soldier_fall")`
- `p.PlayAnim("attack")` → `p.PlayAnim("soldier_attack")`
- `p.PlayAnim("attack2")` → `p.PlayAnim("soldier_attack2")`

- [ ] **Step 2: Clear HitSet on attack/attack2 Enter**

Update `attackState.Enter` and `attack2State.Enter`:

```go
func (attackState) Enter(p *Player) {
    p.PlayAnim("soldier_attack")
    if p.Grounded {
        p.VX = 0
    }
    p.HitSet = map[combat.Fighter]bool{}
}
```

Same pattern for `attack2State.Enter`.

Add import at top of `states.go`:

```go
import (
    "time"

    "claude-pixel/internal/combat"
    "claude-pixel/internal/input"
)
```

- [ ] **Step 3: Add `hitState` + `deathState` at bottom of `states.go`**

```go
type hitState struct{}

func (hitState) ID() StateID { return StateHit }
func (hitState) Enter(p *Player) {
    p.PlayAnim("soldier_hit")
    p.HitFlag = true
    // VX set by OnHit before Transition; keep it. VY also.
}
func (hitState) Exit(p *Player) {
    p.HitFlag = false
}
func (hitState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
    // Ignore all intents.
    _ = in
    if p.Grounded {
        return StateIdle
    }
    return StateHit
}

type deathState struct{}

func (deathState) ID() StateID { return StateDeath }
func (deathState) Enter(p *Player) {
    p.PlayAnim("soldier_death")
    p.VX = 0
}
func (deathState) Exit(p *Player) {}
func (deathState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
    _ = in
    return StateDeath // terminal; game observes Current.Done() for GameOver transition
}
```

- [ ] **Step 4: Add `Player.OnHit` method**

Append to `internal/player/player.go` (already imports `combat`):

```go
func (p *Player) OnHit(knockbackVX, knockbackVY, attackerX float64) {
    p.Lives--
    if p.Lives <= 0 {
        p.FSM.Transition(p, StateDeath)
        return
    }
    dir := 1.0
    if attackerX > p.X {
        dir = -1.0
    }
    p.VX = dir * knockbackVX
    p.VY = knockbackVY
    p.Grounded = false
    p.FSM.Transition(p, StateHit)
}
```

- [ ] **Step 5: Build**

Run: `go build ./internal/player`

Expected: success.

- [ ] **Step 6: Commit**

```bash
git add internal/player/player.go internal/player/fsm.go internal/player/states.go
git commit -m "feat(player): Hit + Death states, OnHit with knockback, updated anim IDs"
```

---

### Task 26: Player `combat.Fighter` impl

**Files:**
- Create: `internal/player/fighter.go`

- [ ] **Step 1: Write file**

```go
package player

import "claude-pixel/internal/combat"

func (p *Player) Pos() (float64, float64) { return p.X, p.Y }

func (p *Player) FacingDir() int { return p.Facing }

func (p *Player) CurrentAnimID() string { return p.CurrentAnim }

func (p *Player) CurrentFrame() int {
    if p.Current == nil {
        return 0
    }
    return p.Current.FrameIndex()
}

func (p *Player) Body() combat.Box { return p.Boxes["body"] }

func (p *Player) ActiveHits() []combat.Box {
    switch p.CurrentAnim {
    case "soldier_attack":
        b := p.Boxes["attack"]
        if b.Active(p.CurrentFrame()) {
            return []combat.Box{b}
        }
    case "soldier_attack2":
        b := p.Boxes["attack2"]
        if b.Active(p.CurrentFrame()) {
            return []combat.Box{b}
        }
    }
    return nil
}

func (p *Player) IsInvulnerable() bool {
    return p.HitFlag || p.FSM.CurrentID() == StateDeath
}

func (p *Player) Alive() bool {
    return p.Lives > 0 && p.FSM.CurrentID() != StateDeath
}

func (p *Player) AlreadyHit(t combat.Fighter) bool {
    if p.HitSet == nil {
        return false
    }
    return p.HitSet[t]
}

func (p *Player) MarkHit(t combat.Fighter) {
    if p.HitSet == nil {
        p.HitSet = map[combat.Fighter]bool{}
    }
    p.HitSet[t] = true
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: success. `debug` pkg still imports `player` — no API break there (all existing methods still present).

- [ ] **Step 3: Commit**

```bash
git add internal/player/fighter.go
git commit -m "feat(player): implement combat.Fighter"
```

---

### Task 27: Update existing player FSM test for renamed anim IDs; add hit/death tests

**Files:**
- Modify: `internal/player/fsm_test.go`

- [ ] **Step 1: Find current refs**

```bash
grep -n '"idle"\|"run"\|"jump"\|"fall"\|"attack"\|"attack2"' internal/player/fsm_test.go
```

Replace anim-ID literals in test fixtures with `soldier_`-prefixed versions.

- [ ] **Step 2: Add hit/death tests**

Append:

```go
func TestPlayerOnHitEntersHit(t *testing.T) {
    p := newTestPlayer(t)
    p.Lives = 3
    p.OnHit(200, -300, p.X+10)
    if p.FSM.CurrentID() != StateHit {
        t.Errorf("want hit, got %q", p.FSM.CurrentID())
    }
    if p.Lives != 2 {
        t.Errorf("want 2 lives, got %d", p.Lives)
    }
    if p.Grounded {
        t.Errorf("expected airborne after knockback")
    }
    if p.VX >= 0 {
        t.Errorf("expected leftward bounce, got VX=%v", p.VX)
    }
}

func TestPlayerHitToIdleOnGround(t *testing.T) {
    p := newTestPlayer(t)
    p.Lives = 3
    p.OnHit(200, -300, p.X+10)
    p.Grounded = true
    p.FSM.Handle(p, input.Intent{}, 16*time.Millisecond)
    if p.FSM.CurrentID() != StateIdle {
        t.Errorf("want idle after land, got %q", p.FSM.CurrentID())
    }
    if p.HitFlag {
        t.Errorf("HitFlag should be cleared on Exit")
    }
}

func TestPlayerDeathOnZeroLives(t *testing.T) {
    p := newTestPlayer(t)
    p.Lives = 1
    p.OnHit(200, -300, p.X+10)
    if p.FSM.CurrentID() != StateDeath {
        t.Errorf("want death, got %q", p.FSM.CurrentID())
    }
}
```

`newTestPlayer` may need updating to accept `Boxes` and `StartLives` in its `Config`. Adjust existing helper to match the new `Config`:

```go
func newTestPlayer(t *testing.T) *Player {
    anims := map[string]*anim.Animation{
        "soldier_idle":    stubAnim("soldier_idle", 10, 1000, true),
        "soldier_run":     stubAnim("soldier_run", 10, 1000, true),
        "soldier_jump":    stubAnim("soldier_jump", 3, 500, false),
        "soldier_fall":    stubAnim("soldier_fall", 3, 500, false),
        "soldier_attack":  stubAnim("soldier_attack", 4, 500, false),
        "soldier_attack2": stubAnim("soldier_attack2", 6, 750, false),
        "soldier_hit":     stubAnim("soldier_hit", 1, 200, false),
        "soldier_death":   stubAnim("soldier_death", 10, 1000, false),
    }
    boxes := map[string]combat.Box{
        "body":    {OffsetX: -20, OffsetY: -70, W: 40, H: 70, FrameStart: -1, FrameEnd: -1},
        "attack":  {OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2},
        "attack2": {OffsetX: 20, OffsetY: -60, W: 80, H: 60, FrameStart: 2, FrameEnd: 4},
    }
    return New(Config{
        StartX:     400,
        StartY:     600,
        Physics:    &Physics{RunSpeed: 220, SprintSpeed: 330, AirControl: 0.8, JumpVelocity: -600, Gravity: 1800, MaxFallSpeed: 900},
        Anims:      anims,
        Boxes:      boxes,
        StartLives: 10,
    })
}

func stubAnim(id string, frames, durMs int, loop bool) *anim.Animation {
    spec := &anim.AnimationSpec{ID: id, FrameCount: frames, DurationMs: durMs, Loop: loop}
    return anim.NewAnimation(spec, nil)
}
```

(Existing helper likely named slightly differently — unify, keeping existing test bodies working after renames.)

Update imports at top of `fsm_test.go` to include `combat`.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/player -v`

Expected: PASS on all tests (existing + 3 new).

- [ ] **Step 4: Commit**

```bash
git add internal/player/fsm_test.go
git commit -m "test(player): hit/death transitions; update anim IDs"
```

---

## Phase 6 — HUD + font

### Task 28: Font loader

**Files:**
- Create: `internal/hud/font.go`

- [ ] **Step 1: Write file**

```go
package hud

import (
    "bytes"
    "fmt"
    "os"

    "github.com/hajimehoshi/ebiten/v2/text/v2"
)

var source *text.GoTextFaceSource

// LoadFont reads the TTF file at path and stores the shared face source.
// Call once at boot before creating any faces.
func LoadFont(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("read font %s: %w", path, err)
    }
    src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
    if err != nil {
        return fmt.Errorf("parse font %s: %w", path, err)
    }
    source = src
    return nil
}

// NewFace returns a monogram face at the given pixel size.
// Panics if LoadFont was not called.
func NewFace(size float64) *text.GoTextFace {
    if source == nil {
        panic("hud: font not loaded; call LoadFont() first")
    }
    return &text.GoTextFace{Source: source, Size: size}
}
```

- [ ] **Step 2: Build**

Run: `go build ./internal/hud`

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/hud/font.go
git commit -m "feat(hud): monogram font loader via text/v2"
```

---

### Task 29: HUD struct + Draw

**Files:**
- Create: `internal/hud/hud.go`

- [ ] **Step 1: Write file**

```go
package hud

import (
    "fmt"
    "image/color"
    "time"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/text/v2"

    "claude-pixel/internal/anim"
)

type LivesProvider interface {
    Lives() int
}

type HUD struct {
    Heart    *anim.Animation
    Face     *text.GoTextFace
    Provider LivesProvider
    Scale    float64 // heart render scale
    WindowW  int
}

func NewHUD(heart *anim.Animation, face *text.GoTextFace, provider LivesProvider, windowW int, scale float64) *HUD {
    return &HUD{Heart: heart, Face: face, Provider: provider, Scale: scale, WindowW: windowW}
}

func (h *HUD) Update(dt time.Duration) { h.Heart.Update(dt) }

func (h *HUD) Draw(screen *ebiten.Image) {
    lives := h.Provider.Lives()
    if lives < 0 {
        lives = 0
    }
    label := fmt.Sprintf("x%d", lives)

    // measure text
    textW, textH := text.Measure(label, h.Face, 0)

    const padding = 16
    const gap = 8
    heartSize := 16.0 * h.Scale

    // right-aligned
    textX := float64(h.WindowW) - padding - textW
    heartX := textX - gap - heartSize

    // vertical: top padding; center heart with text baseline roughly
    topY := float64(padding)

    // draw heart (animated)
    if frame := h.Heart.CurrentFrame(); frame != nil {
        op := &ebiten.DrawImageOptions{}
        op.GeoM.Scale(h.Scale, h.Scale)
        op.GeoM.Translate(heartX, topY)
        op.Filter = ebiten.FilterNearest
        screen.DrawImage(frame, op)
    }

    // draw text (right-aligned at textX)
    textOp := &text.DrawOptions{}
    textOp.GeoM.Translate(textX, topY)
    textOp.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
    text.Draw(screen, label, h.Face, textOp)

    _ = textH // reserved for baseline alignment refinement
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/hud/hud.go
git commit -m "feat(hud): heart + lives text rendering, right-aligned top"
```

---

### Task 30: GameOver overlay

**Files:**
- Create: `internal/hud/gameover.go`

- [ ] **Step 1: Write file**

```go
package hud

import (
    "image/color"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
    "github.com/hajimehoshi/ebiten/v2/vector"
)

type GameOver struct {
    Title    *text.GoTextFace
    Subtitle *text.GoTextFace
    WindowW  int
    WindowH  int
}

func NewGameOver(title, subtitle *text.GoTextFace, w, h int) *GameOver {
    return &GameOver{Title: title, Subtitle: subtitle, WindowW: w, WindowH: h}
}

func (g *GameOver) Draw(screen *ebiten.Image) {
    // dim background
    vector.DrawFilledRect(screen, 0, 0, float32(g.WindowW), float32(g.WindowH),
        color.RGBA{0, 0, 0, 160}, false)

    drawCentered := func(s string, face *text.GoTextFace, yFrac float64) {
        w, _ := text.Measure(s, face, 0)
        op := &text.DrawOptions{}
        op.GeoM.Translate(float64(g.WindowW)/2-w/2, float64(g.WindowH)*yFrac)
        op.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
        text.Draw(screen, s, face, op)
    }

    drawCentered("GAME OVER", g.Title, 0.40)
    drawCentered("Press R to restart", g.Subtitle, 0.55)
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/hud/gameover.go
git commit -m "feat(hud): GameOver overlay (dim + title + subtitle)"
```

---

### Task 31: HUD format test

**Files:**
- Create: `internal/hud/hud_test.go`

- [ ] **Step 1: Write minimal test (formatter-only; rendering needs ebiten)**

Since HUD's main logic is the text format + heart frame pick, extract a tiny helper and test it:

Add to `internal/hud/hud.go` above `(h *HUD) Draw`:

```go
func formatLives(lives int) string {
    if lives < 0 {
        lives = 0
    }
    return fmt.Sprintf("x%d", lives)
}
```

Then in `Draw`, use `label := formatLives(h.Provider.Lives())`.

`internal/hud/hud_test.go`:

```go
package hud

import "testing"

func TestFormatLives(t *testing.T) {
    cases := []struct {
        in   int
        want string
    }{
        {10, "x10"},
        {1, "x1"},
        {0, "x0"},
        {-3, "x0"},
    }
    for _, c := range cases {
        if got := formatLives(c.in); got != c.want {
            t.Errorf("formatLives(%d) = %q, want %q", c.in, got, c.want)
        }
    }
}
```

- [ ] **Step 2: Run test**

Run: `go test ./internal/hud -v`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/hud/hud.go internal/hud/hud_test.go
git commit -m "test(hud): formatLives"
```

---

## Phase 7 — Game wiring

### Task 32: Update `debug.FieldSource` + fields catalog

**Files:**
- Modify: `internal/debug/fields.go`

- [ ] **Step 1: Extend `FieldSource` interface**

```go
type FieldSource interface {
    Player() *player.Player
    Intent() *input.Intent
    EngineFPS() float64
    EngineTPS() float64
    OrcCount() int
    NextSpawnS() float64
}
```

- [ ] **Step 2: Add catalog entries**

Append to `Catalog`:

```go
    "orc_count":          {"orc_count", func(s FieldSource) string { return fmt.Sprintf("Orcs: %d", s.OrcCount()) }},
    "orc_next_spawn_s":   {"orc_next_spawn_s", func(s FieldSource) string { return fmt.Sprintf("NextSpawn: %.2fs", s.NextSpawnS()) }},
    "player_lives":       {"player_lives", func(s FieldSource) string { return fmt.Sprintf("Lives: %d", s.Player().Lives) }},
    "player_invulnerable": {"player_invulnerable", func(s FieldSource) string { return fmt.Sprintf("Invul: %t", s.Player().HitFlag) }},
```

- [ ] **Step 3: Build**

Run: `go build ./...`

Expected: FAIL in `internal/game` since `Game` no longer satisfies `FieldSource` without the new methods. Addressed in Task 34.

- [ ] **Step 4: Commit**

```bash
git add internal/debug/fields.go
git commit -m "feat(debug): add orc/lives/invul fields to overlay catalog"
```

---

### Task 33: Hitbox repo loader + combat tuning key plumbing

**Files:**
- Create: `internal/combat/tuning.go`

- [ ] **Step 1: Write file**

```go
package combat

import (
    "context"
    "fmt"

    "claude-pixel/internal/player"
    "claude-pixel/internal/storage"
)

type Tuning struct {
    SoldierKnockbackVX float64
    SoldierKnockbackVY float64
    SoldierMaxLives    int
}

func LoadTuning(repo *storage.Repository[player.TuningParam]) (*Tuning, error) {
    params, err := repo.List(context.Background())
    if err != nil {
        return nil, err
    }
    m := make(map[string]float64, len(params))
    for _, p := range params {
        m[p.Key] = p.Value
    }
    get := func(k string) (float64, error) {
        v, ok := m[k]
        if !ok {
            return 0, fmt.Errorf("missing tuning key %q", k)
        }
        return v, nil
    }
    t := &Tuning{}
    var e error
    if t.SoldierKnockbackVX, e = get("soldier_knockback_vx"); e != nil {
        return nil, e
    }
    if t.SoldierKnockbackVY, e = get("soldier_knockback_vy"); e != nil {
        return nil, e
    }
    var maxL float64
    if maxL, e = get("soldier_max_lives"); e != nil {
        return nil, e
    }
    t.SoldierMaxLives = int(maxL)
    return t, nil
}

// SoldierBoxes loads soldier hitboxes keyed by kind.
func SoldierBoxes(repo *storage.Repository[HitboxSpec]) (map[string]Box, error) {
    all, err := repo.List(context.Background())
    if err != nil {
        return nil, err
    }
    out := make(map[string]Box, 3)
    for _, s := range all {
        if s.Owner != "soldier" {
            continue
        }
        out[s.Kind] = s.ToBox()
    }
    if _, ok := out["body"]; !ok {
        return nil, fmt.Errorf("soldier hitboxes: missing body")
    }
    return out, nil
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: game pkg still broken (Task 34). `combat` pkg compiles.

- [ ] **Step 3: Commit**

```bash
git add internal/combat/tuning.go
git commit -m "feat(combat): load soldier knockback/lives tuning + SoldierBoxes"
```

---

### Task 34: Game wiring — enemies, spawner, HUD, state, combat, F4, R-restart

**Files:**
- Modify: `internal/game/game.go`

- [ ] **Step 1: Rewrite game.go**

```go
package game

import (
    "image/color"
    "math/rand"
    "time"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
    "github.com/hajimehoshi/ebiten/v2/text/v2"
    "github.com/hajimehoshi/ebiten/v2/vector"

    "claude-pixel/internal/anim"
    "claude-pixel/internal/combat"
    "claude-pixel/internal/config"
    "claude-pixel/internal/debug"
    "claude-pixel/internal/enemy"
    "claude-pixel/internal/hud"
    "claude-pixel/internal/input"
    "claude-pixel/internal/player"
    "claude-pixel/internal/spawner"
    "claude-pixel/internal/world"
)

type GameState int

const (
    Playing GameState = iota
    GameOverState
)

type Deps struct {
    Cfg          *config.Config
    Anims        map[string]*anim.Animation
    Physics      *player.Physics
    DebugCfg     *debug.Config
    SoldierBoxes map[string]combat.Box
    CombatTuning *combat.Tuning
    OrcAnims     map[string]*anim.Animation
    OrcBoxes     map[string]combat.Box
    OrcTuning    *enemy.Tuning
    HeartAnim    *anim.Animation
    HUDFace      *text.GoTextFace
    OverTitle    *text.GoTextFace
    OverSubtitle *text.GoTextFace
}

type Game struct {
    cfg          *config.Config
    world        *world.World
    player       *player.Player
    enemies      []*enemy.Enemy
    spawner      *spawner.Spawner
    overlay      *debug.Overlay
    hud          *hud.HUD
    gameOver     *hud.GameOver
    state        GameState
    hitboxDebug  bool
    lastIntent   input.Intent
    combatTuning *combat.Tuning
    orcAnims     map[string]*anim.Animation
    orcBoxes     map[string]combat.Box
    orcTuning    *enemy.Tuning
    physics      *player.Physics
    rng          *rand.Rand
}

func New(d Deps) *Game {
    rng := rand.New(rand.NewSource(time.Now().UnixNano()))
    w := world.New(d.Cfg, d.Physics.Gravity)

    p := player.New(player.Config{
        StartX:     float64(d.Cfg.WindowW) / 2,
        StartY:     w.GroundY,
        Physics:    d.Physics,
        Anims:      d.Anims,
        Boxes:      d.SoldierBoxes,
        StartLives: d.CombatTuning.SoldierMaxLives,
    })
    p.Grounded = true

    g := &Game{
        cfg:          d.Cfg,
        world:        w,
        player:       p,
        overlay:      debug.NewOverlay(d.DebugCfg, nil), // fill below
        combatTuning: d.CombatTuning,
        orcAnims:     d.OrcAnims,
        orcBoxes:     d.OrcBoxes,
        orcTuning:    d.OrcTuning,
        physics:      d.Physics,
        rng:          rng,
        state:        Playing,
    }
    g.overlay = debug.NewOverlay(d.DebugCfg, g)

    orcHalfW := float64(100) * float64(d.Cfg.RenderScale) / 2
    spawnXMin := orcHalfW
    spawnXMax := float64(d.Cfg.WindowW) - orcHalfW
    orcSpriteH := float64(100 * d.Cfg.RenderScale)

    g.spawner = spawner.New(spawner.Config{
        MinIntervalS: d.OrcTuning.SpawnMinS,
        MaxIntervalS: d.OrcTuning.SpawnMaxS,
        MaxAlive:     int(d.OrcTuning.MaxAlive),
        SpawnXMin:    spawnXMin,
        SpawnXMax:    spawnXMax,
        SpawnY:       -orcSpriteH,
        RNG:          rng,
        NewEnemy: func(x, y float64) *enemy.Enemy {
            return enemy.New(enemy.Config{
                StartX: x, StartY: y,
                Physics: d.Physics,
                Tuning:  d.OrcTuning,
                Anims:   d.OrcAnims,
                Boxes:   d.OrcBoxes,
                RNG:     rng,
            })
        },
    })

    g.hud = hud.NewHUD(d.HeartAnim, d.HUDFace, livesProvider{p}, d.Cfg.WindowW, 3)
    g.gameOver = hud.NewGameOver(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)

    return g
}

type livesProvider struct{ p *player.Player }

func (l livesProvider) Lives() int { return l.p.Lives }

// FieldSource impls
func (g *Game) Player() *player.Player { return g.player }
func (g *Game) Intent() *input.Intent  { return &g.lastIntent }
func (g *Game) EngineFPS() float64     { return ebiten.ActualFPS() }
func (g *Game) EngineTPS() float64     { return ebiten.ActualTPS() }
func (g *Game) OrcCount() int          { return len(g.enemies) }
func (g *Game) NextSpawnS() float64    { return g.spawner.NextSpawnS() }

func (g *Game) Layout(outerW, outerH int) (int, int) { return g.cfg.WindowW, g.cfg.WindowH }

func (g *Game) Update() error {
    if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
        g.overlay.Toggle()
    }
    if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
        g.hitboxDebug = !g.hitboxDebug
    }

    if g.state == GameOverState {
        if inpututil.IsKeyJustPressed(ebiten.KeyR) {
            g.reset()
        }
        return nil
    }

    g.lastIntent = input.Poll()
    dt := time.Second / 60

    // Soldier FSM (intents ignored in hit/death by their states).
    g.player.FSM.Handle(g.player, g.lastIntent, dt)

    // Enemy FSMs.
    for _, e := range g.enemies {
        e.FSM.Handle(e, dt)
    }

    // Physics.
    g.player.ApplyPhysics(g.world, dt)
    for _, e := range g.enemies {
        e.ApplyPhysics(g.world, dt)
    }

    // Boundary clamp.
    soldierHalfW := float64(120*g.cfg.RenderScale) / 2
    g.player.X = world.Clamp(g.player.X, soldierHalfW, float64(g.cfg.WindowW)-soldierHalfW)

    orcHalfW := float64(100*g.cfg.RenderScale) / 2
    for _, e := range g.enemies {
        clamped := world.Clamp(e.X, orcHalfW, float64(g.cfg.WindowW)-orcHalfW)
        if clamped != e.X && e.FSM.CurrentID() == enemy.StateRun {
            // bounced off edge: flip facing
            if e.X <= orcHalfW {
                e.Facing = 1
            } else {
                e.Facing = -1
            }
        }
        e.X = clamped
    }

    // Anim advance (current frame tick for rendering + frame windows).
    if g.player.Current != nil {
        g.player.Current.Update(dt)
    }
    for _, e := range g.enemies {
        if e.Current != nil && e.FSM.CurrentID() != enemy.StateFall {
            e.Current.Update(dt)
        }
    }
    g.hud.Update(dt)

    // Spawn.
    if spawned := g.spawner.Tick(dt, len(g.enemies)); spawned != nil {
        g.enemies = append(g.enemies, spawned)
    }

    // Combat resolution.
    g.dispatchSoldierHits()
    g.dispatchOrcHits()

    // Cull dead orcs.
    alive := g.enemies[:0]
    for _, e := range g.enemies {
        if !e.Dead {
            alive = append(alive, e)
        }
    }
    g.enemies = alive

    // Game over check.
    if g.player.FSM.CurrentID() == player.StateDeath && g.player.Current != nil && g.player.Current.Done() {
        g.state = GameOverState
    }

    return nil
}

func (g *Game) dispatchSoldierHits() {
    attackers := []combat.Fighter{g.player}
    victims := make([]combat.Fighter, 0, len(g.enemies))
    for _, e := range g.enemies {
        victims = append(victims, e)
    }
    for _, ev := range combat.Resolve(attackers, victims) {
        orc := ev.Victim.(*enemy.Enemy)
        orc.OnHit(g.player.X)
    }
}

func (g *Game) dispatchOrcHits() {
    attackers := make([]combat.Fighter, 0, len(g.enemies))
    for _, e := range g.enemies {
        attackers = append(attackers, e)
    }
    victims := []combat.Fighter{g.player}
    for _, ev := range combat.Resolve(attackers, victims) {
        orc := ev.Attacker.(*enemy.Enemy)
        g.player.OnHit(g.combatTuning.SoldierKnockbackVX, g.combatTuning.SoldierKnockbackVY, orc.X)
    }
}

func (g *Game) reset() {
    g.enemies = nil
    g.spawner.Reset()
    g.player = player.New(player.Config{
        StartX:     float64(g.cfg.WindowW) / 2,
        StartY:     g.world.GroundY,
        Physics:    g.physics,
        Anims:      g.player.Anims, // reuse loaded anims
        Boxes:      g.player.Boxes,
        StartLives: g.combatTuning.SoldierMaxLives,
    })
    g.player.Grounded = true
    g.hud = hud.NewHUD(g.hud.Heart, g.hud.Face, livesProvider{g.player}, g.cfg.WindowW, g.hud.Scale)
    g.state = Playing
}

func (g *Game) Draw(screen *ebiten.Image) {
    screen.Fill(color.RGBA{0x80, 0x80, 0x80, 0xFF})

    vector.DrawFilledRect(screen, 0, float32(g.world.GroundY), float32(g.cfg.WindowW), float32(g.cfg.WindowH)-float32(g.world.GroundY),
        color.RGBA{0x3A, 0x3A, 0x3A, 0xFF}, false)

    // Enemies first (sorted by Y asc for pseudo-depth).
    enemiesSorted := append([]*enemy.Enemy(nil), g.enemies...)
    // simple insertion sort (n is small).
    for i := 1; i < len(enemiesSorted); i++ {
        for j := i; j > 0 && enemiesSorted[j].Y < enemiesSorted[j-1].Y; j-- {
            enemiesSorted[j], enemiesSorted[j-1] = enemiesSorted[j-1], enemiesSorted[j]
        }
    }
    for _, e := range enemiesSorted {
        g.drawEnemy(screen, e)
    }

    // Soldier.
    g.drawPlayer(screen)

    // HUD.
    g.hud.Draw(screen)

    // Hitbox debug.
    if g.hitboxDebug {
        g.drawHitboxes(screen)
    }

    // Debug overlay (F3).
    g.overlay.Draw(screen)

    // Game over on top.
    if g.state == GameOverState {
        g.gameOver.Draw(screen)
    }
}

func (g *Game) drawPlayer(screen *ebiten.Image) {
    if g.player.Current == nil || g.player.Current.CurrentFrame() == nil {
        return
    }
    op := &ebiten.DrawImageOptions{}
    op.GeoM.Translate(-120.0/2, -80.0)
    if g.player.Facing < 0 {
        op.GeoM.Scale(-1, 1)
    }
    op.GeoM.Scale(float64(g.cfg.RenderScale), float64(g.cfg.RenderScale))
    op.GeoM.Translate(g.player.X, g.player.Y)
    op.Filter = ebiten.FilterNearest
    screen.DrawImage(g.player.Current.CurrentFrame(), op)
}

func (g *Game) drawEnemy(screen *ebiten.Image, e *enemy.Enemy) {
    if e.Current == nil || e.Current.CurrentFrame() == nil {
        return
    }
    op := &ebiten.DrawImageOptions{}
    op.GeoM.Translate(-100.0/2, -100.0)
    if e.Facing < 0 {
        op.GeoM.Scale(-1, 1)
    }
    op.GeoM.Scale(float64(g.cfg.RenderScale), float64(g.cfg.RenderScale))
    op.GeoM.Translate(e.X, e.Y)
    op.Filter = ebiten.FilterNearest
    screen.DrawImage(e.Current.CurrentFrame(), op)
}

func (g *Game) drawHitboxes(screen *ebiten.Image) {
    drawBox := func(anchorX, anchorY float64, facing int, box combat.Box, c color.Color) {
        var minX float64
        if facing >= 0 {
            minX = anchorX + float64(box.OffsetX)
        } else {
            minX = anchorX - float64(box.OffsetX) - float64(box.W)
        }
        minY := anchorY + float64(box.OffsetY)
        // scale by renderScale? Hitboxes are specified in screen (post-scale) coords per spec. Render as-is.
        vector.StrokeRect(screen, float32(minX), float32(minY), float32(box.W), float32(box.H), 2, c, false)
    }

    // soldier body (green), active hits (red)
    drawBox(g.player.X, g.player.Y, g.player.Facing, g.player.Boxes["body"], color.RGBA{0, 0xFF, 0, 0xFF})
    for _, h := range g.player.ActiveHits() {
        drawBox(g.player.X, g.player.Y, g.player.Facing, h, color.RGBA{0xFF, 0, 0, 0xFF})
    }

    for _, e := range g.enemies {
        drawBox(e.X, e.Y, e.Facing, e.Boxes["body"], color.RGBA{0, 0xFF, 0, 0xFF})
        for _, h := range e.ActiveHits() {
            drawBox(e.X, e.Y, e.Facing, h, color.RGBA{0xFF, 0, 0, 0xFF})
        }
    }
}
```

Note on hitbox coord scale: spec stores hitboxes in *post-scale screen units* (same space as `X`, `Y` — which is already post-scale since draw multiplies anchor by `renderScale`). Seeded values were chosen accordingly. No extra scaling at resolve time.

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: success. If `player.Player.Grounded` setter is required (`p.Grounded = true` — it's an exported field, works), no issue.

- [ ] **Step 3: Commit**

```bash
git add internal/game/game.go
git commit -m "feat(game): wire enemies, spawner, combat dispatch, HUD, game-over, F4 hitbox debug, R restart"
```

---

### Task 35: Update `cmd/game/main.go` wiring

**Files:**
- Modify: `cmd/game/main.go`

- [ ] **Step 1: Rewrite main**

```go
package main

import (
    "log"

    "github.com/hajimehoshi/ebiten/v2"

    "claude-pixel/internal/anim"
    "claude-pixel/internal/combat"
    "claude-pixel/internal/config"
    "claude-pixel/internal/debug"
    "claude-pixel/internal/enemy"
    "claude-pixel/internal/game"
    "claude-pixel/internal/hud"
    "claude-pixel/internal/player"
    "claude-pixel/internal/storage"
)

func main() {
    cfg := config.Load()

    db := storage.MustOpen(cfg)
    defer db.Close()

    animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
    tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
    hitboxRepo := storage.NewRepository[combat.HitboxSpec](db, combat.HitboxMapper{})

    anims, err := anim.LoadLibrary(cfg, animRepo)
    if err != nil {
        log.Fatalf("load animations: %v", err)
    }
    physics, err := player.LoadPhysics(tuneRepo)
    if err != nil {
        log.Fatalf("load physics: %v", err)
    }
    combatTuning, err := combat.LoadTuning(tuneRepo)
    if err != nil {
        log.Fatalf("load combat tuning: %v", err)
    }
    orcTuning, err := enemy.LoadTuning(tuneRepo)
    if err != nil {
        log.Fatalf("load orc tuning: %v", err)
    }
    soldierBoxes, err := combat.SoldierBoxes(hitboxRepo)
    if err != nil {
        log.Fatalf("load soldier boxes: %v", err)
    }
    orcBoxes, err := enemy.OrcBoxes(hitboxRepo)
    if err != nil {
        log.Fatalf("load orc boxes: %v", err)
    }
    orcAnims, err := enemy.OrcAnims(anims)
    if err != nil {
        log.Fatalf("pick orc anims: %v", err)
    }
    heart, ok := anims["heart_beat"]
    if !ok {
        log.Fatalf("missing heart_beat anim")
    }
    dbgCfg, err := debug.LoadConfig(cfg.DebugConfigPath)
    if err != nil {
        log.Fatalf("load debug config: %v", err)
    }
    if err := hud.LoadFont(cfg.FontPath); err != nil {
        log.Fatalf("load font: %v", err)
    }

    g := game.New(game.Deps{
        Cfg:          cfg,
        Anims:        anims,
        Physics:      physics,
        DebugCfg:     dbgCfg,
        SoldierBoxes: soldierBoxes,
        CombatTuning: combatTuning,
        OrcAnims:     orcAnims,
        OrcBoxes:     orcBoxes,
        OrcTuning:    orcTuning,
        HeartAnim:    heart,
        HUDFace:      hud.NewFace(32),
        OverTitle:    hud.NewFace(96),
        OverSubtitle: hud.NewFace(32),
    })

    ebiten.SetWindowSize(cfg.WindowW, cfg.WindowH)
    ebiten.SetWindowTitle("claude-pixel")
    if err := ebiten.RunGame(g); err != nil {
        log.Fatal(err)
    }
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add cmd/game/main.go
git commit -m "feat(cmd/game): wire hitboxes repo, orc tuning, hud font, new Deps"
```

---

## Phase 8 — Integration + manual verify

### Task 36: Full test suite green

- [ ] **Step 1: Run everything**

Run: `rm -rf data/ && go test ./...`

Expected: all packages PASS. If any fail, fix before proceeding.

- [ ] **Step 2: Run `go vet`**

Run: `go vet ./...`

Expected: clean.

- [ ] **Step 3: Commit any fixes**

(Only if fixes were needed.)

---

### Task 37: Manual smoke test

- [ ] **Step 1: Launch game**

Run: `rm -rf data/ && make run`

- [ ] **Step 2: Verify soldier boots correctly**

- Soldier appears, plays idle, responds to WASD.
- Walk into left/right edges: sprite stops at window boundary (no clip off-screen).
- No crash.

- [ ] **Step 3: Verify orcs spawn and move**

- Within 3–10s, orcs drop from above screen, land, start running left/right.
- Orcs flip direction when hitting boundary.
- At cap (3 alive), new orcs stop appearing.
- After 2s intervals, some orcs stop to play Attack/Attack2 anim, then resume running.

- [ ] **Step 4: Verify soldier combat**

- J/K attack: when active frame overlaps orc body, orc plays Hurt anim, bounces back, becomes briefly invulnerable.
- Second hit on same orc → plays Death anim → despawns after anim.
- Can't re-hit orc during Hurt i-frames.

- [ ] **Step 5: Verify orc combat**

- When an orc's attack hitbox overlaps soldier body, soldier bounces back + up, becomes invulnerable until grounded.
- Lives counter in top-right decrements.

- [ ] **Step 6: Verify HUD**

- Heart anim loops continuously in top-right.
- Text shows `x10` at start, decrements with each hit taken.

- [ ] **Step 7: Verify Game Over + restart**

- Take 10 hits (or temporarily set `soldier_max_lives = 1` via tune CLI + restart).
- Soldier plays death anim.
- "GAME OVER" + "Press R to restart" appears in monogram font.
- Press R → soldier respawns at center, enemies cleared, spawn timer resets.

- [ ] **Step 8: Verify debug overlay**

- Press F3: overlay appears (existing fields still work).
- Add new fields to `config/debug.json` (`orc_count`, `player_lives`, etc.) → restart → they display.
- Press F4: hitboxes draw (green body, red active attack).

- [ ] **Step 9: Note any issues**

Record anything off (hitbox dim tweaks, knockback feel, HUD scale, font rendering). Adjust via `make tune` or new migration if needed.

- [ ] **Step 10: Commit any tuning migrations**

```bash
git add internal/storage/migrations/014_*.sql
git commit -m "tune: adjust <thing> after smoke test"
```

(Only if changes were made.)

---

### Task 38: Wrap-up — update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Edit CLAUDE.md**

Add under a new "Combat & Enemy" section (brief):

```markdown
## Enemy + combat (2026-04-23)

Stage now spawns orcs. See `docs/superpowers/specs/2026-04-23-combat-and-enemy-design.md`.

- `internal/enemy` — orc entity + FSM (fall, run, attack, attack2, hurt, death).
- `internal/combat` — Box, Fighter, Resolve with AABB + facing flip + dedup.
- `internal/spawner` — timer-based orc spawner (3–10s, cap 3).
- `internal/hud` — heart HUD + GameOver + monogram font.
- `hitboxes` table + new `animations` columns (frame_w/h, path, is_player/is_enemy, grid cols/rows/pick_row).
- New tuning keys: `orc_*`, `soldier_knockback_*`, `soldier_max_lives`. Edit via `make tune`.
- Controls unchanged; add: F4 = hitbox debug overlay; R after Game Over = restart.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add combat + enemy section to CLAUDE.md"
```

---

## Summary

Total tasks: 38 (0–38, with Task 0 being verification-only).

**Commit cadence:** ~30 commits. Each task ends in a commit except for:
- Task 0 (verification only)
- Task 16 (partial — completed in Task 17/18)
- Task 17 (partial — completed in Task 18)
- Task 24 (partial — completed in Task 25)
- Task 36 (conditional)

**Build gate:** After every task, `go build ./...` must succeed before committing (except the explicit multi-task partials).

**Test gate:** After Phase 2, 3, 4, 5, 7: `go test ./...` must be green.

**Manual gate:** Task 37 is the irreducible manual verification — ebiten rendering, combat feel, HUD readability, font rendering cannot be automated and must be checked before declaring the feature done.
