package combat

type Fighter interface {
	Pos() (x, y float64)
	FacingDir() int
	CurrentAnimID() string
	CurrentFrame() int
	Body() Box
	ActiveHits() []Box
	IsInvulnerable() bool
	Alive() bool

	AlreadyHit(target Fighter) bool
	MarkHit(target Fighter)
}

type HitEvent struct {
	Attacker   Fighter
	Victim     Fighter
	AttackKind string
}
