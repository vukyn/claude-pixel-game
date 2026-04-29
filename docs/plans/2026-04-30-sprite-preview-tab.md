# Sprite/anim Preview Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a 4th inspector tab `Sprite` that previews any animation owned by the currently edited enemy kind, with auto-sync to the selected BT state's `state.anim`.

**Architecture:** New backend port/adapter/service/handler trio in `internal/editor` exposes `GET /api/anims/:kind` that filters `storage.Repository[anim.AnimationSpec]` rows in Go (same pattern as tuning). Fiber's `app.Static` serves `assets/` read-only. Frontend mounts a vertical-split pane: `SpriteList` (left) + `SpriteCanvasPanel` (right), powered by a Zustand `spriteStore` and a `<canvas>` rendering loop that mirrors `internal/anim/sheet.go` slicing.

**Tech Stack:** Go (Fiber, sqlite via `internal/storage`), React 19 + TypeScript, Zustand, zod, Vite, Vitest, shadcn/ui Tabs.

**Spec:** [docs/superpowers/specs/2026-04-29-sprite-preview-tab-design.md](../superpowers/specs/2026-04-29-sprite-preview-tab-design.md)

---

## File Map

**Create:**
- `internal/editor/adapter/sqliteanim.go` — `SQLiteAnim` adapter wrapping `storage.Repository[anim.AnimationSpec]`
- `internal/editor/adapter/sqliteanim_test.go` — adapter tests (live sqlite + seed)
- `internal/editor/service/anim.go` — `Anim` service
- `internal/editor/service/anim_test.go` — service tests with stub store
- `tools/editor-web/src/api/sprite-schemas.ts` — zod schemas for AnimSpec
- `tools/editor-web/src/api/sprites.ts` — fetch helpers
- `tools/editor-web/src/api/__tests__/sprites.test.ts` — client tests with msw
- `tools/editor-web/src/state/spriteStore.ts` — Zustand store
- `tools/editor-web/src/state/__tests__/spriteStore.test.ts` — store tests
- `tools/editor-web/src/components/SpriteCanvas.tsx` — `<canvas>` with RAF + slicing
- `tools/editor-web/src/components/__tests__/SpriteCanvas.test.tsx` — slicing math tests
- `tools/editor-web/src/components/SpriteList.tsx` — list with selection
- `tools/editor-web/src/components/SpriteCanvasPanel.tsx` — canvas + controls bar
- `tools/editor-web/src/components/SpritePane.tsx` — composition + auto-sync
- `tools/editor-web/src/components/__tests__/SpritePane.test.tsx` — auto-sync test

**Modify:**
- `internal/editor/port/repository.go` — add `AnimRow` + `AnimStore` interface
- `internal/editor/http/handler.go` — add `Anim *service.Anim` to Deps + `GET /api/anims/:kind`
- `internal/editor/http/handler_test.go` — extend for new route
- `cmd/editor/main.go` — wire `SQLiteAnim` adapter, register static handler
- `tools/editor-web/vite.config.ts` — proxy `/assets` to `:8080`
- `tools/editor-web/src/components/Inspector.tsx` — add `sprite` tab + selectedAnim wiring

---

## Backend

### Task 1: Define `AnimRow` and `AnimStore` port

**Files:**
- Modify: `internal/editor/port/repository.go`

- [ ] **Step 1: Append `AnimStore` interface and `AnimRow` struct**

Open `internal/editor/port/repository.go`. Add at the bottom of the file (after `RegistryStore`):

```go
// AnimStore lists animation specs filtered by an owner-kind prefix.
type AnimStore interface {
	ListByKind(kind string) ([]AnimRow, error)
}

type AnimRow struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	FrameW     int    `json:"frame_w"`
	FrameH     int    `json:"frame_h"`
	FrameCount int    `json:"frame_count"`
	DurationMs int    `json:"duration_ms"`
	Loop       bool   `json:"loop"`
	GridCols   int    `json:"grid_cols"`
	GridRows   int    `json:"grid_rows"`
	PickRow    int    `json:"pick_row"`
	PickCol    int    `json:"pick_col"`
}
```

- [ ] **Step 2: Verify the file compiles**

Run: `go build ./internal/editor/...`
Expected: succeeds with no output.

- [ ] **Step 3: Commit**

```bash
git add internal/editor/port/repository.go
git commit -m "editor: add AnimStore port for sprite preview"
```

---

### Task 2: `SQLiteAnim` adapter (TDD)

**Files:**
- Create: `internal/editor/adapter/sqliteanim_test.go`
- Create: `internal/editor/adapter/sqliteanim.go`

- [ ] **Step 1: Write failing adapter tests**

Create `internal/editor/adapter/sqliteanim_test.go`:

```go
package adapter_test

import (
	"path/filepath"
	"strings"
	"testing"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/config"
	"claude-pixel/internal/editor/adapter"
	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/storage"
)

func newAnimRepo(t *testing.T) *storage.Repository[anim.AnimationSpec] {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(&config.Config{DBPath: dbPath})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
}

func TestSQLiteAnim_ListByKind_FiltersPrefix(t *testing.T) {
	a := adapter.NewSQLiteAnim(newAnimRepo(t))
	rows, err := a.ListByKind("orc")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one orc_* row from seed")
	}
	for _, r := range rows {
		if !strings.HasPrefix(r.ID, "orc_") {
			t.Fatalf("row %q does not match prefix orc_", r.ID)
		}
	}
}

func TestSQLiteAnim_ListByKind_OnlyEnemyRows(t *testing.T) {
	a := adapter.NewSQLiteAnim(newAnimRepo(t))
	rows, err := a.ListByKind("soldier")
	if err != nil {
		t.Fatal(err)
	}
	// Soldier rows are is_player=1, not is_enemy=1, so filter must skip them.
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for soldier (player kind), got %d", len(rows))
	}
}

func TestSQLiteAnim_ListByKind_UnknownReturnsEmpty(t *testing.T) {
	a := adapter.NewSQLiteAnim(newAnimRepo(t))
	rows, err := a.ListByKind("ghost")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("want 0 rows, got %d", len(rows))
	}
}

func TestSQLiteAnim_ListByKind_FieldsCopied(t *testing.T) {
	a := adapter.NewSQLiteAnim(newAnimRepo(t))
	rows, err := a.ListByKind("orc")
	if err != nil {
		t.Fatal(err)
	}
	r := rows[0]
	if r.FrameW <= 0 || r.FrameH <= 0 || r.FrameCount <= 0 || r.DurationMs <= 0 {
		t.Fatalf("expected positive dims/count/duration: %+v", r)
	}
	if r.Path == "" {
		t.Fatalf("path missing: %+v", r)
	}
}

var _ port.AnimStore = (*adapter.SQLiteAnim)(nil)
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/editor/adapter/ -run TestSQLiteAnim`
Expected: FAIL — `undefined: adapter.NewSQLiteAnim` and `undefined: adapter.SQLiteAnim`.

