package anim

import "testing"

func TestGridFrameRectPicksCorrectRow(t *testing.T) {
	// 4 cols x 6 rows of 16x16, pickRow=3 (0-indexed), 4 frames.
	frameW, frameH, cols, pickRow := 16, 16, 4, 3
	for i := 0; i < 4; i++ {
		r := gridFrameRect(frameW, frameH, cols, pickRow, i)
		if r.Dx() != 16 || r.Dy() != 16 {
			t.Errorf("frame %d: want 16x16, got %dx%d", i, r.Dx(), r.Dy())
		}
		if r.Min.X != i*16 || r.Min.Y != 48 {
			t.Errorf("frame %d: want origin (%d,48), got (%d,%d)", i, i*16, r.Min.X, r.Min.Y)
		}
	}
}

func TestGridFrameRectWrapsColumns(t *testing.T) {
	// With cols=4, frame index 5 should wrap to col=1.
	r := gridFrameRect(10, 10, 4, 0, 5)
	if r.Min.X != 10 {
		t.Errorf("frame 5 col wrap: want x=10, got %d", r.Min.X)
	}
}
