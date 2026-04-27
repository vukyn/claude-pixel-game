# Behavior Visual Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a React Flow web editor + Go Fiber API server for editing `assets/behaviors/*.json` and tuning rows, reusing the existing behavior loader as the single source of truth for validation.

**Architecture:** Three independent processes — game (`cmd/game`, untouched), editor server (`cmd/editor`, light hexagonal: handler → service → port → adapter), FE dev server (`tools/editor-web`, Vite + React + TS + Tailwind + React Flow + Zustand). Game keeps existing F5 manual reload; saving in editor does not push to game.

**Tech Stack:**
- BE: Go 1.26, [Fiber v2](https://github.com/gofiber/fiber), reuse `internal/behavior` + `internal/storage`
- FE: Vite, React 18, TypeScript, React Flow, Tailwind, Zustand, dagre, zod
- Test: `go test`, `vitest`, `@testing-library/react`, Playwright (manual)
- Skills to consult during FE tasks: `react-best-practices`, `composition-patterns`, `web-design-guidelines`

**Spec:** `docs/superpowers/specs/2026-04-26-behavior-visual-editor-design.md`

---

## File map

**New BE files:**
- `cmd/editor/main.go` — Fiber boot
- `internal/editor/http/handler.go` — route handlers
- `internal/editor/http/middleware.go` — CORS + log
- `internal/editor/service/behavior.go` — Behavior use cases
- `internal/editor/service/tuning.go` — Tuning use case
- `internal/editor/service/registry.go` — Registry use case
- `internal/editor/port/repository.go` — port interfaces
- `internal/editor/adapter/fsbehavior.go` — FS adapter for behaviors
- `internal/editor/adapter/sqlitetuning.go` — sqlite adapter for tuning
- `internal/editor/adapter/runtimeregistry.go` — registry introspection adapter

**New BE tests:** parallel `*_test.go` per file above.

**Modified BE files:**
- `internal/behavior/loader.go` — add `LoadBytes` helper (path loader becomes thin wrapper)
- `internal/behavior/registry.go` — add `ActionMeta` / `ConditionMeta` + `RegisterActionWithMeta` / `RegisterConditionWithMeta` + `RegisteredActions()` / `RegisteredConditions()` (existing `RegisterAction` / `RegisterCondition` keep working with empty meta for backwards-compat in tests)
- `internal/behavior/registry.go` (init function) — convert each built-in registration to use the meta-aware variant with arg schema
- `.env.example` — add `EDITOR_PORT=8080`
- `Makefile` — add `make editor` target
- `CLAUDE.md` — add "Editor server" section + "Frontend (tools/editor-web)" section

**New FE files (under `tools/editor-web/`):**
- `package.json`, `vite.config.ts`, `tailwind.config.ts`, `postcss.config.js`, `tsconfig.json`, `index.html`, `.gitignore`
- `src/main.tsx`, `src/App.tsx`, `src/index.css`
- `src/api/client.ts`, `src/api/schemas.ts`
- `src/state/editorStore.ts`
- `src/components/TopBar.tsx`, `StatesPanel.tsx`, `BTCanvas.tsx`, `Inspector.tsx`, `TuningDrawer.tsx`
- `src/bt/mapping.ts`, `validation.ts`, `layout.ts`, `types.ts`
- `src/bt/nodes/SelectorNode.tsx`, `SequenceNode.tsx`, `ChanceNode.tsx`, `ActionNode.tsx`, `ConditionNode.tsx`, `WaitNode.tsx`
- `src/bt/__tests__/mapping.test.ts`, `validation.test.ts`, `layout.test.ts`
- `src/state/__tests__/editorStore.test.ts`
- `src/api/__tests__/client.test.ts`
- `tests/e2e/editor.spec.ts` (Playwright, manual run)

---

## Phase 0 — Reusable behavior helpers (BE prereq)

### Task 0.1: `LoadBytes` helper in behavior loader

**Files:**
- Modify: `internal/behavior/loader.go:15-25`
- Test: `internal/behavior/loader_test.go` (append)

- [ ] **Step 1: Write failing test**

Append to `internal/behavior/loader_test.go`:

```go
func TestLoadBytes_ParsesValidJSONWithoutFile(t *testing.T) {
	raw := []byte(`{"kind":"orc","states":[{"id":"idle","anim":"idle","decision":false,"exit_on":"anim_done","next":"idle"}]}`)
	f, err := behavior.LoadBytes(raw, "in-memory.json")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if f.Kind != "orc" || len(f.States) != 1 {
		t.Fatalf("unexpected file: %+v", f)
	}
}

func TestLoadBytes_PropagatesParseError(t *testing.T) {
	_, err := behavior.LoadBytes([]byte("{bad"), "broken.json")
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
```

- [ ] **Step 2: Run test to confirm fail**

```bash
go test ./internal/behavior/ -run LoadBytes -v
```
Expected: `FAIL` — `behavior.LoadBytes undefined`.

- [ ] **Step 3: Refactor loader**

Replace `internal/behavior/loader.go:15-25` with:

```go
// LoadFile parses a behavior JSON file by path. Convenience wrapper around LoadBytes.
func LoadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("behavior: read %s: %w", path, err)
	}
	return LoadBytes(data, path)
}

// LoadBytes parses a behavior JSON document. `source` is used only for error messages.
func LoadBytes(data []byte, source string) (*File, error) {
	var raw FileRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("behavior: parse %s: %w", source, err)
	}
	return buildFile(&raw, source)
}
```

- [ ] **Step 4: Run all behavior tests**

```bash
go test ./internal/behavior/ -v
```
Expected: PASS (existing + new tests).

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/loader.go internal/behavior/loader_test.go
git commit -m "refactor(behavior): expose LoadBytes for in-memory parses"
```

### Task 0.2: Action/Condition metadata + introspection

**Files:**
- Modify: `internal/behavior/registry.go` (top + init)
- Test: `internal/behavior/registry_test.go` (append)

- [ ] **Step 1: Write failing test**

Append to `internal/behavior/registry_test.go`:

```go
func TestRegisteredActions_ReturnsBuiltinsWithArgSchema(t *testing.T) {
	metas := behavior.RegisteredActions()
	byName := map[string]behavior.ActionMeta{}
	for _, m := range metas {
		byName[m.Name] = m
	}
	goto_, ok := byName["goto"]
	if !ok {
		t.Fatal("goto not registered")
	}
	if len(goto_.Args) != 1 || goto_.Args[0].Name != "state" || goto_.Args[0].Type != "state_id" || !goto_.Args[0].Required {
		t.Fatalf("goto arg schema unexpected: %+v", goto_.Args)
	}
	setVx, ok := byName["set_vx_forward"]
	if !ok {
		t.Fatal("set_vx_forward not registered")
	}
	if len(setVx.Args) != 1 || setVx.Args[0].Type != "float" {
		t.Fatalf("set_vx_forward arg schema unexpected: %+v", setVx.Args)
	}
}

func TestRegisteredConditions_ReturnsBuiltinsWithArgSchema(t *testing.T) {
	metas := behavior.RegisteredConditions()
	byName := map[string]behavior.ActionMeta{}
	for _, m := range metas {
		byName[m.Name] = m
	}
	g, ok := byName["grounded"]
	if !ok {
		t.Fatal("grounded not registered")
	}
	if len(g.Args) != 0 {
		t.Fatalf("grounded should have no args: %+v", g.Args)
	}
	frame, ok := byName["anim_frame_ge"]
	if !ok {
		t.Fatal("anim_frame_ge not registered")
	}
	if len(frame.Args) != 1 || frame.Args[0].Type != "int" {
		t.Fatalf("anim_frame_ge arg schema unexpected: %+v", frame.Args)
	}
}
```

- [ ] **Step 2: Run test to confirm fail**

```bash
go test ./internal/behavior/ -run Registered -v
```
Expected: FAIL — types not defined.

- [ ] **Step 3: Add meta types + introspection**

Edit `internal/behavior/registry.go` — replace the `var (...)` block at the top with:

```go
// ArgMeta describes one argument to an action or condition.
type ArgMeta struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "int" | "float" | "string" | "state_id" | "anim_key"
	Required bool   `json:"required"`
}

// ActionMeta describes a registered action or condition for editor introspection.
type ActionMeta struct {
	Name string    `json:"name"`
	Args []ArgMeta `json:"args"`
}

var (
	actions       = map[string]ActionFn{}
	conditions    = map[string]ConditionFn{}
	actionMetas   = map[string]ActionMeta{}
	conditionMeta = map[string]ActionMeta{}
)
```

Below the existing `RegisterAction` / `RegisterCondition`, add:

```go
// RegisterActionWithMeta registers fn under name and records its arg schema for editor introspection.
func RegisterActionWithMeta(name string, args []ArgMeta, fn ActionFn) {
	RegisterAction(name, fn)
	actionMetas[name] = ActionMeta{Name: name, Args: args}
}

// RegisterConditionWithMeta registers fn under name and records its arg schema for editor introspection.
func RegisterConditionWithMeta(name string, args []ArgMeta, fn ConditionFn) {
	RegisterCondition(name, fn)
	conditionMeta[name] = ActionMeta{Name: name, Args: args}
}

// RegisteredActions returns metadata for every registered action, sorted by name.
func RegisteredActions() []ActionMeta { return sortedMetas(actionMetas) }

// RegisteredConditions returns metadata for every registered condition, sorted by name.
func RegisteredConditions() []ActionMeta { return sortedMetas(conditionMeta) }

