# Enemy Behavior JSON — Design Spec

**Date:** 2026-04-24
**Scope:** Phase 1 of the "visual behavior editor" initiative — runtime engine + JSON schema + loader. GUI editor deferred to a later spec; behavior JSON is hand-edited for now.

## Problem

Enemy decision logic is hardcoded in `internal/enemy/states.go`. Each kind's "what do I do next while running" roll lives inside a `runState.Update` block that shells out to `rng.Float64()` with literal 50/50 probabilities. Adding a new enemy kind requires writing a new `runState` variant or forking the existing one. Retuning the probability table requires editing Go code and recompiling.

The `attack_motions` SQLite table and several `tuning` keys (`orc_intent_tick_s`, `orc_run_speed`, `slime_intent_tick_s`, `slime_run_speed`) spread behavior data across two storage layers without either one being the source of truth.

## Goal

Move enemy decision behavior out of Go code and into per-kind JSON files. Introduce a small behavior-tree (BT) runtime so the JSON can express nested probabilistic choices, sequenced actions, conditions, and timed waits. Keep gameplay invariants (event-driven transitions like Hit→Hurt, Death, Fall→Run on ground) owned by Go engine code so a malformed tree cannot brick combat.

Non-goals for this phase:
- Visual flowchart editor GUI (separate spec, separate implementation).
- Per-enemy (per-instance) behavior overrides — behavior is per-kind.
- Behavior hot-reload via file watcher — manual `F5` reload only.

## Design decisions (from brainstorming)

1. **Ship order:** Runtime + JSON schema first; GUI editor later.
2. **State model:** Core FSM skeleton stays in Go. JSON declares which states a kind uses and wires decisions per state. New states = new JSON entry + (optional) new Go state impl.
3. **Node model:** Full behavior tree — `Selector`, `Sequence`, `Condition`, `Chance`, `Action`, `Wait`.
4. **Storage:** One JSON file per kind (`assets/behaviors/<kind>.json`). Boot-only load + manual `F5` reload in-game (no fsnotify).
5. **Source of truth:** JSON owns behavior-logic values (probabilities, reroll intervals, wait durations, run speed used by BT actions, attack-frame VX). `tuning` table owns stats/physics/spawner globals only.
6. **Event transitions:** Engine-owned. Hit → Hurt, death, Grounded → Run transition from Fall, anim-done exit from Attack/Attack2/Hurt — all executed by the Go driver, not by BT nodes.

## Architecture

New package `internal/behavior/` owns the BT runtime, JSON loader, validator, and action/condition registry. The existing `internal/enemy/` package retains the FSM skeleton but its `states.go` logic is rewritten as a generic driver that delegates to the BT for decision states and enforces event transitions for the rest.

```
┌─────────────────────────────────────┐        ┌─────────────────────────────┐
│ assets/behaviors/orc.json           │        │ internal/enemy (FSM core)   │
│ assets/behaviors/slime.json         │  load  │ - state skeleton            │
│ assets/behaviors/README.md          │ ─────▶ │ - event transitions         │
└─────────────────────────────────────┘        │   (hit, death, grounded)    │
                                               └──────────────┬──────────────┘
┌─────────────────────────────────────┐                       │
│ internal/behavior                   │  BuildKind(cfg) ──────┘
│ - tree.go   Node interface, Status  │
│ - nodes.go  Selector/Sequence/...   │  Per tick:
│ - loader.go JSON → tree, validate   │  1. Engine event check
│ - registry.go action/condition map  │  2. If decision state: BT.Tick(ctx)
│ - ctx.go    Tick context            │  3. Apply ctx.PendingGoto
└─────────────────────────────────────┘
```

### Tree runtime (`internal/behavior/tree.go`)

```go
type Status int
const (
    StatusSuccess Status = iota
    StatusFailure
    StatusRunning
)

type Node interface {
    Tick(ctx *Ctx) Status
}

type Tree struct {
    Root Node
}
```

