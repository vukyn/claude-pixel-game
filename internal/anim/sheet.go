package anim

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

func Slice(img *ebiten.Image, frameW, frameH, count int) []*ebiten.Image {
	frames := make([]*ebiten.Image, count)
	for i := 0; i < count; i++ {
		r := image.Rect(i*frameW, 0, (i+1)*frameW, frameH)
		frames[i] = img.SubImage(r).(*ebiten.Image)
	}
	return frames
}

// SliceGrid slices a 2D grid sheet (cols x rows of frameW x frameH), picking
// `count` consecutive frames from row `pickRow` (0-indexed).
func SliceGrid(img *ebiten.Image, frameW, frameH, cols, rows, pickRow, count int) []*ebiten.Image {
	_ = rows
	frames := make([]*ebiten.Image, count)
	for i := 0; i < count; i++ {
		col := i % cols
		x0 := col * frameW
		y0 := pickRow * frameH
		r := image.Rect(x0, y0, x0+frameW, y0+frameH)
		frames[i] = img.SubImage(r).(*ebiten.Image)
	}
	return frames
}
