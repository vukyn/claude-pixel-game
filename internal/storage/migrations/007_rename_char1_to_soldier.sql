-- 007_rename_char1_to_soldier.sql
UPDATE animations SET id = 'soldier_idle'    WHERE id = 'idle';
UPDATE animations SET id = 'soldier_run'     WHERE id = 'run';
UPDATE animations SET id = 'soldier_jump'    WHERE id = 'jump';
UPDATE animations SET id = 'soldier_fall'    WHERE id = 'fall';
UPDATE animations SET id = 'soldier_dash'    WHERE id = 'dash';
UPDATE animations SET id = 'soldier_attack'  WHERE id = 'attack';
UPDATE animations SET id = 'soldier_attack2' WHERE id = 'attack2';
