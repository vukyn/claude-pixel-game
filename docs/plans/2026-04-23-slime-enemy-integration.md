# Slime Enemy Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add slime as a second enemy kind alongside orc, driven by reusable `Kind` + `AttackMotion` abstractions and a multi-kind spawner capped at 3 concurrent enemies.

**Architecture:** Parameterize `enemy.Enemy` with a `Kind` struct (name, anim prefix, frame dims, tuning, boxes, motions). Store unprefixed anim keys inside `Kind.Anims` so FSM states use static strings. New `attack_motions` table drives optional per-attack horizontal displacement applied on specific frame windows. Spawner accepts a list of `KindFactory` entries and picks uniformly per spawn. Global spawn tuning (`enemy_spawn_*`, `enemy_max_alive`) replaces the old orc-scoped keys.

**Tech Stack:** Go 1.22, Ebiten v2, SQLite via `database/sql` + embedded migrations, `urfave/cli/v3`, stdlib `testing`.

**Spec:** `docs/superpowers/specs/2026-04-23-slime-enemy-integration-design.md`

---

## Strategy

Refactor proceeds in stages that keep the codebase compiling at each commit:

1. **Migrations** (Tasks 1–6) — additive SQL only; no code changes.
2. **New combat type** (Task 7) — `AttackMotionSpec` + mapper; standalone.
3. **New enemy types alongside old** (Tasks 8–10) — `AttackMotion`, `Kind`, generic loaders, split tuning. Old `OrcAnims`/`OrcBoxes`/`LoadTuning` remain callable.
4. **Switch `Enemy` to `Kind`** (Tasks 11–14) — atomic refactor of enemy struct + FSM states + fighter accessors; delete dead loader symbols.
5. **Atomic Kind refactor** (Task 11) — enemy struct + FSM states + fighter + spawner + game wiring + main.go all in one commit.
6. **Motion hook tests** (Task 12) — slime backstep + orc no-motion regression.
7. **Tune CLI motions** (Task 13) — mirror `hitboxes` subcommand shape.
8. **Docs + manual verify** (Tasks 14–15) — `CLAUDE.md` + fresh-DB smoke run.

Every task ends with `go build ./...` + relevant `go test`. Commit after each task unless noted.

---

## Task 1: Migration 017 — seed slime animations

**Files:**
- Create: `internal/storage/migrations/017_seed_slime_animations.sql`

- [ ] **Step 1: Write migration**

```sql
-- 017_seed_slime_animations.sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path, is_player, is_enemy)
VALUES
    ('slime_idle',    'Idle.png',     6, 900, 1, 96, 96, 'slime/Idle.png',    0, 1),
    ('slime_run',     'Run.png',      8, 700, 1, 96, 96, 'slime/Run.png',     0, 1),
    ('slime_attack',  'Attack.png',   8, 650, 0, 96, 96, 'slime/Attack.png',  0, 1),
    ('slime_attack2', 'Attack2.png',  8, 700, 0, 96, 96, 'slime/Attack2.png', 0, 1),
    ('slime_hurt',    'Hurt.png',     4, 400, 0, 96, 96, 'slime/Hurt.png',    0, 1),
    ('slime_death',   'Death.png',   10, 800, 0, 96, 96, 'slime/Death.png',   0, 1);
```

- [ ] **Step 2: Apply + verify**

Run: `rm -rf data/ && go run ./cmd/game &` then immediately `sleep 2 && pkill -f "cmd/game"` (just trigger migration apply). Alternative without launching game: `go test ./internal/storage/... -run TestRepository -count=1` (exercises `MustOpen`).

Then query:

```bash
sqlite3 data/game.db "SELECT id, frame_count, frame_w, frame_h FROM animations WHERE id LIKE 'slime_%' ORDER BY id;"
```

Expected:
```
slime_attack|8|96|96
slime_attack2|8|96|96
slime_death|10|96|96
slime_hurt|4|96|96
slime_idle|6|96|96
slime_run|8|96|96
```

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/017_seed_slime_animations.sql
git commit -m "feat(storage): seed slime animations"
```

---

## Task 2: Migration 018 — seed slime hitboxes

**Files:**
- Create: `internal/storage/migrations/018_seed_slime_hitboxes.sql`

- [ ] **Step 1: Write migration**

```sql
-- 018_seed_slime_hitboxes.sql
INSERT OR IGNORE INTO hitboxes
    (id, owner, kind, offset_x, offset_y, width, height, active_frame_start, active_frame_end)
VALUES
    ('slime_body',    'slime', 'body',    -20, -40, 40, 40, -1, -1),
    ('slime_attack',  'slime', 'attack',   15, -35, 45, 35,  4,  5),
    ('slime_attack2', 'slime', 'attack2',  15, -35, 55, 40,  3,  5);
```

- [ ] **Step 2: Apply + verify**

Run: `rm -rf data/ && go test ./internal/storage/... -count=1` then:

```bash
sqlite3 data/game.db "SELECT id, kind, width, height, active_frame_start, active_frame_end FROM hitboxes WHERE owner='slime';"
```

Expected three rows matching the migration.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/018_seed_slime_hitboxes.sql
git commit -m "feat(storage): seed slime hitboxes"
```

---

## Task 3: Migration 019 — attack_motions schema

**Files:**
- Create: `internal/storage/migrations/019_attack_motions_schema.sql`

- [ ] **Step 1: Write migration**

```sql
-- 019_attack_motions_schema.sql
CREATE TABLE attack_motions (
    id                 TEXT    PRIMARY KEY,
    owner              TEXT    NOT NULL,
    kind               TEXT    NOT NULL,
    vx                 INTEGER NOT NULL,
    frame_start        INTEGER NOT NULL,
    frame_end          INTEGER NOT NULL
);
```

- [ ] **Step 2: Apply + verify**

Run: `rm -rf data/ && go test ./internal/storage/... -count=1`

```bash
sqlite3 data/game.db ".schema attack_motions"
```

Expected: `CREATE TABLE attack_motions (...)`. Then confirm empty: `sqlite3 data/game.db "SELECT COUNT(*) FROM attack_motions;"` → `0`.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/019_attack_motions_schema.sql
git commit -m "feat(storage): attack_motions table schema"
```

---

## Task 4: Migration 020 — seed slime attack2 motion

**Files:**
- Create: `internal/storage/migrations/020_seed_slime_attack_motion.sql`

- [ ] **Step 1: Write migration**

```sql
-- 020_seed_slime_attack_motion.sql
-- Slime Attack2: retreats (negative VX relative to facing) on frames 3-5 (0-indexed).
INSERT OR IGNORE INTO attack_motions
    (id, owner, kind, vx, frame_start, frame_end)
VALUES
    ('slime_attack2_motion', 'slime', 'attack2', -60, 3, 5);
```

- [ ] **Step 2: Apply + verify**

Run: `rm -rf data/ && go test ./internal/storage/... -count=1`

```bash
sqlite3 data/game.db "SELECT id, owner, kind, vx, frame_start, frame_end FROM attack_motions;"
```

Expected: `slime_attack2_motion|slime|attack2|-60|3|5`.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/020_seed_slime_attack_motion.sql
git commit -m "feat(storage): seed slime attack2 backstep motion"
```

---

## Task 5: Migration 021 — rename orc spawn keys to enemy_*

**Files:**
- Create: `internal/storage/migrations/021_rename_spawn_keys.sql`

- [ ] **Step 1: Write migration**

```sql
-- 021_rename_spawn_keys.sql
-- Move orc_spawn_*/orc_max_alive to global enemy_* scope.
INSERT INTO tuning (key, value, min_value, max_value, unit, description)
SELECT 'enemy_spawn_min_s', value, min_value, max_value, unit, 'minimum enemy spawn interval (all kinds)'
FROM tuning WHERE key='orc_spawn_min_s';

INSERT INTO tuning (key, value, min_value, max_value, unit, description)
SELECT 'enemy_spawn_max_s', value, min_value, max_value, unit, 'maximum enemy spawn interval (all kinds)'
FROM tuning WHERE key='orc_spawn_max_s';

INSERT INTO tuning (key, value, min_value, max_value, unit, description)
SELECT 'enemy_max_alive', value, min_value, max_value, unit, 'max concurrent enemies (all kinds)'
FROM tuning WHERE key='orc_max_alive';

DELETE FROM tuning WHERE key IN ('orc_spawn_min_s','orc_spawn_max_s','orc_max_alive');
```

- [ ] **Step 2: Apply + verify**

Run: `rm -rf data/ && go test ./internal/storage/... -count=1`

```bash
sqlite3 data/game.db "SELECT key, value FROM tuning WHERE key LIKE 'enemy_%' OR key LIKE 'orc_spawn%' OR key = 'orc_max_alive' ORDER BY key;"
```

Expected:
```
enemy_max_alive|3
enemy_spawn_max_s|10
enemy_spawn_min_s|3
```

No `orc_spawn_*` or `orc_max_alive` rows.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/021_rename_spawn_keys.sql
git commit -m "feat(storage): rename orc spawn keys to enemy_* scope"
```

---

## Task 6: Migration 022 — seed slime tuning

**Files:**
- Create: `internal/storage/migrations/022_seed_slime_tuning.sql`

- [ ] **Step 1: Write migration**

```sql
-- 022_seed_slime_tuning.sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('slime_max_lives',         2,    1,  10,  '',     'starting slime lives'),
    ('slime_run_speed',        60,    0, 500, 'px/s',  'slime ground speed'),
    ('slime_intent_tick_s',     2,  0.5,  10, 's',     'slime intent reroll period'),
    ('slime_hurt_bounce_vx',  120,    0, 500, 'px/s',  'slime hurt horizontal bounce'),
    ('slime_hurt_bounce_vy', -180, -500,   0, 'px/s',  'slime hurt vertical pop'),
    ('slime_foot_padding',     20,    0,  96, 'px',    'transparent px at slime sprite frame bottom');
