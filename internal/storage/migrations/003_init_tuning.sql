CREATE TABLE tuning (
    key         TEXT    PRIMARY KEY,
    value       REAL    NOT NULL,
    min_value   REAL    NOT NULL,
    max_value   REAL    NOT NULL,
    unit        TEXT    NOT NULL DEFAULT '',
    description TEXT    NOT NULL
);
