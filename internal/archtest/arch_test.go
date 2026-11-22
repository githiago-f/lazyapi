// Package archtest checks dependency rules between packages.
package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// core packages are shared by the CLI, the MCP server and the TUI: they must
// not import UI code (internal/app, internal/components), which would also
// risk import cycles (store → app/pane/requests → responses → runner → store).
var core = []string{
	"diff", "env", "history", "importer", "inspect", "jq", "mcp", "mock",
	"model", "project", "report", "runner", "script", "store",
}

func TestCoreDoesNotImportUI(t *testing.T) {
	for _, pkg := range core {
		dir := filepath.Join("..", pkg)
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Errorf("%s: no Go files (%v)", pkg, err)
			continue
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range parsed.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				// bubbletea itself is allowed: store/file_system.go has tea.Cmd wrappers.
				if strings.Contains(path, "/internal/app") || strings.Contains(path, "/internal/components") {
					t.Errorf("%s imports %s: core packages must not depend on the TUI", f, path)
				}
			}
		}
	}
	if _, err := os.Stat(filepath.Join("..", "app")); err != nil {
		t.Fatalf("layout changed: %v", err)
	}
}
