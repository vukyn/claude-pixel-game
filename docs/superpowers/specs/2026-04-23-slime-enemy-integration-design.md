# Slime enemy integration + multi-kind spawner

**Date:** 2026-04-23
**Status:** Approved — ready for planning

## Goal

Add slime as a second enemy kind alongside orc. Spawner picks uniformly among declared kinds. Total concurrent enemy cap stays at 3. Slime's Attack2 includes a per-frame-window horizontal displacement (backstep) — a reusable mechanic applicable to any attack state of any kind.

## Non-goals

- No new player mechanics.
- No per-kind AI differences beyond motion hook (slime uses same run/attack/attack2/hurt/death FSM as orc for now).
- No spawn-weight tuning UI yet (weight field seeded at 1 for both kinds, ready for future use).
- No hot reload; tuning changes require `make run` restart.

## Source assets

`assets/sprites/slime/*.png` — all 96×96 frames:

| File         | Frames | Loop | Notes                                         |
|--------------|--------|------|-----------------------------------------------|
| Idle.png     | 6      | yes  | 576×96                                        |
| Run.png      | 8      | yes  | 768×96                                        |
| Attack.png   | 8      | no   | 768×96, hitbox active frames 4–5 (0-indexed)  |
| Attack2.png  | 8      | no   | 768×96, hitbox active frames 3–5 (0-indexed)  |
| Hurt.png     | 4      | no   | 384×96                                        |
| Death.png    | 10     | no   | 960×96                                        |

(`Block.png`, `Jump.png` exist but unused.)

User-provided frame indices (`Attack: 5,6` / `Attack2: 4,5,6`) were 1-indexed. Stored values are 0-indexed to match `hitboxes.active_frame_*` convention.

## Data model

### New tables

**`attack_motions`** — per-attack horizontal displacement:

```sql
CREATE TABLE attack_motions (
    id                 TEXT    PRIMARY KEY,
    owner              TEXT    NOT NULL,       -- 'orc' | 'slime' | ...
    kind               TEXT    NOT NULL,       -- 'attack' | 'attack2'
    vx                 INTEGER NOT NULL,       -- signed px/s; + = forward (along facing), - = backward
    frame_start        INTEGER NOT NULL,       -- 0-indexed inclusive
    frame_end          INTEGER NOT NULL        -- 0-indexed inclusive
);
```

Semantics: during attack/attack2 state, if `current_frame ∈ [frame_start, frame_end]`, enemy's `VX = facing * vx`; otherwise 0. No row for (owner, kind) = no motion (existing orc behavior unchanged).

### Existing tables extended

- `animations` — seed 6 slime rows.
- `hitboxes` — seed 3 slime rows (body, attack, attack2).
- `tuning` — seed 6 slime keys + rename 3 orc spawn keys to `enemy_*` (moved to global scope).

## Migrations (order preserved, additive)

1. `017_seed_slime_animations.sql` — 6 INSERTs into `animations`, frame_w=frame_h=96, `path='slime/<File>.png'`, `is_enemy=1`.
2. `018_seed_slime_hitboxes.sql`:
   - `slime_body` — offset=(-20,-40), w=40, h=40, frame=-1/-1
   - `slime_attack` — offset=(15,-35), w=45, h=35, frame=4/5
   - `slime_attack2` — offset=(15,-35), w=55, h=40, frame=3/5
   - (Dims predefined rough; user tunes via `tune hitboxes set` after visual verify.)
3. `019_attack_motions_schema.sql` — `CREATE TABLE attack_motions`.
4. `020_seed_slime_attack_motion.sql` — `slime_attack2_motion`: owner=slime, kind=attack2, vx=-60, frame_start=3, frame_end=5.
5. `021_rename_spawn_keys.sql` — copy+delete:
   - `orc_spawn_min_s` → `enemy_spawn_min_s`
   - `orc_spawn_max_s` → `enemy_spawn_max_s`
   - `orc_max_alive` → `enemy_max_alive` (description updated to "max concurrent enemies (all kinds)")
6. `022_seed_slime_tuning.sql`:
   - `slime_max_lives=2` (min=1, max=10)
   - `slime_run_speed=60` px/s (min=0, max=500)
   - `slime_intent_tick_s=2` s (min=0.5, max=10)
   - `slime_hurt_bounce_vx=120` px/s (min=0, max=500)
   - `slime_hurt_bounce_vy=-180` px/s (min=-500, max=0)
   - `slime_foot_padding=20` px (min=0, max=96)

