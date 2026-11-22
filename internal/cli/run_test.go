package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/githiago-f/lazyapi/internal/runner"
)

// captureStdout runs fn and returns what it printed.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	runErr := fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done, runErr
}

func TestRun_ExitCodes(t *testing.T) {
	spec := writeSpec(t, smokeSpec)

	if code := ExitCode(run([]string{"run"})); code != runner.ExitUsage {
		t.Errorf("missing spec: code %d, want 2", code)
	}
	if code := ExitCode(run([]string{"run", spec, "--bogus"})); code != runner.ExitUsage {
		t.Errorf("unknown flag: code %d, want 2", code)
	}
	if code := ExitCode(run([]string{"run", spec, "--output", "xml"})); code != runner.ExitUsage {
		t.Errorf("bad output: code %d, want 2", code)
	}
	if code := ExitCode(run([]string{"run", filepath.Join(t.TempDir(), "nope.yml")})); code != runner.ExitErrored {
		t.Errorf("missing file: code %d, want 3", code)
	}
	// Connection refused → error (3).
	_, err := captureStdout(t, func() error { return run([]string{"run", spec, "--server", "http://127.0.0.1:1", "--op", "listItems"}) })
	if code := ExitCode(err); code != runner.ExitErrored {
		t.Errorf("network error: code %d, want 3 (%v)", code, err)
	}
	// Undeclared status → failure (1).
	srv := newHitServer(t, func(*http.Request) int { return 500 })
	_, err = captureStdout(t, func() error { return run([]string{"run", spec, "--server", srv.URL, "--op", "listItems"}) })
	if code := ExitCode(err); code != runner.ExitFailed {
		t.Errorf("failed check: code %d, want 1 (%v)", code, err)
	}
}

func TestRun_JSONOutput(t *testing.T) {
	srv := newHitServer(t, func(r *http.Request) int {
		if r.URL.Path == "/items" {
			return 500
		}
		return 200
	})
	spec := writeSpec(t, smokeSpec)

	out, err := captureStdout(t, func() error { return RunTests([]string{spec, "--server", srv.URL, "--output", "json"}) })
	if ExitCode(err) != 1 {
		t.Fatalf("exit = %d (%v)", ExitCode(err), err)
	}
	var rep runner.Report
	if jerr := json.Unmarshal([]byte(out), &rep); jerr != nil {
		t.Fatalf("stdout is not a JSON report: %v\n%s", jerr, out)
	}
	if rep.Summary.Failed != 1 || rep.Summary.Passed != 1 || rep.Summary.Skipped != 2 {
		t.Errorf("summary = %+v", rep.Summary)
	}
	for _, op := range rep.Operations {
		switch op.OperationID {
		case "listItems":
			if op.Outcome != runner.Failed || op.Response == nil || op.Response.Body == "" {
				t.Errorf("failed op should carry its body: %+v", op)
			}
			var status *runner.Check
			for i := range op.Checks {
				if op.Checks[i].Kind == runner.CheckStatus {
					status = &op.Checks[i]
				}
			}
			if status == nil || status.Passed || status.Message == "" {
				t.Errorf("status check = %+v", op.Checks)
			}
			if op.Checks[0].Kind != runner.CheckRequest || !op.Checks[0].Passed {
				t.Errorf("request check should come first and pass: %+v", op.Checks)
			}
		case "getUser":
			if op.Outcome != runner.Passed || op.Response.Body != "" {
				t.Errorf("passed op should omit body by default: %+v", op)
			}
		case "createItem":
			if op.Outcome != runner.Skipped || !strings.Contains(op.Reason, "--methods") {
				t.Errorf("POST should be skipped: %+v", op)
			}
		}
	}
}

