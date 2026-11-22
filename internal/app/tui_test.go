package app

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/config"
	"github.com/githiago-f/lazyapi/internal/history"
)

const tuiSpec = `openapi: 3.0.0
info: {title: Shop, version: "1"}
servers:
  - url: %s
paths:
  /health:
    get:
      operationId: health
      summary: Service health
      tags: [system]
      responses: {"200": {description: ok}}
  /orders:
    post:
      operationId: createOrder
      summary: Create an order
      tags: [orders]
      responses: {"201": {description: created}}
`

// newTUI starts the whole TUI on a spec served by an httptest server.
func newTUI(t *testing.T) (*teatest.TestModel, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(201)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"up"}`))
		}
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	spec := filepath.Join(dir, "shop.yml")
	if err := os.WriteFile(spec, []byte(fmt.Sprintf(tuiSpec, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}
	return start(t, spec), srv
}

// start runs the TUI on spec. The program is stopped (and waited for) when
// the test ends: the TUI keeps the active page in a package-level variable,
// so two programs must never run at the same time.
func start(t *testing.T, spec string) *teatest.TestModel {
	t.Helper()
	t.Setenv("LAZYAPI_HISTORY_DIR", filepath.Join(filepath.Dir(spec), "history"))
	// Drafts and unsaved edits live under os.TempDir(): keep them in the test dir.
	t.Setenv("TMPDIR", t.TempDir())
	config.DefaultConfig.Active = config.RequestList
	components.Z.NewGlobal()
	tm := teatest.NewTestModel(t, NewTui(spec, ""), teatest.WithInitialTermSize(140, 40))
	t.Cleanup(func() {
		_ = tm.Quit()
		tm.WaitFinished(t, teatest.WithFinalTimeout(20*time.Second))
	})
	return tm
}

// waitFor waits until every text appears on screen (colors stripped).
func waitFor(t *testing.T, tm *teatest.TestModel, texts ...string) {
	t.Helper()
	var seen bytes.Buffer
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		seen.Write(b)
		plain := ansi.Strip(seen.String())
		for _, s := range texts {
			if !strings.Contains(plain, s) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(20*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

// finalScreen stops the program and returns its last frame (colors
// stripped). Use it for text that changes in place (counters), which the
// renderer redraws only partially.
func finalScreen(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
	m := tm.FinalModel(t, teatest.WithFinalTimeout(20*time.Second))
	return ansi.Strip(m.View().Content)
}

func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+k":
		return tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// The list starts on POST /orders (tags sorted: orders, system).

func TestTUI_DestructiveSendAsksFirst(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/health", "/orders")

	tm.Send(press("s"))
	waitFor(t, tm, "Send a request that may change data?", "/orders")
	tm.Send(press("n"))
	waitFor(t, tm, "Request not sent")

	tm.Send(press("s"))
	waitFor(t, tm, "may change data?")
	tm.Send(press("y"))
	waitFor(t, tm, "201 Created")
}

func TestTUI_FilterAndSafeSend(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("/"))
	tm.Type("health")
	tm.Send(press("enter"))
	waitFor(t, tm, "Service health")
	tm.Send(press("s"))
	// GET is safe: no confirmation, the response shows up with its checks.
	waitFor(t, tm, "200 OK", "checks")
}

func TestTUI_PaletteRunsTag(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("ctrl+k"))
	waitFor(t, tm, "type to search")
	tm.Type("tag system")
	tm.Send(press("enter"))
	waitFor(t, tm, "shop.yml · tag system", "finished")
	if out := finalScreen(t, tm); !strings.Contains(out, "✓ 1 passed") || !strings.Contains(out, "GET /health") || strings.Contains(out, "POST /orders") {
		t.Errorf("runner screen:\n%s", out)
	}
}

func TestTUI_RunnerEscGoesBack(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("R"))
	waitFor(t, tm, "finished")
	tm.Send(press("esc"))
	waitFor(t, tm, "Press s to send")
}

func TestTUI_RunWritesNeedsConfirmation(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("ctrl+k"))
	tm.Type("all operations")
	tm.Send(press("enter"))
	waitFor(t, tm, "Run every operation", "2 operations")
	tm.Send(press("y"))
	waitFor(t, tm, "finished")
	if out := finalScreen(t, tm); !strings.Contains(out, "✓ 2 passed") || !strings.Contains(out, "POST /orders") {
		t.Errorf("runner screen:\n%s", out)
	}
}

func TestTUI_HistoryAfterSend(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("down"))
	tm.Send(press("s"))
	waitFor(t, tm, "200 OK")
	tm.Send(press("H"))
	waitFor(t, tm, "History", "1 request", "GET /health")
	tm.Send(press("d"))
	waitFor(t, tm, "No earlier response")
	tm.Send(press("esc"))
	tm.Send(press("enter")) // replay opens the editor
	waitFor(t, tm, "Send", "Docs")
}

func TestTUI_ClearHistory(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("down"))
	tm.Send(press("s"))
	waitFor(t, tm, "200 OK")
	tm.Send(press("H"))
	waitFor(t, tm, "1 request")
	tm.Send(press("X"))
	waitFor(t, tm, "Delete all 1 saved request?")
	tm.Send(press("y"))
	waitFor(t, tm, "History cleared", "Nothing sent yet.")
	if files, _ := filepath.Glob(filepath.Join(history.Dir(), "*.jsonl")); len(files) != 0 {
		t.Errorf("history files left: %v", files)
	}
}

func TestTUI_EditorSendAndEscBack(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/orders")
	tm.Send(press("down"))
	tm.Send(press("enter"))
	waitFor(t, tm, "No response yet")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	waitFor(t, tm, "200 OK")
	tm.Send(press("esc"))
	waitFor(t, tm, "Press s to send")
}

func TestTUI_SwitchNamedEnvironment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	spec := filepath.Join(dir, "shop.yml")
	if err := os.WriteFile(spec, []byte(fmt.Sprintf(tuiSpec, "http://unused.invalid")), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf("default: local\nenvironments:\n  local:\n    server: %s\n  staging:\n    server: http://staging.invalid\n", srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "lazyapi.config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	tm := start(t, spec)

	waitFor(t, tm, "env: local")
	// The default environment's server is used: the request reaches srv.
	tm.Send(press("down"))
	tm.Send(press("s"))
	waitFor(t, tm, "200 OK")

	tm.Send(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	waitFor(t, tm, "Environment: staging", "staging → http://staging.invalid")
	tm.Send(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	waitFor(t, tm, "env: none · ctrl+v") // the toast reuses "Environment: " on screen: only the title is redrawn whole
}

func TestTUI_EscCancelsRequestInFlight(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	dir := t.TempDir()
	spec := filepath.Join(dir, "shop.yml")
	if err := os.WriteFile(spec, []byte(fmt.Sprintf(tuiSpec, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}
	tm := start(t, spec)

	waitFor(t, tm, "/orders")
	tm.Send(press("down"))
	tm.Send(press("s"))
	waitFor(t, tm, "Sending")
	tm.Send(press("esc"))
	waitFor(t, tm, "request cancelled")
}

func TestTUI_FormBodyFields(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got <- r.Header.Get("Content-Type") + " " + r.PostForm.Encode()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	spec := filepath.Join(dir, "auth.yml")
	if err := os.WriteFile(spec, []byte(fmt.Sprintf(`openapi: 3.0.0
