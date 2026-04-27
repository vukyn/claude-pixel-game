# Editor: BT canvas edits, Undo/Redo, Transitions tab — Design

**Date:** 2026-04-27
**Scope:** `tools/editor-web` (frontend only). No BE changes.
**Pairs with:** [2026-04-26-behavior-visual-editor-design.md](./2026-04-26-behavior-visual-editor-design.md)

## Goals

Add three FE features to the behavior visual editor:

1. **State Transitions overview** — second tab in canvas area showing inter-state transitions (`state.next` + BT `switch_state` actions), read-only.
2. **Undo/Redo** — `Cmd/Ctrl+Z` / `Cmd+Shift+Z` (and `Ctrl+Y`) plus TopBar buttons. Behavior-level snapshot. Cap 50. Persist past save.
3. **Add/remove BT nodes from canvas** — right-click context menu with: Add child (cascading type submenu), Convert to, Delete. Plus "Add root" CTA when a decision state has no BT.

Tuning drawer is excluded from undo. Server contract unchanged; existing `PUT /behaviors/:kind` validates final JSON.

## Non-goals

- No drag-to-reparent on canvas (preserve existing fixed layout via dagre).
- No tuning undo.
- No BE history/audit.
- No edit mode on Transitions tab (read-only).
- No multi-select, no keyboard add/delete (right-click only).

## Architecture overview

```
tools/editor-web/src/
  state/
    editorStore.ts            # add zundo temporal middleware + lastSaved tracking
    btMutations.ts            # NEW — addChild, deleteAt, convertAt, setRoot pure helpers
  bt/
    transitions.ts            # NEW — extractTransitions(behavior) → graph
    convert.ts                # NEW — canConvert + convertNode strict rules
    mapping.ts                # unchanged (still single source for path<->graph)
  components/
    CanvasArea.tsx            # NEW — Tabs shell wrapping BT/Transitions
    BTCanvas.tsx              # add right-click handler, "Add root" CTA
    TransitionsCanvas.tsx     # NEW — read-only React Flow graph
    NodeContextMenu.tsx       # NEW — pane-level floating DropdownMenu (root add)
    TopBar.tsx                # add Undo/Redo buttons + global hotkey
    nodes/*Node.tsx           # wrap each in shadcn <ContextMenu> for per-node menu
  App.tsx                     # mount CanvasArea instead of BTCanvas; register hotkeys
```

**New deps:**
- `zundo` — Zustand temporal middleware.

shadcn primitives used (already installed or add via `npx shadcn@latest add`):
- `tabs`, `dropdown-menu`, `context-menu` (radix ContextMenu wrapper).

## Components

### 1. Undo/Redo (zundo)

Wrap `useEditorStore` with `temporal`:

```ts
import { temporal } from 'zundo'

export const useEditorStore = create<EditorState>()(
  temporal(
    (set, get) => ({ ...existing... }),
    {
      partialize: (s) => ({ behavior: s.behavior }),
      limit: 50,
      equality: (a, b) => a.behavior === b.behavior,  // ref equality, immutable updates
    }
  )
)

export const useTemporal = () => useStore(useEditorStore.temporal)
```

**Why partialize on `behavior` only:** tuning, registry, dirty, selection are not part of undoable history. Tuning drawer mutates a separate API path; undo must not revert it.

**Lifecycle:**
- Every `setBehavior(b)` causes middleware to push prior `behavior` to `pastStates` and clear `futureStates`.
- `load(kind)` → after success, call `useEditorStore.temporal.getState().clear()`.
- `save()` → no clear. User can `Cmd+Z` past a save.
- Cap 50 — middleware drops oldest when exceeded.

**Dirty tracking:** add `lastSaved: BehaviorJSON | null` to store. `dirty` derived as `behavior !== lastSaved` and recomputed inside `setBehavior` and after temporal restores. On `save()` set `lastSaved = behavior`. On `load()` set `lastSaved = behavior`.

Because zundo's restore re-runs `set()`, register a temporal subscriber to recompute dirty after undo/redo.

**Hotkeys:** registered once in `App.tsx`:
- `Cmd/Ctrl+Z` (no Shift) → if `pastStates.length > 0` → `temporal.getState().undo()`.
- `Cmd+Shift+Z` or `Ctrl+Y` → if `futureStates.length > 0` → `temporal.getState().redo()`.
- Skip when `event.target` is `<input>`, `<textarea>`, or has `contenteditable=""` — preserve native field undo.

**TopBar buttons:**

