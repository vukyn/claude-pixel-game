package behavior

import "testing"

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
