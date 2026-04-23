-- 017_seed_slime_animations.sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path, is_player, is_enemy)
VALUES
    ('slime_idle',    'Idle.png',     6, 900, 1, 96, 96, 'slime/Idle.png',    0, 1),
    ('slime_run',     'Run.png',      8, 700, 1, 96, 96, 'slime/Run.png',     0, 1),
    ('slime_attack',  'Attack.png',   8, 650, 0, 96, 96, 'slime/Attack.png',  0, 1),
    ('slime_attack2', 'Attack2.png',  8, 700, 0, 96, 96, 'slime/Attack2.png', 0, 1),
    ('slime_hurt',    'Hurt.png',     4, 400, 0, 96, 96, 'slime/Hurt.png',    0, 1),
    ('slime_death',   'Death.png',   10, 800, 0, 96, 96, 'slime/Death.png',   0, 1);
