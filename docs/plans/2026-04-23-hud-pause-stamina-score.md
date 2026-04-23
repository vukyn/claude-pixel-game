# HUD Overhaul: Pause, Stamina, Score, Data-Driven Layout — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add pause (Esc), sprint stamina (5s pool, 5s refill), per-kill score counter, data-driven HUD layout, migrate heart asset path, and render stamina bar from a sprite column.

**Architecture:** Two small pure packages (`internal/stamina`, `internal/score`) own state; `internal/hud` gains a storage-backed `Layout` so x/y/w/h/anchor/scale live in SQLite; `internal/game` gains a `Mode` enum (Playing/Paused/GameOver) and dispatches score on enemy death; `internal/anim` extends grid slicing to support `pick_col` (vertical column strips). New SQL migrations (023–029) add columns, tables, and seed rows. `cmd/tune` grows a `hud` subcommand.

**Tech Stack:** Go 1.22+, ebiten/v2, SQLite via `modernc.org/sqlite`, `urfave/cli/v3`.

**Spec:** `docs/superpowers/specs/2026-04-23-hud-pause-stamina-score-design.md`

---

## File Map

### New files
- `internal/stamina/pool.go` — `Pool` struct with drain/regen on tick
- `internal/stamina/pool_test.go` — table-driven tests
- `internal/score/counter.go` — `Counter{total int}`
- `internal/score/counter_test.go` — unit tests
- `internal/hud/layout.go` — `Element`, `Anchor`, `Layout`, mapper, `LoadLayout`, `Resolve`
- `internal/hud/layout_test.go` — Resolve + unknown-anchor + missing-key tests
- `internal/hud/pause.go` — PAUSED overlay
- `internal/storage/migrations/023_move_heart_to_huds.sql`
- `internal/storage/migrations/024_animations_add_pick_col.sql`
- `internal/storage/migrations/025_seed_stamina_bar_animation.sql`
- `internal/storage/migrations/026_hud_layout_schema.sql`
- `internal/storage/migrations/027_seed_hud_layout.sql`
- `internal/storage/migrations/028_seed_stamina_tuning.sql`
- `internal/storage/migrations/029_seed_enemy_points.sql`

### Modified files
- `internal/anim/spec.go` — add `PickCol` column + field
- `internal/anim/animation.go` — add `PickCol` to `AnimationSpec`
- `internal/anim/sheet.go` — add `SliceColumn` helper
- `internal/anim/library.go` — branch on `PickCol >= 0` to slice column
- `internal/enemy/kind.go` — `Kind.Points int` + load via `BuildKind`
- `internal/enemy/tuning.go` — `Tuning.Points` + load `<prefix>_points`
- `internal/player/player.go` — `Stamina *stamina.Pool` field
- `internal/player/states.go` — `groundSpeed` gates on `Stamina.CanSprint()`
- `internal/input/input.go` — `Intent.PauseEdge`
- `internal/hud/hud.go` — refactor to `Layout`-driven draw; stamina + score providers
- `internal/game/game.go` — `Mode` enum replacing `state`; pause handling; score dispatch; reset score
- `cmd/tune/main.go` — `hud` subcommand (list/get/set)
- `cmd/game/main.go` — load layout, construct stamina+score, wire HUD
- `CLAUDE.md` — update tuning key table, migrations count, controls, CLI reference

---

## Task Sequence Overview

1. Stamina package (pure, TDD)
2. Score package (pure, TDD)
3. Anim `pick_col` support (schema + loader)
4. Migrations batch A (heart move, anim pick_col, stamina bar seed, stamina tuning, enemy points)
5. Enemy `Kind.Points` wiring
6. Player stamina field + sprint gating
7. HUD layout package + mapper
8. HUD layout migrations (schema + seed)
9. HUD struct refactor (layout-driven, stamina bar, score text)
10. Pause overlay + Input.Intent.PauseEdge
11. Game `Mode` enum + pause loop + score dispatch + reset
12. `cmd/tune hud` subcommand
13. `cmd/game/main.go` wiring
14. CLAUDE.md doc refresh
15. Manual verification

---

## Task 1: Stamina Package

**Files:**
- Create: `internal/stamina/pool.go`
- Create: `internal/stamina/pool_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/stamina/pool_test.go
package stamina

import (
	"math"
	"testing"
	"time"
)

func approxEq(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestPoolStartsFull(t *testing.T) {
	p := NewPool(100, 20, 20)
	if p.Cur != 100 {
		t.Fatalf("want Cur=100, got %f", p.Cur)
	}
	if !p.CanSprint() {
		t.Fatal("want CanSprint true at full")
	}
	if !approxEq(p.Fraction(), 1.0) {
		t.Fatalf("want Fraction=1.0, got %f", p.Fraction())
	}
}

func TestPoolDrainsToZeroIn5s(t *testing.T) {
	p := NewPool(100, 20, 20)
	for i := 0; i < 300; i++ {
		p.Update(time.Second/60, true)
	}
	if !approxEq(p.Cur, 0) {
		t.Fatalf("want Cur=0 after 5s drain, got %f", p.Cur)
	}
	if p.CanSprint() {
		t.Fatal("want CanSprint false when empty")
	}
}

func TestPoolRegensToMaxIn5s(t *testing.T) {
	p := NewPool(100, 20, 20)
	p.Cur = 0
	for i := 0; i < 300; i++ {
		p.Update(time.Second/60, false)
	}
	if !approxEq(p.Cur, 100) {
		t.Fatalf("want Cur=100 after 5s regen, got %f", p.Cur)
	}
}

func TestPoolClampsAtZero(t *testing.T) {
	p := NewPool(100, 20, 20)
	p.Cur = 1
	p.Update(time.Second, true) // would drain 20, clamps at 0
	if p.Cur != 0 {
		t.Fatalf("want Cur=0 clamped, got %f", p.Cur)
	}
}

func TestPoolClampsAtMax(t *testing.T) {
	p := NewPool(100, 20, 20)
	p.Cur = 99
	p.Update(time.Second, false)
	if p.Cur != 100 {
		t.Fatalf("want Cur=100 clamped, got %f", p.Cur)
	}
}

func TestPoolNoChangeWhenNotSprintingAndFull(t *testing.T) {
	p := NewPool(100, 20, 20)
	p.Update(time.Second, false)
	if p.Cur != 100 {
		t.Fatalf("want Cur=100, got %f", p.Cur)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stamina/...`
Expected: FAIL with "undefined: NewPool".

- [ ] **Step 3: Write minimal implementation**

```go
// internal/stamina/pool.go
package stamina

import "time"

type Pool struct {
	Max         float64
	Cur         float64
	DrainPerSec float64
	RegenPerSec float64
}

func NewPool(max, drain, regen float64) *Pool {
	return &Pool{Max: max, Cur: max, DrainPerSec: drain, RegenPerSec: regen}
}

func (p *Pool) Update(dt time.Duration, sprinting bool) {
	dtS := dt.Seconds()
	if sprinting {
		p.Cur -= p.DrainPerSec * dtS
	} else {
		p.Cur += p.RegenPerSec * dtS
	}
	if p.Cur < 0 {
		p.Cur = 0
	}
	if p.Cur > p.Max {
		p.Cur = p.Max
	}
}

func (p *Pool) Fraction() float64 {
	if p.Max <= 0 {
		return 0
	}
	return p.Cur / p.Max
}

func (p *Pool) CanSprint() bool { return p.Cur > 0 }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/stamina/... -v`
Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/stamina/
git commit -m "feat(stamina): pool with drain/regen tick" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Score Package

