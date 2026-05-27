package aienv

import (
	"testing"
)

func TestOrcNumActions(t *testing.T) {
	if OrcNumActions != 6 {
		t.Errorf("OrcNumActions = %d, want 6", OrcNumActions)
	}
}

func TestOrcActionIdle(t *testing.T) {
	a := OrcAction(0)
	if a.VXMode != OrcVXStop {
		t.Errorf("action 0: VXMode = %d, want OrcVXStop", a.VXMode)
	}
	if a.Transition != "" {
		t.Errorf("action 0: Transition = %q, want empty", a.Transition)
	}
}

func TestOrcActionToward(t *testing.T) {
	a := OrcAction(1)
	if a.VXMode != OrcVXToward {
		t.Errorf("action 1: VXMode = %d, want OrcVXToward", a.VXMode)
	}
}

func TestOrcActionAway(t *testing.T) {
	a := OrcAction(2)
	if a.VXMode != OrcVXAway {
		t.Errorf("action 2: VXMode = %d, want OrcVXAway", a.VXMode)
	}
}

func TestOrcActionAttack1(t *testing.T) {
	a := OrcAction(3)
	if a.Transition != "attack" {
		t.Errorf("action 3: Transition = %q, want attack", a.Transition)
	}
}

func TestOrcActionAttack2(t *testing.T) {
	a := OrcAction(4)
	if a.Transition != "attack2" {
		t.Errorf("action 4: Transition = %q, want attack2", a.Transition)
	}
}

func TestOrcActionFlip(t *testing.T) {
	a := OrcAction(5)
	if a.VXMode != OrcVXStop {
		t.Errorf("action 5: VXMode = %d, want OrcVXStop", a.VXMode)
	}
	if !a.Flip {
		t.Error("action 5: Flip should be true")
	}
}

func TestOrcActionOutOfRange(t *testing.T) {
	a := OrcAction(99)
	if a.VXMode != OrcVXStop {
		t.Errorf("out of range: VXMode = %d, want OrcVXStop", a.VXMode)
	}
}
