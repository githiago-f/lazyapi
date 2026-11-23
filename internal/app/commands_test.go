package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		p := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("api.yml", "openapi: 3.0.0")
	write("rows.json", "  [ {\"id\": 1} ]")
	write("package.json", `{"name": "x"}`)
	write(".lazyapi/data/users.csv", "id\n1\n")
	write("notes.txt", "x")
	got := dataFiles(filepath.Join(dir, "api.yml"))
	var names []string
	for _, g := range got {
		rel, _ := filepath.Rel(dir, g)
		names = append(names, rel)
	}
	if strings.Join(names, ",") != "rows.json,.lazyapi/data/users.csv" {
		t.Errorf("data files = %v", names)
	}
}
