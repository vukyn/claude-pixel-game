package input

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Intent struct {
	Left, Right    bool
	JumpPressed    bool
	DashPressed    bool
	AttackPressed  bool
	Attack2Pressed bool
}

func Poll() Intent {
	return Intent{
		Left:           ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft),
		Right:          ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight),
		JumpPressed:    inpututil.IsKeyJustPressed(ebiten.KeySpace),
		DashPressed:    inpututil.IsKeyJustPressed(ebiten.KeyShiftLeft) || inpututil.IsKeyJustPressed(ebiten.KeyShiftRight),
		AttackPressed:  inpututil.IsKeyJustPressed(ebiten.KeyJ) || inpututil.IsKeyJustPressed(ebiten.KeyX),
		Attack2Pressed: inpututil.IsKeyJustPressed(ebiten.KeyK) || inpututil.IsKeyJustPressed(ebiten.KeyC),
	}
}
