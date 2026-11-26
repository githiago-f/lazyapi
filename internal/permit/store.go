package permit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store is the file of remembered grants (0600, in the user's config
// directory). It is read on every check, so `lazyapi allow` run in a
// terminal applies to a running MCP server.
type Store struct {
	Path string
	mu   sync.Mutex
}

// Grant is one remembered origin.
type Grant struct {
	Origin    string    `json:"origin"`
	Writes    bool      `json:"writes"` // also POST, PUT, PATCH, DELETE…
	GrantedAt time.Time `json:"grantedAt"`
}

type storeFile struct {
	Grants []Grant `json:"grants"`
}

// DefaultStore is $LAZYAPI_PERMISSIONS, else <config dir>/lazyapi/permissions.json.
func DefaultStore() (*Store, error) {
	if p := os.Getenv("LAZYAPI_PERMISSIONS"); p != "" {
		return &Store{Path: p}, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("no config directory for permissions: %w", err)
	}
	return &Store{Path: filepath.Join(dir, "lazyapi", "permissions.json")}, nil
}

func (s *Store) load() (storeFile, error) {
	var f storeFile
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("%s: %w", s.Path, err)
	}
	return f, nil
}

func (s *Store) save(f storeFile) error {
	sort.Slice(f.Grants, func(i, j int) bool { return f.Grants[i].Origin < f.Grants[j].Origin })
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

// Allowed reports whether origin is remembered (for writes too when write).
// An unreadable file allows nothing.
func (s *Store) Allowed(origin string, write bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return false
	}
	for _, g := range f.Grants {
		if g.Origin == origin {
			return !write || g.Writes || origin == Loopback
		}
	}
	return false
}

// Grant remembers origin (with writes, or adding them to a read grant).
func (s *Store) Grant(origin string, writes bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return err
	}
	for i, g := range f.Grants {
		if g.Origin == origin {
			f.Grants[i].Writes = g.Writes || writes
			return s.save(f)
		}
	}
	f.Grants = append(f.Grants, Grant{Origin: origin, Writes: writes || origin == Loopback, GrantedAt: time.Now().UTC().Truncate(time.Second)})
	return s.save(f)
}

// Revoke forgets origin; false when it was not remembered.
func (s *Store) Revoke(origin string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	if err != nil {
		return false, err
	}
	for i, g := range f.Grants {
		if g.Origin == origin {
			f.Grants = append(f.Grants[:i], f.Grants[i+1:]...)
			return true, s.save(f)
		}
	}
	return false, nil
}

// List returns the remembered grants, sorted by origin.
func (s *Store) List() ([]Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load()
	return f.Grants, err
}
