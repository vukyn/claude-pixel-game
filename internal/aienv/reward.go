package aienv

type RewardInput struct {
	EnemyKilledPoints int
	LivesLost         int
	Died              bool
	TimedOut          bool
	FinalScore        int
	HitsLanded        int
	AttackWhiffed     bool
	DistDelta         float64
}

func CalcReward(in RewardInput) float64 {
	return CalcRewardScaled(in, 1.0)
}

func CalcRewardScaled(in RewardInput, shapedScale float64) float64 {
	reward := 0.0

	reward += float64(in.EnemyKilledPoints)
	reward += float64(in.LivesLost) * -5.0

	if in.Died {
		reward += -50.0
		return reward
	}

	if in.TimedOut {
		reward += float64(in.FinalScore) / 10.0
		return reward
	}

	reward += 0.01

	shaped := 0.0
	if in.HitsLanded > 0 {
		shaped += float64(in.HitsLanded) * 2.0
	}
	if in.AttackWhiffed {
		shaped += -0.1
	}
	if in.DistDelta < 0 {
		shaped += 0.5
	} else if in.DistDelta > 0 {
		shaped += -0.2
	}

	reward += shaped * shapedScale

	return reward
}