- [ ] **Step 3: Implement adapter**

Create `internal/editor/adapter/sqliteanim.go`:

```go
package adapter

import (
	"context"
	"fmt"
	"strings"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/storage"
)

type SQLiteAnim struct{ repo *storage.Repository[anim.AnimationSpec] }

func NewSQLiteAnim(repo *storage.Repository[anim.AnimationSpec]) *SQLiteAnim {
	return &SQLiteAnim{repo: repo}
}

func (s *SQLiteAnim) ListByKind(kind string) ([]port.AnimRow, error) {
	all, err := s.repo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("sqliteanim: list: %w", err)
	}
	prefix := kind + "_"
	out := make([]port.AnimRow, 0, len(all))
	for _, a := range all {
		if !a.IsEnemy {
			continue
		}
		if !strings.HasPrefix(a.ID, prefix) {
			continue
		}
		out = append(out, port.AnimRow{
			ID:         a.ID,
			Path:       a.Path,
			FrameW:     a.FrameW,
			FrameH:     a.FrameH,
			FrameCount: a.FrameCount,
			DurationMs: a.DurationMs,
			Loop:       a.Loop,
			GridCols:   a.GridCols,
			GridRows:   a.GridRows,
			PickRow:    a.PickRow,
			PickCol:    a.PickCol,
		})
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests, confirm they pass**

Run: `go test ./internal/editor/adapter/ -run TestSQLiteAnim -v`
Expected: PASS — 4 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/editor/adapter/sqliteanim.go internal/editor/adapter/sqliteanim_test.go
git commit -m "editor: SQLiteAnim adapter filters anims by kind prefix"
```

---

### Task 3: `Anim` service (TDD)

**Files:**
- Create: `internal/editor/service/anim_test.go`
- Create: `internal/editor/service/anim.go`

- [ ] **Step 1: Write failing service tests**

Create `internal/editor/service/anim_test.go`:

```go
package service_test

import (
	"errors"
	"testing"

	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/editor/service"
)

type stubAnim struct {
	listFn func(string) ([]port.AnimRow, error)
}

func (s stubAnim) ListByKind(k string) ([]port.AnimRow, error) { return s.listFn(k) }

func TestAnim_List_DelegatesToStore(t *testing.T) {
	called := ""
	want := []port.AnimRow{{ID: "orc_idle"}}
	svc := service.NewAnim(stubAnim{listFn: func(k string) ([]port.AnimRow, error) {
		called = k
		return want, nil
	}})
	got, err := svc.List("orc")
	if err != nil {
		t.Fatal(err)
	}
	if called != "orc" {
		t.Fatalf("kind passed: %q", called)
	}
	if len(got) != 1 || got[0].ID != "orc_idle" {
		t.Fatalf("rows: %+v", got)
	}
}

func TestAnim_List_PropagatesError(t *testing.T) {
	svc := service.NewAnim(stubAnim{listFn: func(string) ([]port.AnimRow, error) {
		return nil, errors.New("boom")
	}})
	if _, err := svc.List("orc"); err == nil {
		t.Fatal("want error")
	}
}

func TestAnim_List_RejectsEmptyKind(t *testing.T) {
	svc := service.NewAnim(stubAnim{listFn: func(string) ([]port.AnimRow, error) {
		t.Fatal("store should not be called")
		return nil, nil
	}})
	if _, err := svc.List(""); err == nil {
		t.Fatal("want error for empty kind")
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/editor/service/ -run TestAnim`
Expected: FAIL — `undefined: service.NewAnim`.

- [ ] **Step 3: Implement service**

Create `internal/editor/service/anim.go`:

```go
package service

import (
	"errors"

	"claude-pixel/internal/editor/port"
)

type Anim struct{ store port.AnimStore }

func NewAnim(store port.AnimStore) *Anim { return &Anim{store: store} }

func (a *Anim) List(kind string) ([]port.AnimRow, error) {
	if kind == "" {
		return nil, errors.New("anim: kind required")
	}
	return a.store.ListByKind(kind)
}
```

- [ ] **Step 4: Run tests, confirm they pass**

Run: `go test ./internal/editor/service/ -run TestAnim -v`
Expected: PASS — 3 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/editor/service/anim.go internal/editor/service/anim_test.go
git commit -m "editor: Anim service with empty-kind guard"
```

---

### Task 4: HTTP route `GET /api/anims/:kind` (TDD)

**Files:**
- Modify: `internal/editor/http/handler.go`
- Modify: `internal/editor/http/handler_test.go`

- [ ] **Step 1: Add stub `stubAnimStore` and failing test**

Open `internal/editor/http/handler_test.go`. Add this block after the `stubRegistry` declaration (around line 41):

```go
type stubAnimStore struct {
	listFn func(string) ([]port.AnimRow, error)
}

