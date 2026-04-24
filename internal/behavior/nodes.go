package behavior

import "math/rand"

// Selector ticks children left→right. First non-Failure result wins.
type Selector struct {
	Children []Node
}

func (s *Selector) Tick(ctx *Ctx) Status {
	for _, ch := range s.Children {
		st := ch.Tick(ctx)
		if st != StatusFailure {
			return st
		}
	}
	return StatusFailure
}

// Sequence ticks children left→right. First non-Success result wins.
type Sequence struct {
	Children []Node
}

func (s *Sequence) Tick(ctx *Ctx) Status {
	for _, ch := range s.Children {
		st := ch.Tick(ctx)
		if st != StatusSuccess {
			return st
		}
	}
	return StatusSuccess
}

// ChanceBranch is a weighted arm of a Chance node.
type ChanceBranch struct {
	Weight int
	Node   Node
}

// Chance rolls once when idle, picks a branch by weight, then forwards
// ticks to it until it returns Success or Failure. Re-rolls on the next
// Tick after a terminal result.
type Chance struct {
	Branches []ChanceBranch

	active    int
	hasActive bool
}

func (c *Chance) Tick(ctx *Ctx) Status {
	if !c.hasActive {
		c.active = pickWeighted(ctx.RNG, c.Branches)
		c.hasActive = true
	}
	st := c.Branches[c.active].Node.Tick(ctx)
	if st != StatusRunning {
		c.hasActive = false
	}
	return st
}

func pickWeighted(rng *rand.Rand, branches []ChanceBranch) int {
	total := 0
	for _, b := range branches {
		if b.Weight > 0 {
			total += b.Weight
		}
	}
	if total == 0 {
		return len(branches) - 1
	}
	r := rng.Intn(total)
	for i, b := range branches {
		if b.Weight <= 0 {
			continue
		}
		if r < b.Weight {
			return i
		}
		r -= b.Weight
	}
	return len(branches) - 1
}

// Wait returns Running until the accumulated DT exceeds Seconds, then Success.
// Restarts automatically after reporting Success.
type Wait struct {
	Seconds float64

	elapsed float64
}

func (w *Wait) Tick(ctx *Ctx) Status {
	w.elapsed += ctx.DT.Seconds()
	if w.elapsed < w.Seconds {
		return StatusRunning
	}
	w.elapsed = 0
	return StatusSuccess
}