No migration edits orc rows — orc configuration preserved byte-identical except the 3 spawn keys move under `enemy_*`.

## Code refactor — enemy package

### New: `internal/enemy/kind.go`

```go
type Kind struct {
    Name       string                     // "orc" | "slime"
    AnimPrefix string                     // same as Name for current kinds
    FrameW     int
    FrameH     int
    Tuning     *Tuning
    Boxes      map[string]combat.Box      // "body" | "attack" | "attack2", pre-scaled
    Anims      map[string]*anim.Animation // unprefixed keys: "idle","run","attack","attack2","hurt","death"
    Motions    map[string]AttackMotion    // optional; key "attack" | "attack2"
}

type AttackMotion struct {
    VX         float64
    FrameStart int
    FrameEnd   int
}

type KindConfig struct {
    Name        string
    Prefix      string
    FrameW      int
    FrameH      int
    AnimLib     map[string]*anim.Animation
    HitboxSpecs []combat.HitboxSpec
    MotionSpecs []combat.AttackMotionSpec
    TuneRepo    *storage.Repository[player.TuningParam]
    RenderScale int
}

func BuildKind(cfg KindConfig) (*Kind, error) // wraps AnimsFor + BoxesFor + MotionsFor + LoadTuningFor
```

### Refactor: `internal/enemy/enemy.go`

```go
type Config struct {
    StartX, StartY float64
    Physics        *player.Physics
    Kind           *Kind
    RNG            *rand.Rand
}

type Enemy struct {
    X, Y, VX, VY float64
    Facing       int
    Grounded     bool
    Lives        int
    Physics      *player.Physics
    Kind         *Kind   // replaces Tuning/Anims/Boxes fields
    FSM          *FSM
    Current      *anim.Animation
    CurrentAnim  string  // unprefixed
    IntentTimer  float64
    HitSet       map[combat.Fighter]bool
    Dead         bool
    rng          *rand.Rand
}
```

`New` initializes `Lives = int(cfg.Kind.Tuning.MaxLives)`, `RunSpeed` inlined as `cfg.Kind.Tuning.RunSpeed` where needed. All `e.Tuning.*` reads become `e.Kind.Tuning.*`. All `e.Anims[...]` reads become `e.Kind.Anims[...]`. All `e.Boxes[...]` reads become `e.Kind.Boxes[...]`.

### Refactor: `internal/enemy/states.go`

Replace every `"orc_<state>"` string with the unprefixed variant:

```go
fallState.Enter:    e.PlayAnim("idle")      // was "orc_idle"
runState.Enter:     e.PlayAnim("run")
attackState.Enter:  e.PlayAnim("attack")
attack2State.Enter: e.PlayAnim("attack2")
hurtState.Enter:    e.PlayAnim("hurt")
deathState.Enter:   e.PlayAnim("death")
```

Add motion hook in `attackState.Update` + `attack2State.Update`:

```go
func (attack2State) Update(e *Enemy, dt time.Duration) StateID {
    applyMotion(e, "attack2")
    if e.Current != nil && e.Current.Done() {
        return StateRun
    }
    return StateAttack2
}

func applyMotion(e *Enemy, kind string) {
    m, ok := e.Kind.Motions[kind]
    if !ok {
        // no motion configured — VX already 0 from Enter
        return
    }
    f := e.CurrentFrame()
    if f >= m.FrameStart && f <= m.FrameEnd {
        e.VX = float64(e.Facing) * m.VX
    } else {
        e.VX = 0
    }
}
```

Same call shape in `attackState.Update`. Orc has no motions, so `applyMotion` is a no-op for orc.

### Refactor: `internal/enemy/fighter.go`

```go
func (e *Enemy) Body() combat.Box { return e.Kind.Boxes["body"] }

func (e *Enemy) ActiveHits() []combat.Box {
    switch e.CurrentAnim {
    case "attack":
        b := e.Kind.Boxes["attack"]
        if b.Active(e.CurrentFrame()) { return []combat.Box{b} }
    case "attack2":
        b := e.Kind.Boxes["attack2"]
        if b.Active(e.CurrentFrame()) { return []combat.Box{b} }
    }
    return nil
}
```

### Refactor: `internal/enemy/loader.go`

Replace `OrcAnims` / `OrcBoxes` with generic:

