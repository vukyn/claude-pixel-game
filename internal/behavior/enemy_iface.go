package behavior

// EnemyTarget is the minimal surface the behavior runtime needs on an enemy.
// It lives here (not in internal/enemy) to keep the behavior package
// independent of enemy internals and to avoid import cycles.
type EnemyTarget interface {
	Facing() int
	SetFacing(int)
	SetVX(float64)
	CurrentAnimDone() bool
	Grounded() bool
	CurrentAnimFrame() int
	PlayAnim(id string)
}