**Files:**
- Create: `internal/score/counter.go`
- Create: `internal/score/counter_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/score/counter_test.go
package score

import "testing"

func TestCounterStartsZero(t *testing.T) {
	c := &Counter{}
	if c.Total() != 0 {
		t.Fatalf("want 0, got %d", c.Total())
	}
}

func TestCounterAddAccumulates(t *testing.T) {
	c := &Counter{}
	c.Add(10)
	c.Add(15)
	if c.Total() != 25 {
		t.Fatalf("want 25, got %d", c.Total())
	}
}

func TestCounterResetZeroes(t *testing.T) {
	c := &Counter{}
	c.Add(42)
	c.Reset()
	if c.Total() != 0 {
		t.Fatalf("want 0 after Reset, got %d", c.Total())
	}
}

func TestCounterIgnoresNegativeOrZero(t *testing.T) {
	c := &Counter{}
	c.Add(-5)
	c.Add(0)
	if c.Total() != 0 {
		t.Fatalf("want 0, got %d", c.Total())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/score/...`
Expected: FAIL with "undefined: Counter".

- [ ] **Step 3: Write minimal implementation**

```go
// internal/score/counter.go
package score

type Counter struct {
	total int
}

func (c *Counter) Add(n int) {
	if n <= 0 {
		return
	}
	c.total += n
}

func (c *Counter) Total() int { return c.total }

func (c *Counter) Reset() { c.total = 0 }
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/score/... -v`
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/score/
git commit -m "feat(score): counter with add/reset" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Anim `pick_col` — Spec + Slicer

Extend `AnimationSpec` with optional `PickCol` (default -1) and add column slicer. No DB change yet — that's Task 4.

**Files:**
- Modify: `internal/anim/animation.go`
- Modify: `internal/anim/spec.go`
- Modify: `internal/anim/sheet.go`
- Modify: `internal/anim/sheet_test.go`

- [ ] **Step 1: Extend `AnimationSpec` with `PickCol` field**

Edit `internal/anim/animation.go`, add field after `PickRow`:

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
	PickCol    int // -1 = row mode (use PickRow). >=0 = column-strip mode.
}
```

- [ ] **Step 2: Add column slicer + test**

Append to `internal/anim/sheet.go`:

```go
// SliceColumn slices a 2D grid sheet (cols x rows of frameW x frameH), picking
// `count` consecutive frames from column `pickCol` (0-indexed), top-to-bottom.
func SliceColumn(img *ebiten.Image, frameW, frameH, pickCol, count int) []*ebiten.Image {
	frames := make([]*ebiten.Image, count)
	for i := 0; i < count; i++ {
		x0 := pickCol * frameW
		y0 := i * frameH
		frames[i] = img.SubImage(image.Rect(x0, y0, x0+frameW, y0+frameH)).(*ebiten.Image)
	}
	return frames
}
```

Append to `internal/anim/sheet_test.go`:

```go
func TestSliceColumnRects(t *testing.T) {
	img := ebiten.NewImage(48*4, 16*10)
	frames := SliceColumn(img, 48, 16, 2, 10)
	if len(frames) != 10 {
		t.Fatalf("want 10 frames, got %d", len(frames))
	}
	// Frame 0 should start at x=2*48=96, y=0
	b := frames[0].Bounds()
	if b.Min.X != 96 || b.Min.Y != 0 {
		t.Fatalf("frame 0 min=(%d,%d), want (96,0)", b.Min.X, b.Min.Y)
	}
	// Frame 9 should start at x=96, y=9*16=144
	b9 := frames[9].Bounds()
	if b9.Min.X != 96 || b9.Min.Y != 144 {
		t.Fatalf("frame 9 min=(%d,%d), want (96,144)", b9.Min.X, b9.Min.Y)
	}
}
```

- [ ] **Step 3: Run column-slicer test (expect fail if existing sheet_test uses NewImage without ebiten runtime)**

Run: `go test ./internal/anim/... -run TestSliceColumnRects -v`
Expected: PASS (existing grid test already uses `ebiten.NewImage` in a headless-safe way).

If PASS skip Step 4. If FAIL due to rectangle math, fix and rerun.

- [ ] **Step 4: Update `SpecMapper` to include `pick_col`**

Edit `internal/anim/spec.go`:

```go
func (SpecMapper) Columns() []string {
	return []string{
		"id", "file", "frame_count", "duration_ms", "loop",
		"frame_w", "frame_h", "path", "is_player", "is_enemy",
		"grid_cols", "grid_rows", "pick_row", "pick_col",
	}
}

func (SpecMapper) Scan(row storage.Scanner) (AnimationSpec, error) {
	var s AnimationSpec
	var loopInt, isPlayerInt, isEnemyInt int
	err := row.Scan(
		&s.ID, &s.File, &s.FrameCount, &s.DurationMs, &loopInt,
		&s.FrameW, &s.FrameH, &s.Path, &isPlayerInt, &isEnemyInt,
		&s.GridCols, &s.GridRows, &s.PickRow, &s.PickCol,
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
		s.GridCols, s.GridRows, s.PickRow, s.PickCol,
	}
}
```

- [ ] **Step 5: Update `LoadLibrary` to branch on `PickCol`**

Edit `internal/anim/library.go`, replace the grid branch body:

```go
if spec.GridCols > 0 {
    wantW := spec.FrameW * spec.GridCols
    wantH := spec.FrameH * spec.GridRows
    if w != wantW || h != wantH {
        return nil, fmt.Errorf("sheet %s (grid): got %dx%d, want %dx%d", spec.Path, w, h, wantW, wantH)
    }
    if spec.PickCol >= 0 && spec.PickRow != 0 {
        return nil, fmt.Errorf("animation %s: pick_row and pick_col are mutually exclusive (set pick_row=0 when using pick_col)", spec.ID)
    }
    if spec.PickCol >= 0 {
        frames = SliceColumn(img, spec.FrameW, spec.FrameH, spec.PickCol, spec.FrameCount)
    } else {
        frames = SliceGrid(img, spec.FrameW, spec.FrameH, spec.GridCols, spec.GridRows, spec.PickRow, spec.FrameCount)
    }
}
```

- [ ] **Step 6: Run tests — everything compiles, no regressions**

Run: `go test ./...`
Expected: All existing tests PASS. No DB migration yet means `LoadLibrary` not exercised against real DB; unit tests use in-memory specs so should still compile. If any existing code constructs `AnimationSpec{}` literals without `PickCol`, zero-value = 0 which would incorrectly trigger column mode. Fix: in `LoadLibrary`, check `PickCol > 0 OR (PickCol == 0 AND row-zero-explicitly-set)` — but simpler: require migration to set default to -1, and for zero-value Go literals they default to 0 which could conflict.

**Guard against Go zero-value collision:** change the check to require `spec.PickCol > 0` OR add an explicit `HasPickCol bool` column. Choose option: change sentinel from -1 to use a dedicated flag column. Simpler approach: treat `PickCol >= 0` as opt-in AND make `LoadLibrary` only consult `PickCol` when row came from DB (always the case here), AND set DB default to -1. For Go-literal tests that don't set PickCol, they also don't set `GridCols > 0` (they use `Slice`, not `SliceGrid`), so the column branch is never reached.

Accept sentinel=-1 with DB default -1. Proceed.

- [ ] **Step 7: Commit**

```bash
git add internal/anim/
git commit -m "feat(anim): support pick_col for vertical column strips" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Migrations Batch A — Asset Move, Anim Column, Seeds

Seven migrations added in sequence; each is small, commit all together since they're data changes that must land atomically for next-run boot.

**Files:**
- Create: `internal/storage/migrations/023_move_heart_to_huds.sql`
- Create: `internal/storage/migrations/024_animations_add_pick_col.sql`
- Create: `internal/storage/migrations/025_seed_stamina_bar_animation.sql`
- Create: `internal/storage/migrations/028_seed_stamina_tuning.sql`
- Create: `internal/storage/migrations/029_seed_enemy_points.sql`

