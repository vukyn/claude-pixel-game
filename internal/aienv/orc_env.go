package aienv

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/config"
	"claude-pixel/internal/enemy"
	"claude-pixel/internal/player"
	"claude-pixel/internal/score"
	"claude-pixel/internal/spawner"
	"claude-pixel/internal/stamina"
	"claude-pixel/internal/storage"
	"claude-pixel/internal/world"
)

type OrcEnvConfig struct {
	Seed int64
}

type OrcStepResult struct {
	PlayerObs  []float64
	OrcObs     [][]float64
	OrcRewards []float64
	OrcDones   []bool
	Done       bool
	Info       map[string]any
}

type orcPrevState struct {
	lives int
	dist  float64
}

type OrcTrainEnv struct {
	cfg             *config.Config
	w               *world.World
	p               *player.Player
	enemies         []*enemy.Enemy
	sp              *spawner.Spawner
	sc              *score.Counter
	combatTuning    *combat.Tuning
	orcKind         *enemy.Kind
	physics         *player.Physics
	staminaT        *player.StaminaTuning
	spawnTuning     *enemy.SpawnTuning
	timeoutS        float64
	elapsedS        float64
	rng             *rand.Rand
	anims           map[string]*anim.Animation
	soldierBoxes    map[string]combat.Box
	prevPlayerLives int
	prevOrcStates   map[*enemy.Enemy]*orcPrevState
	stepsSinceHit   int
}

func NewOrcTrainEnv(ecfg OrcEnvConfig) (*OrcTrainEnv, error) {
	cfg := config.Load()
	db := storage.MustOpen(cfg)
	defer db.Close()

	animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
	hitboxRepo := storage.NewRepository[combat.HitboxSpec](db, combat.HitboxMapper{})

	animSpecs, err := animRepo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load anim specs: %w", err)
	}
	anims := buildHeadlessAnims(animSpecs)

	physics, err := player.LoadPhysics(tuneRepo)
	if err != nil {
		return nil, fmt.Errorf("load physics: %w", err)
	}
	staminaT, err := player.LoadStaminaTuning(tuneRepo)
	if err != nil {
		return nil, fmt.Errorf("load stamina: %w", err)
	}
	tuneParams, err := tuneRepo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list tuning: %w", err)
	}
	tuneMap := make(map[string]float64, len(tuneParams))
	for _, p := range tuneParams {
		tuneMap[p.Key] = p.Value
	}
	combatTuning, err := combat.LoadTuning(tuneMap)
	if err != nil {
		return nil, fmt.Errorf("load combat tuning: %w", err)
	}
	spawnTuning, err := enemy.LoadSpawnTuning(tuneRepo)
	if err != nil {
		return nil, fmt.Errorf("load spawn tuning: %w", err)
	}
	hitboxSpecs, err := hitboxRepo.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list hitboxes: %w", err)
	}
	soldierBoxes, err := combat.SoldierBoxes(hitboxSpecs, cfg.RenderScale)
	if err != nil {
		return nil, fmt.Errorf("load soldier boxes: %w", err)
	}
	orcKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "orc", Prefix: "orc", FrameW: 100, FrameH: 100,
		AnimLib: anims, HitboxSpecs: hitboxSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
		BehaviorPath: cfg.AssetsDir + "/behaviors/orc.json",
	})
	if err != nil {
		return nil, fmt.Errorf("build orc kind: %w", err)
	}

	seed := ecfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &OrcTrainEnv{
		cfg:          cfg,
		physics:      physics,
		staminaT:     staminaT,
		combatTuning: combatTuning,
		spawnTuning:  spawnTuning,
		orcKind:      orcKind,
		anims:        anims,
		soldierBoxes: soldierBoxes,
		timeoutS:     tuneMap["game_timeout_s"],
		rng:          rand.New(rand.NewSource(seed)),
	}, nil
}