func TestRun_JUnitAndTAP(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)

	out, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--output", "junit", "--op", "listItems,createItem"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var suites struct {
		Tests   int `xml:"tests,attr"`
		Skipped int `xml:"skipped,attr"`
	}
	if xerr := xml.Unmarshal([]byte(out), &suites); xerr != nil {
		t.Fatalf("invalid junit: %v\n%s", xerr, out)
	}
	if suites.Tests != 3 || suites.Skipped != 1 {
		t.Errorf("junit counts = %+v\n%s", suites, out)
	}

	out, err = captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--output", "tap", "--op", "listItems,createItem"})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TAP version 14", "ok 1 - GET /items › request matches the OpenAPI spec",
		"ok 2 - GET /items › status 200 is declared", "# SKIP method POST", "1..3"} {
		if !strings.Contains(out, want) {
			t.Errorf("TAP missing %q:\n%s", want, out)
		}
	}
}

func TestRun_OutputFileKeepsPrettyOnStdout(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)
	reportPath := filepath.Join(t.TempDir(), "report.xml")

	out, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--output", "junit", "--output-file", reportPath, "--op", "listItems"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Passed GET /items") || !strings.Contains(out, "Operations: 1 passed") {
		t.Errorf("stdout should have the pretty log:\n%s", out)
	}
	data, _ := os.ReadFile(reportPath)
	if !strings.Contains(string(data), "<testsuites") {
		t.Errorf("report file = %s", data)
	}
}

func TestRun_GrepBailParallelRetries(t *testing.T) {
	var calls atomic.Int32
	srv := newHitServer(t, func(r *http.Request) int {
		if r.URL.Path == "/flaky" && calls.Add(1) < 3 {
			return 503
		}
		return 200
	})
	spec := writeSpec(t, smokeSpec+`  /flaky:
    get:
      operationId: flaky
      responses:
        "200":
          description: OK
`)

	if _, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--grep", "flaky", "--retries", "2"})
	}); err != nil {
		t.Errorf("retries should recover from two 503s: %v", err)
	}
	if hits := srv.Hits(); len(hits) != 3 {
		t.Errorf("hits = %v", hits)
	}

	if _, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--parallel", "4", "--param", "id=1"})
	}); err != nil {
		t.Errorf("parallel run: %v", err)
	}

	bad := newHitServer(t, func(*http.Request) int { return 500 })
	out, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", bad.URL, "--bail", "--param", "id=1", "--output", "json"})
	})
	if ExitCode(err) != 1 {
		t.Fatalf("bail exit = %d", ExitCode(err))
	}
	var rep runner.Report
	if jerr := json.Unmarshal([]byte(out), &rep); jerr != nil {
		t.Fatal(jerr)
	}
	if len(bad.Hits()) != 1 || !strings.Contains(rep.Operations[len(rep.Operations)-1].Reason, "--bail") {
		t.Errorf("bail should stop after the first failure: hits=%v", bad.Hits())
	}
}

func TestSend_OpFlagBodyAndJSON(t *testing.T) {
	var gotBody string
	srv := newHitServer(t, func(r *http.Request) int {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		return 201
	})
	spec := writeSpec(t, smokeSpec)
	bodyFile := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(bodyFile, []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return SendRequest([]string{spec, "--op", "createItem", "--server", srv.URL, "--body", "@" + bodyFile, "--output", "json"})
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotBody != `{"name":"x"}` {
		t.Errorf("body = %q", gotBody)
	}
	var res runner.OpResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("not JSON: %v\n%s", jerr, out)
	}
	if res.Outcome != runner.Passed || res.Response.Status != 201 || res.Response.Body == "" || res.Request.Body != `{"name":"x"}` {
		t.Errorf("result = %+v", res)
	}
}

