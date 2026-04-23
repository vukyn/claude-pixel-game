package world

import "claude-pixel/internal/config"

type World struct {
	Gravity float64
	GroundY float64
}

func New(cfg *config.Config, gravity float64) *World {
	return &World{
		Gravity: gravity,
		GroundY: float64(cfg.WindowH) - 120,
	}
}

func Clamp(x, min, max float64) float64 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}
