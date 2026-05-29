package aienv

import (
	"math"
	"testing"
)

func TestOrcReward_Survival(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{})
	if math.Abs(r-0.001) > 1e-6 {
		t.Errorf("idle step = %f, want 0.001", r)
	}
}

func TestOrcReward_HitPlayer(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{HitPlayer: 1})
	if r < 8 {
		t.Errorf("hit player = %f, want >= 8", r)
	}
}

func TestOrcReward_PlayerDied(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{PlayerDied: true})
	if r < 19 {
		t.Errorf("player died = %f, want >= 19", r)
	}
}

func TestOrcReward_OrcDied(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{OrcDied: true})
	if r > -4 {
		t.Errorf("orc died = %f, want <= -4", r)
	}
}

func TestOrcReward_LifeLost(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{OrcLivesLost: 1})
	if r > -0.5 {
		t.Errorf("life lost = %f, want <= -0.5", r)
	}
}

func TestOrcReward_Whiff(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{AttackWhiffed: true})
	base := OrcCalcReward(OrcRewardInput{})
	if r >= base {
		t.Errorf("whiff should decrease reward: got %f, base %f", r, base)
	}
}

func TestOrcReward_DodgeSuccess(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{DodgeSuccess: true})
	base := OrcCalcReward(OrcRewardInput{})
	if r <= base {
		t.Errorf("dodge should increase reward: got %f, base %f", r, base)
	}
}

func TestOrcReward_MoveToward(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{DistDelta: -50})
	base := OrcCalcReward(OrcRewardInput{})
	if r <= base {
		t.Errorf("move toward should increase reward: got %f, base %f", r, base)
	}
}

func TestOrcReward_Stagnant(t *testing.T) {
	r := OrcCalcReward(OrcRewardInput{Stagnant: true})
	base := OrcCalcReward(OrcRewardInput{})
	if r >= base {
		t.Errorf("stagnant should decrease reward: got %f, base %f", r, base)
	}
}
