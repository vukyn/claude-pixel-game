# Hitbox visual editor — design

**Date:** 2026-04-30
**Status:** approved (pending plan)

## Goal

Replace blind editing of `hitboxes` rows (currently SQL-only) with a 5th inspector tab `Hitbox` that lets a designer drag boxes on a sprite frame. Shortens the "tweak offset → re-run game → eyeball overlap" loop to "drag → Save → F5".

## Non-goals

- Soldier / player hitboxes. Editor is enemy-scoped; soldier rows untouched.
- Adding new hitbox kinds beyond `body`, `attack`, `attack2`. Existing rows only.
- Frame-window slider / "capture current frame" buttons. Numeric inputs only.
- Persisting edits past `data/` wipe. Same pattern as tuning — user re-edits `migrations/002_seed_data.sql` to lock in changes.
- Bulk / multi-box select. One box at a time.

## User flow

1. User opens editor for kind `orc`.
2. Clicks new `Hitbox` tab in the right inspector.
3. Tab loads 3 rows for `owner='orc'`. Box-kind chip row shows `body` selected by default. Backdrop = `orc_idle` (auto-picked).
4. Box rendered as green rectangle on the sprite frame, with 8 drag handles (4 corners + 4 edges) and an interior drag area.
5. User drags top-right corner → width/height update. Drag interior → offsets update. Side panel shows live numeric values; dirty dot appears next to Save.
6. User switches to `attack` chip. Backdrop swaps to `orc_attack`. Frame scrubber on the canvas activates; box appears only on frames within `[frame_start, frame_end]`. User edits `frame_end` numeric input from 4 → 5 — box now visible on frame 5 too.
7. User clicks Save → `PUT /api/hitboxes/orc_attack`. Dirty dot clears.
8. Press F5 in game → behaviors + hitboxes reload (existing path).

## Architecture

### Backend (`cmd/editor`, `internal/editor`)

**Endpoints:**

`GET /api/hitboxes/:owner` →
```json
[
  { "id":"orc_body","owner":"orc","kind":"body","offset_x":24,"offset_y":18,
    "width":40,"height":78,"frame_start":-1,"frame_end":-1 },
  { "id":"orc_attack","owner":"orc","kind":"attack","offset_x":56,"offset_y":30,
    "width":48,"height":40,"frame_start":2,"frame_end":4 },
  { "id":"orc_attack2","owner":"orc","kind":"attack2","offset_x":50,"offset_y":36,
    "width":56,"height":36,"frame_start":3,"frame_end":5 }
]
```

`PUT /api/hitboxes/:id` body:
```json
{ "offset_x":24,"offset_y":18,"width":40,"height":78,"frame_start":-1,"frame_end":-1 }
```

Validation:
- `width > 0`, `height > 0`
- `offset_x >= 0`, `offset_y >= 0`
- `frame_start == -1 && frame_end == -1` (always-on body) **or** `0 <= frame_start <= frame_end`
- `id` must exist (404 otherwise)

Returns 200 with updated row, 400 with `{error}` on validation fail.

`owner`/`kind` immutable via PUT. Adding new rows out of scope.

**Hexagonal split:**
```
internal/editor/
  port/hitbox_store.go        # interface HitboxStore { ListByOwner(owner) ([]HitboxSpec, error); Get(id) (HitboxSpec, error); Update(spec HitboxSpec) error }
  adapter/sqlite_hitbox.go    # wraps storage.Repository[combat.HitboxSpec] + helper for ListByOwner (WHERE owner=?)
  service/hitbox.go           # HitboxService.List(owner), Get(id), Update(id, patch) — runs validators
  http/hitbox_handler.go      # Fiber handlers; Update merges patch onto Get result so id/owner/kind preserved
```

`SQLiteHitbox.ListByOwner` runs `SELECT ... FROM hitboxes WHERE owner = ? ORDER BY kind`. Reuses `combat.HitboxMapper` columns.

Wiring in `cmd/editor/main.go`: instantiate adapter, build service, register routes.

### Frontend (`tools/editor-web`)

**Inspector tab:**
- `Inspector.tsx`: add `<TabsTrigger value="hitbox">Hitbox</TabsTrigger>` and matching `TabsContent` mounting `<HitboxPane owner={selectedKind}/>`.

**Components:**
```
src/components/
  HitboxPane.tsx          # top toolbar (chips + Save) + canvas + side panel
  HitboxCanvas.tsx        # composes <SpriteCanvas/> backdrop + <HitboxOverlay/>
  HitboxOverlay.tsx       # absolute layer over canvas: box rect + 8 handles + interior
  HitboxFields.tsx        # numeric inputs (offset_x, offset_y, width, height, frame_start, frame_end)
```

**State (`src/state/hitboxStore.ts`, Zustand):**
```ts
type Draft = HitboxSpec;
{
  rows: HitboxSpec[];          // server snapshot
  selectedKind: 'body'|'attack'|'attack2';
  draft: Draft;                // editable copy of selected row
  dirty: boolean;
  loadOwner(owner): Promise<void>;       // GET /api/hitboxes/:owner
  selectKind(kind): void;                // copies row → draft
  patch(field, value): void;             // mutates draft, sets dirty
  save(): Promise<void>;                 // PUT, on success replace rows[] entry, clear dirty
  reset(): void;                         // discard draft, restore from rows
}
```

