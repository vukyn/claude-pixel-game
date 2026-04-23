-- 021_rename_spawn_keys.sql
-- Move orc_spawn_*/orc_max_alive to global enemy_* scope.
INSERT INTO tuning (key, value, min_value, max_value, unit, description)
SELECT 'enemy_spawn_min_s', value, min_value, max_value, unit, 'minimum enemy spawn interval (all kinds)'
FROM tuning WHERE key='orc_spawn_min_s';

INSERT INTO tuning (key, value, min_value, max_value, unit, description)
SELECT 'enemy_spawn_max_s', value, min_value, max_value, unit, 'maximum enemy spawn interval (all kinds)'
FROM tuning WHERE key='orc_spawn_max_s';

INSERT INTO tuning (key, value, min_value, max_value, unit, description)
SELECT 'enemy_max_alive', value, min_value, max_value, unit, 'max concurrent enemies (all kinds)'
FROM tuning WHERE key='orc_max_alive';

DELETE FROM tuning WHERE key IN ('orc_spawn_min_s','orc_spawn_max_s','orc_max_alive');
