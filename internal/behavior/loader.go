package behavior

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFile parses a behavior JSON file by path. Convenience wrapper around LoadBytes.
func LoadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("behavior: read %s: %w", path, err)
	}
	return LoadBytes(data, path)
}

// LoadBytes parses a behavior JSON document. `source` is used only for error messages.
func LoadBytes(data []byte, source string) (*File, error) {
	var raw FileRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("behavior: parse %s: %w", source, err)
	}
	return buildFile(&raw, source)
}

// FileRaw is the JSON-facing shape. Exported so callers can
// marshal/unmarshal without duplicating the tags.
type FileRaw struct {
	Kind   string     `json:"kind"`
	States []StateRaw `json:"states"`
}

type StateRaw struct {
	ID            string         `json:"id"`
	Anim          string         `json:"anim"`
	Decision      bool           `json:"decision"`
	BT            map[string]any `json:"bt,omitempty"`
	ExitOn        string         `json:"exit_on,omitempty"`
	Next          string         `json:"next,omitempty"`
	OnExitActions []string       `json:"on_exit_actions,omitempty"`
	OnFrameVX     []FrameVXRaw   `json:"on_frame_vx,omitempty"`
}

type FrameVXRaw struct {
	FrameStart int     `json:"frame_start"`
	FrameEnd   int     `json:"frame_end"`
	VX         float64 `json:"vx"`
}

// File is the parsed, validated behavior spec. Node trees are already
// constructed for decision states.
type File struct {
	Kind   string
	States []State
}

// State is an opaque per-state record the enemy package converts into its
// own typed StateDecl.
type State struct {
	ID            string
	Anim          string
	Decision      bool
	BT            *Tree
	ExitOn        string
	Next          string
	OnExitActions []string
	OnFrameVX     []FrameVX
}

type FrameVX struct {
	FrameStart int
	FrameEnd   int
	VX         float64
}

func buildFile(raw *FileRaw, path string) (*File, error) {
	if raw.Kind == "" {
		return nil, fmt.Errorf("behavior: %s: missing \"kind\"", path)
	}
	if len(raw.States) == 0 {
		return nil, fmt.Errorf("behavior: %s: states is empty", path)
	}
	out := &File{Kind: raw.Kind}
	seen := map[string]bool{}
	for i := range raw.States {
		s := raw.States[i]
		if s.ID == "" {
			return nil, fmt.Errorf("behavior: %s: state #%d missing id", path, i)
		}
		if seen[s.ID] {
			return nil, fmt.Errorf("behavior: %s: duplicate state id %q", path, s.ID)
		}
		seen[s.ID] = true
		if s.Anim == "" {
			return nil, fmt.Errorf("behavior: %s: state %q missing anim", path, s.ID)
		}
		if s.Decision && s.BT == nil {
			return nil, fmt.Errorf("behavior: %s: decision state %q missing bt", path, s.ID)
		}
		if !s.Decision && s.BT != nil {
			return nil, fmt.Errorf("behavior: %s: non-decision state %q must not have bt", path, s.ID)
		}
		ps := State{
			ID:            s.ID,
			Anim:          s.Anim,
			Decision:      s.Decision,
			ExitOn:        s.ExitOn,
			Next:          s.Next,
			OnExitActions: append([]string(nil), s.OnExitActions...),
		}
		for _, fv := range s.OnFrameVX {
			ps.OnFrameVX = append(ps.OnFrameVX, FrameVX{FrameStart: fv.FrameStart, FrameEnd: fv.FrameEnd, VX: fv.VX})
		}
		if s.Decision {
			root, err := buildNode(s.BT)
			if err != nil {
				return nil, fmt.Errorf("behavior: %s: state %q: %w", path, s.ID, err)
			}
			ps.BT = &Tree{Root: root}
		}
		out.States = append(out.States, ps)
	}
	if err := validateTransitions(out, path); err != nil {
		return nil, err
	}
	return out, nil
}

