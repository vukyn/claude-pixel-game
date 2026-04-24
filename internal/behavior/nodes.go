package behavior

// Selector ticks children left→right. First non-Failure result wins.
type Selector struct {
	Children []Node
}

func (s *Selector) Tick(ctx *Ctx) Status {
	for _, ch := range s.Children {
		st := ch.Tick(ctx)
		if st != StatusFailure {
			return st
		}
	}
	return StatusFailure
}

// Sequence ticks children left→right. First non-Success result wins.
type Sequence struct {
	Children []Node
}

func (s *Sequence) Tick(ctx *Ctx) Status {
	for _, ch := range s.Children {
		st := ch.Tick(ctx)
		if st != StatusSuccess {
			return st
		}
	}
	return StatusSuccess
}
