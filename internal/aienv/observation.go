package aienv

import "math"

const ObsSize = 25

type EnemyState struct {
	RelX      float64
	RelY      float64
	Lives     int
	MaxLives  int
	State     int
	Attacking bool
}

type GameState struct {
	PlayerX, PlayerY   float64
	PlayerVX, PlayerVY float64
	Facing             int
	Grounded           bool
	Lives, MaxLives    int
	Stamina            float64
	StateID            int
	NumStates          int
	TimeoutS           float64
	ElapsedS           float64
	Enemies            []EnemyState
	MaxAlive           int
	Score              int
	MaxSpeed           float64
	MaxFall            float64
	WindowW, WindowH   float64
}

func Observe(gs GameState) [ObsSize]float64 {
	var obs [ObsSize]float64

	obs[0] = clamp01(gs.PlayerX / gs.WindowW)
	obs[1] = clamp01(gs.PlayerY / gs.WindowH)
	obs[2] = clamp01((gs.PlayerVX/safeDivisor(gs.MaxSpeed) + 1) / 2)
	obs[3] = clamp01((gs.PlayerVY/safeDivisor(gs.MaxFall) + 1) / 2)

	if gs.Facing >= 0 {
		obs[4] = 1
	}
	if gs.Grounded {
		obs[5] = 1
	}
	obs[6] = clamp01(float64(gs.Lives) / safeDivisor(float64(gs.MaxLives)))
	obs[7] = clamp01(gs.Stamina)

	obs[8] = clamp01(float64(gs.StateID) / safeDivisor(float64(gs.NumStates)))

	remaining := gs.TimeoutS - gs.ElapsedS
	if remaining < 0 {
		remaining = 0
	}
	obs[9] = clamp01(remaining / safeDivisor(gs.TimeoutS))

	sorted := sortByDistance(gs.Enemies)
	for i := 0; i < 3; i++ {
		base := 10 + i*3
		if i < len(sorted) {
			e := sorted[i]
			obs[base+0] = clamp01(math.Abs(e.RelX) / gs.WindowW)
			obs[base+1] = clamp01((e.RelY/gs.WindowH + 1) / 2)
			obs[base+2] = clamp01(float64(e.Lives) / safeDivisor(float64(e.MaxLives)))
		}
	}

	obs[19] = clamp01(float64(len(gs.Enemies)) / safeDivisor(float64(gs.MaxAlive)))

	maxScore := 1000.0
	obs[20] = clamp01(float64(gs.Score) / maxScore)

	if len(sorted) > 0 {
		nearest := sorted[0]
		diag := math.Sqrt(gs.WindowW*gs.WindowW + gs.WindowH*gs.WindowH)
		dist := math.Sqrt(nearest.RelX*nearest.RelX + nearest.RelY*nearest.RelY)
		obs[21] = clamp01(dist / diag)
		obs[22] = clamp01((math.Atan2(nearest.RelY, nearest.RelX)/math.Pi + 1) / 2)
		obs[23] = clamp01(float64(nearest.State) / 6.0)
		if nearest.Attacking {
			obs[24] = 1
		}
	}

	return obs
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	if math.IsNaN(v) {
		return 0
	}
	return v
}

func safeDivisor(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}

func sortByDistance(enemies []EnemyState) []EnemyState {
	if len(enemies) == 0 {
		return nil
	}
	sorted := make([]EnemyState, len(enemies))
	copy(sorted, enemies)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			di := sorted[j].RelX*sorted[j].RelX + sorted[j].RelY*sorted[j].RelY
			dj := sorted[j-1].RelX*sorted[j-1].RelX + sorted[j-1].RelY*sorted[j-1].RelY
			if di < dj {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			}
		}
	}
	return sorted
}
