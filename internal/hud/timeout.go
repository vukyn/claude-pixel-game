package hud

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type TimeOut struct {
	Title    *text.GoTextFace
	Subtitle *text.GoTextFace
	WindowW  int
	WindowH  int
}

func NewTimeOut(title, subtitle *text.GoTextFace, w, h int) *TimeOut {
	return &TimeOut{Title: title, Subtitle: subtitle, WindowW: w, WindowH: h}
}

func (t *TimeOut) Draw(screen *ebiten.Image, score int) {
	vector.DrawFilledRect(screen, 0, 0, float32(t.WindowW), float32(t.WindowH),
		color.RGBA{0, 0, 0, 160}, false)

	drawCentered := func(s string, face *text.GoTextFace, yFrac float64) {
		w, _ := text.Measure(s, face, 0)
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(t.WindowW)/2-w/2, float64(t.WindowH)*yFrac)
		op.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
		text.Draw(screen, s, face, op)
	}

	drawCentered("TIME'S UP", t.Title, 0.33)
	drawCentered(fmt.Sprintf("Score: %d", score), t.Subtitle, 0.50)
	drawCentered("Press R to restart", t.Subtitle, 0.62)
}
