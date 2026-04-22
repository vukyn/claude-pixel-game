# Char1 Controller Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a playable `char1` character demo using ebiten v2: 7 state machine states, 7 sprite animations driven from SQLite, tunable physics via a small CLI, gravity + flat ground, and a JSON-configured F3 debug overlay.

**Architecture:** A single Go module with two binaries (`cmd/game`, `cmd/tune`) sharing `internal/` packages. Config comes from `.env` (godotenv). Animation specs and physics tuning live in SQLite, accessed through a generic `Repository[T]`. A state machine with per-state structs drives the player; input is decoupled via an `Intent` struct. Debug overlay reads a JSON catalog of fields to display.

**Tech Stack:** Go 1.26 · `github.com/hajimehoshi/ebiten/v2` · `modernc.org/sqlite` (pure Go) · `github.com/joho/godotenv` · `github.com/urfave/cli/v3`

**Spec:** `docs/superpowers/specs/2026-04-21-char1-controller-design.md`

---

## File Structure

Files created in this plan:

```
.env                                      # committed runtime config
.env.example                              # template
.gitignore                                # exclude ./data, etc.
Makefile                                  # modified
config/debug.json                         # debug overlay layout

cmd/game/main.go                          # ebiten entry
cmd/tune/main.go                          # CLI entry

internal/config/config.go                 # godotenv loader
internal/config/config_test.go

internal/storage/sqlite.go                # Open + migration runner
internal/storage/repository.go            # generic Repository[T] + Mapper[T]
internal/storage/repository_test.go
internal/storage/migrations/001_init_animations.sql
internal/storage/migrations/002_seed_char1_animations.sql
internal/storage/migrations/003_init_tuning.sql
internal/storage/migrations/004_seed_tuning.sql
internal/storage/migrations.go            # embed FS + apply logic

internal/anim/spec.go                     # AnimationSpec + Mapper
internal/anim/animation.go                # runtime Animation
internal/anim/animation_test.go
internal/anim/sheet.go                    # Slice
internal/anim/library.go                  # LoadLibrary

internal/player/tuning.go                 # TuningParam + Mapper
internal/player/tuning_validator.go
internal/player/tuning_validator_test.go
internal/player/physics.go                # Physics + LoadPhysics
internal/player/player.go                 # Player struct + ApplyPhysics
internal/player/fsm.go                    # FSM + State interface
internal/player/fsm_test.go
internal/player/states.go                 # idleState, runState, jumpState, fallState, dashState, attackState, attack2State

internal/world/world.go

internal/input/input.go                   # Poll → Intent

internal/debug/fields.go                  # Catalog + FieldSource
internal/debug/config.go                  # LoadConfig
internal/debug/config_test.go
internal/debug/overlay.go                 # Overlay

internal/game/game.go                     # ebiten.Game impl
```

Module name: `claude-pixel` (already set in `go.mod`). All imports use `claude-pixel/internal/...`.

---

## Task 1: Bootstrap module, deps, env files, gitignore

**Files:**
- Modify: `go.mod`
- Create: `.env`, `.env.example`, `.gitignore`
- Modify: `Makefile`

- [ ] **Step 1: Add dependencies**

Run (each `go get` writes the dep into `go.mod`):
```bash
go get github.com/hajimehoshi/ebiten/v2@latest
go get github.com/joho/godotenv@latest
go get modernc.org/sqlite@latest
go get github.com/urfave/cli/v3@latest
```

Expected: `go.mod` now lists these four modules as `require` entries.

**Do NOT run `go mod tidy` in this task.** `tidy` removes any dep that has no importer, and the first real importer does not appear until Task 2 (`config/config.go` imports `godotenv`). Each subsequent task that adds an importer can safely run `go mod tidy` for its own package; we defer a repo-wide tidy until Task 18 (post-verification).

- [ ] **Step 2: Create `.env`**

Create `.env`:
```
DB_PATH=./data/game.db
ASSETS_DIR=./assets/sprites/char1
SPRITE_FRAME_W=120
SPRITE_FRAME_H=80
WINDOW_WIDTH=1280
WINDOW_HEIGHT=720
RENDER_SCALE=3
DEBUG_CONFIG_PATH=./config/debug.json
```

- [ ] **Step 3: Create `.env.example`** with identical content to `.env`.

- [ ] **Step 4: Create `.gitignore`**

```
# Build artifacts
/bin/
*.exe

# Runtime data (regenerable from migrations)
/data/

# Editor
.vscode/
.idea/
```

- [ ] **Step 5: Update `Makefile`**

Replace existing content with:
```makefile
.PHONY: run tune tidy test

run:
	go run ./cmd/game

tune:
	go run ./cmd/tune $(ARGS)

tidy:
	go mod tidy

test:
	go test ./...
```

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum .env .env.example .gitignore Makefile
git commit -m "chore: bootstrap deps, env template, gitignore, makefile"
```

---

## Task 2: Config layer

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

`internal/config/config_test.go`:
```go
package config

import (
	"os"
	"testing"
)

func TestLoadReadsEnvVars(t *testing.T) {
	os.Setenv("DB_PATH", "/tmp/x.db")
	os.Setenv("ASSETS_DIR", "/tmp/assets")
	os.Setenv("SPRITE_FRAME_W", "120")
	os.Setenv("SPRITE_FRAME_H", "80")
	os.Setenv("WINDOW_WIDTH", "1280")
	os.Setenv("WINDOW_HEIGHT", "720")
	os.Setenv("RENDER_SCALE", "3")
	os.Setenv("DEBUG_CONFIG_PATH", "/tmp/debug.json")
	t.Cleanup(func() {
		for _, k := range []string{"DB_PATH", "ASSETS_DIR", "SPRITE_FRAME_W", "SPRITE_FRAME_H", "WINDOW_WIDTH", "WINDOW_HEIGHT", "RENDER_SCALE", "DEBUG_CONFIG_PATH"} {
			os.Unsetenv(k)
		}
	})

	cfg := Load()
	if cfg.DBPath != "/tmp/x.db" || cfg.SpriteFrameW != 120 || cfg.RenderScale != 3 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadPanicsOnMissingKey(t *testing.T) {
	os.Unsetenv("DB_PATH")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when DB_PATH missing")
		}
	}()
	Load()
}
```

- [ ] **Step 2: Run — should fail to compile**

Run: `go test ./internal/config/...`
Expected: FAIL (`Load` undefined).

- [ ] **Step 3: Implement**

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
	SpriteFrameW    int
	SpriteFrameH    int
	WindowW         int
	WindowH         int
	RenderScale     int
	DebugConfigPath string
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		DBPath:          mustString("DB_PATH"),
		AssetsDir:       mustString("ASSETS_DIR"),
		SpriteFrameW:    mustInt("SPRITE_FRAME_W"),
		SpriteFrameH:    mustInt("SPRITE_FRAME_H"),
		WindowW:         mustInt("WINDOW_WIDTH"),
		WindowH:         mustInt("WINDOW_HEIGHT"),
		RenderScale:     mustInt("RENDER_SCALE"),
		DebugConfigPath: mustString("DEBUG_CONFIG_PATH"),
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

- [ ] **Step 4: Run — should pass**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): load required env vars via godotenv, panic on missing"
```

---

## Task 3: Storage — SQLite open + migration runner

