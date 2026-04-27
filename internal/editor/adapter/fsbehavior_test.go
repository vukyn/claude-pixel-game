package adapter_test

import (
	"os"
	"path/filepath"
	"testing"

	"claude-pixel/internal/editor/adapter"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFSBehavior_ListReturnsKnownKinds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "orc.json"), `{"kind":"orc","states":[{"id":"a","anim":"idle","decision":false,"exit_on":"anim_done","next":"a"}]}`)
	writeFile(t, filepath.Join(dir, "slime.json"), `{"kind":"slime","states":[{"id":"a","anim":"idle","decision":false,"exit_on":"anim_done","next":"a"},{"id":"b","anim":"run","decision":false,"exit_on":"anim_done","next":"a"}]}`)
	writeFile(t, filepath.Join(dir, "README.md"), `should be ignored`)

	s := adapter.NewFSBehavior(dir)
	refs, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("want 2 refs, got %d (%+v)", len(refs), refs)
	}
	byKind := map[string]int{}
	for _, r := range refs {
		byKind[r.Kind] = r.StateCount
	}
	if byKind["orc"] != 1 || byKind["slime"] != 2 {
		t.Fatalf("unexpected counts: %+v", byKind)
	}
}

func TestFSBehavior_GetReturnsBytes(t *testing.T) {
	dir := t.TempDir()
	body := `{"kind":"orc","states":[{"id":"a","anim":"idle","decision":false,"exit_on":"anim_done","next":"a"}]}`
	writeFile(t, filepath.Join(dir, "orc.json"), body)
	s := adapter.NewFSBehavior(dir)
	got, err := s.Get("orc")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("body mismatch:\nwant %s\ngot  %s", body, got)
	}
}

func TestFSBehavior_GetNotFound(t *testing.T) {
	s := adapter.NewFSBehavior(t.TempDir())
	if _, err := s.Get("ghost"); err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestFSBehavior_PutAtomicAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "orc.json")
	writeFile(t, target, `original`)
	s := adapter.NewFSBehavior(dir)
	if err := s.Put("orc", []byte(`updated`)); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "updated" {
		t.Fatalf("want updated, got %s", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("temp file leaked: %s", e.Name())
		}
	}
}

func TestFSBehavior_RejectsKindWithPathSeparators(t *testing.T) {
	s := adapter.NewFSBehavior(t.TempDir())
	if _, err := s.Get("../etc/passwd"); err == nil {
		t.Fatal("want error, got nil")
	}
	if err := s.Put("a/b", []byte(`x`)); err == nil {
		t.Fatal("want error, got nil")
	}
}
