CREATE TABLE hud_layout (
    key     TEXT    PRIMARY KEY,
    x       INTEGER NOT NULL,
    y       INTEGER NOT NULL,
    w       INTEGER NOT NULL,
    h       INTEGER NOT NULL,
    anchor  TEXT    NOT NULL CHECK(anchor IN ('top_left','top_right','bottom_left','bottom_right')),
    scale   REAL    NOT NULL DEFAULT 1.0
);
