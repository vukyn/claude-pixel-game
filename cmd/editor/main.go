package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	port := os.Getenv("EDITOR_PORT")
	if port == "" {
		port = "8080"
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})
	addr := ":" + port
	log.Printf("editor server listening on %s", addr)
	if err := app.Listen(addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