```

- [ ] **Step 2: Apply + verify**

Run: `rm -rf data/ && go test ./internal/storage/... -count=1`

```bash
sqlite3 data/game.db "SELECT key, value, unit FROM tuning WHERE key LIKE 'slime_%' ORDER BY key;"
```

Expected 6 rows.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/migrations/022_seed_slime_tuning.sql
git commit -m "feat(storage): seed slime tuning keys"
```

---

## Task 7: AttackMotionSpec + mapper

**Files:**
- Create: `internal/combat/motion.go`
- Create: `internal/combat/motion_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/combat/motion_test.go`:

```go
package combat

import (
	"context"
	"testing"

	"claude-pixel/internal/config"
	"claude-pixel/internal/storage"
)

func TestAttackMotionRepositoryRoundtrip(t *testing.T) {
	cfg := &config.Config{DBPath: ":memory:"}
	db, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	repo := storage.NewRepository[AttackMotionSpec](db, AttackMotionMapper{})
	in := AttackMotionSpec{
		ID:         "test_motion",
		Owner:      "slime",
		Kind:       "attack2",
		VX:         -60,
		FrameStart: 3,
		FrameEnd:   5,
	}
	if err := repo.Upsert(context.Background(), in); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	out, err := repo.Get(context.Background(), "test_motion")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: got %+v want %+v", out, in)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/combat/... -run TestAttackMotion -v`
Expected: FAIL with "undefined: AttackMotionSpec" / "undefined: AttackMotionMapper".

- [ ] **Step 3: Write motion.go**

Create `internal/combat/motion.go`:

```go
package combat

import "claude-pixel/internal/storage"

type AttackMotionSpec struct {
	ID         string
	Owner      string
	Kind       string
	VX         float64
	FrameStart int
	FrameEnd   int
}

func (s AttackMotionSpec) GetID() string { return s.ID }

type AttackMotionMapper struct{}

func (AttackMotionMapper) Table() string { return "attack_motions" }

func (AttackMotionMapper) Columns() []string {
	return []string{"id", "owner", "kind", "vx", "frame_start", "frame_end"}
}

func (AttackMotionMapper) Scan(row storage.Scanner) (AttackMotionSpec, error) {
	var s AttackMotionSpec
	err := row.Scan(&s.ID, &s.Owner, &s.Kind, &s.VX, &s.FrameStart, &s.FrameEnd)
	return s, err
}

func (AttackMotionMapper) Values(s AttackMotionSpec) []any {
	return []any{s.ID, s.Owner, s.Kind, s.VX, s.FrameStart, s.FrameEnd}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/combat/... -run TestAttackMotion -v`
Expected: PASS.

Also verify full suite: `go test ./...` — should be green.

- [ ] **Step 5: Commit**

```bash
git add internal/combat/motion.go internal/combat/motion_test.go
git commit -m "feat(combat): AttackMotionSpec + mapper"
```

---

## Task 8: Introduce `Kind` + `AttackMotion` types (alongside existing enemy code)

**Files:**
- Create: `internal/enemy/kind.go`

- [ ] **Step 1: Write kind.go**

Create `internal/enemy/kind.go`:

```go
package enemy

import (
	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
)

// AttackMotion describes horizontal displacement applied during a specific
// frame window of an attack/attack2 state. VX is signed: positive = forward
// along the facing direction; negative = backward.
type AttackMotion struct {
	VX         float64
	FrameStart int
	FrameEnd   int
}

// Kind bundles per-enemy-kind metadata. All animation keys in Anims are
// unprefixed ("idle", "run", "attack", "attack2", "hurt", "death") so FSM
// states use static strings regardless of owner.
type Kind struct {
	Name       string
	AnimPrefix string
	FrameW     int
	FrameH     int
	Tuning     *Tuning
	Boxes      map[string]combat.Box
	Anims      map[string]*anim.Animation
	Motions    map[string]AttackMotion
}
```

- [ ] **Step 2: Verify compile**

Run: `go build ./...`
Expected: succeeds (new file adds types only).

- [ ] **Step 3: Commit**

```bash
git add internal/enemy/kind.go
git commit -m "feat(enemy): introduce Kind + AttackMotion types"
```

---

## Task 9: Generic loaders `AnimsFor`, `BoxesFor`, `MotionsFor`

**Files:**
- Modify: `internal/enemy/loader.go`
- Create: `internal/enemy/loader_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/enemy/loader_test.go`:

```go
package enemy

import (
	"testing"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
)

func TestAnimsForReturnsUnprefixedMap(t *testing.T) {
	stub := func(id string) *anim.Animation {
		return anim.NewAnimation(&anim.AnimationSpec{ID: id, FrameCount: 1, DurationMs: 100}, nil)
	}
	lib := map[string]*anim.Animation{
		"orc_idle":    stub("orc_idle"),
		"orc_run":     stub("orc_run"),
		"orc_attack":  stub("orc_attack"),
		"orc_attack2": stub("orc_attack2"),
		"orc_hurt":    stub("orc_hurt"),
		"orc_death":   stub("orc_death"),
		"soldier_idle": stub("soldier_idle"),
	}
	out, err := AnimsFor(lib, "orc")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	for _, k := range []string{"idle", "run", "attack", "attack2", "hurt", "death"} {
		if _, ok := out[k]; !ok {
			t.Errorf("missing unprefixed key %q", k)
		}
	}
	if _, ok := out["soldier_idle"]; ok {
		t.Errorf("should not leak soldier_idle into orc map")
	}
}

func TestAnimsForErrorsOnMissing(t *testing.T) {
	lib := map[string]*anim.Animation{"orc_idle": nil}
	_, err := AnimsFor(lib, "orc")
	if err == nil {
		t.Errorf("expected error for missing keys")
	}
}

func TestBoxesForFiltersByOwnerAndScales(t *testing.T) {
	specs := []combat.HitboxSpec{
		{ID: "orc_body", Owner: "orc", Kind: "body", Width: 50, Height: 80, FrameStart: -1, FrameEnd: -1},
		{ID: "orc_attack", Owner: "orc", Kind: "attack", Width: 60, Height: 60, FrameStart: 2, FrameEnd: 3},
		{ID: "slime_body", Owner: "slime", Kind: "body", Width: 40, Height: 40, FrameStart: -1, FrameEnd: -1},
	}
	out, err := BoxesFor(specs, "orc", 2)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out["body"].W != 100 || out["body"].H != 160 {
		t.Errorf("body not scaled: %+v", out["body"])
	}
	if _, ok := out["attack"]; !ok {
		t.Errorf("missing attack box")
	}
	if _, ok := out["slime_body"]; ok {
		t.Errorf("slime leaked into orc boxes")
	}
}

func TestBoxesForMissingBody(t *testing.T) {
	specs := []combat.HitboxSpec{
		{ID: "orc_attack", Owner: "orc", Kind: "attack", Width: 60, Height: 60, FrameStart: 2, FrameEnd: 3},
	}
	_, err := BoxesFor(specs, "orc", 1)
	if err == nil {
		t.Errorf("expected missing-body error")
	}
}

func TestMotionsForFiltersByOwner(t *testing.T) {
	specs := []combat.AttackMotionSpec{
		{ID: "slime_attack2_motion", Owner: "slime", Kind: "attack2", VX: -60, FrameStart: 3, FrameEnd: 5},
		{ID: "orc_attack_motion", Owner: "orc", Kind: "attack", VX: 30, FrameStart: 1, FrameEnd: 2},
	}
	out := MotionsFor(specs, "slime")
	if len(out) != 1 {
		t.Fatalf("want 1 motion, got %d", len(out))
	}
	m, ok := out["attack2"]
	if !ok {
		t.Fatalf("missing attack2")
	}
	if m.VX != -60 || m.FrameStart != 3 || m.FrameEnd != 5 {
		t.Errorf("wrong motion: %+v", m)
	}
}

func TestMotionsForEmptyWhenNoMatch(t *testing.T) {
	out := MotionsFor(nil, "orc")
	if len(out) != 0 {
		t.Errorf("want empty, got %d", len(out))
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/enemy/... -run "TestAnimsFor|TestBoxesFor|TestMotionsFor" -v`
Expected: FAIL with "undefined: AnimsFor" / "undefined: BoxesFor" / "undefined: MotionsFor".

- [ ] **Step 3: Rewrite loader.go with generic loaders**

Replace entire `internal/enemy/loader.go`:

```go
package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
)

var kindAnimKeys = []string{"idle", "run", "attack", "attack2", "hurt", "death"}

// AnimsFor picks the 6 animations belonging to `prefix` out of a loaded
// library. The returned map is keyed by unprefixed state name so FSM states
// are owner-agnostic.
func AnimsFor(lib map[string]*anim.Animation, prefix string) (map[string]*anim.Animation, error) {
	out := make(map[string]*anim.Animation, len(kindAnimKeys))
	for _, k := range kindAnimKeys {
		id := prefix + "_" + k
		a, ok := lib[id]
		if !ok {
			return nil, fmt.Errorf("%s anims: missing %q", prefix, id)
		}
		out[k] = a
	}
	return out, nil
}

// BoxesFor filters HitboxSpec list by owner and multiplies offsets/dims by
// scale. Requires a "body" box; "attack"/"attack2" are optional.
func BoxesFor(specs []combat.HitboxSpec, owner string, scale int) (map[string]combat.Box, error) {
	out := make(map[string]combat.Box, 3)
	for _, s := range specs {
		if s.Owner != owner {
			continue
		}
		out[s.Kind] = s.ToBox().Scale(scale)
	}
	if _, ok := out["body"]; !ok {
		return nil, fmt.Errorf("%s hitboxes: missing body", owner)
	}
	return out, nil
}

// MotionsFor filters AttackMotionSpec list by owner and returns a map keyed
// by kind ("attack" | "attack2"). Empty map (not error) if owner has none.
func MotionsFor(specs []combat.AttackMotionSpec, owner string) map[string]AttackMotion {
	out := map[string]AttackMotion{}
	for _, s := range specs {
		if s.Owner != owner {
			continue
		}
		out[s.Kind] = AttackMotion{VX: s.VX, FrameStart: s.FrameStart, FrameEnd: s.FrameEnd}
	}
	return out
}

// OrcAnims + OrcBoxes kept as thin wrappers until game.go switches to Kind
// in later tasks. Removed in Task 11.
func OrcAnims(lib map[string]*anim.Animation) (map[string]*anim.Animation, error) {
	return AnimsFor(lib, "orc")
}

func OrcBoxes(specs []combat.HitboxSpec, scale int) (map[string]combat.Box, error) {
	return BoxesFor(specs, "orc", scale)
}
```

