package stamina

import "time"

type Pool struct {
	Max         float64
	Cur         float64
	DrainPerSec float64
	RegenPerSec float64
	// Locked goes true when Cur reaches 0 and clears only when Cur is fully
	// refilled to Max. Sprint is blocked while locked.
	Locked bool
}

func NewPool(max, drain, regen float64) *Pool {
	return &Pool{Max: max, Cur: max, DrainPerSec: drain, RegenPerSec: regen}
}

func (p *Pool) Update(dt time.Duration, sprinting bool) {
	dtS := dt.Seconds()
	if sprinting {
		p.Cur -= p.DrainPerSec * dtS
	} else {
		p.Cur += p.RegenPerSec * dtS
	}
	if p.Cur < 1e-4 {
		p.Cur = 0
	}
	if p.Cur > p.Max {
		p.Cur = p.Max
	}
	if p.Cur == 0 {
		p.Locked = true
	} else if p.Cur >= p.Max {
		p.Locked = false
	}
}

func (p *Pool) Fraction() float64 {
	if p.Max <= 0 {
		return 0
	}
	return p.Cur / p.Max
}

func (p *Pool) CanSprint() bool { return !p.Locked && p.Cur > 0 }