**Files:**
- Create: `internal/storage/sqlite.go`
- Create: `internal/storage/migrations.go`
- Create: `internal/storage/migrations/001_init_animations.sql`
- Create: `internal/storage/migrations/002_seed_char1_animations.sql`
- Create: `internal/storage/migrations/003_init_tuning.sql`
- Create: `internal/storage/migrations/004_seed_tuning.sql`

- [ ] **Step 1: Create migration files**

`internal/storage/migrations/001_init_animations.sql`:
```sql
CREATE TABLE animations (
    id           TEXT    PRIMARY KEY,
    file         TEXT    NOT NULL,
    frame_count  INTEGER NOT NULL,
    duration_ms  INTEGER NOT NULL,
    loop         INTEGER NOT NULL
);
```

`internal/storage/migrations/002_seed_char1_animations.sql`:
```sql
INSERT OR IGNORE INTO animations (id, file, frame_count, duration_ms, loop) VALUES
    ('idle',    '_Idle.png',    10, 1000, 1),
    ('run',     '_Run.png',     10, 1000, 1),
    ('jump',    '_Jump.png',     3,  500, 0),
    ('fall',    '_Fall.png',     3,  500, 0),
    ('dash',    '_Dash.png',     2,  500, 0),
    ('attack',  '_Attack.png',   4, 1500, 0),
    ('attack2', '_Attack2.png',  6, 1500, 0);
```

`internal/storage/migrations/003_init_tuning.sql`:
```sql
CREATE TABLE tuning (
    key         TEXT    PRIMARY KEY,
    value       REAL    NOT NULL,
    min_value   REAL    NOT NULL,
    max_value   REAL    NOT NULL,
    unit        TEXT    NOT NULL DEFAULT '',
    description TEXT    NOT NULL
);
```

`internal/storage/migrations/004_seed_tuning.sql`:
```sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('run_speed',        280,     50,   1000, 'px/s',   'Horizontal ground movement speed'),
    ('air_control',      0.8,      0,      1, '',       'Horizontal movement multiplier while airborne'),
    ('jump_velocity',   -650,  -2000,   -100, 'px/s',   'Jump impulse applied on takeoff (negative = upward)'),
    ('gravity',         2000,    100,   5000, 'px/s^2', 'Downward acceleration applied each tick'),
    ('max_fall_speed',   900,    100,   3000, 'px/s',   'Terminal vertical velocity clamp'),
    ('dash_speed',       700,    100,   2000, 'px/s',   'Horizontal velocity during dash'),
    ('dash_duration_ms', 500,     50,   2000, 'ms',     'Dash duration');
```

- [ ] **Step 2: Implement migration runner**

`internal/storage/migrations.go`:
```go
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func applyMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version    TEXT PRIMARY KEY,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists int
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check %s: %w", name, err)
		}
		if exists > 0 {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 3: Implement Open / MustOpen**

`internal/storage/sqlite.go`:
```go
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"claude-pixel/internal/config"
)

func Open(cfg *config.Config) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	if err := applyMigrations(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func MustOpen(cfg *config.Config) *sql.DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}
```

- [ ] **Step 4: Smoke-test with a tiny program**

Run:
```bash
go build ./...
```
Expected: compiles.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/
git commit -m "feat(storage): sqlite open + embedded migration runner with schema_migrations tracking"
```

---

## Task 4: Generic Repository[T] + Mapper[T]

**Files:**
- Create: `internal/storage/repository.go`
- Create: `internal/storage/repository_test.go`

- [ ] **Step 1: Write failing test**

`internal/storage/repository_test.go`:
```go
package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

type toyEntity struct {
	ID   string
	Name string
	Qty  int
}

func (t toyEntity) GetID() string { return t.ID }

type toyMapper struct{}

func (toyMapper) Table() string     { return "toys" }
func (toyMapper) Columns() []string { return []string{"id", "name", "qty"} }
func (toyMapper) Scan(row Scanner) (toyEntity, error) {
	var t toyEntity
	return t, row.Scan(&t.ID, &t.Name, &t.Qty)
}
func (toyMapper) Values(t toyEntity) []any { return []any{t.ID, t.Name, t.Qty} }

func openMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE toys (id TEXT PRIMARY KEY, name TEXT NOT NULL, qty INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	db := openMemDB(t)
	repo := NewRepository[toyEntity](db, toyMapper{})

	if err := repo.Upsert(ctx, toyEntity{ID: "a", Name: "Apple", Qty: 3}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "a")
	if err != nil || got.Name != "Apple" || got.Qty != 3 {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	if err := repo.Upsert(ctx, toyEntity{ID: "a", Name: "Apple", Qty: 9}); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(ctx, "a")
	if got.Qty != 9 {
		t.Fatalf("upsert did not update: %+v", got)
	}

	if err := repo.Upsert(ctx, toyEntity{ID: "b", Name: "Banana", Qty: 1}); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list=%v err=%v", list, err)
	}

	if err := repo.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, "a"); err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}
```

- [ ] **Step 2: Run — FAIL (types undefined)**

Run: `go test ./internal/storage/...`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/storage/repository.go`:
```go
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Entity interface {
	GetID() string
}

type Scanner interface {
	Scan(dest ...any) error
}

type Mapper[T Entity] interface {
	Table() string
	Columns() []string
	Scan(row Scanner) (T, error)
	Values(t T) []any
}

type Repository[T Entity] struct {
	db     *sql.DB
	mapper Mapper[T]
}

func NewRepository[T Entity](db *sql.DB, m Mapper[T]) *Repository[T] {
	return &Repository[T]{db: db, mapper: m}
}

func (r *Repository[T]) Get(ctx context.Context, id string) (T, error) {
	var zero T
	cols := r.mapper.Columns()
	q := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", strings.Join(cols, ", "), r.mapper.Table(), cols[0])
	row := r.db.QueryRowContext(ctx, q, id)
	t, err := r.mapper.Scan(row)
	if err != nil {
		return zero, err
	}
	return t, nil
}

func (r *Repository[T]) List(ctx context.Context) ([]T, error) {
	cols := r.mapper.Columns()
	q := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), r.mapper.Table())
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		t, err := r.mapper.Scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository[T]) Upsert(ctx context.Context, t T) error {
	cols := r.mapper.Columns()
	placeholders := strings.Repeat("?,", len(cols))
	placeholders = placeholders[:len(placeholders)-1]

	updateParts := make([]string, 0, len(cols)-1)
	for _, c := range cols[1:] {
		updateParts = append(updateParts, fmt.Sprintf("%s=excluded.%s", c, c))
	}

	q := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(%s) DO UPDATE SET %s",
		r.mapper.Table(),
		strings.Join(cols, ", "),
		placeholders,
		cols[0],
		strings.Join(updateParts, ", "),
	)
	_, err := r.db.ExecContext(ctx, q, r.mapper.Values(t)...)
	return err
}

func (r *Repository[T]) Delete(ctx context.Context, id string) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", r.mapper.Table(), r.mapper.Columns()[0])
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}
```

- [ ] **Step 4: Run — PASS**

Run: `go test ./internal/storage/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/repository.go internal/storage/repository_test.go
git commit -m "feat(storage): generic Repository[T] with Mapper[T] for reusable CRUD"
```

---

## Task 5: Animation runtime (pure math, TDD)

**Files:**
- Create: `internal/anim/animation.go`
- Create: `internal/anim/animation_test.go`

- [ ] **Step 1: Write failing test**

`internal/anim/animation_test.go`:
```go
package anim

