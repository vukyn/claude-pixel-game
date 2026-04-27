package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"claude-pixel/internal/editor/port"
)

type FSBehavior struct{ dir string }

func NewFSBehavior(dir string) *FSBehavior { return &FSBehavior{dir: dir} }

func (s *FSBehavior) List() ([]port.BehaviorRef, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("fsbehavior: list %s: %w", s.dir, err)
	}
	var refs []port.BehaviorRef
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var head struct {
			Kind   string `json:"kind"`
			States []any  `json:"states"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			continue
		}
		if head.Kind == "" {
			continue
		}
		refs = append(refs, port.BehaviorRef{Kind: head.Kind, Path: path, StateCount: len(head.States)})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Kind < refs[j].Kind })
	return refs, nil
}

func (s *FSBehavior) Get(kind string) ([]byte, error) {
	if err := validateKind(kind); err != nil {
		return nil, err
	}
	path := filepath.Join(s.dir, kind+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("fsbehavior: kind %q not found", kind)
		}
		return nil, fmt.Errorf("fsbehavior: read %q: %w", kind, err)
	}
	return data, nil
}

func (s *FSBehavior) Put(kind string, raw []byte) error {
	if err := validateKind(kind); err != nil {
		return err
	}
	path := filepath.Join(s.dir, kind+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("fsbehavior: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("fsbehavior: rename: %w", err)
	}
	return nil
}

func validateKind(kind string) error {
	if kind == "" || strings.ContainsAny(kind, `/\.`) {
		return fmt.Errorf("fsbehavior: invalid kind %q", kind)
	}
	return nil
}
