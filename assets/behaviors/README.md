# Enemy behavior JSON

Each enemy kind has its own file here (`orc.json`, `slime.json`, …). The
runtime loader lives in `internal/behavior`. See
`docs/superpowers/specs/2026-04-24-enemy-behavior-json-design.md` for the
design rationale.

## Top-level shape

```
{
  "kind": "<kind>",        // must match the AnimPrefix / tuning prefix
  "states": [StateDecl, …]
}
```

## State declaration

| field | type | required | notes |
|---|---|---|---|
| `id` | string | yes | unique per file; `goto` / `next` reference these |
| `anim` | string | yes | unprefixed anim key (e.g. `"run"`); kind prefix added at runtime |
| `decision` | bool | yes | true → driven by `bt`; false → engine drives exit via `exit_on` |
| `bt` | Node | iff decision | root BT for the state |
| `exit_on` | string | for non-decision | `"anim_done"`, `"anim_done_and_grounded"`, `"grounded"` |
| `next` | string | for non-decision | target state id, or `"__dead"` |
| `on_exit_actions` | string[] | optional | registered action names, no args |
| `on_frame_vx` | {frame_start, frame_end, vx}[] | optional | per-frame VX slide during state |

## Node types

- `selector` — `{ "type": "selector", "children": [...] }` — first non-Failure wins.
- `sequence` — `{ "type": "sequence", "children": [...] }` — first non-Success wins.
- `chance`   — `{ "type": "chance", "branches": [{ "weight": int, "node": Node }, ...] }`
- `wait`     — `{ "type": "wait", "seconds": float }`
- `action`   — `{ "type": "action", "name": "<registered>", "args": { ... } }`
- `condition`— `{ "type": "condition", "name": "<registered>", "args": { ... } }`

## Built-in actions (v1)

- `goto(state)` — queue a state transition.
- `flip_facing` — negate facing.
- `randomize_facing` — roll ±1.
- `set_vx_forward(speed)` — VX = facing × speed.
- `stop` — VX = 0.
- `play_anim(key)` — play anim by unprefixed key.

## Built-in conditions (v1)

- `grounded`
- `anim_done`
- `anim_frame_ge(frame)` / `anim_frame_le(frame)`

## Reload

Boot reads every file once. Press **F5** in-game to re-parse. On parse
failure, the old tree is retained and an error is logged.