import (
	"testing"
	"time"
)

func makeAnim(loop bool, count int, dur time.Duration) *Animation {
	return &Animation{
		spec: &AnimationSpec{ID: "x", FrameCount: count, DurationMs: int(dur / time.Millisecond), Loop: loop},
	}
}

func TestFrameIndexLoop(t *testing.T) {
	a := makeAnim(true, 10, 1000*time.Millisecond) // 100ms per frame
	cases := []struct {
		elapsed time.Duration
		want    int
	}{
		{0, 0},
		{99 * time.Millisecond, 0},
		{100 * time.Millisecond, 1},
		{950 * time.Millisecond, 9},
		{1000 * time.Millisecond, 0}, // wrap
		{1500 * time.Millisecond, 5},
	}
	for _, c := range cases {
		a.elapsed = c.elapsed
		if got := a.FrameIndex(); got != c.want {
			t.Errorf("elapsed %v: got %d want %d", c.elapsed, got, c.want)
		}
	}
}

func TestFrameIndexNonLoopClamps(t *testing.T) {
	a := makeAnim(false, 3, 300*time.Millisecond) // 100ms per frame
	a.elapsed = 299 * time.Millisecond
	if got := a.FrameIndex(); got != 2 {
		t.Errorf("clamp at last: got %d want 2", got)
	}
	a.elapsed = 5 * time.Second
	if got := a.FrameIndex(); got != 2 {
		t.Errorf("clamp beyond: got %d want 2", got)
	}
	if !a.Done() {
		t.Errorf("expected Done=true")
	}
}

func TestLoopNeverDone(t *testing.T) {
	a := makeAnim(true, 5, 500*time.Millisecond)
	a.elapsed = 10 * time.Second
	if a.Done() {
		t.Errorf("loop should never be Done")
	}
}

func TestUpdateAdvancesElapsed(t *testing.T) {
	a := makeAnim(true, 10, 1000*time.Millisecond)
	a.Update(50 * time.Millisecond)
	a.Update(50 * time.Millisecond)
	if a.elapsed != 100*time.Millisecond {
		t.Errorf("elapsed=%v", a.elapsed)
	}
}

func TestResetZeroesElapsed(t *testing.T) {
	a := makeAnim(true, 10, 1000*time.Millisecond)
	a.elapsed = 500 * time.Millisecond
	a.Reset()
	if a.elapsed != 0 {
		t.Errorf("elapsed=%v", a.elapsed)
	}
}
```

- [ ] **Step 2: Run — FAIL**

Run: `go test ./internal/anim/...`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement spec + animation (no frames yet — frames wired in Task 7)**

`internal/anim/animation.go`:
```go
package anim

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type AnimationSpec struct {
	ID         string
	File       string
	FrameCount int
	DurationMs int
	Loop       bool
}

func (a AnimationSpec) GetID() string { return a.ID }

type Animation struct {
	spec    *AnimationSpec
	frames  []*ebiten.Image
	elapsed time.Duration
}

func NewAnimation(spec *AnimationSpec, frames []*ebiten.Image) *Animation {
	return &Animation{spec: spec, frames: frames}
}

func (a *Animation) Update(dt time.Duration) { a.elapsed += dt }

func (a *Animation) Reset() { a.elapsed = 0 }

func (a *Animation) Elapsed() time.Duration { return a.elapsed }

func (a *Animation) SpecID() string { return a.spec.ID }

func (a *Animation) FrameIndex() int {
	if a.spec.FrameCount <= 0 {
		return 0
	}
	totalMs := a.elapsed.Milliseconds()
	perFrameMs := int64(a.spec.DurationMs) / int64(a.spec.FrameCount)
	if perFrameMs <= 0 {
		return 0
	}
	idx := int(totalMs / perFrameMs)
	if a.spec.Loop {
		return idx % a.spec.FrameCount
	}
	if idx >= a.spec.FrameCount {
		return a.spec.FrameCount - 1
	}
	return idx
}

func (a *Animation) Done() bool {
	if a.spec.Loop {
		return false
	}
	return a.elapsed.Milliseconds() >= int64(a.spec.DurationMs)
}

func (a *Animation) CurrentFrame() *ebiten.Image {
	if len(a.frames) == 0 {
		return nil
	}
	return a.frames[a.FrameIndex()]
}
```

- [ ] **Step 4: Run — PASS**

Run: `go test ./internal/anim/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anim/
git commit -m "feat(anim): Animation runtime with loop/non-loop frame indexing, tested"
```

---

## Task 6: Animation Mapper + sheet slicer + library loader

**Files:**
- Create: `internal/anim/spec.go` (Mapper)
- Create: `internal/anim/sheet.go`
- Create: `internal/anim/library.go`

- [ ] **Step 1: Implement Mapper**

`internal/anim/spec.go`:
```go
package anim

import "claude-pixel/internal/storage"

type SpecMapper struct{}

func (SpecMapper) Table() string     { return "animations" }
func (SpecMapper) Columns() []string { return []string{"id", "file", "frame_count", "duration_ms", "loop"} }
func (SpecMapper) Scan(row storage.Scanner) (AnimationSpec, error) {
	var s AnimationSpec
	var loopInt int
	err := row.Scan(&s.ID, &s.File, &s.FrameCount, &s.DurationMs, &loopInt)
	s.Loop = loopInt != 0
	return s, err
}
func (SpecMapper) Values(s AnimationSpec) []any {
	loop := 0
	if s.Loop {
		loop = 1
	}
	return []any{s.ID, s.File, s.FrameCount, s.DurationMs, loop}
}
```

- [ ] **Step 2: Implement sheet slicer**

`internal/anim/sheet.go`:
```go
package anim

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

func Slice(img *ebiten.Image, frameW, frameH, count int) []*ebiten.Image {
	frames := make([]*ebiten.Image, count)
	for i := 0; i < count; i++ {
		r := image.Rect(i*frameW, 0, (i+1)*frameW, frameH)
		frames[i] = img.SubImage(r).(*ebiten.Image)
	}
	return frames
}
```

- [ ] **Step 3: Implement LoadLibrary**

`internal/anim/library.go`:
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
		path := filepath.Join(cfg.AssetsDir, spec.File)
		img, _, err := ebitenutil.NewImageFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		wantW := cfg.SpriteFrameW * spec.FrameCount
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		if w != wantW || h != cfg.SpriteFrameH {
			return nil, fmt.Errorf("sheet %s: got %dx%d, want %dx%d", spec.File, w, h, wantW, cfg.SpriteFrameH)
		}
		frames := Slice(img, cfg.SpriteFrameW, cfg.SpriteFrameH, spec.FrameCount)
		out[spec.ID] = NewAnimation(&spec, frames)
	}
	return out, nil
}

// Verify *ebiten.Image variable is referenced so import isn't stripped.
var _ = (*ebiten.Image)(nil)
```

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: compiles.

- [ ] **Step 5: Commit**

```bash
git add internal/anim/spec.go internal/anim/sheet.go internal/anim/library.go
git commit -m "feat(anim): Mapper, Slice, LoadLibrary (DB → PNG → frames)"
```

---

## Task 7: Tuning — struct, mapper, validator

**Files:**
- Create: `internal/player/tuning.go`
- Create: `internal/player/tuning_validator.go`
- Create: `internal/player/tuning_validator_test.go`

- [ ] **Step 1: Implement TuningParam + Mapper**

