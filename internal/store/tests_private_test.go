package store

import (
	"path/filepath"
	"testing"
)

func TestTestsPathRefusesPrivateDirs(t *testing.T) {
	spec := filepath.Join(t.TempDir(), "api.yml")
	for _, rel := range []string{".lazyapi/.local/tokens.json", ".lazyapi/secrets/dev.env", "./.lazyapi/secrets"} {
		if _, err := resolveTestsPath(spec, rel); err == nil {
			t.Errorf("%s must be refused", rel)
		}
	}
	if _, err := resolveTestsPath(spec, ".lazyapi/tests/me.js"); err != nil {
		t.Error(err)
	}
}
