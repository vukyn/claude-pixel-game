package hud

import (
	"bytes"
	"fmt"
	"os"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var source *text.GoTextFaceSource

// LoadFont reads the TTF file at path and stores the shared face source.
// Call once at boot before creating any faces.
func LoadFont(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read font %s: %w", path, err)
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse font %s: %w", path, err)
	}
	source = src
	return nil
}

// NewFace returns a monogram face at the given pixel size.
// Panics if LoadFont was not called.
func NewFace(size float64) *text.GoTextFace {
	if source == nil {
		panic("hud: font not loaded; call LoadFont() first")
	}
	return &text.GoTextFace{Source: source, Size: size}
}
