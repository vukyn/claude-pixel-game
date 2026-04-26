# Behavior Visual Editor — Design

**Date**: 2026-04-26
**Status**: Approved (brainstorming)
**Owner**: kyndv@hasaki.vn
**Related**: `docs/superpowers/specs/2026-04-24-enemy-behavior-json-design.md`

## Goal

Visual web editor for `assets/behaviors/*.json` (per-enemy-kind behavior tree
declarations). Replace hand-editing with drag-and-drop graph + form-driven
inspector. Edit tuning values from the same UI. Game stays unchanged — user
presses F5 in-game to reload after saves.

## Scope

**In:**
- React Flow editor for behavior JSON (orc, slime, future kinds).
- Two-pane layout: state list (sidebar) + BT graph (main canvas) + inspector (right).
- Tuning UI: edit values within `min/max`, debounced save.
- Go Fiber API server: behaviors CRUD + tuning edit + registry introspection.
- Validation on both FE (instant feedback) and BE (single source of truth).

**Out (YAGNI):**
- Auth (localhost-only dev tool).
- Multi-user collaboration / locking. Last-write-wins documented.
- Undo/redo (phase 2 if needed).
- Sprite preview / animation playback.
- Adding new tuning keys, hitbox edits, schema migrations — still SQL-only.
- Adding a new enemy kind via UI (still: write Kind in Go + sprite assets + JSON
  template + tuning seed). UI only edits existing kinds' behavior + tuning.
- Auto reload from editor → game (game keeps existing F5 manual reload).

## Architecture

```
┌───────────────────────┐         HTTP            ┌──────────────────────────┐
│  React FE             │ ◄─────────────────────► │  Go Fiber editor server  │
│  Vite + TS + Tailwind │                         │  cmd/editor/main.go      │
│  + React Flow         │                         │                          │
└───────────────────────┘                         │  Light hexagonal:        │
                                                  │   handler → service →    │
                                                  │   port (interface) →     │
                                                  │   adapter (FS / sqlite)  │
                                                  └──────────┬───────────────┘
                                                             │
                                                             ▼
                          ┌─────────────────────────────────────────────────┐
                          │ assets/behaviors/*.json   (file system)         │
                          │ data/game.db tuning rows  (sqlite)              │
                          │ internal/behavior loader+validator (reused)     │
                          │ internal/behavior action/condition registry     │
                          └─────────────────────────────────────────────────┘
```

Three independent processes: game (`cmd/game`), editor server (`cmd/editor`),
FE dev server (`tools/editor-web`). Editor server imports `internal/behavior` +
`internal/storage` directly to keep validator as the single source of truth.

## Backend (Go Fiber)

### Layout

```
cmd/editor/main.go              # Fiber boot, port from EDITOR_PORT env
internal/editor/
  http/
    handler.go                  # route handlers, JSON marshal
    middleware.go               # CORS for FE on :5173, request log
  service/
    behavior.go                 # use cases: List/Get/Update/Validate
    tuning.go                   # use cases: List/Update tuning value
    registry.go                 # use case: List actions/conditions
  port/
    repository.go               # interfaces: BehaviorStore, TuningStore, RegistryStore
  adapter/
    fsbehavior.go               # BehaviorStore: FS read/write under assets/behaviors/
    sqlitetuning.go             # TuningStore: storage.Repository[player.TuningParam]
    runtimeregistry.go          # RegistryStore: introspect behavior.ActionRegistry
```

Light hexagonal: each layer depends only on the layer immediately inside.
Handlers know nothing about FS or sqlite — they call services. Services depend
on port interfaces, which adapters implement. This keeps tests simple
(swap adapters with hand-rolled fakes) without ceremony.

