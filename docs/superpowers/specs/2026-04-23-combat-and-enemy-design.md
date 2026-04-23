# Combat & Enemy — Design Spec

**Date:** 2026-04-23
**Status:** Approved for implementation
**Scope:** Rename `char1` → `soldier`, integrate orc enemy, window boundary clamp, spawner, combat (hitboxes + i-frames + knockback), heart HUD with monogram font, game-over + restart.

---

## 1. Goals & Non-goals

### Goals
- Rename soldier assets + DB IDs from `char1`/`idle` → `soldier/soldier_idle`.
- Integrate orc sprite (100×100 frames) with 6 animations: Idle, Run, Attack, Attack2, Hurt, Death.
- Window boundary clamp on soldier + orc (sprite never crosses edge).
- Orc auto-spawn: random interval 3–10s, cap 3 alive, spawn above screen, fall to ground.
- Orc auto-move left/right, flips direction at boundary.
- Every 2s: 50/50 orc picks `run` vs `attack` intent; attack picks 50/50 between `attack`/`attack2`.
- Hitbox system for soldier + orc (body boxes + attack boxes gated by frame windows).
- Soldier attack hits orc → orc plays Hurt, bounces, invulnerable during Hurt anim.
- Orc has 2 lives. 2nd hit → Death anim → despawn.
- Orc attack hits soldier → soldier bounces back + upward (airborne), invulnerable until grounded.
- Soldier has 10 lives. 0 lives → Death anim → Game Over screen.
- Game Over: dim screen, "GAME OVER" + "Press R to restart" in monogram font. R resets stage.
- Soldier gets new states: `Hit`, `Death`.
- HUD top-right: animated heart (4-frame loop from row 4 of 4×6 grid) + "x{lives}" in monogram.
- All on-screen text (HUD, Game Over) uses `assets/fonts/monogram/ttf/monogram.ttf`. Debug overlay unchanged.
- Hitbox + combat tuning values live in SQLite, retunable via `make tune`.

### Non-goals (this iteration)
- Sound, menus, pause, scoring, level progression.
- Orc-orc collision or soldier-orc body blocking (they pass through each other; only attack boxes register).
- Hot reload of tuning values.
- More enemy types (storage marks `is_enemy`/`is_player` for future extensibility, but only orc implemented).
- Dash state (remains loaded-but-unbound per existing spec).

---

## 2. Package layout

```
internal/
  anim/        (extended) — loader reads per-spec frame_w/h/path; new SliceGrid for heart
  combat/      NEW — Box type, resolver, HitEvent, Fighter interface
  config/      (changed) — drops SPRITE_FRAME_W/H; ASSETS_DIR now base dir
  debug/       (extended) — orc_count, orc_next_spawn_s, player_lives, player_invulnerable fields; F4 hitbox overlay
  enemy/       NEW — Enemy, EnemyFSM, 7 states (idle, fall, run, attack, attack2, hurt, death)
  game/        (extended) — manages enemies slice, spawner, HUD, GameState (Playing | GameOver)
  hud/         NEW — HUD (heart + lives text), GameOver overlay, monogram font loader
  input/       (unchanged)
  player/      (extended) — Hit + Death states, Lives, HitFlag, Boxes, HitSet, CurrentAnim
  spawner/     NEW — timer, interval roll, cap enforcement
  storage/     (extended) — new migrations 007–013
  world/       (extended) — Clamp(x, halfW)
```

### Import rules (no cycles)
- `combat` imports `anim` only; takes `Fighter` interface so it doesn't know `player`/`enemy`.
- `spawner` imports `enemy` + `anim`.
- `hud` imports `anim` + `combat` (for LivesProvider? no — own interface).
- `game` is the top-level wiring point.

---

## 3. Data model & migrations

All migrations additive; never edit applied ones.

### `007_rename_char1_to_soldier.sql`
Updates `animations.id`: `idle` → `soldier_idle`, etc. for all 7 existing rows.

