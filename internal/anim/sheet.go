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
