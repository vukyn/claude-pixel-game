-- 028_seed_stamina_tuning.sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('stamina_max',          100, 10,  500, '',   'max stamina pool'),
    ('stamina_drain_per_s',   20,  1,  500, '/s', 'stamina drain rate while sprinting'),
    ('stamina_regen_per_s',   20,  1,  500, '/s', 'stamina regen rate while not sprinting');