### `008_animations_schema_v2.sql`
Adds columns to `animations`:
- `frame_w INTEGER NOT NULL DEFAULT 120`
- `frame_h INTEGER NOT NULL DEFAULT 80`
- `path TEXT NOT NULL DEFAULT ''` (relative to new `ASSETS_DIR`)
- `is_player INTEGER NOT NULL DEFAULT 0`
- `is_enemy INTEGER NOT NULL DEFAULT 0`
- `grid_cols INTEGER NOT NULL DEFAULT 0`
- `grid_rows INTEGER NOT NULL DEFAULT 0`
- `pick_row INTEGER NOT NULL DEFAULT 0`

Backfill: `UPDATE animations SET path='soldier/'||file, is_player=1 WHERE id LIKE 'soldier_%'`.

### `009_seed_orc_animations.sql`
Inserts 6 rows (id, file, frame_count, duration_ms, loop, frame_w=100, frame_h=100, path, is_enemy=1):

| id | file | frames | ms | loop |
|---|---|---|---|---|
| orc_idle | Idle.png | 6 | 900 | 1 |
| orc_run | Run.png | 8 | 700 | 1 |
| orc_attack | Attack.png | 6 | 600 | 0 |
| orc_attack2 | Attack2.png | 6 | 700 | 0 |
| orc_hurt | Hurt.png | 4 | 400 | 0 |
| orc_death | Death.png | 4 | 500 | 0 |

### `010_seed_heart_animation.sql`
Single row `heart_beat`: file='HeartsBeat.png', frame_count=4, duration_ms=400, loop=1, frame_w=16, frame_h=16, path='heart/HeartsBeat.png', grid_cols=4, grid_rows=6, pick_row=3 (0-indexed = row 4). Neither player nor enemy.

### `011_hitboxes_schema.sql`
```sql
CREATE TABLE hitboxes (
    id                 TEXT PRIMARY KEY,
    owner              TEXT    NOT NULL,   -- 'soldier' | 'orc'
    kind               TEXT    NOT NULL,   -- 'body' | 'attack' | 'attack2'
    offset_x           INTEGER NOT NULL,   -- relative to sprite anchor (bottom-center)
    offset_y           INTEGER NOT NULL,   -- typically negative (above feet)
    width              INTEGER NOT NULL,
    height             INTEGER NOT NULL,
    active_frame_start INTEGER NOT NULL DEFAULT -1,  -- -1 = always active
    active_frame_end   INTEGER NOT NULL DEFAULT -1
);
```

### `012_seed_hitboxes.sql`

| id | owner | kind | off_x | off_y | w | h | frame_start | frame_end |
|---|---|---|---|---|---|---|---|---|
| soldier_body | soldier | body | -20 | -70 | 40 | 70 | -1 | -1 |
| soldier_attack | soldier | attack | 20 | -60 | 60 | 50 | 1 | 2 |
| soldier_attack2 | soldier | attack2 | 20 | -60 | 80 | 60 | 2 | 4 |
| orc_body | orc | body | -25 | -80 | 50 | 80 | -1 | -1 |
| orc_attack | orc | attack | 25 | -70 | 60 | 60 | 2 | 3 |
| orc_attack2 | orc | attack2 | 25 | -70 | 70 | 60 | 3 | 4 |

Hitbox dims are initial guesses; retune via `make tune` after playtest (requires extending tune CLI to touch `hitboxes` — out of scope for v1; for v1 the tuning table alone is CLI-editable, hitboxes edited via SQL or a new migration).

### `013_seed_combat_tuning.sql`
Adds rows to existing `tuning` table:

| key | value | min | max | unit | description |
|---|---|---|---|---|---|
| orc_hurt_bounce_vx | 120 | 0 | 500 | px/s | horizontal bounce away from attacker |
| orc_hurt_bounce_vy | -180 | -500 | 0 | px/s | vertical pop on hurt |
| soldier_knockback_vx | 200 | 0 | 500 | px/s | horizontal knockback away |
| soldier_knockback_vy | -300 | -600 | 0 | px/s | upward pop (airborne → i-frame) |
| soldier_max_lives | 10 | 1 | 99 | — | starting lives |
| orc_max_lives | 2 | 1 | 10 | — | starting lives |
| orc_spawn_min_s | 3 | 1 | 60 | s | min spawn interval |
| orc_spawn_max_s | 10 | 1 | 60 | s | max spawn interval |
| orc_max_alive | 3 | 1 | 10 | — | max concurrent orcs |
| orc_intent_tick_s | 2 | 0.5 | 10 | s | intent-reroll period |
| orc_run_speed | 80 | 0 | 500 | px/s | orc ground speed |

