package anim

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestSliceGridPicksCorrectRow(t *testing.T) {
	// 4 cols x 6 rows of 16x16 => 64x96
	img := ebiten.NewImage(64, 96)
	frames := SliceGrid(img, 16, 16, 4, 6, 3, 4)
	if len(frames) != 4 {
		t.Fatalf("want 4 frames, got %d", len(frames))
	}
	for i, f := range frames {
		b := f.Bounds()
		if b.Dx() != 16 || b.Dy() != 16 {
			t.Errorf("frame %d: want 16x16, got %dx%d", i, b.Dx(), b.Dy())
		}
		// pick_row=3 (0-indexed) -> y origin 48; col i -> x origin i*16
		if b.Min.X != i*16 || b.Min.Y != 48 {
			t.Errorf("frame %d: want origin (%d,48), got (%d,%d)", i, i*16, b.Min.X, b.Min.Y)
		}
	}
}