func validateTransitions(f *File, path string) error {
	ids := map[string]bool{}
	for _, s := range f.States {
		ids[s.ID] = true
	}
	for _, s := range f.States {
		if !s.Decision {
			if s.Next != "" && s.Next != "__dead" && !ids[s.Next] {
				return fmt.Errorf("behavior: %s: state %q next=%q undeclared", path, s.ID, s.Next)
			}
		}
		for _, a := range s.OnExitActions {
			if !HasAction(a) {
				return fmt.Errorf("behavior: %s: state %q on_exit_actions: unknown action %q", path, s.ID, a)
			}
		}
		if s.BT != nil {
			if err := validateGotos(s.BT.Root, ids, path, s.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateGotos(n Node, ids map[string]bool, path, state string) error {
	switch v := n.(type) {
	case *Selector:
		for _, c := range v.Children {
			if err := validateGotos(c, ids, path, state); err != nil {
				return err
			}
		}
	case *Sequence:
		for _, c := range v.Children {
			if err := validateGotos(c, ids, path, state); err != nil {
				return err
			}
		}
	case *Chance:
		for _, b := range v.Branches {
			if err := validateGotos(b.Node, ids, path, state); err != nil {
				return err
			}
		}
	case *Action:
		if v.Name == "goto" {
			tgt, _ := argString(v.Args, "state")
			if !ids[tgt] {
				return fmt.Errorf("behavior: %s: state %q goto target %q undeclared", path, state, tgt)
			}
		}
	}
	return nil
}

func buildNode(raw map[string]any) (Node, error) {
	t, _ := raw["type"].(string)
	switch t {
	case "selector":
		children, err := buildChildren(raw)
		if err != nil {
			return nil, err
		}
		return &Selector{Children: children}, nil
	case "sequence":
		children, err := buildChildren(raw)
		if err != nil {
			return nil, err
		}
		return &Sequence{Children: children}, nil
	case "chance":
		branchesRaw, ok := raw["branches"].([]any)
		if !ok || len(branchesRaw) == 0 {
			return nil, fmt.Errorf("chance node has empty branches")
		}
		var branches []ChanceBranch
		for i, b := range branchesRaw {
			bm, _ := b.(map[string]any)
			w, err := argFloat(bm, "weight")
			if err != nil {
				return nil, fmt.Errorf("chance branch #%d: %w", i, err)
			}
			if w != float64(int(w)) {
				return nil, fmt.Errorf("chance branch #%d: weight must be integer, got %v", i, w)
			}
			if w <= 0 {
				return nil, fmt.Errorf("chance branch #%d: weight must be > 0", i)
			}
			nodeRaw, ok := bm["node"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("chance branch #%d: missing node", i)
			}
			child, err := buildNode(nodeRaw)
			if err != nil {
				return nil, err
			}
			branches = append(branches, ChanceBranch{Weight: int(w), Node: child})
		}
		return &Chance{Branches: branches}, nil
	case "wait":
		s, err := argFloat(raw, "seconds")
		if err != nil {
			return nil, err
		}
		return &Wait{Seconds: s}, nil
	case "action":
		name, _ := raw["name"].(string)
		if !HasAction(name) {
			return nil, fmt.Errorf("unknown action %q", name)
		}
		args, _ := raw["args"].(map[string]any)
		return &Action{Name: name, Args: args}, nil
	case "condition":
		name, _ := raw["name"].(string)
		if !HasCondition(name) {
			return nil, fmt.Errorf("unknown condition %q", name)
		}
		args, _ := raw["args"].(map[string]any)
		return &Condition{Name: name, Args: args}, nil
	}
	return nil, fmt.Errorf("unknown node type %q", t)
}

func buildChildren(raw map[string]any) ([]Node, error) {
	arr, _ := raw["children"].([]any)
	if len(arr) == 0 {
		return nil, fmt.Errorf("%q has no children", raw["type"])
	}
	var out []Node
	for i, c := range arr {
		cm, _ := c.(map[string]any)
		child, err := buildNode(cm)
		if err != nil {
			return nil, fmt.Errorf("child #%d: %w", i, err)
		}
		out = append(out, child)
	}
	return out, nil
}
