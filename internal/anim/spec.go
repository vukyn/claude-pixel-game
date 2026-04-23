package anim

import "claude-pixel/internal/storage"

type SpecMapper struct{}

func (SpecMapper) Table() string { return "animations" }

func (SpecMapper) Columns() []string {
	return []string{
		"id", "file", "frame_count", "duration_ms", "loop",
		"frame_w", "frame_h", "path", "is_player", "is_enemy",
		"grid_cols", "grid_rows", "pick_row", "pick_col",
	}
}

func (SpecMapper) Scan(row storage.Scanner) (AnimationSpec, error) {
	var s AnimationSpec
	var loopInt, isPlayerInt, isEnemyInt int
	err := row.Scan(
		&s.ID, &s.File, &s.FrameCount, &s.DurationMs, &loopInt,
		&s.FrameW, &s.FrameH, &s.Path, &isPlayerInt, &isEnemyInt,
		&s.GridCols, &s.GridRows, &s.PickRow, &s.PickCol,
	)
	s.Loop = loopInt != 0
	s.IsPlayer = isPlayerInt != 0
	s.IsEnemy = isEnemyInt != 0
	return s, err
}

func (SpecMapper) Values(s AnimationSpec) []any {
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	return []any{
		s.ID, s.File, s.FrameCount, s.DurationMs, b(s.Loop),
		s.FrameW, s.FrameH, s.Path, b(s.IsPlayer), b(s.IsEnemy),
		s.GridCols, s.GridRows, s.PickRow, s.PickCol,
	}
}