`internal/player/tuning.go`:
```go
package player

import "claude-pixel/internal/storage"

type TuningParam struct {
	Key         string
	Value       float64
	MinValue    float64
	MaxValue    float64
	Unit        string
	Description string
}

func (t TuningParam) GetID() string { return t.Key }

type TuningMapper struct{}

func (TuningMapper) Table() string     { return "tuning" }
func (TuningMapper) Columns() []string { return []string{"key", "value", "min_value", "max_value", "unit", "description"} }
func (TuningMapper) Scan(row storage.Scanner) (TuningParam, error) {
	var p TuningParam
	err := row.Scan(&p.Key, &p.Value, &p.MinValue, &p.MaxValue, &p.Unit, &p.Description)
	return p, err
}
func (TuningMapper) Values(p TuningParam) []any {
	return []any{p.Key, p.Value, p.MinValue, p.MaxValue, p.Unit, p.Description}
}
```

- [ ] **Step 2: Write failing validator test**

`internal/player/tuning_validator_test.go`:
```go
package player

import "testing"

func TestValidateTuningInRange(t *testing.T) {
	p := TuningParam{Key: "x", MinValue: 0, MaxValue: 10, Unit: "px"}
	if err := ValidateTuning(p, 5); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTuningBelowMin(t *testing.T) {
	p := TuningParam{Key: "x", MinValue: 0, MaxValue: 10}
	if err := ValidateTuning(p, -1); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateTuningAboveMax(t *testing.T) {
	p := TuningParam{Key: "x", MinValue: 0, MaxValue: 10}
	if err := ValidateTuning(p, 11); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3: Run — FAIL** (`go test ./internal/player/...`).

- [ ] **Step 4: Implement validator**

`internal/player/tuning_validator.go`:
```go
package player

import "fmt"

func ValidateTuning(p TuningParam, newValue float64) error {
	if newValue < p.MinValue || newValue > p.MaxValue {
		return fmt.Errorf("value out of range: %v not in [%v, %v] %s", newValue, p.MinValue, p.MaxValue, p.Unit)
	}
	return nil
}
```

- [ ] **Step 5: Run — PASS**.

- [ ] **Step 6: Commit**

```bash
git add internal/player/tuning.go internal/player/tuning_validator.go internal/player/tuning_validator_test.go
git commit -m "feat(player): TuningParam, Mapper, and validator"
```

---

## Task 8: Physics loader

**Files:**
- Create: `internal/player/physics.go`

- [ ] **Step 1: Implement**

`internal/player/physics.go`:
```go
package player

import (
	"context"
	"fmt"
	"time"

	"claude-pixel/internal/storage"
)

type Physics struct {
	RunSpeed     float64
	AirControl   float64
	JumpVelocity float64
	Gravity      float64
	MaxFallSpeed float64
	DashSpeed    float64
	DashDuration time.Duration
}

