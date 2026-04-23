package hud

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type GameOver struct {
	Title    *text.GoTextFace
	Subtitle *text.GoTextFace
	WindowW  int
	WindowH  int
}

func NewGameOver(title, subtitle *text.GoTextFace, w, h int) *GameOver {
	return &GameOver{Title: title, Subtitle: subtitle, WindowW: w, WindowH: h}
}

func (g *GameOver) Draw(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, 0, float32(g.WindowW), float32(g.WindowH),
		color.RGBA{0, 0, 0, 160}, false)

	drawCentered := func(s string, face *text.GoTextFace, yFrac float64) {
		w, _ := text.Measure(s, face, 0)
		op := &text.DrawOptions{}
		op.GeoM.Translate(float64(g.WindowW)/2-w/2, float64(g.WindowH)*yFrac)
		op.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
		text.Draw(screen, s, face, op)
	}

	drawCentered("GAME OVER", g.Title, 0.40)
	drawCentered("Press R to restart", g.Subtitle, 0.55)
}
