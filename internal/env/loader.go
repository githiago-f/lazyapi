// Package env resolves and loads the environment variables into
// app's context
package env

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
)

// Load reads the given dotenv file and merges it with system environment
// variables. System env is loaded first; .env values override system values.
// If filepath is empty, returns only system env.
func Load(filepath string) (map[string]string, error) {
	if filepath == "" {
		return loadSystemEnv(), nil
	}
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	return mergeDotenv(loadSystemEnv(), string(data)), nil
}

func loadSystemEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

// Store is a stateful wrapper around Load that caches the result and only
// re-reads the files when their content changes. It merges several env
// files in order (later files win): an environment's committed variables,
// then its secrets, then an explicit --env file.
type Store struct {
	paths []string
	env   map[string]string
	hash  string
}

// NewStore reads the given env files ("" entries are ignored).
func NewStore(paths ...string) *Store {
	s := &Store{}
	s.SetPaths(paths...)
	return s
}

// Path returns the last env file in use ("" means system environment only).
func (s *Store) Path() string {
	if len(s.paths) == 0 {
		return ""
	}
	return s.paths[len(s.paths)-1]
}

// Paths returns every env file in use, in merge order.
func (s *Store) Paths() []string { return append([]string(nil), s.paths...) }

// SetPath switches to another env file and drops the cache.
func (s *Store) SetPath(path string) { s.SetPaths(path) }

// SetPaths switches to other env files and drops the cache.
func (s *Store) SetPaths(paths ...string) {
	s.paths = nil
	for _, p := range paths {
		if p != "" {
			s.paths = append(s.paths, p)
		}
	}
	s.env = nil
	s.hash = ""
}

func (s *Store) read() ([]string, string, error) {
	var datas []string
	h := md5.New()
	for _, p := range s.paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, "", err
		}
		datas = append(datas, string(data))
		h.Write([]byte(p))
		h.Write(data)
	}
	return datas, hex.EncodeToString(h.Sum(nil)), nil
}

// Load returns the current environment map, re-reading the files only if
// their content has changed since the last call. Thread-safe for
// single-goroutine TUI use (bubbletea is single-threaded).
func (s *Store) Load() (map[string]string, error) {
	if len(s.paths) == 0 {
		if s.env == nil {
			s.env = loadSystemEnv()
		}
		return s.env, nil
	}
	datas, hash, err := s.read()
	if err != nil {
		return nil, err
	}
	if hash == s.hash && s.env != nil {
		return s.env, nil
	}
	env := loadSystemEnv()
	for _, d := range datas {
		env = mergeDotenv(env, d)
	}
	s.env, s.hash = env, hash
	return s.env, nil
}

// FileValues returns only the variables defined in the env files (not the
// process environment); nil when there is no file.
func (s *Store) FileValues() map[string]string {
	if len(s.paths) == 0 {
		return nil
	}
	datas, _, err := s.read()
	if err != nil {
		return nil
	}
	var out map[string]string
	for _, d := range datas {
		out = mergeDotenv(out, d)
	}
	return out
}

// ForceReload always re-reads the file and updates the cache.
func (s *Store) ForceReload() (map[string]string, error) {
	s.hash = ""
	return s.Load()
}

func mergeDotenv(base map[string]string, data string) map[string]string {
	out := make(map[string]string, len(base))
	for k, v := range base {
		out[k] = v
	}
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		key, val, ok := parseEnvLine(scanner.Text())
		if ok {
			out[key] = val
		}
	}
	return out
}

func parseEnvLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key = strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	// "double quoted" values may use Go/JSON escapes (\n, \", \\).
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		if u, err := strconv.Unquote(val); err == nil {
			return key, u, true
		}
	}
	val = strings.Trim(val, `"'`)
	return key, val, true
}
