package service

import (
	"encoding/json"
	"fmt"

	"claude-pixel/internal/behavior"
	"claude-pixel/internal/editor/port"
)

// Behavior service wraps a BehaviorStore with validate-then-write semantics.
type Behavior struct{ store port.BehaviorStore }

func NewBehavior(store port.BehaviorStore) *Behavior { return &Behavior{store: store} }

// ValidationError describes a single problem found during behavior JSON validation.
type ValidationError struct {
	Message  string `json:"message"`
	NodePath string `json:"node_path,omitempty"`
}

// ValidationResult is the structured outcome of Validate; json-tagged for HTTP use.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

func (b *Behavior) List() ([]port.BehaviorRef, error) { return b.store.List() }
func (b *Behavior) Get(kind string) ([]byte, error)   { return b.store.Get(kind) }

// Validate checks raw JSON for structural and semantic correctness.
// It pre-checks JSON validity, kind-match, then delegates to behavior.LoadBytes.
func (b *Behavior) Validate(kind string, raw []byte) ValidationResult {
	if !json.Valid(raw) {
		return ValidationResult{Valid: false, Errors: []ValidationError{{Message: "invalid JSON"}}}
	}
	var head struct {
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(raw, &head)
	if head.Kind != kind {
		return ValidationResult{Valid: false, Errors: []ValidationError{{
			Message: fmt.Sprintf("kind mismatch: file %q vs body %q", kind, head.Kind),
		}}}
	}
	if _, err := behavior.LoadBytes(raw, kind+".json"); err != nil {
		return ValidationResult{Valid: false, Errors: []ValidationError{{Message: err.Error()}}}
	}
	return ValidationResult{Valid: true}
}

// Update validates raw JSON then atomically writes it via the store.
// Returns an error if validation fails or the store write fails.
func (b *Behavior) Update(kind string, raw []byte) error {
	res := b.Validate(kind, raw)
	if !res.Valid {
		return fmt.Errorf("validation failed: %s", res.Errors[0].Message)
	}
	return b.store.Put(kind, raw)
}
