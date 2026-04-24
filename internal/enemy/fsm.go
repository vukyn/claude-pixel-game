package enemy

import (
	"time"

	"claude-pixel/internal/behavior"
	"claude-pixel/internal/combat"
)

// Tick runs one frame of the generic FSM driver. Priority:
//  1. Engine-owned event transitions (hit, death, fall→run on grounded)
//  2. Anim advance + on_frame_vx
//  3. Non-decision states: exit_on rule → on_exit_actions → transition
//  4. Decision states: BT tick, honor PendingGoto
func (e *Enemy) Tick(dt time.Duration) {
	// 1. Event transitions (bypass BT).
	if e.Lives <= 0 && e.CurrentState != "death" {
		e.transition("death")
		return
	}
	if e.OnHitPending {
		e.OnHitPending = false
		e.transition("hurt")
		return
	}
	if e.CurrentState == "fall" && e.Grounded {
		e.runOnExitActionsFor(e.states[e.CurrentState])
		e.transition("run")
		return
	}

	st := e.states[e.CurrentState]
	if st == nil {
		return
	}

	// 2. Anim advance.
	if st.Anim != nil {
		st.Anim.Update(dt)
	}
	// Per-frame VX slide.
	frame := e.currentFrameIndex()
	if vx, ok := currentFrameVX(st, frame); ok {
		e.VX = float64(e.Facing) * vx
	}

	// 3. Non-decision state: exit_on rule.
	if !st.Decision {
		if exitRuleMet(e, st) {
			e.runOnExitActionsFor(st)
			if st.Next == "__dead" {
				e.Dead = true
				return
			}
			if st.Next != "" {
				e.transition(st.Next)
			}
		}
		return
	}

	// 4. Decision state: BT.
	if st.BT == nil || st.BT.Root == nil {
		return
	}
	ctx := &behavior.Ctx{Enemy: enemyAdapter{e: e}, DT: dt, RNG: e.rng}
	st.BT.Root.Tick(ctx)
	e.BranchTag = ctx.BranchTag
	if ctx.PendingGoto != "" && ctx.PendingGoto != e.CurrentState {
		e.transition(ctx.PendingGoto)
	}
}

func (e *Enemy) transition(to string) {
	st, ok := e.states[to]
	if !ok {
		return
	}
	e.CurrentState = to
	e.HitSet = map[combat.Fighter]bool{}
	if st.Anim != nil {
		st.Anim.Reset()
		e.Current = st.Anim
		e.CurrentAnim = st.AnimKey
	}
	e.VX = 0
}

func (e *Enemy) runOnExitActionsFor(st *StateDecl) {
	if st == nil || len(st.OnExitActions) == 0 {
		return
	}
	ctx := &behavior.Ctx{Enemy: enemyAdapter{e: e}, RNG: e.rng}
	for _, name := range st.OnExitActions {
		_, _ = behavior.RunAction(name, nil, ctx)
	}
}

func (e *Enemy) currentFrameIndex() int {
	if e.Current == nil {
		return 0
	}
	return e.Current.FrameIndex()
}

func exitRuleMet(e *Enemy, st *StateDecl) bool {
	switch st.ExitOn {
	case "anim_done":
		return e.Current != nil && e.Current.Done()
	case "anim_done_and_grounded":
		return e.Current != nil && e.Current.Done() && e.Grounded
	case "grounded":
		return e.Grounded
	}
	return false
}

func currentFrameVX(st *StateDecl, frame int) (float64, bool) {
	for _, fv := range st.OnFrameVX {
		if frame >= fv.FrameStart && frame <= fv.FrameEnd {
			return fv.VX, true
		}
	}
	return 0, false
}
