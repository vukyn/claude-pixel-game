package anim

import "claude-pixel/internal/storage"

type SpecMapper struct{}

func (SpecMapper) Table() string { return "animations" }

func (SpecMapper) Columns() []string {
	return []string{"id", "file", "frame_count", "duration_ms", "loop"}
}

func (SpecMapper) Scan(row storage.Scanner) (AnimationSpec, error) {
	var s AnimationSpec
	var loopInt int
	err := row.Scan(&s.ID, &s.File, &s.FrameCount, &s.DurationMs, &loopInt)
	s.Loop = loopInt != 0
	return s, err
}

func (SpecMapper) Values(s AnimationSpec) []any {
	loop := 0
	if s.Loop {
		loop = 1
	}
	return []any{s.ID, s.File, s.FrameCount, s.DurationMs, loop}
}