func TestSend_DryRunAndRedaction(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)
	out, err := captureStdout(t, func() error {
		return SendRequest([]string{spec, "--op", "getItem", "--param", "id=5", "--server", srv.URL,
			"--header", "Authorization=Bearer secret", "--query", "q=1", "--dry-run", "--output", "json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var res runner.OpResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatal(jerr)
	}
	if res.Outcome != runner.Planned || res.Request.URL != srv.URL+"/items/5?q=1" {
		t.Errorf("planned = %+v", res.Request)
	}
	if res.Request.Headers["Authorization"] != "<redacted>" || strings.Contains(out, "secret") {
		t.Errorf("authorization should be redacted: %s", out)
	}
	if len(srv.Hits()) != 0 {
		t.Error("dry run sent a request")
	}
}

func TestListDescribeValidate(t *testing.T) {
	spec := writeSpec(t, contractSpec)
	writeTestScript(t, spec, "me.js", `test("x", () => {`) // does not compile

	out, err := captureStdout(t, func() error { return run([]string{"list", "ops", spec, "--json"}) })
	if err != nil {
		t.Fatal(err)
	}
	var ops []map[string]any
	if jerr := json.Unmarshal([]byte(out), &ops); jerr != nil || len(ops) != 2 || ops[1]["testsFile"] != "tests/me.js" {
		t.Errorf("list = %s (%v)", out, jerr)
	}

	out, err = captureStdout(t, func() error { return run([]string{"describe", spec, "me", "--json"}) })
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if jerr := json.Unmarshal([]byte(out), &d); jerr != nil {
		t.Fatal(jerr)
	}
	if d["method"] != "GET" || d["path"] != "/me" || len(d["responses"].([]any)) != 1 {
		t.Errorf("describe = %s", out)
	}
	if code := ExitCode(run([]string{"describe", spec, "nope"})); code != runner.ExitErrored {
		t.Errorf("unknown operation: code %d", code)
	}

	out, err = captureStdout(t, func() error { return run([]string{"validate", "spec", spec, "--json"}) })
	if ExitCode(err) != 1 {
		t.Fatalf("validate should fail on a broken script: %v", err)
	}
	if !strings.Contains(out, "does not compile") || !strings.Contains(out, "no servers declared") {
		t.Errorf("validate = %s", out)
	}
}

func TestRun_RequestCheckFlagsInvalidParams(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, `openapi: "3.0.0"
info: {title: t, version: "1"}
paths:
  /orders/{id}:
    get:
      operationId: getOrder
      parameters:
        - {in: path, name: id, required: true, schema: {type: integer}}
        - {in: query, name: limit, required: true, schema: {type: integer, maximum: 10}}
      responses:
        "200": {description: OK}
`)
	out, err := captureStdout(t, func() error {
		return SendRequest([]string{spec, "--op", "getOrder", "--server", srv.URL, "--param", "id=abc", "--query", "limit=50", "--output", "json"})
	})
	if ExitCode(err) != 1 {
		t.Fatalf("exit = %d (%v)", ExitCode(err), err)
	}
	var res runner.OpResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatal(jerr)
	}
	c := res.Checks[0]
	if c.Kind != runner.CheckRequest || c.Passed || !strings.Contains(c.Message, `path parameter "id"`) || !strings.Contains(c.Message, `query parameter "limit"`) {
		t.Errorf("request check = %+v", c)
	}
	if len(srv.Hits()) != 1 {
		t.Error("an invalid request is still sent (the server's answer is useful)")
	}
	// --no-contract disables both directions.
	if _, err := captureStdout(t, func() error {
		return SendRequest([]string{spec, "--op", "getOrder", "--server", srv.URL, "--param", "id=abc", "--query", "limit=50", "--no-contract"})
	}); err != nil {
		t.Errorf("--no-contract: %v", err)
	}
}

const authSpec = `openapi: "3.0.0"
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
              schema: {type: object, required: [token], properties: {token: {type: string}}}
  /secret:
    get:
      operationId: getSecret
      x-lazyapi-tests: tests/secret.js
      responses:
        "200": {description: OK}
  /items:
    get:
      operationId: listItems
      responses:
        "200": {description: OK}
    delete:
      operationId: clearItems
      responses:
        "204": {description: Cleared}
`

