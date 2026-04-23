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

type StaminaProvider interface {
	StaminaFraction() float64
}

type ScoreProvider interface {
	Score() int
}

type HUD struct {
	Heart      *anim.Animation
	StaminaBar *anim.Animation
	Face       *text.GoTextFace
	Lives      LivesProvider
	Stamina    StaminaProvider
	Score      ScoreProvider
	Layout     Layout
	WindowW    int
	WindowH    int
}

func NewHUD(
	heart *anim.Animation,
	staminaBar *anim.Animation,
	face *text.GoTextFace,
	lives LivesProvider,
	stamina StaminaProvider,
	score ScoreProvider,
	layout Layout,
	windowW, windowH int,
) *HUD {
	return &HUD{
		Heart: heart, StaminaBar: staminaBar, Face: face,
		Lives: lives, Stamina: stamina, Score: score,
		Layout: layout, WindowW: windowW, WindowH: windowH,
	}
}

func (h *HUD) Update(dt time.Duration) {
	if h.Heart != nil {
		h.Heart.Update(dt)
	}
}

func formatLives(n int) string {
	if n < 0 {
		n = 0
	}
	return fmt.Sprintf("x%d", n)
}

func formatScore(n int) string { return fmt.Sprintf("Score: %d", n) }

func (h *HUD) Draw(screen *ebiten.Image) {
	h.drawHeart(screen)
	h.drawLives(screen)
	h.drawScore(screen)
	h.drawStamina(screen)
}

func (h *HUD) drawHeart(screen *ebiten.Image) {
	if h.Heart == nil {
		return
	}
	frame := h.Heart.CurrentFrame()
	if frame == nil {
		return
	}
	e := h.Layout["heart"]
	x, y := h.Layout.Resolve("heart", h.WindowW, h.WindowH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(e.Scale, e.Scale)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(frame, op)
}

// drawTextElement handles variable-width text with anchor-aware placement.
// W=0 in DB means "measure at draw time".
func (h *HUD) drawTextElement(screen *ebiten.Image, key, label string) {
	if h.Face == nil {
		return
	}
	tw, _ := text.Measure(label, h.Face, 0)
	orig := h.Layout[key]
	e := orig
	e.W = int(tw)
	h.Layout[key] = e
	x, y := h.Layout.Resolve(key, h.WindowW, h.WindowH)
	h.Layout[key] = orig // restore stored W

	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
	text.Draw(screen, label, h.Face, op)
}

func (h *HUD) drawLives(screen *ebiten.Image) {
	if h.Lives == nil {
		return
	}
	h.drawTextElement(screen, "lives_text", formatLives(h.Lives.Lives()))
}

func (h *HUD) drawScore(screen *ebiten.Image) {
	if h.Score == nil {
		return
	}
	h.drawTextElement(screen, "score_text", formatScore(h.Score.Score()))
}

func (h *HUD) drawStamina(screen *ebiten.Image) {
	if h.StaminaBar == nil || h.Stamina == nil {
		return
	}
	frac := h.Stamina.StaminaFraction()
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}
	// frame 0 = full, frame 9 = empty (10 frames total)
	idx := int((1.0-frac)*9 + 0.5)
	if idx < 0 {
		idx = 0
	} else if idx > 9 {
		idx = 9
	}
	frame := h.StaminaBar.FrameAt(idx)
	if frame == nil {
		return
	}
	e := h.Layout["stamina_bar"]
	x, y := h.Layout.Resolve("stamina_bar", h.WindowW, h.WindowH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(e.Scale, e.Scale)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(frame, op)
}
