package combat

import "claude-pixel/internal/storage"

type AttackMotionSpec struct {
	ID         string
	Owner      string
	Kind       string
	VX         float64
	FrameStart int
	FrameEnd   int
}

func (s AttackMotionSpec) GetID() string { return s.ID }

type AttackMotionMapper struct{}

func (AttackMotionMapper) Table() string { return "attack_motions" }

func (AttackMotionMapper) Columns() []string {
	return []string{"id", "owner", "kind", "vx", "frame_start", "frame_end"}
}

func (AttackMotionMapper) Scan(row storage.Scanner) (AttackMotionSpec, error) {
	var s AttackMotionSpec
	err := row.Scan(&s.ID, &s.Owner, &s.Kind, &s.VX, &s.FrameStart, &s.FrameEnd)
	return s, err
}

func (AttackMotionMapper) Values(s AttackMotionSpec) []any {
	return []any{s.ID, s.Owner, s.Kind, s.VX, s.FrameStart, s.FrameEnd}
}