### Endpoints

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/api/behaviors` | — | `[{kind, path, state_count}]` |
| GET | `/api/behaviors/:kind` | — | raw JSON object |
| PUT | `/api/behaviors/:kind` | `{json}` | `{ok}` or `400 {errors:[]}` |
| POST | `/api/behaviors/:kind/validate` | `{json}` | `{valid, errors:[]}` |
| GET | `/api/tuning?prefix=orc` | — | `[{key, value, min, max, unit, description}]` |
| PUT | `/api/tuning/:key` | `{value}` | `{ok, old, new}` or `400` |
| GET | `/api/registry/actions` | — | `[{name, args:[{name,type,required}]}]` |
| GET | `/api/registry/conditions` | — | same shape |

### Validation reuse

`service/behavior.Update` and `Validate` both call
`internal/behavior.LoadBehaviorFromBytes` (new helper — current loader takes
a path; refactor to accept bytes + filename for error messages, with the
existing path loader becoming a thin wrapper). Same parser, same rules.

### Persistence

PUT `/api/behaviors/:kind` writes file atomically: write to
`<file>.tmp`, fsync, rename to `<file>`. Avoids half-written files if process
dies mid-write.

PUT `/api/tuning/:key` uses existing `storage.Repository[player.TuningParam]`
update path. Existing range validator gates the write.

### Registry introspection

Extend `internal/behavior` to expose:

```go
type ActionMeta struct {
    Name string
    Args []ArgMeta
}
type ArgMeta struct {
    Name     string
    Type     string  // "int", "float", "string", "state_id", "anim_key"
    Required bool
}
func RegisteredActions() []ActionMeta
func RegisteredConditions() []ActionMeta
```

Each action/condition declares its metadata at registration time alongside its
handler. No reflection. List is computed once at boot.

### Migrations

None. Tuning table already has `min_value`, `max_value`, `unit`, `description`.
Behaviors are files. Editor server adds zero schema.

### Tests

| Layer | Test | Tool |
|---|---|---|
| `adapter/fsbehavior` | round-trip read/write, atomic rename, file-not-found 404 | `t.TempDir()` |
| `adapter/sqlitetuning` | reuse existing repo tests; add range-violation case | existing helpers |
| `adapter/runtimeregistry` | snapshot test action/condition list shape | golden file |
| `service/behavior` | hand-rolled mock `BehaviorStore`; validate-then-save path; rollback on validate failure | std lib |
| `service/tuning` | range validation + error mapping | hand-rolled mock |
| `http/handler` | `httptest` request → assert status + body shape | fiber test helper |

## Frontend (React + React Flow)

### Layout

```
tools/editor-web/
  package.json                  # vite, react, typescript, reactflow, tailwind, zustand, dagre, zod
  vite.config.ts                # proxy /api → localhost:EDITOR_PORT
  tailwind.config.ts
  index.html
  src/
    main.tsx
    App.tsx                     # 3-column shell layout
    api/
      client.ts                 # typed fetch wrappers
      schemas.ts                # zod schemas matching BE response types
    state/
      editorStore.ts            # zustand store
    components/
      TopBar.tsx                # file picker, validate, save, dirty indicator
      StatesPanel.tsx           # left list, add/delete state, badges
      BTCanvas.tsx              # React Flow canvas wrapper
      Inspector.tsx             # right panel, tabs Node/State/JSON
      TuningDrawer.tsx          # collapsible, per-prefix tabs, slider rows
    bt/
      nodes/
        SelectorNode.tsx
        SequenceNode.tsx
        ChanceNode.tsx
        ActionNode.tsx
        ConditionNode.tsx
        WaitNode.tsx
      mapping.ts                # JSON tree ↔ React Flow {nodes, edges}
      layout.ts                 # dagre auto-layout
      validation.ts             # FE pre-validate (mirrors BE rules)
```

### State (zustand)

```ts
type EditorStore = {
  currentKind: string | null
  behavior: BehaviorJSON | null
  dirty: boolean
  selectedStateId: string | null
  selectedNodePath: number[] | null   // path into BT subtree
  registry: { actions: ActionMeta[]; conditions: ActionMeta[] }
  validation: { valid: boolean; errors: ValidationError[] }
  load(kind): Promise<void>
  save(): Promise<void>
  selectState(id): void
  selectNode(path): void
  updateNode(path, patch): void
  addNode(parentPath, type): void
  removeNode(path): void
}
```

### JSON ↔ Graph mapping (`bt/mapping.ts`)

- BT JSON tree → flat `{nodes, edges}` arrays for React Flow.
- Each React Flow node has stable `id` = path string
  (e.g. `"root.children.2.branches.0.node"`).
- Edges: parent → children (sequence/selector) and parent → branch.node
  (chance) with `label = "w50"`.
- Reverse: graph → JSON rebuilds tree on save. Children order matters
  (sequence/selector are position-dependent), so each node carries an `order`
  field used during serialize.

### Layout (`bt/layout.ts`)

`dagre` package, `rankdir: 'LR'`, `nodesep: 40`, `ranksep: 80`. Triggered on
load and after structural change (add/remove node, change parent). User-dragged
positions are kept in-session but not persisted — next load = fresh dagre.

### Inspector tabs

- **Node**: form fields by node type. Action/condition: dropdown sourced from
  `registry`, args rendered dynamically from arg schema (number / string /
  `state_id` enum / `anim_key` enum).
- **State**: anim picker, decision toggle, exit_on enum, next dropdown (other
  state ids), `on_frame_vx` repeater, `on_exit_actions` chips.
- **JSON**: read-only view of the selected BT subtree. Read-only because edits
  go through forms to keep structure invariants.

### Pre-validation

Mirrors BE rules. Runs on every store change. Status bar shows ✓/✗ + error
count. Save disabled while invalid. Errors carry `node_path`, used to
highlight bad nodes in the canvas (red border).

### Save flow

```
dirty = true → Save enabled → POST /api/behaviors/:kind/validate
  ✓ valid → PUT /api/behaviors/:kind → 200 → toast "Saved. F5 in-game to apply" → dirty = false
  ✗ invalid → toast errors[] → highlight bad nodes → Save remains disabled
