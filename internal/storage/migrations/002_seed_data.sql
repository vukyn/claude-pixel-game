-- 002_seed_data.sql
-- Final state of all seed data. This is a separate migration from the schema definitions to allow easier iteration on seed data without modifying the schema migration history.

-- ============================================================================
-- animations
-- ============================================================================

INSERT INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path,
     is_player, is_enemy, grid_cols, grid_rows, pick_row, pick_col)
VALUES
    -- soldier (player)
    ('soldier_idle',    '_Idle.png',    10, 1000, 1, 120, 80, 'sprites/soldier/_Idle.png',    1, 0, 0, 0, 0, -1),
    ('soldier_run',     '_Run.png',     10, 1000, 1, 120, 80, 'sprites/soldier/_Run.png',     1, 0, 0, 0, 0, -1),
    ('soldier_jump',    '_Jump.png',     3,  500, 0, 120, 80, 'sprites/soldier/_Jump.png',    1, 0, 0, 0, 0, -1),
    ('soldier_fall',    '_Fall.png',     3,  500, 0, 120, 80, 'sprites/soldier/_Fall.png',    1, 0, 0, 0, 0, -1),
    ('soldier_dash',    '_Dash.png',     2,  500, 0, 120, 80, 'sprites/soldier/_Dash.png',    1, 0, 0, 0, 0, -1),
    ('soldier_attack',  '_Attack.png',   4,  500, 0, 120, 80, 'sprites/soldier/_Attack.png',  1, 0, 0, 0, 0, -1),
    ('soldier_attack2', '_Attack2.png',  6,  750, 0, 120, 80, 'sprites/soldier/_Attack2.png', 1, 0, 0, 0, 0, -1),
    ('soldier_hit',     '_Hit.png',      1,  200, 0, 120, 80, 'sprites/soldier/_Hit.png',     1, 0, 0, 0, 0, -1),
    ('soldier_death',   '_Death.png',   10, 1000, 0, 120, 80, 'sprites/soldier/_Death.png',   1, 0, 0, 0, 0, -1),

    -- orc (enemy)
    ('orc_idle',    'Idle.png',    6, 900, 1, 100, 100, 'sprites/orc/Idle.png',    0, 1, 0, 0, 0, -1),
    ('orc_run',     'Run.png',     8, 700, 1, 100, 100, 'sprites/orc/Run.png',     0, 1, 0, 0, 0, -1),
    ('orc_attack',  'Attack.png',  6, 600, 0, 100, 100, 'sprites/orc/Attack.png',  0, 1, 0, 0, 0, -1),
    ('orc_attack2', 'Attack2.png', 6, 700, 0, 100, 100, 'sprites/orc/Attack2.png', 0, 1, 0, 0, 0, -1),
    ('orc_hurt',    'Hurt.png',    4, 400, 0, 100, 100, 'sprites/orc/Hurt.png',    0, 1, 0, 0, 0, -1),
    ('orc_death',   'Death.png',   4, 500, 0, 100, 100, 'sprites/orc/Death.png',   0, 1, 0, 0, 0, -1),

    -- slime (enemy)
    ('slime_idle',    'Idle.png',     6, 900, 1, 96, 96, 'sprites/slime/Idle.png',    0, 1, 0, 0, 0, -1),
    ('slime_run',     'Run.png',      8, 700, 1, 96, 96, 'sprites/slime/Run.png',     0, 1, 0, 0, 0, -1),
    ('slime_attack',  'Attack.png',   8, 650, 0, 96, 96, 'sprites/slime/Attack.png',  0, 1, 0, 0, 0, -1),
    ('slime_attack2', 'Attack2.png',  8, 700, 0, 96, 96, 'sprites/slime/Attack2.png', 0, 1, 0, 0, 0, -1),
    ('slime_hurt',    'Hurt.png',     4, 400, 0, 96, 96, 'sprites/slime/Hurt.png',    0, 1, 0, 0, 0, -1),
    ('slime_death',   'Death.png',   10, 800, 0, 96, 96, 'sprites/slime/Death.png',   0, 1, 0, 0, 0, -1),

    -- HUD (paths NOT prefixed with sprites/ — live under assets/huds/)
    ('heart_beat',  'heartbeat.png', 4, 400, 1, 16, 16, 'huds/healthbar/heartbeat.png', 0, 0, 4,  6, 3, -1),
    ('stamina_bar', 'healthbar.png', 10,  0, 0, 48, 16, 'huds/healthbar/healthbar.png', 0, 0, 4, 10, 0,  2);

-- ============================================================================
-- tuning
-- ============================================================================

