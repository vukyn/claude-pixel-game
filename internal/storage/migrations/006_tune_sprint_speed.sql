-- Dial sprint_speed down to 1.5x run_speed (420 px/s given run_speed=280)
-- so sprint feels like a controlled speed-up rather than a burst.
UPDATE tuning SET value = 420 WHERE key = 'sprint_speed';