```tsx
<Button size="sm" variant="ghost" disabled={!canUndo} onClick={() => temporal.undo()} aria-label="Undo (Cmd+Z)">
  <Undo2 />
</Button>
<Button size="sm" variant="ghost" disabled={!canRedo} onClick={() => temporal.redo()} aria-label="Redo (Cmd+Shift+Z)">
  <Redo2 />
</Button>
```

`canUndo` = `pastStates.length > 0`. `canRedo` = `futureStates.length > 0`.

### 2. BT mutations — `state/btMutations.ts`

Pure functions over `BTNode`. All return new immutable trees.

```ts
addChild(bt: BTNode, parentPath: string, child: BTNode, opts?: { weight?: number }): BTNode
deleteAt(bt: BTNode, path: string): BTNode | null
convertAt(bt: BTNode, path: string, toType: BTNode['type']): BTNode
setRoot(rootType: BTNode['type'], opts?: { name?: string }): BTNode
```

- **`addChild`** — when parent is `selector`/`sequence`, append to `children`. When parent is `chance`, wrap as `{ weight: opts?.weight ?? 1, node: child }` and append to `branches`. Throws if parent is a leaf.
- **`deleteAt`** — caller must have already checked `path !== 'root'`. Removes the indexed entry from parent's `children`/`branches`. Returns updated tree.
- **`convertAt`** — calls `convert.ts` `canConvert` first; if not ok, throws. Otherwise `convertNode(node, toType)` and splices back via `setAtPath`.
- **`setRoot`** — fresh node with defaults: `selector`/`sequence` → `{children: []}`; `chance` → `{branches: []}`; `wait` → `{seconds: 1}`; `action`/`condition` → `{name: opts?.name ?? '', args: {}}`.

Path operations reuse `mapping.ts` `getAtPath`/`setAtPath`. Add internal helper `parentPathOf(path)` and `lastSegment(path)` for delete.

### 3. Convert rules — `bt/convert.ts`

```ts
export function canConvert(from: BTNode, to: BTNode['type']): { ok: boolean; reason?: string }
export function convertNode(from: BTNode, to: BTNode['type']): BTNode  // throws if !canConvert
```

| From → To | Allowed | Behavior |
|---|---|---|
| composite ↔ composite (`selector`↔`sequence`) | ✅ | preserve `children` array |
| `selector`/`sequence` ↔ `chance` | ✅ | wrap each child as `{weight: 1, node: child}` / unwrap `branches.map(b => b.node)` |
| leaf ↔ leaf (`action` ↔ `condition`) | ✅ | reset `name=''`, `args={}` (user picks new in Inspector) |
| `wait` ↔ `action`/`condition` | ✅ | drop seconds, reset name/args |
| leaf → composite | ✅ | new node with empty `children` (or `branches`) |
| composite → leaf | ❌ if non-empty children/branches | reason: "delete children first"; ✅ if empty |

### 4. Context menu

**Per-node menu** — wrap each node component (`SelectorNode`, etc.) in shadcn `<ContextMenu>`:

```tsx
<ContextMenu>
  <ContextMenuTrigger asChild><div className="...node visual...">...</div></ContextMenuTrigger>
  <ContextMenuContent>
    {isComposite && <AddChildSubmenu path={id} />}
    {convertOptions(node).length > 0 && <ConvertToSubmenu node={node} path={id} />}
    <ContextMenuSeparator />
    <ContextMenuItem onSelect={() => onDelete(id)} disabled={id === 'root'}>Delete</ContextMenuItem>
  </ContextMenuContent>
</ContextMenu>
```

`AddChildSubmenu` items:
```
Selector
Sequence
Chance
Wait
Action ►            (lists registry.actions)
Condition ►         (lists registry.conditions)
```

`ConvertToSubmenu` items: only types `t` where `canConvert(node, t).ok === true`.

Each menu action calls a single `applyMutation` helper in `BTCanvas` that:
1. Computes new `bt` via `btMutations.*`.
2. Calls `setBehavior({ ...behavior, states: states.map(s => s.id === selectedStateId ? { ...s, bt: newBt } : s) })`.

**Pane menu** — separate floating `<DropdownMenu>` controlled by `BTCanvas` local state. Triggered via React Flow `onPaneContextMenu` only when `state.bt === undefined` (decision state with no root yet). Content: same type cascade as `AddChildSubmenu`. Selecting a type calls `setRoot(type)` and assigns to state.

**"Add root" CTA** — same as pane menu but rendered as a button in the `BTCanvas` empty state when `state.decision && !state.bt`. Clicking opens the same DropdownMenu used by pane.

