-- 014_retune_hitboxes.sql
-- Shrink hitbox dims to match visible character region inside sprite frames
-- (after 3x render scale). Original values in migration 012 were calibrated
-- for full-frame coverage; visible characters occupy ~50-70% of each frame
-- so scaled boxes appeared oversized in F4 debug draw.

UPDATE hitboxes SET offset_x=-15, offset_y=-40, width=20, height=40 WHERE id='soldier_body';
UPDATE hitboxes SET offset_x= 15, offset_y=-40, width=35, height=35 WHERE id='soldier_attack';
UPDATE hitboxes SET offset_x= 12, offset_y=-40, width=35, height=35 WHERE id='soldier_attack2';
UPDATE hitboxes SET offset_x=-8, offset_y=-15, width=15, height=15 WHERE id='orc_body';
UPDATE hitboxes SET offset_x= 12, offset_y=-18, width=15, height=15 WHERE id='orc_attack';
UPDATE hitboxes SET offset_x= 12, offset_y=-18, width=15, height=15 WHERE id='orc_attack2';
