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

const fixedDT = time.Second / 60

type EnvConfig struct {
	Seed int64
}

type GameEnv struct {
	cfg          *config.Config
	w            *world.World
	p            *player.Player
	enemies      []*enemy.Enemy
	sp           *spawner.Spawner
	sc           *score.Counter
	combatTuning *combat.Tuning
	kinds        []*enemy.Kind
	physics      *player.Physics
	staminaT     *player.StaminaTuning
	spawnTuning  *enemy.SpawnTuning
	timeoutS     float64
	elapsedS     float64
	rng          *rand.Rand
	prevScore    int
	prevLives    int
	prevDist     float64
	anims        map[string]*anim.Animation
	soldierBoxes map[string]combat.Box
	shapedScale  float64
}

func NewGameEnv(ecfg EnvConfig) (*GameEnv, error) {
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
	slimeKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "slime", Prefix: "slime", FrameW: 96, FrameH: 96,
		AnimLib: anims, HitboxSpecs: hitboxSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
		BehaviorPath: cfg.AssetsDir + "/behaviors/slime.json",
	})
	if err != nil {
		return nil, fmt.Errorf("build slime kind: %w", err)
	}

	seed := ecfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &GameEnv{
		cfg:          cfg,
		physics:      physics,
		staminaT:     staminaT,
		combatTuning: combatTuning,
		spawnTuning:  spawnTuning,
		kinds:        []*enemy.Kind{orcKind, slimeKind},
		anims:        anims,
		soldierBoxes: soldierBoxes,
		timeoutS:     tuneMap["game_timeout_s"],
		rng:          rand.New(rand.NewSource(seed)),
		shapedScale:  1.0,
	}, nil
}

func (env *GameEnv) SetShapedScale(s float64) { env.shapedScale = s }

func (env *GameEnv) Reset() []float64 {
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
	env.enemies = nil
	env.sc = &score.Counter{}
	env.elapsedS = 0
	env.prevScore = 0
	env.prevLives = env.combatTuning.SoldierMaxLives
	env.prevDist = 0

	env.sp = spawner.New(spawner.Config{
		MinIntervalS: env.spawnTuning.MinS,
		MaxIntervalS: env.spawnTuning.MaxS,
		MaxAlive:     env.spawnTuning.MaxAlive,
		SpawnXMin:    0,
		SpawnXMax:    float64(env.cfg.WindowW),
		RNG:          env.rng,
		Kinds:        env.buildKindFactories(),
	})

	obs := env.observe()
	return obs[:]
}

func (env *GameEnv) Step(action int) (obs []float64, reward float64, done bool, info map[string]interface{}) {
	intent := ToIntent(action)
	dt := fixedDT

	if env.p.Stamina != nil {
		env.p.Stamina.Update(dt, env.p.IsSprinting(intent))
	}
	env.p.FSM.Handle(env.p, intent, dt)

	for _, e := range env.enemies {
		e.Tick(dt)
	}

	env.p.ApplyPhysics(env.w, dt)
	for _, e := range env.enemies {
		e.ApplyPhysics(env.w, dt)
	}

	soldierBodyHalfW := float64(env.p.Boxes["body"].W) / 2
	env.p.X = world.Clamp(env.p.X, soldierBodyHalfW, float64(env.cfg.WindowW)-soldierBodyHalfW)

	for _, e := range env.enemies {
		bodyHalfW := float64(e.Kind.Boxes["body"].W) / 2
		clamped := world.Clamp(e.X, bodyHalfW, float64(env.cfg.WindowW)-bodyHalfW)
		if clamped != e.X && e.CurrentState == "run" {
			if e.X <= bodyHalfW {
				e.Facing = 1
			} else {
				e.Facing = -1
			}
		}
		e.X = clamped
	}

	if env.p.Current != nil {
		env.p.Current.Update(dt)
	}

	if spawned := env.sp.Tick(dt, len(env.enemies)); spawned != nil {
		env.enemies = append(env.enemies, spawned)
	}

	soldierHits := env.dispatchSoldierHits()
	env.dispatchEnemyHits()

	alive := env.enemies[:0]
	for _, e := range env.enemies {
		if e.Dead {
			env.sc.Add(e.Kind.Tuning.Points)
			continue
		}
		alive = append(alive, e)
	}
	env.enemies = alive

	env.elapsedS += dt.Seconds()

	died := env.p.FSM.CurrentID() == player.StateDeath
	timedOut := env.timeoutS > 0 && env.elapsedS >= env.timeoutS
	done = died || timedOut

	livesLost := env.prevLives - env.p.Lives
	killedPoints := env.sc.Total() - env.prevScore
	currDist := env.nearestEnemyDist()
	distDelta := currDist - env.prevDist

	whiffed := false
	isAttacking := env.p.CurrentAnim == "soldier_attack" || env.p.CurrentAnim == "soldier_attack2"
	if isAttacking && soldierHits == 0 {
		whiffed = true
	}

	jumpedNoReason := false
	if intent.JumpPressed && currDist > 200 {
		jumpedNoReason = true
	}

	reward = CalcRewardScaled(RewardInput{
		EnemyKilledPoints: killedPoints,
		LivesLost:         livesLost,
		Died:              died,
		TimedOut:          timedOut,
		FinalScore:        env.sc.Total(),
		HitsLanded:        soldierHits,
		AttackWhiffed:     whiffed,
		JumpedNoReason:    jumpedNoReason,
		DistDelta:         distDelta,
	}, env.shapedScale)

	env.prevScore = env.sc.Total()
	env.prevLives = env.p.Lives
	env.prevDist = currDist

	obsArr := env.observe()
	info = map[string]interface{}{
		"score":   env.sc.Total(),
		"lives":   env.p.Lives,
		"elapsed": env.elapsedS,
	}

	return obsArr[:], reward, done, info
}

