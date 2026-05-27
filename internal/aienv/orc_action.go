package aienv

const OrcNumActions = 6

type OrcVXMode int

const (
	OrcVXStop   OrcVXMode = iota
	OrcVXToward
	OrcVXAway
)

type OrcActionResult struct {
	VXMode     OrcVXMode
	Transition string
	Flip       bool
}

func OrcAction(action int) OrcActionResult {
	switch action {
	case 0:
		return OrcActionResult{VXMode: OrcVXStop}
	case 1:
		return OrcActionResult{VXMode: OrcVXToward}
	case 2:
		return OrcActionResult{VXMode: OrcVXAway}
	case 3:
		return OrcActionResult{Transition: "attack"}
	case 4:
		return OrcActionResult{Transition: "attack2"}
	case 5:
		return OrcActionResult{VXMode: OrcVXStop, Flip: true}
	default:
		return OrcActionResult{VXMode: OrcVXStop}
	}
}