(026 & 027 are HUD layout — deferred to Task 7 after the HUD layout mapper exists.)

- [ ] **Step 1: Write 023 — move heart asset path**

```sql
-- 023_move_heart_to_huds.sql
UPDATE animations
   SET path = 'huds/healthbar/heartbeat.png',
       file = 'heartbeat.png'
 WHERE id = 'heart_beat';
```

- [ ] **Step 2: Write 024 — add pick_col column**

```sql
-- 024_animations_add_pick_col.sql
ALTER TABLE animations ADD COLUMN pick_col INTEGER NOT NULL DEFAULT -1;
```

- [ ] **Step 3: Write 025 — seed stamina_bar animation row**

```sql
-- 025_seed_stamina_bar_animation.sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path,
     is_player, is_enemy, grid_cols, grid_rows, pick_row, pick_col)
VALUES
    ('stamina_bar', 'healthbar.png', 10, 0, 0, 48, 16,
     'huds/healthbar/healthbar.png', 0, 0, 4, 10, 0, 2);
```

- [ ] **Step 4: Write 028 — seed stamina tuning rows**

```sql
-- 028_seed_stamina_tuning.sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('stamina_max',          100, 10,  500, '',   'max stamina pool'),
    ('stamina_drain_per_s',   20,  1,  500, '/s', 'stamina drain rate while sprinting'),
    ('stamina_regen_per_s',   20,  1,  500, '/s', 'stamina regen rate while not sprinting');
```

- [ ] **Step 5: Write 029 — seed enemy points**

```sql
-- 029_seed_enemy_points.sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('orc_points',   10, 0, 1000, '', 'points awarded when orc killed'),
    ('slime_points', 15, 0, 1000, '', 'points awarded when slime killed');
```

- [ ] **Step 6: Delete old DB and re-run migrations**

Run: `rm -rf data/ && go run ./cmd/tune list | tail -20`
Expected: new keys `stamina_max`, `stamina_drain_per_s`, `stamina_regen_per_s`, `orc_points`, `slime_points` appear. No errors.

- [ ] **Step 7: Verify stamina_bar anim spec loads (smoke)**

Run: `go build ./...`
Expected: builds cleanly. `LoadLibrary` will be exercised at next `make run` — not yet, asset consumer missing.

- [ ] **Step 8: Commit**

