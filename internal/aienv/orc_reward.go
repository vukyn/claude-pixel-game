package aienv

type OrcRewardInput struct {
	HitPlayer    int
	PlayerDied   bool
	OrcLivesLost int
	OrcDied      bool
	DodgeSuccess bool
	Stagnant     bool
	DistDelta    float64
}

func OrcCalcReward(in OrcRewardInput) float64 {
	reward := 0.0

	reward += float64(in.HitPlayer) * 15.0

	if in.PlayerDied {
		reward += 30.0
		return reward
	}

	reward += float64(in.OrcLivesLost) * -2.0

	if in.OrcDied {
		reward += -5.0
		return reward
	}

	reward += 0.01

	if in.DodgeSuccess {
		reward += 1.0
	}
	if in.Stagnant {
		reward += -0.3
	}
	if in.DistDelta < 0 {
		reward += 0.1
	}

	return reward
}