func LoadPhysics(repo *storage.Repository[TuningParam]) (*Physics, error) {
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
	ph := &Physics{}
	var e error
	if ph.RunSpeed, e = pick("run_speed"); e != nil { return nil, e }
	if ph.AirControl, e = pick("air_control"); e != nil { return nil, e }
	if ph.JumpVelocity, e = pick("jump_velocity"); e != nil { return nil, e }
	if ph.Gravity, e = pick("gravity"); e != nil { return nil, e }
	if ph.MaxFallSpeed, e = pick("max_fall_speed"); e != nil { return nil, e }
	if ph.DashSpeed, e = pick("dash_speed"); e != nil { return nil, e }
	dd, e := pick("dash_duration_ms"); if e != nil { return nil, e }
	ph.DashDuration = time.Duration(dd) * time.Millisecond
	return ph, nil
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: compiles.

- [ ] **Step 3: Commit**

```bash
git add internal/player/physics.go
git commit -m "feat(player): LoadPhysics reads all tuning keys or errors"
```

---

## Task 9: World

**Files:**
- Create: `internal/world/world.go`

- [ ] **Step 1: Implement**

`internal/world/world.go`:
```go
package world

import "claude-pixel/internal/config"

type World struct {
	Gravity float64
	GroundY float64
}

func New(cfg *config.Config, gravity float64) *World {
	return &World{
		Gravity: gravity,
		GroundY: float64(cfg.WindowH) - 120,
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/world/
git commit -m "feat(world): flat ground at cfg.WindowH - 120"
```

---

## Task 10: Input → Intent

**Files:**
- Create: `internal/input/input.go`

- [ ] **Step 1: Implement**

`internal/input/input.go`:
```go
package input

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Intent struct {
	Left, Right    bool
	JumpPressed    bool
	DashPressed    bool
	AttackPressed  bool
	Attack2Pressed bool
}

func Poll() Intent {
	return Intent{
		Left:           ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft),
		Right:          ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight),
		JumpPressed:    inpututil.IsKeyJustPressed(ebiten.KeySpace),
		DashPressed:    inpututil.IsKeyJustPressed(ebiten.KeyShiftLeft) || inpututil.IsKeyJustPressed(ebiten.KeyShiftRight),
		AttackPressed:  inpututil.IsKeyJustPressed(ebiten.KeyJ) || inpututil.IsKeyJustPressed(ebiten.KeyX),
		Attack2Pressed: inpututil.IsKeyJustPressed(ebiten.KeyK) || inpututil.IsKeyJustPressed(ebiten.KeyC),
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/input/
git commit -m "feat(input): Poll() → Intent for A/D/arrows, Space/Shift/J-X/K-C"
```

---

## Task 11: Player struct + physics step

**Files:**
- Create: `internal/player/player.go`

- [ ] **Step 1: Implement**

`internal/player/player.go`:
```go
package player

import (
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/world"
)

type Player struct {
	X, Y       float64
	VX, VY     float64
	Facing     int
	Grounded   bool
	HasAirDash bool
	DashTimer  time.Duration
	FSM        *FSM
	Physics    *Physics
	Anims      map[string]*anim.Animation
	Current    *anim.Animation
}

type Config struct {
	StartX, StartY float64
	Physics        *Physics
	Anims          map[string]*anim.Animation
}

func (p *Player) PlayAnim(id string) {
	a, ok := p.Anims[id]
	if !ok {
		return
	}
	a.Reset()
	p.Current = a
}

func (p *Player) ApplyPhysics(w *world.World, dt time.Duration) {
	dtS := dt.Seconds()

	if p.FSM != nil && p.FSM.CurrentID() == StateDash {
		p.X += float64(p.Facing) * p.Physics.DashSpeed * dtS
	} else {
		p.VY += p.Physics.Gravity * dtS
		if p.VY > p.Physics.MaxFallSpeed {
			p.VY = p.Physics.MaxFallSpeed
		}
		p.X += p.VX * dtS
		p.Y += p.VY * dtS
	}

	if p.Y >= w.GroundY {
		p.Y = w.GroundY
		p.VY = 0
		p.Grounded = true
		p.HasAirDash = true
	} else {
		p.Grounded = false
	}
}
```

Note: `New()` (constructor that wires FSM + states) is added in Task 13 once all state types exist.

- [ ] **Step 2: Build**

Run: `go build ./internal/player/...`
Expected: compiles (no `New` yet — it is added in Task 13).

- [ ] **Step 3: Commit**

```bash
git add internal/player/player.go
git commit -m "feat(player): Player struct + ApplyPhysics (gravity, ground clamp, dash override)"
```

---

## Task 12: FSM interface and empty shell

**Files:**
- Create: `internal/player/fsm.go`
- Create: `internal/player/fsm_test.go`

- [ ] **Step 1: Write failing FSM test (against shell + a toy state)**

`internal/player/fsm_test.go`:
```go
package player

import (
	"testing"
	"time"

	"claude-pixel/internal/input"
)

type countingState struct {
	id       StateID
	next     StateID
	enters   int
	exits    int
	updates  int
}

func (c *countingState) ID() StateID                                                   { return c.id }
func (c *countingState) Enter(p *Player)                                               { c.enters++ }
func (c *countingState) Exit(p *Player)                                                { c.exits++ }
func (c *countingState) Update(p *Player, in input.Intent, dt time.Duration) StateID  { c.updates++; return c.next }

func TestFSMTransitions(t *testing.T) {
	a := &countingState{id: "A", next: "A"}
	b := &countingState{id: "B", next: "B"}

	fsm := NewFSM("A")
	fsm.Register(a)
	fsm.Register(b)
	fsm.Start(&Player{})

	if a.enters != 1 {
		t.Fatal("initial Enter not called")
	}

	fsm.Handle(&Player{}, input.Intent{}, time.Millisecond)
	if a.updates != 1 || fsm.CurrentID() != "A" {
		t.Fatal("no-op transition failed")
	}

	a.next = "B"
	fsm.Handle(&Player{}, input.Intent{}, time.Millisecond)
	if a.exits != 1 || b.enters != 1 || fsm.CurrentID() != "B" {
		t.Fatalf("transition A->B failed, exits=%d enters(b)=%d current=%s", a.exits, b.enters, fsm.CurrentID())
	}
}
```

- [ ] **Step 2: Run — FAIL** (undefined).

- [ ] **Step 3: Implement FSM**

`internal/player/fsm.go`:
```go
package player

import (
	"time"

	"claude-pixel/internal/input"
)

type StateID string

const (
	StateIdle    StateID = "idle"
	StateRun     StateID = "run"
	StateJump    StateID = "jump"
	StateFall    StateID = "fall"
	StateDash    StateID = "dash"
	StateAttack  StateID = "attack"
	StateAttack2 StateID = "attack2"
)

type State interface {
	ID() StateID
	Enter(p *Player)
	Update(p *Player, in input.Intent, dt time.Duration) StateID
	Exit(p *Player)
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

func (f *FSM) Start(p *Player) {
	f.current = f.states[f.initialID]
	if f.current != nil {
		f.current.Enter(p)
	}
}

func (f *FSM) CurrentID() StateID {
	if f.current == nil {
		return ""
	}
	return f.current.ID()
}

func (f *FSM) Handle(p *Player, in input.Intent, dt time.Duration) {
	if f.current == nil {
		return
	}
	next := f.current.Update(p, in, dt)
	if next != f.current.ID() {
		f.current.Exit(p)
		ns, ok := f.states[next]
		if !ok {
			return
		}
		f.current = ns
		f.current.Enter(p)
	}
}
```

- [ ] **Step 4: Run — PASS**

Run: `go test ./internal/player/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/player/fsm.go internal/player/fsm_test.go
git commit -m "feat(player): FSM with typed StateID + Register/Start/Handle"
```

---

## Task 13: States implementation (all 7) + `player.New`

**Files:**
- Create: `internal/player/states.go`
- Modify: `internal/player/player.go` (add `New`)
- Modify: `internal/player/fsm_test.go` (add transition tests for real states)

- [ ] **Step 1: Add `New` constructor to `player.go`**

Append to `internal/player/player.go`:
```go
func New(cfg Config) *Player {
	p := &Player{
		X:       cfg.StartX,
		Y:       cfg.StartY,
		Facing:  1,
		Physics: cfg.Physics,
		Anims:   cfg.Anims,
	}
	p.FSM = NewFSM(StateIdle)
	p.FSM.Register(&idleState{})
	p.FSM.Register(&runState{})
	p.FSM.Register(&jumpState{})
	p.FSM.Register(&fallState{})
	p.FSM.Register(&dashState{})
	p.FSM.Register(&attackState{})
	p.FSM.Register(&attack2State{})
	p.FSM.Start(p)
	return p
}
```

- [ ] **Step 2: Implement all states**

`internal/player/states.go`:
```go
package player

import (
	"time"

	"claude-pixel/internal/input"
)

func moveDir(in input.Intent) int {
	switch {
	case in.Right && !in.Left:
		return 1
	case in.Left && !in.Right:
		return -1
	default:
		return 0
	}
}

// --- Idle ---
type idleState struct{}

func (idleState) ID() StateID      { return StateIdle }
func (idleState) Enter(p *Player)  { p.PlayAnim("idle"); p.VX = 0 }
func (idleState) Exit(p *Player)   {}
func (idleState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if !p.Grounded {
		return StateFall
	}
	if in.DashPressed {
		return StateDash
	}
	if in.AttackPressed {
		return StateAttack
	}
	if in.Attack2Pressed {
		return StateAttack2
	}
	if in.JumpPressed {
		return StateJump
	}
	if moveDir(in) != 0 {
		return StateRun
	}
	return StateIdle
}

// --- Run ---
type runState struct{}

func (runState) ID() StateID     { return StateRun }
func (runState) Enter(p *Player) { p.PlayAnim("run") }
func (runState) Exit(p *Player)  {}
func (runState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * p.Physics.RunSpeed

	if !p.Grounded {
		return StateFall
	}
	if in.DashPressed {
		return StateDash
	}
	if in.AttackPressed {
		return StateAttack
	}
	if in.Attack2Pressed {
		return StateAttack2
	}
	if in.JumpPressed {
		return StateJump
	}
	if d == 0 {
		return StateIdle
	}
	return StateRun
}

// --- Jump ---
type jumpState struct{}

func (jumpState) ID() StateID     { return StateJump }
func (jumpState) Enter(p *Player) { p.PlayAnim("jump"); p.VY = p.Physics.JumpVelocity }
func (jumpState) Exit(p *Player)  {}
func (jumpState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * p.Physics.RunSpeed * p.Physics.AirControl

	if in.DashPressed && p.HasAirDash {
		return StateDash
	}
	if in.AttackPressed {
		return StateAttack
	}
	if in.Attack2Pressed {
		return StateAttack2
	}
	if p.VY >= 0 {
		return StateFall
	}
	return StateJump
}

// --- Fall ---
type fallState struct{}

func (fallState) ID() StateID     { return StateFall }
func (fallState) Enter(p *Player) { p.PlayAnim("fall") }
func (fallState) Exit(p *Player)  {}
func (fallState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	d := moveDir(in)
	if d != 0 {
		p.Facing = d
	}
	p.VX = float64(d) * p.Physics.RunSpeed * p.Physics.AirControl

	if in.DashPressed && p.HasAirDash {
		return StateDash
	}
	if in.AttackPressed {
		return StateAttack
	}
	if in.Attack2Pressed {
		return StateAttack2
	}
	if p.Grounded {
		if d == 0 {
			return StateIdle
		}
		return StateRun
	}
	return StateFall
}

// --- Dash ---
type dashState struct{}

func (dashState) ID() StateID { return StateDash }
func (dashState) Enter(p *Player) {
	p.PlayAnim("dash")
	p.DashTimer = 0
	p.VX = 0 // dash overrides via ApplyPhysics using Facing * DashSpeed
	p.VY = 0
	if !p.Grounded {
		p.HasAirDash = false
	}
}
func (dashState) Exit(p *Player) {}
func (dashState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	p.DashTimer += dt
	if p.DashTimer >= p.Physics.DashDuration {
		if p.Grounded {
			return StateIdle
		}
		return StateFall
	}
	return StateDash
}

// --- Attack ---
type attackState struct{}

func (attackState) ID() StateID     { return StateAttack }
func (attackState) Enter(p *Player) { p.PlayAnim("attack"); if p.Grounded { p.VX = 0 } }
func (attackState) Exit(p *Player)  {}
func (attackState) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if in.DashPressed {
		return StateDash
	}
	if in.JumpPressed && p.Grounded {
		return StateJump
	}
	if p.Current.Done() {
		if p.Grounded {
			if moveDir(in) == 0 {
				return StateIdle
			}
			return StateRun
		}
		return StateFall
	}
	return StateAttack
}

// --- Attack2 ---
type attack2State struct{}

func (attack2State) ID() StateID     { return StateAttack2 }
func (attack2State) Enter(p *Player) { p.PlayAnim("attack2"); if p.Grounded { p.VX = 0 } }
func (attack2State) Exit(p *Player)  {}
func (attack2State) Update(p *Player, in input.Intent, dt time.Duration) StateID {
	if in.DashPressed {
		return StateDash
	}
	if in.JumpPressed && p.Grounded {
		return StateJump
	}
	if p.Current.Done() {
		if p.Grounded {
			if moveDir(in) == 0 {
				return StateIdle
			}
			return StateRun
		}
		return StateFall
	}
	return StateAttack2
}
```

- [ ] **Step 3: Add transition tests**

Append to `internal/player/fsm_test.go`:
```go
import "claude-pixel/internal/anim"

func newTestPlayer(t *testing.T) *Player {
	t.Helper()
	specs := map[string]*anim.AnimationSpec{
		"idle":    {ID: "idle", FrameCount: 1, DurationMs: 100, Loop: true},
		"run":     {ID: "run", FrameCount: 1, DurationMs: 100, Loop: true},
		"jump":    {ID: "jump", FrameCount: 1, DurationMs: 100, Loop: false},
		"fall":    {ID: "fall", FrameCount: 1, DurationMs: 100, Loop: false},
		"dash":    {ID: "dash", FrameCount: 1, DurationMs: 100, Loop: false},
		"attack":  {ID: "attack", FrameCount: 1, DurationMs: 100, Loop: false},
		"attack2": {ID: "attack2", FrameCount: 1, DurationMs: 100, Loop: false},
	}
	anims := map[string]*anim.Animation{}
	for k, s := range specs {
		anims[k] = anim.NewAnimation(s, nil)
	}
	return New(Config{
		StartX: 0, StartY: 0,
		Physics: &Physics{RunSpeed: 100, AirControl: 1, JumpVelocity: -100, Gravity: 500, MaxFallSpeed: 500, DashSpeed: 200, DashDuration: 50 * time.Millisecond},
		Anims:   anims,
	})
}

func TestIdleToRunToJumpToFallToIdle(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = true

	p.FSM.Handle(p, input.Intent{Right: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateRun {
		t.Fatalf("expected Run, got %s", p.FSM.CurrentID())
	}

	p.FSM.Handle(p, input.Intent{Right: true, JumpPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateJump {
		t.Fatalf("expected Jump, got %s", p.FSM.CurrentID())
	}
	if p.VY != -100 {
		t.Fatalf("jump impulse not applied, VY=%v", p.VY)
	}

	p.VY = 50
	p.FSM.Handle(p, input.Intent{Right: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateFall {
		t.Fatalf("expected Fall, got %s", p.FSM.CurrentID())
	}

	p.Grounded = true
	p.FSM.Handle(p, input.Intent{}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateIdle {
		t.Fatalf("expected Idle, got %s", p.FSM.CurrentID())
	}
}

func TestAttackCancelByDash(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = true
	p.FSM.Handle(p, input.Intent{AttackPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateAttack {
		t.Fatalf("expected Attack, got %s", p.FSM.CurrentID())
	}
	p.FSM.Handle(p, input.Intent{DashPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateDash {
		t.Fatalf("dash cancel failed: %s", p.FSM.CurrentID())
	}
}

func TestAirDashConsumedOnce(t *testing.T) {
	p := newTestPlayer(t)
	p.Grounded = false
	p.HasAirDash = true
	p.FSM.Handle(p, input.Intent{}, 16*time.Millisecond) // should land at Fall
	if p.FSM.CurrentID() != StateFall {
		t.Fatalf("expected Fall, got %s", p.FSM.CurrentID())
	}
	p.FSM.Handle(p, input.Intent{DashPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() != StateDash {
		t.Fatalf("air-dash not triggered: %s", p.FSM.CurrentID())
	}
	// Finish dash, still airborne → Fall, HasAirDash now false
	for i := 0; i < 10; i++ {
		p.FSM.Handle(p, input.Intent{}, 10*time.Millisecond)
	}
	if p.HasAirDash {
		t.Fatal("HasAirDash should be false after air-dash")
	}
	p.FSM.Handle(p, input.Intent{DashPressed: true}, 16*time.Millisecond)
	if p.FSM.CurrentID() == StateDash {
		t.Fatal("second air-dash should be refused")
	}
}
```

- [ ] **Step 4: Run — PASS**

Run: `go test ./internal/player/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/player/player.go internal/player/states.go internal/player/fsm_test.go
git commit -m "feat(player): 7 states + New constructor + transition tests"
```

---

## Task 14: Debug — field catalog + JSON config loader

**Files:**
- Create: `internal/debug/fields.go`
- Create: `internal/debug/config.go`
- Create: `internal/debug/config_test.go`
- Create: `config/debug.json`

- [ ] **Step 1: Create default debug.json**

`config/debug.json`:
```json
{
  "sections": [
    { "title": "State",      "fields": ["state", "facing", "grounded", "has_air_dash"] },
    { "title": "Kinematics", "fields": ["x", "y", "vx", "vy"] },
    { "title": "Animation",  "fields": ["anim_id", "anim_frame", "anim_elapsed_ms"] },
    { "title": "Intent",     "fields": ["intent_left", "intent_right", "intent_jump", "intent_dash", "intent_attack", "intent_attack2"] },
    { "title": "Engine",     "fields": ["fps", "tps"] }
  ]
}
```

- [ ] **Step 2: Implement fields catalog**

`internal/debug/fields.go`:
```go
package debug

import (
	"fmt"

	"claude-pixel/internal/input"
	"claude-pixel/internal/player"
)

type FieldSource interface {
	Player() *player.Player
	Intent() *input.Intent
	EngineFPS() float64
	EngineTPS() float64
}

type Field struct {
	Key    string
	Format func(s FieldSource) string
}

var Catalog = map[string]Field{
	"state":           {"state", func(s FieldSource) string { return "State: " + string(s.Player().FSM.CurrentID()) }},
	"facing":          {"facing", func(s FieldSource) string { return fmt.Sprintf("Facing: %+d", s.Player().Facing) }},
	"grounded":        {"grounded", func(s FieldSource) string { return fmt.Sprintf("Grounded: %t", s.Player().Grounded) }},
	"has_air_dash":    {"has_air_dash", func(s FieldSource) string { return fmt.Sprintf("HasAirDash: %t", s.Player().HasAirDash) }},
	"x":               {"x", func(s FieldSource) string { return fmt.Sprintf("X: %.2f", s.Player().X) }},
	"y":               {"y", func(s FieldSource) string { return fmt.Sprintf("Y: %.2f", s.Player().Y) }},
	"vx":              {"vx", func(s FieldSource) string { return fmt.Sprintf("VX: %.2f", s.Player().VX) }},
	"vy":              {"vy", func(s FieldSource) string { return fmt.Sprintf("VY: %.2f", s.Player().VY) }},
	"anim_id":         {"anim_id", func(s FieldSource) string { a := s.Player().Current; if a == nil { return "AnimID: -" }; return "AnimID: " + a.SpecID() }},
	"anim_frame":      {"anim_frame", func(s FieldSource) string { a := s.Player().Current; if a == nil { return "Frame: -" }; return fmt.Sprintf("Frame: %d", a.FrameIndex()) }},
	"anim_elapsed_ms": {"anim_elapsed_ms", func(s FieldSource) string { a := s.Player().Current; if a == nil { return "Elapsed: -" }; return fmt.Sprintf("Elapsed: %d ms", a.Elapsed().Milliseconds()) }},
	"intent_left":     {"intent_left", func(s FieldSource) string { return fmt.Sprintf("Left: %t", s.Intent().Left) }},
	"intent_right":    {"intent_right", func(s FieldSource) string { return fmt.Sprintf("Right: %t", s.Intent().Right) }},
	"intent_jump":     {"intent_jump", func(s FieldSource) string { return fmt.Sprintf("Jump: %t", s.Intent().JumpPressed) }},
	"intent_dash":     {"intent_dash", func(s FieldSource) string { return fmt.Sprintf("Dash: %t", s.Intent().DashPressed) }},
	"intent_attack":   {"intent_attack", func(s FieldSource) string { return fmt.Sprintf("Attack: %t", s.Intent().AttackPressed) }},
	"intent_attack2":  {"intent_attack2", func(s FieldSource) string { return fmt.Sprintf("Attack2: %t", s.Intent().Attack2Pressed) }},
	"fps":             {"fps", func(s FieldSource) string { return fmt.Sprintf("FPS: %.1f", s.EngineFPS()) }},
	"tps":             {"tps", func(s FieldSource) string { return fmt.Sprintf("TPS: %.1f", s.EngineTPS()) }},
}
```

- [ ] **Step 3: Write failing config test**

`internal/debug/config_test.go`:
```go
package debug

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigValid(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "debug.json")
	os.WriteFile(p, []byte(`{"sections":[{"title":"A","fields":["state","x","y"]}]}`), 0o644)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sections) != 1 || cfg.Sections[0].Title != "A" {
		t.Fatalf("bad cfg: %+v", cfg)
	}
}

func TestLoadConfigRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "debug.json")
	os.WriteFile(p, []byte(`{"sections":[{"title":"A","fields":["not_a_real_field"]}]}`), 0o644)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error on unknown field")
	}
}
```

- [ ] **Step 4: Run — FAIL**.

- [ ] **Step 5: Implement config loader**

`internal/debug/config.go`:
```go
package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type Section struct {
	Title  string   `json:"title"`
	Fields []string `json:"fields"`
}

type Config struct {
	Sections []Section `json:"sections"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read debug config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse debug config: %w", err)
	}
	var unknown []string
	for _, s := range cfg.Sections {
		for _, f := range s.Fields {
			if _, ok := Catalog[f]; !ok {
				unknown = append(unknown, f)
			}
		}
	}
	if len(unknown) > 0 {
		valid := make([]string, 0, len(Catalog))
		for k := range Catalog {
			valid = append(valid, k)
		}
		sort.Strings(valid)
		return nil, fmt.Errorf("debug config references unknown fields %v; valid keys: %v", unknown, valid)
	}
	return &cfg, nil
}
```

- [ ] **Step 6: Run — PASS**.

- [ ] **Step 7: Commit**

```bash
git add internal/debug/ config/debug.json
git commit -m "feat(debug): field catalog + JSON config loader with unknown-key validation"
```

---

## Task 15: Debug overlay

**Files:**
- Create: `internal/debug/overlay.go`

- [ ] **Step 1: Implement**

`internal/debug/overlay.go`:
```go
package debug

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Overlay struct {
	cfg     *Config
	source  FieldSource
	enabled bool
}

