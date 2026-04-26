# Enemy Behavior JSON Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hardcoded enemy decision logic in `internal/enemy/states.go` with a JSON-driven behavior-tree runtime. Core FSM skeleton stays in Go; per-kind `assets/behaviors/<kind>.json` files own decision trees, state declarations, and per-attack frame VX.

**Architecture:** New `internal/behavior/` package provides a small behavior-tree runtime (Selector / Sequence / Chance / Condition / Action / Wait) plus an action/condition registry and a JSON loader with strict validation. `internal/enemy/` retains the FSM skeleton but `states.go` is replaced by a generic driver that (1) enforces engine-owned event transitions (hit → hurt, dead → death, fall → run on grounded), (2) delegates decision-state logic to the loaded BT, (3) honors per-state `exit_on`, `next`, `on_exit_actions`, and `on_frame_vx` declarations. The SQLite `attack_motions` table is deleted (motions move into the per-state `on_frame_vx` field). Four `tuning` rows (`orc_run_speed`, `orc_intent_tick_s`, `slime_run_speed`, `slime_intent_tick_s`) are dropped — their values live inside the JSON now. Boot-only load + manual `F5` reload. No fsnotify.

**Tech Stack:** Go 1.22+, `encoding/json`, `math/rand`, `ebiten/v2` + `inpututil` (for F5), existing `internal/storage` / `internal/enemy` / `internal/combat` / `internal/game` / `internal/debug` packages.

---

## File Structure

**New files:**
- `internal/behavior/tree.go` — `Status`, `Node` interface, `Ctx`, `Tree`, helpers.
- `internal/behavior/nodes.go` — `Selector`, `Sequence`, `Chance`, `Wait` node types.
- `internal/behavior/action.go` — `Action` / `Condition` node types (call into registry).
- `internal/behavior/registry.go` — action/condition registration, v1 built-ins.
- `internal/behavior/loader.go` — JSON parse, node build, validation.
- `internal/behavior/tree_test.go`, `nodes_test.go`, `action_test.go`, `registry_test.go`, `loader_test.go`
- `internal/behavior/enemy_iface.go` — minimal interface the behavior package uses to mutate the enemy (avoids import cycle with `internal/enemy`).
- `internal/enemy/state_decl.go` — `StateDecl`, `FrameVX` types; per-state declarative data parsed from JSON.
- `assets/behaviors/orc.json`, `slime.json`, `README.md`
- `docs/superpowers/plans/2026-04-24-enemy-behavior-json.md` (this file)

**Modified files:**
- `internal/enemy/enemy.go` — `Enemy` struct: drop `IntentTimer`; add `CurrentState string`, `BranchTag string`, `timers map[string]float64`, `nextStateOverride string`.
- `internal/enemy/fsm.go` — replaced: `StateID` removed; generic driver reads `Kind.States` map and runs BT for decision states.
- `internal/enemy/kind.go` — `KindConfig` gains `BehaviorPath string`; `Kind` gains `States map[string]*StateDecl`, `InitialState string`; `BuildKind` calls `behavior.LoadFile`.
- `internal/enemy/loader.go` — remove `MotionsFor`; add `LoadBehavior`.
- `internal/enemy/tuning.go` — drop `RunSpeed`, `IntentTickS` fields; remove their key reads.
- `internal/enemy/fsm_test.go` — rewrite against synthetic `StateDecl` + synthetic BT.
- `internal/enemy/loader_test.go` — drop motion tests; add behavior-file load test.
- `internal/game/game.go` — F5 reload handler; pass nearest-enemy state/branch to debug overlay.
- `internal/debug/fields.go` — add `enemy_state`, `enemy_bt_last_branch`.
- `cmd/game/main.go` — wire `BehaviorPath` into each `KindConfig`; remove `motionRepo` and `motionSpecs`.
- `cmd/tune/main.go` — remove `motions` subcommand.
- `internal/storage/migrations/001_init_schema.sql` — drop `attack_motions` table.
- `internal/storage/migrations/002_seed_data.sql` — drop 4 tuning rows + `slime_attack2_motion` row.
- `CLAUDE.md` — update controls table (F5), tuning key count (30 → 26), behavior JSON reference.

**Deleted files:**
- `internal/enemy/states.go`
- `internal/combat/motion.go`
- `internal/combat/motion_test.go`

---

## Task 1: Scaffold `internal/behavior` package

**Files:**
- Create: `internal/behavior/tree.go`
- Create: `internal/behavior/enemy_iface.go`
- Create: `internal/behavior/tree_test.go`

- [ ] **Step 1: Write the failing test**

`internal/behavior/tree_test.go`:

```go
package behavior

import (
	"math/rand"
	"testing"
	"time"
)

type stubEnemy struct {
	facing       int
	vx           float64
	pendingGoto  string
	animDone     bool
	grounded     bool
	currentFrame int
}

func (e *stubEnemy) Facing() int                     { return e.facing }
func (e *stubEnemy) SetFacing(f int)                  { e.facing = f }
func (e *stubEnemy) SetVX(v float64)                   { e.vx = v }
func (e *stubEnemy) CurrentAnimDone() bool             { return e.animDone }
func (e *stubEnemy) Grounded() bool                    { return e.grounded }
func (e *stubEnemy) CurrentAnimFrame() int             { return e.currentFrame }
func (e *stubEnemy) PlayAnim(id string)                {}

func newCtx(e EnemyTarget) *Ctx {
	return &Ctx{Enemy: e, DT: 16 * time.Millisecond, RNG: rand.New(rand.NewSource(1))}
}

func TestStatusStringerCoversAllValues(t *testing.T) {
	cases := []Status{StatusSuccess, StatusFailure, StatusRunning}
	seen := map[string]bool{}
	for _, s := range cases {
		str := s.String()
		if str == "" {
			t.Fatalf("status %d has empty string", s)
		}
		if seen[str] {
			t.Fatalf("duplicate stringer output %q", str)
		}
		seen[str] = true
	}
}

func TestCtxSetBranchAppends(t *testing.T) {
	c := newCtx(&stubEnemy{})
	c.SetBranch("run")
	c.SetBranch("chance#0")
	c.SetBranch("attack")
	if got, want := c.BranchTag, "run/chance#0/attack"; got != want {
		t.Fatalf("BranchTag = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run TestStatus -v`
