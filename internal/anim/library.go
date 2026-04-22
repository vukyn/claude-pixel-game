package anim

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"claude-pixel/internal/config"
	"claude-pixel/internal/storage"
)

func LoadLibrary(cfg *config.Config, repo *storage.Repository[AnimationSpec]) (map[string]*Animation, error) {
	specs, err := repo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}
	out := make(map[string]*Animation, len(specs))
	for i := range specs {
		spec := specs[i]
		path := filepath.Join(cfg.AssetsDir, spec.File)
		img, _, err := ebitenutil.NewImageFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		wantW := cfg.SpriteFrameW * spec.FrameCount
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		if w != wantW || h != cfg.SpriteFrameH {
			return nil, fmt.Errorf("sheet %s: got %dx%d, want %dx%d", spec.File, w, h, wantW, cfg.SpriteFrameH)
		}
		frames := Slice(img, cfg.SpriteFrameW, cfg.SpriteFrameH, spec.FrameCount)
		out[spec.ID] = NewAnimation(&spec, frames)
	}
	return out, nil
}

var _ = (*ebiten.Image)(nil)
