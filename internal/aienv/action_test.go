package aienv

import (
	"testing"

	"claude-pixel/internal/input"
)

func TestToIntent_Idle(t *testing.T) {
	intent := ToIntent(0)
	want := input.Intent{}
	if intent != want {
		t.Errorf("action 0: got %+v, want %+v", intent, want)
	}
}

func TestToIntent_MoveLeft(t *testing.T) {
	intent := ToIntent(1)
	if !intent.Left || intent.Right {
		t.Errorf("action 1: got Left=%v Right=%v", intent.Left, intent.Right)
	}
}

func TestToIntent_MoveRight(t *testing.T) {
	intent := ToIntent(2)
	if intent.Left || !intent.Right {
		t.Errorf("action 2: got Left=%v Right=%v", intent.Left, intent.Right)
	}
}

func TestToIntent_Jump(t *testing.T) {
	intent := ToIntent(3)
	if !intent.JumpPressed {
		t.Errorf("action 3: JumpPressed should be true")
	}
}

func TestToIntent_MoveLeftJump(t *testing.T) {
	intent := ToIntent(4)
	if !intent.Left || !intent.JumpPressed {
		t.Errorf("action 4: Left=%v JumpPressed=%v", intent.Left, intent.JumpPressed)
	}
}

func TestToIntent_MoveRightJump(t *testing.T) {
	intent := ToIntent(5)
	if !intent.Right || !intent.JumpPressed {
		t.Errorf("action 5: Right=%v JumpPressed=%v", intent.Right, intent.JumpPressed)
	}
}

func TestToIntent_Attack1(t *testing.T) {
	intent := ToIntent(6)
	if !intent.AttackPressed {
		t.Errorf("action 6: AttackPressed should be true")
	}
}

func TestToIntent_Attack2(t *testing.T) {
	intent := ToIntent(7)
	if !intent.Attack2Pressed {
		t.Errorf("action 7: Attack2Pressed should be true")
	}
}

func TestToIntent_SprintLeft(t *testing.T) {
	intent := ToIntent(8)
	if !intent.Left || !intent.SprintHeld {
		t.Errorf("action 8: Left=%v SprintHeld=%v", intent.Left, intent.SprintHeld)
	}
}

func TestToIntent_SprintRight(t *testing.T) {
	intent := ToIntent(9)
	if !intent.Right || !intent.SprintHeld {
		t.Errorf("action 9: Right=%v SprintHeld=%v", intent.Right, intent.SprintHeld)
	}
}

func TestToIntent_OutOfRange(t *testing.T) {
	intent := ToIntent(99)
	want := input.Intent{}
	if intent != want {
		t.Errorf("out of range action: got %+v, want idle", intent)
	}
}

func TestNumActions(t *testing.T) {
	if NumActions != 10 {
		t.Errorf("NumActions = %d, want 10", NumActions)
	}
}
