-- 015_seed_foot_padding.sql
-- Foot padding = transparent px at bottom of sprite frame (pre-scale).
-- Used by renderer to shift draw anchor up so visible feet land at world Y.
INSERT OR IGNORE INTO tuning (key, value, min_value, max_value, unit, description) VALUES
    ('soldier_foot_padding',  0, 0, 80, 'px', 'transparent px at soldier sprite frame bottom (pre-render scale)'),
    ('orc_foot_padding',     45, 0, 100, 'px', 'transparent px at orc sprite frame bottom (pre-render scale)');