// tokenServer requires the token issued by /login on /secret.
func tokenServer(t *testing.T) *hitServer {
	return newHitServer(t, func(r *http.Request) int {
		switch r.URL.Path {
		case "/secret":
			if r.Header.Get("Authorization") != "Bearer abc" {
				return http.StatusUnauthorized
			}
		case "/items":
			if r.Method == http.MethodDelete {
				return http.StatusNoContent
			}
		case "/teapot":
			return http.StatusTeapot
		}
		return http.StatusOK
	})
}

func TestRun_BeforeAuthenticatesRequest(t *testing.T) {
	srv := tokenServer(t)
	// hitServer answers {"ok":true}; the login token comes from a fixed body.
	spec := writeSpec(t, authSpec)
	writeTestScript(t, spec, "secret.js", `
export function before(req) {
  lazyapi.send("login");               // a real call, checked like any other
  req.headers.Authorization = "Bearer " + env.get("TOKEN");
}
test("authorized", () => expect(response).toHaveStatus(200));
`)
	envFile := filepath.Join(filepath.Dir(spec), ".env")
	if err := os.WriteFile(envFile, []byte("TOKEN=abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error {
		return SendRequest([]string{spec, "--op", "getSecret", "--server", srv.URL, "--env", envFile})
	}); err != nil {
		t.Fatalf("before() should authenticate the request: %v", err)
	}
	hits := strings.Join(srv.Hits(), ",")
	if hits != "POST /login,GET /secret" {
		t.Errorf("hits = %s", hits)
	}

	// A throwing before() fails the operation and does not send it.
	writeTestScript(t, spec, "secret.js", `export function before() { throw new Error("no credentials"); }`)
	out, err := captureStdout(t, func() error {
		return SendRequest([]string{spec, "--op", "getSecret", "--server", srv.URL, "--output", "json"})
	})
	if ExitCode(err) != 1 || !strings.Contains(out, "before(): Error: no credentials") {
		t.Errorf("throwing before: %v\n%s", err, out)
	}
	if strings.Count(strings.Join(srv.Hits(), ","), "GET /secret") != 1 {
		t.Error("the request must not be sent when before() fails")
	}
}

func TestRun_OrdersReadsBeforeDeletes(t *testing.T) {
	srv := tokenServer(t)
	spec := writeSpec(t, authSpec)
	writeTestScript(t, spec, "secret.js", `test("x", () => {});`)
	if _, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--methods", "GET,DELETE", "--no-contract"})
	}); ExitCode(err) > 1 {
		t.Fatal(err)
	}
	hits := srv.Hits()
	if hits[len(hits)-1] != "DELETE /items" {
		t.Errorf("DELETE should run last: %v", hits)
	}
}