func sortedMetas(m map[string]ActionMeta) []ActionMeta {
	out := make([]ActionMeta, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
```

Add `"sort"` to the imports.

- [ ] **Step 4: Convert built-in registrations to use meta variant**

In the `init()` of `internal/behavior/registry.go`, replace each `RegisterAction(...)` / `RegisterCondition(...)` call with the `WithMeta` variant. Example:

```go
RegisterActionWithMeta("goto",
	[]ArgMeta{{Name: "state", Type: "state_id", Required: true}},
	func(args map[string]any, ctx *Ctx) (Status, error) {
		s, err := argString(args, "state")
		if err != nil { return StatusFailure, err }
		ctx.PendingGoto = s
		return StatusSuccess, nil
	},
)
RegisterActionWithMeta("flip_facing", nil, func(_ map[string]any, ctx *Ctx) (Status, error) {
	ctx.Enemy.SetFacing(-ctx.Enemy.Facing())
	return StatusSuccess, nil
})
RegisterActionWithMeta("randomize_facing", nil, func(_ map[string]any, ctx *Ctx) (Status, error) {
	if ctx.RNG.Intn(2) == 0 { ctx.Enemy.SetFacing(1) } else { ctx.Enemy.SetFacing(-1) }
	return StatusSuccess, nil
})
RegisterActionWithMeta("set_vx_forward",
	[]ArgMeta{{Name: "speed", Type: "float", Required: true}},
	func(args map[string]any, ctx *Ctx) (Status, error) {
		speed, err := argFloat(args, "speed")
		if err != nil { return StatusFailure, err }
		ctx.Enemy.SetVX(float64(ctx.Enemy.Facing()) * speed)
		return StatusSuccess, nil
	},
)
RegisterActionWithMeta("stop", nil, func(_ map[string]any, ctx *Ctx) (Status, error) {
	ctx.Enemy.SetVX(0)
	return StatusSuccess, nil
})
RegisterActionWithMeta("play_anim",
	[]ArgMeta{{Name: "key", Type: "anim_key", Required: true}},
	func(args map[string]any, ctx *Ctx) (Status, error) {
		key, err := argString(args, "key")
		if err != nil { return StatusFailure, err }
		ctx.Enemy.PlayAnim(key)
		return StatusSuccess, nil
	},
)

RegisterConditionWithMeta("grounded", nil, func(_ map[string]any, ctx *Ctx) (bool, error) {
	return ctx.Enemy.Grounded(), nil
})
RegisterConditionWithMeta("anim_done", nil, func(_ map[string]any, ctx *Ctx) (bool, error) {
	return ctx.Enemy.CurrentAnimDone(), nil
})
RegisterConditionWithMeta("anim_frame_ge",
	[]ArgMeta{{Name: "frame", Type: "int", Required: true}},
	func(args map[string]any, ctx *Ctx) (bool, error) {
		f, err := argFloat(args, "frame")
		if err != nil { return false, err }
		return ctx.Enemy.CurrentAnimFrame() >= int(f), nil
	},
)
RegisterConditionWithMeta("anim_frame_le",
	[]ArgMeta{{Name: "frame", Type: "int", Required: true}},
	func(args map[string]any, ctx *Ctx) (bool, error) {
		f, err := argFloat(args, "frame")
		if err != nil { return false, err }
		return ctx.Enemy.CurrentAnimFrame() <= int(f), nil
	},
)
```

- [ ] **Step 5: Run all behavior tests**

```bash
go test ./internal/behavior/ -v
```
Expected: PASS (new + existing).

- [ ] **Step 6: Commit**

```bash
git add internal/behavior/registry.go internal/behavior/registry_test.go
git commit -m "feat(behavior): action/condition metadata for editor introspection"
```

---

## Phase 1 — Backend (Go Fiber editor server)

### Task 1.1: Add Fiber dependency + skeleton `cmd/editor/main.go`

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `cmd/editor/main.go`

- [ ] **Step 1: Add Fiber**

```bash
go get github.com/gofiber/fiber/v2@latest
```
Expected: `go: added github.com/gofiber/fiber/v2 ...`

- [ ] **Step 2: Create skeleton main**

Create `cmd/editor/main.go`:

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	port := os.Getenv("EDITOR_PORT")
	if port == "" {
		port = "8080"
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})
	addr := ":" + port
	log.Printf("editor server listening on %s", addr)
	if err := app.Listen(addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Verify it builds**

```bash
go build ./cmd/editor
```
Expected: no output.

- [ ] **Step 4: Smoke run**

```bash
EDITOR_PORT=8090 go run ./cmd/editor &
sleep 1
curl -s http://localhost:8090/api/health
kill %1 2>/dev/null
```
Expected: `{"ok":true}`.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum cmd/editor/main.go
git commit -m "feat(editor): scaffold Fiber server with /api/health"
```

### Task 1.2: Port interfaces

**Files:**
- Create: `internal/editor/port/repository.go`

- [ ] **Step 1: Write file**

```go
package port

import "claude-pixel/internal/behavior"

// BehaviorStore owns persistence of behavior JSON files.
type BehaviorStore interface {
	List() ([]BehaviorRef, error)
	Get(kind string) ([]byte, error)         // raw JSON bytes
	Put(kind string, raw []byte) error       // atomic write
}

type BehaviorRef struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	StateCount int    `json:"state_count"`
}

// TuningStore owns tuning rows with range validation.
type TuningStore interface {
	List(prefix string) ([]TuningRow, error)
	Update(key string, value float64) (oldValue float64, err error)
}

type TuningRow struct {
	Key         string  `json:"key"`
	Value       float64 `json:"value"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Unit        string  `json:"unit"`
	Description string  `json:"description"`
}

// RegistryStore exposes the behavior package's action/condition catalog.
type RegistryStore interface {
	Actions() []behavior.ActionMeta
	Conditions() []behavior.ActionMeta
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/editor/port
```
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/editor/port/repository.go
git commit -m "feat(editor): port interfaces for behavior/tuning/registry"
```

### Task 1.3: FS adapter for behaviors

**Files:**
- Create: `internal/editor/adapter/fsbehavior.go`
- Test: `internal/editor/adapter/fsbehavior_test.go`

- [ ] **Step 1: Write failing tests**

```go
package adapter_test

import (
	"os"
	"path/filepath"
	"testing"

	"claude-pixel/internal/editor/adapter"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFSBehavior_ListReturnsKnownKinds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "orc.json"), `{"kind":"orc","states":[{"id":"a","anim":"idle","decision":false,"exit_on":"anim_done","next":"a"}]}`)
	writeFile(t, filepath.Join(dir, "slime.json"), `{"kind":"slime","states":[{"id":"a","anim":"idle","decision":false,"exit_on":"anim_done","next":"a"},{"id":"b","anim":"run","decision":false,"exit_on":"anim_done","next":"a"}]}`)
	writeFile(t, filepath.Join(dir, "README.md"), `should be ignored`)

	s := adapter.NewFSBehavior(dir)
	refs, err := s.List()
	if err != nil { t.Fatal(err) }
	if len(refs) != 2 { t.Fatalf("want 2 refs, got %d (%+v)", len(refs), refs) }
	byKind := map[string]int{}
	for _, r := range refs { byKind[r.Kind] = r.StateCount }
	if byKind["orc"] != 1 || byKind["slime"] != 2 {
		t.Fatalf("unexpected counts: %+v", byKind)
	}
}

func TestFSBehavior_GetReturnsBytes(t *testing.T) {
	dir := t.TempDir()
	body := `{"kind":"orc","states":[{"id":"a","anim":"idle","decision":false,"exit_on":"anim_done","next":"a"}]}`
	writeFile(t, filepath.Join(dir, "orc.json"), body)
	s := adapter.NewFSBehavior(dir)
	got, err := s.Get("orc")
	if err != nil { t.Fatal(err) }
	if string(got) != body { t.Fatalf("body mismatch:\nwant %s\ngot  %s", body, got) }
}

func TestFSBehavior_GetNotFound(t *testing.T) {
	s := adapter.NewFSBehavior(t.TempDir())
	if _, err := s.Get("ghost"); err == nil { t.Fatal("want error, got nil") }
}

func TestFSBehavior_PutAtomicAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "orc.json")
	writeFile(t, target, `original`)
	s := adapter.NewFSBehavior(dir)
	if err := s.Put("orc", []byte(`updated`)); err != nil { t.Fatal(err) }
	got, _ := os.ReadFile(target)
	if string(got) != "updated" { t.Fatalf("want updated, got %s", got) }
	// no .tmp left over
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" { t.Fatalf("temp file leaked: %s", e.Name()) }
	}
}

func TestFSBehavior_RejectsKindWithPathSeparators(t *testing.T) {
	s := adapter.NewFSBehavior(t.TempDir())
	if _, err := s.Get("../etc/passwd"); err == nil { t.Fatal("want error, got nil") }
	if err := s.Put("a/b", []byte(`x`)); err == nil { t.Fatal("want error, got nil") }
}
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/editor/adapter/ -run FSBehavior -v
```
Expected: FAIL — package missing.

- [ ] **Step 3: Implement**

Create `internal/editor/adapter/fsbehavior.go`:

```go
package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"claude-pixel/internal/editor/port"
)

type FSBehavior struct{ dir string }

func NewFSBehavior(dir string) *FSBehavior { return &FSBehavior{dir: dir} }

func (s *FSBehavior) List() ([]port.BehaviorRef, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("fsbehavior: list %s: %w", s.dir, err)
	}
	var refs []port.BehaviorRef
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil { return nil, err }
		var head struct {
			Kind   string `json:"kind"`
			States []any  `json:"states"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			continue // skip unparseable
		}
		if head.Kind == "" { continue }
		refs = append(refs, port.BehaviorRef{Kind: head.Kind, Path: path, StateCount: len(head.States)})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Kind < refs[j].Kind })
	return refs, nil
}

func (s *FSBehavior) Get(kind string) ([]byte, error) {
	if err := validateKind(kind); err != nil { return nil, err }
	path := filepath.Join(s.dir, kind+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("fsbehavior: kind %q not found", kind)
		}
		return nil, fmt.Errorf("fsbehavior: read %q: %w", kind, err)
	}
	return data, nil
}

