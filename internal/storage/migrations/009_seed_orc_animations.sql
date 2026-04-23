-- 009_seed_orc_animations.sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path, is_player, is_enemy)
VALUES
    ('orc_idle',    'Idle.png',    6, 900, 1, 100, 100, 'orc/Idle.png',    0, 1),
    ('orc_run',     'Run.png',     8, 700, 1, 100, 100, 'orc/Run.png',     0, 1),
    ('orc_attack',  'Attack.png',  6, 600, 0, 100, 100, 'orc/Attack.png',  0, 1),
    ('orc_attack2', 'Attack2.png', 6, 700, 0, 100, 100, 'orc/Attack2.png', 0, 1),
    ('orc_hurt',    'Hurt.png',    4, 400, 0, 100, 100, 'orc/Hurt.png',    0, 1),
    ('orc_death',   'Death.png',   4, 500, 0, 100, 100, 'orc/Death.png',   0, 1);
