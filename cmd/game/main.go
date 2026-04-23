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

	// Load raw tuning once; combat + enemy tuning pull from the same map.
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
	orcTuning, err := enemy.LoadTuning(tuneRepo)
	if err != nil {
		log.Fatalf("load orc tuning: %v", err)
	}

	hitboxSpecs, err := hitboxRepo.List(context.Background())
	if err != nil {
		log.Fatalf("list hitboxes: %v", err)
	}
	soldierBoxes, err := combat.SoldierBoxes(hitboxSpecs)
	if err != nil {
		log.Fatalf("load soldier boxes: %v", err)
	}
	orcBoxes, err := enemy.OrcBoxes(hitboxSpecs)
	if err != nil {
		log.Fatalf("load orc boxes: %v", err)
	}
	orcAnims, err := enemy.OrcAnims(anims)
	if err != nil {
		log.Fatalf("pick orc anims: %v", err)
	}
	heart, ok := anims["heart_beat"]
	if !ok {
		log.Fatalf("missing heart_beat anim")
	}
	dbgCfg, err := debug.LoadConfig(cfg.DebugConfigPath)
	if err != nil {
		log.Fatalf("load debug config: %v", err)
	}
	if err := hud.LoadFont(cfg.FontPath); err != nil {
		log.Fatalf("load font: %v", err)
	}

	g := game.New(game.Deps{
		Cfg:          cfg,
		Anims:        anims,
		Physics:      physics,
		DebugCfg:     dbgCfg,
		SoldierBoxes: soldierBoxes,
		CombatTuning: combatTuning,
		OrcAnims:     orcAnims,
		OrcBoxes:     orcBoxes,
		OrcTuning:    orcTuning,
		HeartAnim:    heart,
		HUDFace:      hud.NewFace(32),
		OverTitle:    hud.NewFace(96),
		OverSubtitle: hud.NewFace(32),
	})

	ebiten.SetWindowSize(cfg.WindowW, cfg.WindowH)
	ebiten.SetWindowTitle("claude-pixel")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