The `Ctx` struct threads shared state: pointer to the `Enemy` being ticked, `DT time.Duration`, `RNG *rand.Rand`, and a mutable `PendingGoto string` that action nodes write to. The FSM driver reads `PendingGoto` after `Tick` returns and forces the state transition.

### Node types (`internal/behavior/nodes.go`)

| Type | JSON fields | Semantics |
|---|---|---|
| `selector` | `children: Node[]` | Tick children left→right. First `Success` or `Running` returns. All `Failure` → `Failure`. |
| `sequence` | `children: Node[]` | Tick children left→right. First `Failure` or `Running` returns. All `Success` → `Success`. |
| `chance` | `branches: { weight: int, node: Node }[]` | Roll once (first Tick), execute chosen branch. Returns branch's Status. |
| `condition` | `name: string, args: map` | Call registry condition; `true` → `Success`, `false` → `Failure`. |
| `action` | `name: string, args: map` | Call registry action; status per action impl. |
| `wait` | `seconds: float` | Returns `Running` until `seconds` elapsed, then `Success`. Per-node timer. |

### Registry (`internal/behavior/registry.go`)

Global `init()` registers built-in actions and conditions by name. Unknown names are caught by the loader at boot, not at tick time.

**v1 built-in actions:**
- `goto(state: string)` — sets `ctx.PendingGoto`, returns `Success`.
- `flip_facing` — `ctx.Enemy.Facing *= -1`, returns `Success`.
- `randomize_facing` — rolls `Facing ∈ {-1, +1}` uniformly via `ctx.RNG`, returns `Success`. Used both from BT and from `on_exit_actions`.
- `set_vx_forward(speed: float)` — `ctx.Enemy.VX = float64(Facing) * speed`, returns `Success`.
- `stop` — `ctx.Enemy.VX = 0`, returns `Success`.
- `play_anim(key: string)` — plays named anim on the enemy, returns `Success`.

**v1 built-in conditions:**
- `grounded` — true iff `ctx.Enemy.Grounded`.
- `anim_done` — true iff current animation finished.
- `timer_elapsed(key: string)` — per-enemy named timer expiry (timers set by `wait`/`set_timer` actions; `set_timer` is not in v1 but the key namespace is reserved).

Additional actions/conditions can be registered from Go without schema changes — they appear available to every kind's JSON automatically.

### State declaration

