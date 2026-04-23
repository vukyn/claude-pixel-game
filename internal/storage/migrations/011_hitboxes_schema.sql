-- 011_hitboxes_schema.sql
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
