package recents_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"liteview/internal/recents"
)

func newStore(t *testing.T) (*recents.Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg", "recents.json") // parent dir does not exist yet
	return recents.New(p), p
}

func TestList_missingFileIsEmpty(t *testing.T) {
	s, _ := newStore(t)
	got := s.List()
	if got == nil {
		t.Fatal("expected non-nil empty slice for missing file, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestList_corruptFileIsEmpty(t *testing.T) {
	s, p := newStore(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := s.List()
	if got == nil {
		t.Fatal("expected non-nil empty slice for corrupt file, got nil")
	}
	if len(got) != 0 {
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
	if err := s.Add("/a.db"); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("/b.db"); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("/a.db"); err != nil {
		t.Fatal(err)
	}
	if got := s.List(); !reflect.DeepEqual(got, []string{"/a.db", "/b.db"}) {
		t.Fatalf("mru wrong: %v", got)
	}
	for i := 0; i < 15; i++ {
		if err := s.Add(fmt.Sprintf("/f%d.db", i)); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.List(); len(got) != 10 || got[0] != "/f14.db" {
		t.Fatalf("cap wrong: %v", got)
	}
}

func TestRemove(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Add("/a.db"); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("/b.db"); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(real, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(real); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("/definitely/missing.db"); err != nil {
		t.Fatal(err)
	}
	if got := s.Last(); got != real {
		t.Fatalf("Last = %q, want %q", got, real)
	}
}

func TestList_fileWithJsonNull_returnsNonNilEmpty(t *testing.T) {
	s, p := newStore(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("null"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := s.List()
	if got == nil {
		t.Fatal("expected non-nil empty slice for JSON null file, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	// Verify that List() result can be marshaled to valid JSON
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("failed to marshal List() result: %v", err)
	}
	if string(b) != "[]" {
		t.Fatalf("expected marshaled List() to be '[]', got %q", string(b))
	}
}

func TestList_fileWithEmptyArray_returnsNonNilEmpty(t *testing.T) {
	s, p := newStore(t)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := s.List()
	if got == nil {
		t.Fatal("expected non-nil empty slice for empty JSON array, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestRemove_onlyEntry_leavesNonNilEmptyAndNoNull(t *testing.T) {
	s, p := newStore(t)
	if err := s.Add("/only.db"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("/only.db"); err != nil {
		t.Fatal(err)
	}
	got := s.List()
	if got == nil {
		t.Fatal("expected non-nil empty slice after removing only entry, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	// Verify file on disk contains [] not null
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(b) != "[]" {
		t.Fatalf("expected file to contain '[]', got %q", string(b))
	}
}
