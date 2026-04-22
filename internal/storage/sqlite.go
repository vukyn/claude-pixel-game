package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"claude-pixel/internal/config"
)

func Open(cfg *config.Config) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	if err := applyMigrations(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func MustOpen(cfg *config.Config) *sql.DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}
