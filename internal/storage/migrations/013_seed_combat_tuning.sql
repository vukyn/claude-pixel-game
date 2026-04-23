-- 013_seed_combat_tuning.sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('orc_hurt_bounce_vx',    120,    0, 500, 'px/s', 'horizontal bounce away from attacker when orc is hurt'),
    ('orc_hurt_bounce_vy',   -180, -500,   0, 'px/s', 'vertical pop applied on orc hurt'),
    ('soldier_knockback_vx',  200,    0, 500, 'px/s', 'horizontal knockback away when soldier is hit'),
    ('soldier_knockback_vy', -300, -600,   0, 'px/s', 'upward pop when soldier is hit (airborne i-frame)'),
    ('soldier_max_lives',      10,    1,  99, '',     'starting soldier lives'),
    ('orc_max_lives',           2,    1,  10, '',     'starting orc lives'),
    ('orc_spawn_min_s',         3,    1,  60, 's',    'minimum orc spawn interval'),
    ('orc_spawn_max_s',        10,    1,  60, 's',    'maximum orc spawn interval'),
    ('orc_max_alive',           3,    1,  10, '',     'max concurrent orcs'),
    ('orc_intent_tick_s',       2,  0.5,  10, 's',    'orc intent reroll period'),
    ('orc_run_speed',          80,    0, 500, 'px/s', 'orc ground speed');