```bash
git add internal/storage/migrations/
git commit -m "feat(migrations): heart path + pick_col + stamina/score seeds" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Enemy `Kind.Points`

**Files:**
- Modify: `internal/enemy/tuning.go`
- Modify: `internal/enemy/kind.go`
- Modify: `internal/enemy/loader_test.go` (add test)

- [ ] **Step 1: Write the failing test**

Append to `internal/enemy/loader_test.go`:

```go
func TestLoadTuningForIncludesPoints(t *testing.T) {
	db := testDB(t)
	repo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
	ctx := context.Background()
	seed := []player.TuningParam{
		{Key: "orc_max_lives", Value: 2, MaxValue: 10},
		{Key: "orc_run_speed", Value: 60, MaxValue: 500},
		{Key: "orc_intent_tick_s", Value: 2, MaxValue: 10},
		{Key: "orc_hurt_bounce_vx", Value: 120, MaxValue: 500},
		{Key: "orc_hurt_bounce_vy", Value: -180, MinValue: -500},
		{Key: "orc_foot_padding", Value: 20, MaxValue: 100},
		{Key: "orc_points", Value: 10, MaxValue: 1000},
	}
	for _, p := range seed {
		if err := repo.Upsert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	tun, err := LoadTuningFor(repo, "orc")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if tun.Points != 10 {
		t.Fatalf("want Points=10, got %d", tun.Points)
	}
}
```

(If `testDB` helper does not yet exist in this file, search for it. If absent, copy pattern from existing `TestLoadTuningFor` test — reuse its setup helper.)

- [ ] **Step 2: Run test, expect fail**

Run: `go test ./internal/enemy/... -run TestLoadTuningForIncludesPoints -v`
Expected: FAIL — `tun.Points` undefined.

- [ ] **Step 3: Add `Points` to `Tuning` + load**

Edit `internal/enemy/tuning.go`:

```go
type Tuning struct {
	MaxLives     float64
	RunSpeed     float64
	IntentTickS  float64
	HurtBounceVX float64
	HurtBounceVY float64
	FootPadding  int
	Points       int
}
```

And in `LoadTuningFor`, after `t.FootPadding = int(pad)`:

```go
pts, err := pick(prefix + "_points")
if err != nil {
    return nil, err
}
t.Points = int(pts)
```

- [ ] **Step 4: Expose `Points` on `Kind`**

Edit `internal/enemy/kind.go` — no struct change needed (Points is already on `Tuning`, accessed via `kind.Tuning.Points`). Skip code change here, just confirm.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/enemy/... -v`
Expected: all PASS including new `TestLoadTuningForIncludesPoints`.

- [ ] **Step 6: Commit**

```bash
git add internal/enemy/
git commit -m "feat(enemy): load per-kind points tuning" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Player Stamina Field + Sprint Gate

**Files:**
- Modify: `internal/player/player.go`
- Modify: `internal/player/states.go`
- Modify: `internal/player/fsm_test.go` (new tests)
- Modify: `cmd/game/main.go` (load stamina tuning, construct pool)

- [ ] **Step 1: Add stamina-tuning loader**

Edit `internal/player/physics.go` — append:

```go
type StaminaTuning struct {
	Max, DrainPerSec, RegenPerSec float64
}

func LoadStaminaTuning(repo *storage.Repository[TuningParam]) (*StaminaTuning, error) {
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
	st := &StaminaTuning{}
	var e error
	if st.Max, e = pick("stamina_max"); e != nil {
		return nil, e
	}
	if st.DrainPerSec, e = pick("stamina_drain_per_s"); e != nil {
		return nil, e
	}
	if st.RegenPerSec, e = pick("stamina_regen_per_s"); e != nil {
		return nil, e
	}
	return st, nil
}
```

- [ ] **Step 2: Add `Stamina *stamina.Pool` to Player**

Edit `internal/player/player.go`:

```go
import (
	// ... existing imports
	"claude-pixel/internal/stamina"
)

type Player struct {
	// ... existing fields
	Stamina *stamina.Pool
}

type Config struct {
	// ... existing
	Stamina *stamina.Pool
}

// In New(...):
func New(cfg Config) *Player {
	p := &Player{
		// ... existing
		Stamina: cfg.Stamina,
	}
	// ... rest unchanged
}
```

- [ ] **Step 3: Gate sprint in `groundSpeed` + drain**

Edit `internal/player/states.go`:

```go
func groundSpeed(p *Player, in input.Intent) float64 {
	if in.SprintHeld && p.Stamina != nil && p.Stamina.CanSprint() {
		return p.Physics.SprintSpeed
	}
	return p.Physics.RunSpeed
}
```

Drain happens in game loop — do that in Task 11 (game integration). For now expose a helper:

```go
// IsSprinting reports whether the player will sprint this tick given intent.
func (p *Player) IsSprinting(in input.Intent) bool {
	if p.FSM == nil {
		return false
	}
	if !p.Grounded {
		return false
	}
	if p.Stamina == nil || !p.Stamina.CanSprint() {
		return false
	}
	if !in.SprintHeld {
		return false
	}
	if moveDir(in) == 0 {
		return false
	}
	return true
}
```

- [ ] **Step 4: Write test — sprint blocked when empty**

Append to `internal/player/fsm_test.go`:

```go
func TestRunUsesRunSpeedWhenStaminaEmpty(t *testing.T) {
	pool := stamina.NewPool(100, 20, 20)
	pool.Cur = 0 // empty
	p := newTestPlayer(t, pool)
	p.FSM.Transition(p, StateRun)
	in := input.Intent{Right: true, SprintHeld: true}
	p.FSM.Handle(p, in, time.Second/60)
	if p.VX != p.Physics.RunSpeed {
		t.Fatalf("want VX=RunSpeed=%f when stamina empty, got %f", p.Physics.RunSpeed, p.VX)
	}
}

func TestRunUsesSprintSpeedWhenStaminaAvailable(t *testing.T) {
	pool := stamina.NewPool(100, 20, 20)
	p := newTestPlayer(t, pool)
	p.FSM.Transition(p, StateRun)
	in := input.Intent{Right: true, SprintHeld: true}
	p.FSM.Handle(p, in, time.Second/60)
	if p.VX != p.Physics.SprintSpeed {
		t.Fatalf("want VX=SprintSpeed=%f, got %f", p.Physics.SprintSpeed, p.VX)
	}
}
```

If `newTestPlayer` helper already exists with a different signature, inspect `fsm_test.go` first and adapt to its pattern. If it doesn't accept a `*stamina.Pool`, add a variadic/option form or create a sibling helper `newTestPlayerWithStamina`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/player/... -v`
Expected: new tests PASS, existing tests still PASS. Existing tests construct Player without stamina — `Stamina==nil` branch in `groundSpeed` means `SprintHeld=true` falls through to `RunSpeed`. Verify any existing sprint test still passes; if it asserted SprintSpeed, it now needs a pool.

**If existing sprint test breaks:** update it to construct a pool. Example: replace `player.New(player.Config{...})` with one that sets `Stamina: stamina.NewPool(100, 20, 20)`.

- [ ] **Step 6: Commit**

```bash
git add internal/player/
git commit -m "feat(player): sprint gated by stamina pool" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: HUD Layout Package + Mapper

Define `Element`, `Anchor`, `Layout`, mapper, `LoadLayout`, and `Resolve`. Add storage table + seed.

**Files:**
- Create: `internal/hud/layout.go`
- Create: `internal/hud/layout_test.go`
- Create: `internal/storage/migrations/026_hud_layout_schema.sql`
- Create: `internal/storage/migrations/027_seed_hud_layout.sql`

- [ ] **Step 1: Write 026 schema migration**

```sql
-- 026_hud_layout_schema.sql
CREATE TABLE hud_layout (
    key     TEXT    PRIMARY KEY,
    x       INTEGER NOT NULL,
    y       INTEGER NOT NULL,
    w       INTEGER NOT NULL,
    h       INTEGER NOT NULL,
    anchor  TEXT    NOT NULL CHECK(anchor IN ('top_left','top_right','bottom_left','bottom_right')),
    scale   REAL    NOT NULL DEFAULT 1.0
);
```

- [ ] **Step 2: Write 027 seed migration**

```sql
-- 027_seed_hud_layout.sql
INSERT OR IGNORE INTO hud_layout (key, x, y, w, h, anchor, scale) VALUES
    ('heart',        48, 16, 16, 16, 'top_right', 2.0),
    ('lives_text',   16, 16,  0,  0, 'top_right', 1.0),
    ('score_text',   16, 16,  0,  0, 'top_left',  1.0),
    ('stamina_bar',  16, 48, 48, 16, 'top_left',  2.0);
```

- [ ] **Step 3: Write failing layout test**

```go
// internal/hud/layout_test.go
package hud

import "testing"

func TestResolveTopLeft(t *testing.T) {
	l := Layout{"score_text": Element{X: 16, Y: 16, W: 100, H: 24, Anchor: AnchorTopLeft, Scale: 1}}
	x, y := l.Resolve("score_text", 800, 600)
	if x != 16 || y != 16 {
		t.Fatalf("want (16,16), got (%f,%f)", x, y)
	}
}

func TestResolveTopRight(t *testing.T) {
	l := Layout{"heart": Element{X: 48, Y: 16, W: 32, H: 32, Anchor: AnchorTopRight, Scale: 2}}
	x, y := l.Resolve("heart", 800, 600)
	// top-right anchor: element's right edge sits at windowW - X, top at Y
	// element width=32 → left = 800 - 48 - 32 = 720? Actually spec: x is offset of element's right edge from screen right.
	// Wait — spec revised: "top_right: (x,y) = offset of element's top-right from screen top-right (grows left/down)".
	// So element's top-right corner at (windowW - X, Y) = (800-48, 16) = (752, 16).
	// Returned top-left = (752 - W, 16) = (752 - 32, 16) = (720, 16).
	if x != 720 || y != 16 {
		t.Fatalf("want (720,16), got (%f,%f)", x, y)
	}
}

func TestResolveBottomLeft(t *testing.T) {
	l := Layout{"bar": Element{X: 10, Y: 20, W: 50, H: 10, Anchor: AnchorBottomLeft, Scale: 1}}
	x, y := l.Resolve("bar", 800, 600)
	// bottom-left: element's bottom-left at (X, windowH - Y) = (10, 580). Top-left = (10, 580 - H) = (10, 570).
	if x != 10 || y != 570 {
		t.Fatalf("want (10,570), got (%f,%f)", x, y)
	}
}

func TestResolveBottomRight(t *testing.T) {
	l := Layout{"bar": Element{X: 10, Y: 20, W: 50, H: 10, Anchor: AnchorBottomRight, Scale: 1}}
	x, y := l.Resolve("bar", 800, 600)
	// bottom-right: bottom-right at (800-10, 600-20)=(790, 580). Top-left = (740, 570).
	if x != 740 || y != 570 {
		t.Fatalf("want (740,570), got (%f,%f)", x, y)
	}
}

func TestParseAnchorValidAndInvalid(t *testing.T) {
	a, err := ParseAnchor("top_left")
	if err != nil || a != AnchorTopLeft {
		t.Fatalf("top_left: got %v err=%v", a, err)
	}
	if _, err := ParseAnchor("middle"); err == nil {
		t.Fatal("want error for unknown anchor")
	}
}
```

- [ ] **Step 4: Run test, expect fail**

Run: `go test ./internal/hud/... -v`
Expected: FAIL — undefined Layout/Element/Anchor.

- [ ] **Step 5: Implement layout.go**

```go
// internal/hud/layout.go
package hud

import (
	"context"
	"fmt"

	"claude-pixel/internal/storage"
)

type Anchor int

const (
	AnchorTopLeft Anchor = iota
	AnchorTopRight
	AnchorBottomLeft
	AnchorBottomRight
)

func (a Anchor) String() string {
	switch a {
	case AnchorTopLeft:
		return "top_left"
	case AnchorTopRight:
		return "top_right"
	case AnchorBottomLeft:
		return "bottom_left"
	case AnchorBottomRight:
		return "bottom_right"
	}
	return "unknown"
}

func ParseAnchor(s string) (Anchor, error) {
	switch s {
	case "top_left":
		return AnchorTopLeft, nil
	case "top_right":
		return AnchorTopRight, nil
	case "bottom_left":
		return AnchorBottomLeft, nil
	case "bottom_right":
		return AnchorBottomRight, nil
	}
	return 0, fmt.Errorf("unknown anchor %q (valid: top_left, top_right, bottom_left, bottom_right)", s)
}

type Element struct {
	X, Y, W, H int
	Anchor     Anchor
	Scale      float64
}

type Layout map[string]Element

// Resolve returns the element's absolute top-left in screen pixels.
// For variable-width text (W=0), caller should measure width and subtract
// for right-anchored, etc. W and H here are stored values from DB.
func (l Layout) Resolve(key string, screenW, screenH int) (x, y float64) {
	e, ok := l[key]
	if !ok {
		return 0, 0
	}
	switch e.Anchor {
	case AnchorTopLeft:
		return float64(e.X), float64(e.Y)
	case AnchorTopRight:
		return float64(screenW - e.X - e.W), float64(e.Y)
	case AnchorBottomLeft:
		return float64(e.X), float64(screenH - e.Y - e.H)
	case AnchorBottomRight:
		return float64(screenW - e.X - e.W), float64(screenH - e.Y - e.H)
	}
	return 0, 0
}

// LayoutRow is the DB entity representation.
type LayoutRow struct {
	Key      string
	X        int
	Y        int
	W        int
	H        int
	AnchorS  string
	Scale    float64
}

func (r LayoutRow) GetID() string { return r.Key }

type LayoutMapper struct{}

func (LayoutMapper) Table() string { return "hud_layout" }

func (LayoutMapper) Columns() []string {
	return []string{"key", "x", "y", "w", "h", "anchor", "scale"}
}

func (LayoutMapper) Scan(row storage.Scanner) (LayoutRow, error) {
	var r LayoutRow
	err := row.Scan(&r.Key, &r.X, &r.Y, &r.W, &r.H, &r.AnchorS, &r.Scale)
	return r, err
}

func (LayoutMapper) Values(r LayoutRow) []any {
	return []any{r.Key, r.X, r.Y, r.W, r.H, r.AnchorS, r.Scale}
}

// LoadLayout reads every hud_layout row, parses anchors, validates required keys.
var requiredLayoutKeys = []string{"heart", "lives_text", "score_text", "stamina_bar"}

func LoadLayout(repo *storage.Repository[LayoutRow]) (Layout, error) {
	rows, err := repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	out := Layout{}
	for _, r := range rows {
		a, err := ParseAnchor(r.AnchorS)
		if err != nil {
			return nil, fmt.Errorf("hud_layout.%s: %w", r.Key, err)
		}
		if r.Scale <= 0 {
			return nil, fmt.Errorf("hud_layout.%s: scale must be > 0 (got %f)", r.Key, r.Scale)
		}
		out[r.Key] = Element{X: r.X, Y: r.Y, W: r.W, H: r.H, Anchor: a, Scale: r.Scale}
	}
	for _, k := range requiredLayoutKeys {
		if _, ok := out[k]; !ok {
			return nil, fmt.Errorf("hud_layout missing required key %q; required: %v", k, requiredLayoutKeys)
		}
	}
	return out, nil
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/hud/... -v`
Expected: PASS.

- [ ] **Step 7: Regenerate DB**

Run: `rm -rf data/ && go build ./... && go run ./cmd/tune list | head -5`
Expected: builds + `tune list` runs (migrations 023–029 applied; 026+027 create the hud_layout table + rows).

- [ ] **Step 8: Commit**

```bash
git add internal/hud/layout.go internal/hud/layout_test.go internal/storage/migrations/026_hud_layout_schema.sql internal/storage/migrations/027_seed_hud_layout.sql
git commit -m "feat(hud): storage-backed layout with anchor resolution" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: HUD Struct Refactor — Layout-Driven + Stamina Bar + Score

**Files:**
- Modify: `internal/hud/hud.go`

- [ ] **Step 1: Replace `HUD` struct with layout-driven form**

Full rewrite of `internal/hud/hud.go`:

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

type StaminaProvider interface {
	StaminaFraction() float64
}

type ScoreProvider interface {
	Score() int
}

type HUD struct {
	Heart      *anim.Animation
	StaminaBar *anim.Animation
	Face       *text.GoTextFace
	Lives      LivesProvider
	Stamina    StaminaProvider
	Score      ScoreProvider
	Layout     Layout
	WindowW    int
	WindowH    int
}

func NewHUD(
	heart *anim.Animation,
	staminaBar *anim.Animation,
	face *text.GoTextFace,
	lives LivesProvider,
	stamina StaminaProvider,
	score ScoreProvider,
	layout Layout,
	windowW, windowH int,
) *HUD {
	return &HUD{
		Heart: heart, StaminaBar: staminaBar, Face: face,
		Lives: lives, Stamina: stamina, Score: score,
		Layout: layout, WindowW: windowW, WindowH: windowH,
	}
}

func (h *HUD) Update(dt time.Duration) {
	if h.Heart != nil {
		h.Heart.Update(dt)
	}
	// stamina bar frame is chosen by stamina value, not time — no Update.
}

func formatLives(n int) string {
	if n < 0 {
		n = 0
	}
	return fmt.Sprintf("x%d", n)
}

func formatScore(n int) string { return fmt.Sprintf("Score: %d", n) }

func (h *HUD) Draw(screen *ebiten.Image) {
	h.drawHeart(screen)
	h.drawLives(screen)
	h.drawScore(screen)
	h.drawStamina(screen)
}

func (h *HUD) drawHeart(screen *ebiten.Image) {
	if h.Heart == nil {
		return
	}
	frame := h.Heart.CurrentFrame()
	if frame == nil {
		return
	}
	e := h.Layout["heart"]
	x, y := h.Layout.Resolve("heart", h.WindowW, h.WindowH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(e.Scale, e.Scale)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(frame, op)
}

func (h *HUD) drawLives(screen *ebiten.Image) {
	if h.Lives == nil || h.Face == nil {
		return
	}
	label := formatLives(h.Lives.Lives())
	tw, _ := text.Measure(label, h.Face, 0)
	e := h.Layout["lives_text"]
	// W=0 in DB → substitute measured width so right-anchor math works
	e.W = int(tw)
	// apply measured width only for this resolve:
	h.Layout["lives_text"] = e
	x, y := h.Layout.Resolve("lives_text", h.WindowW, h.WindowH)
	// restore W=0 so next frame re-measures (guard against face size change)
	e.W = 0
	h.Layout["lives_text"] = e

	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
	text.Draw(screen, label, h.Face, op)
}

func (h *HUD) drawScore(screen *ebiten.Image) {
	if h.Score == nil || h.Face == nil {
		return
	}
	label := formatScore(h.Score.Score())
	tw, _ := text.Measure(label, h.Face, 0)
	e := h.Layout["score_text"]
	e.W = int(tw)
	h.Layout["score_text"] = e
	x, y := h.Layout.Resolve("score_text", h.WindowW, h.WindowH)
	e.W = 0
	h.Layout["score_text"] = e

	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
	text.Draw(screen, label, h.Face, op)
}

func (h *HUD) drawStamina(screen *ebiten.Image) {
	if h.StaminaBar == nil || h.Stamina == nil {
		return
	}
	// manual frame selection based on fraction
	frac := h.Stamina.StaminaFraction()
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}
	// frame 0 = full, frame 9 = empty
	idx := int((1.0-frac)*9 + 0.5)
	if idx < 0 {
		idx = 0
	} else if idx > 9 {
		idx = 9
	}
	frame := h.StaminaBar.FrameAt(idx)
	if frame == nil {
		return
	}
	e := h.Layout["stamina_bar"]
	x, y := h.Layout.Resolve("stamina_bar", h.WindowW, h.WindowH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(e.Scale, e.Scale)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(frame, op)
}
```

- [ ] **Step 2: Add `FrameAt(i int)` to Animation**

Edit `internal/anim/animation.go`, append:

```go
// FrameAt returns the frame at explicit index (0-based), for non-time-driven
// animations (e.g. progress bars). Clamps to range; returns nil if no frames.
func (a *Animation) FrameAt(i int) *ebiten.Image {
	if len(a.frames) == 0 {
		return nil
	}
	if i < 0 {
		i = 0
	}
	if i >= len(a.frames) {
		i = len(a.frames) - 1
	}
	return a.frames[i]
}
```

- [ ] **Step 3: Update `hud_test.go` (if existing test stubs new `HUD` struct)**

Read `internal/hud/hud_test.go`; if it references the old `NewHUD(heart, face, provider, windowW, scale)` signature, update call-sites to the new one using `Layout{}` + new providers or skip (mock minimally).

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: build ERRORS in `internal/game/game.go` and `cmd/game/main.go` that call old `hud.NewHUD(...)`. These are fixed in Task 11+13.

Run: `go test ./internal/hud/...`
Expected: PASS (layout_test.go still passes; hud_test.go either passes or needs trivial update).

- [ ] **Step 5: Commit**

```bash
git add internal/hud/hud.go internal/anim/animation.go internal/hud/hud_test.go
git commit -m "refactor(hud): layout-driven draw with stamina + score" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

(Commit is allowed to break `game` + `cmd/game/main.go` temporarily; they're fixed in the very next tasks. If the workflow requires green main after every commit, batch Task 8+11+13 into a single commit at Task 13.)

---

## Task 9: Pause Overlay + Input PauseEdge

**Files:**
- Create: `internal/hud/pause.go`
- Modify: `internal/input/input.go`

- [ ] **Step 1: Add `PauseEdge` to Intent**

Edit `internal/input/input.go`:

```go
type Intent struct {
	Left, Right    bool
	JumpPressed    bool
	SprintHeld     bool
	AttackPressed  bool
	Attack2Pressed bool
	PauseEdge      bool
}

func Poll() Intent {
	return Intent{
		Left:           ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft),
		Right:          ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight),
		JumpPressed:    inpututil.IsKeyJustPressed(ebiten.KeySpace),
		SprintHeld:     ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight),
		AttackPressed:  inpututil.IsKeyJustPressed(ebiten.KeyJ) || inpututil.IsKeyJustPressed(ebiten.KeyX),
		Attack2Pressed: inpututil.IsKeyJustPressed(ebiten.KeyK) || inpututil.IsKeyJustPressed(ebiten.KeyC),
		PauseEdge:      inpututil.IsKeyJustPressed(ebiten.KeyEscape),
	}
}
```

- [ ] **Step 2: Create pause overlay**

```go
// internal/hud/pause.go
package hud

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Pause struct {
	Title    *text.GoTextFace
	Subtitle *text.GoTextFace
	WindowW  int
	WindowH  int
}

func NewPause(title, subtitle *text.GoTextFace, w, h int) *Pause {
	return &Pause{Title: title, Subtitle: subtitle, WindowW: w, WindowH: h}
}

func (p *Pause) Draw(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, 0, float32(p.WindowW), float32(p.WindowH),
		color.RGBA{0, 0, 0, 160}, false)

	drawCentered := func(s string, face *text.GoTextFace, yFrac float64) {
		w, _ := text.Measure(s, face, 0)
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(p.WindowW)/2-w/2, float64(p.WindowH)*yFrac)
		op.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
		text.Draw(screen, s, face, op)
	}

	drawCentered("PAUSED", p.Title, 0.40)
	drawCentered("Press any key to resume", p.Subtitle, 0.55)
}
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: still broken in game.go; hud package itself builds.

- [ ] **Step 4: Commit**

```bash
git add internal/hud/pause.go internal/input/input.go
git commit -m "feat(input,hud): pause edge intent + PAUSED overlay" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Game `Mode` Enum + Pause + Score Dispatch

**Files:**
- Modify: `internal/game/game.go`

- [ ] **Step 1: Replace `GameState` with `Mode` and add new fields**

Edit `internal/game/game.go`:

```go
type Mode int

const (
	ModePlaying Mode = iota
	ModePaused
	ModeGameOver
)
```

Delete the old `GameState` consts (`Playing`, `GameOverState`). Update the struct:

```go
type Game struct {
	cfg                *config.Config
	world              *world.World
	player             *player.Player
	enemies            []*enemy.Enemy
	spawner            *spawner.Spawner
	overlay            *debug.Overlay
	hud                *hud.HUD
	gameOver           *hud.GameOver
	pause              *hud.Pause
	mode               Mode
	hitboxDebug        bool
	lastIntent         input.Intent
	combatTuning       *combat.Tuning
	kinds              []*enemy.Kind
	physics            *player.Physics
	staminaTuning      *player.StaminaTuning
	rng                *rand.Rand
	score              *score.Counter
	swallowNextIntent  bool
}
```

Add import: `"claude-pixel/internal/score"` and `"claude-pixel/internal/stamina"`.

- [ ] **Step 2: Update `Deps` struct**

```go
type Deps struct {
	Cfg           *config.Config
	Anims         map[string]*anim.Animation
	Physics       *player.Physics
	StaminaTuning *player.StaminaTuning
	DebugCfg      *debug.Config
	SoldierBoxes  map[string]combat.Box
	CombatTuning  *combat.Tuning
	EnemyKinds    []*enemy.Kind
	SpawnTuning   *enemy.SpawnTuning
	HeartAnim     *anim.Animation
	StaminaAnim   *anim.Animation
	HUDFace       *text.GoTextFace
	OverTitle     *text.GoTextFace
	OverSubtitle  *text.GoTextFace
	Layout        hud.Layout
}
```

- [ ] **Step 3: Update `New(d Deps)` to construct stamina pool, score, pause overlay, HUD**

```go
func New(d Deps) *Game {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	w := world.New(d.Cfg, d.Physics.Gravity)

	pool := stamina.NewPool(d.StaminaTuning.Max, d.StaminaTuning.DrainPerSec, d.StaminaTuning.RegenPerSec)

	p := player.New(player.Config{
		StartX:     float64(d.Cfg.WindowW) / 2,
		StartY:     w.GroundY,
		Physics:    d.Physics,
		Anims:      d.Anims,
		Boxes:      d.SoldierBoxes,
		StartLives: d.CombatTuning.SoldierMaxLives,
		Stamina:    pool,
	})
	p.Grounded = true

	sc := &score.Counter{}

	g := &Game{
		cfg:           d.Cfg,
		world:         w,
		player:        p,
		combatTuning:  d.CombatTuning,
		kinds:         d.EnemyKinds,
		physics:       d.Physics,
		staminaTuning: d.StaminaTuning,
		rng:           rng,
		mode:          ModePlaying,
		score:         sc,
	}
	g.overlay = debug.NewOverlay(d.DebugCfg, g)

	// ... existing kindFactories + spawner construction unchanged ...

	g.hud = hud.NewHUD(
		d.HeartAnim, d.StaminaAnim, d.HUDFace,
		livesProvider{p}, staminaProvider{pool}, scoreProvider{sc},
		d.Layout, d.Cfg.WindowW, d.Cfg.WindowH,
	)
	g.gameOver = hud.NewGameOver(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)
	g.pause = hud.NewPause(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)

	return g
}

type staminaProvider struct{ pool *stamina.Pool }
func (s staminaProvider) StaminaFraction() float64 { return s.pool.Fraction() }

type scoreProvider struct{ c *score.Counter }
func (s scoreProvider) Score() int { return s.c.Total() }
```

- [ ] **Step 4: Rewrite `Update` with pause logic + stamina + score**

Replace the body of `Game.Update`:

```go
func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.overlay.Toggle()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
		g.hitboxDebug = !g.hitboxDebug
	}

	if g.mode == ModeGameOver {
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.reset()
		}
		return nil
	}

	if g.mode == ModePaused {
		if len(inpututil.AppendJustPressedKeys(nil)) > 0 {
			g.mode = ModePlaying
			g.swallowNextIntent = true
		}
		return nil
	}

	intent := input.Poll()
	if intent.PauseEdge {
		g.mode = ModePaused
		return nil
	}
	if g.swallowNextIntent {
		intent.JumpPressed = false
		intent.AttackPressed = false
		intent.Attack2Pressed = false
		intent.PauseEdge = false
		g.swallowNextIntent = false
	}
	g.lastIntent = intent
	dt := time.Second / 60

	// Stamina tick based on whether player will sprint this frame
	if g.player.Stamina != nil {
		g.player.Stamina.Update(dt, g.player.IsSprinting(intent))
	}

	g.player.FSM.Handle(g.player, g.lastIntent, dt)
	for _, e := range g.enemies {
		e.FSM.Handle(e, dt)
	}

	g.player.ApplyPhysics(g.world, dt)
	for _, e := range g.enemies {
		e.ApplyPhysics(g.world, dt)
	}

	// ... existing clamp block unchanged ...

	if g.player.Current != nil {
		g.player.Current.Update(dt)
	}
	for _, e := range g.enemies {
		if e.Current != nil && e.FSM.CurrentID() != enemy.StateFall {
			e.Current.Update(dt)
		}
	}
	g.hud.Update(dt)

	if spawned := g.spawner.Tick(dt, len(g.enemies)); spawned != nil {
		g.enemies = append(g.enemies, spawned)
	}

	g.dispatchSoldierHits()
	g.dispatchOrcHits()

	// Award score for newly-dead enemies before compacting
	alive := g.enemies[:0]
	for _, e := range g.enemies {
		if e.Dead {
			g.score.Add(e.Kind.Tuning.Points)
			continue
		}
		alive = append(alive, e)
	}
	g.enemies = alive

	if g.player.FSM.CurrentID() == player.StateDeath && g.player.Current != nil && g.player.Current.Done() {
		g.mode = ModeGameOver
	}

	return nil
}
```

- [ ] **Step 5: Update `reset` to refill stamina + reset score**

```go
func (g *Game) reset() {
	oldAnims := g.player.Anims
	oldBoxes := g.player.Boxes
	g.enemies = nil
	g.spawner.Reset()

	pool := stamina.NewPool(g.staminaTuning.Max, g.staminaTuning.DrainPerSec, g.staminaTuning.RegenPerSec)
	g.player = player.New(player.Config{
		StartX:     float64(g.cfg.WindowW) / 2,
		StartY:     g.world.GroundY,
		Physics:    g.physics,
		Anims:      oldAnims,
		Boxes:      oldBoxes,
		StartLives: g.combatTuning.SoldierMaxLives,
		Stamina:    pool,
	})
	g.player.Grounded = true

	g.score.Reset()

	// Refresh HUD providers that reference the new player/pool
	g.hud.Lives = livesProvider{g.player}
	g.hud.Stamina = staminaProvider{pool}
	g.mode = ModePlaying
}
```

- [ ] **Step 6: Update `Draw` to render pause overlay**

Append inside `Draw`, after the GameOver draw:

```go
if g.mode == ModePaused {
    g.pause.Draw(screen)
}
if g.mode == ModeGameOver {
    g.gameOver.Draw(screen)
}
```

(Remove the old duplicate `g.state == GameOverState` check — replaced by `g.mode == ModeGameOver`.)

Also update getter `Intent()`:

```go
func (g *Game) Intent() *input.Intent { return &g.lastIntent }
```

(unchanged).

- [ ] **Step 7: Search for all `g.state` / `Playing` / `GameOverState` references and replace**

Run: `grep -n "g.state\|GameOverState\|GameState" internal/game/ cmd/`
Expected: only `mode`-based references remain. Any hit → update.

- [ ] **Step 8: Build**

Run: `go build ./...`
Expected: still fails in `cmd/game/main.go` (Deps missing `StaminaTuning`, `StaminaAnim`, `Layout`). Fixed in Task 12.

- [ ] **Step 9: Commit**

```bash
git add internal/game/game.go
git commit -m "feat(game): mode enum with pause + score dispatch + stamina tick" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: `cmd/tune hud` Subcommand

**Files:**
- Modify: `cmd/tune/main.go`

- [ ] **Step 1: Add `hudCmd` factory + register in app**

At top of file add import:

```go
import (
	// ... existing
	"claude-pixel/internal/hud"
)
```

Add `hudRepo` to main():

```go
hudRepo := storage.NewRepository[hud.LayoutRow](db, hud.LayoutMapper{})
```

And register `hudCmd(hudRepo)` in the Commands slice.

Append to bottom of file:

```go
func hudCmd(repo *storage.Repository[hud.LayoutRow]) *cli.Command {
	return &cli.Command{
		Name:  "hud",
		Usage: "CRUD operations on the hud_layout table",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List every hud_layout row",
				Action: func(ctx context.Context, c *cli.Command) error {
					rows, err := repo.List(ctx)
					if err != nil {
						return err
					}
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "KEY\tX\tY\tW\tH\tANCHOR\tSCALE")
					for _, r := range rows {
						fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%s\t%.2f\n",
							r.Key, r.X, r.Y, r.W, r.H, r.AnchorS, r.Scale)
					}
					return w.Flush()
				},
			},
			{
				Name:      "get",
				Usage:     "Show one hud_layout row",
				ArgsUsage: "<key>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 1 {
						return fmt.Errorf("usage: tune hud get <key>")
					}
					key := c.Args().Get(0)
					r, err := repo.Get(ctx, key)
					if err != nil {
						return fmt.Errorf("unknown hud layout key %q", key)
					}
					fmt.Printf("key=%s x=%d y=%d w=%d h=%d anchor=%s scale=%.2f\n",
						r.Key, r.X, r.Y, r.W, r.H, r.AnchorS, r.Scale)
					return nil
				},
			},
			{
				Name:        "set",
				Usage:       "Update one field of a hud_layout row",
				ArgsUsage:   "<key> <field> <value>",
				Description: "Valid fields: x, y, w, h, anchor, scale",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 3 {
						return fmt.Errorf("usage: tune hud set <key> <field> <value>")
					}
					key := c.Args().Get(0)
					field := c.Args().Get(1)
					raw := c.Args().Get(2)

					r, err := repo.Get(ctx, key)
					if err != nil {
						return fmt.Errorf("unknown hud layout key %q", key)
					}
					before := formatHUDRow(r)
					if err := applyHUDField(&r, field, raw); err != nil {
						return err
					}
					if err := repo.Upsert(ctx, r); err != nil {
						return err
					}
					fmt.Printf("OK: %s.%s updated\n  was: %s\n  now: %s\n", key, field, before, formatHUDRow(r))
					return nil
				},
			},
		},
	}
}

