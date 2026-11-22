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

func TestRunViewFlowSteps(t *testing.T) {
	m := New()
	m.SetSize(140, 24)
	m.id = 1
	flow := runner.OpResult{Kind: runner.KindFlow, Method: "FLOW", Path: "flows/signup.js", Outcome: runner.Failed,
		Steps: []runner.Step{
			{Method: "POST", Path: "/users", Status: 201,
				Request:  &runner.RequestInfo{Method: "POST", URL: "http://x/users", Body: `{"name":"ana"}`},
				Response: &runner.ResponseInfo{Status: 201, Body: `{"id":7}`}},
			{Method: "GET", Path: "/users/{id}", Status: 500, Response: &runner.ResponseInfo{Status: 500, Body: `oops`}},
		},
		Checks: []runner.Check{
			{Kind: runner.CheckStatus, Name: "#1 POST /users · status 201 is declared", Passed: true},
			{Kind: runner.CheckStatus, Name: "#2 GET /users/{id} · status 500 is declared", Passed: false, Message: "declared: 200"},
			{Kind: runner.CheckTest, Name: "user is readable", Passed: false},
		}}
	m, _ = m.Update(resultMsg{id: 1, res: flow})
	m.running = false

	if out := ansi.Strip(m.View()); !strings.Contains(out, "▸ flow flows/signup.js") {
		t.Fatalf("collapsed flow:\n%s", out)
	}
	m, _ = m.Update(press("enter")) // expand
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "↳ 1. POST /users") || !strings.Contains(out, "↳ 2. GET /users/{id}") || m.detailFocus {
		t.Fatalf("enter should expand the flow, not focus details:\n%s", out)
	}

	m, _ = m.Update(press("j"))
	m, _ = m.Update(press("j")) // step 2
	r, _ := m.selected()
	if r.Outcome != runner.Failed || len(r.Checks) != 1 || r.Checks[0].Name != "status 500 is declared" || r.Response.Body != "oops" {
		t.Errorf("step result = %+v", r)
	}
	if out := ansi.Strip(m.View()); !strings.Contains(out, "500") {
		t.Errorf("details should show the step response:\n%s", out)
	}

	m, _ = m.Update(press("k"))
	if r, _ := m.selected(); r.Outcome != runner.Passed || r.Request.Body != `{"name":"ana"}` {
		t.Errorf("step 1 = %+v", r)
	}
	m, _ = m.Update(press("k"))
	m, _ = m.Update(press("enter")) // collapse
	if len(m.visible()) != 1 {
		t.Errorf("collapse: %d rows", len(m.visible()))
	}
}