Note: `OrcAnims` wrapper previously returned prefixed map; now returns unprefixed. That breaks callers — but only `main.go` + `fsm_test.go` consume it, and both are rewritten in Tasks 11/14/20. To keep this task isolated, restore the old prefix behavior for the wrapper:

```go
// OrcAnims kept for backwards compat during refactor. Returns prefixed keys
// like the original. Removed in Task 11.
func OrcAnims(lib map[string]*anim.Animation) (map[string]*anim.Animation, error) {
	want := []string{"orc_idle", "orc_run", "orc_attack", "orc_attack2", "orc_hurt", "orc_death"}
	out := make(map[string]*anim.Animation, len(want))
	for _, k := range want {
		a, ok := lib[k]
		if !ok {
			return nil, fmt.Errorf("orc anims: missing %q", k)
		}
		out[k] = a
	}
	return out, nil
}

// OrcBoxes kept for backwards compat during refactor. Removed in Task 11.
func OrcBoxes(specs []combat.HitboxSpec, scale int) (map[string]combat.Box, error) {
	return BoxesFor(specs, "orc", scale)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/enemy/... -v`
Expected: new loader tests PASS. Existing FSM tests still PASS (they build their own maps).

Run: `go build ./...` — must succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/enemy/loader.go internal/enemy/loader_test.go
git commit -m "feat(enemy): generic AnimsFor/BoxesFor/MotionsFor loaders"
```

---

## Task 10: Split tuning — per-kind `LoadTuningFor` + global `LoadSpawnTuning`

**Files:**
- Modify: `internal/enemy/tuning.go`

Existing callers (`cmd/game/main.go` via `enemy.LoadTuning`, then `internal/game/game.go` via `orcTuning.SpawnMinS`/`.SpawnMaxS`/`.MaxAlive`) must keep compiling. Solution: add the new types + loaders, keep the deprecated spawn fields on `Tuning`, rewire the old `LoadTuning` to populate them. Task 11 (atomic refactor) deletes the shim and both callers together.

- [ ] **Step 1: Rewrite tuning.go**

Replace entire `internal/enemy/tuning.go`:

```go
package enemy

