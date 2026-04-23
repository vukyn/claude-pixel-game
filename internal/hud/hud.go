package hud

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"claude-pixel/internal/anim"
)

type LivesProvider interface {
	Lives() int
}

type HUD struct {
	Heart    *anim.Animation
	Face     *text.GoTextFace
	Provider LivesProvider
	Scale    float64
	WindowW  int
}

func NewHUD(heart *anim.Animation, face *text.GoTextFace, provider LivesProvider, windowW int, scale float64) *HUD {
	return &HUD{Heart: heart, Face: face, Provider: provider, Scale: scale, WindowW: windowW}
}

func (h *HUD) Update(dt time.Duration) { h.Heart.Update(dt) }

func formatLives(lives int) string {
	if lives < 0 {
		lives = 0
	}
	return fmt.Sprintf("x%d", lives)
}

func (h *HUD) Draw(screen *ebiten.Image) {
	label := formatLives(h.Provider.Lives())

	textW, _ := text.Measure(label, h.Face, 0)

	const padding = 16.0
	const gap = 8.0
	heartSize := 16.0 * h.Scale

	textX := float64(h.WindowW) - padding - textW
	heartX := textX - gap - heartSize
	topY := padding

	if frame := h.Heart.CurrentFrame(); frame != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(h.Scale, h.Scale)
		op.GeoM.Translate(heartX, topY)
		op.Filter = ebiten.FilterNearest
		screen.DrawImage(frame, op)
	}

	textOp := &text.DrawOptions{}
	textOp.GeoM.Translate(textX, topY)
	textOp.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
	text.Draw(screen, label, h.Face, textOp)
}