func (s *FSBehavior) Put(kind string, raw []byte) error {
	if err := validateKind(kind); err != nil { return err }
	path := filepath.Join(s.dir, kind+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("fsbehavior: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("fsbehavior: rename: %w", err)
	}
	return nil
}

func validateKind(kind string) error {
	if kind == "" || strings.ContainsAny(kind, `/\.`) {
		return fmt.Errorf("fsbehavior: invalid kind %q", kind)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/editor/adapter/ -run FSBehavior -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/editor/adapter/fsbehavior.go internal/editor/adapter/fsbehavior_test.go
git commit -m "feat(editor): FS adapter for behavior JSON files"
```

### Task 1.4: SQLite adapter for tuning

**Files:**
- Create: `internal/editor/adapter/sqlitetuning.go`
- Test: `internal/editor/adapter/sqlitetuning_test.go`

- [ ] **Step 1: Write failing tests**

```go
package adapter_test

import (
	"strings"
	"testing"

	"claude-pixel/internal/editor/adapter"
	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func newTuningRepo(t *testing.T) *storage.Repository[player.TuningParam] {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil { t.Fatalf("open: %v", err) }
	t.Cleanup(func() { db.Close() })
	if err := storage.Migrate(db); err != nil { t.Fatalf("migrate: %v", err) }
	return storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
}

func TestSQLiteTuning_ListWithPrefix(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	rows, err := a.List("orc")
	if err != nil { t.Fatal(err) }
	for _, r := range rows {
		if !strings.HasPrefix(r.Key, "orc_") {
			t.Fatalf("row %q does not match prefix orc_", r.Key)
		}
	}
	if len(rows) == 0 { t.Fatal("expected at least one orc_* row from seed") }
}

func TestSQLiteTuning_UpdateWithinRange(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	old, err := a.Update("orc_max_lives", 5)
	if err != nil { t.Fatal(err) }
	if old == 5 { t.Fatalf("old value should differ from new (was %v)", old) }
	rows, _ := a.List("orc")
	for _, r := range rows {
		if r.Key == "orc_max_lives" && r.Value != 5 {
			t.Fatalf("value not persisted: %v", r.Value)
		}
	}
}

func TestSQLiteTuning_UpdateOutOfRangeRejected(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	_, err := a.Update("orc_max_lives", 9_999_999)
	if err == nil { t.Fatal("want error, got nil") }
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("err should mention out of range, got %v", err)
	}
}

func TestSQLiteTuning_UpdateUnknownKey(t *testing.T) {
	repo := newTuningRepo(t)
	a := adapter.NewSQLiteTuning(repo)
	_, err := a.Update("ghost_key", 1)
	if err == nil { t.Fatal("want error, got nil") }
}

var _ port.TuningStore = (*adapter.SQLiteTuning)(nil)
```

- [ ] **Step 2: Run to confirm fail**

```bash
go test ./internal/editor/adapter/ -run SQLiteTuning -v
```
Expected: FAIL — type missing.

- [ ] **Step 3: Implement**

Create `internal/editor/adapter/sqlitetuning.go`:

```go
package adapter

import (
	"context"
	"fmt"
	"strings"

	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

type SQLiteTuning struct{ repo *storage.Repository[player.TuningParam] }

func NewSQLiteTuning(repo *storage.Repository[player.TuningParam]) *SQLiteTuning {
	return &SQLiteTuning{repo: repo}
}

func (s *SQLiteTuning) List(prefix string) ([]port.TuningRow, error) {
	all, err := s.repo.List(context.Background())
	if err != nil { return nil, fmt.Errorf("sqlitetuning: list: %w", err) }
	var out []port.TuningRow
	for _, t := range all {
		if prefix != "" && !strings.HasPrefix(t.Key, prefix+"_") {
			continue
		}
		out = append(out, port.TuningRow{
			Key: t.Key, Value: t.Value,
			Min: t.MinValue, Max: t.MaxValue,
			Unit: t.Unit, Description: t.Description,
		})
	}
	return out, nil
}

func (s *SQLiteTuning) Update(key string, value float64) (float64, error) {
	current, err := s.repo.Get(context.Background(), key)
	if err != nil { return 0, fmt.Errorf("sqlitetuning: get %q: %w", key, err) }
	if current == nil { return 0, fmt.Errorf("sqlitetuning: unknown key %q", key) }
	if value < current.MinValue || value > current.MaxValue {
		return current.Value, fmt.Errorf("sqlitetuning: value out of range: %v not in [%v, %v] %s",
			value, current.MinValue, current.MaxValue, current.Unit)
	}
	old := current.Value
	current.Value = value
	if err := s.repo.Update(context.Background(), *current); err != nil {
		return old, fmt.Errorf("sqlitetuning: update: %w", err)
	}
	return old, nil
}
```

> Note: if `storage.Repository[T]` Get/List signatures differ in the codebase, adjust the calls. Verify by reading `internal/storage/repository.go` before coding. The intent is unchanged: read row, range-check, write back via existing repo helpers.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/editor/adapter/ -run SQLiteTuning -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/editor/adapter/sqlitetuning.go internal/editor/adapter/sqlitetuning_test.go
git commit -m "feat(editor): sqlite adapter for tuning rows with range guard"
```

### Task 1.5: Runtime registry adapter

**Files:**
- Create: `internal/editor/adapter/runtimeregistry.go`
- Test: `internal/editor/adapter/runtimeregistry_test.go`

- [ ] **Step 1: Write failing tests**

```go
package adapter_test

import (
	"testing"

	"claude-pixel/internal/editor/adapter"
)

func TestRuntimeRegistry_ExposesActions(t *testing.T) {
	r := adapter.NewRuntimeRegistry()
	got := r.Actions()
	if len(got) == 0 { t.Fatal("expected actions") }
	for _, a := range got {
		if a.Name == "" { t.Fatalf("action with empty name: %+v", a) }
	}
}

func TestRuntimeRegistry_ExposesConditions(t *testing.T) {
	r := adapter.NewRuntimeRegistry()
	if len(r.Conditions()) == 0 { t.Fatal("expected conditions") }
}
```

- [ ] **Step 2: Run fail**

```bash
go test ./internal/editor/adapter/ -run RuntimeRegistry -v
```
Expected: FAIL.

- [ ] **Step 3: Implement**

Create `internal/editor/adapter/runtimeregistry.go`:

```go
package adapter

import "claude-pixel/internal/behavior"

type RuntimeRegistry struct{}

func NewRuntimeRegistry() *RuntimeRegistry { return &RuntimeRegistry{} }

func (RuntimeRegistry) Actions() []behavior.ActionMeta    { return behavior.RegisteredActions() }
func (RuntimeRegistry) Conditions() []behavior.ActionMeta { return behavior.RegisteredConditions() }
```

- [ ] **Step 4: Run + commit**

```bash
go test ./internal/editor/adapter/ -run RuntimeRegistry -v
git add internal/editor/adapter/runtimeregistry.go internal/editor/adapter/runtimeregistry_test.go
git commit -m "feat(editor): runtime registry adapter introspecting behavior pkg"
```

### Task 1.6: Behavior service

**Files:**
- Create: `internal/editor/service/behavior.go`
- Test: `internal/editor/service/behavior_test.go`

- [ ] **Step 1: Write failing tests**

```go
package service_test

import (
	"errors"
	"strings"
	"testing"

	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/editor/service"
)

type fakeBehaviorStore struct {
	listFn func() ([]port.BehaviorRef, error)
	getFn  func(string) ([]byte, error)
	putFn  func(string, []byte) error
}

func (f fakeBehaviorStore) List() ([]port.BehaviorRef, error)        { return f.listFn() }
func (f fakeBehaviorStore) Get(k string) ([]byte, error)             { return f.getFn(k) }
func (f fakeBehaviorStore) Put(k string, raw []byte) error           { return f.putFn(k, raw) }

const validOrc = `{"kind":"orc","states":[{"id":"idle","anim":"idle","decision":false,"exit_on":"anim_done","next":"idle"}]}`

func TestBehaviorService_ListPassesThrough(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{
		listFn: func() ([]port.BehaviorRef, error) {
			return []port.BehaviorRef{{Kind: "orc", Path: "/x/orc.json", StateCount: 1}}, nil
		},
	})
	refs, err := s.List()
	if err != nil || len(refs) != 1 || refs[0].Kind != "orc" {
		t.Fatalf("unexpected: %+v %v", refs, err)
	}
}

func TestBehaviorService_GetPassesThrough(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{
		getFn: func(k string) ([]byte, error) { return []byte(validOrc), nil },
	})
	got, err := s.Get("orc")
	if err != nil || string(got) != validOrc {
		t.Fatalf("unexpected: %s %v", got, err)
	}
}

func TestBehaviorService_ValidateOK(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{})
	res := s.Validate("orc", []byte(validOrc))
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %+v", res.Errors)
	}
}

func TestBehaviorService_ValidateFailsOnUnknownAction(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{})
	bad := `{"kind":"orc","states":[{"id":"a","anim":"idle","decision":true,"bt":{"type":"action","name":"do_evil","args":{}}}]}`
	res := s.Validate("orc", []byte(bad))
	if res.Valid { t.Fatal("expected invalid") }
	if len(res.Errors) == 0 { t.Fatal("expected errors") }
}

func TestBehaviorService_ValidateRejectsKindMismatch(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{})
	res := s.Validate("slime", []byte(validOrc))
	if res.Valid { t.Fatal("expected invalid (kind mismatch)") }
	if !strings.Contains(strings.Join(errorMessages(res.Errors), " "), "kind") {
		t.Fatalf("expected kind-mismatch error, got %+v", res.Errors)
	}
}

func TestBehaviorService_UpdateValidatesBeforeWrite(t *testing.T) {
	called := false
	s := service.NewBehavior(fakeBehaviorStore{
		putFn: func(k string, raw []byte) error { called = true; return nil },
	})
	bad := `{"kind":"orc","states":[]}`
	if err := s.Update("orc", []byte(bad)); err == nil {
		t.Fatal("expected validation error")
	}
	if called { t.Fatal("Put should not be called when validation fails") }
}

func TestBehaviorService_UpdateWritesOnSuccess(t *testing.T) {
	written := []byte(nil)
	s := service.NewBehavior(fakeBehaviorStore{
		putFn: func(k string, raw []byte) error { written = raw; return nil },
	})
	if err := s.Update("orc", []byte(validOrc)); err != nil { t.Fatal(err) }
	if string(written) != validOrc { t.Fatalf("write payload mismatch: %s", written) }
}

func TestBehaviorService_UpdatePropagatesStoreError(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{
		putFn: func(k string, raw []byte) error { return errors.New("disk full") },
	})
	err := s.Update("orc", []byte(validOrc))
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("expected disk full error, got %v", err)
	}
}

func errorMessages(errs []service.ValidationError) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs { out = append(out, e.Message) }
	return out
}
```

- [ ] **Step 2: Run fail**

```bash
go test ./internal/editor/service/ -v
```
Expected: FAIL — package missing.

- [ ] **Step 3: Implement**

Create `internal/editor/service/behavior.go`:

```go
package service

import (
	"encoding/json"
	"fmt"

	"claude-pixel/internal/behavior"
	"claude-pixel/internal/editor/port"
)

type Behavior struct{ store port.BehaviorStore }

func NewBehavior(store port.BehaviorStore) *Behavior { return &Behavior{store: store} }

type ValidationError struct {
	Message  string `json:"message"`
	NodePath string `json:"node_path,omitempty"`
}

type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

func (b *Behavior) List() ([]port.BehaviorRef, error)    { return b.store.List() }
func (b *Behavior) Get(kind string) ([]byte, error)      { return b.store.Get(kind) }

func (b *Behavior) Validate(kind string, raw []byte) ValidationResult {
	if !json.Valid(raw) {
		return ValidationResult{Valid: false, Errors: []ValidationError{{Message: "invalid JSON"}}}
	}
	var head struct{ Kind string `json:"kind"` }
	_ = json.Unmarshal(raw, &head)
	if head.Kind != kind {
		return ValidationResult{Valid: false, Errors: []ValidationError{{Message: fmt.Sprintf("kind mismatch: file %q vs body %q", kind, head.Kind)}}}
	}
	if _, err := behavior.LoadBytes(raw, kind+".json"); err != nil {
		return ValidationResult{Valid: false, Errors: []ValidationError{{Message: err.Error()}}}
	}
	return ValidationResult{Valid: true}
}

func (b *Behavior) Update(kind string, raw []byte) error {
	res := b.Validate(kind, raw)
	if !res.Valid {
		return fmt.Errorf("validation failed: %s", res.Errors[0].Message)
	}
	return b.store.Put(kind, raw)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/editor/service/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/editor/service/behavior.go internal/editor/service/behavior_test.go
git commit -m "feat(editor): behavior service with validate-then-write"
```

### Task 1.7: Tuning service

**Files:**
- Create: `internal/editor/service/tuning.go`
- Test: `internal/editor/service/tuning_test.go`

- [ ] **Step 1: Write failing tests**

```go
package service_test

import (
	"errors"
	"testing"

	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/editor/service"
)

type fakeTuningStore struct {
	rows []port.TuningRow
	updateFn func(key string, value float64) (float64, error)
}

func (f fakeTuningStore) List(prefix string) ([]port.TuningRow, error) {
	var out []port.TuningRow
	for _, r := range f.rows {
		if prefix == "" || (len(r.Key) > len(prefix) && r.Key[:len(prefix)+1] == prefix+"_") {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f fakeTuningStore) Update(key string, value float64) (float64, error) { return f.updateFn(key, value) }

func TestTuningService_List(t *testing.T) {
	s := service.NewTuning(fakeTuningStore{rows: []port.TuningRow{
		{Key: "orc_max_lives", Value: 2, Min: 1, Max: 10},
		{Key: "slime_max_lives", Value: 2},
	}})
	rows, err := s.List("orc")
	if err != nil || len(rows) != 1 || rows[0].Key != "orc_max_lives" {
		t.Fatalf("unexpected: %+v %v", rows, err)
	}
}

func TestTuningService_UpdateOK(t *testing.T) {
	s := service.NewTuning(fakeTuningStore{updateFn: func(k string, v float64) (float64, error) {
		return 2, nil
	}})
	old, err := s.Update("orc_max_lives", 5)
	if err != nil || old != 2 { t.Fatalf("unexpected: old=%v err=%v", old, err) }
}

func TestTuningService_UpdateError(t *testing.T) {
	s := service.NewTuning(fakeTuningStore{updateFn: func(k string, v float64) (float64, error) {
		return 0, errors.New("out of range")
	}})
	if _, err := s.Update("orc_max_lives", 999); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run fail**

```bash
go test ./internal/editor/service/ -run Tuning -v
```
Expected: FAIL.

- [ ] **Step 3: Implement**

Create `internal/editor/service/tuning.go`:

```go
package service

import "claude-pixel/internal/editor/port"

type Tuning struct{ store port.TuningStore }

func NewTuning(store port.TuningStore) *Tuning { return &Tuning{store: store} }

func (t *Tuning) List(prefix string) ([]port.TuningRow, error) { return t.store.List(prefix) }
func (t *Tuning) Update(key string, value float64) (float64, error) {
	return t.store.Update(key, value)
}
```

- [ ] **Step 4: Run + commit**

```bash
go test ./internal/editor/service/ -v
git add internal/editor/service/tuning.go internal/editor/service/tuning_test.go
git commit -m "feat(editor): tuning service"
```

### Task 1.8: Registry service

**Files:**
- Create: `internal/editor/service/registry.go`
- Test: `internal/editor/service/registry_test.go`

- [ ] **Step 1: Write failing test**

```go
package service_test

import (
	"testing"

	"claude-pixel/internal/behavior"
	"claude-pixel/internal/editor/service"
)

type fakeRegistry struct{ a, c []behavior.ActionMeta }

func (f fakeRegistry) Actions() []behavior.ActionMeta    { return f.a }
func (f fakeRegistry) Conditions() []behavior.ActionMeta { return f.c }

func TestRegistryService_ReturnsRegistryContents(t *testing.T) {
	s := service.NewRegistry(fakeRegistry{
		a: []behavior.ActionMeta{{Name: "goto"}},
		c: []behavior.ActionMeta{{Name: "grounded"}},
	})
	if len(s.Actions()) != 1 || s.Actions()[0].Name != "goto" { t.Fatal("actions wrong") }
	if len(s.Conditions()) != 1 || s.Conditions()[0].Name != "grounded" { t.Fatal("conditions wrong") }
}
```

- [ ] **Step 2: Run fail**

```bash
go test ./internal/editor/service/ -run Registry -v
```
Expected: FAIL.

- [ ] **Step 3: Implement**

Create `internal/editor/service/registry.go`:

```go
package service

import (
	"claude-pixel/internal/behavior"
	"claude-pixel/internal/editor/port"
)

type Registry struct{ store port.RegistryStore }

func NewRegistry(store port.RegistryStore) *Registry { return &Registry{store: store} }

func (r *Registry) Actions() []behavior.ActionMeta    { return r.store.Actions() }
func (r *Registry) Conditions() []behavior.ActionMeta { return r.store.Conditions() }
```

- [ ] **Step 4: Run + commit**

```bash
go test ./internal/editor/service/ -run Registry -v
git add internal/editor/service/registry.go internal/editor/service/registry_test.go
git commit -m "feat(editor): registry service"
```

### Task 1.9: HTTP handlers

**Files:**
- Create: `internal/editor/http/handler.go`
- Create: `internal/editor/http/middleware.go`
- Test: `internal/editor/http/handler_test.go`

- [ ] **Step 1: Write failing tests**

```go
package http_test

import (
	"bytes"
	"encoding/json"
	"errors"
	httpstd "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"claude-pixel/internal/behavior"
	"claude-pixel/internal/editor/http"
	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/editor/service"
)

type stubBehaviorStore struct {
	listFn func() ([]port.BehaviorRef, error)
	getFn  func(string) ([]byte, error)
	putFn  func(string, []byte) error
}

func (s stubBehaviorStore) List() ([]port.BehaviorRef, error)            { return s.listFn() }
func (s stubBehaviorStore) Get(k string) ([]byte, error)                  { return s.getFn(k) }
func (s stubBehaviorStore) Put(k string, raw []byte) error                { return s.putFn(k, raw) }

type stubTuningStore struct {
	listFn   func(string) ([]port.TuningRow, error)
	updateFn func(string, float64) (float64, error)
}

func (s stubTuningStore) List(p string) ([]port.TuningRow, error)         { return s.listFn(p) }
func (s stubTuningStore) Update(k string, v float64) (float64, error)     { return s.updateFn(k, v) }

type stubRegistry struct{ a, c []behavior.ActionMeta }
func (s stubRegistry) Actions() []behavior.ActionMeta    { return s.a }
func (s stubRegistry) Conditions() []behavior.ActionMeta { return s.c }

const validOrc = `{"kind":"orc","states":[{"id":"idle","anim":"idle","decision":false,"exit_on":"anim_done","next":"idle"}]}`

func newApp(b port.BehaviorStore, t port.TuningStore, r port.RegistryStore) *fiber.App {
	app := fiber.New()
	http.Register(app, http.Deps{
		Behavior: service.NewBehavior(b),
		Tuning:   service.NewTuning(t),
		Registry: service.NewRegistry(r),
	})
	return app
}

func do(t *testing.T, app *fiber.App, method, path string, body []byte) (int, []byte) {
	t.Helper()
	var r *httpstd.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	resp, err := app.Test(r, -1)
	if err != nil { t.Fatalf("Test: %v", err) }
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

func TestGetBehaviors(t *testing.T) {
	app := newApp(stubBehaviorStore{listFn: func() ([]port.BehaviorRef, error) {
		return []port.BehaviorRef{{Kind: "orc", Path: "/x/orc.json", StateCount: 6}}, nil
	}}, nil, nil)
	code, body := do(t, app, "GET", "/api/behaviors", nil)
	if code != 200 { t.Fatalf("status %d body %s", code, body) }
	if !strings.Contains(string(body), `"kind":"orc"`) { t.Fatalf("body: %s", body) }
}

func TestGetBehaviorByKind(t *testing.T) {
	app := newApp(stubBehaviorStore{getFn: func(k string) ([]byte, error) {
		if k != "orc" { t.Fatalf("kind: %q", k) }
		return []byte(validOrc), nil
	}}, nil, nil)
	code, body := do(t, app, "GET", "/api/behaviors/orc", nil)
	if code != 200 || string(body) != validOrc {
		t.Fatalf("status %d body %s", code, body)
	}
}

func TestGetBehaviorNotFound(t *testing.T) {
	app := newApp(stubBehaviorStore{getFn: func(string) ([]byte, error) {
		return nil, errors.New("not found")
	}}, nil, nil)
	code, _ := do(t, app, "GET", "/api/behaviors/ghost", nil)
	if code != 404 { t.Fatalf("status %d", code) }
}

func TestPutBehaviorValid(t *testing.T) {
	called := false
	app := newApp(stubBehaviorStore{putFn: func(k string, raw []byte) error {
		called = true
		return nil
	}}, nil, nil)
	code, _ := do(t, app, "PUT", "/api/behaviors/orc", []byte(validOrc))
	if code != 200 { t.Fatalf("status %d", code) }
	if !called { t.Fatal("Put not called") }
}

func TestPutBehaviorInvalid(t *testing.T) {
	app := newApp(stubBehaviorStore{}, nil, nil)
	bad := `{"kind":"orc","states":[]}`
	code, body := do(t, app, "PUT", "/api/behaviors/orc", []byte(bad))
	if code != 400 { t.Fatalf("status %d body %s", code, body) }
	var v service.ValidationResult
	_ = json.Unmarshal(body, &v)
	if v.Valid || len(v.Errors) == 0 { t.Fatalf("body: %+v", v) }
}

func TestValidateBehavior(t *testing.T) {
	app := newApp(stubBehaviorStore{}, nil, nil)
	code, body := do(t, app, "POST", "/api/behaviors/orc/validate", []byte(validOrc))
	if code != 200 { t.Fatalf("status %d body %s", code, body) }
	if !strings.Contains(string(body), `"valid":true`) { t.Fatalf("body %s", body) }
}

func TestGetTuning(t *testing.T) {
	app := newApp(nil, stubTuningStore{listFn: func(p string) ([]port.TuningRow, error) {
		return []port.TuningRow{{Key: "orc_max_lives", Value: 2, Min: 1, Max: 10, Unit: "—"}}, nil
	}}, nil)
	code, body := do(t, app, "GET", "/api/tuning?prefix=orc", nil)
	if code != 200 || !strings.Contains(string(body), "orc_max_lives") {
		t.Fatalf("status %d body %s", code, body)
	}
}

func TestPutTuning(t *testing.T) {
	app := newApp(nil, stubTuningStore{updateFn: func(k string, v float64) (float64, error) {
		if k != "orc_max_lives" || v != 5 { t.Fatalf("args: %s %v", k, v) }
		return 2, nil
	}}, nil)
	code, body := do(t, app, "PUT", "/api/tuning/orc_max_lives", []byte(`{"value":5}`))
	if code != 200 || !strings.Contains(string(body), `"old":2`) {
		t.Fatalf("status %d body %s", code, body)
	}
}

func TestPutTuningOutOfRange(t *testing.T) {
	app := newApp(nil, stubTuningStore{updateFn: func(string, float64) (float64, error) {
		return 0, errors.New("value out of range: 999 not in [1, 10] —")
	}}, nil)
	code, _ := do(t, app, "PUT", "/api/tuning/orc_max_lives", []byte(`{"value":999}`))
	if code != 400 { t.Fatalf("status %d", code) }
}

func TestGetRegistryActions(t *testing.T) {
	app := newApp(nil, nil, stubRegistry{a: []behavior.ActionMeta{{Name: "goto"}}})
	code, body := do(t, app, "GET", "/api/registry/actions", nil)
	if code != 200 || !strings.Contains(string(body), "goto") { t.Fatalf("body %s", body) }
}

func TestGetRegistryConditions(t *testing.T) {
	app := newApp(nil, nil, stubRegistry{c: []behavior.ActionMeta{{Name: "grounded"}}})
	code, body := do(t, app, "GET", "/api/registry/conditions", nil)
	if code != 200 || !strings.Contains(string(body), "grounded") { t.Fatalf("body %s", body) }
}
```

- [ ] **Step 2: Run fail**

```bash
go test ./internal/editor/http/ -v
```
Expected: FAIL — package missing.

- [ ] **Step 3: Implement handlers**

Create `internal/editor/http/handler.go`:

```go
package http

import (
	"github.com/gofiber/fiber/v2"

	"claude-pixel/internal/editor/service"
)

type Deps struct {
	Behavior *service.Behavior
	Tuning   *service.Tuning
	Registry *service.Registry
}

func Register(app *fiber.App, d Deps) {
	app.Get("/api/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	app.Get("/api/behaviors", func(c *fiber.Ctx) error {
		refs, err := d.Behavior.List()
		if err != nil { return fiber.NewError(500, err.Error()) }
		return c.JSON(refs)
	})
	app.Get("/api/behaviors/:kind", func(c *fiber.Ctx) error {
		raw, err := d.Behavior.Get(c.Params("kind"))
		if err != nil { return fiber.NewError(404, err.Error()) }
		c.Set("Content-Type", "application/json")
		return c.Send(raw)
	})
	app.Put("/api/behaviors/:kind", func(c *fiber.Ctx) error {
		body := c.Body()
		if err := d.Behavior.Update(c.Params("kind"), body); err != nil {
			return c.Status(400).JSON(d.Behavior.Validate(c.Params("kind"), body))
		}
		return c.JSON(fiber.Map{"ok": true})
	})
	app.Post("/api/behaviors/:kind/validate", func(c *fiber.Ctx) error {
		return c.JSON(d.Behavior.Validate(c.Params("kind"), c.Body()))
	})

	app.Get("/api/tuning", func(c *fiber.Ctx) error {
		rows, err := d.Tuning.List(c.Query("prefix"))
		if err != nil { return fiber.NewError(500, err.Error()) }
		return c.JSON(rows)
	})
	app.Put("/api/tuning/:key", func(c *fiber.Ctx) error {
		var body struct{ Value float64 `json:"value"` }
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(400, "body must be {\"value\": number}")
		}
		old, err := d.Tuning.Update(c.Params("key"), body.Value)
		if err != nil { return fiber.NewError(400, err.Error()) }
		return c.JSON(fiber.Map{"ok": true, "old": old, "new": body.Value})
	})

	app.Get("/api/registry/actions",    func(c *fiber.Ctx) error { return c.JSON(d.Registry.Actions()) })
	app.Get("/api/registry/conditions", func(c *fiber.Ctx) error { return c.JSON(d.Registry.Conditions()) })
}
```

Create `internal/editor/http/middleware.go`:

```go
package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func DefaultMiddleware(app *fiber.App) {
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5173",
		AllowMethods:     "GET,POST,PUT,OPTIONS",
		AllowHeaders:     "Content-Type",
		AllowCredentials: false,
	}))
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/editor/http/ -v
```
Expected: PASS (all 11 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/editor/http/handler.go internal/editor/http/middleware.go internal/editor/http/handler_test.go
git commit -m "feat(editor): HTTP handlers for behaviors/tuning/registry"
```

### Task 1.10: Wire `cmd/editor/main.go`

**Files:**
- Modify: `cmd/editor/main.go`

- [ ] **Step 1: Replace skeleton main with full wiring**

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"

	"claude-pixel/internal/editor/adapter"
	editorhttp "claude-pixel/internal/editor/http"
	"claude-pixel/internal/editor/service"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	_ = godotenv.Load()
	port := os.Getenv("EDITOR_PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/game.db"
	}
	behaviorsDir := os.Getenv("BEHAVIORS_DIR")
	if behaviorsDir == "" {
		behaviorsDir = "assets/behaviors"
	}

	db, err := storage.Open(dbPath)
	if err != nil { fatal("open db: %v", err) }
	defer db.Close()
	if err := storage.Migrate(db); err != nil { fatal("migrate: %v", err) }

	tuningRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	editorhttp.DefaultMiddleware(app)
	editorhttp.Register(app, editorhttp.Deps{
		Behavior: service.NewBehavior(adapter.NewFSBehavior(behaviorsDir)),
		Tuning:   service.NewTuning(adapter.NewSQLiteTuning(tuningRepo)),
		Registry: service.NewRegistry(adapter.NewRuntimeRegistry()),
	})

	addr := ":" + port
	log.Printf("editor server listening on %s (db=%s, behaviors=%s)", addr, dbPath, behaviorsDir)
	if err := app.Listen(addr); err != nil { fatal("listen: %v", err) }
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
```

- [ ] **Step 2: Smoke test full stack**

```bash
EDITOR_PORT=8090 DB_PATH=data/game.db BEHAVIORS_DIR=assets/behaviors go run ./cmd/editor &
sleep 2
echo "--- behaviors ---"
curl -s http://localhost:8090/api/behaviors
echo
echo "--- registry actions ---"
curl -s http://localhost:8090/api/registry/actions | head -c 200
echo
echo "--- tuning orc ---"
curl -s http://localhost:8090/api/tuning?prefix=orc | head -c 200
echo
kill %1 2>/dev/null
```
Expected: JSON for each endpoint, registry shows `goto` etc., behaviors shows `orc` + `slime`.

- [ ] **Step 3: Add Makefile target**

Append to `Makefile`:

```makefile
.PHONY: editor
editor:
	go run ./cmd/editor
```

- [ ] **Step 4: Update `.env.example`**

Append:

```
EDITOR_PORT=8080
BEHAVIORS_DIR=assets/behaviors
```

- [ ] **Step 5: Commit**

```bash
git add cmd/editor/main.go Makefile .env.example
git commit -m "feat(editor): wire full editor server with adapters + middleware"
```

---

## Phase 2 — Frontend (React + React Flow)

> Before starting Phase 2, invoke the `react-best-practices` and `composition-patterns` skills to internalize patterns. Apply `web-design-guidelines` checks before final commit.

### Task 2.1: Vite scaffold

**Files:**
- Create: `tools/editor-web/` (entire scaffold)

- [ ] **Step 1: Scaffold via npm**

```bash
mkdir -p tools && cd tools && npm create vite@latest editor-web -- --template react-ts
cd editor-web && npm install
```
Expected: Vite project created.

- [ ] **Step 2: Add deps**

```bash
cd tools/editor-web
npm install reactflow zustand dagre zod
npm install -D tailwindcss@latest postcss autoprefixer @types/dagre vitest @testing-library/react @testing-library/jest-dom jsdom @vitest/coverage-v8 msw
npx tailwindcss init -p
```
Expected: `tailwind.config.js` + `postcss.config.js` created (rename `.js` → `.ts`/`.cjs` if your eslint complains; default is fine).

- [ ] **Step 3: Configure Tailwind**

Replace `tools/editor-web/tailwind.config.js` content with:

```js
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: { extend: {} },
  plugins: [],
}
```

Replace `tools/editor-web/src/index.css`:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

html, body, #root { height: 100%; margin: 0; }
body { background: #1a1d23; color: #e6e9ef; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; }
```

