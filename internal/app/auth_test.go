package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/githiago-f/lazyapi/internal/components"
	"github.com/githiago-f/lazyapi/internal/model"
)

const oidcSpec = `openapi: 3.0.0
info: {title: Shop, version: "1"}
servers:
  - url: %s
security: [{keycloak: [openid]}]
components:
  securitySchemes:
    keycloak: {type: openIdConnect, openIdConnectUrl: "%s/realms/acme"}
paths:
  /me:
    get:
      operationId: me
      summary: Who am I
      responses: {"200": {description: ok}, "401": {description: no}}
`

// A send without a session logs in (authorization code + PKCE through the
// "browser"), stores the token and resends the request with it.
func TestTUI_SendLogsInWithOpenIDConnect(t *testing.T) {
	model.ResetTokenCache()
	mux := http.NewServeMux()
	var kc *httptest.Server
	mux.HandleFunc("/realms/acme/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 kc.URL + "/realms/acme",
			"authorization_endpoint": kc.URL + "/realms/acme/auth",
			"token_endpoint":         kc.URL + "/realms/acme/token",
		})
	})
	mux.HandleFunc("/realms/acme/auth", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code=c1&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("/realms/acme/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "c1" || r.Form.Get("code_verifier") == "" || r.Form.Get("client_id") != "shop-cli" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"tui-token","refresh_token":"r","expires_in":300}`))
	})
	kc = httptest.NewServer(mux)
	t.Cleanup(kc.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tui-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"name":"ana"}`))
	}))
	t.Cleanup(api.Close)

	prev := openBrowser
	openBrowser = func(link string) error {
		go func() {
			if resp, err := http.Get(link); err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
	t.Cleanup(func() { openBrowser = prev })

	dir := t.TempDir()
	spec := filepath.Join(dir, "shop.yml")
	if err := os.WriteFile(spec, []byte(fmt.Sprintf(oidcSpec, api.URL, kc.URL)), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := "default: dev\nenvironments:\n  dev:\n    auth:\n      keycloak: {clientId: shop-cli, redirectUri: \"http://127.0.0.1:0/callback\"}\n"
	_ = os.MkdirAll(filepath.Join(dir, ".lazyapi"), 0o755)
	if err := os.WriteFile(filepath.Join(dir, ".lazyapi", "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	tm := start(t, spec)
	waitFor(t, tm, "/me", "env: dev")
	tm.Send(press("s"))
	waitFor(t, tm, "Logged in to keycloak", "200 OK")
	if _, err := os.Stat(filepath.Join(dir, ".lazyapi", ".local", "tokens.json")); err != nil {
		t.Error("the session is stored next to the spec")
	}
}

// Selects open with a click, pick with a click and take type-ahead.
func TestTUI_MouseAndKeyboardSelect(t *testing.T) {
	tm, _ := newTUI(t)
	waitFor(t, tm, "/health", "/orders")
	tm.Send(press("enter"))
	waitFor(t, tm, "POST ▾", "Send")

	clickZone := func(id string) {
		t.Helper()
		z := components.Z.Get(id)
		if z.IsZero() {
			t.Fatalf("zone %s was not drawn", id)
		}
		tm.Send(tea.MouseClickMsg{X: z.StartX + 1, Y: (z.StartY + z.EndY) / 2, Button: tea.MouseLeft})
	}
	clickZone("select:editor-method")
	waitFor(t, tm, "▸ POST", "OPTIONS")
	clickZone("select:editor-method:4") // DELETE
	waitFor(t, tm, "DELETE ▾")

	// Keyboard: enter opens, "p" jumps to the next method starting with p.
	tm.Send(press("enter"))
	waitFor(t, tm, "▸ DELETE")
	tm.Send(press("p"))
	tm.Send(press("enter"))
	if screen := finalScreen(t, tm); !strings.Contains(screen, "POST ▾") {
		t.Errorf("type-ahead:\n%s", screen)
	}
}
