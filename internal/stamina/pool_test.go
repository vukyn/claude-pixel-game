package stamina

import (
	"math"
	"testing"
	"time"
)

func approxEq(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestPoolStartsFull(t *testing.T) {
	p := NewPool(100, 20, 20)
	if p.Cur != 100 {
		t.Fatalf("want Cur=100, got %f", p.Cur)
	}
	if !p.CanSprint() {
		t.Fatal("want CanSprint true at full")
	}
	if !approxEq(p.Fraction(), 1.0) {
		t.Fatalf("want Fraction=1.0, got %f", p.Fraction())
	}
}

func TestPoolDrainsToZeroIn5s(t *testing.T) {
	p := NewPool(100, 20, 20)
	for i := 0; i < 300; i++ {
		p.Update(time.Second/60, true)
	}
	if !approxEq(p.Cur, 0) {
		t.Fatalf("want Cur=0 after 5s drain, got %f", p.Cur)
	}
	if p.CanSprint() {
		t.Fatal("want CanSprint false when empty")
	}
}

func TestPoolRegensToMaxIn5s(t *testing.T) {
	p := NewPool(100, 20, 20)
	p.Cur = 0
	for i := 0; i < 300; i++ {
		p.Update(time.Second/60, false)
	}
	if !approxEq(p.Cur, 100) {
		t.Fatalf("want Cur=100 after 5s regen, got %f", p.Cur)
	}
}

func TestPoolClampsAtZero(t *testing.T) {
	p := NewPool(100, 20, 20)
	p.Cur = 1
	p.Update(time.Second, true)
	if p.Cur != 0 {
		t.Fatalf("want Cur=0 clamped, got %f", p.Cur)
	}
}

func TestPoolClampsAtMax(t *testing.T) {
	p := NewPool(100, 20, 20)
	p.Cur = 99
	p.Update(time.Second, false)
	if p.Cur != 100 {
		t.Fatalf("want Cur=100 clamped, got %f", p.Cur)
	}
}

func TestPoolNoChangeWhenNotSprintingAndFull(t *testing.T) {
	p := NewPool(100, 20, 20)
	p.Update(time.Second, false)
	if p.Cur != 100 {
		t.Fatalf("want Cur=100, got %f", p.Cur)
	}
}

func TestLockoutAfterDepletionUntilFullRefill(t *testing.T) {
	p := NewPool(100, 20, 20)
	// drain to 0
	for i := 0; i < 300; i++ {
		p.Update(time.Second/60, true)
	}
	if !p.Locked {
		t.Fatal("want Locked=true at Cur=0")
	}
	if p.CanSprint() {
		t.Fatal("want CanSprint=false while Locked")
	}
	// partial regen — still locked
	for i := 0; i < 150; i++ { // 2.5s of regen → Cur ≈ 50
		p.Update(time.Second/60, false)
	}
	if !p.Locked {
		t.Fatalf("want still Locked at Cur=%f (< Max)", p.Cur)
	}
	if p.CanSprint() {
		t.Fatal("want CanSprint=false at 50%% while Locked")
	}
	// finish regen to full
	for i := 0; i < 151; i++ {
		p.Update(time.Second/60, false)
	}
	if p.Locked {
		t.Fatalf("want Locked=false at Cur=%f (>= Max)", p.Cur)
	}
	if !p.CanSprint() {
		t.Fatal("want CanSprint=true after full refill")
	}
}
