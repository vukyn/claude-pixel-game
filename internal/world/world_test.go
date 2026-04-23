package world

import "testing"

func TestClamp(t *testing.T) {
	cases := []struct {
		in, min, max, want float64
	}{
		{50, 100, 200, 100},
		{150, 100, 200, 150},
		{250, 100, 200, 200},
		{100, 100, 200, 100},
		{200, 100, 200, 200},
	}
	for _, c := range cases {
		got := Clamp(c.in, c.min, c.max)
		if got != c.want {
			t.Errorf("Clamp(%v, %v, %v) = %v, want %v", c.in, c.min, c.max, got, c.want)
		}
	}
}
