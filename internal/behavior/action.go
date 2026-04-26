package behavior

// Action is a leaf node that invokes a registered action by name. The
// loader validates the name exists at parse time, so tick-time errors
// here indicate a registry mutation after load — rare, but surfaced as
// Failure to keep tick paths lenient.
type Action struct {
	Name string
	Args map[string]any
}

func (a *Action) Tick(ctx *Ctx) Status {
	st, err := RunAction(a.Name, a.Args, ctx)
	if err != nil {
		return StatusFailure
	}
	return st
}

// Condition wraps a registered condition. True → Success, false → Failure.
type Condition struct {
	Name string
	Args map[string]any
}

func (c *Condition) Tick(ctx *Ctx) Status {
	ok, err := RunCondition(c.Name, c.Args, ctx)
	if err != nil || !ok {
		return StatusFailure
	}
	return StatusSuccess
}
