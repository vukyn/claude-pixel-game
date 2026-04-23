package score

import "testing"

func TestCounterStartsZero(t *testing.T) {
	c := &Counter{}
	if c.Total() != 0 {
		t.Fatalf("want 0, got %d", c.Total())
	}
}

func TestCounterAddAccumulates(t *testing.T) {
	c := &Counter{}
	c.Add(10)
	c.Add(15)
	if c.Total() != 25 {
		t.Fatalf("want 25, got %d", c.Total())
	}
}

func TestCounterResetZeroes(t *testing.T) {
	c := &Counter{}
	c.Add(42)
	c.Reset()
	if c.Total() != 0 {
		t.Fatalf("want 0 after Reset, got %d", c.Total())
	}
}

func TestCounterIgnoresNegativeOrZero(t *testing.T) {
	c := &Counter{}
	c.Add(-5)
	c.Add(0)
	if c.Total() != 0 {
		t.Fatalf("want 0, got %d", c.Total())
	}
}
