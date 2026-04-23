package hud

import "testing"

func TestResolveTopLeft(t *testing.T) {
	l := Layout{"score_text": Element{X: 16, Y: 16, W: 100, H: 24, Anchor: AnchorTopLeft, Scale: 1}}
	x, y := l.Resolve("score_text", 800, 600)
	if x != 16 || y != 16 {
		t.Fatalf("want (16,16), got (%f,%f)", x, y)
	}
}

func TestResolveTopRight(t *testing.T) {
	l := Layout{"heart": Element{X: 48, Y: 16, W: 32, H: 32, Anchor: AnchorTopRight, Scale: 2}}
	x, y := l.Resolve("heart", 800, 600)
	// top_right: element's top-right at (windowW - X, Y). top-left = (windowW - X - W, Y).
	if x != 720 || y != 16 {
		t.Fatalf("want (720,16), got (%f,%f)", x, y)
	}
}

func TestResolveBottomLeft(t *testing.T) {
	l := Layout{"bar": Element{X: 10, Y: 20, W: 50, H: 10, Anchor: AnchorBottomLeft, Scale: 1}}
	x, y := l.Resolve("bar", 800, 600)
	// bottom_left: bottom-left at (X, windowH - Y). top-left = (X, windowH - Y - H).
	if x != 10 || y != 570 {
		t.Fatalf("want (10,570), got (%f,%f)", x, y)
	}
}

func TestResolveBottomRight(t *testing.T) {
	l := Layout{"bar": Element{X: 10, Y: 20, W: 50, H: 10, Anchor: AnchorBottomRight, Scale: 1}}
	x, y := l.Resolve("bar", 800, 600)
	// bottom-right: bottom-right at (windowW - X, windowH - Y). top-left = (740, 570).
	if x != 740 || y != 570 {
		t.Fatalf("want (740,570), got (%f,%f)", x, y)
	}
}

func TestParseAnchorValidAndInvalid(t *testing.T) {
	a, err := ParseAnchor("top_left")
	if err != nil || a != AnchorTopLeft {
		t.Fatalf("top_left: got %v err=%v", a, err)
	}
	if _, err := ParseAnchor("middle"); err == nil {
		t.Fatal("want error for unknown anchor")
	}
}
