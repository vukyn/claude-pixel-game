package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"

	"claude-pixel/internal/config"
	"claude-pixel/internal/editor/adapter"
	editorhttp "claude-pixel/internal/editor/http"
	"claude-pixel/internal/editor/service"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

func main() {
	_ = godotenv.Load()
	port := os.Getenv("EDITOR_PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/game.db"
	}
	behaviorsDir := os.Getenv("BEHAVIORS_DIR")
	if behaviorsDir == "" {
		behaviorsDir = "assets/behaviors"
	}

	db, err := storage.Open(&config.Config{DBPath: dbPath})
	if err != nil {
		fatal("open db: %v", err)
	}
	defer db.Close()

	tuningRepo := storage.NewRepository[player.TuningParam](db, player.TuningMapper{})

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	editorhttp.DefaultMiddleware(app)
	editorhttp.Register(app, editorhttp.Deps{
		Behavior: service.NewBehavior(adapter.NewFSBehavior(behaviorsDir)),
		Tuning:   service.NewTuning(adapter.NewSQLiteTuning(tuningRepo)),
		Registry: service.NewRegistry(adapter.NewRuntimeRegistry()),
	})

	addr := ":" + port
	log.Printf("editor server listening on %s (db=%s, behaviors=%s)", addr, dbPath, behaviorsDir)
	if err := app.Listen(addr); err != nil {
		fatal("listen: %v", err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