```go
func AnimsFor(lib map[string]*anim.Animation, prefix string) (map[string]*anim.Animation, error) {
    want := []string{"idle","run","attack","attack2","hurt","death"}
    out := make(map[string]*anim.Animation, len(want))
    for _, k := range want {
        id := prefix + "_" + k
        a, ok := lib[id]
        if !ok {
            return nil, fmt.Errorf("%s anims: missing %q", prefix, id)
        }
        out[k] = a
    }
    return out, nil
}

func BoxesFor(specs []combat.HitboxSpec, owner string, scale int) (map[string]combat.Box, error) {
    out := make(map[string]combat.Box, 3)
    for _, s := range specs {
        if s.Owner != owner { continue }
        out[s.Kind] = s.ToBox().Scale(scale)
    }
    if _, ok := out["body"]; !ok {
        return nil, fmt.Errorf("%s hitboxes: missing body", owner)
    }
    return out, nil
}

func MotionsFor(specs []combat.AttackMotionSpec, owner string) map[string]AttackMotion {
    out := map[string]AttackMotion{}
    for _, s := range specs {
        if s.Owner != owner { continue }
        out[s.Kind] = AttackMotion{VX: s.VX, FrameStart: s.FrameStart, FrameEnd: s.FrameEnd}
    }
    return out
}
```

Old `OrcAnims`/`OrcBoxes` deleted (no consumers after refactor).

### Refactor: `internal/enemy/tuning.go`

```go
type Tuning struct {
    MaxLives     float64
    RunSpeed     float64
    IntentTickS  float64
    HurtBounceVX float64
    HurtBounceVY float64
    FootPadding  int
}

type SpawnTuning struct {
    MinS     float64
    MaxS     float64
    MaxAlive int
}

func LoadTuningFor(repo *storage.Repository[player.TuningParam], prefix string) (*Tuning, error)
func LoadSpawnTuning(repo *storage.Repository[player.TuningParam]) (*SpawnTuning, error)
```

`LoadTuningFor` reads 6 keys: `<prefix>_max_lives`, `<prefix>_run_speed`, `<prefix>_intent_tick_s`, `<prefix>_hurt_bounce_vx`, `<prefix>_hurt_bounce_vy`, `<prefix>_foot_padding`.

`LoadSpawnTuning` reads 3 keys: `enemy_spawn_min_s`, `enemy_spawn_max_s`, `enemy_max_alive`.

Old `LoadTuning` deleted.

## Code refactor — spawner package

### `internal/spawner/spawner.go`

```go
type KindFactory struct {
    Name     string
    Weight   int                                 // relative weight; uniform = 1 for all
    NewEnemy func(x, y float64) *enemy.Enemy     // factory handles its own per-kind clamp + SpawnY
}

type Config struct {
    MinIntervalS float64
    MaxIntervalS float64
    MaxAlive     int
    SpawnXMin    float64     // global window bounds (0..WindowW)
    SpawnXMax    float64
    RNG          *rand.Rand
    Kinds        []KindFactory
}

type Spawner struct { /* …, kinds []KindFactory, totalWeight int */ }

func (s *Spawner) Tick(dt time.Duration, alive int) *enemy.Enemy {
    s.nextSpawn -= dt.Seconds()
    if s.nextSpawn > 0 { return nil }
    s.nextSpawn = s.rollInterval()
    if alive >= s.MaxAlive { return nil }
    k := s.pickKind()
    return k.NewEnemy(s.rollSpawnX(), 0) // y ignored; factory sets its own StartY
}

func (s *Spawner) pickKind() KindFactory {
    if len(s.kinds) == 1 { return s.kinds[0] }
    r := s.rng.Intn(s.totalWeight)
    for _, k := range s.kinds {
        if r < k.Weight { return k }
        r -= k.Weight
    }
    return s.kinds[len(s.kinds)-1]
}
```

Removed: `SpawnY`, single `NewEnemy` field. Factory closure embeds per-kind `SpawnY` (= `-FrameH * RenderScale`).

## Code refactor — game package

### `internal/game/game.go`

`Deps` gains `EnemyKinds []*enemy.Kind` and `SpawnTuning *enemy.SpawnTuning`, drops `OrcAnims`/`OrcBoxes`/`OrcTuning`.

```go
type Deps struct {
    Cfg          *config.Config
    Anims        map[string]*anim.Animation
    Physics      *player.Physics
    DebugCfg     *debug.Config
    SoldierBoxes map[string]combat.Box
    CombatTuning *combat.Tuning
    EnemyKinds   []*enemy.Kind
    SpawnTuning  *enemy.SpawnTuning
    HeartAnim    *anim.Animation
    HUDFace      *text.GoTextFace
    OverTitle    *text.GoTextFace
    OverSubtitle *text.GoTextFace
}
```