import (
	"context"
	"fmt"

	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

// Tuning holds per-kind physics/AI knobs read from the tuning table with
// a prefix (e.g. "orc_run_speed", "slime_run_speed").
type Tuning struct {
	MaxLives     float64
	RunSpeed     float64
	IntentTickS  float64
	HurtBounceVX float64
	HurtBounceVY float64
	FootPadding  int

	// Deprecated shim fields — read via LoadSpawnTuning instead. Populated by
	// the legacy LoadTuning wrapper so pre-refactor callers keep compiling.
	// Removed in Task 11.
	SpawnMinS float64
	SpawnMaxS float64
	MaxAlive  float64
}

// SpawnTuning is global (all kinds) and lives under the enemy_* key prefix.
type SpawnTuning struct {
	MinS     float64
	MaxS     float64
	MaxAlive int
}

func loadTuneMap(repo *storage.Repository[player.TuningParam]) (map[string]float64, error) {
	params, err := repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	m := make(map[string]float64, len(params))
	for _, p := range params {
		m[p.Key] = p.Value
	}
	return m, nil
}

// LoadTuningFor reads six per-kind keys: <prefix>_max_lives, <prefix>_run_speed,
// <prefix>_intent_tick_s, <prefix>_hurt_bounce_vx, <prefix>_hurt_bounce_vy,
// <prefix>_foot_padding.
func LoadTuningFor(repo *storage.Repository[player.TuningParam], prefix string) (*Tuning, error) {
	m, err := loadTuneMap(repo)
	if err != nil {
		return nil, err
	}
	pick := func(k string) (float64, error) {
		v, ok := m[k]
		if !ok {
			return 0, fmt.Errorf("missing tuning key %q", k)
		}
		return v, nil
	}
	t := &Tuning{}
	keys := []struct {
		k string
		p *float64
	}{
		{prefix + "_max_lives", &t.MaxLives},
		{prefix + "_run_speed", &t.RunSpeed},
		{prefix + "_intent_tick_s", &t.IntentTickS},
		{prefix + "_hurt_bounce_vx", &t.HurtBounceVX},
		{prefix + "_hurt_bounce_vy", &t.HurtBounceVY},
	}
	for _, k := range keys {
		v, err := pick(k.k)
		if err != nil {
			return nil, err
		}
		*k.p = v
	}
	pad, err := pick(prefix + "_foot_padding")
	if err != nil {
		return nil, err
	}
	t.FootPadding = int(pad)
	return t, nil
}

// LoadSpawnTuning reads the three global spawn keys: enemy_spawn_min_s,
// enemy_spawn_max_s, enemy_max_alive.
func LoadSpawnTuning(repo *storage.Repository[player.TuningParam]) (*SpawnTuning, error) {
	m, err := loadTuneMap(repo)
	if err != nil {
		return nil, err
	}
	pick := func(k string) (float64, error) {
		v, ok := m[k]
		if !ok {
			return 0, fmt.Errorf("missing tuning key %q", k)
		}
		return v, nil
	}
	st := &SpawnTuning{}
	if v, err := pick("enemy_spawn_min_s"); err != nil {
		return nil, err
	} else {
		st.MinS = v
	}
	if v, err := pick("enemy_spawn_max_s"); err != nil {
		return nil, err
	} else {
		st.MaxS = v
	}
	if v, err := pick("enemy_max_alive"); err != nil {
		return nil, err
	} else {
		st.MaxAlive = int(v)
	}
	return st, nil
}

// LoadTuning keeps pre-refactor callers compiling. Loads orc per-kind tuning
// + populates the deprecated Spawn* fields from the global enemy_* keys so
// callers like internal/game/game.go still read them as before. Removed in
// Task 11 alongside the shim fields.
func LoadTuning(repo *storage.Repository[player.TuningParam]) (*Tuning, error) {
	t, err := LoadTuningFor(repo, "orc")
	if err != nil {
		return nil, err
	}
	st, err := LoadSpawnTuning(repo)
	if err != nil {
		return nil, err
	}
	t.SpawnMinS = st.MinS
	t.SpawnMaxS = st.MaxS
	t.MaxAlive = float64(st.MaxAlive)
	return t, nil
}
```

- [ ] **Step 2: Run tests + build**

Run: `go build ./...`
Expected: success. Existing callers (`main.go` → `LoadTuning`, `game.go` → `orcTuning.SpawnMinS`) still compile via shim fields.

Run: `go test ./...`
Expected: PASS. FSM test builds its own `Tuning` literal — ignores the new shim fields.

- [ ] **Step 3: Commit**

```bash
git add internal/enemy/tuning.go
git commit -m "feat(enemy): split tuning into per-kind + global SpawnTuning"
```

---

## Task 11: Atomic Kind refactor (enemy + game + main)

**Files:**
- Modify: `internal/enemy/enemy.go`
- Modify: `internal/enemy/fighter.go`
- Modify: `internal/enemy/states.go`
- Modify: `internal/enemy/fsm_test.go`
- Modify: `internal/game/game.go`
- Modify: `internal/debug/fields.go`
- Modify: `config/debug.json`
- Modify: `cmd/game/main.go`

Single atomic refactor. All files updated together. `go build ./...` + `go test ./...` must both pass at commit.

- [ ] **Step 1: Rewrite `internal/enemy/enemy.go`**

Replace the entire file with:

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
	FSM          *FSM
	Current      *anim.Animation
	CurrentAnim  string
	IntentTimer  float64
	HitSet       map[combat.Fighter]bool
	Dead         bool
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
	e.FSM = NewFSM(StateFall)
	e.FSM.Register(&fallState{})
	e.FSM.Register(&runState{})
	e.FSM.Register(&attackState{})
	e.FSM.Register(&attack2State{})
	e.FSM.Register(&hurtState{})
	e.FSM.Register(&deathState{})
	e.FSM.Start(e)
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

func (e *Enemy) OnHit(attackerX float64) {
	e.Lives--
	if e.Lives <= 0 {
		e.FSM.Transition(e, StateDeath)
		return
	}
	dir := 1.0
	if attackerX > e.X {
		dir = -1.0
	}
	e.VX = dir * e.Kind.Tuning.HurtBounceVX
	e.VY = e.Kind.Tuning.HurtBounceVY
	e.Grounded = false
	e.FSM.Transition(e, StateHurt)
}
```

- [ ] **Step 2: Rewrite `internal/enemy/fighter.go`**

Replace the entire file with:

```go
package enemy

import "claude-pixel/internal/combat"

func (e *Enemy) Pos() (float64, float64) { return e.X, e.Y }

func (e *Enemy) FacingDir() int { return e.Facing }

func (e *Enemy) CurrentAnimID() string { return e.CurrentAnim }

func (e *Enemy) CurrentFrame() int {
	if e.Current == nil {
		return 0
	}
	return e.Current.FrameIndex()
}

func (e *Enemy) Body() combat.Box { return e.Kind.Boxes["body"] }

func (e *Enemy) ActiveHits() []combat.Box {
	switch e.CurrentAnim {
	case "attack":
		box := e.Kind.Boxes["attack"]
		if box.Active(e.CurrentFrame()) {
			return []combat.Box{box}
		}
	case "attack2":
		box := e.Kind.Boxes["attack2"]
		if box.Active(e.CurrentFrame()) {
			return []combat.Box{box}
		}
	}
	return nil
}

func (e *Enemy) IsInvulnerable() bool {
	id := e.FSM.CurrentID()
	return id == StateHurt || id == StateDeath
}

func (e *Enemy) Alive() bool {
	return !e.Dead && e.FSM.CurrentID() != StateDeath
}

func (e *Enemy) AlreadyHit(t combat.Fighter) bool {
	if e.HitSet == nil {
		return false
	}
	return e.HitSet[t]
}

func (e *Enemy) MarkHit(t combat.Fighter) {
	if e.HitSet == nil {
		e.HitSet = map[combat.Fighter]bool{}
	}
	e.HitSet[t] = true
}
```

- [ ] **Step 3: Rewrite `internal/enemy/states.go`**

Replace the entire file with:

```go
package enemy

import (
	"time"

	"claude-pixel/internal/combat"
)

// applyMotion applies the per-frame horizontal displacement configured for
// an attack kind ("attack" | "attack2"). If no motion is configured, VX is
// left at whatever the Enter handler set (typically 0).
func applyMotion(e *Enemy, kind string) {
	m, ok := e.Kind.Motions[kind]
	if !ok {
		return
	}
	f := e.CurrentFrame()
	if f >= m.FrameStart && f <= m.FrameEnd {
		e.VX = float64(e.Facing) * m.VX
	} else {
		e.VX = 0
	}
}

type fallState struct{}

func (fallState) ID() StateID { return StateFall }
func (fallState) Enter(e *Enemy) {
	e.PlayAnim("idle")
	e.VX = 0
}
func (fallState) Exit(e *Enemy) {}
func (fallState) Update(e *Enemy, dt time.Duration) StateID {
	if e.Grounded {
		if e.rng.Intn(2) == 0 {
			e.Facing = 1
		} else {
			e.Facing = -1
		}
		return StateRun
	}
	return StateFall
}

type runState struct{}

func (runState) ID() StateID { return StateRun }
func (runState) Enter(e *Enemy) {
	e.PlayAnim("run")
	e.IntentTimer = e.Kind.Tuning.IntentTickS
}
func (runState) Exit(e *Enemy) {}
func (runState) Update(e *Enemy, dt time.Duration) StateID {
	e.VX = float64(e.Facing) * e.Kind.Tuning.RunSpeed

	e.IntentTimer -= dt.Seconds()
	if e.IntentTimer <= 0 {
		e.IntentTimer = e.Kind.Tuning.IntentTickS
		if e.rng.Float64() < 0.5 {
			if e.rng.Float64() < 0.5 {
				return StateAttack
			}
			return StateAttack2
		}
	}
	return StateRun
}

type attackState struct{}

func (attackState) ID() StateID { return StateAttack }
func (attackState) Enter(e *Enemy) {
	e.PlayAnim("attack")
	e.VX = 0
	e.HitSet = map[combat.Fighter]bool{}
}
func (attackState) Exit(e *Enemy) {}
func (attackState) Update(e *Enemy, dt time.Duration) StateID {
	applyMotion(e, "attack")
	if e.Current != nil && e.Current.Done() {
		return StateRun
	}
	return StateAttack
}

type attack2State struct{}

func (attack2State) ID() StateID { return StateAttack2 }
func (attack2State) Enter(e *Enemy) {
	e.PlayAnim("attack2")
	e.VX = 0
	e.HitSet = map[combat.Fighter]bool{}
}
func (attack2State) Exit(e *Enemy) {}
func (attack2State) Update(e *Enemy, dt time.Duration) StateID {
	applyMotion(e, "attack2")
	if e.Current != nil && e.Current.Done() {
		return StateRun
	}
	return StateAttack2
}

type hurtState struct{}

func (hurtState) ID() StateID { return StateHurt }
func (hurtState) Enter(e *Enemy) {
	e.PlayAnim("hurt")
}
func (hurtState) Exit(e *Enemy) {}
func (hurtState) Update(e *Enemy, dt time.Duration) StateID {
	if e.Current != nil && e.Current.Done() && e.Grounded {
		if e.rng.Intn(2) == 0 {
			e.Facing = 1
		} else {
			e.Facing = -1
		}
		return StateRun
	}
	return StateHurt
}

type deathState struct{}

func (deathState) ID() StateID { return StateDeath }
func (deathState) Enter(e *Enemy) {
	e.PlayAnim("death")
	e.VX = 0
}
func (deathState) Exit(e *Enemy) {}
func (deathState) Update(e *Enemy, dt time.Duration) StateID {
	if e.Current != nil && e.Current.Done() {
		e.Dead = true
	}
	return StateDeath
}
```

- [ ] **Step 4: Rewrite `internal/enemy/fsm_test.go`**

Replace the entire file with:

```go
package enemy

import (
	"math/rand"
	"testing"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
)

func stubAnim(id string, frames, durMs int, loop bool) *anim.Animation {
	spec := &anim.AnimationSpec{ID: id, FrameCount: frames, DurationMs: durMs, Loop: loop}
	return anim.NewAnimation(spec, nil)
}

func newOrcKind() *Kind {
	return &Kind{
		Name:       "orc",
		AnimPrefix: "orc",
		FrameW:     100,
		FrameH:     100,
		Tuning:     &Tuning{MaxLives: 2, RunSpeed: 80, IntentTickS: 2, HurtBounceVX: 120, HurtBounceVY: -180},
		Anims: map[string]*anim.Animation{
			"idle":    stubAnim("orc_idle", 6, 900, true),
			"run":     stubAnim("orc_run", 8, 700, true),
			"attack":  stubAnim("orc_attack", 6, 600, false),
			"attack2": stubAnim("orc_attack2", 6, 700, false),
			"hurt":    stubAnim("orc_hurt", 4, 400, false),
			"death":   stubAnim("orc_death", 4, 500, false),
		},
		Boxes: map[string]combat.Box{
			"body":    {OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1},
			"attack":  {OffsetX: 25, OffsetY: -70, W: 60, H: 60, FrameStart: 2, FrameEnd: 3},
			"attack2": {OffsetX: 25, OffsetY: -70, W: 70, H: 60, FrameStart: 3, FrameEnd: 4},
		},
		Motions: nil,
	}
}

func newTestEnemyOrc() *Enemy {
	ph := &player.Physics{Gravity: 1800, MaxFallSpeed: 900}
	return New(Config{
		StartX: 400, StartY: -100,
		Physics: ph,
		Kind:    newOrcKind(),
		RNG:     rand.New(rand.NewSource(1)),
	})
}

func TestEnemyStartsInFall(t *testing.T) {
	e := newTestEnemyOrc()
	if e.FSM.CurrentID() != StateFall {
		t.Errorf("want fall, got %q", e.FSM.CurrentID())
	}
}

func TestEnemyFallToRunOnGrounded(t *testing.T) {
	e := newTestEnemyOrc()
	e.Grounded = true
	e.FSM.Handle(e, 16*time.Millisecond)
	if e.FSM.CurrentID() != StateRun {
		t.Errorf("want run, got %q", e.FSM.CurrentID())
	}
}

func TestEnemyHurtOnDamage(t *testing.T) {
	e := newTestEnemyOrc()
	e.Grounded = true
	e.FSM.Handle(e, 16*time.Millisecond)
	e.OnHit(e.X + 10)
	if e.FSM.CurrentID() != StateHurt {
		t.Errorf("want hurt, got %q", e.FSM.CurrentID())
	}
	if e.VX >= 0 {
		t.Errorf("expected leftward bounce, got VX=%v", e.VX)
	}
}

func TestEnemyDiesOnFatalHit(t *testing.T) {
	e := newTestEnemyOrc()
	e.Grounded = true
	e.FSM.Handle(e, 16*time.Millisecond)
	e.Lives = 1
	e.OnHit(e.X + 10)
	if e.FSM.CurrentID() != StateDeath {
		t.Errorf("want death, got %q", e.FSM.CurrentID())
	}
}
```

- [ ] **Step 5: Update `internal/game/game.go` Deps + fields + init**

Replace the `Deps` struct:

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

Replace the `Game` struct fields block — drop `orcAnims`, `orcBoxes`, `orcTuning`; add `kinds []*enemy.Kind`:

```go
type Game struct {
	cfg          *config.Config
	world        *world.World
	player       *player.Player
	enemies      []*enemy.Enemy
	spawner      *spawner.Spawner
	overlay      *debug.Overlay
	hud          *hud.HUD
	gameOver     *hud.GameOver
	state        GameState
	hitboxDebug  bool
	lastIntent   input.Intent
	combatTuning *combat.Tuning
	kinds        []*enemy.Kind
	physics      *player.Physics
	rng          *rand.Rand
}
```

Rewrite `New` (replace existing body):

```go
func New(d Deps) *Game {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	w := world.New(d.Cfg, d.Physics.Gravity)

	p := player.New(player.Config{
		StartX:     float64(d.Cfg.WindowW) / 2,
		StartY:     w.GroundY,
		Physics:    d.Physics,
		Anims:      d.Anims,
		Boxes:      d.SoldierBoxes,
		StartLives: d.CombatTuning.SoldierMaxLives,
	})
	p.Grounded = true

	g := &Game{
		cfg:          d.Cfg,
		world:        w,
		player:       p,
		combatTuning: d.CombatTuning,
		kinds:        d.EnemyKinds,
		physics:      d.Physics,
		rng:          rng,
		state:        Playing,
	}
	g.overlay = debug.NewOverlay(d.DebugCfg, g)

	kindFactories := make([]spawner.KindFactory, 0, len(d.EnemyKinds))
	for _, k := range d.EnemyKinds {
		k := k
		halfW := float64(k.Boxes["body"].W) / 2
		spriteH := float64(k.FrameH * d.Cfg.RenderScale)
		kindFactories = append(kindFactories, spawner.KindFactory{
			Name:   k.Name,
			Weight: 1,
			NewEnemy: func(x, _ float64) *enemy.Enemy {
				if x < halfW {
					x = halfW
				}
				if maxX := float64(d.Cfg.WindowW) - halfW; x > maxX {
					x = maxX
				}
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

	g.hud = hud.NewHUD(d.HeartAnim, d.HUDFace, livesProvider{p}, d.Cfg.WindowW, 3)
	g.gameOver = hud.NewGameOver(d.OverTitle, d.OverSubtitle, d.Cfg.WindowW, d.Cfg.WindowH)

	return g
}
```

Rename method `OrcCount` → `EnemyCount`:

```go
func (g *Game) EnemyCount() int { return len(g.enemies) }
```

(Remove the old `OrcCount`.)

Update clamp loop in `Update`:

Find the `for _, e := range g.enemies {` block that computes `orcBodyHalfW` and update:

```go
for _, e := range g.enemies {
	bodyHalfW := float64(e.Kind.Boxes["body"].W) / 2
	leftLimit := bodyHalfW
	rightLimit := float64(g.cfg.WindowW) - bodyHalfW
	clamped := world.Clamp(e.X, leftLimit, rightLimit)
	if clamped != e.X && e.FSM.CurrentID() == enemy.StateRun {
		if e.X <= leftLimit {
			e.Facing = 1
		} else {
			e.Facing = -1
		}
	}
	e.X = clamped
}
```

Rewrite `drawEnemy`:

```go
func (g *Game) drawEnemy(screen *ebiten.Image, e *enemy.Enemy) {
	if e.Current == nil || e.Current.CurrentFrame() == nil {
		return
	}
	pad := e.Kind.Tuning.FootPadding
	fw := float64(e.Kind.FrameW)
	fh := float64(e.Kind.FrameH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-fw/2, -fh+float64(pad))
	if e.Facing < 0 {
		op.GeoM.Scale(-1, 1)
	}
	op.GeoM.Scale(float64(g.cfg.RenderScale), float64(g.cfg.RenderScale))
	op.GeoM.Translate(e.X, e.Y)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(e.Current.CurrentFrame(), op)
}
```

Update `drawHitboxes` — replace `e.Boxes["body"]` with `e.Kind.Boxes["body"]`:

```go
for _, e := range g.enemies {
	drawBox(e.X, e.Y, e.Facing, e.Kind.Boxes["body"], color.RGBA{0, 0xFF, 0, 0xFF})
	for _, h := range e.ActiveHits() {
		drawBox(e.X, e.Y, e.Facing, h, color.RGBA{0xFF, 0, 0, 0xFF})
	}
}
```

Update `reset()` — remove references to `g.player.Anims`/`g.player.Boxes` capture logic if any touches enemy kinds; leave player logic untouched (player still owns its own Anims/Boxes).

- [ ] **Step 6: Update `internal/debug/fields.go`**

Find `FieldSource` interface — rename method:

```go
type FieldSource interface {
	Player() *player.Player
	Intent() *input.Intent
	EngineFPS() float64
	EngineTPS() float64
	EnemyCount() int
	NextSpawnS() float64
}
```

Rename `Catalog` entry `"orc_count"` → `"enemy_count"`:

```go
"enemy_count":      {"enemy_count", func(s FieldSource) string { return fmt.Sprintf("Enemies: %d", s.EnemyCount()) }},
"orc_next_spawn_s": {"orc_next_spawn_s", func(s FieldSource) string { return fmt.Sprintf("NextSpawn: %.2fs", s.NextSpawnS()) }},
```

(Leave `orc_next_spawn_s` key alone; future task can rename if desired. Plan scope: only `orc_count` → `enemy_count`.)

Wait — spec §3 says `orc_next_spawn_s` untouched. Keep as is.

Actually: re-check `config/debug.json` — it doesn't currently reference either. Still safe to leave.

- [ ] **Step 7: Update `config/debug.json`**

Current file has no enemy-related fields. Leave as is, OR add an "Enemies" section:

```json
{
  "sections": [
    { "title": "State",      "fields": ["state", "facing", "grounded"] },
    { "title": "Kinematics", "fields": ["x", "y", "vx", "vy"] },
    { "title": "Animation",  "fields": ["anim_id", "anim_frame", "anim_elapsed_ms"] },
    { "title": "Intent",     "fields": ["intent_left", "intent_right", "intent_jump", "intent_sprint", "intent_attack", "intent_attack2"] },
    { "title": "Engine",     "fields": ["fps", "tps"] },
    { "title": "Enemies",    "fields": ["enemy_count", "orc_next_spawn_s"] }
  ]
}
```

- [ ] **Step 8: Update `cmd/game/main.go`**

Replace full `main` body:

```go
package main

import (
	"context"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/config"
	"claude-pixel/internal/debug"
	"claude-pixel/internal/enemy"
	"claude-pixel/internal/game"
	"claude-pixel/internal/hud"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	cfg := config.Load()

	db := storage.MustOpen(cfg)
	defer db.Close()

	animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
	hitboxRepo := storage.NewRepository[combat.HitboxSpec](db, combat.HitboxMapper{})
	motionRepo := storage.NewRepository[combat.AttackMotionSpec](db, combat.AttackMotionMapper{})

	anims, err := anim.LoadLibrary(cfg, animRepo)
	if err != nil {
		log.Fatalf("load animations: %v", err)
	}
	physics, err := player.LoadPhysics(tuneRepo)
	if err != nil {
		log.Fatalf("load physics: %v", err)
	}

	tuneParams, err := tuneRepo.List(context.Background())
	if err != nil {
		log.Fatalf("list tuning: %v", err)
	}
	tuneMap := make(map[string]float64, len(tuneParams))
	for _, p := range tuneParams {
		tuneMap[p.Key] = p.Value
	}
	combatTuning, err := combat.LoadTuning(tuneMap)
	if err != nil {
		log.Fatalf("load combat tuning: %v", err)
	}

	spawnTuning, err := enemy.LoadSpawnTuning(tuneRepo)
	if err != nil {
		log.Fatalf("load spawn tuning: %v", err)
	}

	hitboxSpecs, err := hitboxRepo.List(context.Background())
	if err != nil {
		log.Fatalf("list hitboxes: %v", err)
	}
	motionSpecs, err := motionRepo.List(context.Background())
	if err != nil {
		log.Fatalf("list attack_motions: %v", err)
	}

	soldierBoxes, err := combat.SoldierBoxes(hitboxSpecs, cfg.RenderScale)
	if err != nil {
		log.Fatalf("load soldier boxes: %v", err)
	}

	orcKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "orc", Prefix: "orc", FrameW: 100, FrameH: 100,
		AnimLib: anims, HitboxSpecs: hitboxSpecs, MotionSpecs: motionSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
	})
	if err != nil {
		log.Fatalf("build orc kind: %v", err)
	}

	slimeKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "slime", Prefix: "slime", FrameW: 96, FrameH: 96,
		AnimLib: anims, HitboxSpecs: hitboxSpecs, MotionSpecs: motionSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
	})
	if err != nil {
		log.Fatalf("build slime kind: %v", err)
	}

	heart, ok := anims["heart_beat"]
	if !ok {
		log.Fatalf("missing heart_beat anim")
	}
	dbgCfg, err := debug.LoadConfig(cfg.DebugConfigPath)
	if err != nil {
		log.Fatalf("load debug config: %v", err)
	}
	if err := hud.LoadFont(cfg.FontPath); err != nil {
		log.Fatalf("load font: %v", err)
	}

	g := game.New(game.Deps{
		Cfg:          cfg,
		Anims:        anims,
		Physics:      physics,
		DebugCfg:     dbgCfg,
		SoldierBoxes: soldierBoxes,
		CombatTuning: combatTuning,
		EnemyKinds:   []*enemy.Kind{orcKind, slimeKind},
		SpawnTuning:  spawnTuning,
		HeartAnim:    heart,
		HUDFace:      hud.NewFace(32),
		OverTitle:    hud.NewFace(96),
		OverSubtitle: hud.NewFace(32),
	})

	ebiten.SetWindowSize(cfg.WindowW, cfg.WindowH)
	ebiten.SetWindowTitle("claude-pixel")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
```

`BuildKind` does not exist yet — Step 9 adds it.

- [ ] **Step 9: Add `BuildKind` to `internal/enemy/kind.go`**

Append to `internal/enemy/kind.go`:

```go
import (
	// (keep existing imports)
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

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
	return &Kind{
		Name:       cfg.Name,
		AnimPrefix: cfg.Prefix,
		FrameW:     cfg.FrameW,
		FrameH:     cfg.FrameH,
		Tuning:     tuning,
		Boxes:      boxes,
		Anims:      anims,
		Motions:    motions,
	}, nil
}
```

Full imports block of `kind.go` after this edit:

```go
import (
	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)
```

- [ ] **Step 10: Remove deprecated `Tuning` fields + `LoadTuning` + `OrcAnims`/`OrcBoxes`**

Edit `internal/enemy/tuning.go` — remove `SpawnMinS`, `SpawnMaxS`, `MaxAlive` fields from `Tuning` and remove the `LoadTuning` wrapper function. Final struct:

```go
type Tuning struct {
	MaxLives     float64
	RunSpeed     float64
	IntentTickS  float64
	HurtBounceVX float64
	HurtBounceVY float64
	FootPadding  int
}
```

Edit `internal/enemy/loader.go` — remove `OrcAnims` and `OrcBoxes` functions entirely. Keep only `AnimsFor`, `BoxesFor`, `MotionsFor` (and the `kindAnimKeys` var).

- [ ] **Step 11: Rewrite `internal/spawner/spawner.go`**

The new `game.go` (Step 5) passes `Kinds []KindFactory` to `spawner.New`. Replace the package:

```go
package spawner

import (
	"math/rand"
	"time"

	"claude-pixel/internal/enemy"
)

type KindFactory struct {
	Name     string
	Weight   int
	NewEnemy func(x, y float64) *enemy.Enemy
}

type Spawner struct {
	MinIntervalS float64
	MaxIntervalS float64
	MaxAlive     int
	nextSpawn    float64
	spawnXMin    float64
	spawnXMax    float64
	rng          *rand.Rand
	kinds        []KindFactory
	totalWeight  int
}

type Config struct {
	MinIntervalS float64
	MaxIntervalS float64
	MaxAlive     int
	SpawnXMin    float64
	SpawnXMax    float64
	RNG          *rand.Rand
	Kinds        []KindFactory
}

func New(cfg Config) *Spawner {
	tw := 0
	for _, k := range cfg.Kinds {
		if k.Weight <= 0 {
			continue
		}
		tw += k.Weight
	}
	s := &Spawner{
		MinIntervalS: cfg.MinIntervalS,
		MaxIntervalS: cfg.MaxIntervalS,
		MaxAlive:     cfg.MaxAlive,
		spawnXMin:    cfg.SpawnXMin,
		spawnXMax:    cfg.SpawnXMax,
		rng:          cfg.RNG,
		kinds:        cfg.Kinds,
		totalWeight:  tw,
	}
	s.nextSpawn = s.rollInterval()
	return s
}

func (s *Spawner) NextSpawnS() float64 { return s.nextSpawn }

func (s *Spawner) Reset() {
	s.nextSpawn = s.rollInterval()
}

func (s *Spawner) Tick(dt time.Duration, alive int) *enemy.Enemy {
	s.nextSpawn -= dt.Seconds()
	if s.nextSpawn > 0 {
		return nil
	}
	s.nextSpawn = s.rollInterval()
	if alive >= s.MaxAlive {
		return nil
	}
	k := s.pickKind()
	if k.NewEnemy == nil {
		return nil
	}
	return k.NewEnemy(s.rollSpawnX(), 0)
}

func (s *Spawner) pickKind() KindFactory {
	switch len(s.kinds) {
	case 0:
		return KindFactory{}
	case 1:
		return s.kinds[0]
	}
	if s.totalWeight <= 0 {
		return s.kinds[0]
	}
	r := s.rng.Intn(s.totalWeight)
	for _, k := range s.kinds {
		if k.Weight <= 0 {
			continue
		}
		if r < k.Weight {
			return k
		}
		r -= k.Weight
	}
	return s.kinds[len(s.kinds)-1]
}

func (s *Spawner) rollInterval() float64 {
	return s.MinIntervalS + s.rng.Float64()*(s.MaxIntervalS-s.MinIntervalS)
}

func (s *Spawner) rollSpawnX() float64 {
	return s.spawnXMin + s.rng.Float64()*(s.spawnXMax-s.spawnXMin)
}
```

- [ ] **Step 12: Update `internal/spawner/spawner_test.go`**

Rewrite to use new `Kinds` shape:

```go
package spawner

import (
	"math/rand"
	"testing"
	"time"

	"claude-pixel/internal/enemy"
)

func fakeFactory(name string, calls *int) KindFactory {
	return KindFactory{
		Name:   name,
		Weight: 1,
		NewEnemy: func(x, y float64) *enemy.Enemy {
			*calls++
			return &enemy.Enemy{X: x, Y: y}
		},
	}
}

func TestSpawnerRespectsInterval(t *testing.T) {
	calls := 0
	s := New(Config{
		MinIntervalS: 2, MaxIntervalS: 2, MaxAlive: 5,
		SpawnXMin: 100, SpawnXMax: 200,
		RNG:   rand.New(rand.NewSource(1)),
		Kinds: []KindFactory{fakeFactory("test", &calls)},
	})
	if got := s.Tick(time.Second, 0); got != nil {
		t.Errorf("at t=1s, should not spawn yet")
	}
	if got := s.Tick(time.Second, 0); got == nil {
		t.Errorf("at t=2s, should spawn")
	}
	if calls != 1 {
		t.Errorf("want 1 factory call, got %d", calls)
	}
}

func TestSpawnerSkipsWhenAtCap(t *testing.T) {
	calls := 0
	s := New(Config{
		MinIntervalS: 1, MaxIntervalS: 1, MaxAlive: 2,
		SpawnXMin: 100, SpawnXMax: 200,
		RNG:   rand.New(rand.NewSource(1)),
		Kinds: []KindFactory{fakeFactory("test", &calls)},
	})
	if got := s.Tick(time.Second, 2); got != nil {
		t.Errorf("at cap=2 alive=2, should skip")
	}
	if calls != 0 {
		t.Errorf("want 0 factory calls, got %d", calls)
	}
	if got := s.Tick(time.Second, 2); got != nil {
		t.Errorf("still at cap, should skip")
	}
	if got := s.Tick(time.Second, 1); got == nil {
		t.Errorf("under cap, should spawn")
	}
	if calls != 1 {
		t.Errorf("want 1 factory call, got %d", calls)
	}
}

func TestSpawnerIntervalWithinRange(t *testing.T) {
	s := New(Config{
		MinIntervalS: 3, MaxIntervalS: 10, MaxAlive: 5,
		SpawnXMin: 100, SpawnXMax: 200,
		RNG: rand.New(rand.NewSource(42)),
		Kinds: []KindFactory{{Name: "test", Weight: 1,
			NewEnemy: func(x, y float64) *enemy.Enemy { return &enemy.Enemy{} }}},
	})
	for i := 0; i < 50; i++ {
		iv := s.rollInterval()
		if iv < 3 || iv > 10 {
			t.Fatalf("interval out of range: %v", iv)
		}
	}
}

func TestSpawnerPicksAmongKindsUniformly(t *testing.T) {
	orcCalls := 0
	slimeCalls := 0
	s := New(Config{
		MinIntervalS: 0, MaxIntervalS: 0, MaxAlive: 10000,
		SpawnXMin: 100, SpawnXMax: 200,
		RNG: rand.New(rand.NewSource(1)),
		Kinds: []KindFactory{
			fakeFactory("orc", &orcCalls),
			fakeFactory("slime", &slimeCalls),
		},
	})
	const N = 2000
	for i := 0; i < N; i++ {
		s.Tick(time.Second, 0)
	}
	if orcCalls+slimeCalls != N {
		t.Fatalf("expected %d total spawns, got %d orc + %d slime", N, orcCalls, slimeCalls)
	}
	ratio := float64(orcCalls) / float64(N)
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("ratio %f outside [0.4, 0.6] — distribution skewed", ratio)
	}
}
```

- [ ] **Step 13: Run build + all tests**

Run: `go build ./...`
Expected: success.

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 14: Commit**

```bash
git add internal/enemy/enemy.go internal/enemy/fighter.go internal/enemy/states.go \
       internal/enemy/fsm_test.go internal/enemy/kind.go internal/enemy/tuning.go \
       internal/enemy/loader.go \
       internal/spawner/spawner.go internal/spawner/spawner_test.go \
       internal/game/game.go internal/debug/fields.go config/debug.json \
       cmd/game/main.go
git commit -m "refactor(enemy): parameterize Enemy by Kind + multi-kind spawner"
```

---

## Task 12: Add motion hook tests (slime + orc regression)

**Files:**
- Modify: `internal/enemy/fsm_test.go`

- [ ] **Step 1: Add helper + slime kind builder**

Append to `internal/enemy/fsm_test.go`:

```go
func newSlimeKind() *Kind {
	return &Kind{
		Name:       "slime",
		AnimPrefix: "slime",
		FrameW:     96,
		FrameH:     96,
		Tuning:     &Tuning{MaxLives: 2, RunSpeed: 60, IntentTickS: 2, HurtBounceVX: 120, HurtBounceVY: -180},
		Anims: map[string]*anim.Animation{
			"idle":    stubAnim("slime_idle", 6, 900, true),
			"run":     stubAnim("slime_run", 8, 700, true),
			"attack":  stubAnim("slime_attack", 8, 650, false),
			"attack2": stubAnim("slime_attack2", 8, 700, false),
			"hurt":    stubAnim("slime_hurt", 4, 400, false),
			"death":   stubAnim("slime_death", 10, 800, false),
		},
		Boxes: map[string]combat.Box{
			"body":    {OffsetX: -20, OffsetY: -40, W: 40, H: 40, FrameStart: -1, FrameEnd: -1},
			"attack":  {OffsetX: 15, OffsetY: -35, W: 45, H: 35, FrameStart: 4, FrameEnd: 5},
			"attack2": {OffsetX: 15, OffsetY: -35, W: 55, H: 40, FrameStart: 3, FrameEnd: 5},
		},
		Motions: map[string]AttackMotion{
			"attack2": {VX: -60, FrameStart: 3, FrameEnd: 5},
		},
	}
}

func newTestEnemySlime() *Enemy {
	ph := &player.Physics{Gravity: 1800, MaxFallSpeed: 900}
	return New(Config{
		StartX: 400, StartY: -100,
		Physics: ph,
		Kind:    newSlimeKind(),
		RNG:     rand.New(rand.NewSource(1)),
	})
}

func TestSlimeAttack2BackstepAppliesVXOnActiveFrames(t *testing.T) {
	e := newTestEnemySlime()
	e.Facing = 1
	e.Grounded = true
	// force into attack2
	e.FSM.Transition(e, StateAttack2)
	if e.FSM.CurrentID() != StateAttack2 {
		t.Fatalf("failed to enter attack2, got %q", e.FSM.CurrentID())
	}

	// Attack2 anim: 8 frames over 700ms => ~87.5ms per frame.
	// Advance to frame 3 = 3 * 87.5ms = 262.5ms.
	e.Current.Update(263 * time.Millisecond)
	e.FSM.Handle(e, 16*time.Millisecond)
	if f := e.CurrentFrame(); f != 3 {
		t.Fatalf("expected frame 3, got %d", f)
	}
	wantVX := float64(e.Facing) * -60
	if e.VX != wantVX {
		t.Errorf("frame %d: want VX=%v, got %v", e.CurrentFrame(), wantVX, e.VX)
	}

	// Advance to frame 6 (past FrameEnd=5). 6 * 87.5ms = 525ms total.
	e.Current.Update(262 * time.Millisecond)
	e.FSM.Handle(e, 16*time.Millisecond)
	if f := e.CurrentFrame(); f != 6 {
		t.Fatalf("expected frame 6, got %d", f)
	}
	if e.VX != 0 {
		t.Errorf("frame %d (past window): want VX=0, got %v", e.CurrentFrame(), e.VX)
	}
}

func TestSlimeAttack2BackstepFacingLeftReversesDirection(t *testing.T) {
	e := newTestEnemySlime()
	e.Facing = -1
	e.Grounded = true
	e.FSM.Transition(e, StateAttack2)
	e.Current.Update(263 * time.Millisecond)
	e.FSM.Handle(e, 16*time.Millisecond)
	// facing=-1, motion.VX=-60, so e.VX = -1 * -60 = +60 (slime slides right while facing left = retreat)
	if e.VX != 60 {
		t.Errorf("want VX=60 (retreat while facing left), got %v", e.VX)
	}
}

func TestOrcAttack2NoMotionKeepsVXZero(t *testing.T) {
	e := newTestEnemyOrc()
	e.Facing = 1
	e.Grounded = true
	e.FSM.Transition(e, StateAttack2)
	e.Current.Update(300 * time.Millisecond)
	e.FSM.Handle(e, 16*time.Millisecond)
	if e.VX != 0 {
		t.Errorf("orc has no motion configured; VX should stay 0, got %v", e.VX)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/enemy/... -run "TestSlime|TestOrcAttack2" -v`
Expected: all PASS.

Run full suite: `go test ./...` — green.

- [ ] **Step 3: Commit**

```bash
git add internal/enemy/fsm_test.go
git commit -m "test(enemy): slime attack2 backstep motion + orc no-motion regression"
```

---

## Task 13: Tune CLI `motions` subcommand

**Files:**
- Modify: `cmd/tune/main.go`

- [ ] **Step 1: Update imports + wire new subcommand**

In `cmd/tune/main.go`, the imports already include `"claude-pixel/internal/combat"`. Add a new repo + register subcommand. Update the `main` function's subcommand block:

```go
func main() {
	cfg := config.Load()
	db := storage.MustOpen(cfg)
	defer db.Close()

	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
	hbRepo := storage.NewRepository[combat.HitboxSpec](db, combat.HitboxMapper{})
	mvRepo := storage.NewRepository[combat.AttackMotionSpec](db, combat.AttackMotionMapper{})

	app := &cli.Command{
		Name:  "claude-pixel-tune",
		Usage: "Manage tuning + hitbox + attack-motion rows stored in SQLite",
		Commands: []*cli.Command{
			tuningListCmd(tuneRepo),
			tuningSetCmd(tuneRepo),
			hitboxesCmd(hbRepo),
			motionsCmd(mvRepo),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Add `motionsCmd` + helpers**

Append to `cmd/tune/main.go`:

```go
func motionsCmd(repo *storage.Repository[combat.AttackMotionSpec]) *cli.Command {
	return &cli.Command{
		Name:  "motions",
		Usage: "CRUD operations on the attack_motions table",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List every attack-motion row",
				Action: func(ctx context.Context, c *cli.Command) error {
					rows, err := repo.List(ctx)
					if err != nil {
						return err
					}
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "ID\tOWNER\tKIND\tVX\tFRAME_START\tFRAME_END")
					for _, m := range rows {
						fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%d\t%d\n",
							m.ID, m.Owner, m.Kind, m.VX, m.FrameStart, m.FrameEnd)
					}
					return w.Flush()
				},
			},
			{
				Name:      "get",
				Usage:     "Show one attack-motion row by id",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 1 {
						return fmt.Errorf("usage: tune motions get <id>")
					}
					id := c.Args().Get(0)
					m, err := repo.Get(ctx, id)
					if err != nil {
						return fmt.Errorf("unknown motion id %q", id)
					}
					fmt.Printf("id=%s owner=%s kind=%s vx=%.2f frame_start=%d frame_end=%d\n",
						m.ID, m.Owner, m.Kind, m.VX, m.FrameStart, m.FrameEnd)
					return nil
				},
			},
			{
				Name:        "set",
				Usage:       "Update one field of an existing attack-motion row",
				ArgsUsage:   "<id> <field> <value>",
				Description: "Valid fields: owner, kind, vx, frame_start, frame_end",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 3 {
						return fmt.Errorf("usage: tune motions set <id> <field> <value>")
					}
					id := c.Args().Get(0)
					field := c.Args().Get(1)
					raw := c.Args().Get(2)

					m, err := repo.Get(ctx, id)
					if err != nil {
						return fmt.Errorf("unknown motion id %q", id)
					}
					before := formatMotion(m)
					if err := applyMotionField(&m, field, raw); err != nil {
						return err
					}
					if err := repo.Upsert(ctx, m); err != nil {
						return err
					}
					fmt.Printf("OK: %s.%s updated\n  was: %s\n  now: %s\n", id, field, before, formatMotion(m))
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "Insert (upsert) an attack-motion row",
				ArgsUsage: "<id> <owner> <kind> <vx> <frame_start> <frame_end>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 6 {
						return fmt.Errorf("usage: tune motions add <id> <owner> <kind> <vx> <fs> <fe>")
					}
					m := combat.AttackMotionSpec{
						ID:    c.Args().Get(0),
						Owner: c.Args().Get(1),
						Kind:  c.Args().Get(2),
					}
					vx, err := strconv.ParseFloat(c.Args().Get(3), 64)
					if err != nil {
						return fmt.Errorf("vx=%q is not a number", c.Args().Get(3))
					}
					m.VX = vx
					fs, err := strconv.Atoi(c.Args().Get(4))
					if err != nil {
						return fmt.Errorf("frame_start=%q is not an integer", c.Args().Get(4))
					}
					m.FrameStart = fs
					fe, err := strconv.Atoi(c.Args().Get(5))
					if err != nil {
						return fmt.Errorf("frame_end=%q is not an integer", c.Args().Get(5))
					}
					m.FrameEnd = fe
					if err := repo.Upsert(ctx, m); err != nil {
						return err
					}
					fmt.Printf("OK: added/updated %s\n", m.ID)
					return nil
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete an attack-motion row by id",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, c *cli.Command) error {
					if c.Args().Len() != 1 {
						return fmt.Errorf("usage: tune motions delete <id>")
					}
					id := c.Args().Get(0)
					if _, err := repo.Get(ctx, id); err != nil {
						return fmt.Errorf("unknown motion id %q", id)
					}
					if err := repo.Delete(ctx, id); err != nil {
						return err
					}
					fmt.Printf("OK: deleted %s\n", id)
					return nil
				},
			},
		},
	}
}

