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
		path := filepath.Join(cfg.AssetsDir, spec.Path)
		img, _, err := ebitenutil.NewImageFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}

		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		var frames []*ebiten.Image

		if spec.GridCols > 0 {
			wantW := spec.FrameW * spec.GridCols
			wantH := spec.FrameH * spec.GridRows
			if w != wantW || h != wantH {
				return nil, fmt.Errorf("sheet %s (grid): got %dx%d, want %dx%d", spec.Path, w, h, wantW, wantH)
			}
			frames = SliceGrid(img, spec.FrameW, spec.FrameH, spec.GridCols, spec.GridRows, spec.PickRow, spec.FrameCount)
		} else {
			wantW := spec.FrameW * spec.FrameCount
			if w != wantW || h != spec.FrameH {
				return nil, fmt.Errorf("sheet %s (strip): got %dx%d, want %dx%d", spec.Path, w, h, wantW, spec.FrameH)
			}
			frames = Slice(img, spec.FrameW, spec.FrameH, spec.FrameCount)
		}
		out[spec.ID] = NewAnimation(&spec, frames)
	}
	return out, nil
}
