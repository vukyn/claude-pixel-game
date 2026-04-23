-- 018_seed_slime_hitboxes.sql
INSERT OR IGNORE INTO hitboxes
    (id, owner, kind, offset_x, offset_y, width, height, active_frame_start, active_frame_end)
VALUES
    ('slime_body',    'slime', 'body',    -8, -11, 14, 11, -1, -1),
    ('slime_attack',  'slime', 'attack',   12, -15, 15, 15,  4,  5),
    ('slime_attack2', 'slime', 'attack2',  15, -15, 15, 15,  3,  5);