func TestRun_Flows(t *testing.T) {
	srv := tokenServer(t)
	spec := writeSpec(t, authSpec)
	dir := filepath.Join(filepath.Dir(spec), "flows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, src string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := write("a_good.js", `
const items = lazyapi.send("listItems");
test("items listed", () => expect(items).toHaveStatus(200));
lazyapi.send("clearItems");
`)
	write("b_bad.js", `
// /login declares a token schema: {"ok":true} breaks the contract.
lazyapi.send("login");
`)

	out, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--flow", good, "--output", "json"})
	})
	if err != nil {
		t.Fatalf("good flow: %v\n%s", err, out)
	}
	var rep runner.Report
	if jerr := json.Unmarshal([]byte(out), &rep); jerr != nil {
		t.Fatal(jerr)
	}
	if len(rep.Operations) != 1 || rep.Operations[0].Kind != runner.KindFlow || len(rep.Operations[0].Steps) != 2 {
		t.Fatalf("--flow runs only that flow: %+v", rep.Operations)
	}
	if rep.Operations[0].Path != "flows/a_good.js" || rep.Operations[0].Steps[1].Status != 204 {
		t.Errorf("flow result = %+v", rep.Operations[0])
	}

	out, err = captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--flows", "--op", "listItems", "--output", "json"})
	})
	if ExitCode(err) != 1 {
		t.Fatalf("bad flow should fail: %v", err)
	}
	if jerr := json.Unmarshal([]byte(out), &rep); jerr != nil {
		t.Fatal(jerr)
	}
	if len(rep.Operations) != 3 || rep.Operations[2].Outcome != runner.Failed ||
		!strings.Contains(out, "#1 POST /login · response matches the OpenAPI schema") {
		t.Errorf("--flows = operations + every flow; contract checked per step:\n%s", out)
	}

	// No assertions and no requests is a failure, not a silent pass.
	empty := write("c_empty.js", `const x = 1;`)
	if _, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--server", srv.URL, "--flow", empty})
	}); ExitCode(err) != 1 {
		t.Errorf("empty flow should fail: %v", err)
	}
}

func TestRun_NamedEnvironment(t *testing.T) {
	var gotAuth, gotTenant string
	srv := newHitServer(t, func(r *http.Request) int {
		gotAuth, gotTenant = r.Header.Get("Authorization"), r.URL.Query().Get("tenant")
		if gotAuth != "Bearer from-env-file" {
			return http.StatusUnauthorized
		}
		return http.StatusOK
	})
	spec := writeSpec(t, `openapi: "3.0.0"
info: {title: t, version: "1"}
servers: [{url: "http://127.0.0.1:1"}]
security: [{bearerAuth: []}]
components:
  securitySchemes:
    bearerAuth: {type: http, scheme: bearer}
paths:
  /me:
    get:
      operationId: me
      parameters:
        - {in: query, name: tenant, required: true, schema: {type: string}}
      responses:
        "200": {description: OK}
`)
	dir := filepath.Dir(spec)
	if err := os.WriteFile(filepath.Join(dir, ".env.staging"), []byte("API_TOKEN=from-env-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf(`default: broken
environments:
  broken:
    server: http://127.0.0.1:1
  staging:
    server: %s
    env: .env.staging
    vars: {tenant: acme}
    auth:
      bearerAuth: {token: "{{env.API_TOKEN}}"}
`, srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "lazyapi.config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return SendRequest([]string{spec, "--op", "me", "--env-name", "staging", "--query", "tenant={{var.tenant}}", "--output", "json"})
	})
	if err != nil {
		t.Fatalf("staging: %v\n%s", err, out)
	}
	if gotAuth != "Bearer from-env-file" || gotTenant != "acme" {
		t.Errorf("auth=%q tenant=%q", gotAuth, gotTenant)
	}

	// Without --env-name the default (broken) environment is used...
	out, err = captureStdout(t, func() error { return RunTests([]string{spec, "--output", "json"}) })
	if ExitCode(err) != 3 || !strings.Contains(out, `"environment": "broken"`) {
		t.Errorf("default env: %v\n%s", err, out)
	}
	// ...and an unknown name is a usage error listing the options.
	if err := RunTests([]string{spec, "--env-name", "prod"}); ExitCode(err) != 2 || !strings.Contains(err.Error(), "broken, staging") {
		t.Errorf("unknown env: %v", err)
	}
}

func TestSend_JQ(t *testing.T) {
	srv := newHitServer(t, ok200)
	spec := writeSpec(t, smokeSpec)

	out, err := captureStdout(t, func() error {
		return SendRequest([]string{spec, "--op", "listItems", "--server", srv.URL, "--jq", ".ok", "--output", "json"})
	})
	if err != nil {
		t.Fatalf("send: %v\n%s", err, out)
	}
	var res runner.OpResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("not JSON: %v\n%s", jerr, out)
	}
	if res.Response.Body != "true" {
		t.Errorf("jq body = %q", res.Response.Body)
	}

	err = SendRequest([]string{spec, "--op", "listItems", "--server", srv.URL, "--jq", ".["})
	if ExitCode(err) != 2 {
		t.Errorf("invalid --jq should be a usage error, got %v", err)
	}
}