func NewOverlay(cfg *Config, source FieldSource) *Overlay {
	return &Overlay{cfg: cfg, source: source}
}

func (o *Overlay) Toggle()       { o.enabled = !o.enabled }
func (o *Overlay) Enabled() bool { return o.enabled }

func (o *Overlay) Draw(screen *ebiten.Image) {
	if !o.enabled {
		return
	}
	var b strings.Builder
	for i, sec := range o.cfg.Sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("-- ")
		b.WriteString(sec.Title)
		b.WriteString(" --\n")
		for _, key := range sec.Fields {
			f := Catalog[key]
			b.WriteString(f.Format(o.source))
			b.WriteString("\n")
		}
	}
	ebitenutil.DebugPrintAt(screen, b.String(), 8, 8)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/debug/overlay.go
git commit -m "feat(debug): Overlay renders sections + fields, toggle via Enabled"
```

---

## Task 16: Game loop + entry

**Files:**
- Create: `internal/game/game.go`
- Create: `cmd/game/main.go`

- [ ] **Step 1: Implement Game**

`internal/game/game.go`:
```go
package game

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/config"
	"claude-pixel/internal/debug"
	"claude-pixel/internal/input"
	"claude-pixel/internal/player"
	"claude-pixel/internal/world"
)

type Game struct {
	cfg        *config.Config
	world      *world.World
	player     *player.Player
	overlay    *debug.Overlay
	lastIntent input.Intent
}

