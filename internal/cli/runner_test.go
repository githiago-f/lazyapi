package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// hitServer records every "METHOD /path" it receives and answers with
// the status returned by statusFor.
type hitServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

func newHitServer(t *testing.T, statusFor func(r *http.Request) int) *hitServer {
	t.Helper()
	hs := &hitServer{}
	hs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hs.mu.Lock()
		hs.hits = append(hs.hits, r.Method+" "+r.URL.Path)
		hs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusFor(r))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(hs.Close)
	return hs
}

func (hs *hitServer) Hits() []string {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return append([]string(nil), hs.hits...)
}

const smokeSpec = `openapi: "3.0.0"
info:
  title: Test
  version: 1.0.0
paths:
  /items:
    get:
      operationId: listItems
      tags: [items]
      responses:
        "200":
          description: OK
    post:
      operationId: createItem
      tags: [items]
      responses:
        "201":
          description: Created
  /items/{id}:
    get:
      operationId: getItem
      tags: [detail]
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK
        "404":
          description: Not found
  /users/{userId}:
    get:
      operationId: getUser
      parameters:
        - in: path
          name: userId
          required: true
          example: u-7
          schema:
            type: string
      responses:
        "200":
          description: OK
`

func writeSpec(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func ok200(*http.Request) int { return http.StatusOK }

func TestSmoke_DefaultsToSafeMethodsAndSpecExamples(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)

	if err := SmokeTests([]string{spec, "--server", srv.URL}); err != nil {
		t.Fatalf("SmokeTests: %v", err)
	}

	got := strings.Join(srv.Hits(), ",")
	// POST is skipped by default; /items/{id} has no example so it is skipped;
	// /users/{userId} uses the declared example.
	if got != "GET /items,GET /users/u-7" {
		t.Errorf("hits = %q", got)
	}
}

func TestSmoke_MethodsAllAndParamOverride(t *testing.T) {
	srv := newHitServer(t, func(r *http.Request) int {
		if r.Method == http.MethodPost {
			return http.StatusCreated
		}
		return http.StatusOK
	})
	spec := writeSpec(t, smokeSpec)

	if err := SmokeTests([]string{spec, "--server", srv.URL, "--methods", "all", "--param", "id=42"}); err != nil {
		t.Fatalf("SmokeTests: %v", err)
	}
	hits := srv.Hits()
	for _, want := range []string{"POST /items", "GET /items/42"} {
		if !strings.Contains(strings.Join(hits, ","), want) {
			t.Errorf("missing hit %q in %v", want, hits)
		}
	}
}

func TestSmoke_FiltersByOpAndTag(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)

	if err := SmokeTests([]string{spec, "--server", srv.URL, "--op", "getUser"}); err != nil {
		t.Fatal(err)
	}
	if err := SmokeTests([]string{spec, "--server", srv.URL, "--tag", "detail", "--param", "id=1"}); err != nil {
		t.Fatal(err)
	}
	if err := SmokeTests([]string{spec, "--server", srv.URL, "--op", "GET /items"}); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(srv.Hits(), ",")
	if got != "GET /users/u-7,GET /items/1,GET /items" {
		t.Errorf("hits = %q", got)
	}
}

func TestSmoke_DryRunSendsNothing(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)

	if err := SmokeTests([]string{spec, "--server", srv.URL, "--dry-run", "--methods", "all"}); err != nil {
		t.Fatal(err)
	}
	if hits := srv.Hits(); len(hits) != 0 {
		t.Errorf("dry run sent requests: %v", hits)
	}
}

func TestSmoke_DeclaredStatusIsAccepted(t *testing.T) {
	srv := newHitServer(t, func(*http.Request) int { return http.StatusNotFound })
	spec := writeSpec(t, smokeSpec)

	// 404 is declared for getItem → ok.
	if err := SmokeTests([]string{spec, "--server", srv.URL, "--op", "getItem", "--param", "id=x"}); err != nil {
		t.Errorf("declared 404 should pass: %v", err)
	}
	// 404 is not declared for listItems → error.
	if err := SmokeTests([]string{spec, "--server", srv.URL, "--op", "listItems"}); err == nil {
		t.Error("undeclared 404 should fail")
	}
}

func TestSmoke_RunsScriptFromSpecExtension(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, strings.Replace(smokeSpec,
		"      operationId: listItems\n",
		"      operationId: listItems\n      x-lazyapi-tests: tests/list.js\n", 1))
	testsDir := filepath.Join(filepath.Dir(spec), "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "list.js"), []byte(`test("is 201", () => expect(response.status).toBe(201));`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := SmokeTests([]string{spec, "--server", srv.URL, "--op", "listItems"})
	if err == nil || !strings.Contains(err.Error(), "1 failed") {
		t.Errorf("expected failing script from x-lazyapi-tests, got %v", err)
	}
}