- [ ] **Step 4: Vite proxy + vitest config**

Replace `tools/editor-web/vite.config.ts`:

```ts
/// <reference types="vitest" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { '/api': 'http://localhost:8080' },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/setupTests.ts'],
  },
})
```

Create `tools/editor-web/src/setupTests.ts`:

```ts
import '@testing-library/jest-dom'
```

- [ ] **Step 5: tsconfig path baseUrl**

In `tools/editor-web/tsconfig.json`, ensure `compilerOptions.baseUrl` = `"."` and `paths` includes `"@/*": ["src/*"]`. If the file uses references, edit `tsconfig.app.json` instead.

- [ ] **Step 6: Add `.gitignore`**

Create `tools/editor-web/.gitignore`:

```
node_modules/
dist/
.vite/
coverage/
```

- [ ] **Step 7: Verify dev + test boot**

```bash
cd tools/editor-web
npm run dev &
sleep 3
curl -s http://localhost:5173 | head -c 80
kill %1 2>/dev/null
npx vitest run --reporter=basic || true
```
Expected: dev shows HTML, vitest reports 0 tests (no failures).

- [ ] **Step 8: Commit**

```bash
git add tools/editor-web
git commit -m "feat(editor-web): Vite + React + TS + Tailwind + Vitest scaffold"
```

