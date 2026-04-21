# Char1 Controller — Design Spec

**Date:** 2026-04-21
**Status:** Draft for user review
**Scope:** First playable demo: a single character (`char1`) with 7 animations, a keyboard-driven state machine, gravity + flat ground, tunable physics via SQLite, and a config-driven debug overlay.

---

## 1. Goals & Non-goals

### Goals
- Render `char1` sprite sheets from `assets/sprites/char1/*.png` using [ebiten v2](https://github.com/hajimehoshi/ebiten).
- Slice each horizontal strip into individual frames using a configurable frame size.
- Drive the character through a **state machine** over 7 states (Idle, Run, Jump, Fall, Dash, Attack, Attack2) with "responsive fighter" transition rules (attack cancelable by jump/dash, air-dash allowed, no double-jump).
- Keyboard controls with both WASD+JKL and Arrows+Space/Shift/X/C accepted.
- Simple mock world: gray background, gray ground slab, gravity pulling the character down to the ground line.
- Animation library and physics tuning values live in **SQLite**, loaded at boot.
- Small **CLI** (`cmd/tune`, `urfave/cli/v3`) to update tuning values (update-only, validated against min/max).
- **Debug overlay** toggled with F3; which fields appear is driven by a JSON config file.
- All operational paths (DB path, assets dir, frame size, window size, debug config path) are configured via `.env` (loaded by `godotenv`); missing env keys panic at boot.

### Non-goals (this iteration)
- Enemies, collisions other than ground, hitboxes.
- Additional animations beyond the 7 listed in `plan-animation.md` (the library is designed so that adding a row to the seed migration is enough).
- Sound, UI, menus, save/load, level design.
- Hot reload of `debug.json` or tuning changes (tuning changes require game restart; deliberate YAGNI).

---

## 2. Animation inventory (from `plan-animation.md`)

All sheets are horizontal strips, frame size **120 × 80 px** (folder is named `120x80_PNGSheets` and each width matches `FrameCount × 120`, height 80). Frame size is NOT hard-coded — it comes from env.

| Animation | Frames | Duration | Loop | File |
|---|---|---|---|---|
| Idle | 10 | 1.0 s | yes | `_Idle.png` |
| Run | 10 | 1.0 s | yes | `_Run.png` |
| Jump | 3 | 0.5 s | no | `_Jump.png` |
| Fall | 3 | 0.5 s | no (clamp last frame) | `_Fall.png` |
| Dash | 2 | 0.5 s | no | `_Dash.png` |
| Attack | 4 | 1.5 s | no | `_Attack.png` |
| Attack2 | 6 | 1.5 s | no | `_Attack2.png` |

Frame duration = `Duration / FrameCount` (uniform). Non-loop animations clamp on the final frame when elapsed ≥ duration and signal `Done()`.

---

## 3. Architecture overview

```
claude-pixel/
├── .env                          # runtime config, committed
├── .env.example                  # template
├── cmd/
│   ├── game/main.go              # ebiten entry point
│   └── tune/main.go              # CLI entry point (urfave/cli/v3)
├── config/
│   └── debug.json                # debug overlay layout, committed
├── data/
│   └── game.db                   # SQLite, git-ignored, regenerable from migrations
├── docs/
│   └── superpowers/specs/…
├── assets/
│   └── sprites/char1/*.png
└── internal/
    ├── config/                   # godotenv loader, Config struct, mustString/mustInt
    ├── storage/
    │   ├── sqlite.go             # Open + migrate + seed
    │   ├── migrations/           # embedded *.sql files
    │   └── repository.go         # generic Repository[T] + Mapper[T]
    ├── anim/
    │   ├── spec.go               # AnimationSpec + Mapper
    │   ├── sheet.go              # frame slicing
    │   ├── animation.go          # runtime Animation (elapsed, Update, Frame, Done, Reset)
    │   └── library.go            # LoadLibrary (PNG -> frames)
    ├── input/
    │   └── input.go              # Poll() -> Intent
    ├── player/
    │   ├── player.go             # struct Player
    │   ├── physics.go            # struct Physics + LoadPhysics(repo)
    │   ├── tuning.go             # TuningParam + Mapper
    │   ├── fsm.go                # FSM, State interface
    │   └── states.go             # idleState, runState, …, attack2State
    ├── world/
    │   └── world.go              # World: Gravity, GroundY
    ├── debug/
    │   ├── fields.go             # Field catalog + FieldSource interface
    │   ├── config.go             # LoadConfig + JSON validation
    │   └── overlay.go            # Overlay struct, Toggle, Draw
    └── game/
        └── game.go               # ebiten.Game: Update/Draw/Layout
```

### Module dependency diagram

```mermaid
flowchart TD
    ENV[".env (godotenv)"] --> CFG["internal/config"]
    CFG --> STOR["internal/storage<br/>SQLite + migrations + seed"]
    CFG --> DBGCFG["config/debug.json"]
    CFG --> ASSETS["assets/sprites/char1/*.png"]

    STOR --> ANIMREPO["Repository[AnimationSpec]"]
    STOR --> TUNEREPO["Repository[TuningParam]"]

    ANIMREPO --> ANIMLIB["anim.LoadLibrary<br/>slice sheets -> frames"]
    ASSETS --> ANIMLIB
    TUNEREPO --> PHYS["player.Physics"]

    ANIMLIB --> PLAYER["player.Player + FSM<br/>(7 states)"]
    PHYS --> PLAYER
    WORLD["world.World (gravity, ground)"] --> PLAYER

    INPUT["input.Intent<br/>(keys -> intent)"] --> GAME["game.Game<br/>Update / Draw / Layout"]
    PLAYER --> GAME
    WORLD --> GAME

    DBGCFG --> DEBUG["debug.Overlay<br/>F3 toggle"]
    GAME --> DEBUG

    GAME --> EBITEN["ebiten.RunGame (cmd/game)"]

    TUNEREPO --> TUNECLI["cmd/tune (urfave/cli/v3)<br/>list / set / help"]
```

---

## 4. Config (`internal/config`)

### `.env` (committed)

```
# storage & assets
DB_PATH=./data/game.db
ASSETS_DIR=./assets/sprites/char1

# sprite frame size (per-frame in a horizontal strip)
SPRITE_FRAME_W=120
SPRITE_FRAME_H=80

# window & render
WINDOW_WIDTH=1280
WINDOW_HEIGHT=720
RENDER_SCALE=3

# debug overlay
DEBUG_CONFIG_PATH=./config/debug.json
```

### Contract

```go
type Config struct {
    DBPath          string
    AssetsDir       string
    SpriteFrameW    int
    SpriteFrameH    int
    WindowW         int
    WindowH         int
    RenderScale     int
    DebugConfigPath string
}

// Load reads .env via godotenv.Load (non-fatal if file missing — env may already be set)
// then reads each key via os.Getenv. Missing or unparseable keys panic with the key name.
func Load() *Config
```

No default values in code. `.env` ships the defaults.

---

## 5. Storage layer (`internal/storage`)

### Driver

`modernc.org/sqlite` (pure Go, no CGO). Chosen so Windows users do not need a C toolchain. Interface is `database/sql`, so swapping to `mattn/go-sqlite3` later is a one-line change with no ripple.

### Generic repository

```go
type Entity interface {
    GetID() string
}

type Scanner interface {
    Scan(dest ...any) error
}

type Mapper[T Entity] interface {
    Table() string
    Columns() []string              // column order for SELECT / INSERT / UPDATE
    Scan(row Scanner) (T, error)    // row -> T
    Values(t T) []any               // T -> values in Columns() order
}

type Repository[T Entity] struct {
    db     *sql.DB
    mapper Mapper[T]
}

func NewRepository[T Entity](db *sql.DB, m Mapper[T]) *Repository[T]

func (r *Repository[T]) Get(ctx context.Context, id string) (T, error)
func (r *Repository[T]) List(ctx context.Context) ([]T, error)
func (r *Repository[T]) Upsert(ctx context.Context, t T) error   // INSERT … ON CONFLICT(id) DO UPDATE
func (r *Repository[T]) Delete(ctx context.Context, id string) error
```

The `Upsert` SQL is built from `mapper.Table()`, `mapper.Columns()`, and the first column (ID) used as the conflict target.

### Migrations

SQL files embedded via `embed.FS` under `internal/storage/migrations/`. Applied in lexical order, tracked in a `schema_migrations` table (`version TEXT PRIMARY KEY, applied_at TIMESTAMP`). Each file runs inside a transaction.

```
001_init_animations.sql       -- CREATE TABLE animations
002_seed_char1_animations.sql -- INSERT the 7 rows
003_init_tuning.sql           -- CREATE TABLE tuning
004_seed_tuning.sql           -- INSERT the 7 rows
```

### Boot sequence

```go
func Open(cfg *config.Config) (*sql.DB, error)   // returns err on failure
func MustOpen(cfg *config.Config) *sql.DB        // convenience wrapper; panics on err (used by cmd/game and cmd/tune)
```

1. Ensure directory of `cfg.DBPath` exists (`os.MkdirAll`).
2. `sql.Open("sqlite", cfg.DBPath)` and `db.Ping()`.
3. Apply pending migrations.
4. Return `*sql.DB`.

Seeds live inside migrations (`INSERT OR IGNORE` style). After migrations apply once, subsequent boots are no-ops.

---

## 6. Animation system (`internal/anim`)

### SQL schema (`001_init_animations.sql`)

```sql
CREATE TABLE animations (
    id           TEXT    PRIMARY KEY,
    file         TEXT    NOT NULL,
    frame_count  INTEGER NOT NULL,
    duration_ms  INTEGER NOT NULL,
    loop         INTEGER NOT NULL
);
```

### Seed (`002_seed_char1_animations.sql`)

```sql
INSERT OR IGNORE INTO animations (id, file, frame_count, duration_ms, loop) VALUES
    ('idle',    '_Idle.png',    10, 1000, 1),
    ('run',     '_Run.png',     10, 1000, 1),
    ('jump',    '_Jump.png',     3,  500, 0),
    ('fall',    '_Fall.png',     3,  500, 0),
    ('dash',    '_Dash.png',     2,  500, 0),
    ('attack',  '_Attack.png',   4, 1500, 0),
    ('attack2', '_Attack2.png',  6, 1500, 0);
```

### Types

```go
type AnimationSpec struct {
    ID         string
    File       string
    FrameCount int
    DurationMs int
    Loop       bool
}
func (a AnimationSpec) GetID() string { return a.ID }

type Animation struct {  // runtime instance
    spec    *AnimationSpec
    frames  []*ebiten.Image
    elapsed time.Duration
}

func (a *Animation) Update(dt time.Duration)
func (a *Animation) CurrentFrame() *ebiten.Image
func (a *Animation) FrameIndex() int
func (a *Animation) Elapsed() time.Duration
func (a *Animation) SpecID() string
func (a *Animation) Done() bool    // true iff !spec.Loop && elapsed >= duration
func (a *Animation) Reset()
```

- Frame index = `int(elapsed / frameDuration)` where `frameDuration = DurationMs / FrameCount`.
- Non-loop: clamp index at `FrameCount - 1` when done.
- Loop: index `% FrameCount`.

### Slicing (`sheet.go`)

```go
func Slice(img *ebiten.Image, frameW, frameH, count int) []*ebiten.Image {
    frames := make([]*ebiten.Image, count)
    for i := 0; i < count; i++ {
        r := image.Rect(i*frameW, 0, (i+1)*frameW, frameH)
        frames[i] = img.SubImage(r).(*ebiten.Image)
    }
    return frames
}
```

`frameW` and `frameH` come from `cfg.SpriteFrameW/H` — no hard-coded 120/80 anywhere in `anim`.

### `LoadLibrary`

```go
func LoadLibrary(
    cfg *config.Config,
    repo *storage.Repository[AnimationSpec],
) (map[string]*Animation, error)
```

1. `specs, err := repo.List(ctx)`
2. For each spec: open `filepath.Join(cfg.AssetsDir, spec.File)`, decode PNG via `ebitenutil.NewImageFromFile` or `image.Decode` + `ebiten.NewImageFromImage`.
3. `Slice` into frames; build an `*Animation` keyed by `spec.ID`.
4. Return the map.

Errors (missing file, unreadable PNG, frame count mismatch) surface immediately — fail fast.

---

## 7. Player + state machine (`internal/player`)

### State machine diagram

```mermaid
stateDiagram-v2
    [*] --> Idle

    Idle --> Run: move
    Idle --> Jump: jump
    Idle --> Dash: dash
    Idle --> Attack: attack
    Idle --> Attack2: attack2
    Idle --> Fall: off-edge

    Run --> Idle: no move
    Run --> Jump: jump
    Run --> Dash: dash
    Run --> Attack: attack
    Run --> Attack2: attack2
    Run --> Fall: off-edge

    Jump --> Fall: vy >= 0
    Jump --> Dash: air-dash
    Jump --> Attack: air-attack
    Jump --> Attack2: air-attack2

    Fall --> Idle: grounded, no move
    Fall --> Run: grounded, move
    Fall --> Dash: air-dash
    Fall --> Attack: air-attack
    Fall --> Attack2: air-attack2

    Dash --> Idle: timer done, grounded
    Dash --> Fall: timer done, airborne

    Attack --> Idle: anim done, grounded, no move
    Attack --> Run: anim done, grounded, move
    Attack --> Fall: anim done, airborne
    Attack --> Jump: cancel (grounded only)
    Attack --> Dash: cancel

    Attack2 --> Idle: anim done, grounded, no move
    Attack2 --> Run: anim done, grounded, move
    Attack2 --> Fall: anim done, airborne
    Attack2 --> Jump: cancel (grounded only)
    Attack2 --> Dash: cancel

    note right of Jump
      Only from grounded. No double-jump.
      hasAirDash resets on land.
    end note

    note right of Dash
      Constant vx = Facing * DashSpeed.
      Gravity disabled during dash.
      One air-dash per airborne phase.
    end note

    note right of Attack
      Attack / Attack2 are SEPARATE keys
      (J/X vs K/C), not a combo chain.
      Cancelable by Dash (any) or Jump (ground).
    end note
```

### Per-state contract

| State | Animation | VX | Physics | Exit conditions |
|---|---|---|---|---|
| Idle | Idle (loop) | 0 | gravity on, stick to ground | move → Run / off-edge → Fall / jump/dash/attack/attack2 |
| Run | Run (loop) | ±RunSpeed (input sign) | gravity on | no move → Idle / off-edge → Fall / jump/dash/attack/attack2 |
| Jump | Jump (non-loop) | ±RunSpeed × AirControl | gravity on (after impulse) | vy ≥ 0 → Fall / dash (cancel) / attack (cancel) |
| Fall | Fall (clamp last frame) | ±RunSpeed × AirControl | gravity on | grounded → Idle/Run / dash / attack |
| Dash | Dash (non-loop) | Facing × DashSpeed (forced) | gravity OFF | timer ≥ DashDuration → Idle/Fall (based on grounded) |
| Attack | Attack (non-loop) | 0 on ground; preserved on air | gravity on | anim.Done → Idle/Run/Fall / jump cancel (ground) / dash cancel |
| Attack2 | Attack2 (non-loop) | same as Attack | gravity on | anim.Done → Idle/Run/Fall / jump cancel (ground) / dash cancel |

### Additional invariants

- **No double-jump.** Jump input only consumed when `grounded == true`.
- **Air-dash:** `hasAirDash bool` flag on `Player`. Set to `true` when grounded. Dash while airborne consumes it; dash is refused while airborne if `hasAirDash == false`.
- **Facing:** Updated whenever `Left` or `Right` intent is held (last pressed wins). Dash uses the facing at the moment Dash is entered; further input during Dash has no effect on direction.
- **Attack cancel to Jump on ground only** — in the air there is no meaningful "jump cancel" (character is already airborne); dash cancel remains available in air.
- **Off-edge detection:** `Y >= GroundY` means grounded. If `Grounded == true` and a horizontal step would not land on ground (future-proof: today ground is a full slab, so this never happens; keeping the transition for completeness).

### FSM implementation

```go
type StateID string

const (
    StateIdle    StateID = "idle"
    StateRun     StateID = "run"
    StateJump    StateID = "jump"
    StateFall    StateID = "fall"
    StateDash    StateID = "dash"
    StateAttack  StateID = "attack"
    StateAttack2 StateID = "attack2"
)

type State interface {
    ID() StateID
    Enter(p *Player)
    Update(p *Player, in input.Intent, dt time.Duration) StateID
    Exit(p *Player)
}

type FSM struct {
    states  map[StateID]State
    current State
}

func NewFSM(initial StateID) *FSM
func (f *FSM) CurrentID() StateID
func (f *FSM) Handle(p *Player, in input.Intent, dt time.Duration)
```

Each state is a small struct in `states.go`. Cancel rules are encoded in each state's `Update` (e.g. `attackState.Update` checks `in.DashPressed` and `in.JumpPressed` before defaulting to animation-driven exit).

### `Player`

```go
type Player struct {
    X, Y       float64   // feet position in world space
    VX, VY     float64
    Facing     int       // +1 or -1
    Grounded   bool
    HasAirDash bool
    DashTimer  time.Duration   // advanced by dashState.Update; reset on dash Enter
    FSM        *FSM
    Physics    *Physics
    Anims      map[string]*Animation
    Current    *Animation   // pointer into Anims
}

func (p *Player) PlayAnim(id string)                       // Reset + set Current
func (p *Player) ApplyPhysics(w *world.World, dt time.Duration)
```

---

## 8. Physics tuning (`internal/player`)

### SQL schema (`003_init_tuning.sql`)

```sql
CREATE TABLE tuning (
    key         TEXT    PRIMARY KEY,
    value       REAL    NOT NULL,
    min_value   REAL    NOT NULL,
    max_value   REAL    NOT NULL,
    unit        TEXT    NOT NULL DEFAULT '',
    description TEXT    NOT NULL
);
```

### Seed (`004_seed_tuning.sql`)

```sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('run_speed',        280,   50,  1000, 'px/s',  'Horizontal ground movement speed'),
    ('air_control',      0.8,    0,     1, '',      'Horizontal movement multiplier while airborne'),
    ('jump_velocity',   -650, -2000,  -100, 'px/s',  'Jump impulse applied on takeoff (negative = upward)'),
    ('gravity',         2000,  100,  5000, 'px/s^2','Downward acceleration applied each tick'),
    ('max_fall_speed',   900,  100,  3000, 'px/s',  'Terminal vertical velocity clamp'),
    ('dash_speed',       700,  100,  2000, 'px/s',  'Horizontal velocity during dash'),
    ('dash_duration_ms', 500,   50,  2000, 'ms',    'Dash duration');
```

### Types

```go
type TuningParam struct {
    Key         string
    Value       float64
    MinValue    float64
    MaxValue    float64
    Unit        string
    Description string
}
func (t TuningParam) GetID() string { return t.Key }

type Physics struct {
    RunSpeed      float64
    AirControl    float64
    JumpVelocity  float64
    Gravity       float64
    MaxFallSpeed  float64
    DashSpeed     float64
    DashDuration  time.Duration
}

func LoadPhysics(repo *storage.Repository[TuningParam]) (*Physics, error)
```

`LoadPhysics` fetches all rows, converts `dash_duration_ms` from REAL to `time.Duration`, and errors if any expected key is missing.

---

## 9. Input (`internal/input`)

### `Intent`

```go
type Intent struct {
    Left, Right     bool
    JumpPressed     bool  // edge (just pressed this tick)
    DashPressed     bool  // edge
    AttackPressed   bool  // edge
    Attack2Pressed  bool  // edge
}

func Poll() Intent
```

### Key map (fixed, not configurable in this iteration)

| Intent | Keys |
|---|---|
| Left | `A`, `ArrowLeft` |
| Right | `D`, `ArrowRight` |
| JumpPressed | `Space` (edge) |
| DashPressed | `ShiftLeft` or `ShiftRight` (edge) |
| AttackPressed | `J` or `X` (edge) |
| Attack2Pressed | `K` or `C` (edge) |

Edges use `inpututil.IsKeyJustPressed`; holds use `ebiten.IsKeyPressed`. The state machine only ever sees `Intent` — key remapping can later be added without touching FSM code.

---

## 10. World (`internal/world`)

```go
type World struct {
    Gravity float64   // copied from Physics.Gravity at boot
    GroundY float64   // GroundY = float64(cfg.WindowH) - 120 (for 1280x720 -> 600)
}
```

Flat slab from `y = GroundY` down to `cfg.WindowH`. No horizontal walls.

---

## 11. Debug overlay (`internal/debug`)

### Field catalog (`fields.go`)

```go
type FieldSource interface {
    Player() *player.Player
    Intent() *input.Intent
    EngineFPS() float64
    EngineTPS() float64
}

type Field struct {
    Key    string
    Format func(s FieldSource) string
}

var Catalog = map[string]Field{
    "state":           {"state",           func(s FieldSource) string { return "State: " + string(s.Player().FSM.CurrentID()) }},
    "facing":          {"facing",          func(s FieldSource) string { return fmt.Sprintf("Facing: %+d", s.Player().Facing) }},
    "grounded":        {"grounded",        func(s FieldSource) string { return fmt.Sprintf("Grounded: %t", s.Player().Grounded) }},
    "has_air_dash":    {"has_air_dash",    func(s FieldSource) string { return fmt.Sprintf("HasAirDash: %t", s.Player().HasAirDash) }},
    "x":               {"x",               func(s FieldSource) string { return fmt.Sprintf("X: %.2f", s.Player().X) }},
    "y":               {"y",               func(s FieldSource) string { return fmt.Sprintf("Y: %.2f", s.Player().Y) }},
    "vx":              {"vx",              func(s FieldSource) string { return fmt.Sprintf("VX: %.2f", s.Player().VX) }},
    "vy":              {"vy",              func(s FieldSource) string { return fmt.Sprintf("VY: %.2f", s.Player().VY) }},
    "anim_id":         {"anim_id",         func(s FieldSource) string { a := s.Player().Current; if a == nil { return "AnimID: -" }; return "AnimID: " + a.SpecID() }},
    "anim_frame":      {"anim_frame",      func(s FieldSource) string { a := s.Player().Current; if a == nil { return "Frame: -" }; return fmt.Sprintf("Frame: %d", a.FrameIndex()) }},
    "anim_elapsed_ms": {"anim_elapsed_ms", func(s FieldSource) string { a := s.Player().Current; if a == nil { return "Elapsed: -" }; return fmt.Sprintf("Elapsed: %d ms", a.Elapsed().Milliseconds()) }},
    "intent_left":     {"intent_left",     func(s FieldSource) string { return fmt.Sprintf("Left: %t", s.Intent().Left) }},
    "intent_right":    {"intent_right",    func(s FieldSource) string { return fmt.Sprintf("Right: %t", s.Intent().Right) }},
    "intent_jump":     {"intent_jump",     func(s FieldSource) string { return fmt.Sprintf("Jump: %t", s.Intent().JumpPressed) }},
    "intent_dash":     {"intent_dash",     func(s FieldSource) string { return fmt.Sprintf("Dash: %t", s.Intent().DashPressed) }},
    "intent_attack":   {"intent_attack",   func(s FieldSource) string { return fmt.Sprintf("Attack: %t", s.Intent().AttackPressed) }},
    "intent_attack2":  {"intent_attack2",  func(s FieldSource) string { return fmt.Sprintf("Attack2: %t", s.Intent().Attack2Pressed) }},
    "fps":             {"fps",             func(s FieldSource) string { return fmt.Sprintf("FPS: %.1f", s.EngineFPS()) }},
    "tps":             {"tps",             func(s FieldSource) string { return fmt.Sprintf("TPS: %.1f", s.EngineTPS()) }},
}
```

### `config/debug.json` (default, committed)

```json
{
  "sections": [
    { "title": "State",      "fields": ["state", "facing", "grounded", "has_air_dash"] },
    { "title": "Kinematics", "fields": ["x", "y", "vx", "vy"] },
    { "title": "Animation",  "fields": ["anim_id", "anim_frame", "anim_elapsed_ms"] },
    { "title": "Intent",     "fields": ["intent_left", "intent_right", "intent_jump", "intent_dash", "intent_attack", "intent_attack2"] },
    { "title": "Engine",     "fields": ["fps", "tps"] }
  ]
}
```

### Contract

```go
type Section struct {
    Title  string   `json:"title"`
    Fields []string `json:"fields"`
}
type Config struct {
    Sections []Section `json:"sections"`
}

func LoadConfig(path string) (*Config, error)
// Validates every field referenced exists in Catalog.
// On any unknown field: return error listing unknowns and the full catalog of valid keys.

type Overlay struct {
    cfg     *Config
    source  FieldSource
    enabled bool
}

func New(cfg *Config, source FieldSource) *Overlay
func (o *Overlay) Toggle()                      // bound to F3 inside game.Game.Update
func (o *Overlay) Enabled() bool
func (o *Overlay) Draw(screen *ebiten.Image)    // iterates sections, renders "-- Title --" then fields
```

### Behavior

- Overlay is **off by default**.
- Toggle is handled in `game.Game.Update` via `inpututil.IsKeyJustPressed(ebiten.KeyF3)`.
- JSON loaded once at boot; invalid content causes panic (fail fast — this is a dev-only path).
- No hot-reload in this iteration. Adding a watcher later is a pure addition.

---

## 12. Game loop (`internal/game` + `cmd/game`)

### `ebiten.Game` methods

```go
type Game struct {
    cfg       *config.Config
    world     *world.World
    player    *player.Player
    overlay   *debug.Overlay
    lastIntent input.Intent
}

func (g *Game) Update() error
func (g *Game) Draw(screen *ebiten.Image)
func (g *Game) Layout(outerW, outerH int) (int, int) { return g.cfg.WindowW, g.cfg.WindowH }
```

### `Update()`

```
if F3 just pressed: overlay.Toggle()

intent := input.Poll()
g.lastIntent = intent
dt := time.Second / 60

g.player.FSM.Handle(g.player, intent, dt)
g.player.ApplyPhysics(g.world, dt)
g.player.Current.Update(dt)
return nil
```

### `Draw()`

1. Background: `screen.Fill(color.RGBA{0x80, 0x80, 0x80, 0xFF})`.
2. Ground slab: filled rect from `(0, GroundY)` to `(WindowW, WindowH)` in `color.RGBA{0x3A, 0x3A, 0x3A, 0xFF}`.
3. Player:
   ```
   op := &ebiten.DrawImageOptions{}
   op.GeoM.Translate(-float64(frameW)/2, -float64(frameH))   // feet at origin
   if player.Facing < 0 { op.GeoM.Scale(-1, 1) }
   op.GeoM.Scale(float64(cfg.RenderScale), float64(cfg.RenderScale))
   op.GeoM.Translate(player.X, player.Y)
   op.Filter = ebiten.FilterNearest
   screen.DrawImage(player.Current.CurrentFrame(), op)
   ```
4. Overlay: `g.overlay.Draw(screen)` (no-op if disabled).

### `cmd/game/main.go`

```go
func main() {
    cfg := config.Load()
    db := storage.MustOpen(cfg)
    defer db.Close()

    animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.Mapper{})
    tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})

    anims, err := anim.LoadLibrary(cfg, animRepo); must(err)
    physics, err := player.LoadPhysics(tuneRepo);  must(err)

    dbgCfg, err := debug.LoadConfig(cfg.DebugConfigPath); must(err)

    g := game.New(cfg, anims, physics, dbgCfg)

    ebiten.SetWindowSize(cfg.WindowW, cfg.WindowH)
    ebiten.SetWindowTitle("claude-pixel")
    must(ebiten.RunGame(g))
}
```

---

## 13. CLI (`cmd/tune`)

### Commands (`urfave/cli/v3`)

```
claude-pixel-tune
├── list                        List every tunable parameter
├── set <key> <value>           Update a single parameter (existing keys only)
└── help [command]              Auto-generated; `help set` lists all keys + ranges
```

### Behavior

- **Shares `cmd/tune`'s bootstrap with `cmd/game`:** loads `.env` via `config.Load`, opens DB via `storage.Open`, constructs `Repository[TuningParam]`. No game subsystems loaded.
- **`tune list`** — reads all rows via `repo.List`, prints a formatted table (key, value, min, max, unit, description). English labels.
- **`tune set <key> <value>`** —
  1. `repo.Get(key)` → error `unknown tuning key "<key>". Run "tune list" to see valid keys.` if row missing. **No create.**
  2. Parse `<value>` as `float64` → error on parse failure.
  3. Call `Validate(param, newValue)` (see below). Error format: `value out of range: <x> not in [<min>, <max>] <unit>`.
  4. `repo.Upsert` with updated `value`.
  5. Print `OK: <key> = <new> <unit> (was <old>)`.
- **Validation** lives in `internal/player/tuning_validator.go`, separate from CLI handler, so it can be reused from other entry points later:
  ```go
  // internal/player (same package as TuningParam)
  func ValidateTuning(p TuningParam, newValue float64) error
  ```

### Makefile additions

```makefile
run:
	go run ./cmd/game

tune:
	go run ./cmd/tune $(ARGS)       # usage: make tune ARGS="list" or ARGS="set run_speed 320"
```

---

## 14. Dependencies (`go.mod`)

- `github.com/hajimehoshi/ebiten/v2`
- `github.com/joho/godotenv`
- `modernc.org/sqlite`
- `github.com/urfave/cli/v3`

No CGO required.

---

## 15. Testing strategy

Unit-testable without ebiten:

- **`anim.Animation`** — pure math on `elapsed`, `FrameIndex`, `Done`. Table-driven tests for loop vs non-loop, exact boundaries.
- **`storage.Repository[T]`** — in-memory SQLite (`:memory:`) with a toy `Entity` + `Mapper`.
- **`player.FSM`** — all transitions can be driven by synthetic `Intent` sequences; no rendering needed.
- **`debug.LoadConfig`** — validates unknown fields are rejected with a useful error message.
- **`tuning.Validate`** — table-driven.

Ebiten-coupled code (`game.Draw`, `anim.Slice`, `Overlay.Draw`) is validated manually — run the binary, press each key, confirm animation + state transitions visually against the debug overlay.

---

## 16. Extensibility notes

- **New animation (e.g. Crouch, Roll):** add a row to `005_seed_extra_animations.sql` + (if it introduces new behavior) a new `State` struct in `internal/player/states.go` registered in the FSM map. No change to the animation runtime or loader.
- **New entity in DB:** define a `T` + a `Mapper[T]`, construct `storage.NewRepository[T]` — full CRUD for free.
- **New tuning parameter:** add a row to the seed migration; add a field to `Physics`; update `LoadPhysics`. CLI `list` and `help` pick up the new row automatically (data-driven).
- **New debug field:** register in `Catalog`; reference from `debug.json`.
- **Key remapping:** replace `input.Poll` internals while keeping `Intent` shape — FSM untouched.

---

## 17. Open questions / deferred

- Hot-reload of `debug.json` and tuning rows (currently restart required).
- Serialization of save state (e.g. player position) — no persistence of game state today.
- Sound, UI, multiple characters, enemies — explicitly out of scope.