```

### Tuning UI

Collapsible drawer below the top bar. Tabs auto-grouped by key prefix
(`physics` for un-prefixed, `stamina_*`, `orc_*`, `slime_*`, `soldier_*`,
`enemy_spawn_*`). Each row: key + description, slider clamped to [min, max],
input mirroring slider, unit, status (`✓ saved` / `⟳ saving` / `✗ out of range`).
Edit fires PUT after 400ms debounce. Out-of-range = inline red border, no PUT.
"Reset all" reverts un-saved local edits only.

### Tests

| Layer | Test | Tool |
|---|---|---|
| `bt/mapping.ts` | JSON ↔ graph round-trip preserves structure + order | vitest |
| `bt/validation.ts` | each rule: 1 pass + 1 fail case | vitest |
| `bt/layout.ts` | every node has x/y; root.x ≤ children.x | vitest |
| `state/editorStore` | actions update state correctly; path math | vitest |
| `api/client` | mock fetch; map 400/500/network errors | vitest + msw |
| key components | StatesPanel render + add/delete; Inspector form ↔ store; BTCanvas selection | vitest + @testing-library/react |

E2E (Playwright, run manually):
- Edit orc.json → save → re-fetch → assert persisted.
- Tuning slider edit → assert PUT fired and DB row updated.

## Data flow

### Page boot

```
1. GET /api/registry/actions + /api/registry/conditions → cache in store
2. GET /api/behaviors → file picker populated
3. User picks "orc" → GET /api/behaviors/orc + GET /api/tuning?prefix=orc
4. mapping.ts JSON → {nodes, edges} → dagre layout → React Flow render
```

### Edit cycle (BT node)

```
User clicks node → selectNode(path) → Inspector reads behavior[path] →
form change → updateNode(path, patch) → behavior tree updated →
re-validate (FE) → status bar updates → mark dirty
```

### Tuning edit cycle

```
Slider/input change → local update → debounce 400ms →
PUT /api/tuning/:key {value}
  200 → status "✓ saved" (fades 3s)
  400 (out of range) → status "✗ out of range" + revert local
```

## Error handling

| Source | Failure | UI behavior |
|---|---|---|
| Network | API unreachable | Status bar red "API offline", Save disabled, retry button |
| Validate | BE rejects JSON | Toast + error list, highlight nodes from `errors[].node_path` |
| Save | Disk write fails | Toast "Save failed: <reason>", dirty preserved, retry |
| Tuning PUT | 400 out of range | Inline red border + revert local value |
| File missing | Behavior file deleted externally | 404 → toast + return to file picker |
| Concurrency | 2 tabs edit same kind | Last-write-wins (documented). Phase 2: ETag if needed. |

## Validation rules (FE + BE both enforce)

1. `kind` field matches filename.
2. State `id` unique within file.
3. `goto.state` references existing state id (or `__dead`).
4. Action / condition `name` ∈ registry; required args present and typed.
5. `chance.branches[].weight > 0`.
6. Decision state has `bt`; non-decision has `exit_on` + `next`.
7. `anim` key resolves against animation library.
8. `on_frame_vx` ranges within `[0, anim.frame_count]`.

## Run

```bash
# Backend
go run ./cmd/editor                      # listens on EDITOR_PORT (default 8080)

# Frontend (separate terminal)
cd tools/editor-web && npm install && npm run dev   # vite on :5173

# Game (unchanged)
make run                                  # F5 in-game reloads behaviors
```

`.env` adds `EDITOR_PORT=8080`. Vite config proxies `/api` to that port.
`tools/editor-web/node_modules` and `tools/editor-web/dist` added to `.gitignore`.
`.superpowers/` already created during brainstorming — add to `.gitignore`.

## Future work (out of scope here)

- Undo/redo (Cmd+Z) via store history.
- ETag-based optimistic concurrency for multi-tab.
- Sprite + anim preview pane in Inspector.
- "Open in editor" button from in-game F3 debug overlay.
- Side-by-side state-transition graph (`next` + `goto` calls) as second tab.
</content>
</invoke>