### Task 2.2: API client + zod schemas

**Files:**
- Create: `tools/editor-web/src/api/schemas.ts`
- Create: `tools/editor-web/src/api/client.ts`
- Test: `tools/editor-web/src/api/__tests__/client.test.ts`

- [ ] **Step 1: Write failing test**

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { listBehaviors, getBehavior, putBehavior, validateBehavior, listTuning, putTuning, listActions, listConditions } from '../client'

const fetchMock = vi.fn()

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
  fetchMock.mockReset()
})
afterEach(() => vi.unstubAllGlobals())

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

describe('api client', () => {
  it('listBehaviors returns refs', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ kind: 'orc', path: '/x/orc.json', state_count: 6 }]))
    const refs = await listBehaviors()
    expect(refs[0].kind).toBe('orc')
    expect(refs[0].state_count).toBe(6)
  })

  it('getBehavior returns parsed json', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ kind: 'orc', states: [] }))
    const b = await getBehavior('orc')
    expect(b.kind).toBe('orc')
  })

  it('putBehavior throws on 400 with errors', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ valid: false, errors: [{ message: 'bad' }] }, 400))
    await expect(putBehavior('orc', { kind: 'orc', states: [] })).rejects.toThrow(/bad/)
  })

  it('validateBehavior returns ValidationResult', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ valid: true }))
    const r = await validateBehavior('orc', { kind: 'orc', states: [] })
    expect(r.valid).toBe(true)
  })

  it('listTuning passes prefix', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ key: 'orc_max_lives', value: 2, min: 1, max: 10, unit: '—', description: 'x' }]))
    await listTuning('orc')
    expect(fetchMock).toHaveBeenCalledWith('/api/tuning?prefix=orc', expect.anything())
  })

  it('putTuning sends value', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ ok: true, old: 2, new: 5 }))
    const r = await putTuning('orc_max_lives', 5)
    expect(r.new).toBe(5)
  })

  it('listActions returns metas', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ name: 'goto', args: [{ name: 'state', type: 'state_id', required: true }] }]))
    const a = await listActions()
    expect(a[0].name).toBe('goto')
  })

  it('listConditions returns metas', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ name: 'grounded', args: [] }]))
    const c = await listConditions()
    expect(c[0].name).toBe('grounded')
  })
})
```

- [ ] **Step 2: Run fail**

```bash
cd tools/editor-web && npx vitest run src/api
```
Expected: FAIL — module missing.

- [ ] **Step 3: Implement schemas**

Create `tools/editor-web/src/api/schemas.ts`:

```ts
import { z } from 'zod'

export const ArgMetaSchema = z.object({
  name: z.string(),
  type: z.enum(['int', 'float', 'string', 'state_id', 'anim_key']),
  required: z.boolean(),
})
export type ArgMeta = z.infer<typeof ArgMetaSchema>

export const ActionMetaSchema = z.object({
  name: z.string(),
  args: z.array(ArgMetaSchema).default([]),
})
export type ActionMeta = z.infer<typeof ActionMetaSchema>

export const BehaviorRefSchema = z.object({
  kind: z.string(),
  path: z.string(),
  state_count: z.number(),
})
export type BehaviorRef = z.infer<typeof BehaviorRefSchema>

export const FrameVXSchema = z.object({
  frame_start: z.number(),
  frame_end: z.number(),
  vx: z.number(),
})

export const StateDeclSchema = z.object({
  id: z.string(),
  anim: z.string(),
  decision: z.boolean(),
  bt: z.any().optional(),
  exit_on: z.string().optional(),
  next: z.string().optional(),
  on_exit_actions: z.array(z.string()).optional(),
  on_frame_vx: z.array(FrameVXSchema).optional(),
})
export type StateDecl = z.infer<typeof StateDeclSchema>

export const BehaviorJSONSchema = z.object({
  kind: z.string(),
  states: z.array(StateDeclSchema),
})
export type BehaviorJSON = z.infer<typeof BehaviorJSONSchema>

export const TuningRowSchema = z.object({
  key: z.string(),
  value: z.number(),
  min: z.number(),
  max: z.number(),
  unit: z.string(),
  description: z.string(),
})
export type TuningRow = z.infer<typeof TuningRowSchema>

export const ValidationErrorSchema = z.object({
  message: z.string(),
  node_path: z.string().optional(),
})
export const ValidationResultSchema = z.object({
  valid: z.boolean(),
  errors: z.array(ValidationErrorSchema).default([]),
})
export type ValidationResult = z.infer<typeof ValidationResultSchema>
```

- [ ] **Step 4: Implement client**

Create `tools/editor-web/src/api/client.ts`:

```ts
import {
  ActionMetaSchema, BehaviorRefSchema, BehaviorJSONSchema, TuningRowSchema, ValidationResultSchema,
  type ActionMeta, type BehaviorJSON, type BehaviorRef, type TuningRow, type ValidationResult,
} from './schemas'
import { z } from 'zod'

class ApiError extends Error {
  constructor(public status: number, message: string, public body?: unknown) { super(message) }
}

async function req(input: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init)
  return res
}

async function getJson<T>(url: string, schema: z.ZodType<T>): Promise<T> {
  const res = await req(url, { headers: { Accept: 'application/json' } })
  if (!res.ok) throw new ApiError(res.status, await res.text())
  return schema.parse(await res.json())
}

async function sendJson<T>(method: 'POST' | 'PUT', url: string, body: unknown, schema: z.ZodType<T>): Promise<T> {
  const res = await req(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const text = await res.text()
  if (!res.ok) {
    let errMsg = text
    try {
      const parsed = JSON.parse(text) as ValidationResult
      if (parsed.errors?.length) errMsg = parsed.errors.map(e => e.message).join('; ')
    } catch { /* keep text */ }
    throw new ApiError(res.status, errMsg)
  }
  return schema.parse(JSON.parse(text))
}

export async function listBehaviors(): Promise<BehaviorRef[]> {
  return getJson('/api/behaviors', z.array(BehaviorRefSchema))
}
export async function getBehavior(kind: string): Promise<BehaviorJSON> {
  return getJson(`/api/behaviors/${kind}`, BehaviorJSONSchema)
}
export async function putBehavior(kind: string, body: BehaviorJSON): Promise<{ ok: boolean }> {
  return sendJson('PUT', `/api/behaviors/${kind}`, body, z.object({ ok: z.boolean() }))
}
export async function validateBehavior(kind: string, body: BehaviorJSON): Promise<ValidationResult> {
  return sendJson('POST', `/api/behaviors/${kind}/validate`, body, ValidationResultSchema)
}
export async function listTuning(prefix: string): Promise<TuningRow[]> {
  return getJson(`/api/tuning?prefix=${encodeURIComponent(prefix)}`, z.array(TuningRowSchema))
}
export async function putTuning(key: string, value: number): Promise<{ ok: boolean; old: number; new: number }> {
  return sendJson('PUT', `/api/tuning/${encodeURIComponent(key)}`, { value }, z.object({ ok: z.boolean(), old: z.number(), new: z.number() }))
}
export async function listActions(): Promise<ActionMeta[]> {
  return getJson('/api/registry/actions', z.array(ActionMetaSchema))
}
export async function listConditions(): Promise<ActionMeta[]> {
  return getJson('/api/registry/conditions', z.array(ActionMetaSchema))
}

export { ApiError }
```

- [ ] **Step 5: Run tests + commit**

```bash
cd tools/editor-web && npx vitest run src/api
git add tools/editor-web/src/api
git commit -m "feat(editor-web): API client + zod schemas"
```

### Task 2.3: BT types + JSON ↔ graph mapping

**Files:**
- Create: `tools/editor-web/src/bt/types.ts`
- Create: `tools/editor-web/src/bt/mapping.ts`
- Test: `tools/editor-web/src/bt/__tests__/mapping.test.ts`

- [ ] **Step 1: Write failing tests**

```ts
import { describe, it, expect } from 'vitest'
import { toGraph, fromGraph } from '../mapping'

const orcRunBT = {
  type: 'sequence',
  children: [
    { type: 'action', name: 'set_vx_forward', args: { speed: 80 } },
    { type: 'wait', seconds: 2 },
    {
      type: 'chance',
      branches: [
        { weight: 50, node: { type: 'action', name: 'flip_facing' } },
        { weight: 50, node: { type: 'action', name: 'stop' } },
      ],
    },
  ],
}

describe('mapping', () => {
  it('toGraph produces nodes with stable ids and edges', () => {
    const { nodes, edges } = toGraph(orcRunBT)
    expect(nodes.length).toBe(6)
    expect(nodes[0].id).toBe('root')
    expect(nodes[0].type).toBe('sequence')
    expect(edges.length).toBeGreaterThan(0)
    const chanceEdges = edges.filter(e => e.label && String(e.label).startsWith('w'))
    expect(chanceEdges.length).toBe(2)
  })

  it('round-trip preserves structure and order', () => {
    const { nodes, edges } = toGraph(orcRunBT)
    const back = fromGraph(nodes, edges)
    expect(back).toEqual(orcRunBT)
  })

  it('handles single action root', () => {
    const single = { type: 'action', name: 'stop' }
    const { nodes, edges } = toGraph(single)
    expect(nodes.length).toBe(1)
    expect(edges.length).toBe(0)
    expect(fromGraph(nodes, edges)).toEqual(single)
  })

  it('handles condition node', () => {
    const cond = {
      type: 'sequence',
      children: [
        { type: 'condition', name: 'grounded' },
        { type: 'action', name: 'stop' },
      ],
    }
    const { nodes, edges } = toGraph(cond)
    expect(fromGraph(nodes, edges)).toEqual(cond)
  })
})
```

- [ ] **Step 2: Run fail**

```bash
cd tools/editor-web && npx vitest run src/bt/__tests__/mapping
```
Expected: FAIL.

- [ ] **Step 3: Types**

Create `tools/editor-web/src/bt/types.ts`:

```ts
export type BTNodeType = 'selector' | 'sequence' | 'chance' | 'wait' | 'action' | 'condition'

export type BTNode =
  | { type: 'selector'; children: BTNode[] }
  | { type: 'sequence'; children: BTNode[] }
  | { type: 'chance'; branches: { weight: number; node: BTNode }[] }
  | { type: 'wait'; seconds: number }
  | { type: 'action'; name: string; args?: Record<string, unknown> }
  | { type: 'condition'; name: string; args?: Record<string, unknown> }

export interface FlowNode {
  id: string
  type: BTNodeType
  data: Record<string, unknown>
  position: { x: number; y: number }
}

export interface FlowEdge {
  id: string
  source: string
  target: string
  label?: string
  data?: { weight?: number; order: number }
}
```

- [ ] **Step 4: Mapping**

Create `tools/editor-web/src/bt/mapping.ts`:

```ts
import type { BTNode, FlowEdge, FlowNode } from './types'

export function toGraph(root: BTNode): { nodes: FlowNode[]; edges: FlowEdge[] } {
  const nodes: FlowNode[] = []
  const edges: FlowEdge[] = []
  walk(root, 'root', null, undefined, undefined, nodes, edges)
  return { nodes, edges }
}

function walk(node: BTNode, id: string, parentId: string | null, weight: number | undefined, order: number | undefined,
              nodes: FlowNode[], edges: FlowEdge[]) {
  const data: Record<string, unknown> = {}
  if (node.type === 'wait') data.seconds = node.seconds
  if (node.type === 'action' || node.type === 'condition') {
    data.name = node.name
    data.args = node.args ?? {}
  }
  nodes.push({ id, type: node.type, data, position: { x: 0, y: 0 } })

  if (parentId) {
    edges.push({
      id: `${parentId}->${id}`,
      source: parentId,
      target: id,
      label: weight !== undefined ? `w${weight}` : undefined,
      data: { weight, order: order ?? 0 },
    })
  }

  if (node.type === 'sequence' || node.type === 'selector') {
    node.children.forEach((child, i) => walk(child, `${id}.children.${i}`, id, undefined, i, nodes, edges))
  } else if (node.type === 'chance') {
    node.branches.forEach((b, i) => walk(b.node, `${id}.branches.${i}.node`, id, b.weight, i, nodes, edges))
  }
}

export function fromGraph(nodes: FlowNode[], edges: FlowEdge[]): BTNode {
  const byId = new Map(nodes.map(n => [n.id, n]))
  const childrenOf = new Map<string, FlowEdge[]>()
  for (const e of edges) {
    const list = childrenOf.get(e.source) ?? []
    list.push(e)
    childrenOf.set(e.source, list)
  }
  for (const list of childrenOf.values()) {
    list.sort((a, b) => (a.data?.order ?? 0) - (b.data?.order ?? 0))
  }
  return rebuild('root', byId, childrenOf)
}

function rebuild(id: string, byId: Map<string, FlowNode>, childrenOf: Map<string, FlowEdge[]>): BTNode {
  const n = byId.get(id)
  if (!n) throw new Error(`fromGraph: missing node ${id}`)
  const childEdges = childrenOf.get(id) ?? []
  switch (n.type) {
    case 'selector':
    case 'sequence':
      return { type: n.type, children: childEdges.map(e => rebuild(e.target, byId, childrenOf)) } as BTNode
    case 'chance':
      return {
        type: 'chance',
        branches: childEdges.map(e => ({
          weight: (e.data?.weight as number) ?? 0,
          node: rebuild(e.target, byId, childrenOf),
        })),
      }
    case 'wait':
      return { type: 'wait', seconds: n.data.seconds as number }
    case 'action':
      return omitEmptyArgs({ type: 'action', name: n.data.name as string, args: n.data.args as Record<string, unknown> })
    case 'condition':
      return omitEmptyArgs({ type: 'condition', name: n.data.name as string, args: n.data.args as Record<string, unknown> })
  }
}

function omitEmptyArgs(node: BTNode): BTNode {
  if ((node.type === 'action' || node.type === 'condition') && node.args && Object.keys(node.args).length === 0) {
    const { args, ...rest } = node
    return rest as BTNode
  }
  return node
}
```

- [ ] **Step 5: Run tests + commit**

```bash
cd tools/editor-web && npx vitest run src/bt/__tests__/mapping
git add tools/editor-web/src/bt
git commit -m "feat(editor-web): BT JSON ↔ graph mapping with ordered round-trip"
```

### Task 2.4: Validation (mirrors BE rules)

**Files:**
- Create: `tools/editor-web/src/bt/validation.ts`
- Test: `tools/editor-web/src/bt/__tests__/validation.test.ts`

- [ ] **Step 1: Write failing tests**

```ts
import { describe, it, expect } from 'vitest'
import { validateBehavior } from '../validation'
import type { BehaviorJSON } from '../../api/schemas'
import type { ActionMeta } from '../../api/schemas'

const registry: { actions: ActionMeta[]; conditions: ActionMeta[] } = {
  actions: [
    { name: 'goto', args: [{ name: 'state', type: 'state_id', required: true }] },
    { name: 'flip_facing', args: [] },
    { name: 'set_vx_forward', args: [{ name: 'speed', type: 'float', required: true }] },
  ],
  conditions: [{ name: 'grounded', args: [] }],
}

const ok: BehaviorJSON = {
  kind: 'orc',
  states: [
    { id: 'a', anim: 'idle', decision: false, exit_on: 'anim_done', next: 'a' },
  ],
}

describe('validateBehavior', () => {
  it('passes valid file', () => {
    const r = validateBehavior(ok, 'orc', registry)
    expect(r.valid).toBe(true)
  })
  it('catches kind mismatch', () => {
    const r = validateBehavior({ ...ok, kind: 'slime' }, 'orc', registry)
    expect(r.valid).toBe(false)
    expect(r.errors[0].message).toMatch(/kind/)
  })
  it('catches duplicate state id', () => {
    const dup: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'idle', decision: false, exit_on: 'anim_done', next: 'a' },
      { id: 'a', anim: 'run', decision: false, exit_on: 'anim_done', next: 'a' },
    ]}
    expect(validateBehavior(dup, 'orc', registry).valid).toBe(false)
  })
  it('catches goto to undeclared state', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true, bt: { type: 'action', name: 'goto', args: { state: 'ghost' } } },
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
  it('accepts goto __dead', () => {
    const dead: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'die', decision: true, bt: { type: 'action', name: 'goto', args: { state: '__dead' } } },
    ]}
    expect(validateBehavior(dead, 'orc', registry).valid).toBe(true)
  })
  it('catches unknown action', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true, bt: { type: 'action', name: 'do_evil' } },
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
  it('catches missing required action arg', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true, bt: { type: 'action', name: 'set_vx_forward', args: {} } },
    ]}
    const r = validateBehavior(bad, 'orc', registry)
    expect(r.valid).toBe(false)
    expect(r.errors[0].message).toMatch(/speed/)
  })
  it('catches chance branch weight <= 0', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true, bt: { type: 'chance', branches: [
        { weight: 0, node: { type: 'action', name: 'flip_facing' } },
      ]}},
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
  it('catches decision state without bt', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true },
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
  it('catches non-decision state without exit_on', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: false },
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
})
```

- [ ] **Step 2: Run fail**

```bash
cd tools/editor-web && npx vitest run src/bt/__tests__/validation
```
Expected: FAIL.

- [ ] **Step 3: Implement**

Create `tools/editor-web/src/bt/validation.ts`:

```ts
import type { ActionMeta, BehaviorJSON } from '../api/schemas'
import type { BTNode } from './types'

