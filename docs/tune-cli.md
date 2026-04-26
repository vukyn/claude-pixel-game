# Tune CLI Reference

Manage SQLite-backed tuning rows (`tuning`, `hitboxes`, `hud_layout`) without writing SQL.

Source: [cmd/tune/main.go](../cmd/tune/main.go). Schema seed: [internal/storage/migrations/](../internal/storage/migrations/).

## Run

```bash
go run ./cmd/tune <subcommand> [args...]
# or
make tune ARGS="<subcommand> [args...]"
```

Changes commit immediately to `data/game.db`. No hot reload — restart the game with `make run` to apply.

## Global flag: `--agent-mode`

Compact, machine-friendly output. Drops headers, drops tabwriter padding, drops prose decoration. All numeric/structural data preserved. Errors unchanged (already exact).

Enable two ways:

```bash
go run ./cmd/tune --agent-mode list
TUNE_AGENT_MODE=1 go run ./cmd/tune list
```

Output is plain TSV (one record per line). Suited for AI agents, scripts, `cut`, `awk`.

| Mode | Output style |
|---|---|
| default | Tabwriter padded, header row, human prose (`OK: key = X (was Y)`) |
| `--agent-mode` | TSV, no header, single-line records (`OK\tkey\tnew\tprev\tunit`) |

---

## Commands

### `list` — list all tunable parameters

```bash
go run ./cmd/tune list
go run ./cmd/tune --agent-mode list
```

Columns: `KEY VALUE MIN MAX UNIT DESCRIPTION`. Authoritative key list lives in [CLAUDE.md](../CLAUDE.md) under "Tuning CLI".

Agent-mode line:
```
run_speed	280.00	50.00	1000.00	px/s	Horizontal ground movement speed
```

### `set <key> <value>` — update one tuning value

```bash
go run ./cmd/tune set run_speed 320
go run ./cmd/tune --agent-mode set run_speed 320
```

Validates against the row's `min_value`/`max_value`. Rejects unknown keys.

| Mode | Success line |
|---|---|
| default | `OK: run_speed = 320.0000 px/s (was 280.0000)` |
| agent | `OK\trun_speed\t320.0000\t280.0000\tpx/s` (cols: status, key, new, prev, unit) |

Errors (both modes, exit 1):
- `unknown tuning key "..."`
- `value "..." is not a number: ...`
- `value out of range: X not in [min, max] unit`

### `hitboxes list` — list every hitbox row

```bash
go run ./cmd/tune hitboxes list
go run ./cmd/tune --agent-mode hitboxes list
```

Columns: `ID OWNER KIND OFFSET_X OFFSET_Y WIDTH HEIGHT FRAME_START FRAME_END`. `frame_start=-1, frame_end=-1` = always active (body box).

### `hitboxes get <id>` — show one hitbox

```bash
go run ./cmd/tune hitboxes get soldier_attack
go run ./cmd/tune --agent-mode hitboxes get soldier_attack
```

| Mode | Output |
|---|---|
| default | `id=soldier_attack owner=soldier kind=attack offset_x=15 ...` |
| agent | `soldier_attack\tsoldier\tattack\t15\t-40\t35\t35\t1\t2` |

### `hitboxes set <id> <field> <value>` — patch one hitbox field

```bash
go run ./cmd/tune hitboxes set soldier_attack width 40
go run ./cmd/tune --agent-mode hitboxes set soldier_attack width 40
```

Valid fields: `owner`, `kind`, `offset_x`, `offset_y`, `width`, `height`, `active_frame_start` (alias `frame_start`), `active_frame_end` (alias `frame_end`).

| Mode | Success line |
|---|---|
| default | Two lines: `OK: ID.field updated`, then `was: ...` / `now: ...` |
| agent | `OK\tID\tfield\tnew_value` |

### `hitboxes add <id> <owner> <kind> <off_x> <off_y> <w> <h> <fs> <fe>` — upsert a hitbox

```bash
go run ./cmd/tune hitboxes add orc_body orc body -15 -32 30 32 -1 -1
go run ./cmd/tune --agent-mode hitboxes add orc_body orc body -15 -32 30 32 -1 -1
```

| Mode | Success line |
|---|---|
| default | `OK: added/updated <id>` |
| agent | `OK\t<id>\t<owner>\t<kind>\t<off_x>\t<off_y>\t<w>\t<h>\t<fs>\t<fe>` |

### `hitboxes delete <id>` — remove a hitbox row

```bash
go run ./cmd/tune hitboxes delete orc_body
go run ./cmd/tune --agent-mode hitboxes delete orc_body
```

| Mode | Success line |
|---|---|
| default | `OK: deleted <id>` |
| agent | `OK\tdeleted\t<id>` |

### `hud list` / `hud get <key>` / `hud set <key> <field> <value>`

Manage `hud_layout` rows.

```bash
go run ./cmd/tune hud list
go run ./cmd/tune --agent-mode hud list

go run ./cmd/tune hud get heart
go run ./cmd/tune --agent-mode hud get heart

go run ./cmd/tune hud set heart x 80
go run ./cmd/tune --agent-mode hud set heart x 80
```

Keys: `heart`, `lives_text`, `score_text`, `stamina_bar`.

Fields: `x`, `y`, `w`, `h`, `anchor`, `scale`.
- `anchor` ∈ {`top_left`, `top_right`, `bottom_left`, `bottom_right`}.
- `x`/`y` = offset from element's nearest corner to the screen anchor corner.
- Text rows store `w=h=0` → measured at draw time.
- `scale` must be > 0.

Output columns (list/get): `KEY X Y W H ANCHOR SCALE`.

Set success:
| Mode | Line |
|---|---|
| default | `OK: <key>.<field> updated` + was/now block |
| agent | `OK\t<key>\t<field>\t<value>` |

---

## Quick reference — agent-mode output schema

| Command | Per-line columns (TSV) |
|---|---|
| `list` | `key value min max unit description` |
| `set` | `OK key new prev unit` |
| `hitboxes list` / `get` | `id owner kind off_x off_y w h fstart fend` |
| `hitboxes set` | `OK id field value` |
| `hitboxes add` | `OK id owner kind off_x off_y w h fstart fend` |
| `hitboxes delete` | `OK deleted id` |
| `hud list` / `get` | `key x y w h anchor scale` |
| `hud set` | `OK key field value` |

Errors print to stderr verbatim (same in both modes), process exits 1.
