package player

import "claude-pixel/internal/storage"

type TuningParam struct {
	Key         string
	Value       float64
	MinValue    float64
	MaxValue    float64
	Unit        string
	Description string
}

func (t TuningParam) GetID() string { return t.Key }

type TuningMapper struct{}

func (TuningMapper) Table() string { return "tuning" }

func (TuningMapper) Columns() []string {
	return []string{"key", "value", "min_value", "max_value", "unit", "description"}
}

func (TuningMapper) Scan(row storage.Scanner) (TuningParam, error) {
	var p TuningParam
	err := row.Scan(&p.Key, &p.Value, &p.MinValue, &p.MaxValue, &p.Unit, &p.Description)
	return p, err
}

func (TuningMapper) Values(p TuningParam) []any {
	return []any{p.Key, p.Value, p.MinValue, p.MaxValue, p.Unit, p.Description}
}
