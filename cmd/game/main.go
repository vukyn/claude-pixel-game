package main

import (
	"context"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/config"
	"claude-pixel/internal/debug"
	"claude-pixel/internal/enemy"
	"claude-pixel/internal/game"
	"claude-pixel/internal/hud"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	cfg := config.Load()

	db := storage.MustOpen(cfg)
	defer db.Close()

	animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})
	hitboxRepo := storage.NewRepository[combat.HitboxSpec](db, combat.HitboxMapper{})

	anims, err := anim.LoadLibrary(cfg, animRepo)
	if err != nil {
		log.Fatalf("load animations: %v", err)
	}
	physics, err := player.LoadPhysics(tuneRepo)
	if err != nil {
		log.Fatalf("load physics: %v", err)
	}
	staminaTuning, err := player.LoadStaminaTuning(tuneRepo)
	if err != nil {
		log.Fatalf("load stamina tuning: %v", err)
	}

	hudLayoutRepo := storage.NewRepository[hud.LayoutRow](db, hud.LayoutMapper{})
	layout, err := hud.LoadLayout(hudLayoutRepo)
	if err != nil {
		log.Fatalf("load hud layout: %v", err)
	}

	tuneParams, err := tuneRepo.List(context.Background())
	if err != nil {
		log.Fatalf("list tuning: %v", err)
	}
	tuneMap := make(map[string]float64, len(tuneParams))
	for _, p := range tuneParams {
		tuneMap[p.Key] = p.Value
	}
	combatTuning, err := combat.LoadTuning(tuneMap)
	if err != nil {
		log.Fatalf("load combat tuning: %v", err)
	}

	spawnTuning, err := enemy.LoadSpawnTuning(tuneRepo)
	if err != nil {
		log.Fatalf("load spawn tuning: %v", err)
	}

	hitboxSpecs, err := hitboxRepo.List(context.Background())
	if err != nil {
		log.Fatalf("list hitboxes: %v", err)
	}

	soldierBoxes, err := combat.SoldierBoxes(hitboxSpecs, cfg.RenderScale)
	if err != nil {
		log.Fatalf("load soldier boxes: %v", err)
	}

	orcKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "orc", Prefix: "orc", FrameW: 100, FrameH: 100,
		AnimLib: anims, HitboxSpecs: hitboxSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
		BehaviorPath: cfg.AssetsDir + "/behaviors/orc.json",
	})
	if err != nil {
		log.Fatalf("build orc kind: %v", err)
	}

	slimeKind, err := enemy.BuildKind(enemy.KindConfig{
		Name: "slime", Prefix: "slime", FrameW: 96, FrameH: 96,
		AnimLib: anims, HitboxSpecs: hitboxSpecs,
		TuneRepo: tuneRepo, RenderScale: cfg.RenderScale,
		BehaviorPath: cfg.AssetsDir + "/behaviors/slime.json",
	})
	if err != nil {
		log.Fatalf("build slime kind: %v", err)
	}

	heart, ok := anims["heart_beat"]
	if !ok {
		log.Fatalf("missing heart_beat anim")
	}
	staminaAnim, okStam := anims["stamina_bar"]
	if !okStam {
		log.Fatalf("missing stamina_bar anim")
	}
	dbgCfg, err := debug.LoadConfig(cfg.DebugConfigPath)
	if err != nil {
		log.Fatalf("load debug config: %v", err)
	}
	if err := hud.LoadFont(cfg.FontPath); err != nil {
		log.Fatalf("load font: %v", err)
	}

	allKinds := []*enemy.Kind{orcKind, slimeKind}
	var enabledKinds []*enemy.Kind
	for _, k := range allKinds {
		if dbgCfg.AllowSpawn(k.Name) {
			enabledKinds = append(enabledKinds, k)
		}
	}
	if len(enabledKinds) == 0 {
		log.Fatalf("debug spawn_enemies filter excludes all kinds; known: %v, filter: %v",
			[]string{orcKind.Name, slimeKind.Name}, dbgCfg.SpawnEnemies)
	}

	timeoutS := tuneMap["game_timeout_s"]

	g := game.New(game.Deps{
		Cfg:           cfg,
		Anims:         anims,
		Physics:       physics,
		StaminaTuning: staminaTuning,
		DebugCfg:      dbgCfg,
		SoldierBoxes:  soldierBoxes,
		CombatTuning:  combatTuning,
		EnemyKinds:    enabledKinds,
		SpawnTuning:   spawnTuning,
		HeartAnim:     heart,
		StaminaAnim:   staminaAnim,
		HUDFace:       hud.NewFace(32),
		OverTitle:     hud.NewFace(96),
		OverSubtitle:  hud.NewFace(32),
		Layout:        layout,
		TimeoutS:      timeoutS,
	})

	ebiten.SetWindowSize(cfg.WindowW, cfg.WindowH)
	ebiten.SetWindowTitle("claude-pixel")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