Spawner init inside `New`:

```go
kindFactories := make([]spawner.KindFactory, 0, len(d.EnemyKinds))
for _, k := range d.EnemyKinds {
    k := k
    halfW := float64(k.Boxes["body"].W) / 2
    spriteH := float64(k.FrameH * d.Cfg.RenderScale)
    kindFactories = append(kindFactories, spawner.KindFactory{
        Name:   k.Name,
        Weight: 1,
        NewEnemy: func(x, _ float64) *enemy.Enemy {
            if x < halfW { x = halfW }
            if max := float64(d.Cfg.WindowW) - halfW; x > max { x = max }
            return enemy.New(enemy.Config{
                StartX: x, StartY: -spriteH,
                Physics: d.Physics,
                Kind:    k,
                RNG:     rng,
            })
        },
    })
}

g.spawner = spawner.New(spawner.Config{
    MinIntervalS: d.SpawnTuning.MinS,
    MaxIntervalS: d.SpawnTuning.MaxS,
    MaxAlive:     d.SpawnTuning.MaxAlive,
    SpawnXMin:    0,
    SpawnXMax:    float64(d.Cfg.WindowW),
    RNG:          rng,
    Kinds:        kindFactories,
})
```

Renderer `drawEnemy` reads per-kind dims:

```go
func (g *Game) drawEnemy(screen *ebiten.Image, e *enemy.Enemy) {
    if e.Current == nil || e.Current.CurrentFrame() == nil { return }
    pad := e.Kind.Tuning.FootPadding
    fw := float64(e.Kind.FrameW)
    fh := float64(e.Kind.FrameH)
    op := &ebiten.DrawImageOptions{}
    op.GeoM.Translate(-fw/2, -fh+float64(pad))
    if e.Facing < 0 { op.GeoM.Scale(-1, 1) }
    op.GeoM.Scale(float64(g.cfg.RenderScale), float64(g.cfg.RenderScale))
    op.GeoM.Translate(e.X, e.Y)
    op.Filter = ebiten.FilterNearest
    screen.DrawImage(e.Current.CurrentFrame(), op)
}
```

Clamp loop in `Update` reads `e.Kind.Boxes["body"].W`.

`Game.orcAnims/orcBoxes/orcTuning` fields deleted; `reset()` does not need them (player reset unchanged).

### Debug overlay

`game.OrcCount()` → `game.EnemyCount()`. Catalog key `orc_count` → `enemy_count` (label `"Enemies: %d"`). `config/debug.json` updated to reference new key.

## Code refactor — combat package

### New: `internal/combat/motion.go`

```go
type AttackMotionSpec struct {
    ID         string
    Owner      string
    Kind       string
    VX         float64
    FrameStart int
    FrameEnd   int
}

type AttackMotionMapper struct{}
func (AttackMotionMapper) Table() string     { return "attack_motions" }
func (AttackMotionMapper) Columns() []string { return []string{"id","owner","kind","vx","frame_start","frame_end"} }
func (AttackMotionMapper) Scan(row storage.Scanner) (AttackMotionSpec, error)
func (AttackMotionMapper) Values(s AttackMotionSpec) []any
```

## `cmd/game/main.go` wiring

```go
motionRepo := storage.NewRepository[combat.AttackMotionSpec](db, combat.AttackMotionMapper{})
motionSpecs, err := motionRepo.List(ctx)
hitboxSpecs, err := hitboxRepo.List(ctx)

orcKind, err := enemy.BuildKind(enemy.KindConfig{
    Name: "orc", Prefix: "orc", FrameW: 100, FrameH: 100,
    AnimLib: anims, HitboxSpecs: hitboxSpecs, MotionSpecs: motionSpecs,
    TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
})
slimeKind, err := enemy.BuildKind(enemy.KindConfig{
    Name: "slime", Prefix: "slime", FrameW: 96, FrameH: 96,
    AnimLib: anims, HitboxSpecs: hitboxSpecs, MotionSpecs: motionSpecs,
    TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
})
spawnTuning, err := enemy.LoadSpawnTuning(tuneRepo)

g := game.New(game.Deps{
    /* ... */
    EnemyKinds:  []*enemy.Kind{orcKind, slimeKind},
    SpawnTuning: spawnTuning,
    /* ... */
})
```

## `cmd/tune/main.go` — `motions` subcommand

Mirror of `hitboxesCmd`. Repo: `storage.NewRepository[combat.AttackMotionSpec](db, combat.AttackMotionMapper{})`.

