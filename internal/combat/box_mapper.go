package combat

import "claude-pixel/internal/storage"

type HitboxSpec struct {
	ID         string
	Owner      string
	Kind       string
	OffsetX    int
	OffsetY    int
	Width      int
	Height     int
	FrameStart int
	FrameEnd   int
}

func (h HitboxSpec) GetID() string { return h.ID }

func (h HitboxSpec) ToBox() Box {
	return Box{
		OffsetX:    h.OffsetX,
		OffsetY:    h.OffsetY,
		W:          h.Width,
		H:          h.Height,
		FrameStart: h.FrameStart,
		FrameEnd:   h.FrameEnd,
	}
}

type HitboxMapper struct{}

func (HitboxMapper) Table() string { return "hitboxes" }

func (HitboxMapper) Columns() []string {
	return []string{"id", "owner", "kind", "offset_x", "offset_y", "width", "height", "active_frame_start", "active_frame_end"}
}

func (HitboxMapper) Scan(row storage.Scanner) (HitboxSpec, error) {
	var s HitboxSpec
	err := row.Scan(&s.ID, &s.Owner, &s.Kind, &s.OffsetX, &s.OffsetY, &s.Width, &s.Height, &s.FrameStart, &s.FrameEnd)
	return s, err
}

func (HitboxMapper) Values(s HitboxSpec) []any {
	return []any{s.ID, s.Owner, s.Kind, s.OffsetX, s.OffsetY, s.Width, s.Height, s.FrameStart, s.FrameEnd}
}