func formatHUDRow(r hud.LayoutRow) string {
	return fmt.Sprintf("x=%d y=%d w=%d h=%d anchor=%s scale=%.2f",
		r.X, r.Y, r.W, r.H, r.AnchorS, r.Scale)
}

func applyHUDField(r *hud.LayoutRow, field, raw string) error {
	asInt := func() (int, error) {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("value %q is not an integer", raw)
		}
		return n, nil
	}
	switch field {
	case "x":
		n, err := asInt()
		if err != nil {
			return err
		}
		r.X = n
	case "y":
		n, err := asInt()
		if err != nil {
			return err
		}
		r.Y = n
	case "w":
		n, err := asInt()
		if err != nil {
			return err
		}
		r.W = n
	case "h":
		n, err := asInt()
		if err != nil {
			return err
		}
		r.H = n
	case "anchor":
		valid := map[string]bool{"top_left": true, "top_right": true, "bottom_left": true, "bottom_right": true}
		if !valid[raw] {
			return fmt.Errorf("invalid anchor %q (valid: top_left, top_right, bottom_left, bottom_right)", raw)
		}
		r.AnchorS = raw
	case "scale":
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("value %q is not a number", raw)
		}
		if f <= 0 {
			return fmt.Errorf("scale must be > 0 (got %f)", f)
		}
		r.Scale = f
	default:
		return fmt.Errorf("unknown field %q (valid: x, y, w, h, anchor, scale)", field)
	}
	return nil
}

