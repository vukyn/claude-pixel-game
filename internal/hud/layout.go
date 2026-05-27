package hud

import (
	"context"
	"fmt"

	"claude-pixel/internal/storage"
)

type Anchor int

const (
	AnchorTopLeft Anchor = iota
	AnchorTopRight
	AnchorBottomLeft
	AnchorBottomRight
)

func (a Anchor) String() string {
	switch a {
	case AnchorTopLeft:
		return "top_left"
	case AnchorTopRight:
		return "top_right"
	case AnchorBottomLeft:
		return "bottom_left"
	case AnchorBottomRight:
		return "bottom_right"
	}
	return "unknown"
}

func ParseAnchor(s string) (Anchor, error) {
	switch s {
	case "top_left":
		return AnchorTopLeft, nil
	case "top_right":
		return AnchorTopRight, nil
	case "bottom_left":
		return AnchorBottomLeft, nil
	case "bottom_right":
		return AnchorBottomRight, nil
	}
	return 0, fmt.Errorf("unknown anchor %q (valid: top_left, top_right, bottom_left, bottom_right)", s)
}

type Element struct {
	X, Y, W, H int
	Anchor      Anchor
	Scale       float64
}

type Layout map[string]Element

// Resolve returns the element's absolute top-left in screen pixels.
// For variable-width text (W=0), caller should substitute measured width
// into the Element before calling.
func (l Layout) Resolve(key string, screenW, screenH int) (x, y float64) {
	e, ok := l[key]
	if !ok {
		return 0, 0
	}
	switch e.Anchor {
	case AnchorTopLeft:
		return float64(e.X), float64(e.Y)
	case AnchorTopRight:
		return float64(screenW - e.X - e.W), float64(e.Y)
	case AnchorBottomLeft:
		return float64(e.X), float64(screenH - e.Y - e.H)
	case AnchorBottomRight:
		return float64(screenW - e.X - e.W), float64(screenH - e.Y - e.H)
	}
	return 0, 0
}

// LayoutRow is the DB entity.
type LayoutRow struct {
	Key     string
	X       int
	Y       int
	W       int
	H       int
	AnchorS string
	Scale   float64
}

func (r LayoutRow) GetID() string { return r.Key }

type LayoutMapper struct{}

func (LayoutMapper) Table() string { return "hud_layout" }

func (LayoutMapper) Columns() []string {
	return []string{"key", "x", "y", "w", "h", "anchor", "scale"}
}

func (LayoutMapper) Scan(row storage.Scanner) (LayoutRow, error) {
	var r LayoutRow
	err := row.Scan(&r.Key, &r.X, &r.Y, &r.W, &r.H, &r.AnchorS, &r.Scale)
	return r, err
}

func (LayoutMapper) Values(r LayoutRow) []any {
	return []any{r.Key, r.X, r.Y, r.W, r.H, r.AnchorS, r.Scale}
}

var requiredLayoutKeys = []string{"heart", "lives_text", "score_text", "stamina_bar", "timer_text"}

// LoadLayout reads every hud_layout row, parses anchors, requires all
// required keys to be present.
func LoadLayout(repo *storage.Repository[LayoutRow]) (Layout, error) {
	rows, err := repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	out := Layout{}
	for _, r := range rows {
		a, err := ParseAnchor(r.AnchorS)
		if err != nil {
			return nil, fmt.Errorf("hud_layout.%s: %w", r.Key, err)
		}
		if r.Scale <= 0 {
			return nil, fmt.Errorf("hud_layout.%s: scale must be > 0 (got %f)", r.Key, r.Scale)
		}
		out[r.Key] = Element{X: r.X, Y: r.Y, W: r.W, H: r.H, Anchor: a, Scale: r.Scale}
	}
	for _, k := range requiredLayoutKeys {
		if _, ok := out[k]; !ok {
			return nil, fmt.Errorf("hud_layout missing required key %q; required: %v", k, requiredLayoutKeys)
		}
	}
	return out, nil
}
