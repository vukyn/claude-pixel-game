package hud

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Toast struct {
	Face    *text.GoTextFace
	WindowW int
	msg     string
	until   time.Time
}

func NewToast(face *text.GoTextFace, w int) *Toast {
	return &Toast{Face: face, WindowW: w}
}

func (t *Toast) Show(msg string, dur time.Duration) {
	t.msg = msg
	t.until = time.Now().Add(dur)
}

func (t *Toast) Draw(screen *ebiten.Image) {
	if t.msg == "" || time.Now().After(t.until) {
		return
	}
	w, _ := text.Measure(t.msg, t.Face, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(t.WindowW)/2-w/2, 24)
	op.ColorScale.ScaleWithColor(color.RGBA{0xE8, 0xE8, 0xE8, 0xFF})
	text.Draw(screen, t.msg, t.Face, op)
}