INSERT INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    -- physics
    ('run_speed',        280,     50,   1000, 'px/s',   'Horizontal ground movement speed'),
    ('sprint_speed',     420,    100,   2000, 'px/s',   'Horizontal ground movement speed while Shift held'),
    ('air_control',      0.8,      0,      1, '',       'Horizontal movement multiplier while airborne'),
    ('jump_velocity',   -650,  -2000,   -100, 'px/s',   'Jump impulse applied on takeoff (negative = upward)'),
    ('gravity',         2000,    100,   5000, 'px/s^2', 'Downward acceleration applied each tick'),
    ('max_fall_speed',   900,    100,   3000, 'px/s',   'Terminal vertical velocity clamp'),

    -- stamina
    ('stamina_max',          100, 10,  500, '',   'max stamina pool'),
    ('stamina_drain_per_s',   20,  1,  500, '/s', 'stamina drain rate while sprinting'),
    ('stamina_regen_per_s',   20,  1,  500, '/s', 'stamina regen rate while not sprinting'),

    -- soldier combat
    ('soldier_max_lives',      10,    1,  99, '',     'starting soldier lives'),
    ('soldier_knockback_vx',  200,    0, 500, 'px/s', 'horizontal knockback away when soldier is hit'),
    ('soldier_knockback_vy', -300, -600,   0, 'px/s', 'upward pop when soldier is hit (airborne i-frame)'),
    ('soldier_foot_padding',    0,    0,  80, 'px',   'transparent px at soldier sprite frame bottom (pre-render scale)'),

    -- orc
    ('orc_max_lives',          2,    1,  10, '',     'starting orc lives'),
    ('orc_hurt_bounce_vx',   120,    0, 500, 'px/s', 'horizontal bounce away from attacker when orc is hurt'),
    ('orc_hurt_bounce_vy',  -180, -500,   0, 'px/s', 'vertical pop applied on orc hurt'),
    ('orc_foot_padding',      43,    0, 100, 'px',   'transparent px at orc sprite frame bottom (pre-render scale)'),
    ('orc_points',            10,    0,1000, '',     'points awarded when orc killed'),

    -- slime
    ('slime_max_lives',         2,    1,  10, '',     'starting slime lives'),
    ('slime_hurt_bounce_vx',  120,    0, 500, 'px/s', 'slime hurt horizontal bounce'),
    ('slime_hurt_bounce_vy', -180, -500,   0, 'px/s', 'slime hurt vertical pop'),
    ('slime_foot_padding',     39,    0,  96, 'px',   'transparent px at slime sprite frame bottom'),
    ('slime_points',           15,    0,1000, '',    'points awarded when slime killed'),

    -- spawner (all kinds)
    ('enemy_spawn_min_s',  0.5, 0.1, 60, 's', 'minimum enemy spawn interval (all kinds)'),
    ('enemy_spawn_max_s',  1.5, 0.1, 60, 's', 'maximum enemy spawn interval (all kinds)'),
    ('enemy_max_alive',    5, 1, 10, '',  'max concurrent enemies (all kinds)'),

    -- game
    ('game_timeout_s', 30, 5, 300, 's', 'game duration before time-up screen');

-- ============================================================================
-- hitboxes
-- ============================================================================

INSERT INTO hitboxes
    (id, owner, kind, offset_x, offset_y, width, height, active_frame_start, active_frame_end)
VALUES
    ('soldier_body',    'soldier', 'body',    -15, -40, 20, 40, -1, -1),
    ('soldier_attack',  'soldier', 'attack',   15, -40, 35, 35,  1,  2),
    ('soldier_attack2', 'soldier', 'attack2',  12, -40, 35, 35,  2,  4),
    ('orc_body',        'orc',     'body',     -8, -15, 15, 15, -1, -1),
    ('orc_attack',      'orc',     'attack',   12, -18, 15, 15,  2,  3),
    ('orc_attack2',     'orc',     'attack2',  12, -18, 15, 15,  3,  4),
    ('slime_body',      'slime',   'body',     -8, -11, 14, 11, -1, -1),
    ('slime_attack',    'slime',   'attack',   12, -15, 15, 15,  4,  5),
    ('slime_attack2',   'slime',   'attack2',  15, -15, 15, 15,  3,  5);

-- ============================================================================
-- hud_layout
-- ============================================================================

INSERT INTO hud_layout (key, x, y, w, h, anchor, scale) VALUES
    ('heart',       70, 16, 16, 16, 'top_right',  2.0),
    ('lives_text',  16, 16,  0,  0, 'top_right',  1.0),
    ('score_text',  16, 16,  0,  0, 'top_left',   1.0),
    ('stamina_bar', 16, 48, 48, 16, 'top_left',   2.0),
    ('timer_text',   0, 16,  0,  0, 'top_left',   1.0);