### 5. Transitions tab — `bt/transitions.ts`

```ts
type TransitionEdgeKind = 'next' | 'switch_state'

interface TransitionEdge {
  id: string                  // "from->to:kind:idx"
  from: string
  to: string                  // state id or "__dead"
  kind: TransitionEdgeKind
  label?: string              // exit_on for next; "" for switch_state
}

interface TransitionGraph {
  nodes: { id: string; isInitial: boolean }[]
  edges: TransitionEdge[]
}

export function extractTransitions(b: BehaviorJSON, registry: { actions: ActionMeta[] }): TransitionGraph
```

**Walk:**
- Nodes = `b.states.map(s => ({ id: s.id, isInitial: s.id === b.initial_state }))`. Append synthetic `{ id: '__dead', isInitial: false }` if any edge references `__dead`.
- For each state:
  - If `state.next` is set → push edge `{from: state.id, to: state.next, kind: 'next', label: state.exit_on ?? ''}`.
  - If `state.bt` exists → walk recursively. Per Q7=A, only `{type: 'action', name: 'switch_state'}` produces an edge. Resolve target id from the action's `state_id`-typed arg via `registry.actions` metadata (the action declares one `state_id` arg; pick its value from `node.args`). Edge: `{from: state.id, to: <argValue>, kind: 'switch_state', label: ''}`. If arg missing/empty, skip silently. Caller passes `registry` into `extractTransitions(b, registry)`.

Multi-edge handling (same from→to, same kind): keep as separate edges with distinct `id` suffixes; React Flow renders them as parallel curves.

**`components/TransitionsCanvas.tsx`:**

```tsx
const nodeTypes = { state: StateNode }

<ReactFlowProvider>
  <ReactFlow
    nodes={...}              // type='state', data={isInitial}
    edges={...}              // styled by kind
    nodeTypes={nodeTypes}
    nodesDraggable={false}
    nodesConnectable={false}
    elementsSelectable
    fitView
    onNodeClick={(_, n) => { selectState(n.id); setTab('bt') }}
  >
    <Background gap={24} />
    <MiniMap pannable zoomable />
    <Controls />
  </ReactFlow>
</ReactFlowProvider>
```

`StateNode` — rounded card showing state id. Initial state: accent border (e.g. `border-emerald-500`). `__dead` state: muted gray pill.

**Edge styling:**
- `kind='next'` — `style={{ strokeDasharray: '4 2', stroke: 'var(--muted-foreground)' }}`, label = `state.exit_on`.
- `kind='switch_state'` — solid emerald, no dash, label `BT`. Both kinds use `MarkerType.ArrowClosed`.

**Layout:** reuse `bt/layout.ts` dagre helper, parameterize direction via opts (`LR` for transitions, current `TB` for BT). Add `direction` arg with default `'TB'` so existing BT call site is unchanged.

### 6. CanvasArea + tabs

`components/CanvasArea.tsx`:

```tsx
type CanvasTab = 'bt' | 'transitions'

export function CanvasArea() {
  const [tab, setTab] = useState<CanvasTab>('bt')
  return (
    <div className="flex flex-col flex-1 min-h-0">
      <Tabs value={tab} onValueChange={(v) => setTab(v as CanvasTab)} className="flex flex-col flex-1 min-h-0">
        <TabsList className="rounded-none border-b border-border bg-transparent p-0 h-auto">
          <TabsTrigger value="bt" className="rounded-none">BT</TabsTrigger>
          <TabsTrigger value="transitions" className="rounded-none">Transitions</TabsTrigger>
        </TabsList>
        <TabsContent value="bt" className="flex-1 min-h-0"><BTCanvas /></TabsContent>
        <TabsContent value="transitions" className="flex-1 min-h-0">
          <TransitionsCanvas onJumpToState={() => setTab('bt')} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
```

`App.tsx` mounts `<CanvasArea />` between `<StatesPanel />` and `<Inspector />`.

## Data flow

```
right-click node
  → ContextMenu opens
  → user picks "Convert to → Sequence"
  → BTCanvas.applyMutation(path, { kind: 'convert', toType: 'sequence' })
    → btMutations.convertAt(state.bt, path, 'sequence')
    → setBehavior({...behavior, states: ...})
      → zundo middleware pushes prev to pastStates
      → validation runs (validateLocal)
      → dirty recomputed (behavior !== lastSaved)

Cmd+Z
  → App keydown handler
  → useEditorStore.temporal.getState().undo()
    → middleware restores prior behavior via set()
  → subscriber recomputes dirty + validation

Save (TopBar)
  → store.save()
    → BE validate
    → BE PUT
    → set lastSaved = behavior, dirty = false
    → temporal stack untouched (Q3=B)

Tab → Transitions
  → extractTransitions(behavior) memoized
  → dagre layout (LR)
  → React Flow render
  → click state node
    → selectState(id), setTab('bt')
```

