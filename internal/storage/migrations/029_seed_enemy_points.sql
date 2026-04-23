-- 029_seed_enemy_points.sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('orc_points',   10, 0, 1000, '', 'points awarded when orc killed'),
    ('slime_points', 15, 0, 1000, '', 'points awarded when slime killed');
