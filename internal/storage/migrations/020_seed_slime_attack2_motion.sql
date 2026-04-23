-- 020_seed_slime_attack_motion.sql
-- Slime Attack2: retreats (negative VX relative to facing) on frames 3-5 (0-indexed).
INSERT OR IGNORE INTO attack_motions
    (id, owner, kind, vx, frame_start, frame_end)
VALUES
    ('slime_attack2_motion', 'slime', 'attack2', -60, 3, 5);
