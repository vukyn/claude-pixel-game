package adapter_test

import (
	"testing"

	"claude-pixel/internal/editor/adapter"
)

func TestRuntimeRegistry_ExposesActions(t *testing.T) {
	r := adapter.NewRuntimeRegistry()
	got := r.Actions()
	if len(got) == 0 { t.Fatal("expected actions") }
	for _, a := range got {
		if a.Name == "" { t.Fatalf("action with empty name: %+v", a) }
	}
}

func TestRuntimeRegistry_ExposesConditions(t *testing.T) {
	r := adapter.NewRuntimeRegistry()
	if len(r.Conditions()) == 0 { t.Fatal("expected conditions") }
}
