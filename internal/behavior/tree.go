package behavior

import (
	"math/rand"
	"time"
)

type Status int

const (
	StatusSuccess Status = iota
	StatusFailure
	StatusRunning
)

func (s Status) String() string {
	switch s {
	case StatusSuccess:
		return "success"
	case StatusFailure:
		return "failure"
	case StatusRunning:
		return "running"
	}
	return "?"
}

// Node is the behavior-tree node contract. Tick mutates Ctx and returns a
// Status. Pure-data nodes (e.g. Chance after it has picked a branch) hold
// their own small state across ticks — the Tree is re-used per enemy so
// each enemy has its own Tree instance.
type Node interface {
	Tick(ctx *Ctx) Status
}

// Tree is a per-enemy wrapper around a root Node. It has no fields for now
// but keeps the door open for root-level metadata (e.g. a tick counter used
// by debug overlays).
type Tree struct {
	Root Node
}

// Ctx threads per-tick state through the tree.
type Ctx struct {
	Enemy       EnemyTarget
	DT          time.Duration
	RNG         *rand.Rand
	PendingGoto string
	BranchTag   string
}

// SetBranch appends a segment to the breadcrumb path shown in debug.
func (c *Ctx) SetBranch(seg string) {
	if c.BranchTag == "" {
		c.BranchTag = seg
		return
	}
	c.BranchTag += "/" + seg
}
