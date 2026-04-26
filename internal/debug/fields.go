package debug

import (
	"fmt"

	"claude-pixel/internal/input"
	"claude-pixel/internal/player"
)

type FieldSource interface {
	Player() *player.Player
	Intent() *input.Intent
	EngineFPS() float64
	EngineTPS() float64
	EnemyCount() int
	NextSpawnS() float64
	NearestEnemyState() string
	NearestEnemyBranch() string
}

type Field struct {
	Key    string
	Format func(s FieldSource) string
}

var Catalog = map[string]Field{
	"state":           {"state", func(s FieldSource) string { return "State: " + string(s.Player().FSM.CurrentID()) }},
	"facing":          {"facing", func(s FieldSource) string { return fmt.Sprintf("Facing: %+d", s.Player().Facing) }},
	"grounded":        {"grounded", func(s FieldSource) string { return fmt.Sprintf("Grounded: %t", s.Player().Grounded) }},
	"x":               {"x", func(s FieldSource) string { return fmt.Sprintf("X: %.2f", s.Player().X) }},
	"y":               {"y", func(s FieldSource) string { return fmt.Sprintf("Y: %.2f", s.Player().Y) }},
	"vx":              {"vx", func(s FieldSource) string { return fmt.Sprintf("VX: %.2f", s.Player().VX) }},
	"vy":              {"vy", func(s FieldSource) string { return fmt.Sprintf("VY: %.2f", s.Player().VY) }},
	"anim_id":         {"anim_id", func(s FieldSource) string {
		a := s.Player().Current
		if a == nil {
			return "AnimID: -"
		}
		return "AnimID: " + a.SpecID()
	}},
	"anim_frame": {"anim_frame", func(s FieldSource) string {
		a := s.Player().Current
		if a == nil {
			return "Frame: -"
		}
		return fmt.Sprintf("Frame: %d", a.FrameIndex())
	}},
	"anim_elapsed_ms": {"anim_elapsed_ms", func(s FieldSource) string {
		a := s.Player().Current
		if a == nil {
			return "Elapsed: -"
		}
		return fmt.Sprintf("Elapsed: %d ms", a.Elapsed().Milliseconds())
	}},
	"intent_left":    {"intent_left", func(s FieldSource) string { return fmt.Sprintf("Left: %t", s.Intent().Left) }},
	"intent_right":   {"intent_right", func(s FieldSource) string { return fmt.Sprintf("Right: %t", s.Intent().Right) }},
	"intent_jump":    {"intent_jump", func(s FieldSource) string { return fmt.Sprintf("Jump: %t", s.Intent().JumpPressed) }},
	"intent_sprint":  {"intent_sprint", func(s FieldSource) string { return fmt.Sprintf("Sprint: %t", s.Intent().SprintHeld) }},
	"intent_attack":  {"intent_attack", func(s FieldSource) string { return fmt.Sprintf("Attack: %t", s.Intent().AttackPressed) }},
	"intent_attack2": {"intent_attack2", func(s FieldSource) string { return fmt.Sprintf("Attack2: %t", s.Intent().Attack2Pressed) }},
	"fps":            {"fps", func(s FieldSource) string { return fmt.Sprintf("FPS: %.1f", s.EngineFPS()) }},
	"tps":            {"tps", func(s FieldSource) string { return fmt.Sprintf("TPS: %.1f", s.EngineTPS()) }},
	"enemy_count":         {"enemy_count", func(s FieldSource) string { return fmt.Sprintf("Enemies: %d", s.EnemyCount()) }},
	"orc_next_spawn_s":    {"orc_next_spawn_s", func(s FieldSource) string { return fmt.Sprintf("NextSpawn: %.2fs", s.NextSpawnS()) }},
	"player_lives":        {"player_lives", func(s FieldSource) string { return fmt.Sprintf("Lives: %d", s.Player().Lives) }},
	"player_invulnerable": {"player_invulnerable", func(s FieldSource) string { return fmt.Sprintf("Invul: %t", s.Player().HitFlag) }},
	"enemy_state":          {"enemy_state", func(s FieldSource) string { return "EnemyState: " + s.NearestEnemyState() }},
	"enemy_bt_last_branch": {"enemy_bt_last_branch", func(s FieldSource) string { return "EnemyBT: " + s.NearestEnemyBranch() }},
}
