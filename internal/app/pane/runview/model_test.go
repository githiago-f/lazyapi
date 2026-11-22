package runview

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/runner"
)

const spec = `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /ok:
    get:
      operationId: okOp
      responses:
        "200": {description: ok}
  /broken:
    get:
      operationId: brokenOp
      responses:
        "200": {description: ok}
  /write:
    post:
      operationId: writeOp
      responses:
        "201": {description: created}
`

func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// drain feeds the run's messages to the model until it finishes.
func drain(t *testing.T, m Model) Model {
	t.Helper()
	for m.Running() {
		m, _ = m.Update(<-m.ch)
	}
	return m
}

func TestRunView(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broken" {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	doc, err := openapi3.NewLoader().LoadFromData([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}

	m := New()
	m.SetSize(120, 20)
	m.Start(Run{Title: "spec", Spec: doc, SpecPath: "spec.yml",
		Options: runner.Options{Server: srv.URL, Filter: runner.Filter{Methods: runner.SafeMethods}}})
	if m.planned != 3 {
		t.Fatalf("planned = %d", m.planned)
	}
	m = drain(t, m)

	out := ansi.Strip(m.View())
	for _, want := range []string{"✓ 1 passed", "✗ 1 failed", "○ 1 skipped", "GET /broken", "500", "finished"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	// The cursor lands on the failure.
	if r, _ := m.selected(); r.Path != "/broken" {
		t.Errorf("selected %s, want the failure", r.Label())
	}

	m, _ = m.Update(press("F"))
	if v := m.visible(); len(v) != 1 {
		t.Errorf("F should show only failures, got %d rows", len(v))
	}
	_, cmd := m.Update(press("e"))
	if open, ok := cmd().(OpenOperationMsg); !ok || open.Ref.Path != "/broken" || open.Ref.Method != "GET" {
		t.Errorf("e should open the operation, got %#v", cmd())
	}

	m, _ = m.Update(press("enter"))
	if !m.detailFocus {
		t.Error("enter focuses the details")
	}
	m, _ = m.Update(press("esc"))
	if m.detailFocus {
		t.Error("esc returns to the list")
	}
	_, cmd = m.Update(press("esc"))
	if _, ok := cmd().(CloseMsg); !ok {
		t.Error("esc on the list closes the view")
	}

	// Rerun ignores messages of the previous run.
	m.Start(m.run)
	m, _ = m.Update(resultMsg{id: m.id - 1, res: runner.OpResult{Path: "/stale"}})
	m = drain(t, m)
	for _, r := range m.Results() {
		if r.Path == "/stale" {
			t.Error("stale result accepted")
		}
	}
}
