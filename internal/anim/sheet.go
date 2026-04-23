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

// gridFrameRect returns the sub-image rect for frame i of a 2D grid sheet
// (cols x rows of frameW x frameH), picking from row pickRow (0-indexed).
func gridFrameRect(frameW, frameH, cols, pickRow, i int) image.Rectangle {
	col := i % cols
	x0 := col * frameW
	y0 := pickRow * frameH
	return image.Rect(x0, y0, x0+frameW, y0+frameH)
}

// SliceGrid slices a 2D grid sheet (cols x rows of frameW x frameH), picking
// `count` consecutive frames from row `pickRow` (0-indexed).
func SliceGrid(img *ebiten.Image, frameW, frameH, cols, rows, pickRow, count int) []*ebiten.Image {
	_ = rows
	frames := make([]*ebiten.Image, count)
	for i := 0; i < count; i++ {
		frames[i] = img.SubImage(gridFrameRect(frameW, frameH, cols, pickRow, i)).(*ebiten.Image)
	}
	return frames
}

// SliceColumn slices a 2D grid sheet (cols x rows of frameW x frameH), picking
// `count` consecutive frames from column `pickCol` (0-indexed), top-to-bottom.
func SliceColumn(img *ebiten.Image, frameW, frameH, pickCol, count int) []*ebiten.Image {
	frames := make([]*ebiten.Image, count)
	for i := 0; i < count; i++ {
		x0 := pickCol * frameW
		y0 := i * frameH
		frames[i] = img.SubImage(image.Rect(x0, y0, x0+frameW, y0+frameH)).(*ebiten.Image)
	}
	return frames
}
