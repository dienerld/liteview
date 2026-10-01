package recents_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sqliteviewer/internal/recents"
)

func newStore(t *testing.T) (*recents.Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg", "recents.json") // parent dir does not exist yet
	return recents.New(p), p
}

func TestList_missingFileIsEmpty(t *testing.T) {
	s, _ := newStore(t)
	if got := s.List(); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestList_corruptFileIsEmpty(t *testing.T) {
	s, p := newStore(t)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("{not json"), 0o644)
	if got := s.List(); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	if err := s.Add("/a.db"); err != nil { // and it recovers on next write
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.List(), []string{"/a.db"}) {
		t.Fatalf("did not recover: %v", s.List())
	}
}

func TestAdd_mruDedupeAndCap(t *testing.T) {
	s, _ := newStore(t)
	s.Add("/a.db")
	s.Add("/b.db")
	s.Add("/a.db")
	if got := s.List(); !reflect.DeepEqual(got, []string{"/a.db", "/b.db"}) {
		t.Fatalf("mru wrong: %v", got)
	}
	for i := 0; i < 15; i++ {
		s.Add(fmt.Sprintf("/f%d.db", i))
	}
	if got := s.List(); len(got) != 10 || got[0] != "/f14.db" {
		t.Fatalf("cap wrong: %v", got)
	}
}

func TestRemove(t *testing.T) {
	s, _ := newStore(t)
	s.Add("/a.db")
	s.Add("/b.db")
	if err := s.Remove("/a.db"); err != nil {
		t.Fatal(err)
	}
	if got := s.List(); !reflect.DeepEqual(got, []string{"/b.db"}) {
		t.Fatalf("remove wrong: %v", got)
	}
}

func TestLast_skipsMissingFiles(t *testing.T) {
	s, _ := newStore(t)
	real := filepath.Join(t.TempDir(), "real.db")
	os.WriteFile(real, []byte("x"), 0o644)
	s.Add(real)
	s.Add("/definitely/missing.db")
	if got := s.Last(); got != real {
		t.Fatalf("Last = %q, want %q", got, real)
	}
}
