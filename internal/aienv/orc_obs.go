package aienv

import "math"

const OrcObsSize = 16

type OrcGameState struct {
	OrcX, OrcY       float64
	OrcVX, OrcVY     float64
	OrcFacing        int
	OrcGrounded      bool
	OrcLives         int
	OrcMaxLives      int
	OrcStateID       int
	OrcNumStates     int
	PlayerX, PlayerY float64
	PlayerLives      int
	PlayerMaxLives   int
	PlayerStateID    int
	PlayerNumStates  int
	PlayerAttacking  bool
	PlayerFacing     int
	OrcMaxSpeed      float64
	OrcMaxFall       float64
	TimeoutS         float64
	ElapsedS         float64
	WindowW, WindowH float64
}

func OrcObserve(gs OrcGameState) [OrcObsSize]float64 {
	var obs [OrcObsSize]float64

	obs[0] = clamp01(gs.OrcX / gs.WindowW)
	obs[1] = clamp01(gs.OrcY / gs.WindowH)
	obs[2] = clamp01((gs.OrcVX/safeDivisor(gs.OrcMaxSpeed) + 1) / 2)
	obs[3] = clamp01((gs.OrcVY/safeDivisor(gs.OrcMaxFall) + 1) / 2)
	if gs.OrcFacing >= 0 {
		obs[4] = 1
	}
	if gs.OrcGrounded {
		obs[5] = 1
	}
	obs[6] = clamp01(float64(gs.OrcLives) / safeDivisor(float64(gs.OrcMaxLives)))
	obs[7] = clamp01(float64(gs.OrcStateID) / safeDivisor(float64(gs.OrcNumStates)))

	relX := gs.PlayerX - gs.OrcX
	relY := gs.PlayerY - gs.OrcY
	obs[8] = clamp01((relX/gs.WindowW + 1) / 2)
	obs[9] = clamp01((relY/gs.WindowH + 1) / 2)

	obs[10] = clamp01(float64(gs.PlayerLives) / safeDivisor(float64(gs.PlayerMaxLives)))
	obs[11] = clamp01(float64(gs.PlayerStateID) / safeDivisor(float64(gs.PlayerNumStates)))
	if gs.PlayerAttacking {
		obs[12] = 1
	}

	playerFacingToward := false
	if gs.PlayerX > gs.OrcX && gs.PlayerFacing < 0 {
		playerFacingToward = true
	}
	if gs.PlayerX < gs.OrcX && gs.PlayerFacing > 0 {
		playerFacingToward = true
	}
	if playerFacingToward {
		obs[13] = 1
	}

	diag := math.Sqrt(gs.WindowW*gs.WindowW + gs.WindowH*gs.WindowH)
	dist := math.Sqrt(relX*relX + relY*relY)
	obs[14] = clamp01(dist / diag)

	remaining := gs.TimeoutS - gs.ElapsedS
	if remaining < 0 {
		remaining = 0
	}
	obs[15] = clamp01(remaining / safeDivisor(gs.TimeoutS))

	return obs
}