func (env *OrcTrainEnv) Reset() OrcStepResult {
	pool := stamina.NewPool(env.staminaT.Max, env.staminaT.DrainPerSec, env.staminaT.RegenPerSec)
	env.w = world.New(env.cfg, env.physics.Gravity)
	env.p = player.New(player.Config{
		StartX:     float64(env.cfg.WindowW) / 2,
		StartY:     env.w.GroundY,
		Physics:    env.physics,
		Anims:      env.anims,
		Boxes:      env.soldierBoxes,
		StartLives: env.combatTuning.SoldierMaxLives,
		Stamina:    pool,
	})
	env.p.Grounded = true
	env.sc = &score.Counter{}
	env.elapsedS = 0
	env.prevPlayerLives = env.combatTuning.SoldierMaxLives
	env.stepsSinceHit = 0

	env.enemies = nil
	env.prevOrcStates = make(map[*enemy.Enemy]*orcPrevState)

	halfW := float64(env.orcKind.Boxes["body"].W) / 2
	spriteH := float64(env.orcKind.FrameH * env.cfg.RenderScale)
	newOrc := func(x, _ float64) *enemy.Enemy {
		if x < halfW {
			x = halfW
		}
		if maxX := float64(env.cfg.WindowW) - halfW; x > maxX {
			x = maxX
		}
		return enemy.New(enemy.Config{
			StartX: x, StartY: -spriteH,
			Physics: env.physics, Kind: env.orcKind, RNG: env.rng,
		})
	}

	env.sp = spawner.New(spawner.Config{
		MinIntervalS: env.spawnTuning.MinS,
		MaxIntervalS: env.spawnTuning.MaxS,
		MaxAlive:     env.spawnTuning.MaxAlive,
		SpawnXMin:    0,
		SpawnXMax:    float64(env.cfg.WindowW),
		RNG:          env.rng,
		Kinds: []spawner.KindFactory{{
			Name: env.orcKind.Name, Weight: 1,
			NewEnemy: newOrc,
		}},
	})

	firstOrc := newOrc(float64(env.cfg.WindowW)/4, 0)
	firstOrc.Y = env.w.GroundY
	firstOrc.Grounded = true
	env.enemies = append(env.enemies, firstOrc)
	env.prevOrcStates[firstOrc] = &orcPrevState{
		lives: firstOrc.Lives,
		dist:  math.Abs(firstOrc.X - env.p.X),
	}

	return env.buildResult(nil, nil)
}

func (env *OrcTrainEnv) Step(playerAction int, orcActions []int) OrcStepResult {
	dt := fixedDT
	intent := ToIntent(playerAction)

	if env.p.Stamina != nil {
		env.p.Stamina.Update(dt, env.p.IsSprinting(intent))
	}
	env.p.FSM.Handle(env.p, intent, dt)

	for i, e := range env.enemies {
		if e.Dead || e.CurrentState == "death" || e.CurrentState == "hurt" || e.CurrentState == "fall" {
			e.Tick(dt)
			continue
		}
		if e.CurrentState == "attack" || e.CurrentState == "attack2" {
			e.Tick(dt)
			continue
		}

		action := 0
		if i < len(orcActions) {
			action = orcActions[i]
		}
		env.applyOrcAction(e, action)
		if e.Current != nil {
			e.Current.Update(dt)
		}
	}

	for _, e := range env.enemies {
		if e.Lives <= 0 && e.CurrentState != "death" {
			e.Transition("death")
		}
		if e.OnHitPending {
			e.OnHitPending = false
			e.Transition("hurt")
		}
	}

	env.p.ApplyPhysics(env.w, dt)
	for _, e := range env.enemies {
		e.ApplyPhysics(env.w, dt)
	}

	soldierBodyHalfW := float64(env.p.Boxes["body"].W) / 2
	env.p.X = world.Clamp(env.p.X, soldierBodyHalfW, float64(env.cfg.WindowW)-soldierBodyHalfW)
	for _, e := range env.enemies {
		bodyHalfW := float64(e.Kind.Boxes["body"].W) / 2
		e.X = world.Clamp(e.X, bodyHalfW, float64(env.cfg.WindowW)-bodyHalfW)
	}

	if env.p.Current != nil {
		env.p.Current.Update(dt)
	}

	if spawned := env.sp.Tick(dt, len(env.enemies)); spawned != nil {
		env.enemies = append(env.enemies, spawned)
		env.prevOrcStates[spawned] = &orcPrevState{
			lives: spawned.Lives,
			dist:  math.Abs(spawned.X - env.p.X),
		}
	}

	orcHitsOnPlayer := env.dispatchOrcHits()
	env.dispatchPlayerHits()

	playerAttacking := env.p.CurrentAnim == "soldier_attack" || env.p.CurrentAnim == "soldier_attack2"
	orcRewards := make([]float64, len(env.enemies))
	orcDones := make([]bool, len(env.enemies))

	for i, e := range env.enemies {
		prev, ok := env.prevOrcStates[e]
		if !ok {
			prev = &orcPrevState{lives: int(env.orcKind.Tuning.MaxLives)}
			env.prevOrcStates[e] = prev
		}
		livesLost := prev.lives - e.Lives
		currDist := math.Abs(e.X - env.p.X)
		distDelta := currDist - prev.dist

		hitsLanded := 0
		for _, hitOrc := range orcHitsOnPlayer {
			if hitOrc == e {
				hitsLanded++
			}
		}

		dodged := playerAttacking && distDelta > 0 && livesLost == 0

		if hitsLanded > 0 {
			env.stepsSinceHit = 0
		} else {
			env.stepsSinceHit++
		}

		playerDied := env.p.FSM.CurrentID() == player.StateDeath

		orcRewards[i] = OrcCalcReward(OrcRewardInput{
			HitPlayer:    hitsLanded,
			PlayerDied:   playerDied,
			OrcLivesLost: livesLost,
			OrcDied:      e.Dead || e.CurrentState == "death",
			DodgeSuccess: dodged,
			Stagnant:     env.stepsSinceHit > 180,
			DistDelta:    distDelta,
		})

		orcDones[i] = e.Dead || e.CurrentState == "death"
		prev.lives = e.Lives
		prev.dist = currDist
	}

	alive := env.enemies[:0]
	for _, e := range env.enemies {
		if e.Dead {
			delete(env.prevOrcStates, e)
			continue
		}
		alive = append(alive, e)
	}
	env.enemies = alive

	env.elapsedS += dt.Seconds()
	env.prevPlayerLives = env.p.Lives

	return env.buildResult(orcRewards, orcDones)
}

