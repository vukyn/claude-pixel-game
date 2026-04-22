package player

import "fmt"

func ValidateTuning(p TuningParam, newValue float64) error {
	if newValue < p.MinValue || newValue > p.MaxValue {
		return fmt.Errorf("value out of range: %v not in [%v, %v] %s", newValue, p.MinValue, p.MaxValue, p.Unit)
	}
	return nil
}