func formatMotion(m combat.AttackMotionSpec) string {
	return fmt.Sprintf("owner=%s kind=%s vx=%.2f frames=[%d,%d]",
		m.Owner, m.Kind, m.VX, m.FrameStart, m.FrameEnd)
}

func applyMotionField(m *combat.AttackMotionSpec, field, raw string) error {
	switch field {
	case "owner":
		m.Owner = raw
	case "kind":
		m.Kind = raw
	case "vx":
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("value %q is not a number", raw)
		}
		m.VX = v
	case "frame_start":
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("value %q is not an integer", raw)
		}
		m.FrameStart = n
	case "frame_end":
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("value %q is not an integer", raw)
		}
		m.FrameEnd = n
	default:
		return fmt.Errorf("unknown field %q (valid: owner, kind, vx, frame_start, frame_end)", field)
	}
	return nil
}
```

- [ ] **Step 3: Run build + test**

Run: `go build ./...`
Expected: success.

Run: `go test ./...` — all green.

- [ ] **Step 4: Manually verify CLI**

```bash
rm -rf data/
go run ./cmd/tune motions list
```

Expected output (tabwriter):
```
ID                   OWNER  KIND     VX      FRAME_START  FRAME_END
slime_attack2_motion slime attack2  -60.00  3            5
```

```bash
go run ./cmd/tune motions set slime_attack2_motion vx -90
```

Expected: `OK: slime_attack2_motion.vx updated` with before/now.

```bash
go run ./cmd/tune motions get slime_attack2_motion
```

Expected: shows `vx=-90.00`.

```bash
go run ./cmd/tune motions set slime_attack2_motion vx -60
```

Restore original value.

- [ ] **Step 5: Commit**

```bash
git add cmd/tune/main.go
git commit -m "feat(tune): motions CRUD subcommand for attack_motions table"
```

---

## Task 14: CLAUDE.md updates

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Read current CLAUDE.md**

Read the file. Target three sections for edits: the tuning parameter table, the FSM states section, the tune CLI section.

- [ ] **Step 2: Update tuning keys table**

Find the "Combat + enemy (11):" header. Replace the table to reflect renamed + new keys. The updated combat+enemy table should have 14 rows:

```markdown
Combat + enemy (14):
| Key | Unit | Effect |
|---|---|---|
| `soldier_max_lives` | — | Starting soldier lives (default 10) |
| `soldier_knockback_vx` | px/s | Horizontal bounce when hit |
| `soldier_knockback_vy` | px/s | Upward pop when hit (airborne i-frame) |
| `soldier_foot_padding` | px | Transparent px at soldier sprite frame bottom |
| `orc_max_lives` | — | Starting orc lives (default 2) |
| `orc_run_speed` | px/s | Orc ground speed |
| `orc_hurt_bounce_vx` | px/s | Horizontal bounce on hurt |
| `orc_hurt_bounce_vy` | px/s | Upward pop on hurt |
| `orc_intent_tick_s` | s | Interval for run-vs-attack reroll |
| `orc_foot_padding` | px | Transparent px at orc sprite frame bottom |
| `slime_max_lives` | — | Starting slime lives (default 2) |
| `slime_run_speed` | px/s | Slime ground speed |
| `slime_hurt_bounce_vx` | px/s | Horizontal bounce on hurt |
| `slime_hurt_bounce_vy` | px/s | Upward pop on hurt |
| `slime_intent_tick_s` | s | Interval for run-vs-attack reroll |
| `slime_foot_padding` | px | Transparent px at slime sprite frame bottom |
| `enemy_spawn_min_s` | s | Minimum enemy spawn interval (all kinds) |
| `enemy_spawn_max_s` | s | Maximum enemy spawn interval (all kinds) |
| `enemy_max_alive` | — | Concurrent enemy cap across all kinds (default 3) |
```

(Header line count adjusted to reflect actual row count — use whatever total is correct after the edit.)

Also update the opening line of the tuning section listing key counts: `Current keys (17):` → `Current keys (23):` (or actual count after edit).

- [ ] **Step 3: Update FSM states section**

Find "## State machines" section. Add a slime entry alongside Orc:

```markdown
**Slime** (6 states): identical FSM to orc — `Fall` → `Run` → `Attack`/`Attack2` → `Hurt`/`Death`. Attack2 applies a backward VX slide on frames 3–5 (configurable via `attack_motions` table → `slime_attack2_motion`).
```

- [ ] **Step 4: Update combat/hitbox section**

Add a paragraph under "## Combat + hitboxes":

```markdown
**Attack motions** (`attack_motions` table) optionally apply a per-frame-window VX slide during an attack state. VX is signed: positive = forward along facing, negative = backward. Seeded for slime Attack2 only (`slime_attack2_motion`: vx=-60, frames 3–5). Tune via `tune motions set <id> <field> <value>`. Orcs have no motion rows; their attacks keep VX=0.
```

- [ ] **Step 5: Update tune CLI section**

Under "## Tuning CLI", add a new subsection:

```markdown
### Motions (`attack_motions` table)