func (env *GameEnv) dispatchSoldierHits() int {
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

func (env *GameEnv) dispatchEnemyHits() {
	attackers := make([]combat.Fighter, 0, len(env.enemies))
	for _, e := range env.enemies {
		attackers = append(attackers, e)
	}
	victims := []combat.Fighter{env.p}
	for _, ev := range combat.Resolve(attackers, victims) {
		orc := ev.Attacker.(*enemy.Enemy)
		env.p.OnHit(env.combatTuning.SoldierKnockbackVX, env.combatTuning.SoldierKnockbackVY, orc.X)
	}
}

func (env *GameEnv) nearestEnemyDist() float64 {
	if len(env.enemies) == 0 {
		return 0
	}
	best := math.MaxFloat64
	for _, e := range env.enemies {
		d := math.Abs(e.X - env.p.X)
		if d < best {
			best = d
		}
	}
	return best
}

func (env *GameEnv) observe() [ObsSize]float64 {
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
			RelX:      e.X - env.p.X,
			RelY:      e.Y - env.p.Y,
			Lives:     e.Lives,
			MaxLives:  int(e.Kind.Tuning.MaxLives),
			State:     stateIdx,
			Attacking: attacking,
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
		PlayerX:   env.p.X,
		PlayerY:   env.p.Y,
		PlayerVX:  env.p.VX,
		PlayerVY:  env.p.VY,
		Facing:    env.p.Facing,
		Grounded:  env.p.Grounded,
		Lives:     env.p.Lives,
		MaxLives:  env.combatTuning.SoldierMaxLives,
		Stamina:   staminaFrac,
		StateID:   stateIdx,
		NumStates: 8,
		TimeoutS:  env.timeoutS,
		ElapsedS:  env.elapsedS,
		Enemies:   enemies,
		MaxAlive:  env.spawnTuning.MaxAlive,
		Score:     env.sc.Total(),
		MaxSpeed:  env.physics.SprintSpeed,
		MaxFall:   env.physics.MaxFallSpeed,
		WindowW:   float64(env.cfg.WindowW),
		WindowH:   float64(env.cfg.WindowH),
	})
}

func (env *GameEnv) buildKindFactories() []spawner.KindFactory {
	factories := make([]spawner.KindFactory, 0, len(env.kinds))
	for _, k := range env.kinds {
		k := k
		halfW := float64(k.Boxes["body"].W) / 2
		spriteH := float64(k.FrameH * env.cfg.RenderScale)
		factories = append(factories, spawner.KindFactory{
			Name:   k.Name,
			Weight: 1,
			NewEnemy: func(x, _ float64) *enemy.Enemy {
				if x < halfW {
					x = halfW
				}
				if maxX := float64(env.cfg.WindowW) - halfW; x > maxX {
					x = maxX
				}
				return enemy.New(enemy.Config{
					StartX: x, StartY: -spriteH,
					Physics: env.physics,
					Kind:    k,
					RNG:     env.rng,
				})
			},
		})
	}
	return factories
}

func buildHeadlessAnims(specs []anim.AnimationSpec) map[string]*anim.Animation {
	m := make(map[string]*anim.Animation, len(specs))
	for i := range specs {
		s := &specs[i]
		m[s.ID] = anim.NewAnimation(s, nil)
	}
	return m
}