---

## 4. Config changes

### `.env` / `.env.example`
- `ASSETS_DIR=./assets/sprites` (was `./assets/sprites/char1`).
- Remove: `SPRITE_FRAME_W`, `SPRITE_FRAME_H`.

### `config.Config`
Drop fields `SpriteFrameW`, `SpriteFrameH`. `Load()` no longer reads those env keys. Boot still panics on missing env for remaining keys (`ASSETS_DIR`, `DB_PATH`, window dims, render scale, debug config path).

### `anim/library.go`
- Read `frame_w`, `frame_h`, `path` per spec.
- Resolve asset path: `filepath.Join(cfg.AssetsDir, spec.Path)`.
- If `grid_cols > 0`: call `SliceGrid(img, fw, fh, cols, rows, pickRow, frameCount)`. Else: `Slice(...)` (horizontal strip).
- Size-check: expected width = `frame_w × frame_count` (strip) OR `frame_w × grid_cols` (grid); expected height = `frame_h` (strip) OR `frame_h × grid_rows` (grid).

---

## 5. Combat system (`internal/combat`)

### Types

```go
type Box struct {
    OffsetX, OffsetY int
    W, H             int
    FrameStart, FrameEnd int   // -1,-1 = always active
}

func (b Box) Active(frame int) bool {
    return b.FrameStart < 0 || (frame >= b.FrameStart && frame <= b.FrameEnd)
}

type Fighter interface {
    Pos() (x, y float64)
    Facing() int
    CurrentAnimID() string
    CurrentFrame() int
    Body() Box
    ActiveHits() []Box
    IsInvulnerable() bool
    Alive() bool
}

type HitEvent struct {
    Attacker, Victim Fighter
    AttackKind       string
}
```

### Resolver

```go
func Resolve(attackers []Fighter, victims []Fighter) []HitEvent
```

For each (attacker, victim) pair where both `Alive()`, attacker not invulnerable, victim not invulnerable:
- Compute each active attacker hitbox in world space: `(vx, vy, w, h)` with `vx = ax + facing*OffsetX - (w/2 if facing<0 else 0)` — or equivalently mirror the box by its own width. See implementation note below.
- Compute victim body in world space (same facing flip).
- AABB overlap test.
- Dedup: attacker stores `HitSet map[Fighter]bool`; if `HitSet[victim]` already true, skip. On overlap, set to true and emit `HitEvent`.
- HitSet cleared by the attacker on `attack`/`attack2` state entry.

### Facing flip note
Sprite anchor is bottom-center. For `facing=+1`, world rect = `(x + OffsetX, y + OffsetY, W, H)`. For `facing=-1`, world rect = `(x - OffsetX - W, y + OffsetY, W, H)` — mirrors across the anchor.

### Dispatch (in `game.Update`)

