package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
)

// ReloadBehavior re-parses k.BehaviorPath and swaps k.States/InitialState.
// On parse error, k is left untouched and the error is returned so callers
// can log and keep playing.
func ReloadBehavior(k *Kind, lib map[string]*anim.Animation) error {
	if k.BehaviorPath == "" {
		return fmt.Errorf("kind %q has no BehaviorPath", k.Name)
	}
	states, initial, err := LoadBehavior(k.BehaviorPath, k.AnimPrefix+"_", lib)
	if err != nil {
		return err
	}
	k.States = states
	k.InitialState = initial
	return nil
}