func (s stubAnimStore) ListByKind(k string) ([]port.AnimRow, error) { return s.listFn(k) }
```

Update `newApp` signature so it also accepts an `port.AnimStore` argument (and feeds Deps). Replace the existing `newApp` with:

```go
func newApp(b port.BehaviorStore, t port.TuningStore, r port.RegistryStore, an port.AnimStore) *fiber.App {
	app := fiber.New()
	deps := http.Deps{
		Behavior: service.NewBehavior(b),
		Tuning:   service.NewTuning(t),
		Registry: service.NewRegistry(r),
	}
	if an != nil {
		deps.Anim = service.NewAnim(an)
	}
	http.Register(app, deps)
	return app
}
```

Then update every existing `newApp(b, t, r)` call in this file to pass `nil` as the new fourth argument (search-replace `newApp(` callsites to `newApp(..., nil)`). Add at the bottom of the file:

```go
func TestGetAnimsByKind(t *testing.T) {
	app := newApp(nil, nil, nil, stubAnimStore{listFn: func(k string) ([]port.AnimRow, error) {
		if k != "orc" {
			t.Fatalf("kind: %q", k)
		}
		return []port.AnimRow{{
			ID: "orc_idle", Path: "assets/sprites/orc/Idle.png",
			FrameW: 100, FrameH: 100, FrameCount: 6, DurationMs: 600, Loop: true,
			GridCols: 6, GridRows: 1, PickRow: 0, PickCol: 0,
		}}, nil
	}})
	code, body := do(t, app, "GET", "/api/anims/orc", nil)
	if code != 200 {
		t.Fatalf("status %d body %s", code, body)
	}
	if !strings.Contains(string(body), `"id":"orc_idle"`) {
		t.Fatalf("body: %s", body)
	}
}

func TestGetAnimsByKindEmpty(t *testing.T) {
	app := newApp(nil, nil, nil, stubAnimStore{listFn: func(string) ([]port.AnimRow, error) {
		return []port.AnimRow{}, nil
	}})
	code, body := do(t, app, "GET", "/api/anims/ghost", nil)
	if code != 200 || string(body) != "[]" {
		t.Fatalf("status %d body %s", code, body)
	}
}

func TestGetAnimsByKindError(t *testing.T) {
	app := newApp(nil, nil, nil, stubAnimStore{listFn: func(string) ([]port.AnimRow, error) {
		return nil, errors.New("boom")
	}})
	code, _ := do(t, app, "GET", "/api/anims/orc", nil)
	if code != 500 {
		t.Fatalf("status %d", code)
	}
}
```

- [ ] **Step 2: Run tests, confirm they fail**

Run: `go test ./internal/editor/http/ -run TestGetAnims`
Expected: FAIL — `Deps` has no field `Anim`, route 404.

- [ ] **Step 3: Add `Anim` field to Deps and route**

Open `internal/editor/http/handler.go`. Modify `Deps`:

```go
type Deps struct {
	Behavior *service.Behavior
	Tuning   *service.Tuning
	Registry *service.Registry
	Anim     *service.Anim
}
```

Add inside `Register(app, d)`, just before the closing brace of the function:

```go
	if d.Anim != nil {
		app.Get("/api/anims/:kind", func(c *fiber.Ctx) error {
			rows, err := d.Anim.List(c.Params("kind"))
			if err != nil {
				return fiber.NewError(500, err.Error())
			}
			if rows == nil {
				rows = []port.AnimRow{}
			}
			return c.JSON(rows)
		})
	}
```

Add `"claude-pixel/internal/editor/port"` to the import block if not already present.

- [ ] **Step 4: Run tests, confirm they pass**

Run: `go test ./internal/editor/http/ -v`
Expected: PASS — all existing tests plus 3 new.

- [ ] **Step 5: Commit**

```bash
git add internal/editor/http/handler.go internal/editor/http/handler_test.go
git commit -m "editor: GET /api/anims/:kind route"
```

---

### Task 5: Wire `Anim` service + static `/assets` handler in `cmd/editor`

**Files:**
- Modify: `cmd/editor/main.go`

- [ ] **Step 1: Construct repo, adapter, service; register static**

Open `cmd/editor/main.go`. After the existing `tuningRepo := ...` line, add:

```go
	animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
```

Add `"claude-pixel/internal/anim"` to the imports.

After `editorhttp.DefaultMiddleware(app)`, add:

```go
	app.Static("/assets", "./assets", fiber.Static{Browse: false})
```

Update the `editorhttp.Register` call to include `Anim`:

```go
	editorhttp.Register(app, editorhttp.Deps{
		Behavior: service.NewBehavior(adapter.NewFSBehavior(behaviorsDir)),
		Tuning:   service.NewTuning(adapter.NewSQLiteTuning(tuningRepo)),
		Registry: service.NewRegistry(adapter.NewRuntimeRegistry()),
		Anim:     service.NewAnim(adapter.NewSQLiteAnim(animRepo)),
	})
```

- [ ] **Step 2: Build the editor binary**

Run: `go build ./cmd/editor/`
Expected: succeeds, no output.

- [ ] **Step 3: Smoke-test the routes**

Run in one terminal: `make editor`
Run in another: `curl -s http://localhost:8080/api/anims/orc | head -c 200 && echo`
Expected: JSON array starting `[{"id":"orc_idle","path":"assets/sprites/...`

Run: `curl -sI http://localhost:8080/assets/sprites/orc/Idle.png | head -5`
Expected: `HTTP/1.1 200 OK` (or 200 with png content-type). Stop the editor with Ctrl-C.

If `assets/sprites/orc/Idle.png` is not in your tree, run `curl -sI http://localhost:8080/assets/behaviors/orc.json | head -5` — expect 200 with `application/json`.

- [ ] **Step 4: Commit**

```bash
git add cmd/editor/main.go
git commit -m "editor: wire Anim service + serve /assets static"
```

---

## Frontend

### Task 6: Vite proxy `/assets`

**Files:**
- Modify: `tools/editor-web/vite.config.ts`

- [ ] **Step 1: Add `/assets` to the proxy map**

Edit `tools/editor-web/vite.config.ts`. Replace the `proxy` line in the `server` block with:

```ts
    proxy: {
      '/api': 'http://localhost:8080',
      '/assets': 'http://localhost:8080',
    },
```

- [ ] **Step 2: Verify Vite still builds**

Run: `cd tools/editor-web && npm run build`
Expected: builds successfully.

- [ ] **Step 3: Commit**

```bash
git add tools/editor-web/vite.config.ts
git commit -m "editor-web: proxy /assets to editor server"
```

---

### Task 7: zod schema + API client for sprites (TDD)

**Files:**
- Create: `tools/editor-web/src/api/sprite-schemas.ts`
- Create: `tools/editor-web/src/api/sprites.ts`
- Create: `tools/editor-web/src/api/__tests__/sprites.test.ts`

- [ ] **Step 1: Write the schema**

Create `tools/editor-web/src/api/sprite-schemas.ts`:

```ts
import { z } from 'zod'

export const AnimSpecSchema = z.object({
  id: z.string(),
  path: z.string(),
  frame_w: z.number().int().positive(),
  frame_h: z.number().int().positive(),
  frame_count: z.number().int().positive(),
  duration_ms: z.number().int().positive(),
  loop: z.boolean(),
  grid_cols: z.number().int().nonnegative(),
  grid_rows: z.number().int().nonnegative(),
  pick_row: z.number().int(),
  pick_col: z.number().int(),
})
export type AnimSpec = z.infer<typeof AnimSpecSchema>
```

- [ ] **Step 2: Write failing client test**

Create `tools/editor-web/src/api/__tests__/sprites.test.ts`:

```ts
import { describe, it, expect, beforeAll, afterAll, afterEach } from 'vitest'
import { setupServer } from 'msw/node'
import { http, HttpResponse } from 'msw'
import { fetchAnims } from '../sprites'

const server = setupServer(
  http.get('/api/anims/orc', () => HttpResponse.json([
    {
      id: 'orc_idle', path: 'assets/sprites/orc/Idle.png',
      frame_w: 100, frame_h: 100, frame_count: 6, duration_ms: 600, loop: true,
      grid_cols: 6, grid_rows: 1, pick_row: 0, pick_col: 0,
    },
  ])),
  http.get('/api/anims/empty', () => HttpResponse.json([])),
  http.get('/api/anims/boom', () => new HttpResponse('oops', { status: 500 })),
)

beforeAll(() => server.listen())
afterEach(() => server.resetHandlers())
afterAll(() => server.close())

describe('fetchAnims', () => {
  it('parses successful response', async () => {
    const rows = await fetchAnims('orc')
    expect(rows).toHaveLength(1)
    expect(rows[0].id).toBe('orc_idle')
  })

  it('returns empty array for unknown kind', async () => {
    const rows = await fetchAnims('empty')
    expect(rows).toEqual([])
  })

  it('throws on server error', async () => {
    await expect(fetchAnims('boom')).rejects.toThrow()
  })
})
```

- [ ] **Step 3: Run, confirm fails**

Run: `cd tools/editor-web && npm test -- src/api/__tests__/sprites.test.ts`
Expected: FAIL — `fetchAnims` not exported.

- [ ] **Step 4: Implement client**

Create `tools/editor-web/src/api/sprites.ts`:

```ts
import { z } from 'zod'
import { AnimSpecSchema, type AnimSpec } from './sprite-schemas'

class ApiError extends Error {
  constructor(public status: number, message: string) { super(message) }
}

export async function fetchAnims(kind: string): Promise<AnimSpec[]> {
  const res = await fetch(`/api/anims/${encodeURIComponent(kind)}`, {
    headers: { Accept: 'application/json' },
  })
  if (!res.ok) throw new ApiError(res.status, await res.text())
  return z.array(AnimSpecSchema).parse(await res.json())
}

export function spriteUrl(path: string): string {
  // server stores paths like "assets/sprites/orc/Idle.png" — turn into "/assets/sprites/orc/Idle.png"
  if (path.startsWith('/')) return path
  if (path.startsWith('assets/')) return `/${path}`
  return `/assets/${path}`
}
```

- [ ] **Step 5: Run, confirm passes**

Run: `cd tools/editor-web && npm test -- src/api/__tests__/sprites.test.ts`
Expected: PASS — 3 tests.

- [ ] **Step 6: Commit**

```bash
git add tools/editor-web/src/api/sprite-schemas.ts tools/editor-web/src/api/sprites.ts tools/editor-web/src/api/__tests__/sprites.test.ts
git commit -m "editor-web: api client for /api/anims/:kind"
```

---

### Task 8: `spriteStore` Zustand store (TDD)

**Files:**
- Create: `tools/editor-web/src/state/__tests__/spriteStore.test.ts`
- Create: `tools/editor-web/src/state/spriteStore.ts`

- [ ] **Step 1: Write failing tests**

Create `tools/editor-web/src/state/__tests__/spriteStore.test.ts`:

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { useSpriteStore, __resetSpriteStore } from '../spriteStore'
import type { AnimSpec } from '../../api/sprite-schemas'

const orcIdle: AnimSpec = {
  id: 'orc_idle', path: 'assets/sprites/orc/Idle.png',
  frame_w: 100, frame_h: 100, frame_count: 6, duration_ms: 600, loop: true,
  grid_cols: 6, grid_rows: 1, pick_row: 0, pick_col: 0,
}
const orcRun: AnimSpec = { ...orcIdle, id: 'orc_run', path: 'assets/sprites/orc/Run.png', frame_count: 8, duration_ms: 800 }

describe('spriteStore', () => {
  beforeEach(() => __resetSpriteStore())

  it('setAnims populates list and clears selection if not present', () => {
    useSpriteStore.getState().setAnims([orcIdle, orcRun])
    expect(useSpriteStore.getState().anims).toHaveLength(2)
    expect(useSpriteStore.getState().selectedId).toBeNull()
  })

  it('selectById sets selection, resets frame, plays', () => {
    useSpriteStore.getState().setAnims([orcIdle, orcRun])
    useSpriteStore.getState().selectById('orc_run')
    const s = useSpriteStore.getState()
    expect(s.selectedId).toBe('orc_run')
    expect(s.frame).toBe(0)
    expect(s.playing).toBe(true)
  })

  it('selectById ignores unknown id', () => {
    useSpriteStore.getState().setAnims([orcIdle])
    useSpriteStore.getState().selectById('orc_missing')
    expect(useSpriteStore.getState().selectedId).toBeNull()
  })

  it('togglePlay flips playing flag', () => {
    useSpriteStore.getState().setAnims([orcIdle])
    useSpriteStore.getState().selectById('orc_idle')
    expect(useSpriteStore.getState().playing).toBe(true)
    useSpriteStore.getState().togglePlay()
    expect(useSpriteStore.getState().playing).toBe(false)
  })

  it('scrub sets frame and pauses, clamps to [0, frame_count-1]', () => {
    useSpriteStore.getState().setAnims([orcIdle])
    useSpriteStore.getState().selectById('orc_idle')
    useSpriteStore.getState().scrub(3)
    expect(useSpriteStore.getState().frame).toBe(3)
    expect(useSpriteStore.getState().playing).toBe(false)
    useSpriteStore.getState().scrub(99)
    expect(useSpriteStore.getState().frame).toBe(5) // 6 frames -> max idx 5
    useSpriteStore.getState().scrub(-2)
    expect(useSpriteStore.getState().frame).toBe(0)
  })

  it('advanceFrame loops when spec.loop is true', () => {
    useSpriteStore.getState().setAnims([orcIdle])
    useSpriteStore.getState().selectById('orc_idle')
    useSpriteStore.getState().scrub(5)
    useSpriteStore.getState().advanceFrame()
    expect(useSpriteStore.getState().frame).toBe(0)
  })

  it('advanceFrame clamps and pauses on non-looping anim at last frame', () => {
    const oneShot: AnimSpec = { ...orcIdle, id: 'orc_attack', loop: false, frame_count: 4 }
    useSpriteStore.getState().setAnims([oneShot])
    useSpriteStore.getState().selectById('orc_attack')
    useSpriteStore.getState().scrub(3)
    useSpriteStore.getState().togglePlay() // resume after scrub paused us
    useSpriteStore.getState().advanceFrame()
    expect(useSpriteStore.getState().frame).toBe(3)
    expect(useSpriteStore.getState().playing).toBe(false)
  })
})
```

- [ ] **Step 2: Run, confirm fails**

Run: `cd tools/editor-web && npm test -- src/state/__tests__/spriteStore.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement store**

Create `tools/editor-web/src/state/spriteStore.ts`:

```ts
import { create } from 'zustand'
import type { AnimSpec } from '../api/sprite-schemas'

interface SpriteState {
  anims: AnimSpec[]
  selectedId: string | null
  frame: number
  playing: boolean
  setAnims(anims: AnimSpec[]): void
  selectById(id: string): void
  togglePlay(): void
  scrub(frame: number): void
  advanceFrame(): void
}

const initial = {
  anims: [] as AnimSpec[],
  selectedId: null as string | null,
  frame: 0,
  playing: false,
}

function selected(s: { anims: AnimSpec[]; selectedId: string | null }): AnimSpec | undefined {
  return s.anims.find(a => a.id === s.selectedId)
}

function clamp(n: number, max: number): number {
  if (n < 0) return 0
  if (n > max) return max
  return n
}

export const useSpriteStore = create<SpriteState>((set, get) => ({
  ...initial,
  setAnims(anims) {
    const cur = get().selectedId
    const stillThere = anims.some(a => a.id === cur)
    set({ anims, selectedId: stillThere ? cur : null, frame: stillThere ? get().frame : 0 })
  },
  selectById(id) {
    if (!get().anims.some(a => a.id === id)) return
    set({ selectedId: id, frame: 0, playing: true })
  },
  togglePlay() {
    if (!get().selectedId) return
    set(s => ({ playing: !s.playing }))
  },
  scrub(frame) {
    const s = get()
    const spec = selected(s)
    if (!spec) return
    set({ frame: clamp(frame, spec.frame_count - 1), playing: false })
  },
  advanceFrame() {
    const s = get()
    const spec = selected(s)
    if (!spec) return
    const next = s.frame + 1
    if (next >= spec.frame_count) {
      if (spec.loop) set({ frame: 0 })
      else set({ frame: spec.frame_count - 1, playing: false })
    } else {
      set({ frame: next })
    }
  },
}))

export function __resetSpriteStore() {
  useSpriteStore.setState({ ...initial })
}
```

- [ ] **Step 4: Run, confirm passes**

Run: `cd tools/editor-web && npm test -- src/state/__tests__/spriteStore.test.ts`
Expected: PASS — 7 tests.

- [ ] **Step 5: Commit**

```bash
git add tools/editor-web/src/state/spriteStore.ts tools/editor-web/src/state/__tests__/spriteStore.test.ts
git commit -m "editor-web: spriteStore (selection, scrub, frame advance)"
```

---

### Task 9: `SpriteCanvas` slicing math (TDD)

**Files:**
- Create: `tools/editor-web/src/components/__tests__/SpriteCanvas.test.tsx`
- Create: `tools/editor-web/src/components/SpriteCanvas.tsx`

- [ ] **Step 1: Write failing slicing-math test**

Create `tools/editor-web/src/components/__tests__/SpriteCanvas.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest'
import { frameRect } from '../SpriteCanvas'
import type { AnimSpec } from '../../api/sprite-schemas'

const base: AnimSpec = {
  id: 'x', path: 'p',
  frame_w: 100, frame_h: 80,
  frame_count: 6, duration_ms: 600, loop: true,
  grid_cols: 0, grid_rows: 0, pick_row: 0, pick_col: -1,
}

describe('frameRect', () => {
  it('flat strip: column = frame, row = 0', () => {
    const r = frameRect({ ...base, grid_cols: 0, grid_rows: 0 }, 3)
    expect(r).toEqual({ sx: 300, sy: 0, sw: 100, sh: 80 })
  })

  it('row-mode grid: row = pick_row, column = frame', () => {
    const spec = { ...base, grid_cols: 6, grid_rows: 4, pick_row: 2, pick_col: -1 }
    const r = frameRect(spec, 4)
    expect(r).toEqual({ sx: 4 * 100, sy: 2 * 80, sw: 100, sh: 80 })
  })

  it('column-mode grid: column = pick_col, row = frame', () => {
    const spec = { ...base, grid_cols: 4, grid_rows: 6, pick_row: 0, pick_col: 1 }
    const r = frameRect(spec, 3)
    expect(r).toEqual({ sx: 1 * 100, sy: 3 * 80, sw: 100, sh: 80 })
  })
})
```

- [ ] **Step 2: Run, confirm fails**

Run: `cd tools/editor-web && npm test -- src/components/__tests__/SpriteCanvas.test.tsx`
Expected: FAIL — module not found / `frameRect` not exported.

- [ ] **Step 3: Implement `SpriteCanvas` with exported `frameRect`**

Create `tools/editor-web/src/components/SpriteCanvas.tsx`:

```tsx
import { useEffect, useRef } from 'react'
import type { AnimSpec } from '../api/sprite-schemas'
import { spriteUrl } from '../api/sprites'

export interface FrameRect { sx: number; sy: number; sw: number; sh: number }

export function frameRect(spec: AnimSpec, frame: number): FrameRect {
  const { frame_w, frame_h, grid_cols, grid_rows, pick_row, pick_col } = spec
  let col = 0, row = 0
  if (grid_cols > 0 && grid_rows > 0) {
    if (pick_col >= 0) { col = pick_col; row = frame }
    else { col = frame; row = pick_row }
  } else {
    col = frame; row = 0
  }
  return { sx: col * frame_w, sy: row * frame_h, sw: frame_w, sh: frame_h }
}

interface Props {
  spec: AnimSpec
  frame: number
  playing: boolean
  onAdvance: () => void
  scale?: number
}

export function SpriteCanvas({ spec, frame, playing, onAdvance, scale = 2 }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const imgRef = useRef<HTMLImageElement | null>(null)
  const lastIdRef = useRef<string | null>(null)
  const lastTickRef = useRef<number>(0)
  const rafRef = useRef<number | null>(null)

  // Load image when anim changes.
  useEffect(() => {
    if (lastIdRef.current === spec.id && imgRef.current) return
    const img = new Image()
    img.src = spriteUrl(spec.path)
    img.onload = () => { imgRef.current = img; draw() }
    img.onerror = () => { imgRef.current = null; drawError() }
    lastIdRef.current = spec.id
  }, [spec.id, spec.path])

  // Redraw on frame change.
  useEffect(() => { draw() }, [spec, frame])

  // RAF loop for play.
  useEffect(() => {
    if (!playing) {
      if (rafRef.current != null) cancelAnimationFrame(rafRef.current)
      rafRef.current = null
      return
    }
    const perFrameMs = spec.duration_ms / spec.frame_count
    lastTickRef.current = performance.now()
    const tick = (now: number) => {
      if (now - lastTickRef.current >= perFrameMs) {
        lastTickRef.current = now
        onAdvance()
      }
      rafRef.current = requestAnimationFrame(tick)
    }
    rafRef.current = requestAnimationFrame(tick)
    return () => {
      if (rafRef.current != null) cancelAnimationFrame(rafRef.current)
      rafRef.current = null
    }
  }, [playing, spec.id, spec.duration_ms, spec.frame_count, onAdvance])

  function draw() {
    const c = canvasRef.current; if (!c) return
    const ctx = c.getContext('2d'); if (!ctx) return
    c.width = spec.frame_w * scale
    c.height = spec.frame_h * scale
    ctx.imageSmoothingEnabled = false
    ctx.clearRect(0, 0, c.width, c.height)
    const img = imgRef.current
    if (!img) return
    const { sx, sy, sw, sh } = frameRect(spec, frame)
    ctx.drawImage(img, sx, sy, sw, sh, 0, 0, sw * scale, sh * scale)
  }

  function drawError() {
    const c = canvasRef.current; if (!c) return
    const ctx = c.getContext('2d'); if (!ctx) return
    c.width = 200; c.height = 80
    ctx.fillStyle = '#1a1a1a'; ctx.fillRect(0, 0, 200, 80)
    ctx.fillStyle = '#f87171'; ctx.font = '12px monospace'
    ctx.fillText('image load failed', 10, 30)
    ctx.fillText(spec.path, 10, 50)
  }

  return <canvas ref={canvasRef} className="block bg-[#1a1a1a]" />
}
```

- [ ] **Step 4: Run slicing tests, confirm pass**

Run: `cd tools/editor-web && npm test -- src/components/__tests__/SpriteCanvas.test.tsx`
Expected: PASS — 3 tests (render-loop covered manually).

- [ ] **Step 5: Commit**

```bash
git add tools/editor-web/src/components/SpriteCanvas.tsx tools/editor-web/src/components/__tests__/SpriteCanvas.test.tsx
git commit -m "editor-web: SpriteCanvas with grid/strip slicing"
```

---

### Task 10: `SpriteList` component

**Files:**
- Create: `tools/editor-web/src/components/SpriteList.tsx`

- [ ] **Step 1: Implement the list**

Create `tools/editor-web/src/components/SpriteList.tsx`:

```tsx
import { useSpriteStore } from '../state/spriteStore'

export function SpriteList() {
  const anims = useSpriteStore(s => s.anims)
  const selectedId = useSpriteStore(s => s.selectedId)
  const selectById = useSpriteStore(s => s.selectById)
  if (anims.length === 0) {
    return <div className="p-2 text-xs text-muted-foreground">no animations</div>
  }
  return (
    <ul className="text-xs">
      {anims.map(a => (
        <li key={a.id}>
          <button
            onClick={() => selectById(a.id)}
            className={
              'w-full text-left px-2 py-1 rounded ' +
              (a.id === selectedId ? 'bg-accent text-accent-foreground' : 'hover:bg-muted')
            }
          >
            {a.id}
          </button>
        </li>
      ))}
    </ul>
  )
}
```

- [ ] **Step 2: Verify it type-checks**

Run: `cd tools/editor-web && npx tsc --noEmit`
Expected: succeeds.

- [ ] **Step 3: Commit**

```bash
git add tools/editor-web/src/components/SpriteList.tsx
git commit -m "editor-web: SpriteList renders anims with selection state"
```

---

### Task 11: `SpriteCanvasPanel` (canvas + transport bar)

**Files:**
- Create: `tools/editor-web/src/components/SpriteCanvasPanel.tsx`

- [ ] **Step 1: Implement the panel**

Create `tools/editor-web/src/components/SpriteCanvasPanel.tsx`:

```tsx
import { useSpriteStore } from '../state/spriteStore'
import { SpriteCanvas } from './SpriteCanvas'
import { Button } from '@/components/ui/button'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'

export function SpriteCanvasPanel() {
  const anims = useSpriteStore(s => s.anims)
  const selectedId = useSpriteStore(s => s.selectedId)
  const frame = useSpriteStore(s => s.frame)
  const playing = useSpriteStore(s => s.playing)
  const togglePlay = useSpriteStore(s => s.togglePlay)
  const scrub = useSpriteStore(s => s.scrub)
  const advanceFrame = useSpriteStore(s => s.advanceFrame)

  const spec = anims.find(a => a.id === selectedId)
  if (!spec) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>Select an animation</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  }
  const fps = Math.round(spec.frame_count * 1000 / spec.duration_ms)
  return (
    <div className="flex flex-col gap-2 p-2 flex-1 min-h-0">
      <div className="flex-1 flex items-center justify-center min-h-0 overflow-hidden">
        <SpriteCanvas
          spec={spec}
          frame={frame}
          playing={playing}
          onAdvance={advanceFrame}
        />
      </div>
      <div className="flex items-center gap-2 text-xs">
        <Button size="sm" variant="outline" onClick={togglePlay}>{playing ? '⏸' : '▶'}</Button>
        <input
          type="range"
          min={0}
          max={spec.frame_count - 1}
          value={frame}
          onChange={e => scrub(Number(e.target.value))}
          className="flex-1"
        />
        <span className="text-muted-foreground tabular-nums">
          {frame + 1}/{spec.frame_count} · {fps}fps
        </span>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Verify type-check**

Run: `cd tools/editor-web && npx tsc --noEmit`
Expected: succeeds.

- [ ] **Step 3: Commit**

```bash
git add tools/editor-web/src/components/SpriteCanvasPanel.tsx
git commit -m "editor-web: SpriteCanvasPanel with play/scrub/fps readout"
```

---

### Task 12: `SpritePane` composition + auto-sync (TDD)

**Files:**
- Create: `tools/editor-web/src/components/SpritePane.tsx`
- Create: `tools/editor-web/src/components/__tests__/SpritePane.test.tsx`

- [ ] **Step 1: Write failing auto-sync test**

Create `tools/editor-web/src/components/__tests__/SpritePane.test.tsx`:

```tsx
import { describe, it, expect, beforeEach, beforeAll, afterAll, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { setupServer } from 'msw/node'
import { http, HttpResponse } from 'msw'
import { SpritePane } from '../SpritePane'
import { __resetSpriteStore, useSpriteStore } from '../../state/spriteStore'

const animsPayload = [
  { id: 'orc_idle', path: 'assets/sprites/orc/Idle.png',
    frame_w: 100, frame_h: 100, frame_count: 6, duration_ms: 600, loop: true,
    grid_cols: 6, grid_rows: 1, pick_row: 0, pick_col: 0 },
  { id: 'orc_run', path: 'assets/sprites/orc/Run.png',
    frame_w: 100, frame_h: 100, frame_count: 8, duration_ms: 800, loop: true,
    grid_cols: 8, grid_rows: 1, pick_row: 0, pick_col: 0 },
]

const server = setupServer(
  http.get('/api/anims/orc', () => HttpResponse.json(animsPayload)),
)

beforeAll(() => server.listen())
afterEach(() => server.resetHandlers())
afterAll(() => server.close())
beforeEach(() => __resetSpriteStore())

describe('SpritePane', () => {
  it('loads anims for kind and renders list', async () => {
    render(<SpritePane kind="orc" selectedAnim={null} />)
    await waitFor(() => expect(screen.getByText('orc_idle')).toBeInTheDocument())
    expect(screen.getByText('orc_run')).toBeInTheDocument()
  })

  it('auto-selects spec.anim when selectedAnim prop changes', async () => {
    const { rerender } = render(<SpritePane kind="orc" selectedAnim={null} />)
    await waitFor(() => expect(useSpriteStore.getState().anims).toHaveLength(2))
    rerender(<SpritePane kind="orc" selectedAnim="orc_run" />)
    await waitFor(() => expect(useSpriteStore.getState().selectedId).toBe('orc_run'))
    expect(useSpriteStore.getState().frame).toBe(0)
    expect(useSpriteStore.getState().playing).toBe(true)
  })

  it('ignores selectedAnim that does not exist in registry', async () => {
    const { rerender } = render(<SpritePane kind="orc" selectedAnim={null} />)
    await waitFor(() => expect(useSpriteStore.getState().anims).toHaveLength(2))
    rerender(<SpritePane kind="orc" selectedAnim="orc_unknown" />)
    // selectedId remains null because store.selectById guards on existence
    expect(useSpriteStore.getState().selectedId).toBeNull()
  })
})
```

- [ ] **Step 2: Run, confirm fails**

Run: `cd tools/editor-web && npm test -- src/components/__tests__/SpritePane.test.tsx`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement `SpritePane`**

Create `tools/editor-web/src/components/SpritePane.tsx`:

```tsx
import { useEffect } from 'react'
import { fetchAnims } from '../api/sprites'
import { useSpriteStore } from '../state/spriteStore'
import { SpriteList } from './SpriteList'
import { SpriteCanvasPanel } from './SpriteCanvasPanel'

interface Props {
  kind: string | null
  selectedAnim: string | null
}

export function SpritePane({ kind, selectedAnim }: Props) {
  const setAnims = useSpriteStore(s => s.setAnims)
  const selectById = useSpriteStore(s => s.selectById)

  useEffect(() => {
    if (!kind) { setAnims([]); return }
    let cancelled = false
    fetchAnims(kind).then(rows => {
      if (!cancelled) setAnims(rows)
    }).catch(() => {
      if (!cancelled) setAnims([])
    })
    return () => { cancelled = true }
  }, [kind, setAnims])

  useEffect(() => {
    if (selectedAnim) selectById(selectedAnim)
  }, [selectedAnim, selectById])

  if (!kind) {
    return <div className="p-3 text-xs text-muted-foreground">no kind selected</div>
  }
  return (
    <div className="flex h-full">
      <div className="w-[110px] border-r border-border overflow-y-auto">
        <SpriteList />
      </div>
      <div className="flex-1 flex flex-col min-w-0">
        <SpriteCanvasPanel />
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Run, confirm passes**

Run: `cd tools/editor-web && npm test -- src/components/__tests__/SpritePane.test.tsx`
Expected: PASS — 3 tests.

- [ ] **Step 5: Commit**

```bash
git add tools/editor-web/src/components/SpritePane.tsx tools/editor-web/src/components/__tests__/SpritePane.test.tsx
git commit -m "editor-web: SpritePane composition + auto-sync to selectedAnim"
```

---

### Task 13: Mount Sprite tab in `Inspector.tsx`

**Files:**
- Modify: `tools/editor-web/src/components/Inspector.tsx`

- [ ] **Step 1: Add tab and content**

Open `tools/editor-web/src/components/Inspector.tsx`. At the top, add:

```ts
import { SpritePane } from './SpritePane'
```

Find the `Tab` type alias and replace it:

```ts
type Tab = 'node' | 'state' | 'json' | 'sprite'
```

Inside the `Inspector` function, locate the line:

```ts
const selectedNodePath = useEditorStore(s => s.selectedNodePath)
```

Replace with:

```ts
const selectedNodePath = useEditorStore(s => s.selectedNodePath)
const currentKind = useEditorStore(s => s.currentKind)
const behavior = useEditorStore(s => s.behavior)
const selectedStateId = useEditorStore(s => s.selectedStateId)
const selectedAnim = behavior?.states.find(s => s.id === selectedStateId)?.anim ?? null
```

Inside `<TabsList>`, add a new trigger after the JSON one:

```tsx
<TabsTrigger value="sprite" className="rounded-none">Sprite</TabsTrigger>
```

After the existing `<TabsContent value="json">` block, add:

```tsx
<TabsContent value="sprite" className="flex-1 min-h-0 p-0">
  <SpritePane kind={currentKind} selectedAnim={selectedAnim} />
</TabsContent>
```

- [ ] **Step 2: Verify type-check**

Run: `cd tools/editor-web && npx tsc --noEmit`
Expected: succeeds.

- [ ] **Step 3: Run all FE tests**

Run: `cd tools/editor-web && npm test`
Expected: all suites pass.

- [ ] **Step 4: Manual smoke test**

In one terminal: `make editor`
In another: `make web`
Open `http://localhost:5173`, load orc kind. Click `Sprite` tab.
Expected:
- list shows `orc_idle`, `orc_run`, `orc_attack`, `orc_attack2`, `orc_hurt`, `orc_death`
- canvas plays the anim that matches the currently selected state
- click another anim in list → preview switches, plays
- click play/pause toggles RAF loop
- drag scrub slider → frame jumps, pause flips off

Stop both servers.

- [ ] **Step 5: Commit**

```bash
git add tools/editor-web/src/components/Inspector.tsx
git commit -m "editor-web: mount Sprite tab in Inspector with selectedAnim sync"
```

---

## Final verification

### Task 14: Full test sweep + go vet

**Files:** none

- [ ] **Step 1: Backend**

Run: `go test ./...`
Expected: all green.

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 2: Frontend**

Run: `cd tools/editor-web && npm test`
Expected: all suites pass.

Run: `cd tools/editor-web && npm run build`
Expected: build succeeds.

- [ ] **Step 3: Update CLAUDE.md if needed**

If the editor endpoint list in `CLAUDE.md` mentions `/api/anims`, leave alone. If not, append under the Editor server section:

```markdown
- `GET    /assets/*`               serves PNG sheets + behaviors (read-only)
- `GET    /api/anims/:kind`        list animation specs for kind (enemy only)
```

Commit if changed:

```bash
git add CLAUDE.md
git commit -m "docs: add /api/anims and /assets to editor endpoints"
```

---

## Out of scope (future work)

Per spec:

- Editing animation specs
- Hitbox overlay (separate spec [docs/superpowers/specs/2026-04-30-hitbox-visual-editor-design.md](../superpowers/specs/2026-04-30-hitbox-visual-editor-design.md))
- fps override slider, render-scale toggle, loop toggle
- Player anim previews
- Multi-anim comparison