func New(cfg *config.Config, anims map[string]*anim.Animation, physics *player.Physics, dbgCfg *debug.Config) *Game {
	w := world.New(cfg, physics.Gravity)
	p := player.New(player.Config{
		StartX:  float64(cfg.WindowW) / 2,
		StartY:  w.GroundY,
		Physics: physics,
		Anims:   anims,
	})
	p.Grounded = true
	p.HasAirDash = true

	g := &Game{cfg: cfg, world: w, player: p}
	g.overlay = debug.NewOverlay(dbgCfg, g)
	return g
}

// FieldSource impl for Overlay
func (g *Game) Player() *player.Player { return g.player }
func (g *Game) Intent() *input.Intent  { return &g.lastIntent }
func (g *Game) EngineFPS() float64     { return ebiten.ActualFPS() }
func (g *Game) EngineTPS() float64     { return ebiten.ActualTPS() }

func (g *Game) Layout(outerW, outerH int) (int, int) { return g.cfg.WindowW, g.cfg.WindowH }

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.overlay.Toggle()
	}
	g.lastIntent = input.Poll()
	dt := time.Second / 60
	g.player.FSM.Handle(g.player, g.lastIntent, dt)
	g.player.ApplyPhysics(g.world, dt)
	if g.player.Current != nil {
		g.player.Current.Update(dt)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x80, 0x80, 0x80, 0xFF})

	vector.DrawFilledRect(screen, 0, float32(g.world.GroundY), float32(g.cfg.WindowW), float32(g.cfg.WindowH)-float32(g.world.GroundY),
		color.RGBA{0x3A, 0x3A, 0x3A, 0xFF}, false)

	if g.player.Current != nil && g.player.Current.CurrentFrame() != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-float64(g.cfg.SpriteFrameW)/2, -float64(g.cfg.SpriteFrameH))
		if g.player.Facing < 0 {
			op.GeoM.Scale(-1, 1)
		}
		op.GeoM.Scale(float64(g.cfg.RenderScale), float64(g.cfg.RenderScale))
		op.GeoM.Translate(g.player.X, g.player.Y)
		op.Filter = ebiten.FilterNearest
		screen.DrawImage(g.player.Current.CurrentFrame(), op)
	}

	g.overlay.Draw(screen)
}
```

- [ ] **Step 2: Implement main entry**

`cmd/game/main.go`:
```go
package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/config"
	"claude-pixel/internal/debug"
	"claude-pixel/internal/game"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	cfg := config.Load()

	db := storage.MustOpen(cfg)
	defer db.Close()

	animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})

	anims, err := anim.LoadLibrary(cfg, animRepo)
	if err != nil {
		log.Fatalf("load animations: %v", err)
	}
	physics, err := player.LoadPhysics(tuneRepo)
	if err != nil {
		log.Fatalf("load physics: %v", err)
	}
	dbgCfg, err := debug.LoadConfig(cfg.DebugConfigPath)
	if err != nil {
		log.Fatalf("load debug config: %v", err)
	}

	g := game.New(cfg, anims, physics, dbgCfg)

	ebiten.SetWindowSize(cfg.WindowW, cfg.WindowH)
	ebiten.SetWindowTitle("claude-pixel")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: compiles.

