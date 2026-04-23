package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBPath          string
	AssetsDir       string
	WindowW         int
	WindowH         int
	RenderScale     int
	DebugConfigPath string
	FontPath        string
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		DBPath:          mustString("DB_PATH"),
		AssetsDir:       mustString("ASSETS_DIR"),
		WindowW:         mustInt("WINDOW_WIDTH"),
		WindowH:         mustInt("WINDOW_HEIGHT"),
		RenderScale:     mustInt("RENDER_SCALE"),
		DebugConfigPath: mustString("DEBUG_CONFIG_PATH"),
		FontPath:        mustString("FONT_PATH"),
	}
}

func mustString(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("config: required env %q is empty or missing", key))
	}
	return v
}

func mustInt(key string) int {
	s := mustString(key)
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Sprintf("config: env %q = %q is not an integer: %v", key, s, err))
	}
	return n
}
