# Sprite/anim preview tab — design

**Date:** 2026-04-29
**Status:** approved (pending plan)

## Goal

Add a 4th tab `Sprite` to the editor inspector that previews any animation owned by the currently edited enemy kind. Lets a designer see frames + loop while wiring `state.anim` in the BT canvas, without launching the game.

## Non-goals

- Editing animation specs (paths, frame counts, grid). Read-only.
- Hitbox overlay on frames. Reserved for the separate hitbox-editor spec.
- Multi-anim comparison, fps override slider, render-scale toggle, looping toggle.
- Player animations. Tab is enemy-kind scoped (matches inspector context).

## User flow

1. User opens editor for kind `orc` → BT canvas shows states.
2. User clicks state node `attack` (anim = `orc_attack`).
3. Inspector switches to `Sprite` tab. Anim list highlights `orc_attack`. Canvas plays at the spec's native fps.
4. User clicks `orc_run` in the sidebar list → preview switches, plays. Selection in BT canvas is unchanged.
5. User scrubs the range slider → frame jumps, play pauses. Press play to resume.

## Architecture

### Backend (`cmd/editor`, `internal/editor`)

**New endpoint:** `GET /api/anims/:kind`

Returns:
```json
[
  {
    "id": "orc_idle",
    "path": "assets/sprites/orc/Idle.png",
    "frame_w": 100, "frame_h": 100,
    "frame_count": 6, "duration_ms": 600, "loop": true,
    "grid_cols": 6, "grid_rows": 1,
    "pick_row": 0, "pick_col": 0
  }
]
```

Filter rule: `id LIKE '<kind>_%'` AND `is_enemy = 1`. Other queries return empty list.

**New static handler:** `app.Static("/assets", "./assets")` mounted by `cmd/editor/main.go`.
- Serves PNG sheets only (no DB, no behaviors). Read-only.
- Path is relative to editor working dir; rejects `..` traversal (Fiber default).

**Hexagonal split:**
```
internal/editor/
  port/anim_store.go         # interface AnimStore { ListByKindPrefix(prefix string) ([]anim.AnimationSpec, error) }
  adapter/sqlite_anim.go     # wraps storage.Repository[anim.AnimationSpec], filters in SQL
  service/anim.go            # AnimService.List(kind) — validates kind, calls store
  http/anim_handler.go       # GET /api/anims/:kind; 404 if list empty
```

`SQLiteAnim` reuses the existing `storage.Repository[anim.AnimationSpec]` — adds one helper that calls `db.Query` with `WHERE id LIKE ? AND is_enemy = 1`.

Wiring in `cmd/editor/main.go`: instantiate `SQLiteAnim`, build `AnimService`, register handler under `/api`. Static handler registered before route group.

### Frontend (`tools/editor-web`)

**Inspector tab:**
- `Inspector.tsx`: add `<TabsTrigger value="sprite">Sprite</TabsTrigger>` and matching `TabsContent` mounting `<SpritePane kind={...} selectedAnim={...} />`.

**Components:**
```
src/components/
  SpritePane.tsx       # vertical split: <SpriteList/> + <SpriteCanvasPanel/>
  SpriteList.tsx       # ul of anim ids; highlights selected; click → setSelected
  SpriteCanvasPanel.tsx# <SpriteCanvas/> + transport bar (play/pause + scrub + readout)
  SpriteCanvas.tsx     # <canvas> + RAF loop, drawImage from sheet
```

**Data:**
- `src/api/anims.ts` exports `fetchAnims(kind: string): Promise<AnimSpec[]>`.
- `src/state/spriteStore.ts` (Zustand): `{ selectedAnimId, frame, playing, setAnim, togglePlay, scrub(frame) }`. Reset on kind change.
- Hook `useAnims(kind)`: cache list per kind in component state via `useEffect` + abort controller (no TanStack Query; keep stack minimal).

**Frame slicing on canvas:**

Mirrors `internal/anim/sheet.go`. Given `frame, spec`:
```ts
let col, row;
if (spec.grid_cols > 0 && spec.grid_rows > 0) {
  if (spec.pick_col >= 0) { col = spec.pick_col; row = frame; }     // column slice
  else                    { col = frame; row = spec.pick_row; }      // row slice
} else {
  col = frame; row = 0;                                              // flat strip
}
ctx.drawImage(img, col*frame_w, row*frame_h, frame_w, frame_h, 0, 0, frame_w, frame_h);
```

Sprite PNG fetched once per anim via `new Image(); img.src = '/assets/' + path.replace(/^assets\//,'')`.

**Play loop:**

`requestAnimationFrame`; advance frame when `elapsed >= duration_ms / frame_count`. Stop at last frame if `!spec.loop`. Pause sets `playing=false`. Scrub sets `frame=N, playing=false`.

**Auto-sync rule:**

`SpritePane` receives `selectedAnim` prop from parent. On change → `setAnim(selectedAnim); scrub(0); play()`. If `selectedAnim` not in list (mismatch / orphan), keep last selection but show subtle warning ("anim not in registry").

Parent (`Inspector.tsx`) reads `state.anim` from selected BT node via existing canvas selection store.

## Data flow

```
BT canvas click
  → Zustand canvas store sets selectedNode
  → Inspector reads selectedNode.data.state.anim
  → passes as prop to SpritePane
  → spriteStore.setAnim
  → SpriteCanvas: load img if new, restart RAF
```

```
SpriteList click
  → spriteStore.setAnim (no canvas selection change)
  → SpriteCanvas restarts
```

## Error handling

- `GET /api/anims/:kind` empty list → 200 + `[]`. FE shows "no animations registered for kind" message.
- Static fetch 404 → canvas shows "image not found: <path>" overlay.
- Spec rows that don't slice cleanly (sheet width mismatch) → FE caps frame index to image bounds, logs `console.warn`. Game has stricter check at LoadLibrary; editor is lenient (preview only).

## Testing

**Backend:**
- Unit: `service/anim_test.go` — list returns only enemy specs with kind prefix; unknown kind returns empty.
- Adapter: SQL filter wired correctly (use in-memory sqlite + seed two specs).
- Handler: `GET /api/anims/orc` → 200 + array.

**Frontend:**
- Component: `SpriteList` highlights id, click triggers store update.
- `SpriteCanvas` slicing math: render frame N for column-slice spec → assert drawImage called with `(0, N*h, w, h, ...)` (mock canvas).
- Auto-sync: changing `selectedAnim` prop resets frame to 0, sets playing.

Manual smoke: open orc kind, click each state, verify anim plays.

## Out of scope (future)

- fps override / scale toggle (B/C from Q1.3) — easy add later if needed.
- Hitbox overlay — owned by separate hitbox-editor spec (will reuse `SpriteCanvas` as base).
- Player anim previews (would need separate inspector context; currently editor is enemy-only).
