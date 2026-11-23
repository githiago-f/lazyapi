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

	"github.com/githiago-f/lazyapi/internal/mock"
	"github.com/githiago-f/lazyapi/internal/runner"
	"github.com/githiago-f/lazyapi/internal/store"
)

const httpSpec = `openapi: "3.0.0"
info: {title: t, version: "1"}
paths:
  /ok:
    get:
      operationId: ok
      responses:
        "200": {description: OK}
  /moved:
    get:
      operationId: moved
      responses:
        "200": {description: OK}
        "302": {description: Moved}
  /big:
    get:
      operationId: big
      responses:
        "200": {description: OK}
`

func tlsServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/moved":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/big":
			_, _ = w.Write([]byte(strings.Repeat("x", 4096)))
		default:
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
			_, _ = w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sendJSON(t *testing.T, args ...string) (runner.OpResult, error) {
	t.Helper()
	out, err := captureStdout(t, func() error { return SendRequest(append(args, "--output", "json")) })
	var res runner.OpResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil && err == nil {
		t.Fatalf("not JSON: %v\n%s", jerr, out)
	}
	return res, err
}

func TestHTTP_TLSRedirectsAndBodyLimit(t *testing.T) {
	srv := tlsServer(t)
	spec := writeSpec(t, httpSpec)

	if res, err := sendJSON(t, spec, "--op", "ok", "--server", srv.URL); ExitCode(err) != 3 || !strings.Contains(res.Reason, "certificate") {
		t.Errorf("self-signed TLS must fail without --insecure: %v %+v", err, res)
	}

	res, err := sendJSON(t, spec, "--op", "ok", "--server", srv.URL, "--insecure")
	if err != nil {
		t.Fatalf("--insecure: %v", err)
	}
	if res.Response.Timing == nil || res.Response.Timing.TotalMs <= 0 || res.Response.Timing.TLSMs <= 0 {
		t.Errorf("timing = %+v", res.Response.Timing)
	}
	if len(res.Response.Cookies) != 1 || res.Response.Cookies[0].Name != "session" || !res.Response.Cookies[0].HTTPOnly || res.Response.Cookies[0].SameSite != "Lax" {
		t.Errorf("cookies = %+v", res.Response.Cookies)
	}

	if res, _ := sendJSON(t, spec, "--op", "moved", "--server", srv.URL, "--insecure"); res.Response.Status != 200 {
		t.Errorf("redirects are followed by default: %d", res.Response.Status)
	}
	if res, _ := sendJSON(t, spec, "--op", "moved", "--server", srv.URL, "--insecure", "--no-redirects"); res.Response.Status != 302 {
		t.Errorf("--no-redirects: %d", res.Response.Status)
	}

	if res, err := sendJSON(t, spec, "--op", "big", "--server", srv.URL, "--insecure", "--max-body", "1KB"); ExitCode(err) != 3 || !strings.Contains(res.Reason, "larger than 1024 bytes") {
		t.Errorf("--max-body: %v %+v", err, res)
	}
	if err := SendRequest([]string{spec, "--op", "ok", "--max-body", "lots"}); ExitCode(err) != 2 {
		t.Errorf("invalid --max-body should be a usage error: %v", err)
	}
	if err := SendRequest([]string{spec, "--op", "ok", "--client-cert", "c.pem"}); ExitCode(err) != 2 {
		t.Errorf("cert without key should be a usage error: %v", err)
	}
}

func TestHTTP_FromEnvironmentConfig(t *testing.T) {
	srv := tlsServer(t)
	spec := writeSpec(t, httpSpec)
	cfg := fmt.Sprintf("default: dev\nenvironments:\n  dev:\n    server: %s\n    http:\n      insecure: true\n      noRedirects: true\n", srv.URL)
	_ = os.MkdirAll(filepath.Join(filepath.Dir(spec), ".lazyapi"), 0o755)
	if err := os.WriteFile(filepath.Join(filepath.Dir(spec), ".lazyapi", "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := sendJSON(t, spec, "--op", "moved")
	if err != nil || res.Response.Status != 302 {
		t.Errorf("http settings from the environment: %v %+v", err, res.Response)
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"": 0, "512": 512, "10KB": 10 << 10, "50mb": 50 << 20, "1 GB": 1 << 30} {
		if got, err := parseSize(in); err != nil || got != want {
			t.Errorf("parseSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := parseSize("-1MB"); err == nil {
		t.Error("negative size")
	}
}

const formSpec = `openapi: "3.0.0"
info: {title: t, version: "1"}
paths:
  /login:
    post:
      operationId: login
      requestBody:
        required: true
        content:
          application/x-www-form-urlencoded:
            schema:
              type: object
              required: [username, password]
              properties: {username: {type: string}, password: {type: string}}
            example: {username: ana, password: secret}
      responses:
        "200": {description: OK}
        "422": {description: Invalid}
  /avatar:
    post:
      operationId: upload
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              required: [file]
              properties:
                file: {type: string, format: binary}
                caption: {type: string}
      responses:
        "201": {description: Created}
        "422": {description: Invalid}
`

func TestForms_AgainstMock(t *testing.T) {
	spec := writeSpec(t, formSpec)
	parsed, err := store.ParseSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mock.New(parsed, spec))
	defer srv.Close()

	// The spec example (a JSON object) is sent as form fields.
	if res, err := sendJSON(t, spec, "--op", "login", "--server", srv.URL); err != nil || res.Response.Status != 200 {
		t.Errorf("example form body: %v %+v", err, res.Response)
	}
	// --form replaces it; a missing required field is reported by the request check and the mock (422).
	res, err := sendJSON(t, spec, "--op", "login", "--server", srv.URL, "--form", "username=ana")
	if ExitCode(err) != 1 || res.Response.Status != 422 || !strings.Contains(res.Checks[0].Message, "password") {
		t.Errorf("missing field: %v %+v %+v", err, res.Response, res.Checks)
	}

	file := filepath.Join(t.TempDir(), "me.png")
	if err := os.WriteFile(file, []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	if res, err := sendJSON(t, spec, "--op", "upload", "--server", srv.URL, "--file", "file="+file, "--form", "caption=hi"); err != nil || res.Response.Status != 201 {
		t.Errorf("multipart upload: %v %+v %+v", err, res.Response, res.Checks)
	}
	if err := SendRequest([]string{spec, "--op", "upload", "--file", "file=/nope.png"}); ExitCode(err) != 2 {
		t.Errorf("missing file should be a usage error: %v", err)
	}
	if err := SendRequest([]string{spec, "--op", "login", "--form", "a=b", "--body", "{}"}); ExitCode(err) != 2 {
		t.Errorf("--body with --form should be a usage error: %v", err)
	}
}