## Validation interaction

- All mutations route through `setBehavior`, which already runs `validateLocal`. Adding empty composite child temporarily produces no validation diff (empty children are valid where supported by validator). Action/condition nodes added with `name=''` will fail validation — UI shows existing inline errors; save blocked by BE validate.
- Convert that strips children (none — strict rules forbid it) cannot create dangling state references.
- Transitions tab is read-only; no validation hook needed there.

## Error handling

- `convertAt` / `addChild` invariants → throw on programmer error (path missing, parent is leaf, etc.). UI catches and shows toast (reuse existing toast if present, else `console.error` + no-op since menu options for invalid converts are not displayed).
- Undo with empty `pastStates` is no-op (button disabled).
- Hotkey suppressed inside text fields.

## Tests

### Unit (vitest)

| File | Coverage |
|---|---|
| `bt/__tests__/convert.test.ts` | full `canConvert` matrix; `convertNode` preserves children for composite↔composite; chance branch wrap/unwrap; rejects composite→leaf when non-empty |
| `bt/__tests__/transitions.test.ts` | run on shipped `assets/behaviors/orc.json` + `slime.json`; assert state count, `next` edge count, `switch_state` edge count, presence of `__dead` |
| `state/__tests__/btMutations.test.ts` | `addChild` to selector + chance (auto weight=1); `deleteAt` cascade removes whole subtree; `convertAt` throws on invalid; `setRoot` defaults |
| `state/__tests__/editorStore.undo.test.ts` | setBehavior pushes past; undo restores; redo replays; cap=50 drops oldest; load(kind) clears; save does NOT clear; dirty tracked across undo/redo |
| `state/__tests__/editorStore.test.ts` | existing tests untouched, baseline still passes |

### E2E (Playwright, `tools/editor-web/e2e/`)

- Right-click action node → "Convert to" → "Condition" → node card renders as condition.
- Right-click selector → "Add child" → "Wait" → new wait node appears.
- Right-click root selector → "Delete" → menu shows item disabled.
- `Cmd+Z` after edit → reverts. `Cmd+Shift+Z` → reapplies.
- TopBar Undo button disabled at clean load, enabled after edit.
- Click "Transitions" tab → graph renders with N states matching behavior. Click a state node → returns to BT tab with that state selected.
- Decision state with no BT → "Add root" button visible → clicking and picking "Selector" creates root.

Manual: keyboard nav inside menus (radix handles), focus trap on submenus.

## Migration / deployment

- No DB or BE changes.
- New FE deps: `zundo` (~1KB).
- Existing tests must continue passing unchanged.
- Bump version comment in `tools/editor-web/README.md` if listing deps.

## Open questions

None at design time. All Q1–Q8 resolved (see commit message / brainstorming transcript for choices).

## File-by-file change summary

| File | Change |
|---|---|
| `tools/editor-web/package.json` | + `zundo` |
| `src/state/editorStore.ts` | wrap with `temporal`; add `lastSaved`; recompute dirty across restore |
| `src/state/btMutations.ts` | NEW |
| `src/bt/convert.ts` | NEW |
| `src/bt/transitions.ts` | NEW |
| `src/bt/layout.ts` | parameterize direction |
| `src/components/CanvasArea.tsx` | NEW |
| `src/components/BTCanvas.tsx` | right-click wiring, applyMutation, "Add root" CTA, pane DropdownMenu |
| `src/components/TransitionsCanvas.tsx` | NEW |
| `src/components/NodeContextMenu.tsx` | NEW (shared add-child / convert-to / delete submenus) |
| `src/components/TopBar.tsx` | + Undo/Redo buttons |
| `src/components/nodes/*Node.tsx` | wrap content in `<ContextMenu>` |
| `src/components/ui/context-menu.tsx` | shadcn add (if missing) |
| `src/App.tsx` | mount `<CanvasArea />`; register global Cmd+Z hotkey |
| `src/bt/__tests__/convert.test.ts` | NEW |
| `src/bt/__tests__/transitions.test.ts` | NEW |
| `src/state/__tests__/btMutations.test.ts` | NEW |
| `src/state/__tests__/editorStore.undo.test.ts` | NEW |
| `e2e/*.spec.ts` | NEW specs for menu, undo, transitions |
