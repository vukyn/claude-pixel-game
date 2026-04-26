package behavior

import "fmt"

// ActionFn is a behavior-tree action implementation. Returns the Status the
// surrounding Action node should emit. Errors are construction-time concerns
// (unknown name / bad args) — return an error here to surface at load time.
type ActionFn func(args map[string]any, ctx *Ctx) (Status, error)

// ConditionFn returns a boolean that the surrounding Condition node converts
// into Success/Failure.
type ConditionFn func(args map[string]any, ctx *Ctx) (bool, error)

// actions and conditions are written exclusively during package init() and
// read-only thereafter. Not safe for concurrent modification.
var (
	actions    = map[string]ActionFn{}
	conditions = map[string]ConditionFn{}
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
	RegisterAction("goto", func(args map[string]any, ctx *Ctx) (Status, error) {
		s, err := argString(args, "state")
		if err != nil {
			return StatusFailure, err
		}
		ctx.PendingGoto = s
		return StatusSuccess, nil
	})
	RegisterAction("flip_facing", func(_ map[string]any, ctx *Ctx) (Status, error) {
		ctx.Enemy.SetFacing(-ctx.Enemy.Facing())
		return StatusSuccess, nil
	})
	RegisterAction("randomize_facing", func(_ map[string]any, ctx *Ctx) (Status, error) {
		if ctx.RNG.Intn(2) == 0 {
			ctx.Enemy.SetFacing(1)
		} else {
			ctx.Enemy.SetFacing(-1)
		}
		return StatusSuccess, nil
	})
	RegisterAction("set_vx_forward", func(args map[string]any, ctx *Ctx) (Status, error) {
		speed, err := argFloat(args, "speed")
		if err != nil {
			return StatusFailure, err
		}
		ctx.Enemy.SetVX(float64(ctx.Enemy.Facing()) * speed)
		return StatusSuccess, nil
	})
	RegisterAction("stop", func(_ map[string]any, ctx *Ctx) (Status, error) {
		ctx.Enemy.SetVX(0)
		return StatusSuccess, nil
	})
	RegisterAction("play_anim", func(args map[string]any, ctx *Ctx) (Status, error) {
		key, err := argString(args, "key")
		if err != nil {
			return StatusFailure, err
		}
		ctx.Enemy.PlayAnim(key)
		return StatusSuccess, nil
	})

	RegisterCondition("grounded", func(_ map[string]any, ctx *Ctx) (bool, error) {
		return ctx.Enemy.Grounded(), nil
	})
	RegisterCondition("anim_done", func(_ map[string]any, ctx *Ctx) (bool, error) {
		return ctx.Enemy.CurrentAnimDone(), nil
	})
	RegisterCondition("anim_frame_ge", func(args map[string]any, ctx *Ctx) (bool, error) {
		f, err := argFloat(args, "frame")
		if err != nil {
			return false, err
		}
		return ctx.Enemy.CurrentAnimFrame() >= int(f), nil
	})
	RegisterCondition("anim_frame_le", func(args map[string]any, ctx *Ctx) (bool, error) {
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