export interface ValidationError { message: string; node_path?: string }
export interface ValidationResult { valid: boolean; errors: ValidationError[] }

export interface Registry { actions: ActionMeta[]; conditions: ActionMeta[] }

export function validateBehavior(b: BehaviorJSON, expectedKind: string, reg: Registry): ValidationResult {
  const errors: ValidationError[] = []
  if (b.kind !== expectedKind) errors.push({ message: `kind mismatch: file ${expectedKind}, body ${b.kind}` })

  const ids = new Set<string>()
  for (const s of b.states) {
    if (ids.has(s.id)) errors.push({ message: `duplicate state id: ${s.id}` })
    ids.add(s.id)
    if (s.decision && !s.bt) errors.push({ message: `decision state ${s.id} missing bt` })
    if (!s.decision && !s.exit_on) errors.push({ message: `non-decision state ${s.id} missing exit_on` })
  }
  for (const s of b.states) {
    if (!s.decision && s.next && s.next !== '__dead' && !ids.has(s.next)) {
      errors.push({ message: `state ${s.id} next "${s.next}" undeclared` })
    }
    if (s.decision && s.bt) walkNode(s.bt as BTNode, `states.${s.id}.bt`, ids, reg, errors)
  }
  return { valid: errors.length === 0, errors }
}

const actionByName = (reg: Registry, n: string) => reg.actions.find(a => a.name === n)
const condByName = (reg: Registry, n: string) => reg.conditions.find(c => c.name === n)

function walkNode(n: BTNode, path: string, ids: Set<string>, reg: Registry, errors: ValidationError[]) {
  switch (n.type) {
    case 'selector':
    case 'sequence':
      n.children.forEach((c, i) => walkNode(c, `${path}.children.${i}`, ids, reg, errors))
      break
    case 'chance':
      n.branches.forEach((b, i) => {
        if (!b.weight || b.weight <= 0) errors.push({ message: `chance branch weight must be > 0`, node_path: `${path}.branches.${i}` })
        walkNode(b.node, `${path}.branches.${i}.node`, ids, reg, errors)
      })
      break
    case 'action': {
      const meta = actionByName(reg, n.name)
      if (!meta) { errors.push({ message: `unknown action "${n.name}"`, node_path: path }); break }
      for (const arg of meta.args) {
        if (arg.required && (n.args === undefined || n.args[arg.name] === undefined)) {
          errors.push({ message: `action ${n.name} missing required arg "${arg.name}"`, node_path: path })
        }
      }
      if (n.name === 'goto' && n.args?.state && n.args.state !== '__dead' && !ids.has(n.args.state as string)) {
        errors.push({ message: `goto state "${n.args.state}" undeclared`, node_path: path })
      }
      break
    }
    case 'condition': {
      const meta = condByName(reg, n.name)
      if (!meta) errors.push({ message: `unknown condition "${n.name}"`, node_path: path })
      break
    }
    case 'wait':
      if (typeof n.seconds !== 'number' || n.seconds < 0) errors.push({ message: `wait.seconds must be >= 0`, node_path: path })
      break
  }
}
```

- [ ] **Step 4: Run + commit**

```bash
cd tools/editor-web && npx vitest run src/bt/__tests__/validation
git add tools/editor-web/src/bt/validation.ts tools/editor-web/src/bt/__tests__/validation.test.ts
git commit -m "feat(editor-web): FE validation mirroring BE rules"
```

### Task 2.5: dagre layout

**Files:**
- Create: `tools/editor-web/src/bt/layout.ts`
- Test: `tools/editor-web/src/bt/__tests__/layout.test.ts`

- [ ] **Step 1: Write failing test**

```ts
import { describe, it, expect } from 'vitest'
import { layout } from '../layout'
import { toGraph } from '../mapping'

const seq = {
  type: 'sequence' as const,
  children: [
    { type: 'action' as const, name: 'stop' },
    { type: 'action' as const, name: 'flip_facing' },
  ],
}

describe('layout', () => {
  it('assigns x/y to every node', () => {
    const { nodes, edges } = toGraph(seq)
    const laid = layout(nodes, edges)
    for (const n of laid) {
      expect(typeof n.position.x).toBe('number')
      expect(typeof n.position.y).toBe('number')
    }
  })
  it('root x is leftmost (LR layout)', () => {
    const { nodes, edges } = toGraph(seq)
    const laid = layout(nodes, edges)
    const root = laid.find(n => n.id === 'root')!
    const others = laid.filter(n => n.id !== 'root')
    for (const o of others) expect(o.position.x).toBeGreaterThanOrEqual(root.position.x)
  })
})
```

- [ ] **Step 2: Run fail**

```bash
cd tools/editor-web && npx vitest run src/bt/__tests__/layout
```
Expected: FAIL.

- [ ] **Step 3: Implement**

Create `tools/editor-web/src/bt/layout.ts`:

```ts
import dagre from 'dagre'
import type { FlowEdge, FlowNode } from './types'

const NODE_W = 180
const NODE_H = 64

export function layout(nodes: FlowNode[], edges: FlowEdge[]): FlowNode[] {
  const g = new dagre.graphlib.Graph()
  g.setGraph({ rankdir: 'LR', nodesep: 40, ranksep: 80 })
  g.setDefaultEdgeLabel(() => ({}))
  for (const n of nodes) g.setNode(n.id, { width: NODE_W, height: NODE_H })
  for (const e of edges) g.setEdge(e.source, e.target)
  dagre.layout(g)
  return nodes.map(n => {
    const pos = g.node(n.id)
    return { ...n, position: { x: pos.x - NODE_W / 2, y: pos.y - NODE_H / 2 } }
  })
}
```

- [ ] **Step 4: Run + commit**

```bash
cd tools/editor-web && npx vitest run src/bt/__tests__/layout
git add tools/editor-web/src/bt/layout.ts tools/editor-web/src/bt/__tests__/layout.test.ts
git commit -m "feat(editor-web): dagre LR auto-layout for BT graph"
```

### Task 2.6: Zustand editor store

**Files:**
- Create: `tools/editor-web/src/state/editorStore.ts`
- Test: `tools/editor-web/src/state/__tests__/editorStore.test.ts`

- [ ] **Step 1: Write failing test**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useEditorStore, __resetForTest } from '../editorStore'

vi.mock('../../api/client', () => ({
  listBehaviors: vi.fn(async () => [{ kind: 'orc', path: '/x/orc.json', state_count: 1 }]),
  getBehavior:   vi.fn(async () => ({ kind: 'orc', states: [{ id: 'a', anim: 'idle', decision: false, exit_on: 'anim_done', next: 'a' }] })),
  putBehavior:   vi.fn(async () => ({ ok: true })),
  validateBehavior: vi.fn(async () => ({ valid: true, errors: [] })),
  listActions:   vi.fn(async () => [{ name: 'goto', args: [{ name: 'state', type: 'state_id', required: true }] }]),
  listConditions: vi.fn(async () => [{ name: 'grounded', args: [] }]),
}))

beforeEach(() => __resetForTest())

describe('editorStore', () => {
  it('load fetches behavior + registry', async () => {
    await useEditorStore.getState().load('orc')
    const s = useEditorStore.getState()
    expect(s.behavior?.kind).toBe('orc')
    expect(s.registry.actions.length).toBe(1)
  })
  it('selectState marks selected', async () => {
    await useEditorStore.getState().load('orc')
    useEditorStore.getState().selectState('a')
    expect(useEditorStore.getState().selectedStateId).toBe('a')
  })
  it('save resets dirty', async () => {
    await useEditorStore.getState().load('orc')
    useEditorStore.setState({ dirty: true })
    await useEditorStore.getState().save()
    expect(useEditorStore.getState().dirty).toBe(false)
  })
})
```

- [ ] **Step 2: Run fail**

```bash
cd tools/editor-web && npx vitest run src/state
```
Expected: FAIL.

- [ ] **Step 3: Implement store**

Create `tools/editor-web/src/state/editorStore.ts`:

