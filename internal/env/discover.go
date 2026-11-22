package env

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover lists env files in dir: ".env", ".env.<name>" and "<name>.env".
// Templates such as ".env.example" are included; the caller decides.
func Discover(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if name == ".env" || strings.HasPrefix(name, ".env.") || strings.HasSuffix(name, ".env") {
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Strings(out)
	return out
}

// Label is a short display name for an env file path.
func Label(path string) string {
	if path == "" {
		return "no env"
	}
	return filepath.Base(path)
}