JSON declares the full state list per kind. Each state has:
- `id` — string key used by `goto`.
- `anim` — animation key (unprefixed, e.g. `"run"`; `BuildKind` prepends the kind's `AnimPrefix`).
- `decision: bool` — whether BT drives the state.
- `bt: Node` — root node (required iff `decision: true`).
- `exit_on: string` — exit rule for non-decision states (`"anim_done"`, `"anim_done_and_grounded"`, `"grounded"`).
- `next: string` — state to transition to when `exit_on` fires. Special value `"__dead"` flags the enemy as dead.
- `on_exit_actions: string[]` — optional list of registry action names (no args for v1) executed in order right before the transition fires. Used to preserve quirks like Hurt→Run and Fall→Run randomizing `Facing`. v1 built-in: `randomize_facing`.

### JSON schema example — `assets/behaviors/orc.json`

```json
{
  "kind": "orc",
  "states": [
    { "id": "fall", "anim": "idle", "decision": false, "exit_on": "grounded", "next": "run",
      "on_exit_actions": ["randomize_facing"] },

    { "id": "run", "anim": "run", "decision": true,
      "bt": {
        "type": "sequence",
        "children": [
          { "type": "action", "name": "set_vx_forward", "args": { "speed": 80 } },
          { "type": "wait", "seconds": 2 },
          { "type": "chance", "branches": [
              { "weight": 50, "node": {
                  "type": "chance", "branches": [
                    { "weight": 30, "node": { "type": "action", "name": "goto", "args": { "state": "attack"  } } },
                    { "weight": 70, "node": { "type": "action", "name": "goto", "args": { "state": "attack2" } } }
                  ]
              }},
              { "weight": 10, "node": { "type": "action", "name": "goto", "args": { "state": "idle" } } },
              { "weight": 40, "node": { "type": "action", "name": "flip_facing" } }
          ]}
        ]
      }
    },

    { "id": "idle",    "anim": "idle",    "decision": false, "exit_on": "anim_done", "next": "run" },
    { "id": "attack",  "anim": "attack",  "decision": false, "exit_on": "anim_done", "next": "run" },
    { "id": "attack2", "anim": "attack2", "decision": false, "exit_on": "anim_done", "next": "run" },
    { "id": "hurt",    "anim": "hurt",    "decision": false, "exit_on": "anim_done_and_grounded", "next": "run",
                                           "on_exit_actions": ["randomize_facing"] },
    { "id": "death",   "anim": "death",   "decision": false, "exit_on": "anim_done", "next": "__dead" }
  ]
}
```

Orc's current hardcoded behavior translates 1:1: every 2 s reroll, 50% attack (30/70 split), 10% idle (adds a new state not in current FSM), 40% flip.

### Loader + validator

`behavior.LoadFile(path string) (*Tree, []StateDecl, error)` parses the file and validates:

- Unknown `type` → error.
- Unknown `action.name` / `condition.name` → error (checked against registry).
- `chance.branches` empty or total weight ≤ 0 → error.
- `chance.branches[].weight <= 0` → error.
- `goto(state)` where `state` is not declared in `states[]` → error.
- `exit_on` of non-decision state unknown → error.
- `next` references undeclared state and is not `"__dead"` → error.
- Two states share `id` → error.
- `decision: true` but `bt` missing → error.
- `decision: false` but `bt` present → error (caught to prevent silent dead code).

Any validation failure at boot → panic (consistent with existing tuning/env behavior). Any validation failure at F5 reload → log error, retain previously loaded tree.

## Runtime integration

### FSM driver rewrite (`internal/enemy/states.go` → `fsm.go`)

The existing per-state Go structs (`fallState`, `runState`, `attackState`, etc.) are removed. A single generic driver handles all states using the parsed `StateDecl` list:

```go
func (e *Enemy) Update(dt time.Duration) {
    // 1. Engine-owned event transitions (priority, BT bypassed)
    if e.OnHitPending         { e.force("hurt");  return }
    if e.Lives <= 0           { e.force("death"); return }
    st := e.States[e.CurrentState]
    e.tickAnim(dt)

    // 2. Non-decision state: apply exit_on rule, run on_exit_actions, transition
    if !st.Decision {
        if exitRuleMet(e, st.ExitOn) {
            for _, name := range st.OnExitActions {
                behavior.RunAction(name, nil, &behavior.Ctx{Enemy: e, RNG: e.rng})
            }
            if st.Next == "__dead" { e.Dead = true; return }
            e.force(st.Next)
        }
        return
    }

    // 3. Decision state: run BT
    ctx := behavior.Ctx{Enemy: e, DT: dt, RNG: e.rng}
    st.BT.Tick(&ctx)
    if ctx.PendingGoto != "" { e.force(ctx.PendingGoto) }
}
```

`exitRuleMet` recognises `"anim_done"`, `"anim_done_and_grounded"`, `"grounded"`. Unknown rule panics at load time, not at runtime.

### Attack motions move into BT

The `attack_motions` SQLite table is removed. Existing row `slime_attack2_motion` (VX=-60, frames 3–5) is expressed by a sequence inside the `attack2` state of `slime.json`:

```json
{ "id": "attack2", "anim": "attack2", "decision": true,
  "bt": {
    "type": "sequence",
    "children": [
      { "type": "action", "name": "stop" },
      { "type": "condition", "name": "anim_frame_ge", "args": { "frame": 3 } },
      { "type": "condition", "name": "anim_frame_le", "args": { "frame": 5 } },
      { "type": "action", "name": "set_vx_forward", "args": { "speed": -60 } }
    ]
  },
  "exit_on": "anim_done", "next": "run"
}
```

This requires `attack2` to be a decision state with a fallback `goto("run") on anim_done`. Simpler alternative: keep attack states non-decision, but add an `on_frame_vx` field on the state decl that lists `{ frame_start, frame_end, vx }` tuples. This avoids turning every attack into a BT. **Chosen: the on_frame_vx field on state decl.** Keeps BT for top-level decisions only, pushes per-frame VX into a declarative spec close to the anim data.

Updated attack state shape:

```json
{ "id": "attack2", "anim": "attack2", "decision": false,
  "exit_on": "anim_done", "next": "run",
  "on_frame_vx": [ { "frame_start": 3, "frame_end": 5, "vx": -60 } ] }
```

### Tuning table prune

**Remove from `002_seed_data.sql`:**
- `orc_run_speed`
- `orc_intent_tick_s`
- `slime_run_speed`
- `slime_intent_tick_s`

**Remove `attack_motions` table from `001_init_schema.sql`** and drop the `slime_attack2_motion` row from `002_seed_data.sql`.

**Remove from Go code:** `internal/enemy/tuning.go` fields `RunSpeed`, `IntentTickS`; `storage/migrations/*` attack_motions references; `internal/combat/motion.go`; `cmd/tune/` motions subcommand; `internal/enemy/loader.go` `MotionsFor`.

**Keep in tuning:** `*_max_lives`, `*_knockback_vx/vy` (soldier), `*_hurt_bounce_vx/vy` (enemies), `*_foot_padding`, `*_points`, `enemy_spawn_min_s/max_s/max_alive`, all `stamina_*`, soldier physics (run_speed, sprint_speed, jump_velocity, air_control, gravity, max_fall_speed).

Resulting tuning key count: 30 − 4 = 26.

### Kind config

`enemy.KindConfig` gains `BehaviorPath string`. `BuildKind` calls `behavior.LoadFile(path)` and stores both the parsed state list and the per-decision-state BT on `*Kind`. The `Enemy` struct gains `CurrentState string`, `States map[string]*StateDecl`, drops the old `StateID` enum and per-state Go structs.

### F5 reload

`internal/game/game.go` adds an `F5` edge-key handler. Handler iterates the registered kinds (orc, slime), re-calls `behavior.LoadFile` for each, and swaps the tree on the `*Kind`. Existing enemies ticking an old state re-resolve via `e.States[id]` on the next tick — no per-enemy patching needed. On parse error: log `behavior reload failed for <kind>: <err>`, keep the prior tree, do not crash.

### Debug overlay

Two new fields in `internal/debug/fields.go`:
- `enemy_state` — current state id of the nearest enemy.
- `enemy_bt_last_branch` — a short tag set by `chance` nodes when they roll, e.g. `"run/chance#0/attack2"`. Lets F3 show which branch the BT took on the last reroll.

Total catalog grows from 23 → 25 fields.

## File layout

```
assets/
  behaviors/
    orc.json
    slime.json
    README.md                    — node catalog + action/condition list + schema example
internal/
  behavior/
    tree.go                      — Node, Status, Tree, Ctx
    nodes.go                     — Selector, Sequence, Chance, Condition, Action, Wait
    loader.go                    — JSON decode, validate, build tree
    registry.go                  — action/condition registration
    ctx.go                       — Ctx + helpers
    tree_test.go
    nodes_test.go
    loader_test.go
    registry_test.go
  enemy/
    fsm.go                       — generic driver (replaces states.go)
    state_decl.go                — parsed StateDecl shape
    kind.go                      — extended with BehaviorPath
    loader.go                    — drop MotionsFor; load behavior instead
    tuning.go                    — drop RunSpeed, IntentTickS
    enemy.go                     — CurrentState string; drop StateID enum
    fsm_test.go                  — rewritten against synthetic trees
    loader_test.go               — BuildKind with real orc.json
  combat/
    motion.go                    — REMOVED (motions live on state decl now)
  game/
    game.go                      — F5 handler
  storage/migrations/
    001_init_schema.sql          — remove attack_motions CREATE TABLE
    002_seed_data.sql            — remove 4 tuning rows + slime_attack2_motion row
cmd/
  tune/main.go                   — remove motions subcommand
```

## Testing

**Unit tests — `internal/behavior/`:**
- `loader_test.go`: valid JSON parses; unknown node type errors; unknown action name errors; chance weight sum 0 errors; `goto` to unknown state errors; duplicate state id errors; `decision: true` without `bt` errors.
- `nodes_test.go`: Chance with seeded RNG picks expected branch; Sequence short-circuits on Failure; Selector short-circuits on Success; Wait returns Running until elapsed.
- `registry_test.go`: `goto` sets PendingGoto; `flip_facing` toggles; action not in registry fails registration check at load time.

**Integration tests — `internal/enemy/`:**
- `fsm_test.go` rewritten: feed synthetic StateDecl list + BT (no JSON) into `Enemy`, tick, assert state transitions. Cover event priority (hit during decision state → hurt bypasses BT), Fall→Run on grounded, attack anim-done → run via `exit_on`, death terminal.
- `loader_test.go`: `BuildKind` with the real `assets/behaviors/orc.json` — states wired, BT not nil on decision states, `on_frame_vx` parsed.

**Migration regression:**
- Existing storage package tests assert row counts. Update expectations: `attack_motions` table absent, tuning count 26.
- Snapshot-compare default `orc.json` / `slime.json` to committed golden — guards against accidental schema drift.

**Manual verification checklist:**
- `rm -rf data/ && make run` → orc spawns, walks, rerolls intent every 2 s, attacks, flips. Slime same.
- F3 overlay shows `enemy_state` and `enemy_bt_last_branch` for nearest enemy.
- F4 hitbox draw still correct (no regression).
- Edit `orc.json` — set idle weight to 100, others to 0 → F5 reload → orc spawns, runs, idles, never attacks.
- Malformed JSON (delete a closing brace) → F5 reload → error log, orc continues running old behavior.

## Migration plan

1. Add `internal/behavior/` package + tests. No runtime wiring.
2. Write golden `assets/behaviors/orc.json`, `slime.json` that mirror current behavior exactly (2 s reroll, 50/50 × 50/50). Commit README.
3. Add `BehaviorPath` to `KindConfig`, load tree in `BuildKind`, store on `*Kind`. Don't use it yet.
4. Rewrite `enemy/fsm.go` as generic driver. Remove `states.go`. Remove `StateID` enum in favor of `string`.
5. Update callers (`internal/game/game.go`, tests).
6. Update migrations 001 + 002 (drop `attack_motions`, drop 4 tuning rows).
7. Remove `internal/combat/motion.go`, `cmd/tune` motions subcommand.
8. Add F5 reload handler + debug fields.
9. Update `CLAUDE.md`: controls table (F5), tuning count (26), behavior JSON reference.
10. `rm -rf data/ && make run` — verify parity with current feel (slime backstep, orc attack mix).

## Open questions

None at spec time. The `on_frame_vx` field on state decl resolved the attack-motion placement question inline.

## Out of scope for this spec

- Visual editor GUI (separate spec after this lands).
- fsnotify-based hot reload (F5 manual is sufficient for iteration).
- Per-instance behavior overrides (e.g., "boss orc" with different tree).
- Cross-kind shared BT subtrees / templates.
- `set_timer` / `timer_elapsed` generalised timer actions (the registry name is reserved; implementation deferred).
- Blackboard / shared memory between ticks beyond the `Ctx.PendingGoto` channel.
