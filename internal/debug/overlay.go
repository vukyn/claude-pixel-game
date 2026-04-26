package debug

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	debugCharW   = 6
	debugLineH   = 16
	debugPadding = 4
	debugOriginX = 8
	debugOriginY = 8
)

type Overlay struct {
	cfg     *Config
	source  FieldSource
	enabled bool
}

func NewOverlay(cfg *Config, source FieldSource) *Overlay {
	return &Overlay{cfg: cfg, source: source}
}

func (o *Overlay) Toggle()       { o.enabled = !o.enabled }
func (o *Overlay) Enabled() bool { return o.enabled }

func (o *Overlay) Draw(screen *ebiten.Image) {
	if !o.enabled {
		return
	}
	var b strings.Builder
	for i, sec := range o.cfg.Sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("-- ")
		b.WriteString(sec.Title)
		b.WriteString(" --\n")
		for _, key := range sec.Fields {
			f := Catalog[key]
			b.WriteString(f.Format(o.source))
			b.WriteString("\n")
		}
	}
	body := b.String()

	maxLine, lines := 0, 0
	for _, ln := range strings.Split(body, "\n") {
		lines++
		if n := len(ln); n > maxLine {
			maxLine = n
		}
	}
	bgW := float32(maxLine*debugCharW + debugPadding*2)
	bgH := float32(lines*debugLineH + debugPadding*2)
	vector.DrawFilledRect(screen,
		float32(debugOriginX-debugPadding), float32(debugOriginY-debugPadding),
		bgW, bgH,
		color.RGBA{0, 0, 0, 160}, false)

	ebitenutil.DebugPrintAt(screen, body, debugOriginX, debugOriginY)
}
