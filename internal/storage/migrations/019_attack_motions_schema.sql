-- 019_attack_motions_schema.sql
CREATE TABLE attack_motions (
    id                 TEXT    PRIMARY KEY,
    owner              TEXT    NOT NULL,
    kind               TEXT    NOT NULL,
    vx                 INTEGER NOT NULL,
    frame_start        INTEGER NOT NULL,
    frame_end          INTEGER NOT NULL
);
