-- 008_animations_schema_v2.sql
ALTER TABLE animations ADD COLUMN frame_w   INTEGER NOT NULL DEFAULT 120;
ALTER TABLE animations ADD COLUMN frame_h   INTEGER NOT NULL DEFAULT 80;
ALTER TABLE animations ADD COLUMN path      TEXT    NOT NULL DEFAULT '';
ALTER TABLE animations ADD COLUMN is_player INTEGER NOT NULL DEFAULT 0;
ALTER TABLE animations ADD COLUMN is_enemy  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE animations ADD COLUMN grid_cols INTEGER NOT NULL DEFAULT 0;
ALTER TABLE animations ADD COLUMN grid_rows INTEGER NOT NULL DEFAULT 0;
ALTER TABLE animations ADD COLUMN pick_row  INTEGER NOT NULL DEFAULT 0;

UPDATE animations
   SET path      = 'soldier/' || file,
       is_player = 1,
       frame_w   = 120,
       frame_h   = 80
 WHERE id LIKE 'soldier_%';
