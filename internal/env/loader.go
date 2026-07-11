// Package env resolves and loads the environment variables into
// app's context
package env

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"os"
	"strings"
)

// Load reads the given dotenv file and merges it with system environment
// variables. System env is loaded first; .env values override system values.
// If filepath is empty, returns only system env.
func Load(filepath string) (map[string]string, error) {
	env := loadSystemEnv()

	if filepath == "" {
		return env, nil
	}

	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, val, ok := parseEnvLine(scanner.Text())
		if ok {
			env[key] = val
		}
	}

	return env, scanner.Err()
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
// re-reads the file when its content hash changes.
type Store struct {
	filepath string
	env      map[string]string
	hash     string
}

func NewStore(filepath string) *Store {
	return &Store{filepath: filepath}
}

// Load returns the current environment map, re-reading the file only if its
// content has changed since the last call. Thread-safe for single-goroutine
// TUI use (bubbletea is single-threaded).
func (s *Store) Load() (map[string]string, error) {
	if s.filepath == "" {
		if s.env == nil {
			s.env = loadSystemEnv()
		}
		return s.env, nil
	}

	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return nil, err
	}

	h := md5.Sum(data)
	hash := hex.EncodeToString(h[:])

	if hash == s.hash && s.env != nil {
		return s.env, nil
	}

	env := loadSystemEnv()
	env = mergeDotenv(env, string(data))

	s.env = env
	s.hash = hash
	return s.env, nil
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
	val = strings.Trim(val, `"'`)
	return key, val, true
}