After `Resolve`:
- For each event with `Attacker` being the soldier and `Victim` a live orc: call `orc.OnHit(event)`. Orc:
    - `Lives--`; if `Lives <= 0` → enter `death`; else → enter `hurt` (stops VX, applies bounce away from attacker's X, sets invulnerable).
- For each event with `Attacker` being an orc and `Victim` the soldier:
    - Soldier: `Lives--`; if `Lives <= 0` → enter `death`; else → enter `hit` (applies knockback, airborne, invulnerable until grounded).

### Load

`combat.LoadBoxes(repo) (map[owner]map[kind]Box, error)` at boot. Player and Enemy each receive a slice of their boxes keyed by kind.

---

## 6. Player extensions (`internal/player`)

### New fields
```go
Lives       int
HitFlag     bool                     // true during i-frame (hit → grounded)
Boxes       map[string]combat.Box    // "body", "attack", "attack2"
CurrentAnim string
HitSet      map[combat.Fighter]bool  // per-attack dedup
```

### New states

**`hitState`**
- On enter: plays `soldier_hit` (1 frame, held). Sets `HitFlag=true`. Applies knockback: `VX = -Facing * soldier_knockback_vx`; `VY = soldier_knockback_vy`; `Grounded = false`. Invulnerable while `HitFlag` true.
- Update: physics runs normally. On `Grounded` true: clear `HitFlag`, transition → `idle`.
- Intents ignored while in `hit`.

**`deathState`**
- On enter: plays `soldier_death` (10 frames, 1.0s). VX=0, VY=0 (let gravity ground it if airborne — actually set VX=0, allow VY until grounded, then lock).
- Update: on anim `Done()` → signal game state via flag; FSM stays in `death` (terminal).
- Intents ignored.

### Entry triggers
- `hit`: only from combat dispatch (not intent-driven). Can interrupt any state except `death`.
- `death`: triggered when `Lives <= 0` is observed post-hit; interrupts everything.

### Fighter impl
- `Pos() = (X, Y)`
- `Facing() = Facing`
- `CurrentAnimID() = CurrentAnim` (set by each state on entry alongside `PlayAnim`)
- `CurrentFrame() = Current.FrameIndex()` (may need small Animation method added)
- `Body() = Boxes["body"]`
- `ActiveHits()` — if `CurrentAnim == "soldier_attack"` and `"attack"` box active for frame → return `[attackBox]`; same for attack2; else empty.
- `IsInvulnerable() = HitFlag || state == "death"`
- `Alive() = Lives > 0 && state != "death"` (or `!DeathDone`)

### Boundary clamp
Applied in `game.Update` after `ApplyPhysics`:
```go
halfW := (spriteW * renderScale) / 2
player.X = world.Clamp(player.X, halfW, windowW - halfW)
```

---

## 7. Enemy (`internal/enemy`)

### Struct
```go
type Enemy struct {
    X, Y, VX, VY  float64
    Facing        int
    Grounded      bool
    Lives         int
    RunSpeed      float64
    Physics       *player.Physics   // reused: same gravity/max fall
    FSM           *FSM
    Anims         map[string]*anim.Animation
    Boxes         map[string]combat.Box
    Current       *anim.Animation
    CurrentAnim   string
    IntentTimer   float64
    HitSet        map[combat.Fighter]bool
    Dead          bool              // set when death anim done; spawner culls
}
```

### States (all `struct{}`, value-receiver methods)
- `fall` — airborne after spawn. Displays `orc_idle` frame 0 frozen (just sets current anim without updating). Transition: `Grounded` → `run` (pick random Facing ±1).
- `run` — plays `orc_run`, VX = `Facing * RunSpeed`. Boundary flip (see below). On `IntentTimer ≤ 0`: reroll.
- `attack` — VX=0, plays `orc_attack`. Anim done → `run` (keep Facing).
- `attack2` — VX=0, plays `orc_attack2`. Anim done → `run` (keep Facing).
- `hurt` — VX set to `sign(attacker.X - self.X) * -1 * orc_hurt_bounce_vx` (away), VY = `orc_hurt_bounce_vy`, Grounded=false. Invulnerable. Anim done AND Grounded → `run` with NEW random Facing.
- `death` — VX=0, plays `orc_death`. Anim done → `Dead=true`.

### Intent reroll (in `run` state `Update`)
```
IntentTimer -= dt
if IntentTimer <= 0:
    IntentTimer = orc_intent_tick_s
    if rand.Float64() < 0.5: keep running
    else:
        pick attack OR attack2 (50/50) → transition
```

### Boundary flip (in `run`)
After physics + clamp, in `Update`:
```
if X <= halfW && Facing < 0: Facing = +1
if X >= windowW - halfW && Facing > 0: Facing = -1
```

### Fighter impl
Same pattern as Player. `CurrentAnimID` maps anim → kind ("orc_attack" → ActiveHits returns attack box; similarly attack2). Body always returned.

### Damage handling
`Enemy.OnHit(event HitEvent)` called by game after `Resolve`:
- `Lives--`. If `Lives <= 0` → transition to `death`. Else → transition to `hurt`.

### Physics
Enemy reuses `player.Physics` struct (gravity, max fall speed — sprint values ignored) and has its own `ApplyPhysics(world, dt)` method mirroring `player.Player.ApplyPhysics`: gravity accumulation, clamp to MaxFallSpeed, ground-plane resolution at `world.GroundY`. No horizontal friction (VX driven entirely by state).

### HitSet clearing
Cleared on entry to `attack` or `attack2` state (same rule as soldier).

---

## 8. Spawner (`internal/spawner`)

```go
type Spawner struct {
    MinInterval, MaxInterval float64   // seconds, from tuning
    MaxAlive                 int
    NextSpawn                float64   // countdown in seconds
    SpawnXRange              [2]float64
    rng                      *rand.Rand
    NewEnemy                 func(x, y float64) *enemy.Enemy
}

func New(...) *Spawner
func (s *Spawner) Tick(dt time.Duration, alive int) *enemy.Enemy
```

`Tick`:
1. `NextSpawn -= dt.Seconds()`.
2. If `NextSpawn > 0` → return `nil`.
3. `NextSpawn = rollInterval()`.
4. If `alive >= MaxAlive` → return `nil` (skip; timer re-rolled).
5. `x = rollSpawnX()`; `y = -orc_sprite_h * renderScale`.
6. Return `s.NewEnemy(x, y)`.

Game appends result to `enemies []*enemy.Enemy`. Culls `Dead` orcs each frame.

RNG: seeded with `time.Now().UnixNano()` at boot. Tests inject deterministic seed.

---

## 9. HUD + font (`internal/hud`)

### Font loader
```go
var source *text.GoTextFaceSource

func LoadFont(path string) error   // called at boot, reads monogram.ttf
func NewFace(size float64) *text.GoTextFace
```

### HUD
```go
type LivesProvider interface { Lives() int }

type HUD struct {
    Heart     *anim.Animation    // heart_beat anim
    Face      *text.GoTextFace   // monogram @ 32
    Provider  LivesProvider
    Scale     float64            // render scale for heart, default 3
    WindowW   int
}

func (h *HUD) Update(dt time.Duration)
func (h *HUD) Draw(screen *ebiten.Image)
```

Layout (anchored top-right, 16px padding):
- Measure text width of `fmt.Sprintf("x%d", lives)` with face.
- Text position: right edge at `WindowW - 16`.
- Heart drawn immediately left of text with 8px gap, size `16*Scale`.
- Both vertically aligned with top padding 16.
- `FilterNearest` for both.

### GameOver overlay
```go
type GameOver struct {
    Title    *text.GoTextFace  // size 96
    Subtitle *text.GoTextFace  // size 32
    WindowW, WindowH int
}

func (g *GameOver) Draw(screen *ebiten.Image)
```

Draws:
- Full-screen dim rect (black, alpha 128).
- "GAME OVER" centered horizontally, vertically ~40% from top.
- "Press R to restart" centered, ~55% from top.

---

## 10. Game wiring (`internal/game`)

### New fields
```go
enemies       []*enemy.Enemy
spawner       *spawner.Spawner
hud           *hud.HUD
gameOver      *hud.GameOver
state         GameState
hitboxDebug   bool

type GameState int
const ( Playing GameState = iota; GameOverState )
```

### Update() flow
1. If `F3` pressed → toggle debug overlay. If `F4` → toggle hitbox debug. If `state == GameOverState && R` → reset.
2. If `state == GameOverState` → return (no updates).
3. `intent := input.Poll()` (ignored by soldier if in `hit`/`death`).
4. Run soldier FSM + all enemy FSMs.
5. `player.ApplyPhysics(world, dt)` and `enemy.ApplyPhysics(world, dt)` for each enemy.
6. Clamp X (soldier + enemies).
7. `enemy.Run`-state boundary flip check.
8. `spawner.Tick(dt, len(enemies))` → append result.
9. `events := combat.Resolve([soldier], enemies_as_fighters)` → dispatch (soldier hits orc).
10. `events := combat.Resolve(enemies, [soldier])` → dispatch (orc hits soldier).
11. Cull `Dead` enemies.
12. If soldier `state == death && anim.Done()` → `gameState = GameOverState`.
13. `hud.Update(dt)`.

### Draw() flow
1. Fill background + ground rect (existing).
2. Draw enemies (sorted by Y ascending for pseudo-depth).
3. Draw soldier.
4. Draw HUD.
5. If `hitboxDebug`: draw boxes (red = attack, green = body).
6. Draw debug overlay (F3).
7. If `state == GameOverState`: draw GameOver on top.

### Reset (on R)
- Clear `enemies`.
- Reset `spawner.NextSpawn`.
- Recreate `player` at center with full lives, Idle state.
- `state = Playing`.

---

## 11. Debug overlay

Extend `internal/debug/fields.go` catalog with:
- `orc_count` — `len(game.enemies)`
- `orc_next_spawn_s` — `spawner.NextSpawn`
- `player_lives` — `player.Lives`
- `player_invulnerable` — `player.HitFlag`

Update `config/debug.json` to include these in layout (optional — they're available but not shown until user adds them).

New input: `F4` toggles `hitboxDebug` flag on Game.

---

## 12. Tests

- `combat/resolve_test.go` — AABB truth table, facing flip, frame-window gating, invul skip, dedup.
- `enemy/fsm_test.go` — fall→run on land, run→attack on intent tick, boundary flip, hurt→run random dir, death terminal.
- `spawner/spawner_test.go` — interval bounds, cap enforcement, re-roll when at cap.
- `player/fsm_test.go` (additions) — hit state entry on combat event, hit→idle on ground, death on Lives=0.
- `hud/hud_test.go` — "x{N}" formatting, heart grid frame pick.
- `anim/slice_test.go` — SliceGrid picks correct row/col.
- `storage/migrations_test.go` — schema cols + seeded row counts after migrations 007–013.

Manual verify (Ebiten rendering):
- Soldier rename: game runs unchanged with new asset path.
- Heart HUD animates, count decrements on damage.
- Game Over renders with monogram font at size 96 + 32.
- Hitbox overlay draws expected boxes per state.

---

## 13. Risks / open items

1. Ebiten `text/v2` API version — confirm `go.mod` supports it. If not, fall back to legacy `text` pkg using `opentype` face. Implementation plan must check first.
2. Hitbox default values seeded are rough guesses. Expect playtest iteration. Retuning hitboxes currently requires new migration (tune CLI edits `tuning` table only, not `hitboxes`). Extending CLI is out of scope for v1.
3. Heart scale @ 3× → 48px may feel small on 1280×720; revisit during manual verify.
4. Orc-orc no collision by design. They can overlap; visually fine given shared Y depth sort.
5. Soldier attack dedup is per-swing, keyed by victim pointer — survives cleared HitSet on attack state entry. One swing can damage multiple orcs (each once).
6. Orc hurt anim must be long enough that i-frame doesn't reset mid-attack-swing from soldier. 400ms covers soldier's Attack window (~250ms worth of active frames). If feels off, retune `orc_hurt` duration_ms via new migration.

---

## 14. Sequence of implementation (high-level)

Detailed task breakdown goes into the implementation plan. At a glance:

1. Rename char1 → soldier (assets + migrations + env).
2. Schema migrations 008–013 (animations v2, hitboxes table, tuning keys).
3. Seed orc + heart animations; extend loader with frame_w/h/path + grid slicer.
4. `internal/combat` skeleton (Box, Fighter, Resolve).
5. `internal/enemy` with FSM + states.
6. `internal/spawner`.
7. Extend player: Hit + Death states, Boxes, Lives.
8. Boundary clamp (world.Clamp + game integration).
9. Font loader + HUD + GameOver overlay.
10. Game wiring: spawner tick, combat dispatch, reset.
11. Debug overlay fields + F4 hitbox toggle.
12. Tests.
13. Manual verify pass.