```ts
import { create } from 'zustand'
import type { ActionMeta, BehaviorJSON, ValidationResult } from '../api/schemas'
import { getBehavior, listActions, listConditions, putBehavior, validateBehavior } from '../api/client'
import { validateBehavior as validateLocal } from '../bt/validation'

interface EditorState {
  currentKind: string | null
  behavior: BehaviorJSON | null
  dirty: boolean
  selectedStateId: string | null
  selectedNodePath: string | null
  registry: { actions: ActionMeta[]; conditions: ActionMeta[] }
  validation: ValidationResult
  load(kind: string): Promise<void>
  save(): Promise<void>
  selectState(id: string | null): void
  selectNode(path: string | null): void
  setBehavior(b: BehaviorJSON): void
}

const initial = {
  currentKind: null,
  behavior: null,
  dirty: false,
  selectedStateId: null,
  selectedNodePath: null,
  registry: { actions: [], conditions: [] },
  validation: { valid: true, errors: [] },
}

export const useEditorStore = create<EditorState>((set, get) => ({
  ...initial,
  async load(kind) {
    const [behavior, actions, conditions] = await Promise.all([
      getBehavior(kind), listActions(), listConditions(),
    ])
    const registry = { actions, conditions }
    const validation = validateLocal(behavior, kind, registry)
    set({ currentKind: kind, behavior, dirty: false, registry, validation })
  },
  async save() {
    const s = get()
    if (!s.currentKind || !s.behavior) return
    const remote = await validateBehavior(s.currentKind, s.behavior)
    if (!remote.valid) {
      set({ validation: { valid: false, errors: remote.errors } })
      throw new Error(remote.errors.map(e => e.message).join('; '))
    }
    await putBehavior(s.currentKind, s.behavior)
    set({ dirty: false })
  },
  selectState(id) { set({ selectedStateId: id, selectedNodePath: null }) },
  selectNode(path) { set({ selectedNodePath: path }) },
  setBehavior(b) {
    const s = get()
    const validation = s.currentKind ? validateLocal(b, s.currentKind, s.registry) : { valid: true, errors: [] }
    set({ behavior: b, dirty: true, validation })
  },
}))

export function __resetForTest() {
  useEditorStore.setState({ ...initial })
}
```

- [ ] **Step 4: Run + commit**

```bash
cd tools/editor-web && npx vitest run src/state
git add tools/editor-web/src/state
git commit -m "feat(editor-web): zustand editor store with load/save/select"
```

### Task 2.7: Custom React Flow node components

**Files:**
- Create: `tools/editor-web/src/bt/nodes/{Selector,Sequence,Chance,Action,Condition,Wait}Node.tsx`

- [ ] **Step 1: Write each node component (one pattern, repeat per type)**

Example `SequenceNode.tsx` (others follow same pattern with different border color and body):

```tsx
import { Handle, Position, type NodeProps } from 'reactflow'

export function SequenceNode({ selected }: NodeProps) {
  return (
    <div className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
         style={{ borderColor: '#7ed957' }}>
      <Handle type="target" position={Position.Left} style={{ background: '#7ed957' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#7ed957]">sequence</div>
      <div className="text-xs text-[#b8c0cc]">in order</div>
      <Handle type="source" position={Position.Right} style={{ background: '#7ed957' }} />
    </div>
  )
}
```

Color map per type (border + label color):
- selector → `#5aa3f0`
- sequence → `#7ed957`
- chance → `#f0a35a` + body shows branch count from `data.branchCount`
- action → `#c779e0` + body shows `data.name(args summary)`
- condition → `#e0c779` + body shows `data.name`
- wait → `#79c7e0` + body shows `${data.seconds}s`

Each node has a left target Handle and right source Handle (except leaf-only nodes — action/condition/wait may omit source handle).

- [ ] **Step 2: Smoke render**

Manual: open Vite, render any node in `App.tsx` temporarily. Confirm visible.

- [ ] **Step 3: Commit**

```bash
git add tools/editor-web/src/bt/nodes
git commit -m "feat(editor-web): custom React Flow nodes per BT type"
```

### Task 2.8: BTCanvas wrapper component

**Files:**
- Create: `tools/editor-web/src/components/BTCanvas.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useMemo } from 'react'
import ReactFlow, { Background, Controls, MiniMap, type Edge, type Node, ReactFlowProvider } from 'reactflow'
import 'reactflow/dist/style.css'
import { useEditorStore } from '../state/editorStore'
import { fromGraph, toGraph } from '../bt/mapping'
import { layout } from '../bt/layout'
import type { BTNode } from '../bt/types'
import { SelectorNode } from '../bt/nodes/SelectorNode'
import { SequenceNode } from '../bt/nodes/SequenceNode'
import { ChanceNode } from '../bt/nodes/ChanceNode'
import { ActionNode } from '../bt/nodes/ActionNode'
import { ConditionNode } from '../bt/nodes/ConditionNode'
import { WaitNode } from '../bt/nodes/WaitNode'

const nodeTypes = {
  selector: SelectorNode, sequence: SequenceNode, chance: ChanceNode,
  action: ActionNode, condition: ConditionNode, wait: WaitNode,
}

export function BTCanvas() {
  const { behavior, selectedStateId, selectNode, setBehavior } = useEditorStore()
  const state = behavior?.states.find(s => s.id === selectedStateId)

  const { nodes, edges } = useMemo(() => {
    if (!state?.bt) return { nodes: [] as Node[], edges: [] as Edge[] }
    const { nodes, edges } = toGraph(state.bt as BTNode)
    const laid = layout(nodes, edges)
    return {
      nodes: laid.map(n => ({ id: n.id, type: n.type, data: n.data, position: n.position })),
      edges: edges.map(e => ({ id: e.id, source: e.source, target: e.target, label: e.label, data: e.data })),
    }
  }, [state])

  if (!state) return <div className="flex items-center justify-center h-full text-[#8a93a3]">Select a state with a BT.</div>
  if (!state.decision) return <div className="flex items-center justify-center h-full text-[#8a93a3]">Non-decision state — no BT.</div>

  return (
    <ReactFlowProvider>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodeClick={(_, n) => selectNode(n.id)}
        fitView
      >
        <Background gap={24} color="#2c3340" />
        <MiniMap />
        <Controls />
      </ReactFlow>
    </ReactFlowProvider>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add tools/editor-web/src/components/BTCanvas.tsx
git commit -m "feat(editor-web): BTCanvas wrapping React Flow with custom nodes"
```

### Task 2.9: StatesPanel

**Files:**
- Create: `tools/editor-web/src/components/StatesPanel.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useEditorStore } from '../state/editorStore'

export function StatesPanel() {
  const { behavior, selectedStateId, selectState } = useEditorStore()
  if (!behavior) return null
  return (
    <aside className="w-60 border-r border-[#3a4150] bg-[#232831] flex flex-col">
      <div className="px-3 py-2 text-[11px] uppercase tracking-wide text-[#8a93a3] border-b border-[#3a4150] font-semibold">
        States
      </div>
      <div className="flex-1 overflow-y-auto p-1">
        {behavior.states.map(s => (
          <button
            key={s.id}
            onClick={() => selectState(s.id)}
            className={`w-full text-left px-2 py-2 rounded text-sm flex items-center justify-between ${
              selectedStateId === s.id ? 'bg-[#5aa3f0] text-white' : 'hover:bg-[#2c3340]'
            }`}
          >
            <span>{s.id}</span>
            <span className="flex gap-1">
              {s.decision && <span className="text-[9px] px-1.5 py-0.5 rounded-full border border-[#7ed957] text-[#7ed957]">BT</span>}
            </span>
          </button>
        ))}
      </div>
    </aside>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add tools/editor-web/src/components/StatesPanel.tsx
git commit -m "feat(editor-web): StatesPanel with selection + BT badge"
```

### Task 2.10: Inspector with Node / State / JSON tabs

**Files:**
- Create: `tools/editor-web/src/components/Inspector.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useState } from 'react'
import { useEditorStore } from '../state/editorStore'

type Tab = 'node' | 'state' | 'json'

export function Inspector() {
  const [tab, setTab] = useState<Tab>('node')
  const { behavior, selectedStateId, selectedNodePath } = useEditorStore()
  const state = behavior?.states.find(s => s.id === selectedStateId)

  return (
    <aside className="w-72 border-l border-[#3a4150] bg-[#232831] flex flex-col">
      <div className="flex border-b border-[#3a4150]">
        {(['node', 'state', 'json'] as Tab[]).map(t => (
          <button key={t} onClick={() => setTab(t)}
            className={`flex-1 px-3 py-2.5 text-xs uppercase tracking-wide border-b-2 ${
              tab === t ? 'border-[#5aa3f0] text-white' : 'border-transparent text-[#8a93a3]'
            }`}>{t}</button>
        ))}
      </div>
      <div className="flex-1 overflow-y-auto p-3 text-sm">
        {tab === 'node' && <NodeInspector path={selectedNodePath} />}
        {tab === 'state' && <StateInspector />}
        {tab === 'json' && state && (
          <pre className="text-xs whitespace-pre-wrap bg-[#1a1d23] p-3 rounded border border-[#3a4150]">
            {JSON.stringify(state.bt ?? state, null, 2)}
          </pre>
        )}
      </div>
    </aside>
  )
}

function NodeInspector({ path }: { path: string | null }) {
  const { behavior, selectedStateId, registry, setBehavior } = useEditorStore()
  if (!path || !behavior || !selectedStateId) return <p className="text-[#8a93a3] text-xs">Click a node to inspect.</p>
  const state = behavior.states.find(s => s.id === selectedStateId)
  if (!state?.bt) return null
  const node = getAtPath(state.bt, path)
  if (!node) return <p className="text-[#8a93a3] text-xs">Node not found.</p>

  const updateNode = (patch: Record<string, unknown>) => {
    const newBT = setAtPath(state.bt as Record<string, unknown>, path, { ...node, ...patch })
    setBehavior({
      ...behavior,
      states: behavior.states.map(s => s.id === selectedStateId ? { ...s, bt: newBT } : s),
    })
  }

  if (node.type === 'action' || node.type === 'condition') {
    const metas = node.type === 'action' ? registry.actions : registry.conditions
    const meta = metas.find(m => m.name === node.name)
    return (
      <div className="space-y-3">
        <Field label="type"><input className="input" disabled value={node.type} /></Field>
        <Field label="name">
          <select className="input" value={node.name}
            onChange={e => updateNode({ name: e.target.value, args: {} })}>
            {metas.map(m => <option key={m.name}>{m.name}</option>)}
          </select>
        </Field>
        {meta?.args.map(arg => (
          <Field key={arg.name} label={`${arg.name} (${arg.type})${arg.required ? ' *' : ''}`}>
            <input className="input"
              type={arg.type === 'int' || arg.type === 'float' ? 'number' : 'text'}
              value={String((node.args as Record<string, unknown>)?.[arg.name] ?? '')}
              onChange={e => updateNode({ args: { ...(node.args ?? {}), [arg.name]: coerceArg(arg.type, e.target.value) } })}
            />
          </Field>
        ))}
      </div>
    )
  }

  if (node.type === 'wait') {
    return (
      <Field label="seconds">
        <input className="input" type="number" step="0.1" value={node.seconds}
               onChange={e => updateNode({ seconds: Number(e.target.value) })} />
      </Field>
    )
  }

  if (node.type === 'chance') {
    return (
      <div className="space-y-2">
        {node.branches.map((b, i) => (
          <Field key={i} label={`branch ${i} weight`}>
            <input className="input" type="number" value={b.weight}
              onChange={e => {
                const next = [...node.branches]
                next[i] = { ...b, weight: Number(e.target.value) }
                updateNode({ branches: next })
              }} />
          </Field>
        ))}
      </div>
    )
  }

  return <p className="text-[#8a93a3] text-xs">{node.type} has no editable args; use canvas to add/remove children.</p>
}

function coerceArg(type: string, raw: string): unknown {
  if (type === 'int') return parseInt(raw, 10)
  if (type === 'float') return parseFloat(raw)
  return raw
}

function getAtPath(root: unknown, path: string): any {
  if (path === 'root') return root
  const parts = path.split('.').slice(1)
  let cur: any = root
  for (let i = 0; i < parts.length; i++) cur = cur?.[parts[i]]
  return cur
}

function setAtPath(root: Record<string, unknown>, path: string, value: unknown): Record<string, unknown> {
  if (path === 'root') return value as Record<string, unknown>
  const parts = path.split('.').slice(1)
  const clone = JSON.parse(JSON.stringify(root))
  let cur: any = clone
  for (let i = 0; i < parts.length - 1; i++) cur = cur[parts[i]]
  cur[parts[parts.length - 1]] = value
  return clone
}

function StateInspector() {
  const { behavior, selectedStateId, setBehavior } = useEditorStore()
  const state = behavior?.states.find(s => s.id === selectedStateId)
  if (!state || !behavior) return <p className="text-[#8a93a3] text-xs">No state selected.</p>
  const update = (patch: Partial<typeof state>) => {
    setBehavior({
      ...behavior,
      states: behavior.states.map(s => s.id === state.id ? { ...s, ...patch } : s),
    })
  }
  return (
    <div className="space-y-3">
      <Field label="anim">
        <input className="input" value={state.anim} onChange={e => update({ anim: e.target.value })} />
      </Field>
      <Field label="decision">
        <input type="checkbox" checked={state.decision} onChange={e => update({ decision: e.target.checked })} />
      </Field>
      {!state.decision && (
        <>
          <Field label="exit_on">
            <select className="input" value={state.exit_on ?? ''} onChange={e => update({ exit_on: e.target.value })}>
              <option value="">—</option>
              <option value="anim_done">anim_done</option>
              <option value="anim_done_and_grounded">anim_done_and_grounded</option>
              <option value="grounded">grounded</option>
            </select>
          </Field>
          <Field label="next">
            <select className="input" value={state.next ?? ''} onChange={e => update({ next: e.target.value })}>
              <option value="">—</option>
              <option value="__dead">__dead</option>
              {behavior.states.filter(s => s.id !== state.id).map(s => <option key={s.id} value={s.id}>{s.id}</option>)}
            </select>
          </Field>
        </>
      )}
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-[11px] uppercase tracking-wide text-[#8a93a3] mb-1">{label}</span>
      {children}
      <style>{`.input { width: 100%; background: #2c3340; color: #e6e9ef; border: 1px solid #3a4150; border-radius: 4px; padding: 4px 6px; font-size: 13px; font-family: ui-monospace, monospace; }`}</style>
    </label>
  )
}
```

> Note: NodeInspector covers action/condition/wait/chance editing inline. Add/remove children for selector/sequence/chance is done from the canvas (right-click menu) — implement in a follow-up task once base canvas works (out of scope for v1: rely on JSON tab read-only view + manual file edit if needed).

- [ ] **Step 2: Commit**

```bash
git add tools/editor-web/src/components/Inspector.tsx
git commit -m "feat(editor-web): Inspector with State + JSON tabs (Node tab stub)"
```

### Task 2.11: TopBar + save flow

**Files:**
- Create: `tools/editor-web/src/components/TopBar.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useEffect, useState } from 'react'
import { useEditorStore } from '../state/editorStore'
import { listBehaviors } from '../api/client'

