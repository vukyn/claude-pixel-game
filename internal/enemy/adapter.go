package enemy

// enemyAdapter bridges *Enemy into the behavior.EnemyTarget interface so
// the behavior package can mutate enemies without importing internal/enemy.
type enemyAdapter struct{ e *Enemy }

func (a enemyAdapter) Facing() int        { return a.e.Facing }
func (a enemyAdapter) SetFacing(f int)    { a.e.Facing = f }
func (a enemyAdapter) SetVX(v float64)    { a.e.VX = v }
func (a enemyAdapter) Grounded() bool     { return a.e.Grounded }
func (a enemyAdapter) PlayAnim(id string) { a.e.PlayAnim(id) }
func (a enemyAdapter) CurrentAnimDone() bool {
	return a.e.Current != nil && a.e.Current.Done()
}
func (a enemyAdapter) CurrentAnimFrame() int {
	if a.e.Current == nil {
		return 0
	}
	return a.e.Current.FrameIndex()
}
