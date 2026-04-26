package service

import "claude-pixel/internal/editor/port"

type Tuning struct{ store port.TuningStore }

func NewTuning(store port.TuningStore) *Tuning { return &Tuning{store: store} }

func (t *Tuning) List(prefix string) ([]port.TuningRow, error) { return t.store.List(prefix) }

func (t *Tuning) Update(key string, value float64) (float64, error) {
	return t.store.Update(key, value)
}