Commands:

- `motions list` — columns `ID OWNER KIND VX FRAME_START FRAME_END`.
- `motions get <id>` — one-line print.
- `motions set <id> <field> <value>` — fields: `owner, kind, vx, frame_start, frame_end`.
- `motions add <id> <owner> <kind> <vx> <frame_start> <frame_end>` — upsert.
- `motions delete <id>`.

Helper `applyMotionField` parallels `applyHitboxField`. `formatMotion` parallels `formatHitbox`.

## Testing

### Unit

- **`internal/enemy/fsm_test.go`**:
  - `newOrcTestEnemy` — builds `Kind{Name:"orc", FrameW:100, FrameH:100, Anims:<unprefixed>, Boxes:<orc>, Motions:nil, Tuning:<orc>}`.
  - `newSlimeTestEnemy` — same shape, slime values, `Motions: {"attack2": {VX:-60, FrameStart:3, FrameEnd:5}}`.
  - Existing 4 tests parametrize over both kinds via table-driven form.
  - `TestSlimeAttack2BackstepAppliesVXOnActiveFrames` — force `StateAttack2`, advance anim via explicit frame stepping helper, assert:
    - before FrameStart: VX=0
    - at FrameStart: VX = facing * -60
    - mid-window: VX = facing * -60
    - after FrameEnd: VX=0
  - `TestOrcAttack2NoMotionKeepsVXZero` — regression: attack2 state with `Motions==nil` leaves VX untouched at 0.
- **`internal/enemy/loader_test.go`** (new):
  - `AnimsFor` returns unprefixed map for known prefix.
  - `AnimsFor` errors on missing key.
  - `BoxesFor` filters by owner, scales, errors on missing body.
  - `MotionsFor` filters by owner, returns empty map on no matches (not error).
- **`internal/spawner/spawner_test.go`**:
  - Existing tests updated to construct `Kinds: []KindFactory{{Name:"test", Weight:1, NewEnemy:...}}`.
  - New `TestSpawnerPicksAmongKindsUniformly` — 2 fake kinds, 1000 forced spawns, assert both invoked and frequencies within ±20% of 50/50.

### Manual verify (T-list to run in plan exec)

1. `rm -rf data/ && make run` — fresh DB, 22 migrations apply cleanly.
2. Orc regression: spawns, runs, attacks, takes 2 hits, dies, removed — behavior identical to pre-refactor.
3. Slime spawns, falls to ground, runs, occasionally triggers attack/attack2, dies after 2 soldier hits.
4. F4 debug: slime body/attack/attack2 boxes render green/red at correct positions.
5. Slime Attack2: on frames 3–5 of the anim, slime visibly slides backward (opposite of facing). Before/after window: static.
6. Enemy cap: rapid RNG never produces >3 concurrent enemies across kinds.
7. `make tune ARGS="list"` shows: `slime_*` keys (6), `enemy_spawn_min_s`, `enemy_spawn_max_s`, `enemy_max_alive`. No `orc_spawn_*` / `orc_max_alive`. Orc `orc_max_lives`, `orc_run_speed`, etc. preserved.
8. `make tune ARGS="motions list"` shows `slime_attack2_motion` row.
9. `make tune ARGS="motions set slime_attack2_motion vx -120"` → restart → slime backstep speed doubles visibly.
10. `make tune ARGS="hitboxes set slime_body width 30"` still works (existing CRUD untouched).
11. F3 debug overlay shows `Enemies: N` (renamed from `Orcs: N`).

## Risks / open items

- **Slime foot_padding guess** (20) may render feet above/below ground — tunable post-visual-check via `tune set slime_foot_padding <px>`.
- **Slime hitbox dims** are rough guesses; user tunes via existing hitbox CLI after visual verify.
- **Attack motion on orc attack1** is a future extension — current orc has no motion rows, so `applyMotion` short-circuits. No behavior change.
- **Unprefixed anim map** is an enemy-internal convention. Library-level anim IDs (`slime_idle`, etc.) remain prefixed in DB + library — only the map inside `Enemy.Kind.Anims` is unprefixed.

## Follow-ups (out of scope, noted for future)

- Per-kind `SpawnWeight` in `tuning` table (e.g., `orc_spawn_weight=2`, `slime_spawn_weight=1`) — plumbing already accepts weight, just not surfaced.
- Slime-specific AI (e.g., shorter intent tick on low health) — current FSM is shared.
- Extending attack motion to soldier/player attacks — currently enemy-only; would require mirroring `applyMotion` in `player` package.