export function TopBar() {
  const { currentKind, dirty, validation, load, save } = useEditorStore()
  const [kinds, setKinds] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => { listBehaviors().then(refs => setKinds(refs.map(r => r.kind))).catch(e => setError(String(e))) }, [])

  const handleSave = async () => {
    setSaving(true); setError(null)
    try { await save() } catch (e) { setError(String(e)) }
    finally { setSaving(false) }
  }

  return (
    <header className="h-11 px-4 bg-[#232831] border-b border-[#3a4150] flex items-center gap-3">
      <span className="font-semibold text-[#5aa3f0]">⚙ Behavior Editor</span>
      <select className="bg-[#2c3340] text-[#e6e9ef] border border-[#3a4150] rounded px-2 py-1 text-sm"
              value={currentKind ?? ''} onChange={e => load(e.target.value)}>
        <option value="">— pick file —</option>
        {kinds.map(k => <option key={k}>{k}</option>)}
      </select>
      {dirty && <span className="text-[#f0a35a] text-xs">● unsaved changes</span>}
      <span className="flex-1" />
      {error && <span className="text-red-400 text-xs">{error}</span>}
      {!validation.valid && <span className="text-red-400 text-xs">✗ {validation.errors.length} validation errors</span>}
      <button onClick={handleSave} disabled={!dirty || saving || !validation.valid}
        className="bg-[#5aa3f0] disabled:bg-[#3a4150] text-white px-3 py-1 rounded text-sm">
        {saving ? 'Saving…' : 'Save'}
      </button>
    </header>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add tools/editor-web/src/components/TopBar.tsx
git commit -m "feat(editor-web): TopBar with file picker + save"
```

### Task 2.12: TuningDrawer

**Files:**
- Create: `tools/editor-web/src/components/TuningDrawer.tsx`

- [ ] **Step 1: Implement**

```tsx
import { useEffect, useMemo, useState } from 'react'
import { listTuning, putTuning } from '../api/client'
import type { TuningRow } from '../api/schemas'

const PREFIXES = ['physics', 'stamina', 'soldier', 'orc', 'slime', 'enemy_spawn']

export function TuningDrawer() {
  const [open, setOpen] = useState(true)
  const [prefix, setPrefix] = useState('orc')
  const [rows, setRows] = useState<TuningRow[]>([])
  const [pending, setPending] = useState<Record<string, 'saving' | 'saved' | 'error'>>({})

  useEffect(() => {
    const eff = prefix === 'physics' ? '' : prefix
    listTuning(eff).then(setRows).catch(() => setRows([]))
  }, [prefix])

  const grouped = useMemo(() => rows, [rows])

  const handleChange = (key: string, value: number) => {
    setRows(rs => rs.map(r => r.key === key ? { ...r, value } : r))
    setPending(p => ({ ...p, [key]: 'saving' }))
    putTuning(key, value)
      .then(() => setPending(p => ({ ...p, [key]: 'saved' })))
      .catch(() => setPending(p => ({ ...p, [key]: 'error' })))
  }

  if (!open) return (
    <button onClick={() => setOpen(true)} className="px-3 py-1 bg-[#232831] border-b border-[#3a4150] text-xs">▸ Tuning</button>
  )

  return (
    <section className="bg-[#232831] border-b border-[#3a4150]">
      <div className="px-4 py-2 flex items-center gap-2 text-sm">
        <button onClick={() => setOpen(false)} className="text-[#8a93a3]">▾</button>
        <span className="font-semibold text-[#5aa3f0]">Tuning</span>
        <span className="text-[#8a93a3] text-xs">{rows.length} keys</span>
        <span className="flex-1" />
        {PREFIXES.map(p => (
          <button key={p} onClick={() => setPrefix(p)}
            className={`px-2 py-1 text-xs rounded ${prefix === p ? 'bg-[#2c3340] text-white' : 'text-[#8a93a3]'}`}>
            {p}
          </button>
        ))}
      </div>
      <table className="w-full text-xs">
        <tbody>
        {grouped.map(r => (
          <tr key={r.key} className="border-t border-[#2c3340]">
            <td className="px-3 py-2 w-1/3"><div className="text-[#7ed957] font-mono">{r.key}</div><div className="text-[#8a93a3]">{r.description}</div></td>
            <td className="px-3 py-2 w-1/3">
              <input type="range" min={r.min} max={r.max} value={r.value}
                     onChange={e => handleChange(r.key, Number(e.target.value))} className="w-full" />
              <span className="text-[#8a93a3]">[{r.min} .. {r.max}]</span>
            </td>
            <td className="px-3 py-2 w-1/6">
              <input className="w-20 bg-[#2c3340] border border-[#3a4150] rounded px-2 py-1 text-right" type="number"
                     value={r.value} onChange={e => handleChange(r.key, Number(e.target.value))} />
              <span className="text-[#8a93a3] ml-1">{r.unit}</span>
            </td>
            <td className="px-3 py-2 text-right text-[11px]">
              {pending[r.key] === 'saving' && <span className="text-[#f0a35a]">⟳ saving</span>}
              {pending[r.key] === 'saved'  && <span className="text-[#7ed957]">✓ saved</span>}
              {pending[r.key] === 'error'  && <span className="text-red-400">✗ error</span>}
            </td>
          </tr>
        ))}
        </tbody>
      </table>
    </section>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add tools/editor-web/src/components/TuningDrawer.tsx
git commit -m "feat(editor-web): TuningDrawer with per-prefix tabs + slider edit"
```

### Task 2.13: App shell + integration

**Files:**
- Modify: `tools/editor-web/src/App.tsx`
- Modify: `tools/editor-web/src/main.tsx`

- [ ] **Step 1: Compose layout**

Replace `tools/editor-web/src/App.tsx`:

```tsx
import { TopBar } from './components/TopBar'
import { StatesPanel } from './components/StatesPanel'
import { BTCanvas } from './components/BTCanvas'
import { Inspector } from './components/Inspector'
import { TuningDrawer } from './components/TuningDrawer'

export default function App() {
  return (
    <div className="h-screen flex flex-col bg-[#1a1d23]">
      <TopBar />
      <TuningDrawer />
      <main className="flex-1 flex min-h-0">
        <StatesPanel />
        <div className="flex-1 min-w-0"><BTCanvas /></div>
        <Inspector />
      </main>
    </div>
  )
}
```

Ensure `tools/editor-web/src/main.tsx` imports `index.css`.

- [ ] **Step 2: Manual smoke**

```bash
# Terminal A
go run ./cmd/editor

# Terminal B
cd tools/editor-web && npm run dev
```
Open `http://localhost:5173`. Pick `orc`. State list shows. Click `run` (decision state). BT canvas renders nodes. Switch to `slime`. Open a tuning prefix tab, drag a slider, watch ✓ saved.

- [ ] **Step 3: Commit**

```bash
git add tools/editor-web/src/App.tsx tools/editor-web/src/main.tsx
git commit -m "feat(editor-web): assemble 3-column shell with all components"
```

---

## Phase 3 — E2E + docs

### Task 3.1: Playwright smoke test (manual run)

**Files:**
- Create: `tools/editor-web/tests/e2e/editor.spec.ts`
- Modify: `tools/editor-web/package.json` (add `playwright` devDep + `e2e` script)

- [ ] **Step 1: Install Playwright**

```bash
cd tools/editor-web
npm install -D @playwright/test
npx playwright install chromium
```

- [ ] **Step 2: Write spec**

```ts
import { test, expect } from '@playwright/test'

const BASE = 'http://localhost:5173'

test('load orc + see states + render BT', async ({ page }) => {
  await page.goto(BASE)
  await page.locator('select').first().selectOption('orc')
  await expect(page.getByText('run')).toBeVisible()
  await page.getByText('run').click()
  await expect(page.locator('.react-flow__node').first()).toBeVisible()
})
```

- [ ] **Step 3: Add npm script to `package.json`**

```json
"scripts": {
  "e2e": "playwright test"
}
```

- [ ] **Step 4: Manual run**

```bash
# Both servers running
cd tools/editor-web && npm run e2e
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/editor-web/tests tools/editor-web/package.json tools/editor-web/package-lock.json
git commit -m "test(editor-web): Playwright smoke for load + BT render"
```

### Task 3.2: tools/editor-web README

**Files:**
- Create: `tools/editor-web/README.md`

- [ ] **Step 1: Write README**

```markdown
# editor-web

React + Vite + Tailwind FE for the behavior visual editor.

## Run

```bash
# 1. Start the Go editor server (from repo root)
go run ./cmd/editor          # listens on EDITOR_PORT (default 8080)

# 2. Start Vite (this dir)
npm install
npm run dev                  # http://localhost:5173 with /api proxied
```

## Test

```bash
npm run test                 # vitest unit
npm run e2e                  # playwright (requires both servers running)
```

## Layout

- `src/api/`        — typed fetch wrappers + zod schemas
- `src/state/`      — zustand editor store
- `src/bt/`         — JSON ↔ graph mapping, validation, dagre layout, node components
- `src/components/` — TopBar, StatesPanel, BTCanvas, Inspector, TuningDrawer
```

- [ ] **Step 2: Commit**

```bash
git add tools/editor-web/README.md
git commit -m "docs(editor-web): README with run + layout"
```

### Task 3.3: Update `CLAUDE.md`

**Files:**
- Modify: `CLAUDE.md` (add new section after "Tuning CLI" section)

- [ ] **Step 1: Append section**

Insert above the "Debug overlay" section in `CLAUDE.md`:

```markdown
## Editor server (`cmd/editor`)

Go Fiber HTTP server backing the React FE in `tools/editor-web`. Reuses
`internal/behavior` loader as the single source of truth for validation.

```bash
make editor                     # go run ./cmd/editor (port from EDITOR_PORT, default 8080)
```

Endpoints (`/api/...`):
- `GET    /behaviors`            list available kinds
- `GET    /behaviors/:kind`      raw JSON
- `PUT    /behaviors/:kind`      validate + atomic write
- `POST   /behaviors/:kind/validate`
- `GET    /tuning?prefix=orc`    list tuning rows
- `PUT    /tuning/:key`          range-checked update
- `GET    /registry/actions`     action metadata for editor dropdowns
- `GET    /registry/conditions`  condition metadata

Architecture: light hexagonal — `cmd/editor` wires `internal/editor/{http,service,port,adapter}`. Adapters use `internal/storage` repo + FS. Game (`cmd/game`) is unchanged; press F5 in-game to reload behaviors after editor saves.

## Editor FE (`tools/editor-web`)

React + Vite + TS + Tailwind + React Flow + Zustand. See [tools/editor-web/README.md](tools/editor-web/README.md). Two-pane: left state list / center React Flow BT graph / right inspector. Tuning drawer below top bar. FE pre-validates with same rules as BE; both must pass for save.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): document editor server + FE under tools/editor-web"
```

### Task 3.4: Sanity sweep + final commit

- [ ] **Step 1: Run all Go tests**

```bash
go test ./...
```
Expected: PASS.

- [ ] **Step 2: Run all FE unit tests**

```bash
cd tools/editor-web && npm run test -- --run
```
Expected: PASS.

- [ ] **Step 3: Build production FE**

```bash
cd tools/editor-web && npm run build
```
Expected: `dist/` produced, no errors.

- [ ] **Step 4: Manual end-to-end**

Both servers up. Edit a chance branch weight on orc.json. Save. Verify file content changed. `make run`, F5 in game, observe new behavior.

- [ ] **Step 5: Push branch**

```bash
git push -u origin feat/behavior-visual-editor
```
</content>
</invoke>