func (env *OrcTrainEnv) applyOrcAction(e *enemy.Enemy, action int) {
	ar := OrcAction(action)

	if ar.Transition != "" && e.CurrentState == "run" {
		e.Transition(ar.Transition)
		return
	}

	if ar.Flip {
		e.Facing = -e.Facing
		e.VX = 0
		return
	}

	speed := 120.0
	switch ar.VXMode {
	case OrcVXStop:
		e.VX = 0
	case OrcVXToward:
		if env.p.X > e.X {
			e.Facing = 1
		} else {
			e.Facing = -1
		}
		e.VX = float64(e.Facing) * speed
	case OrcVXAway:
		if env.p.X > e.X {
			e.Facing = -1
		} else {
			e.Facing = 1
		}
		e.VX = float64(e.Facing) * speed
	}
}

func (env *OrcTrainEnv) dispatchOrcHits() []*enemy.Enemy {
	attackers := make([]combat.Fighter, 0, len(env.enemies))
	for _, e := range env.enemies {
		attackers = append(attackers, e)
	}
	victims := []combat.Fighter{env.p}
	hits := combat.Resolve(attackers, victims)
	var hitOrcs []*enemy.Enemy
	for _, ev := range hits {
		orc := ev.Attacker.(*enemy.Enemy)
		env.p.OnHit(env.combatTuning.SoldierKnockbackVX, env.combatTuning.SoldierKnockbackVY, orc.X)
		hitOrcs = append(hitOrcs, orc)
	}
	return hitOrcs
}

func (env *OrcTrainEnv) dispatchPlayerHits() int {
	attackers := []combat.Fighter{env.p}
	victims := make([]combat.Fighter, 0, len(env.enemies))
	for _, e := range env.enemies {
		victims = append(victims, e)
	}
	hits := combat.Resolve(attackers, victims)
	for _, ev := range hits {
		orc := ev.Victim.(*enemy.Enemy)
		orc.OnHit(env.p.X)
	}
	return len(hits)
}

func (env *OrcTrainEnv) buildResult(orcRewards []float64, orcDones []bool) OrcStepResult {
	playerObs := env.playerObserve()

	orcObs := make([][]float64, len(env.enemies))
	for i, e := range env.enemies {
		obs := env.orcObserve(e)
		orcObs[i] = obs[:]
	}

	if orcRewards == nil {
		orcRewards = make([]float64, len(env.enemies))
	}
	if orcDones == nil {
		orcDones = make([]bool, len(env.enemies))
	}

	playerDied := env.p.FSM.CurrentID() == player.StateDeath
	timedOut := env.timeoutS > 0 && env.elapsedS >= env.timeoutS

	return OrcStepResult{
		PlayerObs:  playerObs[:],
		OrcObs:     orcObs,
		OrcRewards: orcRewards,
		OrcDones:   orcDones,
		Done:       playerDied || timedOut,
		Info: map[string]any{
			"player_lives": env.p.Lives,
			"orc_count":    len(env.enemies),
			"elapsed":      env.elapsedS,
		},
	}
}

