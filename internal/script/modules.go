package script

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// LockFileName is the lockfile written next to the spec.
const LockFileName = "lazyapi.lock"

// maxModuleSize caps a single fetched module.
const maxModuleSize = 20 << 20

// ModuleLoader fetches https:// imports, caches them on disk and pins their
// content in a lockfile so runs are reproducible.
//
//   - A module listed in the lockfile must match its recorded integrity.
//   - A new module is added to the lockfile, unless Frozen is set (CI), in
//     which case it is an error.
//   - Offline never touches the network: modules must already be cached.
//   - Reload re-downloads every module and rewrites its lock entry.
type ModuleLoader struct {
	CacheDir     string
	LockPath     string // empty disables the lockfile
	Offline      bool
	Reload       bool
	Frozen       bool
	AllowedHosts []string // empty allows any host
	Client       *http.Client

	mu     sync.Mutex
	lock   *lockFile
	dirty  bool
	loaded bool
}

type lockFile struct {
	Version int               `json:"version"`
	Modules map[string]string `json:"modules"`
}

// DefaultCacheDir returns the per-user module cache directory.
func DefaultCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "lazyapi", "modules")
}

// NewModuleLoader returns a loader using the default cache and a lockfile in
// lockDir (no lockfile when lockDir is empty).
func NewModuleLoader(lockDir string) *ModuleLoader {
	m := &ModuleLoader{CacheDir: DefaultCacheDir()}
	if lockDir != "" {
		m.LockPath = filepath.Join(lockDir, LockFileName)
	}
	return m
}

// Integrity returns the lockfile integrity string for data.
func Integrity(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

// Fetch returns the content of rawURL, enforcing scheme, host allowlist and
// lockfile integrity.
func (m *ModuleLoader) Fetch(rawURL string) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("URL imports are disabled: %s", rawURL)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid import URL %q: %w", rawURL, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("import %s: only https:// URLs are allowed", rawURL)
	}
	if len(m.AllowedHosts) > 0 && !slices.Contains(m.AllowedHosts, u.Hostname()) {
		return nil, fmt.Errorf("import %s: host %q is not in the allowed list (%s)",
			rawURL, u.Hostname(), strings.Join(m.AllowedHosts, ", "))
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadLock(); err != nil {
		return nil, err
	}
	locked, inLock := m.lock.Modules[rawURL]

	cachePath := filepath.Join(m.CacheDir, cacheKey(rawURL))
	var data []byte
	if !m.Reload {
		if cached, err := os.ReadFile(cachePath); err == nil {
			// A cached copy that does not match the lock is treated as stale.
			if !inLock || Integrity(cached) == locked {
				data = cached
			}
		}
	}

	if data == nil {
		if m.Offline {
			return nil, fmt.Errorf("import %s: not in the module cache and --offline is set", rawURL)
		}
		if data, err = m.download(rawURL); err != nil {
			return nil, err
		}
		if err := writeAtomic(cachePath, data); err != nil {
			return nil, fmt.Errorf("caching %s: %w", rawURL, err)
		}
	}

	sum := Integrity(data)
	switch {
	case inLock && sum == locked:
	case inLock && !m.Reload:
		return nil, fmt.Errorf("import %s: integrity mismatch (%s has %s, downloaded %s); "+
			"the module changed upstream — pin a version in the URL, or rerun with --reload to accept it",
			rawURL, LockFileName, locked, sum)
	case m.Frozen && m.LockPath != "":
		return nil, fmt.Errorf("import %s: not present in %s and --frozen-lockfile is set", rawURL, LockFileName)
	default:
		m.lock.Modules[rawURL] = sum
		m.dirty = true
	}
	return data, nil
}

func (m *ModuleLoader) download(rawURL string) ([]byte, error) {
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("import %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("import %s: HTTP %s", rawURL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxModuleSize+1))
	if err != nil {
		return nil, fmt.Errorf("import %s: %w", rawURL, err)
	}
	if len(data) > maxModuleSize {
		return nil, fmt.Errorf("import %s: module larger than %d bytes", rawURL, maxModuleSize)
	}
	return data, nil
}

func (m *ModuleLoader) loadLock() error {
	if m.loaded {
		return nil
	}
	m.loaded = true
	m.lock = &lockFile{Version: 1, Modules: map[string]string{}}
	if m.LockPath == "" {
		return nil
	}
	data, err := os.ReadFile(m.LockPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", m.LockPath, err)
	}
	if err := json.Unmarshal(data, m.lock); err != nil {
		return fmt.Errorf("parsing %s: %w", m.LockPath, err)
	}
	if m.lock.Modules == nil {
		m.lock.Modules = map[string]string{}
	}
	return nil
}

// SaveLock writes the lockfile if new modules were pinned.
func (m *ModuleLoader) SaveLock() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirty || m.LockPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(m.lock, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(m.LockPath, append(data, '\n')); err != nil {
		return fmt.Errorf("writing %s: %w", m.LockPath, err)
	}
	m.dirty = false
	return nil
}

func cacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
