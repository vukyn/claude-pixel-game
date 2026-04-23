package anim

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type AnimationSpec struct {
	ID         string
	File       string
	FrameCount int
	DurationMs int
	Loop       bool
	FrameW     int
	FrameH     int
	Path       string
	IsPlayer   bool
	IsEnemy    bool
	GridCols   int
	GridRows   int
	PickRow    int
	PickCol    int // -1 = row mode (use PickRow). >=0 = column-strip mode.
}

func (a AnimationSpec) GetID() string { return a.ID }

type Animation struct {
	spec    *AnimationSpec
	frames  []*ebiten.Image
	elapsed time.Duration
}

func NewAnimation(spec *AnimationSpec, frames []*ebiten.Image) *Animation {
	return &Animation{spec: spec, frames: frames}
}

func (a *Animation) Update(dt time.Duration) { a.elapsed += dt }

func (a *Animation) Reset() { a.elapsed = 0 }

func (a *Animation) Elapsed() time.Duration { return a.elapsed }

func (a *Animation) SpecID() string { return a.spec.ID }

func (a *Animation) FrameIndex() int {
	if a.spec.FrameCount <= 0 {
		return 0
	}
	totalMs := a.elapsed.Milliseconds()
	perFrameMs := int64(a.spec.DurationMs) / int64(a.spec.FrameCount)
	if perFrameMs <= 0 {
		return 0
	}
	idx := int(totalMs / perFrameMs)
	if a.spec.Loop {
		return idx % a.spec.FrameCount
	}
	if idx >= a.spec.FrameCount {
		return a.spec.FrameCount - 1
	}
	return idx
}

func (a *Animation) Done() bool {
	if a.spec.Loop {
		return false
	}
	return a.elapsed.Milliseconds() >= int64(a.spec.DurationMs)
}

func (a *Animation) CurrentFrame() *ebiten.Image {
	if len(a.frames) == 0 {
		return nil
	}
	return a.frames[a.FrameIndex()]
}

// FrameAt returns the frame at explicit index (0-based), for non-time-driven
// animations (e.g. progress bars). Clamps to range; returns nil if no frames.
func (a *Animation) FrameAt(i int) *ebiten.Image {
	if len(a.frames) == 0 {
		return nil
	}
	if i < 0 {
		i = 0
	}
	if i >= len(a.frames) {
		i = len(a.frames) - 1
	}
	return a.frames[i]
}
