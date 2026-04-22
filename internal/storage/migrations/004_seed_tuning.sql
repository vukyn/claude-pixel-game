INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('run_speed',        280,     50,   1000, 'px/s',   'Horizontal ground movement speed'),
    ('air_control',      0.8,      0,      1, '',       'Horizontal movement multiplier while airborne'),
    ('jump_velocity',   -650,  -2000,   -100, 'px/s',   'Jump impulse applied on takeoff (negative = upward)'),
    ('gravity',         2000,    100,   5000, 'px/s^2', 'Downward acceleration applied each tick'),
    ('max_fall_speed',   900,    100,   3000, 'px/s',   'Terminal vertical velocity clamp'),
    ('dash_speed',       700,    100,   2000, 'px/s',   'Horizontal velocity during dash'),
    ('dash_duration_ms', 500,     50,   2000, 'ms',     'Dash duration');
