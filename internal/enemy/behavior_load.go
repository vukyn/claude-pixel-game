package enemy

import (
	"fmt"

	"claude-pixel/internal/anim"
	"claude-pixel/internal/behavior"
)

// LoadBehavior reads a kind's JSON, resolves anim keys against the library
// (prefixing with the kind's AnimPrefix), and returns state declarations +
// the initial state id (the id of the first state in the file).
func LoadBehavior(path, animPrefix string, lib map[string]*anim.Animation) (map[string]*StateDecl, string, error) {
	f, err := behavior.LoadFile(path)
	if err != nil {
		return nil, "", err
	}
	if len(f.States) == 0 {
		return nil, "", fmt.Errorf("behavior %s: no states", path)
	}
	prefixed := make(map[string]*anim.Animation, len(lib))
	for _, s := range f.States {
		key := animPrefix + s.Anim
		a, ok := lib[key]
		if !ok {
			return nil, "", fmt.Errorf("state %q: anim %q not in library", s.ID, key)
		}
		prefixed[s.Anim] = a
	}
	decls, err := ConvertStates(f.States, prefixed)
	if err != nil {
		return nil, "", err
	}
	return decls, f.States[0].ID, nil
}
