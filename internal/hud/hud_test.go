package hud

import "testing"

func TestFormatLives(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{10, "x10"},
		{1, "x1"},
		{0, "x0"},
		{-3, "x0"},
	}
	for _, c := range cases {
		if got := formatLives(c.in); got != c.want {
			t.Errorf("formatLives(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
