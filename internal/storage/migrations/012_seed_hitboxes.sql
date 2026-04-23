-- 012_seed_hitboxes.sql
INSERT OR IGNORE INTO hitboxes
    (id, owner, kind, offset_x, offset_y, width, height, active_frame_start, active_frame_end)
VALUES
    ('soldier_body',    'soldier', 'body',    -20, -70, 40, 70, -1, -1),
    ('soldier_attack',  'soldier', 'attack',   20, -60, 60, 50,  1,  2),
    ('soldier_attack2', 'soldier', 'attack2',  20, -60, 80, 60,  2,  4),
    ('orc_body',        'orc',     'body',    -25, -80, 50, 80, -1, -1),
    ('orc_attack',      'orc',     'attack',   25, -70, 60, 60,  2,  3),
    ('orc_attack2',     'orc',     'attack2',  25, -70, 70, 60,  3,  4);