info: {title: Auth, version: "1"}
servers: [{url: %s}]
paths:
  /login:
    post:
      operationId: login
      requestBody:
        content:
          application/x-www-form-urlencoded:
            example: {user: ana, password: secret}
      responses: {"200": {description: ok}}
`, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}
	tm := start(t, spec)
	waitFor(t, tm, "/login")
	tm.Send(press("enter")) // open in the editor
	waitFor(t, tm, "No response yet")

	// tab ×4 → request tabs; → ×4 → Body; enter edits it.
	for i := 0; i < 4; i++ {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	for i := 0; i < 4; i++ {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	tm.Send(press("enter"))
	waitFor(t, tm, "Form fields", "password", "secret")

	// Rows: password, user (spec examples are serialized with sorted keys). Edit the first value.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab}) // password value
	tm.Type("2")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	select {
	case body := <-got:
		if body != "application/x-www-form-urlencoded password=secret2&user=ana" {
			t.Errorf("server got %q", body)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the form was not sent")
	}
}

func TestTUI_CompletionAndCodeEditor(t *testing.T) {
	got := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.URL.RequestURI()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")
	if err := os.WriteFile(spec, []byte(fmt.Sprintf(`openapi: 3.0.0
info: {title: API, version: "1"}
servers: [{url: %s}]
paths:
  /items:
    post:
      operationId: createItem
      requestBody:
        content:
          application/json:
            example: {name: a}
      responses: {"200": {description: ok}}
`, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=abc123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lazyapi.config.yml"), []byte("default: dev\nenvironments:\n  dev:\n    env: .env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tm := start(t, spec)
	waitFor(t, tm, "/items")
	tm.Send(press("enter"))
	waitFor(t, tm, "No response yet")

	// URI: "$TO" opens the variable menu; tab inserts {{env.TOKEN}}.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnd})
	tm.Type("?t=$TO")
	waitFor(t, tm, "env.TOKEN", "tab accept")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	select {
	case uri := <-got:
		if uri != "/items?t=abc123" {
			t.Errorf("sent %q", uri)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("request not sent")
	}

	// Body: the JSON editor flags invalid JSON in its status line. The
	// response took the focus: tab ×5 wraps around to the request tabs.
	waitFor(t, tm, "200")
	for i := 0; i < 5; i++ {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	for i := 0; i < 4; i++ {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	tm.Send(press("enter"))
	waitFor(t, tm, "JSON", "✓ valid")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnd, Mod: tea.ModCtrl})
	tm.Type(",")
	waitFor(t, tm, "✗ invalid")
}
