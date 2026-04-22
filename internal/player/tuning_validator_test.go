package player

import "testing"

func TestValidateTuningInRange(t *testing.T) {
	p := TuningParam{Key: "x", MinValue: 0, MaxValue: 10, Unit: "px"}
	if err := ValidateTuning(p, 5); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTuningBelowMin(t *testing.T) {
	p := TuningParam{Key: "x", MinValue: 0, MaxValue: 10}
	if err := ValidateTuning(p, -1); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateTuningAboveMax(t *testing.T) {
	p := TuningParam{Key: "x", MinValue: 0, MaxValue: 10}
	if err := ValidateTuning(p, 11); err == nil {
		t.Fatal("expected error")
	}
}