```

(Only add the `strings` import if you use it elsewhere in the file. Remove it if unused.)

- [ ] **Step 2: Smoke test CLI**

Run: `rm -rf data/ && go run ./cmd/tune hud list`
Expected:

```
KEY          X   Y   W   H   ANCHOR       SCALE
heart        48  16  16  16  top_right    2.00
lives_text   16  16  0   0   top_right    1.00
score_text   16  16  0   0   top_left     1.00
stamina_bar  16  48  48  16  top_left     2.00
```

Run: `go run ./cmd/tune hud set heart x 64`
Expected: `OK: heart.x updated`.

Run: `go run ./cmd/tune hud set heart anchor bogus`
Expected: error `invalid anchor "bogus"`.

- [ ] **Step 3: Commit**

```bash
git add cmd/tune/main.go
git commit -m "feat(tune): hud subcommand (list/get/set)" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: Wire Everything in `cmd/game/main.go`

**Files:**
- Modify: `cmd/game/main.go`

- [ ] **Step 1: Load stamina tuning, stamina anim, layout; pass into game.Deps**

Insert after existing tuneRepo + physics loads:

```go
staminaTuning, err := player.LoadStaminaTuning(tuneRepo)
if err != nil {
    log.Fatalf("load stamina tuning: %v", err)
}

hudLayoutRepo := storage.NewRepository[hud.LayoutRow](db, hud.LayoutMapper{})
layout, err := hud.LoadLayout(hudLayoutRepo)
if err != nil {
    log.Fatalf("load hud layout: %v", err)
}
```

