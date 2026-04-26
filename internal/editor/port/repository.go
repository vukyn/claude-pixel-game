package port

import "claude-pixel/internal/behavior"

// BehaviorStore owns persistence of behavior JSON files.
type BehaviorStore interface {
	List() ([]BehaviorRef, error)
	Get(kind string) ([]byte, error)         // raw JSON bytes
	Put(kind string, raw []byte) error       // atomic write
}

type BehaviorRef struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	StateCount int    `json:"state_count"`
}

// TuningStore owns tuning rows with range validation.
type TuningStore interface {
	List(prefix string) ([]TuningRow, error)
	Update(key string, value float64) (oldValue float64, err error)
}

type TuningRow struct {
	Key         string  `json:"key"`
	Value       float64 `json:"value"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Unit        string  `json:"unit"`
	Description string  `json:"description"`
}

// RegistryStore exposes the behavior package's action/condition catalog.
type RegistryStore interface {
	Actions() []behavior.ActionMeta
	Conditions() []behavior.ActionMeta
}
