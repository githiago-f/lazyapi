package cli

import (
	"encoding/json"
	"encoding/xml"
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
