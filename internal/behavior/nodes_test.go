package behavior

import (
	"math/rand"
	"testing"
	"time"
)

type fakeNode struct {
	out    Status
	called int
}

func (f *fakeNode) Tick(*Ctx) Status {
	f.called++
	return f.out
}

func TestSelectorReturnsFirstSuccess(t *testing.T) {
	a := &fakeNode{out: StatusFailure}
	b := &fakeNode{out: StatusSuccess}
	c := &fakeNode{out: StatusSuccess}
	sel := &Selector{Children: []Node{a, b, c}}
	if got := sel.Tick(newCtx(&stubEnemy{})); got != StatusSuccess {
		t.Fatalf("Tick = %v, want success", got)
	}
	if a.called != 1 || b.called != 1 || c.called != 0 {
		t.Fatalf("call counts: a=%d b=%d c=%d", a.called, b.called, c.called)
	}
}

func TestSelectorPropagatesRunning(t *testing.T) {
	sel := &Selector{Children: []Node{
		&fakeNode{out: StatusFailure},
		&fakeNode{out: StatusRunning},
		&fakeNode{out: StatusSuccess},
	}}
	if got := sel.Tick(newCtx(&stubEnemy{})); got != StatusRunning {
		t.Fatalf("Tick = %v, want running", got)
	}
}

func TestSelectorAllFailureReturnsFailure(t *testing.T) {
	sel := &Selector{Children: []Node{
		&fakeNode{out: StatusFailure},
		&fakeNode{out: StatusFailure},
	}}
	if got := sel.Tick(newCtx(&stubEnemy{})); got != StatusFailure {
		t.Fatalf("Tick = %v, want failure", got)
	}
}

func TestSequenceReturnsFirstFailure(t *testing.T) {
	a := &fakeNode{out: StatusSuccess}
	b := &fakeNode{out: StatusFailure}
	c := &fakeNode{out: StatusSuccess}
	seq := &Sequence{Children: []Node{a, b, c}}
	if got := seq.Tick(newCtx(&stubEnemy{})); got != StatusFailure {
		t.Fatalf("Tick = %v, want failure", got)
	}
	if a.called != 1 || b.called != 1 || c.called != 0 {
		t.Fatalf("call counts: a=%d b=%d c=%d", a.called, b.called, c.called)
	}
}

func TestSequenceAllSuccessReturnsSuccess(t *testing.T) {
	seq := &Sequence{Children: []Node{
		&fakeNode{out: StatusSuccess},
		&fakeNode{out: StatusSuccess},
	}}
	if got := seq.Tick(newCtx(&stubEnemy{})); got != StatusSuccess {
		t.Fatalf("Tick = %v, want success", got)
	}
}

func TestSequencePropagatesRunning(t *testing.T) {
	seq := &Sequence{Children: []Node{
		&fakeNode{out: StatusSuccess},
		&fakeNode{out: StatusRunning},
		&fakeNode{out: StatusSuccess},
	}}
	if got := seq.Tick(newCtx(&stubEnemy{})); got != StatusRunning {
		t.Fatalf("Tick = %v, want running", got)
	}
}

func TestChancePicksBranchUsingWeights(t *testing.T) {
	// Seed produces deterministic sequence. With rand.New(rand.NewSource(1)),
	// first Intn(100) = 81. Weights [30,70] → cumulative [30,100]; 81 falls
	// in the second branch.
	winner := &fakeNode{out: StatusSuccess}
	loser := &fakeNode{out: StatusSuccess}
	ch := &Chance{Branches: []ChanceBranch{
		{Weight: 30, Node: loser},
		{Weight: 70, Node: winner},
	}}
	ctx := &Ctx{RNG: rand.New(rand.NewSource(1))}
	if got := ch.Tick(ctx); got != StatusSuccess {
		t.Fatalf("Tick = %v", got)
	}
	if loser.called != 0 || winner.called != 1 {
		t.Fatalf("branch calls: loser=%d winner=%d", loser.called, winner.called)
	}
}

func TestChanceStickyWhileRunning(t *testing.T) {
	running := &fakeNode{out: StatusRunning}
	other := &fakeNode{out: StatusSuccess}
	ch := &Chance{Branches: []ChanceBranch{
		{Weight: 100, Node: running},
		{Weight: 1, Node: other},
	}}
	ctx := &Ctx{RNG: rand.New(rand.NewSource(1))}
	// First tick rolls; branch 0 (running) selected — weight 100 vs 1.
	ch.Tick(ctx)
	ch.Tick(ctx)
	if running.called != 2 || other.called != 0 {
		t.Fatalf("sticky tick2: running=%d other=%d, want 2,0", running.called, other.called)
	}
	// Flip to Success; this tick resolves and clears active.
	running.out = StatusSuccess
	ch.Tick(ctx)
	if running.called != 3 {
		t.Fatalf("after resolve: running.called=%d, want 3", running.called)
	}
	// Next tick re-rolls — with weights 100/1 and the running seed, running
	// is overwhelmingly likely to be picked again. Key check: ONE of them
	// was called, i.e., a new Tick fired on a freshly-picked branch.
	before := running.called + other.called
	ch.Tick(ctx)
	if running.called+other.called != before+1 {
		t.Fatalf("expected reroll to tick exactly one branch; diff=%d",
			running.called+other.called-before)
	}
}

func TestChanceAllZeroWeightsDoesNotPanic(t *testing.T) {
	node := &fakeNode{out: StatusSuccess}
	ch := &Chance{Branches: []ChanceBranch{
		{Weight: 0, Node: node},
	}}
	ctx := &Ctx{RNG: rand.New(rand.NewSource(1))}
	// Must not panic; guard returns len(branches)-1 = 0, so node is ticked.
	got := ch.Tick(ctx)
	if got != StatusSuccess {
		t.Fatalf("Tick = %v, want success", got)
	}
	if node.called != 1 {
		t.Fatalf("node.called = %d, want 1", node.called)
	}
}

func TestWaitReturnsRunningUntilElapsed(t *testing.T) {
	w := &Wait{Seconds: 1.0}
	ctx := &Ctx{DT: 400 * time.Millisecond}
	if got := w.Tick(ctx); got != StatusRunning {
		t.Fatalf("first tick = %v, want running", got)
	}
	if got := w.Tick(ctx); got != StatusRunning {
		t.Fatalf("second tick = %v, want running", got)
	}
	if got := w.Tick(ctx); got != StatusSuccess {
		t.Fatalf("third tick = %v, want success", got)
	}
	// After success, re-ticking restarts the timer.
	if got := w.Tick(ctx); got != StatusRunning {
		t.Fatalf("re-tick = %v, want running (restart)", got)
	}
}
