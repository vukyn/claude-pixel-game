package service_test

import (
	"errors"
	"testing"

	"claude-pixel/internal/editor/port"
	"claude-pixel/internal/editor/service"
)

type fakeTuningStore struct {
	rows     []port.TuningRow
	updateFn func(key string, value float64) (float64, error)
}

func (f fakeTuningStore) List(prefix string) ([]port.TuningRow, error) {
	var out []port.TuningRow
	for _, r := range f.rows {
		if prefix == "" || (len(r.Key) > len(prefix) && r.Key[:len(prefix)+1] == prefix+"_") {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f fakeTuningStore) Update(key string, value float64) (float64, error) {
	return f.updateFn(key, value)
}

func TestTuningService_List(t *testing.T) {
	s := service.NewTuning(fakeTuningStore{rows: []port.TuningRow{
		{Key: "orc_max_lives", Value: 2, Min: 1, Max: 10},
		{Key: "slime_max_lives", Value: 2},
	}})
	rows, err := s.List("orc")
	if err != nil || len(rows) != 1 || rows[0].Key != "orc_max_lives" {
		t.Fatalf("unexpected: %+v %v", rows, err)
	}
}

func TestTuningService_UpdateOK(t *testing.T) {
	s := service.NewTuning(fakeTuningStore{updateFn: func(k string, v float64) (float64, error) {
		return 2, nil
	}})
	old, err := s.Update("orc_max_lives", 5)
	if err != nil || old != 2 {
		t.Fatalf("unexpected: old=%v err=%v", old, err)
	}
}

func TestTuningService_UpdateError(t *testing.T) {
	s := service.NewTuning(fakeTuningStore{updateFn: func(k string, v float64) (float64, error) {
		return 0, errors.New("out of range")
	}})
	if _, err := s.Update("orc_max_lives", 999); err == nil {
		t.Fatal("expected error")
	}
}