After `heart` is extracted:

```go
staminaAnim, ok := anims["stamina_bar"]
if !ok {
    log.Fatalf("missing stamina_bar anim")
}
```

Update `game.New(game.Deps{...})` call:

```go
g := game.New(game.Deps{
    Cfg:           cfg,
    Anims:         anims,
    Physics:       physics,
    StaminaTuning: staminaTuning,
    DebugCfg:      dbgCfg,
    SoldierBoxes:  soldierBoxes,
    CombatTuning:  combatTuning,
    EnemyKinds:    enabledKinds,
    SpawnTuning:   spawnTuning,
    HeartAnim:     heart,
    StaminaAnim:   staminaAnim,
    HUDFace:       hud.NewFace(32),
    OverTitle:     hud.NewFace(96),
    OverSubtitle:  hud.NewFace(32),
    Layout:        layout,
})
```

- [ ] **Step 2: Build + launch**

Run: `rm -rf data/ && go build ./... && go test ./...`
Expected: builds cleanly; all unit tests pass.

Run: `make run` (manual launch — stop after confirming window draws without panic).
Expected: game window. Heart visible top-right. Score text top-left "Score: 0". Stamina bar top-left below score. Held Shift drains bar. Esc pauses.

- [ ] **Step 3: Commit**

```bash
git add cmd/game/main.go
git commit -m "feat(main): wire stamina pool + score + hud layout" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 13: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add new tuning keys to the tuning list, new controls, new CLI subcommand**

Edit tuning key table — add rows for `stamina_max`, `stamina_drain_per_s`, `stamina_regen_per_s`, `orc_points`, `slime_points` (bumps count from 25 to 30).

Add the hud CLI under the "Motions" section:

```markdown
### HUD layout (`hud_layout` table)

