-- Snappier combat feel: reduce attack durations.
UPDATE animations SET duration_ms = 500 WHERE id = 'attack';
UPDATE animations SET duration_ms = 750 WHERE id = 'attack2';

-- Shift is now a held sprint modifier (Shift + A/D = move faster),
-- not an edge-triggered dash burst. Rename dash_speed to sprint_speed
-- and drop the obsolete dash_duration_ms parameter.
UPDATE tuning
   SET key         = 'sprint_speed',
       description = 'Horizontal ground movement speed while Shift held'
 WHERE key = 'dash_speed';

DELETE FROM tuning WHERE key = 'dash_duration_ms';
