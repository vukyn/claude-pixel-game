package debug

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
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
	ebitenutil.DebugPrintAt(screen, b.String(), 8, 8)
}