Expected: FAIL with `undefined: Status` (package doesn't exist yet).

- [ ] **Step 3: Write minimal implementation**

`internal/behavior/enemy_iface.go`:

```go
package behavior

// EnemyTarget is the minimal surface the behavior runtime needs on an enemy.
// It lives here (not in internal/enemy) to keep the behavior package
// independent of enemy internals and to avoid import cycles.
type EnemyTarget interface {
	Facing() int
	SetFacing(int)
	SetVX(float64)
	CurrentAnimDone() bool
	Grounded() bool
	CurrentAnimFrame() int
	PlayAnim(id string)
}
```

`internal/behavior/tree.go`:

```go
package behavior

import (
	"math/rand"
	"time"
)

type Status int

const (
	StatusSuccess Status = iota
	StatusFailure
	StatusRunning
)

func (s Status) String() string {
	switch s {
	case StatusSuccess:
		return "success"
	case StatusFailure:
		return "failure"
	case StatusRunning:
		return "running"
	}
	return "?"
}

// Node is the behavior-tree node contract. Tick mutates Ctx and returns a
// Status. Pure-data nodes (e.g. Chance after it has picked a branch) hold
// their own small state across ticks — the Tree is re-used per enemy so
// each enemy has its own Tree instance.
type Node interface {
	Tick(ctx *Ctx) Status
}

// Tree is a per-enemy wrapper around a root Node. It has no fields for now
// but keeps the door open for root-level metadata (e.g. a tick counter used
// by debug overlays).
type Tree struct {
	Root Node
}

// Ctx threads per-tick state through the tree.
type Ctx struct {
	Enemy       EnemyTarget
	DT          time.Duration
	RNG         *rand.Rand
	PendingGoto string
	BranchTag   string
}

// SetBranch appends a segment to the breadcrumb path shown in debug.
func (c *Ctx) SetBranch(seg string) {
	if c.BranchTag == "" {
		c.BranchTag = seg
		return
	}
	c.BranchTag += "/" + seg
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS (`TestStatusStringerCoversAllValues`, `TestCtxSetBranchAppends`).

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/
git commit -m "feat(behavior): scaffold Node/Tree/Ctx + EnemyTarget iface"
```

---

## Task 2: Selector + Sequence nodes

**Files:**
- Create: `internal/behavior/nodes.go`
- Create: `internal/behavior/nodes_test.go`

- [ ] **Step 1: Write the failing test**

`internal/behavior/nodes_test.go`:

```go
package behavior

import "testing"

type fakeNode struct {
	out    Status
	called int
}

func (f *fakeNode) Tick(*Ctx) Status {
	f.called++
	return f.out
}

func TestSelectorReturnsFirstSuccess(t *testing.T) {
	a := &fakeNode{out: StatusFailure}
	b := &fakeNode{out: StatusSuccess}
	c := &fakeNode{out: StatusSuccess}
	sel := &Selector{Children: []Node{a, b, c}}
	if got := sel.Tick(newCtx(&stubEnemy{})); got != StatusSuccess {
		t.Fatalf("Tick = %v, want success", got)
	}
	if a.called != 1 || b.called != 1 || c.called != 0 {
		t.Fatalf("call counts: a=%d b=%d c=%d", a.called, b.called, c.called)
	}
}

func TestSelectorPropagatesRunning(t *testing.T) {
	sel := &Selector{Children: []Node{
		&fakeNode{out: StatusFailure},
		&fakeNode{out: StatusRunning},
		&fakeNode{out: StatusSuccess},
	}}
	if got := sel.Tick(newCtx(&stubEnemy{})); got != StatusRunning {
		t.Fatalf("Tick = %v, want running", got)
	}
}

func TestSelectorAllFailureReturnsFailure(t *testing.T) {
	sel := &Selector{Children: []Node{
		&fakeNode{out: StatusFailure},
		&fakeNode{out: StatusFailure},
	}}
	if got := sel.Tick(newCtx(&stubEnemy{})); got != StatusFailure {
		t.Fatalf("Tick = %v, want failure", got)
	}
}

func TestSequenceReturnsFirstFailure(t *testing.T) {
	a := &fakeNode{out: StatusSuccess}
	b := &fakeNode{out: StatusFailure}
	c := &fakeNode{out: StatusSuccess}
	seq := &Sequence{Children: []Node{a, b, c}}
	if got := seq.Tick(newCtx(&stubEnemy{})); got != StatusFailure {
		t.Fatalf("Tick = %v, want failure", got)
	}
	if a.called != 1 || b.called != 1 || c.called != 0 {
		t.Fatalf("call counts: a=%d b=%d c=%d", a.called, b.called, c.called)
	}
}

func TestSequenceAllSuccessReturnsSuccess(t *testing.T) {
	seq := &Sequence{Children: []Node{
		&fakeNode{out: StatusSuccess},
		&fakeNode{out: StatusSuccess},
	}}
	if got := seq.Tick(newCtx(&stubEnemy{})); got != StatusSuccess {
		t.Fatalf("Tick = %v, want success", got)
	}
}

func TestSequencePropagatesRunning(t *testing.T) {
	seq := &Sequence{Children: []Node{
		&fakeNode{out: StatusSuccess},
		&fakeNode{out: StatusRunning},
		&fakeNode{out: StatusSuccess},
	}}
	if got := seq.Tick(newCtx(&stubEnemy{})); got != StatusRunning {
		t.Fatalf("Tick = %v, want running", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run TestSelector -v`
Expected: FAIL with `undefined: Selector`.

- [ ] **Step 3: Write minimal implementation**

`internal/behavior/nodes.go`:

```go
package behavior

// Selector ticks children left→right. First non-Failure result wins.
type Selector struct {
	Children []Node
}

func (s *Selector) Tick(ctx *Ctx) Status {
	for _, ch := range s.Children {
		st := ch.Tick(ctx)
		if st != StatusFailure {
			return st
		}
	}
	return StatusFailure
}

// Sequence ticks children left→right. First non-Success result wins.
type Sequence struct {
	Children []Node
}

func (s *Sequence) Tick(ctx *Ctx) Status {
	for _, ch := range s.Children {
		st := ch.Tick(ctx)
		if st != StatusSuccess {
			return st
		}
	}
	return StatusSuccess
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS (Selector + Sequence tests).

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/nodes.go internal/behavior/nodes_test.go
git commit -m "feat(behavior): Selector + Sequence nodes"
```

---

## Task 3: Chance + Wait nodes

**Files:**
- Modify: `internal/behavior/nodes.go`
- Modify: `internal/behavior/nodes_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/behavior/nodes_test.go`:

```go
import (
	"math/rand"
	"time"
)

func TestChancePicksBranchUsingWeights(t *testing.T) {
	// Seed produces deterministic sequence. With rand.New(rand.NewSource(1)),
	// first Intn(100) = 81. Weights [30,70] → cumulative [30,100]; 81 falls
	// in the second branch.
	winner := &fakeNode{out: StatusSuccess}
	loser := &fakeNode{out: StatusSuccess}
	ch := &Chance{Branches: []ChanceBranch{
		{Weight: 30, Node: loser},
		{Weight: 70, Node: winner},
	}}
	ctx := &Ctx{RNG: rand.New(rand.NewSource(1))}
	if got := ch.Tick(ctx); got != StatusSuccess {
		t.Fatalf("Tick = %v", got)
	}
	if loser.called != 0 || winner.called != 1 {
		t.Fatalf("branch calls: loser=%d winner=%d", loser.called, winner.called)
	}
}

func TestChanceStickyWhileRunning(t *testing.T) {
	running := &fakeNode{out: StatusRunning}
	other := &fakeNode{out: StatusSuccess}
	ch := &Chance{Branches: []ChanceBranch{
		{Weight: 100, Node: running},
		{Weight: 1, Node: other},
	}}
	ctx := &Ctx{RNG: rand.New(rand.NewSource(1))}
	ch.Tick(ctx)
	ch.Tick(ctx)
	if running.called != 2 {
		t.Fatalf("running.called = %d, want 2", running.called)
	}
	// After Success, next Tick should re-roll.
	running.out = StatusSuccess
	ch.Tick(ctx) // resolves running branch as success
	ch.Tick(ctx) // new roll
	if running.called+other.called < 3 {
		t.Fatalf("expected a reroll after Success")
	}
}

func TestWaitReturnsRunningUntilElapsed(t *testing.T) {
	w := &Wait{Seconds: 1.0}
	ctx := &Ctx{DT: 400 * time.Millisecond}
	if got := w.Tick(ctx); got != StatusRunning {
		t.Fatalf("first tick = %v, want running", got)
	}
	if got := w.Tick(ctx); got != StatusRunning {
		t.Fatalf("second tick = %v, want running", got)
	}
	if got := w.Tick(ctx); got != StatusSuccess {
		t.Fatalf("third tick = %v, want success", got)
	}
	// After success, re-ticking restarts the timer.
	if got := w.Tick(ctx); got != StatusRunning {
		t.Fatalf("re-tick = %v, want running (restart)", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run TestChance -v`
Expected: FAIL with `undefined: Chance`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/behavior/nodes.go`:

```go
import "math/rand"

// ChanceBranch is a weighted arm of a Chance node.
type ChanceBranch struct {
	Weight int
	Node   Node
}

// Chance rolls once when idle, picks a branch by weight, then forwards
// ticks to it until it returns Success or Failure. Re-rolls on the next
// Tick after a terminal result.
type Chance struct {
	Branches []ChanceBranch

	active    int
	hasActive bool
}

func (c *Chance) Tick(ctx *Ctx) Status {
	if !c.hasActive {
		c.active = pickWeighted(ctx.RNG, c.Branches)
		c.hasActive = true
	}
	st := c.Branches[c.active].Node.Tick(ctx)
	if st != StatusRunning {
		c.hasActive = false
	}
	return st
}

func pickWeighted(rng *rand.Rand, branches []ChanceBranch) int {
	total := 0
	for _, b := range branches {
		if b.Weight > 0 {
			total += b.Weight
		}
	}
	r := rng.Intn(total)
	for i, b := range branches {
		if b.Weight <= 0 {
			continue
		}
		if r < b.Weight {
			return i
		}
		r -= b.Weight
	}
	return len(branches) - 1
}

// Wait returns Running until the accumulated DT exceeds Seconds, then Success.
// Restarts automatically after reporting Success.
type Wait struct {
	Seconds float64

	elapsed float64
}

func (w *Wait) Tick(ctx *Ctx) Status {
	w.elapsed += ctx.DT.Seconds()
	if w.elapsed < w.Seconds {
		return StatusRunning
	}
	w.elapsed = 0
	return StatusSuccess
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS (Chance + Wait tests).

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/nodes.go internal/behavior/nodes_test.go
git commit -m "feat(behavior): Chance + Wait nodes"
```

---

## Task 4: Registry + built-in actions / conditions

**Files:**
- Create: `internal/behavior/registry.go`
- Create: `internal/behavior/registry_test.go`

- [ ] **Step 1: Write the failing test**

`internal/behavior/registry_test.go`:

```go
package behavior

import (
	"math/rand"
	"testing"
)

func TestBuiltinGotoSetsPendingGoto(t *testing.T) {
	ctx := newCtx(&stubEnemy{})
	st, err := RunAction("goto", map[string]any{"state": "run"}, ctx)
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if st != StatusSuccess {
		t.Fatalf("status = %v", st)
	}
	if ctx.PendingGoto != "run" {
		t.Fatalf("PendingGoto = %q", ctx.PendingGoto)
	}
}

func TestBuiltinFlipFacing(t *testing.T) {
	e := &stubEnemy{facing: 1}
	ctx := newCtx(e)
	if _, err := RunAction("flip_facing", nil, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if e.facing != -1 {
		t.Fatalf("facing = %d, want -1", e.facing)
	}
}

func TestBuiltinRandomizeFacing(t *testing.T) {
	e := &stubEnemy{facing: 0}
	ctx := &Ctx{Enemy: e, RNG: rand.New(rand.NewSource(1))}
	if _, err := RunAction("randomize_facing", nil, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if e.facing != 1 && e.facing != -1 {
		t.Fatalf("facing = %d, want ±1", e.facing)
	}
}

func TestBuiltinSetVXForwardUsesFacing(t *testing.T) {
	e := &stubEnemy{facing: -1}
	ctx := newCtx(e)
	if _, err := RunAction("set_vx_forward", map[string]any{"speed": 80.0}, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if e.vx != -80 {
		t.Fatalf("vx = %f, want -80", e.vx)
	}
}

func TestBuiltinStopZeroesVX(t *testing.T) {
	e := &stubEnemy{vx: 120}
	ctx := newCtx(e)
	if _, err := RunAction("stop", nil, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if e.vx != 0 {
		t.Fatalf("vx = %f, want 0", e.vx)
	}
}

func TestBuiltinPlayAnimCallsEnemy(t *testing.T) {
	played := ""
	e := &playAnimStub{stubEnemy: stubEnemy{}, played: &played}
	ctx := newCtx(e)
	if _, err := RunAction("play_anim", map[string]any{"key": "idle"}, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if played != "idle" {
		t.Fatalf("played = %q", played)
	}
}

type playAnimStub struct {
	stubEnemy
	played *string
}

func (p *playAnimStub) PlayAnim(id string) { *p.played = id }

func TestBuiltinConditionGrounded(t *testing.T) {
	e := &stubEnemy{grounded: true}
	ctx := newCtx(e)
	ok, err := RunCondition("grounded", nil, ctx)
	if err != nil {
		t.Fatalf("RunCondition: %v", err)
	}
	if !ok {
		t.Fatalf("grounded=true returned false")
	}
	e.grounded = false
	ok, _ = RunCondition("grounded", nil, ctx)
	if ok {
		t.Fatalf("grounded=false returned true")
	}
}

func TestBuiltinConditionAnimDone(t *testing.T) {
	e := &stubEnemy{animDone: true}
	ctx := newCtx(e)
	ok, _ := RunCondition("anim_done", nil, ctx)
	if !ok {
		t.Fatalf("animDone=true returned false")
	}
}

func TestBuiltinConditionAnimFrameGE(t *testing.T) {
	e := &stubEnemy{currentFrame: 5}
	ctx := newCtx(e)
	ok, err := RunCondition("anim_frame_ge", map[string]any{"frame": 4.0}, ctx)
	if err != nil {
		t.Fatalf("RunCondition: %v", err)
	}
	if !ok {
		t.Fatalf("frame 5 >= 4 returned false")
	}
	ok, _ = RunCondition("anim_frame_ge", map[string]any{"frame": 6.0}, ctx)
	if ok {
		t.Fatalf("frame 5 >= 6 returned true")
	}
}

func TestBuiltinConditionAnimFrameLE(t *testing.T) {
	e := &stubEnemy{currentFrame: 5}
	ctx := newCtx(e)
	ok, _ := RunCondition("anim_frame_le", map[string]any{"frame": 5.0}, ctx)
	if !ok {
		t.Fatalf("frame 5 <= 5 returned false")
	}
}

func TestRunActionUnknownReturnsError(t *testing.T) {
	_, err := RunAction("nope_nada", nil, newCtx(&stubEnemy{}))
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestRunConditionUnknownReturnsError(t *testing.T) {
	_, err := RunCondition("nope_nada", nil, newCtx(&stubEnemy{}))
	if err == nil {
		t.Fatal("expected error for unknown condition")
	}
}

func TestHasActionHasConditionLookup(t *testing.T) {
	if !HasAction("goto") {
		t.Fatal("HasAction(goto) false")
	}
	if HasAction("nope_nada") {
		t.Fatal("HasAction(nope_nada) true")
	}
	if !HasCondition("grounded") {
		t.Fatal("HasCondition(grounded) false")
	}
	if HasCondition("nope_nada") {
		t.Fatal("HasCondition(nope_nada) true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run TestBuiltin -v`
Expected: FAIL with `undefined: RunAction`.

- [ ] **Step 3: Write minimal implementation**

`internal/behavior/registry.go`:

```go
package behavior

import "fmt"

// ActionFn is a behavior-tree action implementation. Returns the Status the
// surrounding Action node should emit. Errors are construction-time concerns
// (unknown name / bad args) — return an error here to surface at load time.
type ActionFn func(args map[string]any, ctx *Ctx) (Status, error)

// ConditionFn returns a boolean that the surrounding Condition node converts
// into Success/Failure.
type ConditionFn func(args map[string]any, ctx *Ctx) (bool, error)

var (
	actions    = map[string]ActionFn{}
	conditions = map[string]ConditionFn{}
)

// RegisterAction adds a new action by name. Last registration wins. Call
// from init() in client packages.
func RegisterAction(name string, fn ActionFn) { actions[name] = fn }

// RegisterCondition adds a new condition by name. Last registration wins.
func RegisterCondition(name string, fn ConditionFn) { conditions[name] = fn }

// HasAction reports whether name is registered. Used by the loader to
// reject unknown actions at parse time.
func HasAction(name string) bool {
	_, ok := actions[name]
	return ok
}

// HasCondition reports whether name is registered.
func HasCondition(name string) bool {
	_, ok := conditions[name]
	return ok
}

// RunAction executes a named action. Returns an error if the name isn't
// registered or the action itself fails.
func RunAction(name string, args map[string]any, ctx *Ctx) (Status, error) {
	fn, ok := actions[name]
	if !ok {
		return StatusFailure, fmt.Errorf("behavior: unknown action %q", name)
	}
	return fn(args, ctx)
}

// RunCondition executes a named condition.
func RunCondition(name string, args map[string]any, ctx *Ctx) (bool, error) {
	fn, ok := conditions[name]
	if !ok {
		return false, fmt.Errorf("behavior: unknown condition %q", name)
	}
	return fn(args, ctx)
}

func init() {
	RegisterAction("goto", func(args map[string]any, ctx *Ctx) (Status, error) {
		s, err := argString(args, "state")
		if err != nil {
			return StatusFailure, err
		}
		ctx.PendingGoto = s
		return StatusSuccess, nil
	})
	RegisterAction("flip_facing", func(_ map[string]any, ctx *Ctx) (Status, error) {
		ctx.Enemy.SetFacing(-ctx.Enemy.Facing())
		return StatusSuccess, nil
	})
	RegisterAction("randomize_facing", func(_ map[string]any, ctx *Ctx) (Status, error) {
		if ctx.RNG.Intn(2) == 0 {
			ctx.Enemy.SetFacing(1)
		} else {
			ctx.Enemy.SetFacing(-1)
		}
		return StatusSuccess, nil
	})
	RegisterAction("set_vx_forward", func(args map[string]any, ctx *Ctx) (Status, error) {
		speed, err := argFloat(args, "speed")
		if err != nil {
			return StatusFailure, err
		}
		ctx.Enemy.SetVX(float64(ctx.Enemy.Facing()) * speed)
		return StatusSuccess, nil
	})
	RegisterAction("stop", func(_ map[string]any, ctx *Ctx) (Status, error) {
		ctx.Enemy.SetVX(0)
		return StatusSuccess, nil
	})
	RegisterAction("play_anim", func(args map[string]any, ctx *Ctx) (Status, error) {
		key, err := argString(args, "key")
		if err != nil {
			return StatusFailure, err
		}
		ctx.Enemy.PlayAnim(key)
		return StatusSuccess, nil
	})

	RegisterCondition("grounded", func(_ map[string]any, ctx *Ctx) (bool, error) {
		return ctx.Enemy.Grounded(), nil
	})
	RegisterCondition("anim_done", func(_ map[string]any, ctx *Ctx) (bool, error) {
		return ctx.Enemy.CurrentAnimDone(), nil
	})
	RegisterCondition("anim_frame_ge", func(args map[string]any, ctx *Ctx) (bool, error) {
		f, err := argFloat(args, "frame")
		if err != nil {
			return false, err
		}
		return ctx.Enemy.CurrentAnimFrame() >= int(f), nil
	})
	RegisterCondition("anim_frame_le", func(args map[string]any, ctx *Ctx) (bool, error) {
		f, err := argFloat(args, "frame")
		if err != nil {
			return false, err
		}
		return ctx.Enemy.CurrentAnimFrame() <= int(f), nil
	})
}

func argString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing arg %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("arg %q must be string, got %T", key, v)
	}
	return s, nil
}

func argFloat(args map[string]any, key string) (float64, error) {
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("missing arg %q", key)
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	}
	return 0, fmt.Errorf("arg %q must be number, got %T", key, v)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS (all registry tests).

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/registry.go internal/behavior/registry_test.go
git commit -m "feat(behavior): registry + v1 builtin actions/conditions"
```

---

## Task 5: Action + Condition nodes

**Files:**
- Create: `internal/behavior/action.go`
- Create: `internal/behavior/action_test.go`

- [ ] **Step 1: Write the failing test**

`internal/behavior/action_test.go`:

```go
package behavior

import "testing"

func TestActionNodeCallsRegistry(t *testing.T) {
	e := &stubEnemy{facing: 1}
	ctx := newCtx(e)
	n := &Action{Name: "flip_facing"}
	if got := n.Tick(ctx); got != StatusSuccess {
		t.Fatalf("Tick = %v", got)
	}
	if e.facing != -1 {
		t.Fatalf("facing = %d", e.facing)
	}
}

func TestActionNodePropagatesArgs(t *testing.T) {
	ctx := newCtx(&stubEnemy{})
	n := &Action{Name: "goto", Args: map[string]any{"state": "attack"}}
	n.Tick(ctx)
	if ctx.PendingGoto != "attack" {
		t.Fatalf("PendingGoto = %q", ctx.PendingGoto)
	}
}

func TestConditionNodeTrueIsSuccess(t *testing.T) {
	e := &stubEnemy{grounded: true}
	n := &Condition{Name: "grounded"}
	if got := n.Tick(newCtx(e)); got != StatusSuccess {
		t.Fatalf("Tick = %v, want success", got)
	}
}

func TestConditionNodeFalseIsFailure(t *testing.T) {
	e := &stubEnemy{grounded: false}
	n := &Condition{Name: "grounded"}
	if got := n.Tick(newCtx(e)); got != StatusFailure {
		t.Fatalf("Tick = %v, want failure", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run TestAction -v`
Expected: FAIL with `undefined: Action`.

- [ ] **Step 3: Write minimal implementation**

`internal/behavior/action.go`:

```go
package behavior

// Action is a leaf node that invokes a registered action by name. The
// loader validates the name exists at parse time, so tick-time errors
// here indicate a registry mutation after load — rare, but surfaced as
// Failure to keep tick paths lenient.
type Action struct {
	Name string
	Args map[string]any
}

func (a *Action) Tick(ctx *Ctx) Status {
	st, err := RunAction(a.Name, a.Args, ctx)
	if err != nil {
		return StatusFailure
	}
	return st
}

// Condition wraps a registered condition. True → Success, false → Failure.
type Condition struct {
	Name string
	Args map[string]any
}

func (c *Condition) Tick(ctx *Ctx) Status {
	ok, err := RunCondition(c.Name, c.Args, ctx)
	if err != nil || !ok {
		return StatusFailure
	}
	return StatusSuccess
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/action.go internal/behavior/action_test.go
git commit -m "feat(behavior): Action + Condition nodes"
```

---

## Task 6: JSON loader — parse nodes

**Files:**
- Create: `internal/behavior/loader.go`
- Create: `internal/behavior/loader_test.go`

This task covers the node-tree portion of the loader. State-declaration parsing is layered on in Task 7.

- [ ] **Step 1: Write the failing test**

`internal/behavior/loader_test.go`:

```go
package behavior

import (
	"strings"
	"testing"
)

func TestBuildNodeSelector(t *testing.T) {
	raw := map[string]any{
		"type": "selector",
		"children": []any{
			map[string]any{"type": "action", "name": "flip_facing"},
			map[string]any{"type": "action", "name": "stop"},
		},
	}
	n, err := buildNode(raw, "root")
	if err != nil {
		t.Fatalf("buildNode: %v", err)
	}
	s, ok := n.(*Selector)
	if !ok {
		t.Fatalf("type = %T, want *Selector", n)
	}
	if len(s.Children) != 2 {
		t.Fatalf("children = %d", len(s.Children))
	}
}

func TestBuildNodeChanceTree(t *testing.T) {
	raw := map[string]any{
		"type": "chance",
		"branches": []any{
			map[string]any{"weight": 30.0, "node": map[string]any{"type": "action", "name": "flip_facing"}},
			map[string]any{"weight": 70.0, "node": map[string]any{"type": "action", "name": "stop"}},
		},
	}
	n, err := buildNode(raw, "root")
	if err != nil {
		t.Fatalf("buildNode: %v", err)
	}
	ch, ok := n.(*Chance)
	if !ok {
		t.Fatalf("type = %T", n)
	}
	if len(ch.Branches) != 2 || ch.Branches[0].Weight != 30 {
		t.Fatalf("branches = %+v", ch.Branches)
	}
}

func TestBuildNodeWait(t *testing.T) {
	raw := map[string]any{"type": "wait", "seconds": 2.5}
	n, err := buildNode(raw, "root")
	if err != nil {
		t.Fatalf("buildNode: %v", err)
	}
	w := n.(*Wait)
	if w.Seconds != 2.5 {
		t.Fatalf("seconds = %f", w.Seconds)
	}
}

func TestBuildNodeActionUnknownRejected(t *testing.T) {
	raw := map[string]any{"type": "action", "name": "nope_nada"}
	_, err := buildNode(raw, "root")
	if err == nil || !strings.Contains(err.Error(), "nope_nada") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildNodeConditionUnknownRejected(t *testing.T) {
	raw := map[string]any{"type": "condition", "name": "nope_nada"}
	_, err := buildNode(raw, "root")
	if err == nil || !strings.Contains(err.Error(), "nope_nada") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildNodeUnknownTypeRejected(t *testing.T) {
	raw := map[string]any{"type": "spline"}
	_, err := buildNode(raw, "root")
	if err == nil || !strings.Contains(err.Error(), "spline") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildNodeChanceEmptyBranchesRejected(t *testing.T) {
	raw := map[string]any{"type": "chance", "branches": []any{}}
	_, err := buildNode(raw, "root")
	if err == nil || !strings.Contains(err.Error(), "chance") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildNodeChanceNonPositiveWeightRejected(t *testing.T) {
	raw := map[string]any{
		"type": "chance",
		"branches": []any{
			map[string]any{"weight": 0.0, "node": map[string]any{"type": "action", "name": "stop"}},
		},
	}
	_, err := buildNode(raw, "root")
	if err == nil || !strings.Contains(err.Error(), "weight") {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run TestBuildNode -v`
Expected: FAIL with `undefined: buildNode`.

- [ ] **Step 3: Write minimal implementation**

`internal/behavior/loader.go`:

```go
package behavior

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFile parses a behavior JSON file. It returns the root-level data
// needed by consumers: the kind name, the ordered state declarations, and
// a per-decision-state tree. State-decl parsing lives in this package as
// opaque data; internal/enemy layers its own typed view on top.
//
// See assets/behaviors/README.md for the schema.
func LoadFile(path string) (*File, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("behavior: read %s: %w", path, err)
	}
	var raw FileRaw
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return nil, fmt.Errorf("behavior: parse %s: %w", path, err)
	}
	return buildFile(&raw, path)
}

// FileRaw is the JSON-facing shape. Exported so callers can
// marshal/unmarshal without duplicating the tags.
type FileRaw struct {
	Kind   string         `json:"kind"`
	States []StateRaw     `json:"states"`
}

type StateRaw struct {
	ID             string         `json:"id"`
	Anim           string         `json:"anim"`
	Decision       bool           `json:"decision"`
	BT             map[string]any `json:"bt,omitempty"`
	ExitOn         string         `json:"exit_on,omitempty"`
	Next           string         `json:"next,omitempty"`
	OnExitActions  []string       `json:"on_exit_actions,omitempty"`
	OnFrameVX      []FrameVXRaw   `json:"on_frame_vx,omitempty"`
}

type FrameVXRaw struct {
	FrameStart int     `json:"frame_start"`
	FrameEnd   int     `json:"frame_end"`
	VX         float64 `json:"vx"`
}

// File is the parsed, validated behavior spec. Node trees are already
// constructed for decision states.
type File struct {
	Kind   string
	States []State
}

// State is an opaque per-state record the enemy package converts into its
// own typed StateDecl.
type State struct {
	ID             string
	Anim           string
	Decision       bool
	BT             *Tree
	ExitOn         string
	Next           string
	OnExitActions  []string
	OnFrameVX      []FrameVX
}

type FrameVX struct {
	FrameStart int
	FrameEnd   int
	VX         float64
}

func buildFile(raw *FileRaw, path string) (*File, error) {
	if raw.Kind == "" {
		return nil, fmt.Errorf("behavior: %s: missing \"kind\"", path)
	}
	if len(raw.States) == 0 {
		return nil, fmt.Errorf("behavior: %s: states is empty", path)
	}
	out := &File{Kind: raw.Kind}
	seen := map[string]bool{}
	for i := range raw.States {
		s := raw.States[i]
		if s.ID == "" {
			return nil, fmt.Errorf("behavior: %s: state #%d missing id", path, i)
		}
		if seen[s.ID] {
			return nil, fmt.Errorf("behavior: %s: duplicate state id %q", path, s.ID)
		}
		seen[s.ID] = true
		if s.Anim == "" {
			return nil, fmt.Errorf("behavior: %s: state %q missing anim", path, s.ID)
		}
		if s.Decision && s.BT == nil {
			return nil, fmt.Errorf("behavior: %s: decision state %q missing bt", path, s.ID)
		}
		if !s.Decision && s.BT != nil {
			return nil, fmt.Errorf("behavior: %s: non-decision state %q must not have bt", path, s.ID)
		}
		ps := State{
			ID:            s.ID,
			Anim:          s.Anim,
			Decision:      s.Decision,
			ExitOn:        s.ExitOn,
			Next:          s.Next,
			OnExitActions: append([]string(nil), s.OnExitActions...),
		}
		for _, fv := range s.OnFrameVX {
			ps.OnFrameVX = append(ps.OnFrameVX, FrameVX{FrameStart: fv.FrameStart, FrameEnd: fv.FrameEnd, VX: fv.VX})
		}
		if s.Decision {
			root, err := buildNode(s.BT, s.ID)
			if err != nil {
				return nil, fmt.Errorf("behavior: %s: state %q: %w", path, s.ID, err)
			}
			ps.BT = &Tree{Root: root}
		}
		out.States = append(out.States, ps)
	}
	// Cross-state validation: goto targets + next targets must exist.
	if err := validateTransitions(out, path); err != nil {
		return nil, err
	}
	return out, nil
}

func validateTransitions(f *File, path string) error {
	ids := map[string]bool{}
	for _, s := range f.States {
		ids[s.ID] = true
	}
	for _, s := range f.States {
		if !s.Decision {
			if s.Next != "" && s.Next != "__dead" && !ids[s.Next] {
				return fmt.Errorf("behavior: %s: state %q next=%q undeclared", path, s.ID, s.Next)
			}
		}
		for _, a := range s.OnExitActions {
			if !HasAction(a) {
				return fmt.Errorf("behavior: %s: state %q on_exit_actions: unknown action %q", path, s.ID, a)
			}
		}
		if s.BT != nil {
			if err := validateGotos(s.BT.Root, ids, path, s.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateGotos(n Node, ids map[string]bool, path, state string) error {
	switch v := n.(type) {
	case *Selector:
		for _, c := range v.Children {
			if err := validateGotos(c, ids, path, state); err != nil {
				return err
			}
		}
	case *Sequence:
		for _, c := range v.Children {
			if err := validateGotos(c, ids, path, state); err != nil {
				return err
			}
		}
	case *Chance:
		for _, b := range v.Branches {
			if err := validateGotos(b.Node, ids, path, state); err != nil {
				return err
			}
		}
	case *Action:
		if v.Name == "goto" {
			tgt, _ := argString(v.Args, "state")
			if !ids[tgt] {
				return fmt.Errorf("behavior: %s: state %q goto target %q undeclared", path, state, tgt)
			}
		}
	}
	return nil
}

func buildNode(raw map[string]any, stateID string) (Node, error) {
	t, _ := raw["type"].(string)
	switch t {
	case "selector":
		children, err := buildChildren(raw, stateID)
		if err != nil {
			return nil, err
		}
		return &Selector{Children: children}, nil
	case "sequence":
		children, err := buildChildren(raw, stateID)
		if err != nil {
			return nil, err
		}
		return &Sequence{Children: children}, nil
	case "chance":
		branchesRaw, ok := raw["branches"].([]any)
		if !ok || len(branchesRaw) == 0 {
			return nil, fmt.Errorf("chance node has empty branches")
		}
		var branches []ChanceBranch
		for i, b := range branchesRaw {
			bm, _ := b.(map[string]any)
			w, err := argFloat(bm, "weight")
			if err != nil {
				return nil, fmt.Errorf("chance branch #%d: %w", i, err)
			}
			if w <= 0 {
				return nil, fmt.Errorf("chance branch #%d: weight must be > 0", i)
			}
			nodeRaw, ok := bm["node"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("chance branch #%d: missing node", i)
			}
			child, err := buildNode(nodeRaw, stateID)
			if err != nil {
				return nil, err
			}
			branches = append(branches, ChanceBranch{Weight: int(w), Node: child})
		}
		return &Chance{Branches: branches}, nil
	case "wait":
		s, err := argFloat(raw, "seconds")
		if err != nil {
			return nil, err
		}
		return &Wait{Seconds: s}, nil
	case "action":
		name, _ := raw["name"].(string)
		if !HasAction(name) {
			return nil, fmt.Errorf("unknown action %q", name)
		}
		args, _ := raw["args"].(map[string]any)
		return &Action{Name: name, Args: args}, nil
	case "condition":
		name, _ := raw["name"].(string)
		if !HasCondition(name) {
			return nil, fmt.Errorf("unknown condition %q", name)
		}
		args, _ := raw["args"].(map[string]any)
		return &Condition{Name: name, Args: args}, nil
	}
	return nil, fmt.Errorf("unknown node type %q", t)
}

func buildChildren(raw map[string]any, stateID string) ([]Node, error) {
	arr, _ := raw["children"].([]any)
	if len(arr) == 0 {
		return nil, fmt.Errorf("%q has no children", raw["type"])
	}
	var out []Node
	for i, c := range arr {
		cm, _ := c.(map[string]any)
		child, err := buildNode(cm, stateID)
		if err != nil {
			return nil, fmt.Errorf("child #%d: %w", i, err)
		}
		out = append(out, child)
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS (all Build* tests).

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/loader.go internal/behavior/loader_test.go
git commit -m "feat(behavior): JSON loader + node-tree validation"
```

---

## Task 7: End-to-end loader test + state-decl validation

**Files:**
- Modify: `internal/behavior/loader_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/behavior/loader_test.go`:

```go
import (
	"os"
	"path/filepath"
)

func writeTmp(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "k.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadFileMinimalValid(t *testing.T) {
	p := writeTmp(t, `{
      "kind": "orc",
      "states": [
        { "id":"run","anim":"run","decision":true,
          "bt": { "type":"action","name":"goto","args":{"state":"attack"} } },
        { "id":"attack","anim":"attack","decision":false,"exit_on":"anim_done","next":"run" }
      ]
    }`)
	f, err := LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if f.Kind != "orc" {
		t.Fatalf("kind = %q", f.Kind)
	}
	if len(f.States) != 2 {
		t.Fatalf("states = %d", len(f.States))
	}
	if !f.States[0].Decision || f.States[0].BT == nil {
		t.Fatalf("run state missing BT")
	}
	if f.States[1].Decision || f.States[1].BT != nil {
		t.Fatalf("attack state should be non-decision")
	}
}

func TestLoadFileDuplicateStateIDRejected(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":false,"exit_on":"anim_done","next":"run"},
        {"id":"run","anim":"run","decision":false,"exit_on":"anim_done","next":"run"}
      ]
    }`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "duplicate state id") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileGotoUndeclaredRejected(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":true,
         "bt":{"type":"action","name":"goto","args":{"state":"somewhere"}}}
      ]
    }`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "somewhere") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileNextUndeclaredRejected(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"attack","anim":"attack","decision":false,"exit_on":"anim_done","next":"nope"}
      ]
    }`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileNextDeadAllowed(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"death","anim":"death","decision":false,"exit_on":"anim_done","next":"__dead"}
      ]
    }`)
	if _, err := LoadFile(p); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
}

func TestLoadFileUnknownOnExitActionRejected(t *testing.T) {
	p := writeTmp(t, `{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":false,"exit_on":"grounded","next":"__dead",
         "on_exit_actions":["nope_nada"]}
      ]
    }`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "nope_nada") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileDecisionWithoutBTRejected(t *testing.T) {
	p := writeTmp(t, `{"kind":"orc","states":[{"id":"run","anim":"run","decision":true}]}`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "missing bt") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileNonDecisionWithBTRejected(t *testing.T) {
	p := writeTmp(t, `{"kind":"orc","states":[
      {"id":"run","anim":"run","decision":false,"exit_on":"anim_done","next":"__dead",
       "bt":{"type":"action","name":"stop"}}
    ]}`)
	_, err := LoadFile(p)
	if err == nil || !strings.Contains(err.Error(), "must not have bt") {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run TestLoadFile -v`
Expected: all the "rejected" tests fail until validation in Task 6's implementation actually runs (some may pass depending on error wording — update expected strings to match).

- [ ] **Step 3: Adjust implementation as needed**

If any expected error wording doesn't match, adjust `loader.go` error messages to match test expectations (or adjust test strings — pick whichever is clearer). No new code needed otherwise.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/loader_test.go internal/behavior/loader.go
git commit -m "test(behavior): end-to-end LoadFile validation cases"
```

---

## Task 8: Golden behavior files — `orc.json` + `slime.json` + README

**Files:**
- Create: `assets/behaviors/orc.json`
- Create: `assets/behaviors/slime.json`
- Create: `assets/behaviors/README.md`
- Modify: `internal/behavior/loader_test.go` (add golden-file load test)

The JSON must mirror **current** behavior to preserve feel after the swap: 2 s intent reroll, 50/50 decision (attack vs not), inside the attack arm 50/50 attack/attack2, outside 50/50 keep-running vs flip. Plus slime's attack2 frame-3–5 backstep VX=-60.

- [ ] **Step 1: Write the failing test**

Append to `internal/behavior/loader_test.go`:

```go
func TestLoadFileGoldenOrc(t *testing.T) {
	f, err := LoadFile("../../assets/behaviors/orc.json")
	if err != nil {
		t.Fatalf("LoadFile orc: %v", err)
	}
	if f.Kind != "orc" {
		t.Fatalf("kind = %q", f.Kind)
	}
	needStates := []string{"fall", "run", "attack", "attack2", "hurt", "death"}
	have := map[string]bool{}
	for _, s := range f.States {
		have[s.ID] = true
	}
	for _, id := range needStates {
		if !have[id] {
			t.Fatalf("missing state %q in orc.json", id)
		}
	}
}

func TestLoadFileGoldenSlime(t *testing.T) {
	f, err := LoadFile("../../assets/behaviors/slime.json")
	if err != nil {
		t.Fatalf("LoadFile slime: %v", err)
	}
	var a2 *State
	for i := range f.States {
		if f.States[i].ID == "attack2" {
			a2 = &f.States[i]
			break
		}
	}
	if a2 == nil {
		t.Fatal("slime: missing attack2 state")
	}
	if len(a2.OnFrameVX) != 1 {
		t.Fatalf("slime attack2 on_frame_vx = %d, want 1", len(a2.OnFrameVX))
	}
	fv := a2.OnFrameVX[0]
	if fv.FrameStart != 3 || fv.FrameEnd != 5 || fv.VX != -60 {
		t.Fatalf("slime attack2 on_frame_vx = %+v", fv)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run Golden -v`
Expected: FAIL (files don't exist).

- [ ] **Step 3: Write the golden files**

`assets/behaviors/orc.json`:

```json
{
  "kind": "orc",
  "states": [
    {
      "id": "fall",
      "anim": "idle",
      "decision": false,
      "exit_on": "grounded",
      "next": "run",
      "on_exit_actions": ["randomize_facing"]
    },
    {
      "id": "run",
      "anim": "run",
      "decision": true,
      "bt": {
        "type": "sequence",
        "children": [
          { "type": "action", "name": "set_vx_forward", "args": { "speed": 80 } },
          { "type": "wait", "seconds": 2 },
          {
            "type": "chance",
            "branches": [
              {
                "weight": 50,
                "node": {
                  "type": "chance",
                  "branches": [
                    { "weight": 50, "node": { "type": "action", "name": "goto", "args": { "state": "attack" } } },
                    { "weight": 50, "node": { "type": "action", "name": "goto", "args": { "state": "attack2" } } }
                  ]
                }
              },
              {
                "weight": 50,
                "node": {
                  "type": "chance",
                  "branches": [
                    { "weight": 50, "node": { "type": "action", "name": "flip_facing" } },
                    { "weight": 50, "node": { "type": "action", "name": "stop" } }
                  ]
                }
              }
            ]
          }
        ]
      }
    },
    { "id": "attack",  "anim": "attack",  "decision": false, "exit_on": "anim_done", "next": "run" },
    { "id": "attack2", "anim": "attack2", "decision": false, "exit_on": "anim_done", "next": "run" },
    { "id": "hurt",    "anim": "hurt",    "decision": false, "exit_on": "anim_done_and_grounded", "next": "run",
                       "on_exit_actions": ["randomize_facing"] },
    { "id": "death",   "anim": "death",   "decision": false, "exit_on": "anim_done", "next": "__dead" }
  ]
}
```

`assets/behaviors/slime.json`:

```json
{
  "kind": "slime",
  "states": [
    {
      "id": "fall",
      "anim": "idle",
      "decision": false,
      "exit_on": "grounded",
      "next": "run",
      "on_exit_actions": ["randomize_facing"]
    },
    {
      "id": "run",
      "anim": "run",
      "decision": true,
      "bt": {
        "type": "sequence",
        "children": [
          { "type": "action", "name": "set_vx_forward", "args": { "speed": 60 } },
          { "type": "wait", "seconds": 2 },
          {
            "type": "chance",
            "branches": [
              {
                "weight": 50,
                "node": {
                  "type": "chance",
                  "branches": [
                    { "weight": 50, "node": { "type": "action", "name": "goto", "args": { "state": "attack" } } },
                    { "weight": 50, "node": { "type": "action", "name": "goto", "args": { "state": "attack2" } } }
                  ]
                }
              },
              {
                "weight": 50,
                "node": {
                  "type": "chance",
                  "branches": [
                    { "weight": 50, "node": { "type": "action", "name": "flip_facing" } },
                    { "weight": 50, "node": { "type": "action", "name": "stop" } }
                  ]
                }
              }
            ]
          }
        ]
      }
    },
    { "id": "attack",  "anim": "attack",  "decision": false, "exit_on": "anim_done", "next": "run" },
    { "id": "attack2", "anim": "attack2", "decision": false, "exit_on": "anim_done", "next": "run",
                       "on_frame_vx": [ { "frame_start": 3, "frame_end": 5, "vx": -60 } ] },
    { "id": "hurt",    "anim": "hurt",    "decision": false, "exit_on": "anim_done_and_grounded", "next": "run",
                       "on_exit_actions": ["randomize_facing"] },
    { "id": "death",   "anim": "death",   "decision": false, "exit_on": "anim_done", "next": "__dead" }
  ]
}
```

`assets/behaviors/README.md`:

```markdown
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/behavior/... -v`
Expected: PASS (Golden tests plus everything else).

- [ ] **Step 5: Commit**

```bash
git add assets/behaviors/ internal/behavior/loader_test.go
git commit -m "feat(assets): golden behavior JSON for orc + slime + README"
```

---

## Task 9: `enemy.StateDecl` wrapper

The `internal/behavior` package exports `State`, but the `internal/enemy` package wants its own typed struct with richer references (resolved anim pointer, runtime scratch for `Tree` resets). Introduce `enemy.StateDecl` that converts from `behavior.State`.

**Files:**
- Create: `internal/enemy/state_decl.go`
- Create: `internal/enemy/state_decl_test.go`

- [ ] **Step 1: Write the failing test**

`internal/enemy/state_decl_test.go`:

```go
package enemy

import (
	"testing"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/behavior"
)

func TestConvertStatesResolvesAnims(t *testing.T) {
	anims := map[string]*anim.Animation{
		"run":    {},
		"attack": {},
	}
	bStates := []behavior.State{
		{ID: "run", Anim: "run", Decision: true, BT: &behavior.Tree{}},
		{ID: "attack", Anim: "attack", Decision: false, ExitOn: "anim_done", Next: "run"},
	}
	decls, err := ConvertStates(bStates, anims)
	if err != nil {
		t.Fatalf("ConvertStates: %v", err)
	}
	if decls["run"].Anim != anims["run"] {
		t.Fatalf("run anim not resolved")
	}
	if decls["attack"].Next != "run" {
		t.Fatalf("attack next = %q", decls["attack"].Next)
	}
}

func TestConvertStatesMissingAnimError(t *testing.T) {
	bStates := []behavior.State{
		{ID: "run", Anim: "run", Decision: true, BT: &behavior.Tree{}},
	}
	_, err := ConvertStates(bStates, map[string]*anim.Animation{})
	if err == nil {
		t.Fatal("expected error for missing anim")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/enemy/... -run TestConvertStates -v`
Expected: FAIL with `undefined: ConvertStates`.

- [ ] **Step 3: Write minimal implementation**

`internal/enemy/state_decl.go`:

```go
package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/behavior"
)

// StateDecl is the enemy-side view of a behavior file state. Anim pointer
// is resolved against the kind's animation library; BT is the parsed tree
// for decision states (nil otherwise).
type StateDecl struct {
	ID            string
	Anim          *anim.Animation
	AnimKey       string
	Decision      bool
	BT            *behavior.Tree
	ExitOn        string
	Next          string
	OnExitActions []string
	OnFrameVX     []behavior.FrameVX
}

// ConvertStates turns the generic behavior.State list into enemy StateDecls
// keyed by ID. Fails if an anim key is not present in lib.
func ConvertStates(bStates []behavior.State, lib map[string]*anim.Animation) (map[string]*StateDecl, error) {
	out := make(map[string]*StateDecl, len(bStates))
	for _, s := range bStates {
		a, ok := lib[s.Anim]
		if !ok {
			return nil, fmt.Errorf("state %q: missing anim %q in library", s.ID, s.Anim)
		}
		out[s.ID] = &StateDecl{
			ID:            s.ID,
			Anim:          a,
			AnimKey:       s.Anim,
			Decision:      s.Decision,
			BT:            s.BT,
			ExitOn:        s.ExitOn,
			Next:          s.Next,
			OnExitActions: s.OnExitActions,
			OnFrameVX:     s.OnFrameVX,
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/enemy/... -run TestConvertStates -v`
Expected: PASS. (Other tests in the package may still pass because we haven't changed existing code yet.)

- [ ] **Step 5: Commit**

```bash
git add internal/enemy/state_decl.go internal/enemy/state_decl_test.go
git commit -m "feat(enemy): StateDecl + ConvertStates bridge to behavior pkg"
```

---

## Task 10: Extend `Kind` to hold states and behavior path (no runtime swap yet)

**Files:**
- Modify: `internal/enemy/kind.go`
- Modify: `internal/enemy/loader.go`
- Create: `internal/enemy/behavior_load.go`
- Modify: `cmd/game/main.go`

This task wires the behavior file load into `BuildKind`, but the FSM still runs the old `states.go`. The behavior data is attached to `Kind.States` and ignored by the runtime — a safe parallel state.

- [ ] **Step 1: Write the failing test**

Create `internal/enemy/behavior_load_test.go`:

```go
package enemy

import (
	"testing"

	"claude-pixel/internal/anim"
)

func TestLoadBehaviorResolvesStates(t *testing.T) {
	lib := map[string]*anim.Animation{
		"orc_idle": {}, "orc_run": {}, "orc_attack": {}, "orc_attack2": {},
		"orc_hurt": {}, "orc_death": {},
	}
	states, initial, err := LoadBehavior("../../assets/behaviors/orc.json", "orc_", lib)
	if err != nil {
		t.Fatalf("LoadBehavior: %v", err)
	}
	if initial != "fall" {
		t.Fatalf("initial = %q, want fall", initial)
	}
	for _, id := range []string{"fall", "run", "attack", "attack2", "hurt", "death"} {
		if _, ok := states[id]; !ok {
			t.Fatalf("missing state %q", id)
		}
	}
	if states["run"].BT == nil {
		t.Fatal("run state missing BT")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/enemy/... -run TestLoadBehavior -v`
Expected: FAIL with `undefined: LoadBehavior`.

- [ ] **Step 3: Write minimal implementation**

`internal/enemy/behavior_load.go`:

```go
package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/behavior"
)

// LoadBehavior reads a kind's JSON, resolves anim keys against the library
// (prefixing with the kind's AnimPrefix), and returns state declarations +
// the initial state id (the id of the first state in the file).
func LoadBehavior(path, animPrefix string, lib map[string]*anim.Animation) (map[string]*StateDecl, string, error) {
	f, err := behavior.LoadFile(path)
	if err != nil {
		return nil, "", err
	}
	if len(f.States) == 0 {
		return nil, "", fmt.Errorf("behavior %s: no states", path)
	}
	prefixed := make(map[string]*anim.Animation, len(lib))
	for _, s := range f.States {
		key := animPrefix + s.Anim
		a, ok := lib[key]
		if !ok {
			return nil, "", fmt.Errorf("state %q: anim %q not in library", s.ID, key)
		}
		prefixed[s.Anim] = a
	}
	decls, err := ConvertStates(f.States, prefixed)
	if err != nil {
		return nil, "", err
	}
	return decls, f.States[0].ID, nil
}
```

Modify `internal/enemy/kind.go` — add fields and wire load into `BuildKind`:

```go
package enemy

import (
	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

type AttackMotion struct {
	VX         float64
	FrameStart int
	FrameEnd   int
}

type Kind struct {
	Name         string
	AnimPrefix   string
	FrameW       int
	FrameH       int
	Tuning       *Tuning
	Boxes        map[string]combat.Box
	Anims        map[string]*anim.Animation
	Motions      map[string]AttackMotion
	States       map[string]*StateDecl
	InitialState string
	BehaviorPath string
}

type KindConfig struct {
	Name         string
	Prefix       string
	FrameW       int
	FrameH       int
	AnimLib      map[string]*anim.Animation
	HitboxSpecs  []combat.HitboxSpec
	MotionSpecs  []combat.AttackMotionSpec
	TuneRepo     *storage.Repository[player.TuningParam]
	RenderScale  int
	BehaviorPath string
}

func BuildKind(cfg KindConfig) (*Kind, error) {
	anims, err := AnimsFor(cfg.AnimLib, cfg.Prefix)
	if err != nil {
		return nil, err
	}
	boxes, err := BoxesFor(cfg.HitboxSpecs, cfg.Name, cfg.RenderScale)
	if err != nil {
		return nil, err
	}
	tuning, err := LoadTuningFor(cfg.TuneRepo, cfg.Prefix)
	if err != nil {
		return nil, err
	}
	motions := MotionsFor(cfg.MotionSpecs, cfg.Name)
	k := &Kind{
		Name:         cfg.Name,
		AnimPrefix:   cfg.Prefix,
		FrameW:       cfg.FrameW,
		FrameH:       cfg.FrameH,
		Tuning:       tuning,
		Boxes:        boxes,
		Anims:        anims,
		Motions:      motions,
		BehaviorPath: cfg.BehaviorPath,
	}
	if cfg.BehaviorPath != "" {
		states, initial, err := LoadBehavior(cfg.BehaviorPath, cfg.Prefix+"_", cfg.AnimLib)
		if err != nil {
			return nil, err
		}
		k.States = states
		k.InitialState = initial
	}
	return k, nil
}
```

Modify `cmd/game/main.go` — pass `BehaviorPath`:

Find the existing `orcKind` / `slimeKind` `BuildKind` calls (around lines 82–97). Replace with:

```go
	orcKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "orc", Prefix: "orc", FrameW: 100, FrameH: 100,
		AnimLib: anims, HitboxSpecs: hitboxSpecs, MotionSpecs: motionSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
		BehaviorPath: cfg.AssetsDir + "/behaviors/orc.json",
	})
	if err != nil {
		log.Fatalf("build orc kind: %v", err)
	}

	slimeKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "slime", Prefix: "slime", FrameW: 96, FrameH: 96,
		AnimLib: anims, HitboxSpecs: hitboxSpecs, MotionSpecs: motionSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
		BehaviorPath: cfg.AssetsDir + "/behaviors/slime.json",
	})
	if err != nil {
		log.Fatalf("build slime kind: %v", err)
	}
```

- [ ] **Step 4: Run tests + build**

Run: `go test ./... && go build ./...`
Expected: PASS. Game still runs the old hardcoded FSM; behavior file is loaded but ignored at runtime.

- [ ] **Step 5: Commit**

```bash
git add internal/enemy/behavior_load.go internal/enemy/behavior_load_test.go \
        internal/enemy/kind.go cmd/game/main.go
git commit -m "feat(enemy): BuildKind loads behavior JSON (parallel, not wired to FSM yet)"
```

---

## Task 11: Rewrite FSM — generic driver (RED)

**Files:**
- Modify: `internal/enemy/fsm.go`
- Modify: `internal/enemy/fsm_test.go`

This task writes the new generic driver AND its tests. The existing `states.go` still exists — we'll remove it in Task 12 once the new driver is proven.

- [ ] **Step 1: Write the failing test**

Replace the contents of `internal/enemy/fsm_test.go` with:

```go
package enemy

import (
	"math/rand"
	"testing"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/behavior"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
)

func makeTestKind(t *testing.T) *Kind {
	t.Helper()
	a := func() *anim.Animation {
		return anim.NewAnimation([]*anim.Frame{
			{Duration: 100 * time.Millisecond},
			{Duration: 100 * time.Millisecond},
		}, false)
	}
	lib := map[string]*anim.Animation{
		"idle": a(), "run": a(), "attack": a(), "attack2": a(), "hurt": a(), "death": a(),
	}
	states := map[string]*StateDecl{
		"fall":    {ID: "fall", Anim: lib["idle"], AnimKey: "idle", Decision: false, ExitOn: "grounded", Next: "run", OnExitActions: []string{"randomize_facing"}},
		"run":     {ID: "run", Anim: lib["run"], AnimKey: "run", Decision: true, BT: &behavior.Tree{Root: &behavior.Action{Name: "goto", Args: map[string]any{"state": "attack"}}}},
		"attack":  {ID: "attack", Anim: lib["attack"], AnimKey: "attack", Decision: false, ExitOn: "anim_done", Next: "run"},
		"hurt":    {ID: "hurt", Anim: lib["hurt"], AnimKey: "hurt", Decision: false, ExitOn: "anim_done_and_grounded", Next: "run"},
		"death":   {ID: "death", Anim: lib["death"], AnimKey: "death", Decision: false, ExitOn: "anim_done", Next: "__dead"},
	}
	return &Kind{
		Name:         "test",
		AnimPrefix:   "test",
		FrameW:       10, FrameH: 10,
		Tuning:       &Tuning{MaxLives: 2, HurtBounceVX: 100, HurtBounceVY: -50, FootPadding: 0, Points: 0},
		Boxes:        map[string]combat.Box{"body": {W: 10, H: 10}},
		Anims:        lib,
		States:       states,
		InitialState: "fall",
	}
}

func newTestEnemy(t *testing.T) *Enemy {
	return New(Config{
		StartX: 0, StartY: 0,
		Physics: &player.Physics{Gravity: 0, MaxFallSpeed: 0},
		Kind:    makeTestKind(t),
		RNG:     rand.New(rand.NewSource(1)),
	})
}

func TestFallTransitionsToRunOnGrounded(t *testing.T) {
	e := newTestEnemy(t)
	if e.CurrentState != "fall" {
		t.Fatalf("initial = %q", e.CurrentState)
	}
	e.Grounded = true
	e.Tick(16 * time.Millisecond)
	if e.CurrentState != "run" {
		t.Fatalf("after grounded, state = %q", e.CurrentState)
	}
}

func TestDecisionStateRunsBT(t *testing.T) {
	e := newTestEnemy(t)
	e.CurrentState = "run"
	e.Grounded = true
	e.Tick(16 * time.Millisecond)
	if e.CurrentState != "attack" {
		t.Fatalf("BT goto didn't fire: state = %q", e.CurrentState)
	}
}

func TestAnimDoneTransitionsToNext(t *testing.T) {
	e := newTestEnemy(t)
	e.CurrentState = "attack"
	e.Grounded = true
	// Fast-forward through attack animation frames until Done.
	for i := 0; i < 10 && e.CurrentState == "attack"; i++ {
		e.Tick(60 * time.Millisecond)
	}
	if e.CurrentState != "run" {
		t.Fatalf("after anim_done: state = %q", e.CurrentState)
	}
}

func TestOnHitGoesToHurtBypassingBT(t *testing.T) {
	e := newTestEnemy(t)
	e.CurrentState = "run"
	e.Grounded = true
	e.OnHit(10)
	if e.CurrentState != "hurt" {
		t.Fatalf("state = %q, want hurt", e.CurrentState)
	}
}

func TestLivesZeroGoesToDeath(t *testing.T) {
	e := newTestEnemy(t)
	e.Lives = 0
	e.Grounded = true
	e.Tick(16 * time.Millisecond)
	if e.CurrentState != "death" {
		t.Fatalf("state = %q, want death", e.CurrentState)
	}
}

func TestDeathAnimDoneSetsDead(t *testing.T) {
	e := newTestEnemy(t)
	e.CurrentState = "death"
	e.Grounded = true
	for i := 0; i < 10 && !e.Dead; i++ {
		e.Tick(60 * time.Millisecond)
	}
	if !e.Dead {
		t.Fatal("Dead still false after death anim")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/enemy/... -v`
Expected: FAIL — compile errors (`Enemy.Tick`, `Enemy.CurrentState`, etc. don't exist yet).

- [ ] **Step 3: Write minimal implementation**

Replace `internal/enemy/fsm.go` with:

```go
package enemy

import (
	"time"

	"claude-pixel/internal/behavior"
)

// Tick runs one frame of the generic FSM driver. Priority order:
//   1. Engine-owned event transitions (hit, death, grounded-from-fall).
//   2. Non-decision states: anim tick + exit_on rule.
//   3. Decision states: BT.Tick, then honor ctx.PendingGoto.
func (e *Enemy) Tick(dt time.Duration) {
	// 1. Event transitions.
	if e.OnHitPending {
		e.OnHitPending = false
		e.transition("hurt")
		return
	}
	if e.Lives <= 0 && e.CurrentState != "death" {
		e.transition("death")
		return
	}
	if e.CurrentState == "fall" && e.Grounded {
		e.runOnExitActions("fall")
		e.transition("run")
		return
	}

	st := e.States()[e.CurrentState]
	if st == nil {
		return
	}

	// Anim tick + on_frame_vx.
	if st.Anim != nil {
		st.Anim.Update(dt)
	}
	if vx, ok := currentFrameVX(st, animFrame(st.Anim)); ok {
		e.VX = float64(e.Facing) * vx
	}

	// 2. Non-decision state.
	if !st.Decision {
		if exitRuleMet(e, st) {
			e.runOnExitActionsFor(st)
			if st.Next == "__dead" {
				e.Dead = true
				return
			}
			if st.Next != "" {
				e.transition(st.Next)
			}
		}
		return
	}

	// 3. Decision state: run BT.
	if st.BT == nil {
		return
	}
	ctx := &behavior.Ctx{Enemy: enemyAdapter{e: e}, DT: dt, RNG: e.rng}
	st.BT.Root.Tick(ctx)
	e.BranchTag = ctx.BranchTag
	if ctx.PendingGoto != "" && ctx.PendingGoto != e.CurrentState {
		e.transition(ctx.PendingGoto)
	}
}

func (e *Enemy) transition(to string) {
	e.CurrentState = to
	st := e.States()[to]
	if st == nil {
		return
	}
	if st.Anim != nil {
		st.Anim.Reset()
		e.Current = st.Anim
		e.CurrentAnim = st.AnimKey
	}
	// Enter-side side-effects: decision states start with VX 0 unless the
	// BT re-sets it; non-decision stops on entry.
	e.VX = 0
	// Reset per-state BT state by recreating the Tree wrapper — Chance /
	// Wait nodes hold their own progress; a fresh tick starts them from
	// scratch. Simpler than a per-node Reset method for v1.
	//
	// (Note: the tree instance is *shared* across enemies; sharing means
	// all enemies of a kind advance the same Wait timer unless we clone.
	// See Task 14 addendum for the clone-per-enemy fix.)
}

func (e *Enemy) runOnExitActions(stateID string) {
	st := e.States()[stateID]
	if st == nil {
		return
	}
	e.runOnExitActionsFor(st)
}

func (e *Enemy) runOnExitActionsFor(st *StateDecl) {
	if len(st.OnExitActions) == 0 {
		return
	}
	ctx := &behavior.Ctx{Enemy: enemyAdapter{e: e}, RNG: e.rng}
	for _, name := range st.OnExitActions {
		_, _ = behavior.RunAction(name, nil, ctx)
	}
}

// States is a convenience accessor so the driver can call e.States()[id]
// without dereferencing Kind in every branch.
func (e *Enemy) States() map[string]*StateDecl { return e.Kind.States }

func exitRuleMet(e *Enemy, st *StateDecl) bool {
	switch st.ExitOn {
	case "anim_done":
		return st.Anim != nil && st.Anim.Done()
	case "anim_done_and_grounded":
		return st.Anim != nil && st.Anim.Done() && e.Grounded
	case "grounded":
		return e.Grounded
	}
	return false
}

func currentFrameVX(st *StateDecl, frame int) (float64, bool) {
	for _, fv := range st.OnFrameVX {
		if frame >= fv.FrameStart && frame <= fv.FrameEnd {
			return fv.VX, true
		}
	}
	return 0, false
}

func animFrame(a interface{ Frame() int }) int {
	if a == nil {
		return 0
	}
	return a.Frame()
}
```

- [ ] **Step 4: Stop here — expect FAILING build**

`Tick`, `CurrentState`, `OnHitPending`, `enemyAdapter`, and the updated `Enemy` struct aren't written yet. Task 12 introduces them.

Do NOT commit yet. Continue to Task 12.

---

## Task 12: Enemy struct overhaul + adapter + remove old states.go (GREEN)

**Files:**
- Modify: `internal/enemy/enemy.go`
- Create: `internal/enemy/adapter.go`
- Delete: `internal/enemy/states.go`

- [ ] **Step 1: Update the Enemy struct**

Rewrite `internal/enemy/enemy.go`:

```go
package enemy

import (
	"math/rand"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
	"claude-pixel/internal/world"
)

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
	Kind         *Kind
	Current      *anim.Animation
	CurrentAnim  string
	CurrentState string
	BranchTag    string
	HitSet       map[combat.Fighter]bool
	Dead         bool
	OnHitPending bool
	rng          *rand.Rand
}

func New(cfg Config) *Enemy {
	e := &Enemy{
		X:       cfg.StartX,
		Y:       cfg.StartY,
		Facing:  1,
		Lives:   int(cfg.Kind.Tuning.MaxLives),
		Physics: cfg.Physics,
		Kind:    cfg.Kind,
		HitSet:  map[combat.Fighter]bool{},
		rng:     cfg.RNG,
	}
	e.CurrentState = cfg.Kind.InitialState
	if st := cfg.Kind.States[e.CurrentState]; st != nil && st.Anim != nil {
		e.Current = st.Anim
		e.CurrentAnim = st.AnimKey
	}
	return e
}

func (e *Enemy) PlayAnim(id string) {
	a, ok := e.Kind.Anims[id]
	if !ok {
		return
	}
	a.Reset()
	e.Current = a
	e.CurrentAnim = id
}

func (e *Enemy) ApplyPhysics(w *world.World, dt time.Duration) {
	dtS := dt.Seconds()
	e.VY += e.Physics.Gravity * dtS
	if e.VY > e.Physics.MaxFallSpeed {
		e.VY = e.Physics.MaxFallSpeed
	}
	e.X += e.VX * dtS
	e.Y += e.VY * dtS
	if e.Y >= w.GroundY {
		e.Y = w.GroundY
		e.VY = 0
		e.Grounded = true
	} else {
		e.Grounded = false
	}
}

// OnHit is called by combat resolution. Applies knockback, decrements
// lives, and marks a pending Hurt transition. The FSM driver picks up
// OnHitPending on the next Tick and bypasses the BT.
func (e *Enemy) OnHit(attackerX float64) {
	e.Lives--
	if e.Lives <= 0 {
		// Death bypasses Hurt — FSM driver will notice Lives<=0.
		return
	}
	dir := 1.0
	if attackerX > e.X {
		dir = -1.0
	}
	e.VX = dir * e.Kind.Tuning.HurtBounceVX
	e.VY = e.Kind.Tuning.HurtBounceVY
	e.Grounded = false
	e.OnHitPending = true
}
```

- [ ] **Step 2: Create the adapter**

`internal/enemy/adapter.go`:

```go
package enemy

// enemyAdapter bridges *Enemy into the behavior.EnemyTarget interface so
// the behavior package can mutate enemies without importing internal/enemy.
type enemyAdapter struct{ e *Enemy }

func (a enemyAdapter) Facing() int      { return a.e.Facing }
func (a enemyAdapter) SetFacing(f int)  { a.e.Facing = f }
func (a enemyAdapter) SetVX(v float64)  { a.e.VX = v }
func (a enemyAdapter) Grounded() bool   { return a.e.Grounded }
func (a enemyAdapter) PlayAnim(id string) {
	// Prefer the resolved StateDecl anim (already in Kind.Anims). PlayAnim
	// on the enemy expects an unprefixed id.
	a.e.PlayAnim(id)
}
func (a enemyAdapter) CurrentAnimDone() bool {
	return a.e.Current != nil && a.e.Current.Done()
}
func (a enemyAdapter) CurrentAnimFrame() int {
	if a.e.Current == nil {
		return 0
	}
	return a.e.Current.Frame()
}
```

- [ ] **Step 3: Delete the old state structs**

```bash
git rm internal/enemy/states.go
```

Also drop the old FSM scaffolding at the top of `internal/enemy/fsm.go` (Task 11 already replaced the file body, but if any stale `StateID` / `State` / `FSM` definitions remain, remove them).

- [ ] **Step 4: Remove stale `FSM` references from callers**

Grep for `enemy.StateID`, `FSM`, `StateFall` etc. outside `internal/enemy`:

```bash
grep -rn "enemy\\.State\|\\bFSM\\b" --include="*.go"
```

In `internal/game/game.go` specifically: any `enemy.StateHurt`-style reference becomes a string `"hurt"`. Replace call sites that check FSM state with `enemy.CurrentState`. Any `e.FSM.Handle(dt)` becomes `e.Tick(dt)`.

Expected edits in `internal/game/game.go` (search for `enemy.State` / `FSM.Handle` / `FSM.CurrentID`):

- Replace `enem.FSM.Handle(e, dt)` → `enem.Tick(dt)` (API moved to Enemy).
- Replace `enem.FSM.CurrentID() == enemy.StateDeath` → `enem.CurrentState == "death"`.
- Replace `enem.FSM.CurrentID()` (debug HUD) → `enemy.StateID(enem.CurrentState)` → drop the cast and use the string.

- [ ] **Step 5: Run tests**

Run: `go test ./... -v`
Expected: PASS. If `internal/combat` still references `enemy.AttackMotion` via some glue, defer that to Task 14.

- [ ] **Step 6: Commit**

```bash
git add internal/enemy/ internal/game/game.go
git commit -m "feat(enemy): generic FSM driver, remove hardcoded states.go"
```

---

## Task 13: Drop `RunSpeed` / `IntentTickS` from `enemy.Tuning`

**Files:**
- Modify: `internal/enemy/tuning.go`
- Modify: `internal/enemy/fsm_test.go` (ensure makeTestKind still compiles)

- [ ] **Step 1: Write the failing test**

Append to `internal/enemy/fsm_test.go`:

```go
func TestTuningHasNoRunSpeedOrIntentTick(t *testing.T) {
	var tn Tuning
	// Verify fields removed at the type level by asserting the struct is
	// constructable without those names. This test exists to fail compile
	// if someone re-adds them — at which point the schema-reshape decision
	// needs to be revisited.
	_ = tn
	// Reflect-based assertion:
	typ := reflect.TypeOf(Tuning{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if name == "RunSpeed" || name == "IntentTickS" {
			t.Fatalf("Tuning should no longer have field %q — moved to behavior JSON", name)
		}
	}
}
```

Add the import:

```go
import "reflect"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/enemy/... -run TestTuningHas -v`
Expected: FAIL (fields still present).

- [ ] **Step 3: Write minimal implementation**

In `internal/enemy/tuning.go`, delete `RunSpeed`, `IntentTickS` fields from the `Tuning` struct and drop the matching entries in `LoadTuningFor`'s key list. The remaining pick block:

```go
type Tuning struct {
	MaxLives     float64
	HurtBounceVX float64
	HurtBounceVY float64
	FootPadding  int
	Points       int
}

// ...

	t := &Tuning{}
	keys := []struct {
		k string
		p *float64
	}{
		{prefix + "_max_lives", &t.MaxLives},
		{prefix + "_hurt_bounce_vx", &t.HurtBounceVX},
		{prefix + "_hurt_bounce_vy", &t.HurtBounceVY},
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/enemy/... -v`
Expected: PASS.

Run: `go build ./...`
Expected: FAIL — `cmd/game/main.go` or other callers may still reference removed fields; fix by removing those references.

Run build again after fixes.

- [ ] **Step 5: Commit**

```bash
git add internal/enemy/
git commit -m "refactor(enemy): drop RunSpeed/IntentTickS from Tuning — moved to JSON"
```

---

## Task 14: Clone BT per enemy (fix cross-enemy state bleed)

**Files:**
- Modify: `internal/behavior/tree.go`, `internal/behavior/nodes.go`, `internal/behavior/action.go`
- Modify: `internal/enemy/enemy.go`
- Modify: `internal/enemy/state_decl.go`

Because `Wait.elapsed` and `Chance.active` are per-node fields, sharing a `*Tree` across enemies causes them to share timers. Each enemy must get its own tree clone. Clone at `enemy.New` time.

- [ ] **Step 1: Write the failing test**

Append to `internal/behavior/tree_test.go`:

```go
func TestCloneTreeIsolatesWaitState(t *testing.T) {
	src := &Tree{Root: &Wait{Seconds: 1.0}}
	a := CloneTree(src)
	b := CloneTree(src)
	ctx := &Ctx{DT: 600 * time.Millisecond}
	a.Root.Tick(ctx) // a.elapsed = 0.6
	a.Root.Tick(ctx) // a.elapsed = 1.2 → Success, reset to 0
	bCtx := &Ctx{DT: 100 * time.Millisecond}
	if got := b.Root.Tick(bCtx); got != StatusRunning {
		t.Fatalf("clone b shouldn't have inherited elapsed time: got %v", got)
	}
}

func TestCloneTreePreservesChanceBranches(t *testing.T) {
	src := &Tree{Root: &Chance{Branches: []ChanceBranch{
		{Weight: 1, Node: &Action{Name: "stop"}},
	}}}
	c := CloneTree(src)
	ch := c.Root.(*Chance)
	if len(ch.Branches) != 1 {
		t.Fatalf("branches = %d", len(ch.Branches))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/behavior/... -run TestCloneTree -v`
Expected: FAIL with `undefined: CloneTree`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/behavior/tree.go`:

```go
// CloneTree deep-clones t so per-node runtime state (Wait elapsed, Chance
// active branch) is independent of other users of the same source tree.
func CloneTree(t *Tree) *Tree {
	if t == nil || t.Root == nil {
		return &Tree{}
	}
	return &Tree{Root: cloneNode(t.Root)}
}

func cloneNode(n Node) Node {
	switch v := n.(type) {
	case *Selector:
		out := &Selector{Children: make([]Node, len(v.Children))}
		for i, c := range v.Children {
			out.Children[i] = cloneNode(c)
		}
		return out
	case *Sequence:
		out := &Sequence{Children: make([]Node, len(v.Children))}
		for i, c := range v.Children {
			out.Children[i] = cloneNode(c)
		}
		return out
	case *Chance:
		out := &Chance{Branches: make([]ChanceBranch, len(v.Branches))}
		for i, b := range v.Branches {
			out.Branches[i] = ChanceBranch{Weight: b.Weight, Node: cloneNode(b.Node)}
		}
		return out
	case *Wait:
		return &Wait{Seconds: v.Seconds}
	case *Action:
		return &Action{Name: v.Name, Args: v.Args}
	case *Condition:
		return &Condition{Name: v.Name, Args: v.Args}
	}
	return n
}
```

Update `internal/enemy/state_decl.go` — per-enemy state maps need their own trees. Add a helper:

```go
// CloneStates returns a copy of decls where each decision state has an
// independent BT. Non-decision states are shallow-copied (no runtime state).
func CloneStates(decls map[string]*StateDecl) map[string]*StateDecl {
	out := make(map[string]*StateDecl, len(decls))
	for id, d := range decls {
		cp := *d
		if d.BT != nil {
			cp.BT = behavior.CloneTree(d.BT)
		}
		out[id] = &cp
	}
	return out
}
```

Update `internal/enemy/enemy.go` — `Enemy` gains its own `states` map populated in `New`:

```go
type Enemy struct {
	// … existing fields …
	states map[string]*StateDecl
}

func New(cfg Config) *Enemy {
	e := &Enemy{
		// … existing …
	}
	e.states = CloneStates(cfg.Kind.States)
	e.CurrentState = cfg.Kind.InitialState
	if st := e.states[e.CurrentState]; st != nil && st.Anim != nil {
		e.Current = st.Anim
		e.CurrentAnim = st.AnimKey
	}
	return e
}
```

Rewrite `(*Enemy).States()` in `fsm.go`:

```go
func (e *Enemy) States() map[string]*StateDecl { return e.states }
```

- [ ] **Step 4: Run tests**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/behavior/ internal/enemy/
git commit -m "fix(behavior): clone BT per enemy so Wait/Chance timers are isolated"
```

---

## Task 15: Migration cleanup — drop `attack_motions` + 4 tuning rows

**Files:**
- Modify: `internal/storage/migrations/001_init_schema.sql`
- Modify: `internal/storage/migrations/002_seed_data.sql`
- Modify: `CLAUDE.md`

Per the user's global memory, schema/seed changes go directly into migrations 001/002. User wipes `data/` every test cycle.

- [ ] **Step 1: Remove the `attack_motions` CREATE TABLE**

Edit `internal/storage/migrations/001_init_schema.sql` — delete the entire `CREATE TABLE attack_motions (…)` block.

- [ ] **Step 2: Remove seed rows**

Edit `internal/storage/migrations/002_seed_data.sql`:

1. Delete the four tuning rows:
   - `('orc_run_speed', …)`
   - `('orc_intent_tick_s', …)`
   - `('slime_run_speed', …)`
   - `('slime_intent_tick_s', …)`
2. Delete the entire `INSERT INTO attack_motions …` block and its banner comment.

- [ ] **Step 3: Update CLAUDE.md tuning count**

In `CLAUDE.md`, change `Current keys (30):` → `Current keys (26):` and remove the table rows for the 4 deleted keys. Also delete the "Motions (`attack_motions` table)" subsection entirely — that CLI subcommand goes away in Task 16.

- [ ] **Step 4: Verify**

```bash
rm -rf data/
go test ./internal/storage/... -v
```

Expected: PASS. Then:

```bash
go run ./cmd/tune list | wc -l
```

Expected: 26 data rows + 1 header row = 27 lines (or similar, depending on tabwriter output). Verify no `orc_run_speed` / `orc_intent_tick_s` / `slime_*` deleted keys.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/migrations/001_init_schema.sql internal/storage/migrations/002_seed_data.sql CLAUDE.md
git commit -m "feat(migrations): drop attack_motions table + 4 tuning rows moved to JSON"
```

---

## Task 16: Remove `combat/motion.go` + `cmd/tune` motions subcommand

**Files:**
- Delete: `internal/combat/motion.go`, `internal/combat/motion_test.go`
- Modify: `cmd/tune/main.go` (remove `motions` subcommand)
- Modify: `cmd/game/main.go` (drop motionRepo wiring)
- Modify: `internal/enemy/kind.go` (drop `MotionSpecs` field)
- Modify: `internal/enemy/loader.go` (drop `MotionsFor`)
- Modify: `internal/enemy/kind.go` (drop `Motions` field on `Kind`, `AttackMotion` type if unused)

- [ ] **Step 1: Remove motion files**

```bash
git rm internal/combat/motion.go internal/combat/motion_test.go
```

- [ ] **Step 2: Strip motions subcommand from `cmd/tune/main.go`**

Find the `"motions"` switch arm and delete it plus any helper functions only it uses. Verify with:

```bash
go build ./cmd/tune
```

- [ ] **Step 3: Strip motion wiring from `cmd/game/main.go`**

Remove:

```go
motionRepo := storage.NewRepository[combat.AttackMotionSpec](db, combat.AttackMotionMapper{})
…
motionSpecs, err := motionRepo.List(context.Background())
if err != nil {
    log.Fatalf("list attack_motions: %v", err)
}
```

…and the `MotionSpecs: motionSpecs` fields on each `enemy.KindConfig{…}`.

- [ ] **Step 4: Strip `MotionSpecs` from `KindConfig` and `MotionsFor`**

Edit `internal/enemy/kind.go` and `internal/enemy/loader.go`:

- Delete `MotionSpecs []combat.AttackMotionSpec` field from `KindConfig`.
- Delete the `motions := MotionsFor(...)` call in `BuildKind`. Drop `Motions` field from `Kind` and the `AttackMotion` struct. Drop `MotionsFor` from `loader.go`.
- Any `internal/enemy/loader_test.go` test for `MotionsFor` — delete.

- [ ] **Step 5: Build + test**

```bash
go build ./...
go test ./...
```

Expected: PASS across the board.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove combat/motion + tune motions subcommand + Kind.Motions"
```

---

## Task 17: F5 hot reload in `internal/game/game.go`

**Files:**
- Modify: `internal/game/game.go`
- Create: `internal/enemy/reload.go`
- Create: `internal/enemy/reload_test.go`

- [ ] **Step 1: Write the failing test**

`internal/enemy/reload_test.go`:

```go
package enemy

import (
	"os"
	"path/filepath"
	"testing"

	"claude-pixel/internal/anim"
)

func TestReloadBehaviorSwapsStates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "orc.json")
	os.WriteFile(p, []byte(`{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":true,
         "bt":{"type":"action","name":"goto","args":{"state":"attack"}}},
        {"id":"attack","anim":"attack","decision":false,"exit_on":"anim_done","next":"run"}
      ]}`), 0o600)

	lib := map[string]*anim.Animation{"orc_run": {}, "orc_attack": {}}
	k := &Kind{
		Name: "orc", AnimPrefix: "orc", AnimLib: nil,
		BehaviorPath: p,
	}
	_ = lib // for completeness — ReloadBehavior takes its own lib arg

	if err := ReloadBehavior(k, lib); err != nil {
		t.Fatalf("ReloadBehavior: %v", err)
	}
	if len(k.States) != 2 {
		t.Fatalf("states = %d", len(k.States))
	}

	// Rewrite file with a different shape, reload, expect updated count.
	os.WriteFile(p, []byte(`{
      "kind":"orc",
      "states":[
        {"id":"run","anim":"run","decision":true,
         "bt":{"type":"action","name":"stop"}}
      ]}`), 0o600)
	if err := ReloadBehavior(k, lib); err != nil {
		t.Fatalf("ReloadBehavior (2): %v", err)
	}
	if len(k.States) != 1 {
		t.Fatalf("states after reload = %d", len(k.States))
	}
}

func TestReloadBehaviorBadFileKeepsOldState(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "orc.json")
	os.WriteFile(p, []byte(`{
      "kind":"orc",
      "states":[{"id":"run","anim":"run","decision":true,
                "bt":{"type":"action","name":"stop"}}]}`), 0o600)
	lib := map[string]*anim.Animation{"orc_run": {}}
	k := &Kind{Name: "orc", AnimPrefix: "orc", BehaviorPath: p}
	if err := ReloadBehavior(k, lib); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	prev := k.States

	// Corrupt the file.
	os.WriteFile(p, []byte(`{ not json`), 0o600)
	if err := ReloadBehavior(k, lib); err == nil {
		t.Fatal("expected error from malformed JSON")
	}
	if len(k.States) != len(prev) {
		t.Fatalf("states replaced despite error: now %d", len(k.States))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/enemy/... -run TestReloadBehavior -v`
Expected: FAIL with `undefined: ReloadBehavior`.

- [ ] **Step 3: Write implementation**

`internal/enemy/reload.go`:

```go
package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
)

// ReloadBehavior re-parses k.BehaviorPath and swaps k.States/InitialState.
// On parse error, k is left untouched and the error is returned so callers
// can log and keep playing.
func ReloadBehavior(k *Kind, lib map[string]*anim.Animation) error {
	if k.BehaviorPath == "" {
		return fmt.Errorf("kind %q has no BehaviorPath", k.Name)
	}
	states, initial, err := LoadBehavior(k.BehaviorPath, k.AnimPrefix+"_", lib)
	if err != nil {
		return err
	}
	k.States = states
	k.InitialState = initial
	return nil
}
```

- [ ] **Step 4: Wire F5 in `internal/game/game.go`**

Locate the `Update` method. After the existing key-intent block, add:

```go
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		for _, k := range g.kinds {
			if err := enemy.ReloadBehavior(k, g.anims); err != nil {
				log.Printf("behavior reload failed for %q: %v", k.Name, err)
			}
		}
	}
```

Add imports if missing:

```go
import (
	"log"
	// … existing imports …
)
```

Also add an `anims` field to the `Game` struct and thread it through `New`:

```go
type Game struct {
	// …
	anims map[string]*anim.Animation
}

func New(d Deps) *Game {
	// …
	return &Game{
		// …
		anims: d.Anims,
		kinds: d.EnemyKinds,
		// …
	}
}
```

Note: existing enemies continue ticking the old cloned trees (since clones live on `Enemy.states`), so reload only takes effect for *newly spawned* enemies. This is the simplest correct behavior and matches the spec's "swap the tree on the `*Kind`, existing enemies re-resolve via `e.States[id]`" — but since we clone, re-resolve actually means *new* enemies. Document this caveat in `assets/behaviors/README.md` under the Reload section:

```markdown
Note: F5 swaps the source tree on `*Kind`. Existing live enemies keep their
original cloned tree until they despawn. Respawn to see changes apply to
everything.
```

- [ ] **Step 5: Run tests + build**

```bash
go test ./...
go build ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/enemy/reload.go internal/enemy/reload_test.go \
        internal/game/game.go assets/behaviors/README.md
git commit -m "feat(game): F5 hot-reload enemy behavior JSON (new spawns only)"
```

---

## Task 18: Debug overlay — `enemy_state` + `enemy_bt_last_branch`

**Files:**
- Modify: `internal/debug/fields.go`
- Modify: `internal/game/game.go`
- Modify: `CLAUDE.md`
- Modify: `config/debug.json`

- [ ] **Step 1: Check existing field catalog**

```bash
grep -n "Label" internal/debug/fields.go | head -5
grep -n "orc_lives\|orc_state" internal/debug/fields.go
```

Read the file to see the exact pattern for a field entry (key, label, provider func).

- [ ] **Step 2: Add two new fields**

In `internal/debug/fields.go`, append two entries to the catalog:

```go
	"enemy_state":           {Label: "enemy.state", Get: func(p *Provider) string { return p.EnemyState }},
	"enemy_bt_last_branch":  {Label: "enemy.bt", Get: func(p *Provider) string { return p.EnemyBranch }},
```

Add `EnemyState string` and `EnemyBranch string` to the `Provider` struct.

- [ ] **Step 3: Populate provider in `internal/game/game.go`**

In the `Update` method where the debug overlay is populated, compute nearest enemy to player and set:

```go
	if nearest := g.nearestEnemy(); nearest != nil {
		g.overlay.Provider.EnemyState = nearest.CurrentState
		g.overlay.Provider.EnemyBranch = nearest.BranchTag
	} else {
		g.overlay.Provider.EnemyState = "(none)"
		g.overlay.Provider.EnemyBranch = ""
	}
```

If `nearestEnemy()` doesn't exist, add it — scan `g.enemies`, pick the one with smallest `|X - playerX|`.

- [ ] **Step 4: Add fields to `config/debug.json`**

Read `config/debug.json`, add `"enemy_state"` and `"enemy_bt_last_branch"` keys to the layout — match the shape of existing fields. Coordinates: pick any slot under the existing orc_lives block.

- [ ] **Step 5: Update CLAUDE.md field count**

Change "Catalog: `internal/debug/fields.go` (23 fields: 19 player/engine + 4 orc/lives)" → "(25 fields: 19 player/engine + 4 orc/lives + 2 behavior)".

- [ ] **Step 6: Build + smoke test**

```bash
go build ./...
go test ./internal/debug/...
```

- [ ] **Step 7: Commit**

```bash
git add internal/debug/fields.go internal/game/game.go config/debug.json CLAUDE.md
git commit -m "feat(debug): show current enemy state + BT branch tag in F3 overlay"
```

---

## Task 19: Update `CLAUDE.md` with new control + reference

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add F5 to the controls table**

Under the Controls section, add:

```markdown
| Reload behavior JSON | `F5` (edge) |
```

- [ ] **Step 2: Add behavior doc reference**

Under the "Designs/plans" bullets at the top:

```markdown
- `docs/superpowers/specs/2026-04-24-enemy-behavior-json-design.md` + plan
```

- [ ] **Step 3: Add behavior layout section**

Append a new subsection after the "State machines" section:

```markdown
## Behavior JSON

Per-kind flowchart-like behavior lives in `assets/behaviors/<kind>.json`.
Runtime: `internal/behavior/`. State list, BT for decision states,
per-frame VX, on-exit actions are all declarative. See
`assets/behaviors/README.md` for the schema and `internal/behavior/loader.go`
for the validator. Press **F5** in-game to reload without restart (only
new spawns pick up the change).
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): document behavior JSON + F5 reload"
```

---

## Task 20: Manual verification

No code changes. Run through this checklist and note any deviations — file a follow-up issue rather than hotpatching.

- [ ] **Step 1: Clean DB + boot**

```bash
rm -rf data/
make run
```

Expected: game window opens. Spawns begin after initial delay.

- [ ] **Step 2: Verify orc decision variety**

Watch for ~30 seconds. Orcs should:
- Fall from above screen, land, pick a random facing.
- Run in that direction.
- Every ~2 s either attack (≈25% chance each for attack1 / attack2), flip facing (≈25%), or stop briefly (≈25%).

If behavior feels identical to pre-change: PASS.

- [ ] **Step 3: Verify slime backstep**

Land a few hits near a slime. When the slime fires `attack2`, its sprite should briefly slide backward during frames 3–5. If it no longer backsteps: check `on_frame_vx` parsed correctly in `slime.json` and that `fsm.go.currentFrameVX` is being called.

- [ ] **Step 4: Verify F3 shows new fields**

Press F3. Confirm `enemy.state` and `enemy.bt` lines appear. State should change as the nearest enemy transitions. `enemy.bt` should briefly show paths like `chance#0/attack` on the tick a Run-state BT fires.

- [ ] **Step 5: Verify F5 reload**

Edit `assets/behaviors/orc.json` — change the outermost chance weights to 100/0 (all-attack). Save. In-game, press F5. Wait for next orc spawn — it should immediately attack on the first intent reroll.

- [ ] **Step 6: Verify graceful failure**

Delete the closing `}` from `orc.json`. Press F5. Expected:
- Error logged to stdout / terminal: `behavior reload failed for "orc": …`.
- Game continues with prior tree intact (existing enemies keep behavior).
- Restore the file to valid JSON before committing anything else.

- [ ] **Step 7: Verify no regressions**

- Player combat (J/X, K/C) still lands hits.
- Soldier Hit animation + lives decrement still work.
- GAME OVER triggers on 10 hits.
- Pause (Esc) + resume still work.
- HUD still renders (heart + stamina + score).
- Debug overlay F3 toggles on/off cleanly.
- Hitbox debug F4 still draws body/attack rects.

- [ ] **Step 8: Final commit / push**

If everything passes, no code commit needed — work is already landed across prior tasks. Optionally tag:

```bash
git tag -a behavior-json-v1 -m "Behavior JSON + BT runtime landed"
```

---

## Self-review summary

**Spec coverage:**
- Architecture diagram → Tasks 1-6 (scaffold + nodes + loader).
- JSON schema (node types, state decl, on_frame_vx, on_exit_actions) → Tasks 2, 3, 5, 6, 7, 9.
- Golden orc/slime → Task 8.
- FSM driver rewrite → Tasks 11 + 12.
- `attack_motions` removal + tuning prune → Task 15 + Task 16.
- `Kind` gains `BehaviorPath` + load → Task 10.
- F5 reload → Task 17.
- Debug overlay fields → Task 18.
- CLAUDE.md updates → Tasks 15, 18, 19.
- Manual verification → Task 20.

**Placeholder scan:** None. Every step contains the exact code, test, or command.

**Type consistency:**
- `EnemyTarget` interface appears identically in `internal/behavior/enemy_iface.go` (Task 1) and the stub in tests.
- `Tree`, `Ctx`, `Status` consistent across Tasks 1–6.
- `StateDecl` (enemy side) vs `State` (behavior side) kept distinct deliberately — conversion in Task 9.
- `Enemy.Tick` introduced in Task 11, struct updated in Task 12; callers updated same task.
- `CloneTree` / `CloneStates` pair introduced Task 14 before any multi-enemy behavior test.

---

## Plan complete and saved to `docs/superpowers/plans/2026-04-24-enemy-behavior-json.md`.

Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
