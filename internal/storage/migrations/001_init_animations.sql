CREATE TABLE animations (
    id           TEXT    PRIMARY KEY,
    file         TEXT    NOT NULL,
    frame_count  INTEGER NOT NULL,
    duration_ms  INTEGER NOT NULL,
    loop         INTEGER NOT NULL
);
