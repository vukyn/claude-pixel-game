package aienv

import (
	"math"
	"testing"
)

func TestReward_Survival(t *testing.T) {
	r := CalcReward(RewardInput{})
	if math.Abs(r-0.01) > 1e-6 {
		t.Errorf("idle step reward = %f, want 0.01", r)
	}
}

func TestReward_EnemyKilled(t *testing.T) {
	r := CalcReward(RewardInput{EnemyKilledPoints: 10})
	if r < 10 {
		t.Errorf("enemy killed reward = %f, want >= 10", r)
	}
}

func TestReward_LifeLost(t *testing.T) {
	r := CalcReward(RewardInput{LivesLost: 1})
	if r > -4 {
		t.Errorf("life lost reward = %f, want <= -4", r)
	}
}

func TestReward_Death(t *testing.T) {
	r := CalcReward(RewardInput{Died: true})
	if r > -49 {
		t.Errorf("death reward = %f, want <= -49", r)
	}
}

func TestReward_HitLanded(t *testing.T) {
	r := CalcReward(RewardInput{HitsLanded: 1})
	if r < 2 {
		t.Errorf("hit landed reward = %f, want >= 2", r)
	}
}

func TestReward_MovedToward(t *testing.T) {
	r := CalcReward(RewardInput{DistDelta: -50})
	base := CalcReward(RewardInput{})
	if r <= base {
		t.Errorf("moving toward enemy should increase reward: got %f, baseline %f", r, base)
	}
}

func TestReward_MovedAway(t *testing.T) {
	r := CalcReward(RewardInput{DistDelta: 50})
	base := CalcReward(RewardInput{})
	if r >= base {
		t.Errorf("moving away should decrease reward: got %f, baseline %f", r, base)
	}
}

func TestReward_ShapedScale(t *testing.T) {
	full := CalcReward(RewardInput{DistDelta: -50})
	half := CalcRewardScaled(RewardInput{DistDelta: -50}, 0.5)
	base := CalcReward(RewardInput{})
	baseHalf := CalcRewardScaled(RewardInput{}, 0.5)
	shapedFull := full - base
	shapedHalf := half - baseHalf
	if math.Abs(shapedHalf-shapedFull*0.5) > 0.01 {
		t.Errorf("shaped scale 0.5: got delta %f, want %f", shapedHalf, shapedFull*0.5)
	}
}

func TestReward_Timeout(t *testing.T) {
	r := CalcReward(RewardInput{TimedOut: true, FinalScore: 50})
	if r < 4 {
		t.Errorf("timeout reward = %f, want >= 4 (score/10)", r)
	}
}
