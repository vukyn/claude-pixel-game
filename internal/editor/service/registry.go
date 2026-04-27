package service

import (
	"claude-pixel/internal/behavior"
	"claude-pixel/internal/editor/port"
)

type Registry struct{ store port.RegistryStore }

func NewRegistry(store port.RegistryStore) *Registry { return &Registry{store: store} }

func (r *Registry) Actions() []behavior.ActionMeta    { return r.store.Actions() }
func (r *Registry) Conditions() []behavior.ActionMeta { return r.store.Conditions() }
