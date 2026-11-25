package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/oauth"
)

// keycloak is a fake realm with the device grant.
func keycloak(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/realms/acme/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := srv.URL + "/realms/acme/protocol/openid-connect"
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": srv.URL + "/realms/acme", "authorization_endpoint": base + "/auth",
			"token_endpoint": base + "/token", "device_authorization_endpoint": base + "/auth/device",
		})
	})
	mux.HandleFunc("/realms/acme/protocol/openid-connect/auth/device", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"device_code":"d","user_code":"WXYZ","verification_uri":"https://sso/device"}`))
	})
	mux.HandleFunc("/realms/acme/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("client_id") != "lazyapi" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"access_token":"kc-%s","refresh_token":"r","expires_in":300}`, r.Form.Get("grant_type")[len(r.Form.Get("grant_type"))-4:])
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestLogin_KeycloakIssuerFromEnvironment(t *testing.T) {
	model.ResetTokenCache()
	defer func(d time.Duration) { oauth.DevicePollInterval = d }(oauth.DevicePollInterval)
	oauth.DevicePollInterval = 10 * time.Millisecond

	kc := keycloak(t)
	var gotAuth string
	api := newHitServer(t, func(r *http.Request) int {
		gotAuth = r.Header.Get("Authorization")
		if gotAuth != "Bearer kc-code" {
			return http.StatusUnauthorized
		}
		return http.StatusOK
	})
	spec := writeSpec(t, `openapi: "3.0.0"
info: {title: t, version: "1"}
security: [{keycloak: [openid]}]
components:
  securitySchemes:
    keycloak: {type: openIdConnect, openIdConnectUrl: "https://placeholder/realms/acme"}
paths:
  /me:
    get:
      operationId: me
      responses:
        "200": {description: OK}
        "401": {description: no}
`)
	dir := filepath.Dir(spec)
	_ = os.MkdirAll(filepath.Join(dir, ".lazyapi"), 0o755)
	cfg := fmt.Sprintf(`default: dev
environments:
  dev:
    server: %s
    auth:
      keycloak: {issuer: "%s/realms/acme", clientId: lazyapi}
`, api.URL, kc.URL)
	if err := os.WriteFile(filepath.Join(dir, ".lazyapi", "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	// Not logged in: the request is not sent and the error says what to do.
	res, err := sendJSON(t, spec, "--op", "me")
	if err == nil || !strings.Contains(res.Reason, "lazyapi login") {
		t.Fatalf("before login: %v %+v", err, res)
	}

	out, err := captureStdout(t, func() error { return Login([]string{spec, "--device", "--no-browser"}) })
	if err != nil || !strings.Contains(out, "logged in to keycloak [dev]") {
		t.Fatalf("login: %v\n%s", err, out)
	}
	// The device grant's token serves the authorizationCode scheme ("kc-code" is the
	// device_code grant's suffix).
	if res, err := sendJSON(t, spec, "--op", "me"); err != nil || gotAuth != "Bearer kc-code" {
		t.Fatalf("after login: %v auth=%q %+v", err, gotAuth, res)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lazyapi", ".local", "tokens.json")); err != nil {
		t.Error("the session is persisted")
	}
	out, _ = captureStdout(t, func() error { return Login([]string{spec, "--status"}) })
	if !strings.Contains(out, "keycloak") || !strings.Contains(out, "refreshable") {
		t.Errorf("status: %s", out)
	}
	out, err = captureStdout(t, func() error { return Logout([]string{spec}) })
	if err != nil || !strings.Contains(out, "removed 1") {
		t.Errorf("logout: %v %s", err, out)
	}

	if err := Login([]string{spec, "nope"}); ExitCode(err) != 2 || !strings.Contains(err.Error(), "keycloak") {
		t.Errorf("unknown scheme: %v", err)
	}
}
