package combat

type Box struct {
	OffsetX, OffsetY int
	W, H             int
	FrameStart       int
	FrameEnd         int
}

// Active reports whether this box is live on the given 0-indexed frame.
// FrameStart = -1 means the box is always active (used for body boxes).
func (b Box) Active(frame int) bool {
	if b.FrameStart < 0 {
		return true
	}
	return frame >= b.FrameStart && frame <= b.FrameEnd
}

// Scale returns a copy with OffsetX/Y and W/H multiplied by `s`.
// Frame window is unchanged.
func (b Box) Scale(s int) Box {
	return Box{
		OffsetX:    b.OffsetX * s,
		OffsetY:    b.OffsetY * s,
		W:          b.W * s,
		H:          b.H * s,
		FrameStart: b.FrameStart,
		FrameEnd:   b.FrameEnd,
	}
}