```bash
go run ./cmd/tune hud list
go run ./cmd/tune hud get <key>
go run ./cmd/tune hud set <key> <field> <value>   # fields: x, y, w, h, anchor, scale
```

Keys: `heart`, `lives_text`, `score_text`, `stamina_bar`. Anchors: `top_left`, `top_right`, `bottom_left`, `bottom_right`. x/y = offset of the element's nearest corner from the screen anchor corner.
```

Add controls:

| Action | Keys |
|---|---|
| Pause | `Esc` (edge) |
| Resume | Any key (edge, action swallowed that tick) |

Replace the state-machine "Soldier (8 states)" paragraph to note stamina gates sprint.

Replace migrations count. Add a sentence mentioning HUD layout table in the Layout section.

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: pause, stamina, score, hud layout CLI, new tuning keys" -m "Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 14: Manual Verification

No code changes. Run through each item; if any fails, open a fix task.

- [ ] **Stamina drain-to-empty**

1. `rm -rf data/ && make run`
2. Hold Shift + D for 5 seconds.
3. Observe stamina bar frames stepping from full (frame 0, col 2 top) through mid frames down to empty (frame 9, col 2 bottom).
4. After bar empties, soldier speed visibly drops to run_speed even with Shift still held.

- [ ] **Stamina regen**

1. Release Shift after emptying bar.
2. Bar fills top-to-... wait, it empties top→bottom visually (frame 0 = full). Confirm visual direction matches frame mapping.
3. After 5s of no sprint, bar back to full.

- [ ] **Score per kill**

1. Kill an orc → counter jumps by 10.
2. Kill a slime → counter jumps by 15.
3. Die (10 hits) → GAME OVER → press R → "Score: 0".

- [ ] **Heart asset migration**

1. Heart animation at top-right plays (4-frame beat loop at 400ms).
2. No "load huds/healthbar/heartbeat.png: ..." errors at boot.

- [ ] **Pause Esc**

1. Press Esc → dim overlay + "PAUSED" + "Press any key to resume".
2. Confirm no physics/anim/spawn updates (enemies freeze mid-air if paused during fall).
3. Press Space → resume; soldier does NOT jump.
4. Press Esc → paused again; press J → resume; soldier does NOT attack.
5. Press Esc while GAME OVER — nothing happens (pause ignored in game-over mode).

- [ ] **HUD layout tuning**

1. Quit. `go run ./cmd/tune hud set score_text x 100`.
2. Rerun game → score text moved right by 84px (16 → 100).
3. `go run ./cmd/tune hud set heart anchor bottom_right`.
4. Rerun → heart drawn bottom-right.
5. Restore defaults: `go run ./cmd/tune hud set heart anchor top_right`, `go run ./cmd/tune hud set score_text x 16`.

- [ ] **F3/F4 still work**

1. F3 → debug overlay toggles.
2. F4 → hitbox boxes draw.

- [ ] **All tests green**

Run: `go test ./...`
Expected: all PASS.

---

## Self-Review Notes

Done during plan authoring. Fixed inline:

- Heart scale moved from constructor argument into layout row (was hardcoded `3` originally; now `2.0` — user can retune via CLI).
- Text elements store `W=0`; HUD measures width at draw time and writes it back into the map entry just for `Resolve`, then restores. Stateful but local.
- Migration numbering aligned to existing sequence (last was 022 → new start at 023).
- `pick_col` default = -1 (sentinel for "row mode"), mutual-exclusion checked in loader.
- Player `IsSprinting(intent)` helper exposes the grounded+stamina+shift+move check used both to pick VX in `groundSpeed` and to drive stamina drain in `game.Update` — prevents divergence.
- Pause resume uses `inpututil.AppendJustPressedKeys(nil)` to detect "any key just pressed" without enumerating keycodes.
- `score.Counter.Add(n<=0)` is a no-op — defends against unseeded kinds (missing tuning → Points=0 → no crash, just no score).

## Open Items Passed Through From Spec

None. All design decisions are concrete above.
