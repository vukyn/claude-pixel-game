package service_test

import (
	"testing"

	"claude-pixel/internal/behavior"
	"claude-pixel/internal/editor/service"
)

type fakeRegistry struct{ a, c []behavior.ActionMeta }

func (f fakeRegistry) Actions() []behavior.ActionMeta    { return f.a }
func (f fakeRegistry) Conditions() []behavior.ActionMeta { return f.c }

func TestRegistryService_ReturnsRegistryContents(t *testing.T) {
	s := service.NewRegistry(fakeRegistry{
		a: []behavior.ActionMeta{{Name: "goto"}},
		c: []behavior.ActionMeta{{Name: "grounded"}},
	})
	if len(s.Actions()) != 1 || s.Actions()[0].Name != "goto" {
		t.Fatal("actions wrong")
	}
	if len(s.Conditions()) != 1 || s.Conditions()[0].Name != "grounded" {
		t.Fatal("conditions wrong")
	}
}