- [ ] **Step 4: Manual smoke test**

Run: `make run`
Expected: a 1280×720 window opens; gray background; darker slab at bottom; character standing on it at Idle animation.

- [ ] **Step 5: Commit**

```bash
git add internal/game/ cmd/game/
git commit -m "feat(game): ebiten Game.Update/Draw/Layout + cmd/game entry wiring"
```

---

## Task 17: CLI — cmd/tune

**Files:**
- Create: `cmd/tune/main.go`

- [ ] **Step 1: Implement**

`cmd/tune/main.go`:
```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"claude-pixel/internal/config"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	cfg := config.Load()
	db := storage.MustOpen(cfg)
	defer db.Close()

	repo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})

	app := &cli.Command{
		Name:  "claude-pixel-tune",
		Usage: "Update physics tuning values stored in SQLite",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List every tunable parameter",
				Action: func(ctx context.Context, c *cli.Command) error {
					params, err := repo.List(ctx)
					if err != nil {
						return err
					}
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "KEY\tVALUE\tMIN\tMAX\tUNIT\tDESCRIPTION")
					for _, p := range params {
						fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.2f\t%s\t%s\n",
							p.Key, p.Value, p.MinValue, p.MaxValue, p.Unit, p.Description)
					}
					return w.Flush()
				},
			},
			{
				Name:      "set",
				Usage:     "Update an existing parameter (no creation)",
				ArgsUsage: "<key> <value>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 2 {
						return fmt.Errorf("usage: tune set <key> <value>")
					}
					key := c.Args().Get(0)
					raw := c.Args().Get(1)

					p, err := repo.Get(ctx, key)
					if err != nil {
						return fmt.Errorf("unknown tuning key %q. Run \"tune list\" to see valid keys", key)
					}

					newVal, err := strconv.ParseFloat(raw, 64)
					if err != nil {
						return fmt.Errorf("value %q is not a number: %v", raw, err)
					}

					if err := player.ValidateTuning(p, newVal); err != nil {
						return err
					}

					old := p.Value
					p.Value = newVal
					if err := repo.Upsert(ctx, p); err != nil {
						return err
					}
					fmt.Printf("OK: %s = %.4f %s (was %.4f)\n", p.Key, newVal, p.Unit, old)
					return nil
				},
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: compiles.

- [ ] **Step 3: Smoke-test CLI**

Run:
```bash
go run ./cmd/tune list
```
Expected: a table of 7 rows with the seed values.

```bash
go run ./cmd/tune set run_speed 320
```
Expected output: `OK: run_speed = 320.0000 px/s (was 280.0000)`.

```bash
go run ./cmd/tune set run_speed 99999
```
Expected: error `value out of range: 99999 not in [50, 1000] px/s`.

```bash
go run ./cmd/tune set nope 1
```
Expected: error `unknown tuning key "nope". Run "tune list" to see valid keys`.

```bash
go run ./cmd/tune set run_speed 280
```
Reset to original.

- [ ] **Step 4: Commit**

```bash
git add cmd/tune/
git commit -m "feat(tune): CLI list/set with validation via urfave/cli/v3"
```

---

## Task 18: End-to-end manual verification

**Files:** none changed — this task is a checklist.

- [ ] **Step 1: Clean DB so migrations apply fresh**

```bash
rm -rf data/
```

- [ ] **Step 2: Run the game**

```bash
make run
```

Verify in order:

- [ ] **Window** opens at 1280×720 titled `claude-pixel`.
- [ ] **Background** is light gray; **ground slab** is darker at the bottom.
- [ ] **Idle animation** loops (character breathing / standing idle — 10 frames over ~1 s).
- [ ] Press **D** (or Right arrow): character turns right and plays Run animation.
- [ ] Press **A** (or Left arrow): character flips to face left and runs left.
- [ ] Release movement: returns to Idle.
- [ ] Press **Space** while grounded: Jump animation plays, character rises; apex reached, then Fall animation plays; character lands and returns to Idle/Run.
- [ ] Hold a direction + Space: character jumps with horizontal air control.
- [ ] Press **Space** while airborne: nothing (no double-jump).
- [ ] Press **Shift** on the ground: Dash animation plays; character slides in facing direction; after ~0.5 s returns to Idle/Fall.
- [ ] Jump, then press **Shift** in air: one air-dash triggers; pressing Shift again in the same airborne phase does NOT dash again; after landing and jumping again, the air-dash is available again.
- [ ] Press **J** or **X** while idle: Attack animation plays; movement locked until finished.
- [ ] Press **Shift** during Attack: Attack cancels into Dash.
- [ ] Press **Space** during Attack while grounded: Attack cancels into Jump.
- [ ] Press **K** or **C**: Attack2 plays (6 frames over 1.5 s).
- [ ] Press **F3**: debug overlay appears with all 5 sections populated. Press F3 again: overlay disappears.
- [ ] With overlay on: verify `State`, `Facing`, `X/Y/VX/VY`, `AnimID`, `Frame`, `Elapsed ms`, intent flags, FPS/TPS all update in real time.

- [ ] **Step 3: Verify tuning CLI changes take effect on next run**

```bash
go run ./cmd/tune set run_speed 600
make run
```
Expected: character runs visibly faster. Restore:
```bash
go run ./cmd/tune set run_speed 280
```

- [ ] **Step 4: Edit `config/debug.json`** to remove a section and re-run — overlay reflects the change.

- [ ] **Step 5: Commit if anything needed adjustment.** Otherwise skip.

---

## Notes

- Ebiten is graphics-coupled; the render code (`game.Draw`) and sprite slicing (`anim.Slice`) are verified manually in Task 18 rather than via unit tests.
- Unit tests cover the pure-logic cores: config, Repository, Animation math, FSM transitions, tuning validator, debug config validation.
- Frequent commits: 16 commits total — one per task that produces a working slice.
