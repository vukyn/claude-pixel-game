package adapter

import "claude-pixel/internal/behavior"

type RuntimeRegistry struct{}

func NewRuntimeRegistry() *RuntimeRegistry { return &RuntimeRegistry{} }

func (RuntimeRegistry) Actions() []behavior.ActionMeta    { return behavior.RegisteredActions() }
func (RuntimeRegistry) Conditions() []behavior.ActionMeta { return behavior.RegisteredConditions() }
