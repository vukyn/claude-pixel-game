package behavior

import (
	"math/rand"
	"testing"
	"time"
)

type stubEnemy struct {
	facing       int
	vx           float64
	pendingGoto  string
	animDone     bool
	grounded     bool
	currentFrame int
}

func (e *stubEnemy) Facing() int           { return e.facing }
func (e *stubEnemy) SetFacing(f int)       { e.facing = f }
func (e *stubEnemy) SetVX(v float64)       { e.vx = v }
func (e *stubEnemy) CurrentAnimDone() bool { return e.animDone }
func (e *stubEnemy) Grounded() bool        { return e.grounded }
func (e *stubEnemy) CurrentAnimFrame() int { return e.currentFrame }
func (e *stubEnemy) PlayAnim(id string)    {}

func newCtx(e EnemyTarget) *Ctx {
	return &Ctx{Enemy: e, DT: 16 * time.Millisecond, RNG: rand.New(rand.NewSource(1))}
}

func TestStatusStringerCoversAllValues(t *testing.T) {
	cases := []Status{StatusSuccess, StatusFailure, StatusRunning}
	seen := map[string]bool{}
	for _, s := range cases {
		str := s.String()
		if str == "" {
			t.Fatalf("status %d has empty string", s)
		}
		if seen[str] {
			t.Fatalf("duplicate stringer output %q", str)
		}
		seen[str] = true
	}
}

func TestCtxSetBranchAppends(t *testing.T) {
	c := newCtx(&stubEnemy{})
	c.SetBranch("run")
	c.SetBranch("chance#0")
	c.SetBranch("attack")
	if got, want := c.BranchTag, "run/chance#0/attack"; got != want {
		t.Fatalf("BranchTag = %q, want %q", got, want)
	}
}
