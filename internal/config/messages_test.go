package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every key the code asks for must be in messages.yml (a missing key would
// show its name on screen).
func TestMessagesUsedKeysExist(t *testing.T) {
	calls := regexp.MustCompile(`config\.(T|Plural)\("([^"]+)"[,)]|\b(label|placeholder)\("([a-z_]+)"\)`)
	root := filepath.Join("..", "..", "internal")
	found := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range calls.FindAllStringSubmatch(string(data), -1) {
			var keys []string
			switch {
			case m[1] == "T":
				keys = []string{m[2]}
			case m[1] == "Plural":
				keys = []string{m[2] + ".one", m[2] + ".other"}
			case m[3] == "label":
				keys = []string{"editor.auth.labels." + m[4]}
			case m[3] == "placeholder":
				keys = []string{"editor.auth.placeholders." + m[4]}
			}
			for _, k := range keys {
				found++
				if _, ok := messages[k]; !ok {
					t.Errorf("%s: %q is not in messages.yml", path, k)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found < 50 {
		t.Errorf("only %d message uses found: is the pattern still right?", found)
	}
}

func TestMessagesT(t *testing.T) {
	if got := T("editor.auth.off", "toggle", "ctrl+t"); got != "not sent · ctrl+t turns it on" {
		t.Errorf("T: %q", got)
	}
	if got := Plural("response.matches", 1); got != "1 match" {
		t.Errorf("one: %q", got)
	}
	if got := Plural("response.matches", 3); got != "3 matches" {
		t.Errorf("other: %q", got)
	}
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("missing: %q", got)
	}
}

func TestMessagesOverride(t *testing.T) {
	saved := map[string]string{}
	for k, v := range messages {
		saved[k] = v
	}
	t.Cleanup(func() { messages = saved })

	dir := t.TempDir()
	good := filepath.Join(dir, "pt.yml")
	_ = os.WriteFile(good, []byte("editor:\n  tabs:\n    auth: Autenticação\n"), 0o644)
	if err := loadOverride(good); err != nil {
		t.Fatal(err)
	}
	if T("editor.tabs.auth") != "Autenticação" || T("editor.tabs.body") != "Body" {
		t.Errorf("override merges: %q %q", T("editor.tabs.auth"), T("editor.tabs.body"))
	}

	bad := filepath.Join(dir, "typo.yml")
	_ = os.WriteFile(bad, []byte("editor:\n  tabz:\n    auth: x\n"), 0o644)
	if err := loadOverride(bad); err == nil || !strings.Contains(err.Error(), "editor.tabz.auth") {
		t.Errorf("unknown key: %v", err)
	}
	if err := loadOverride(filepath.Join(dir, "missing.yml")); err == nil {
		t.Error("missing file")
	}
}
