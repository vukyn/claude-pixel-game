package behavior

import (
	"fmt"
	"sort"
)

// ActionFn is a behavior-tree action implementation. Returns the Status the
// surrounding Action node should emit. Errors are construction-time concerns
// (unknown name / bad args) — return an error here to surface at load time.
type ActionFn func(args map[string]any, ctx *Ctx) (Status, error)

// ConditionFn returns a boolean that the surrounding Condition node converts
// into Success/Failure.
type ConditionFn func(args map[string]any, ctx *Ctx) (bool, error)

// ArgMeta describes one argument to an action or condition.
type ArgMeta struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "int" | "float" | "string" | "state_id" | "anim_key"
	Required bool   `json:"required"`
}

// ActionMeta describes a registered action or condition for editor introspection.
type ActionMeta struct {
	Name string    `json:"name"`
	Args []ArgMeta `json:"args"`
}

// actions and conditions are written exclusively during package init() and
// read-only thereafter. Not safe for concurrent modification.
var (
	actions       = map[string]ActionFn{}
	conditions    = map[string]ConditionFn{}
	actionMetas   = map[string]ActionMeta{}
	conditionMeta = map[string]ActionMeta{}
)

// RegisterAction registers fn under name. Panics on duplicate name to surface
// init-time mis-wiring loudly. Call from package init() only.
func RegisterAction(name string, fn ActionFn) {
	if _, exists := actions[name]; exists {
		panic("behavior: duplicate action registration: " + name)
	}
	actions[name] = fn
}

// RegisterCondition registers fn under name. Panics on duplicate name.
// Call from package init() only.
func RegisterCondition(name string, fn ConditionFn) {
	if _, exists := conditions[name]; exists {
		panic("behavior: duplicate condition registration: " + name)
	}
	conditions[name] = fn
}

// RegisterActionWithMeta registers fn under name and records its arg schema for editor introspection.
func RegisterActionWithMeta(name string, args []ArgMeta, fn ActionFn) {
	RegisterAction(name, fn)
	actionMetas[name] = ActionMeta{Name: name, Args: args}
}

// RegisterConditionWithMeta registers fn under name and records its arg schema for editor introspection.
func RegisterConditionWithMeta(name string, args []ArgMeta, fn ConditionFn) {
	RegisterCondition(name, fn)
	conditionMeta[name] = ActionMeta{Name: name, Args: args}
}

// RegisteredActions returns metadata for every registered action, sorted by name.
func RegisteredActions() []ActionMeta { return sortedMetas(actionMetas) }

// RegisteredConditions returns metadata for every registered condition, sorted by name.
func RegisteredConditions() []ActionMeta { return sortedMetas(conditionMeta) }

func sortedMetas(m map[string]ActionMeta) []ActionMeta {
	out := make([]ActionMeta, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// HasAction reports whether name is registered. Used by the loader to
// reject unknown actions at parse time.
func HasAction(name string) bool {
	_, ok := actions[name]
	return ok
}

// HasCondition reports whether name is registered.
func HasCondition(name string) bool {
	_, ok := conditions[name]
	return ok
}

// RunAction executes a named action. Returns an error if the name isn't
// registered or the action itself fails.
func RunAction(name string, args map[string]any, ctx *Ctx) (Status, error) {
	fn, ok := actions[name]
	if !ok {
		return StatusFailure, fmt.Errorf("behavior: unknown action %q", name)
	}
	return fn(args, ctx)
}

// RunCondition executes a named condition.
func RunCondition(name string, args map[string]any, ctx *Ctx) (bool, error) {
	fn, ok := conditions[name]
	if !ok {
		return false, fmt.Errorf("behavior: unknown condition %q", name)
	}
	return fn(args, ctx)
}

func init() {
	RegisterActionWithMeta("goto", []ArgMeta{{Name: "state", Type: "state_id", Required: true}}, func(args map[string]any, ctx *Ctx) (Status, error) {
		s, err := argString(args, "state")
		if err != nil {
			return StatusFailure, err
		}
		ctx.PendingGoto = s
		return StatusSuccess, nil
	})
	RegisterActionWithMeta("flip_facing", nil, func(_ map[string]any, ctx *Ctx) (Status, error) {
		ctx.Enemy.SetFacing(-ctx.Enemy.Facing())
		return StatusSuccess, nil
	})
	RegisterActionWithMeta("randomize_facing", nil, func(_ map[string]any, ctx *Ctx) (Status, error) {
		if ctx.RNG.Intn(2) == 0 {
			ctx.Enemy.SetFacing(1)
		} else {
			ctx.Enemy.SetFacing(-1)
		}
		return StatusSuccess, nil
	})
	RegisterActionWithMeta("set_vx_forward", []ArgMeta{{Name: "speed", Type: "float", Required: true}}, func(args map[string]any, ctx *Ctx) (Status, error) {
		speed, err := argFloat(args, "speed")
		if err != nil {
			return StatusFailure, err
		}
		ctx.Enemy.SetVX(float64(ctx.Enemy.Facing()) * speed)
		return StatusSuccess, nil
	})
	RegisterActionWithMeta("stop", nil, func(_ map[string]any, ctx *Ctx) (Status, error) {
		ctx.Enemy.SetVX(0)
		return StatusSuccess, nil
	})
	RegisterActionWithMeta("play_anim", []ArgMeta{{Name: "key", Type: "anim_key", Required: true}}, func(args map[string]any, ctx *Ctx) (Status, error) {
		key, err := argString(args, "key")
		if err != nil {
			return StatusFailure, err
		}
		ctx.Enemy.PlayAnim(key)
		return StatusSuccess, nil
	})

	RegisterConditionWithMeta("grounded", nil, func(_ map[string]any, ctx *Ctx) (bool, error) {
		return ctx.Enemy.Grounded(), nil
	})
	RegisterConditionWithMeta("anim_done", nil, func(_ map[string]any, ctx *Ctx) (bool, error) {
		return ctx.Enemy.CurrentAnimDone(), nil
	})
	RegisterConditionWithMeta("anim_frame_ge", []ArgMeta{{Name: "frame", Type: "int", Required: true}}, func(args map[string]any, ctx *Ctx) (bool, error) {
		f, err := argFloat(args, "frame")
		if err != nil {
			return false, err
		}
		return ctx.Enemy.CurrentAnimFrame() >= int(f), nil
	})
	RegisterConditionWithMeta("anim_frame_le", []ArgMeta{{Name: "frame", Type: "int", Required: true}}, func(args map[string]any, ctx *Ctx) (bool, error) {
		f, err := argFloat(args, "frame")
		if err != nil {
			return false, err
		}
		return ctx.Enemy.CurrentAnimFrame() <= int(f), nil
	})
}

func argString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing arg %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("arg %q must be string, got %T", key, v)
	}
	return s, nil
}

func argFloat(args map[string]any, key string) (float64, error) {
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("missing arg %q", key)
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	}
	return 0, fmt.Errorf("arg %q must be number, got %T", key, v)
}
