package input

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Intent struct {
	Left, Right    bool
	JumpPressed    bool
	SprintHeld     bool
	AttackPressed  bool
	Attack2Pressed bool
	PauseEdge      bool
}

func Poll() Intent {
	return Intent{
		Left:           ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft),
		Right:          ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight),
		JumpPressed:    inpututil.IsKeyJustPressed(ebiten.KeySpace),
		SprintHeld:     ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight),
		AttackPressed:  inpututil.IsKeyJustPressed(ebiten.KeyJ) || inpututil.IsKeyJustPressed(ebiten.KeyX),
		Attack2Pressed: inpututil.IsKeyJustPressed(ebiten.KeyK) || inpututil.IsKeyJustPressed(ebiten.KeyC),
		PauseEdge:      inpututil.IsKeyJustPressed(ebiten.KeyEscape),
	}
}
