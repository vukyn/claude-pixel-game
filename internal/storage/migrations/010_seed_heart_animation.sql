-- 010_seed_heart_animation.sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path, is_player, is_enemy, grid_cols, grid_rows, pick_row)
VALUES
    ('heart_beat', 'HeartsBeat.png', 4, 400, 1, 16, 16, 'heart/HeartsBeat.png', 0, 0, 4, 6, 3);