```bash
go run ./cmd/tune motions list
go run ./cmd/tune motions get <id>
go run ./cmd/tune motions set <id> <field> <value>   # fields: owner, kind, vx, frame_start, frame_end
go run ./cmd/tune motions add <id> <owner> <kind> <vx> <fs> <fe>
go run ./cmd/tune motions delete <id>
```

Mirrors the `hitboxes` subcommand shape. Used to retune slime backstep feel and to add motions for future enemy kinds.
```

- [ ] **Step 6: Update layout section**

In the `internal/enemy/` comment under `## Layout`, update description:

```
  enemy/               # Enemy + Kind (per-kind metadata), FSM (fall/run/attack/attack2/hurt/death), generic AnimsFor/BoxesFor/MotionsFor loaders, LoadTuningFor, LoadSpawnTuning
```

- [ ] **Step 7: Update controls section if needed**

No new controls. Skip.

- [ ] **Step 8: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(claude): slime enemy + attack motions + renamed enemy_* keys"
```

---

## Task 15: Manual smoke test (fresh DB + visual verify)

**Files:** None (verification task)

- [ ] **Step 1: Fresh DB + launch**

```bash
rm -rf data/
make run
```

Expected: game window opens with soldier in center, grey background.

- [ ] **Step 2: Verify orc regression (unchanged behavior)**

Wait for spawn (3–10s). Orcs should fall in, land, run, and occasionally attack. Kill them with J/X — 2 hits → death → removed.

If orcs behave differently from before this branch (e.g. wrong animation, wrong size, no attacks), stop and debug.

- [ ] **Step 3: Verify slime spawns**

Wait for more spawns. Should see a mix of orcs and slimes (roughly 50/50 over many spawns). Slime sprites are smaller (96×96 vs orc 100×100).

If slime never appears, check:
- `sqlite3 data/game.db "SELECT * FROM animations WHERE id LIKE 'slime_%';"` — 6 rows.
- `sqlite3 data/game.db "SELECT * FROM hitboxes WHERE owner='slime';"` — 3 rows.
- `sqlite3 data/game.db "SELECT * FROM tuning WHERE key LIKE 'slime_%';"` — 6 rows.

- [ ] **Step 4: Verify slime Attack2 backstep**

Let a slime attack. On Attack2, the slime should visibly slide backward (opposite of its facing) during mid-animation (frames 3–5 of the 8-frame anim). Before and after that window, it stays put.

If backstep not visible, check `sqlite3 data/game.db "SELECT * FROM attack_motions;"` — should return one row.

- [ ] **Step 5: Verify F4 hitbox debug**

Press F4. Green body box + red attack boxes should render on slime, same as orc.

If slime hitboxes look badly offset from the sprite, note the offset and adjust via:

```bash
go run ./cmd/tune hitboxes set slime_body offset_y -45   # example
go run ./cmd/tune hitboxes set slime_body height 50
make run
```

Iterate until body box wraps visible sprite.

- [ ] **Step 6: Verify foot padding**

If slime's feet float above or sink below the ground line compared to orc, adjust:

```bash
go run ./cmd/tune set slime_foot_padding 15   # try 15, 25, 30 to taste
make run
```

- [ ] **Step 7: Verify enemy cap**

Lower `enemy_spawn_min_s`/`max_s` to stress:

```bash
go run ./cmd/tune set enemy_spawn_min_s 1
go run ./cmd/tune set enemy_spawn_max_s 2
make run
```

Watch for concurrent count. Should never exceed `enemy_max_alive` (default 3). Restore values after:

```bash
go run ./cmd/tune set enemy_spawn_min_s 3
go run ./cmd/tune set enemy_spawn_max_s 10
```

- [ ] **Step 8: Verify F3 debug overlay**

Press F3. Under the "Enemies" section (added to `config/debug.json` in Task 11 Step 7), `Enemies: N` and `NextSpawn: X.XXs` should display and update.

- [ ] **Step 9: Verify tune list shows all keys**

```bash
go run ./cmd/tune list | grep -E "(slime_|enemy_)"
```

Expected: 6 slime keys + 3 enemy keys. No `orc_spawn_*` or `orc_max_alive`.

- [ ] **Step 10: Commit tuning adjustments (if any)**

If you adjusted hitbox dims or foot padding in steps 5–6, that only edits the DB, not code. To persist those adjustments as the baseline:

Option A: Update the seed migration (only acceptable if nobody has a db yet). Edit `018_seed_slime_hitboxes.sql` or `022_seed_slime_tuning.sql` with tuned values. BUT these are already applied migrations — editing breaks the "never edit applied migration" rule.

Option B: Add a new migration `023_retune_slime.sql` with `UPDATE` statements:

```sql
-- 023_retune_slime.sql
UPDATE tuning SET value = <new-pad> WHERE key = 'slime_foot_padding';
UPDATE hitboxes SET offset_x = <new-x>, offset_y = <new-y>, width = <w>, height = <h> WHERE id = 'slime_body';
-- etc.
```

If no adjustments made, skip this step. If made, commit as:

```bash
git add internal/storage/migrations/023_retune_slime.sql
git commit -m "fix(storage): retune slime hitboxes + foot padding from visual check"
```

---

## Self-Review Checklist (run after completing all tasks)

- [ ] All 6 migrations (017–022) applied in order, schema + data visible in DB.
- [ ] `AttackMotionSpec` round-trips through repository.
- [ ] `enemy.Kind` + `enemy.AttackMotion` defined and used by `Enemy`.
- [ ] FSM states use unprefixed anim IDs (`"idle"`, `"run"`, ...).
- [ ] `applyMotion` triggers on configured frame window; orc (no motion) untouched.
- [ ] `Enemy.Kind.Boxes["body"]` used in game clamp + hitbox debug draw.
- [ ] `drawEnemy` reads `FrameW`/`FrameH`/`FootPadding` from `Kind.Tuning`.
- [ ] Spawner picks uniformly across two kinds; cap still 3.
- [ ] `main.go` builds both kinds via `BuildKind`; wires `EnemyKinds` + `SpawnTuning`.
- [ ] `game.Deps` no longer mentions Orc-specific fields.
- [ ] `debug.FieldSource.EnemyCount()` replaces `OrcCount()`; catalog entry is `enemy_count`.
- [ ] `config/debug.json` has an "Enemies" section referencing `enemy_count`.
- [ ] `tune motions list/get/set/add/delete` works end-to-end against seeded `slime_attack2_motion` row.
- [ ] `CLAUDE.md` documents slime, attack motions, renamed spawn keys, new tune subcommand.
- [ ] `go build ./...` clean; `go test ./...` all green.
- [ ] Manual smoke: orcs + slimes both spawn; slime attack2 slides backward; cap enforced; F3/F4 work.

---

## Notes for the executor

- **File count**: 15 modifications + 8 new files (6 migrations + combat/motion.go + combat/motion_test.go + enemy/kind.go + enemy/loader_test.go).
- **Tricky atomic commit**: Task 11 bundles 8 files. Build will fail at intermediate save points — only `go build ./...` at the end must pass. Stage all files before building.
- **Test determinism**: `TestSpawnerPicksAmongKindsUniformly` uses seed 1 + N=2000; tuned to give ~50/50. If seed drift changes the ratio, widen the bounds (but check the pickKind code first — RNG change usually means a bug).
- **Slime visuals unknown**: hitbox dims + foot padding are educated guesses. Task 15 is explicit about visual iteration. Don't claim success until both orc and slime visually land correctly on the ground line and their hitboxes track their bodies.
- **DO NOT** edit already-applied migrations. New tuning lives in a new migration file.
