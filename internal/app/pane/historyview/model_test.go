package historyview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/githiago-f/lazyapi/internal/history"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/runner"
)

func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func entry(spec string, at time.Time, status int, body string) history.Entry {
	return history.Entry{
		Time: at, Spec: spec,
		Ref:     &model.OpenAPIRef{FilePath: spec, Path: "/pets", Method: "GET"},
		Request: history.Request{Method: "GET", URI: "/pets"},
		Result: runner.OpResult{Method: "GET", Path: "/pets", Outcome: runner.Passed,
			Response: &runner.ResponseInfo{Status: status, Body: body, Headers: map[string]string{"Content-Type": "application/json"}}},
	}
}

func TestHistoryDiffs(t *testing.T) {
	spec := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(spec, []byte(`openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              example: {name: rex, age: 3}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	m := New()
	m.SetSize(140, 30)
	m.SetEntries([]history.Entry{
		entry(spec, now, 200, `{"name":"rex","age":4}`),
		entry(spec, now.Add(-time.Minute), 200, `{"name":"rex","age":3}`),
	})

	m, _ = m.Update(press("d"))
	out := ansi.Strip(m.View())
	for _, want := range []string{"+1 −1", `-   "age": 3,`, `+   "age": 4,`, "status 200 → 200"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff with previous missing %q:\n%s", want, out)
		}
	}

	m, _ = m.Update(press("esc"))
	m, _ = m.Update(press("x"))
	out = ansi.Strip(m.View())
	if !strings.Contains(out, "example (200)") || !strings.Contains(out, `+   "age": 4,`) {
		t.Errorf("diff with example:\n%s", out)
	}

	// The oldest entry has no earlier response.
	m, _ = m.Update(press("esc"))
	m, _ = m.Update(press("j"))
	m, _ = m.Update(press("d"))
	if !strings.Contains(ansi.Strip(m.View()), "No earlier response") {
		t.Errorf("no previous:\n%s", ansi.Strip(m.View()))
	}

	_, cmd := m.Update(press("enter"))
	if r, ok := cmd().(ReplayMsg); !ok || r.Entry.Result.Response.Body != `{"name":"rex","age":3}` {
		t.Errorf("enter should replay the selected entry")
	}
}
