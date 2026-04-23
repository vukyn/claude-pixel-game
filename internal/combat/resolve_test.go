package combat

import (
	"testing"
)

type fakeFighter struct {
	x, y   float64
	facing int
	anim   string
	frame  int
	body   Box
	hits   []Box
	invul  bool
	alive  bool
	hitSet map[Fighter]bool
}

func (f *fakeFighter) Pos() (float64, float64) { return f.x, f.y }
func (f *fakeFighter) FacingDir() int          { return f.facing }
func (f *fakeFighter) CurrentAnimID() string   { return f.anim }
func (f *fakeFighter) CurrentFrame() int       { return f.frame }
func (f *fakeFighter) Body() Box               { return f.body }
func (f *fakeFighter) ActiveHits() []Box       { return f.hits }
func (f *fakeFighter) IsInvulnerable() bool    { return f.invul }
func (f *fakeFighter) Alive() bool             { return f.alive }
func (f *fakeFighter) AlreadyHit(t Fighter) bool {
	if f.hitSet == nil {
		return false
	}
	return f.hitSet[t]
}
func (f *fakeFighter) MarkHit(t Fighter) {
	if f.hitSet == nil {
		f.hitSet = map[Fighter]bool{}
	}
	f.hitSet[t] = true
}

func newFake() *fakeFighter {
	return &fakeFighter{facing: 1, alive: true}
}

func TestResolveEmitsEventOnOverlap(t *testing.T) {
	att := newFake()
	att.x, att.y = 100, 100
	att.anim = "soldier_attack"
	att.hits = []Box{{OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2}}
	att.frame = 2

	vic := newFake()
	vic.x, vic.y = 140, 100
	vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}

	events := Resolve([]Fighter{att}, []Fighter{vic})
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].AttackKind != "attack" {
		t.Errorf("want AttackKind=attack, got %q", events[0].AttackKind)
	}
}

func TestResolveSkipsInvulnerableVictim(t *testing.T) {
	att := newFake()
	att.x, att.y = 100, 100
	att.anim = "soldier_attack"
	att.hits = []Box{{OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2}}
	att.frame = 2

	vic := newFake()
	vic.x, vic.y = 140, 100
	vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}
	vic.invul = true

	events := Resolve([]Fighter{att}, []Fighter{vic})
	if len(events) != 0 {
		t.Fatalf("want 0 events, got %d", len(events))
	}
}

func TestResolveSkipsFrameOutsideWindow(t *testing.T) {
	att := newFake()
	att.x, att.y = 100, 100
	att.anim = "soldier_attack"
	att.hits = []Box{}
	att.frame = 0

	vic := newFake()
	vic.x, vic.y = 140, 100
	vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}

	events := Resolve([]Fighter{att}, []Fighter{vic})
	if len(events) != 0 {
		t.Fatalf("want 0 events, got %d", len(events))
	}
}

func TestResolveDedupsWithinAttackWindow(t *testing.T) {
	att := newFake()
	att.x, att.y = 100, 100
	att.anim = "soldier_attack"
	att.hits = []Box{{OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2}}
	att.frame = 2

	vic := newFake()
	vic.x, vic.y = 140, 100
	vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}

	e1 := Resolve([]Fighter{att}, []Fighter{vic})
	if len(e1) != 1 {
		t.Fatalf("first resolve: want 1, got %d", len(e1))
	}

	e2 := Resolve([]Fighter{att}, []Fighter{vic})
	if len(e2) != 0 {
		t.Fatalf("second resolve: want 0, got %d", len(e2))
	}
}

func TestResolveFlipsBoxForFacingMinus1(t *testing.T) {
	att := newFake()
	att.x, att.y = 100, 100
	att.facing = -1
	att.anim = "soldier_attack"
	att.hits = []Box{{OffsetX: 20, OffsetY: -60, W: 60, H: 50, FrameStart: 1, FrameEnd: 2}}
	att.frame = 2

	vic := newFake()
	vic.x, vic.y = 60, 100
	vic.body = Box{OffsetX: -25, OffsetY: -80, W: 50, H: 80, FrameStart: -1, FrameEnd: -1}

	events := Resolve([]Fighter{att}, []Fighter{vic})
	if len(events) != 1 {
		t.Fatalf("facing=-1 should hit left victim; got %d events", len(events))
	}
}
