-- 024_animations_add_pick_col.sql
ALTER TABLE animations ADD COLUMN pick_col INTEGER NOT NULL DEFAULT -1;
