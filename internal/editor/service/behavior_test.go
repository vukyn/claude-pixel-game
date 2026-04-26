package service_test

import (
	"errors"
	"strings"
	"testing"

	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/editor/service"
)

type fakeBehaviorStore struct {
	listFn func() ([]port.BehaviorRef, error)
	getFn  func(string) ([]byte, error)
	putFn  func(string, []byte) error
}

func (f fakeBehaviorStore) List() ([]port.BehaviorRef, error)      { return f.listFn() }
func (f fakeBehaviorStore) Get(k string) ([]byte, error)           { return f.getFn(k) }
func (f fakeBehaviorStore) Put(k string, raw []byte) error         { return f.putFn(k, raw) }

const validOrc = `{"kind":"orc","states":[{"id":"idle","anim":"idle","decision":false,"exit_on":"anim_done","next":"idle"}]}`

func TestBehaviorService_ListPassesThrough(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{
		listFn: func() ([]port.BehaviorRef, error) {
			return []port.BehaviorRef{{Kind: "orc", Path: "/x/orc.json", StateCount: 1}}, nil
		},
	})
	refs, err := s.List()
	if err != nil || len(refs) != 1 || refs[0].Kind != "orc" {
		t.Fatalf("unexpected: %+v %v", refs, err)
	}
}

func TestBehaviorService_GetPassesThrough(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{
		getFn: func(k string) ([]byte, error) { return []byte(validOrc), nil },
	})
	got, err := s.Get("orc")
	if err != nil || string(got) != validOrc {
		t.Fatalf("unexpected: %s %v", got, err)
	}
}

func TestBehaviorService_ValidateOK(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{})
	res := s.Validate("orc", []byte(validOrc))
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %+v", res.Errors)
	}
}

func TestBehaviorService_ValidateFailsOnUnknownAction(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{})
	bad := `{"kind":"orc","states":[{"id":"a","anim":"idle","decision":true,"bt":{"type":"action","name":"do_evil","args":{}}}]}`
	res := s.Validate("orc", []byte(bad))
	if res.Valid {
		t.Fatal("expected invalid")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors")
	}
}

func TestBehaviorService_ValidateRejectsKindMismatch(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{})
	res := s.Validate("slime", []byte(validOrc))
	if res.Valid {
		t.Fatal("expected invalid (kind mismatch)")
	}
	if !strings.Contains(strings.Join(errorMessages(res.Errors), " "), "kind") {
		t.Fatalf("expected kind-mismatch error, got %+v", res.Errors)
	}
}

func TestBehaviorService_UpdateValidatesBeforeWrite(t *testing.T) {
	called := false
	s := service.NewBehavior(fakeBehaviorStore{
		putFn: func(k string, raw []byte) error { called = true; return nil },
	})
	bad := `{"kind":"orc","states":[]}`
	if err := s.Update("orc", []byte(bad)); err == nil {
		t.Fatal("expected validation error")
	}
	if called {
		t.Fatal("Put should not be called when validation fails")
	}
}

func TestBehaviorService_UpdateWritesOnSuccess(t *testing.T) {
	written := []byte(nil)
	s := service.NewBehavior(fakeBehaviorStore{
		putFn: func(k string, raw []byte) error { written = raw; return nil },
	})
	if err := s.Update("orc", []byte(validOrc)); err != nil {
		t.Fatal(err)
	}
	if string(written) != validOrc {
		t.Fatalf("write payload mismatch: %s", written)
	}
}

func TestBehaviorService_UpdatePropagatesStoreError(t *testing.T) {
	s := service.NewBehavior(fakeBehaviorStore{
		putFn: func(k string, raw []byte) error { return errors.New("disk full") },
	})
	err := s.Update("orc", []byte(validOrc))
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("expected disk full error, got %v", err)
	}
}

func errorMessages(errs []service.ValidationError) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Message)
	}
	return out
}
