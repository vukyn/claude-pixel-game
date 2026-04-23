-- 022_seed_slime_tuning.sql
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('slime_max_lives',         2,    1,  10,  '',     'starting slime lives'),
    ('slime_run_speed',        60,    0, 500, 'px/s',  'slime ground speed'),
    ('slime_intent_tick_s',     2,  0.5,  10, 's',     'slime intent reroll period'),
    ('slime_hurt_bounce_vx',  120,    0, 500, 'px/s',  'slime hurt horizontal bounce'),
    ('slime_hurt_bounce_vy', -180, -500,   0, 'px/s',  'slime hurt vertical pop'),
    ('slime_foot_padding',     39,    0,  96, 'px',    'transparent px at slime sprite frame bottom');
