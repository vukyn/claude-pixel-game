package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/config"
	"claude-pixel/internal/debug"
	"claude-pixel/internal/game"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	cfg := config.Load()

	db := storage.MustOpen(cfg)
	defer db.Close()

	animRepo := storage.NewRepository[anim.AnimationSpec](db, anim.SpecMapper{})
	tuneRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})

	anims, err := anim.LoadLibrary(cfg, animRepo)
	if err != nil {
		log.Fatalf("load animations: %v", err)
	}
	physics, err := player.LoadPhysics(tuneRepo)
	if err != nil {
		log.Fatalf("load physics: %v", err)
	}
	dbgCfg, err := debug.LoadConfig(cfg.DebugConfigPath)
	if err != nil {
		log.Fatalf("load debug config: %v", err)
	}

	g := game.New(cfg, anims, physics, dbgCfg)

	ebiten.SetWindowSize(cfg.WindowW, cfg.WindowH)
	ebiten.SetWindowTitle("claude-pixel")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
