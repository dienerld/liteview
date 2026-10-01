package recents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const maxEntries = 10

type Store struct {
	mu   sync.Mutex
	path string
}

func New(path string) *Store { return &Store{path: path} }

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sqliteviewer", "recents.json"), nil
}

func (s *Store) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) load() []string {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return []string{}
	}
	list := []string{}
	if err := json.Unmarshal(b, &list); err != nil {
		return []string{}
	}
	if list == nil {
		return []string{}
	}
	return list
}

func (s *Store) Add(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := []string{p}
	for _, x := range s.load() {
		if x != p {
			list = append(list, x)
		}
	}
	if len(list) > maxEntries {
		list = list[:maxEntries]
	}
	return s.save(list)
}

func (s *Store) Remove(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := []string{}
	for _, x := range s.load() {
		if x != p {
			list = append(list, x)
		}
	}
	return s.save(list)
}

func (s *Store) Last() string {
	for _, p := range s.List() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (s *Store) save(list []string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
