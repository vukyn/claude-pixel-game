package adapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

type SQLiteTuning struct{ repo *storage.Repository[player.TuningParam] }

func NewSQLiteTuning(repo *storage.Repository[player.TuningParam]) *SQLiteTuning {
	return &SQLiteTuning{repo: repo}
}

func (s *SQLiteTuning) List(prefix string) ([]port.TuningRow, error) {
	all, err := s.repo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("sqlitetuning: list: %w", err)
	}
	out := make([]port.TuningRow, 0, len(all))
	for _, t := range all {
		if prefix != "" && !strings.HasPrefix(t.Key, prefix+"_") {
			continue
		}
		out = append(out, port.TuningRow{
			Key:         t.Key,
			Value:       t.Value,
			Min:         t.MinValue,
			Max:         t.MaxValue,
			Unit:        t.Unit,
			Description: t.Description,
		})
	}
	return out, nil
}

func (s *SQLiteTuning) Update(key string, value float64) (float64, error) {
	current, err := s.repo.Get(context.Background(), key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("sqlitetuning: unknown key %q", key)
		}
		return 0, fmt.Errorf("sqlitetuning: get %q: %w", key, err)
	}
	if value < current.MinValue || value > current.MaxValue {
		return current.Value, fmt.Errorf("sqlitetuning: value out of range: %v not in [%v, %v] %s",
			value, current.MinValue, current.MaxValue, current.Unit)
	}
	old := current.Value
	current.Value = value
	if err := s.repo.Upsert(context.Background(), current); err != nil {
		return old, fmt.Errorf("sqlitetuning: upsert: %w", err)
	}
	return old, nil
}