func TestSmoke_UnknownFlag(t *testing.T) {
	spec := writeSpec(t, smokeSpec)
	if err := SmokeTests([]string{spec, "--nope"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestSend_RequiresPathParamValue(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)

	err := SendRequest([]string{spec, "/items/{id}", "GET", "--server", srv.URL})
	if err == nil || !strings.Contains(err.Error(), "--param id=") {
		t.Fatalf("expected missing path param error, got %v", err)
	}

	// Flags before positionals and repeatable overrides.
	if err := SendRequest([]string{"--server", srv.URL, "--param", "id=9", spec, "/items/{id}", "get"}); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if hits := srv.Hits(); len(hits) != 1 || hits[0] != "GET /items/9" {
		t.Errorf("hits = %v", hits)
	}
}

func TestSend_ServerIndexOutOfRange(t *testing.T) {
	spec := writeSpec(t, smokeSpec)
	if err := SendRequest([]string{spec, "/items", "GET", "--server", "3"}); err == nil {
		t.Error("expected out of range error")
	}
}

func TestParseInterleaved(t *testing.T) {
	fs := newFlagSet("t")
	server := fs.String("server", "", "")
	dry := fs.Bool("dry-run", false, "")

	pos, err := parseInterleaved(fs, []string{"a", "--server", "x", "b", "--dry-run", "c", "--", "--literal"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pos, " ") != "a b c --literal" || *server != "x" || !*dry {
		t.Errorf("pos=%v server=%q dry=%v", pos, *server, *dry)
	}
}

func TestKVFlag(t *testing.T) {
	kv := kvFlag{}
	if err := kv.Set("a=b=c"); err != nil || kv["a"] != "b=c" {
		t.Errorf("Set a=b=c → %v, %v", kv, err)
	}
	if err := kv.Set("novalue"); err == nil {
		t.Error("expected error without '='")
	}
}

const contractSpec = `openapi: "3.0.0"
info: {title: t, version: "1"}
paths:
  /login:
    post:
      operationId: login
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                required: [token]
                properties:
                  token: {type: string}
  /me:
    get:
      operationId: me
      x-lazyapi-tests: tests/me.js
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                required: [id]
                properties:
                  id: {type: integer}
`

// authServer issues a token on POST /login and requires it on GET /me.
func authServer(t *testing.T, meBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"token":"abc"}`))
		case "/me":
			if r.Header.Get("Authorization") != "Bearer abc" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(meBody))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeTestScript(t *testing.T, spec, name, src string) {
	t.Helper()
	dir := filepath.Join(filepath.Dir(spec), "tests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

const meScript = `
import { bearer } from "./helpers.js";
const login = lazyapi.send("login");
const me = lazyapi.send("GET /me", { headers: bearer(login.json().token) });
test("login matches contract", () => expect(login).toMatchSchema());
test("me with token", () => {
  expect(me).toHaveStatus(200);
  expect(me.json().id).toBe(1);
});
vars.set("userId", me.json().id);
`

func TestSend_ScriptChainsRequestsWithImports(t *testing.T) {
	srv := authServer(t, `{"id":1}`)
	spec := writeSpec(t, contractSpec)
	writeTestScript(t, spec, "me.js", meScript)
	writeTestScript(t, spec, "helpers.js", `export const bearer = (t) => ({ Authorization: "Bearer " + t });`)

	// The main request itself is unauthenticated (401, undeclared → warning only);
	// the script logs in and calls /me again with the token.
	if err := SendRequest([]string{spec, "/me", "GET", "--server", srv.URL}); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
}

func TestSend_ContractFailure(t *testing.T) {
	srv := authServer(t, `{"id":"not-a-number"}`)
	spec := writeSpec(t, strings.Replace(contractSpec, "      x-lazyapi-tests: tests/me.js\n", "", 1))

	err := SendRequest([]string{spec, "/me", "GET", "--server", srv.URL, "--header", "Authorization=Bearer abc"})
	if err == nil || !strings.Contains(err.Error(), "1 failed") {
		t.Fatalf("expected contract failure, got %v", err)
	}
	if err := SendRequest([]string{spec, "/me", "GET", "--server", srv.URL, "--header", "Authorization=Bearer abc", "--no-contract"}); err != nil {
		t.Fatalf("--no-contract should skip validation: %v", err)
	}
}

func TestSend_LuaScriptIsRejected(t *testing.T) {
	srv := authServer(t, `{"id":1}`)
	spec := writeSpec(t, strings.Replace(contractSpec, "tests/me.js", "tests/me.lua", 1))
	err := SendRequest([]string{spec, "/me", "GET", "--server", srv.URL})
	if err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("expected Lua rejection, got %v", err)
	}
}

func TestSmoke_ScriptErrorFails(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)
	scriptPath := filepath.Join(filepath.Dir(spec), "broken.js")
	if err := os.WriteFile(scriptPath, []byte(`this is not js`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := SmokeTests([]string{spec, "--server", srv.URL, "--op", "listItems", "--script", scriptPath})
	if err == nil || !strings.Contains(err.Error(), "1 errors") {
		t.Fatalf("expected script error to fail smoke, got %v", err)
	}
}
