package anim

import (
	"testing"
	"time"
)

func makeAnim(loop bool, count int, dur time.Duration) *Animation {
	return &Animation{
		spec: &AnimationSpec{ID: "x", FrameCount: count, DurationMs: int(dur / time.Millisecond), Loop: loop},
	}
}

func TestFrameIndexLoop(t *testing.T) {
	a := makeAnim(true, 10, 1000*time.Millisecond) // 100ms per frame
	cases := []struct {
		elapsed time.Duration
		want    int
	}{
		{0, 0},
		{99 * time.Millisecond, 0},
		{100 * time.Millisecond, 1},
		{950 * time.Millisecond, 9},
		{1000 * time.Millisecond, 0}, // wrap
		{1500 * time.Millisecond, 5},
	}
	for _, c := range cases {
		a.elapsed = c.elapsed
		if got := a.FrameIndex(); got != c.want {
			t.Errorf("elapsed %v: got %d want %d", c.elapsed, got, c.want)
		}
	}
}

func TestFrameIndexNonLoopClamps(t *testing.T) {
	a := makeAnim(false, 3, 300*time.Millisecond) // 100ms per frame
	a.elapsed = 299 * time.Millisecond
	if got := a.FrameIndex(); got != 2 {
		t.Errorf("clamp at last: got %d want 2", got)
	}
	a.elapsed = 5 * time.Second
	if got := a.FrameIndex(); got != 2 {
		t.Errorf("clamp beyond: got %d want 2", got)
	}
	if !a.Done() {
		t.Errorf("expected Done=true")
	}
}

func TestLoopNeverDone(t *testing.T) {
	a := makeAnim(true, 5, 500*time.Millisecond)
	a.elapsed = 10 * time.Second
	if a.Done() {
		t.Errorf("loop should never be Done")
	}
}

func TestUpdateAdvancesElapsed(t *testing.T) {
	a := makeAnim(true, 10, 1000*time.Millisecond)
	a.Update(50 * time.Millisecond)
	a.Update(50 * time.Millisecond)
	if a.elapsed != 100*time.Millisecond {
		t.Errorf("elapsed=%v", a.elapsed)
	}
}

func TestResetZeroesElapsed(t *testing.T) {
	a := makeAnim(true, 10, 1000*time.Millisecond)
	a.elapsed = 500 * time.Millisecond
	a.Reset()
	if a.elapsed != 0 {
		t.Errorf("elapsed=%v", a.elapsed)
	}
}
