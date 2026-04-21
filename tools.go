//go:build tools

package main

import (
	_ "github.com/hajimehoshi/ebiten/v2"
	_ "github.com/joho/godotenv"
	_ "github.com/urfave/cli/v3"
	_ "modernc.org/sqlite"
)
