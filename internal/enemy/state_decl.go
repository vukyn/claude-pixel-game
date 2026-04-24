package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/behavior"
)

// StateDecl is the enemy-side view of a behavior file state. Anim pointer
// is resolved against the kind's animation library; BT is the parsed tree
// for decision states (nil otherwise).
type StateDecl struct {
	ID            string
	Anim          *anim.Animation
	AnimKey       string
	Decision      bool
	BT            *behavior.Tree
	ExitOn        string
	Next          string
	OnExitActions []string
	OnFrameVX     []behavior.FrameVX
}

// ConvertStates turns the generic behavior.State list into enemy StateDecls
// keyed by ID. Fails if an anim key is not present in lib.
func ConvertStates(bStates []behavior.State, lib map[string]*anim.Animation) (map[string]*StateDecl, error) {
	out := make(map[string]*StateDecl, len(bStates))
	for _, s := range bStates {
		a, ok := lib[s.Anim]
		if !ok {
			return nil, fmt.Errorf("state %q: missing anim %q in library", s.ID, s.Anim)
		}
		out[s.ID] = &StateDecl{
			ID:            s.ID,
			Anim:          a,
			AnimKey:       s.Anim,
			Decision:      s.Decision,
			BT:            s.BT,
			ExitOn:        s.ExitOn,
			Next:          s.Next,
			OnExitActions: s.OnExitActions,
			OnFrameVX:     s.OnFrameVX,
		}
	}
	return out, nil
}
