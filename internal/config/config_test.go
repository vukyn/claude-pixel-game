package config

import (
	"os"
	"testing"
)

func TestLoadReadsEnvVars(t *testing.T) {
	os.Setenv("DB_PATH", "/tmp/x.db")
	os.Setenv("ASSETS_DIR", "/tmp/assets")
	os.Setenv("WINDOW_WIDTH", "1280")
	os.Setenv("WINDOW_HEIGHT", "720")
	os.Setenv("RENDER_SCALE", "3")
	os.Setenv("DEBUG_CONFIG_PATH", "/tmp/debug.json")
	os.Setenv("FONT_PATH", "/fonts/m.ttf")
	t.Cleanup(func() {
		for _, k := range []string{"DB_PATH", "ASSETS_DIR", "WINDOW_WIDTH", "WINDOW_HEIGHT", "RENDER_SCALE", "DEBUG_CONFIG_PATH", "FONT_PATH"} {
			os.Unsetenv(k)
		}
	})

	cfg := Load()
	if cfg.DBPath != "/tmp/x.db" ||
		cfg.AssetsDir != "/tmp/assets" ||
		cfg.WindowW != 1280 ||
		cfg.WindowH != 720 ||
		cfg.RenderScale != 3 ||
		cfg.DebugConfigPath != "/tmp/debug.json" ||
		cfg.FontPath != "/fonts/m.ttf" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadPanicsOnMissingKey(t *testing.T) {
	original, had := os.LookupEnv("DB_PATH")
	os.Unsetenv("DB_PATH")
	t.Cleanup(func() {
		if had {
			os.Setenv("DB_PATH", original)
		} else {
			os.Unsetenv("DB_PATH")
		}
	})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when DB_PATH missing")
		}
	}()
	Load()
}
