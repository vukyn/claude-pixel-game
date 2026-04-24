package enemy

import (
	"claude-pixel/internal/anim"
	"claude-pixel/internal/combat"
	"claude-pixel/internal/player"
	"claude-pixel/internal/storage"
)

// AttackMotion describes horizontal displacement applied during a specific
// frame window of an attack/attack2 state. VX is signed: positive = forward
// along the facing direction; negative = backward.
type AttackMotion struct {
	VX         float64
	FrameStart int
	FrameEnd   int
}

// Kind bundles per-enemy-kind metadata. All animation keys in Anims are
// unprefixed ("idle", "run", "attack", "attack2", "hurt", "death") so FSM
// states use static strings regardless of owner.
type Kind struct {
	Name         string
	AnimPrefix   string
	FrameW       int
	FrameH       int
	Tuning       *Tuning
	Boxes        map[string]combat.Box
	Anims        map[string]*anim.Animation
	Motions      map[string]AttackMotion
	States       map[string]*StateDecl
	InitialState string
	BehaviorPath string
}

type KindConfig struct {
	Name         string
	Prefix       string
	FrameW       int
	FrameH       int
	AnimLib      map[string]*anim.Animation
	HitboxSpecs  []combat.HitboxSpec
	MotionSpecs  []combat.AttackMotionSpec
	TuneRepo     *storage.Repository[player.TuningParam]
	RenderScale  int
	BehaviorPath string
}

func BuildKind(cfg KindConfig) (*Kind, error) {
	anims, err := AnimsFor(cfg.AnimLib, cfg.Prefix)
	if err != nil {
		return nil, err
	}
	boxes, err := BoxesFor(cfg.HitboxSpecs, cfg.Name, cfg.RenderScale)
	if err != nil {
		return nil, err
	}
	tuning, err := LoadTuningFor(cfg.TuneRepo, cfg.Prefix)
	if err != nil {
		return nil, err
	}
	motions := MotionsFor(cfg.MotionSpecs, cfg.Name)
	k := &Kind{
		Name:         cfg.Name,
		AnimPrefix:   cfg.Prefix,
		FrameW:       cfg.FrameW,
		FrameH:       cfg.FrameH,
		Tuning:       tuning,
		Boxes:        boxes,
		Anims:        anims,
		Motions:      motions,
		BehaviorPath: cfg.BehaviorPath,
	}
	if cfg.BehaviorPath != "" {
		states, initial, err := LoadBehavior(cfg.BehaviorPath, cfg.Prefix+"_", cfg.AnimLib)
		if err != nil {
			return nil, err
		}
		k.States = states
		k.InitialState = initial
	}
	return k, nil
}