func (env *OrcTrainEnv) playerObserve() [ObsSize]float64 {
	enemies := make([]EnemyState, 0, len(env.enemies))
	for _, e := range env.enemies {
		stateIdx := 0
		attacking := false
		switch e.CurrentState {
		case "run":
			stateIdx = 1
		case "attack":
			stateIdx = 2
			attacking = true
		case "attack2":
			stateIdx = 3
			attacking = true
		case "hurt":
			stateIdx = 4
		case "death":
			stateIdx = 5
		}
		enemies = append(enemies, EnemyState{
			RelX: e.X - env.p.X, RelY: e.Y - env.p.Y,
			Lives: e.Lives, MaxLives: int(env.orcKind.Tuning.MaxLives),
			State: stateIdx, Attacking: attacking,
		})
	}
	stateIdx := 0
	switch env.p.FSM.CurrentID() {
	case player.StateRun:
		stateIdx = 1
	case player.StateJump:
		stateIdx = 2
	case player.StateFall:
		stateIdx = 3
	case player.StateAttack:
		stateIdx = 4
	case player.StateAttack2:
		stateIdx = 5
	case player.StateHit:
		stateIdx = 6
	case player.StateDeath:
		stateIdx = 7
	}
	staminaFrac := 0.0
	if env.p.Stamina != nil {
		staminaFrac = env.p.Stamina.Fraction()
	}
	return Observe(GameState{
		PlayerX: env.p.X, PlayerY: env.p.Y,
		PlayerVX: env.p.VX, PlayerVY: env.p.VY,
		Facing: env.p.Facing, Grounded: env.p.Grounded,
		Lives: env.p.Lives, MaxLives: env.combatTuning.SoldierMaxLives,
		Stamina: staminaFrac, StateID: stateIdx, NumStates: 8,
		TimeoutS: env.timeoutS, ElapsedS: env.elapsedS,
		Enemies: enemies, MaxAlive: env.spawnTuning.MaxAlive,
		Score: env.sc.Total(),
		MaxSpeed: env.physics.SprintSpeed, MaxFall: env.physics.MaxFallSpeed,
		WindowW: float64(env.cfg.WindowW), WindowH: float64(env.cfg.WindowH),
	})
}

func (env *OrcTrainEnv) orcObserve(e *enemy.Enemy) [OrcObsSize]float64 {
	orcStateIdx := 0
	switch e.CurrentState {
	case "run":
		orcStateIdx = 1
	case "attack":
		orcStateIdx = 2
	case "attack2":
		orcStateIdx = 3
	case "hurt":
		orcStateIdx = 4
	case "death":
		orcStateIdx = 5
	}
	playerStateIdx := 0
	switch env.p.FSM.CurrentID() {
	case player.StateRun:
		playerStateIdx = 1
	case player.StateJump:
		playerStateIdx = 2
	case player.StateFall:
		playerStateIdx = 3
	case player.StateAttack:
		playerStateIdx = 4
	case player.StateAttack2:
		playerStateIdx = 5
	case player.StateHit:
		playerStateIdx = 6
	case player.StateDeath:
		playerStateIdx = 7
	}
	playerAttacking := env.p.CurrentAnim == "soldier_attack" || env.p.CurrentAnim == "soldier_attack2"

	return OrcObserve(OrcGameState{
		OrcX: e.X, OrcY: e.Y, OrcVX: e.VX, OrcVY: e.VY,
		OrcFacing: e.Facing, OrcGrounded: e.Grounded,
		OrcLives: e.Lives, OrcMaxLives: int(env.orcKind.Tuning.MaxLives),
		OrcStateID: orcStateIdx, OrcNumStates: 6,
		PlayerX: env.p.X, PlayerY: env.p.Y,
		PlayerLives: env.p.Lives, PlayerMaxLives: env.combatTuning.SoldierMaxLives,
		PlayerStateID: playerStateIdx, PlayerNumStates: 8,
		PlayerAttacking: playerAttacking, PlayerFacing: env.p.Facing,
		OrcMaxSpeed: 120, OrcMaxFall: env.physics.MaxFallSpeed,
		TimeoutS: env.timeoutS, ElapsedS: env.elapsedS,
		WindowW: float64(env.cfg.WindowW), WindowH: float64(env.cfg.WindowH),
	})
}
