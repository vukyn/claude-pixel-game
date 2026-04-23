-- 025_seed_stamina_bar_animation.sql
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path,
     is_player, is_enemy, grid_cols, grid_rows, pick_row, pick_col)
VALUES
    ('stamina_bar', 'healthbar.png', 10, 0, 0, 48, 16,
     'huds/healthbar/healthbar.png', 0, 0, 4, 10, 0, 2);
