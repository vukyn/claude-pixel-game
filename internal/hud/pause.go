package hud

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Pause struct {
	Title    *text.GoTextFace
	Subtitle *text.GoTextFace
	WindowW  int
	WindowH  int
}

func NewPause(title, subtitle *text.GoTextFace, w, h int) *Pause {
	return &Pause{Title: title, Subtitle: subtitle, WindowW: w, WindowH: h}
}

func (p *Pause) Draw(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, 0, float32(p.WindowW), float32(p.WindowH),
		color.RGBA{0, 0, 0, 160}, false)

	drawCentered := func(s string, face *text.GoTextFace, yFrac float64) {
		w, _ := text.Measure(s, face, 0)
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(p.WindowW)/2-w/2, float64(p.WindowH)*yFrac)
		op.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
		text.Draw(screen, s, face, op)
	}

	drawCentered("PAUSED", p.Title, 0.40)
	drawCentered("Press any key to resume", p.Subtitle, 0.55)
}
