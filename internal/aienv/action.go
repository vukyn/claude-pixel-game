package aienv

import "claude-pixel/internal/input"

const NumActions = 10

func ToIntent(action int) input.Intent {
	switch action {
	case 0:
		return input.Intent{}
	case 1:
		return input.Intent{Left: true}
	case 2:
		return input.Intent{Right: true}
	case 3:
		return input.Intent{JumpPressed: true}
	case 4:
		return input.Intent{Left: true, JumpPressed: true}
	case 5:
		return input.Intent{Right: true, JumpPressed: true}
	case 6:
		return input.Intent{AttackPressed: true}
	case 7:
		return input.Intent{Attack2Pressed: true}
	case 8:
		return input.Intent{Left: true, SprintHeld: true}
	case 9:
		return input.Intent{Right: true, SprintHeld: true}
	default:
		return input.Intent{}
	}
}
