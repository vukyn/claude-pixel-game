package behavior

import (
	"math/rand"
	"testing"
)

func TestBuiltinGotoSetsPendingGoto(t *testing.T) {
	ctx := newCtx(&stubEnemy{})
	st, err := RunAction("goto", map[string]any{"state": "run"}, ctx)
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if st != StatusSuccess {
		t.Fatalf("status = %v", st)
	}
	if ctx.PendingGoto != "run" {
		t.Fatalf("PendingGoto = %q", ctx.PendingGoto)
	}
}

func TestBuiltinFlipFacing(t *testing.T) {
	e := &stubEnemy{facing: 1}
	ctx := newCtx(e)
	if _, err := RunAction("flip_facing", nil, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if e.facing != -1 {
		t.Fatalf("facing = %d, want -1", e.facing)
	}
}

func TestBuiltinRandomizeFacing(t *testing.T) {
	e := &stubEnemy{facing: 0}
	ctx := &Ctx{Enemy: e, RNG: rand.New(rand.NewSource(1))}
	if _, err := RunAction("randomize_facing", nil, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if e.facing != 1 && e.facing != -1 {
		t.Fatalf("facing = %d, want ±1", e.facing)
	}
}

func TestBuiltinSetVXForwardUsesFacing(t *testing.T) {
	e := &stubEnemy{facing: -1}
	ctx := newCtx(e)
	if _, err := RunAction("set_vx_forward", map[string]any{"speed": 80.0}, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if e.vx != -80 {
		t.Fatalf("vx = %f, want -80", e.vx)
	}
}

func TestBuiltinStopZeroesVX(t *testing.T) {
	e := &stubEnemy{vx: 120}
	ctx := newCtx(e)
	if _, err := RunAction("stop", nil, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if e.vx != 0 {
		t.Fatalf("vx = %f, want 0", e.vx)
	}
}

func TestBuiltinPlayAnimCallsEnemy(t *testing.T) {
	played := ""
	e := &playAnimStub{stubEnemy: stubEnemy{}, played: &played}
	ctx := newCtx(e)
	if _, err := RunAction("play_anim", map[string]any{"key": "idle"}, ctx); err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	if played != "idle" {
		t.Fatalf("played = %q", played)
	}
}

type playAnimStub struct {
	stubEnemy
	played *string
}

func (p *playAnimStub) PlayAnim(id string) { *p.played = id }

func TestBuiltinConditionGrounded(t *testing.T) {
	e := &stubEnemy{grounded: true}
	ctx := newCtx(e)
	ok, err := RunCondition("grounded", nil, ctx)
	if err != nil {
		t.Fatalf("RunCondition: %v", err)
	}
	if !ok {
		t.Fatalf("grounded=true returned false")
	}
	e.grounded = false
	ok, _ = RunCondition("grounded", nil, ctx)
	if ok {
		t.Fatalf("grounded=false returned true")
	}
}

func TestBuiltinConditionAnimDone(t *testing.T) {
	e := &stubEnemy{animDone: true}
	ctx := newCtx(e)
	ok, _ := RunCondition("anim_done", nil, ctx)
	if !ok {
		t.Fatalf("animDone=true returned false")
	}
}

func TestBuiltinConditionAnimFrameGE(t *testing.T) {
	e := &stubEnemy{currentFrame: 5}
	ctx := newCtx(e)
	ok, err := RunCondition("anim_frame_ge", map[string]any{"frame": 4.0}, ctx)
	if err != nil {
		t.Fatalf("RunCondition: %v", err)
	}
	if !ok {
		t.Fatalf("frame 5 >= 4 returned false")
	}
	ok, _ = RunCondition("anim_frame_ge", map[string]any{"frame": 6.0}, ctx)
	if ok {
		t.Fatalf("frame 5 >= 6 returned true")
	}
}

func TestBuiltinConditionAnimFrameLE(t *testing.T) {
	e := &stubEnemy{currentFrame: 5}
	ctx := newCtx(e)
	ok, _ := RunCondition("anim_frame_le", map[string]any{"frame": 5.0}, ctx)
	if !ok {
		t.Fatalf("frame 5 <= 5 returned false")
	}
}

func TestRunActionUnknownReturnsError(t *testing.T) {
	_, err := RunAction("nope_nada", nil, newCtx(&stubEnemy{}))
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestRunConditionUnknownReturnsError(t *testing.T) {
	_, err := RunCondition("nope_nada", nil, newCtx(&stubEnemy{}))
	if err == nil {
		t.Fatal("expected error for unknown condition")
	}
}

func TestHasActionHasConditionLookup(t *testing.T) {
	if !HasAction("goto") {
		t.Fatal("HasAction(goto) false")
	}
	if HasAction("nope_nada") {
		t.Fatal("HasAction(nope_nada) true")
	}
	if !HasCondition("grounded") {
		t.Fatal("HasCondition(grounded) false")
	}
	if HasCondition("nope_nada") {
		t.Fatal("HasCondition(nope_nada) true")
	}
}

func TestRegisterDuplicateActionPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate action registration")
		}
	}()
	// "goto" is already registered by init.
	RegisterAction("goto", func(_ map[string]any, _ *Ctx) (Status, error) { return StatusSuccess, nil })
}

func TestRegisterDuplicateConditionPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate condition registration")
		}
	}()
	// "grounded" is already registered by init.
	RegisterCondition("grounded", func(_ map[string]any, _ *Ctx) (bool, error) { return true, nil })
}

func TestRegisteredActions_ReturnsBuiltinsWithArgSchema(t *testing.T) {
	metas := RegisteredActions()
	for i := 0; i < len(metas)-1; i++ {
		if metas[i].Name > metas[i+1].Name {
			t.Fatalf("not sorted: %s > %s", metas[i].Name, metas[i+1].Name)
		}
	}
	byName := map[string]ActionMeta{}
	for _, m := range metas {
		byName[m.Name] = m
	}
	goto_, ok := byName["goto"]
	if !ok {
		t.Fatal("goto not registered")
	}
	if len(goto_.Args) != 1 || goto_.Args[0].Name != "state" || goto_.Args[0].Type != "state_id" || !goto_.Args[0].Required {
		t.Fatalf("goto arg schema unexpected: %+v", goto_.Args)
	}
	setVx, ok := byName["set_vx_forward"]
	if !ok {
		t.Fatal("set_vx_forward not registered")
	}
	if len(setVx.Args) != 1 || setVx.Args[0].Type != "float" {
		t.Fatalf("set_vx_forward arg schema unexpected: %+v", setVx.Args)
	}
}

func TestRegisteredConditions_ReturnsBuiltinsWithArgSchema(t *testing.T) {
	metas := RegisteredConditions()
	for i := 0; i < len(metas)-1; i++ {
		if metas[i].Name > metas[i+1].Name {
			t.Fatalf("not sorted: %s > %s", metas[i].Name, metas[i+1].Name)
		}
	}
	byName := map[string]ActionMeta{}
	for _, m := range metas {
		byName[m.Name] = m
	}
	g, ok := byName["grounded"]
	if !ok {
		t.Fatal("grounded not registered")
	}
	if len(g.Args) != 0 {
		t.Fatalf("grounded should have no args: %+v", g.Args)
	}
	frame, ok := byName["anim_frame_ge"]
	if !ok {
		t.Fatal("anim_frame_ge not registered")
	}
	if len(frame.Args) != 1 || frame.Args[0].Type != "int" {
		t.Fatalf("anim_frame_ge arg schema unexpected: %+v", frame.Args)
	}
}
