-- 014_retune_hitboxes.sql
-- Shrink hitbox dims to match visible character region inside sprite frames
-- (after 3x render scale). Original values in migration 012 were calibrated
-- for full-frame coverage; visible characters occupy ~50-70% of each frame
-- so scaled boxes appeared oversized in F4 debug draw.

UPDATE hitboxes SET offset_x=-15, offset_y=-60, width=30, height=60 WHERE id='soldier_body';
UPDATE hitboxes SET offset_x= 15, offset_y=-50, width=35, height=35 WHERE id='soldier_attack';
UPDATE hitboxes SET offset_x= 15, offset_y=-50, width=50, height=40 WHERE id='soldier_attack2';
UPDATE hitboxes SET offset_x=-12, offset_y=-50, width=25, height=50 WHERE id='orc_body';
UPDATE hitboxes SET offset_x= 15, offset_y=-45, width=40, height=40 WHERE id='orc_attack';
UPDATE hitboxes SET offset_x= 15, offset_y=-45, width=50, height=40 WHERE id='orc_attack2';