`loadOwner` runs on mount + when `owner` prop changes. `selectKind` warns + resets if dirty.

**Backdrop selection (auto-pick):**
```ts
const backdropAnim = {
  body:    `${owner}_idle`,
  attack:  `${owner}_attack`,
  attack2: `${owner}_attack2`,
}[selectedKind];
```
Falls through to `${owner}_idle` if mapped anim missing in registry (logged warn, frame scrubber still works against idle).

**Visibility rule:**

Overlay is hidden when `currentFrame < draft.frame_start || currentFrame > draft.frame_end` and `frame_start != -1`. Body box (`frame_start = -1`) always visible.

**Drag math:**

Canvas renders sprite at scale `S` (default 2x). Pointer `dx/dy` divided by `S` → source-pixel deltas → applied to `draft.offset_x/offset_y/width/height`.

8 handles:
- 4 corners — each adjusts two of `{offset_x, offset_y, width, height}` (e.g. top-left handle: `offset_x += dx`, `offset_y += dy`, `width -= dx`, `height -= dy`)
- 4 edges — each adjusts one dimension (e.g. right-edge: `width += dx`)
- Interior drag — `offset_x += dx`, `offset_y += dy`

Clamp: `width >= 1`, `height >= 1` during drag (cannot collapse).

**Save flow:**

`save()` sends only mutable fields (`offset_x, offset_y, width, height, frame_start, frame_end`). On 200 → replace matching row in `rows[]`, copy back to `draft`, clear dirty. On 400 → toast error message from response body. On network error → keep dirty, toast retry.

**Existing `SpriteCanvas` reuse:**

Spec #1 already builds `SpriteCanvas` for the Sprite tab. `HitboxCanvas` mounts the same component but exposes its current frame index up via callback (small prop addition: `onFrameChange?(n)`). No fork.

## Data flow

```
mount HitboxPane(owner)
  → store.loadOwner → GET /api/hitboxes/:owner → rows[]
  → selectKind('body') → draft = rows.find(body)

drag handle
  → pointer events (down/move/up) → store.patch(field, val)
  → overlay re-renders, fields update, dirty=true

scrub frame
  → SpriteCanvas onFrameChange(n) → store.currentFrame = n
  → overlay visibility recomputed

click Save
  → PUT /api/hitboxes/:id with draft → service validates → adapter Update
  → server returns updated row → store replaces rows[i], clears dirty
```

## Error handling

- `GET /api/hitboxes/:owner` empty → tab shows "no hitboxes seeded for owner=<x>". Save disabled.
- `PUT` validation 400 → toast with server error string.
- Switch chip while dirty → confirm modal: "Discard changes?" → Yes resets, No keeps current.
- Backdrop anim missing in registry → fallback to idle; banner above canvas: "preview anim '<id>' not found, using idle".
- Sprite image fails to load → underlying SpriteCanvas already shows "image not found" overlay (spec #1). Box still draggable on top of error placeholder.

## Testing

**Backend:**
- `service/hitbox_test.go` — Update rejects `width=0`, `height=-1`, `frame_start=2 frame_end=1`; accepts `-1/-1`; preserves `id/owner/kind` from existing row.
- `adapter/sqlite_hitbox_test.go` — `ListByOwner('orc')` returns 3 rows ordered by kind; `Update` round-trips offsets.
- `http/hitbox_handler_test.go` — 200 list, 400 invalid PUT body, 404 unknown id.

**Frontend:**
- `hitboxStore.test.ts` — `selectKind` copies row to draft, sets dirty=false; `patch` flips dirty=true; `save` clears dirty on success; `reset` restores from rows.
- `HitboxOverlay.test.tsx` — top-left handle drag updates offset+size correctly with scale=2; interior drag moves only offset; clamp prevents width<1.
- Visibility: rendering with `currentFrame=1, frame_start=2, frame_end=4` → overlay not in DOM; with `currentFrame=3` → overlay rendered.

**Manual smoke:**
- Open orc, edit body box, Save, F5 in game, verify orc takes hits at new geometry.
- Edit attack frame_end from 4 → 5, Save, F5, verify orc's swing is active one extra frame.
- Switch chip while dirty → confirm dialog appears.

## Out of scope (future)

- Soldier/player editing — needs editor pane that scopes by owner instead of enemy kind.
- Add/delete kinds beyond body/attack/attack2 — needs schema thinking on what a "fourth kind" means at runtime (combat.Resolve hardcodes the three).
- Sync to `migrations/002_seed_data.sql` — same lossy story as tuning. Could add a "Export to migration patch" button later.
- Frame-window slider / capture-frame UX — only worthwhile after numeric proves cumbersome.
- Body box visualization for player/soldier in-game during Hitbox tab edit (cross-context preview).
