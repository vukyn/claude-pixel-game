-- 016_seed_soldier_hit_death.sql
-- soldier_hit (1 frame, held during airborne i-frame) and soldier_death
-- (10 frames, terminal) anims were specified by the combat design but never
-- seeded. Without them PlayAnim is a no-op, so player.Current.Done() never
-- becomes true and Game never transitions to GameOverState.
INSERT OR IGNORE INTO animations
    (id, file, frame_count, duration_ms, loop, frame_w, frame_h, path, is_player, is_enemy)
VALUES
    ('soldier_hit',   '_Hit.png',    1,  200, 0, 120, 80, 'soldier/_Hit.png',   1, 0),
    ('soldier_death', '_Death.png', 10, 1000, 0, 120, 80, 'soldier/_Death.png', 1, 0);
