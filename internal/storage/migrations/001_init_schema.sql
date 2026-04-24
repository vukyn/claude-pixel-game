-- 001_init_schema.sql
-- All table definitions.

CREATE TABLE animations (
    id           TEXT    PRIMARY KEY,
    file         TEXT    NOT NULL,
    frame_count  INTEGER NOT NULL,
    duration_ms  INTEGER NOT NULL,
    loop         INTEGER NOT NULL,
    frame_w      INTEGER NOT NULL DEFAULT 120,
    frame_h      INTEGER NOT NULL DEFAULT 80,
    path         TEXT    NOT NULL DEFAULT '',
    is_player    INTEGER NOT NULL DEFAULT 0,
    is_enemy     INTEGER NOT NULL DEFAULT 0,
    grid_cols    INTEGER NOT NULL DEFAULT 0,
    grid_rows    INTEGER NOT NULL DEFAULT 0,
    pick_row     INTEGER NOT NULL DEFAULT 0,
    pick_col     INTEGER NOT NULL DEFAULT -1
);

CREATE TABLE tuning (
    key         TEXT    PRIMARY KEY,
    value       REAL    NOT NULL,
    min_value   REAL    NOT NULL,
    max_value   REAL    NOT NULL,
    unit        TEXT    NOT NULL DEFAULT '',
    description TEXT    NOT NULL
);

CREATE TABLE hitboxes (
    id                 TEXT    PRIMARY KEY,
    owner              TEXT    NOT NULL,
    kind               TEXT    NOT NULL,
    offset_x           INTEGER NOT NULL,
    offset_y           INTEGER NOT NULL,
    width              INTEGER NOT NULL,
    height             INTEGER NOT NULL,
    active_frame_start INTEGER NOT NULL DEFAULT -1,
    active_frame_end   INTEGER NOT NULL DEFAULT -1
);

CREATE TABLE attack_motions (
    id          TEXT    PRIMARY KEY,
    owner       TEXT    NOT NULL,
    kind        TEXT    NOT NULL,
    vx          INTEGER NOT NULL,
    frame_start INTEGER NOT NULL,
    frame_end   INTEGER NOT NULL
);

CREATE TABLE hud_layout (
    key    TEXT    PRIMARY KEY,
    x      INTEGER NOT NULL,
    y      INTEGER NOT NULL,
    w      INTEGER NOT NULL,
    h      INTEGER NOT NULL,
    anchor TEXT    NOT NULL CHECK(anchor IN ('top_left','top_right','bottom_left','bottom_right')),
    scale  REAL    NOT NULL DEFAULT 1.0
);
