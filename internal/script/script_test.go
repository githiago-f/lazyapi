package script

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githiago-f/lazyapi/internal/model"
)

func okResponse(body string) *Response {
	return &Response{
		Status:     200,
		StatusText: "200 OK",
		Header:     http.Header{"Content-Type": {"application/json"}, "X-Trace": {"abc"}},
		Body:       body,
		Duration:   15 * time.Millisecond,
	}
}

func baseData() ExecData {
	return ExecData{
		Request: model.Request{
			Method:    model.POST,
			URI:       "/users/{id}",
			ServerURL: "https://api.example.com",
			Params:    map[string]string{"id": "7"},
			Headers:   map[string]string{"Content-Type": "application/json"},
			Body:      model.Body{Raw: `{"name":"ana"}`},
		},
		Response: okResponse(`{"id":7,"name":"ana","tags":["a","b"],"nested":{"ok":true}}`),
		Env:      map[string]string{"TOKEN": "t0k"},
	}
}

func mustPass(t *testing.T, r Result) {
	t.Helper()
	if r.ScriptError != "" {
		t.Fatalf("script error: %s", r.ScriptError)
	}
	for _, tr := range r.Results {
		if !tr.Passed {
			t.Errorf("FAIL %s: %s", tr.Name, tr.Message)
		}
	}
	if len(r.Results) == 0 {
		t.Fatal("no tests recorded")
	}
}

func TestRun_Globals(t *testing.T) {
	src := `
test("request", () => {
  expect(request.method).toBe("POST");
  expect(request.url).toBe("https://api.example.com/users/7");
  expect(request.path).toBe("/users/{id}");
  expect(request.headers["content-type"]).toBe("application/json");
  expect(request.params.id).toBe("7");
  expect(request.json().name).toBe("ana");
});
test("response", () => {
  expect(response.status).toBe(200);
  expect(response.statusText).toBe("200 OK");
  expect(response.header("x-trace")).toBe("abc");
  expect(response.header("missing")).toBeNull();
  expect(response.json().name).toBe("ana");
  expect(response.time).toBe(15);
});
test("env", () => {
  expect(env.get("TOKEN")).toBe("t0k");
  expect(env.has("NOPE")).toBeFalsy();
  expect(env.get("NOPE")).toBeUndefined();
});
`
	mustPass(t, Run(src, baseData(), Options{}))
}

func TestRun_Matchers(t *testing.T) {
	src := `
const body = response.json();
test("all matchers", () => {
  expect(body).toEqual({ id: 7, name: "ana", tags: ["a", "b"], nested: { ok: true } });
  expect(body.tags).toContain("a");
  expect(body.tags).toHaveLength(2);
  expect(body.name).toMatch(/^an/);
  expect(body).toHaveProperty("nested.ok", true);
  expect(body).toHaveProperty(["tags", "1"]);
  expect(body.id).toBeGreaterThan(6);
  expect(body.id).toBeLessThanOrEqual(7);
  expect(body.missing).toBeUndefined();
  expect(body.id).not.toBe(8);
  expect(response).toHaveStatus(200);
  expect(response).toHaveHeader("content-type", /json/);
  expect(response).not.toHaveHeader("x-missing");
  expect(body).toMatchSchema({ type: "object", required: ["id"], properties: { id: { type: "integer" } } });
});
`
	mustPass(t, Run(src, baseData(), Options{}))
}

func TestRun_FailureMessagesAndOrder(t *testing.T) {
	src := `
test("b", () => expect(response.status).toBe(201));
test("a", () => {});
test("b", () => expect({ id: "x" }).toMatchSchema({ type: "object", properties: { id: { type: "integer" } } }));
test("throws", () => { throw "plain"; });
`
	r := Run(src, baseData(), Options{})
	if r.ScriptError != "" {
		t.Fatal(r.ScriptError)
	}
	names := []string{}
	for _, tr := range r.Results {
		names = append(names, tr.Name)
	}
	if strings.Join(names, ",") != "b,a,b,throws" {
		t.Errorf("order = %v", names)
	}
	if r.Results[0].Message != "expected 200 to be 201" {
		t.Errorf("message = %q", r.Results[0].Message)
	}
	if !strings.Contains(r.Results[2].Message, "schema validation failed") {
		t.Errorf("schema message = %q", r.Results[2].Message)
	}
	if r.Results[3].Message != "plain" {
		t.Errorf("throw message = %q", r.Results[3].Message)
	}
	if r.PassCount != 1 || r.FailCount != 3 || !r.Failed() {
		t.Errorf("counts %d/%d", r.PassCount, r.FailCount)
	}
}

func TestRun_ScriptErrorHasLocation(t *testing.T) {
	r := Run("const x = 1;\nnotDefined();\n", baseData(), Options{Filename: "list.test.js"})
	if !strings.Contains(r.ScriptError, "notDefined") || !strings.Contains(r.ScriptError, "list.test.js:2") {
		t.Errorf("ScriptError = %q, want name and list.test.js:2", r.ScriptError)
	}

	r = Run("test(\"x\", () => {", baseData(), Options{Filename: "broken.js"})
	if !strings.Contains(r.ScriptError, "broken.js:1") {
		t.Errorf("syntax error = %q", r.ScriptError)
	}
}

func TestRun_TypeScriptAndModernSyntax(t *testing.T) {
	src := `
interface User { id: number; nested?: { ok?: boolean } }
const u = response.json<User>();
class Box { #v = 1; get v() { return this.#v; } }
test("ts", () => {
  expect(u?.nested?.ok ?? false).toBe(true);
  expect(new Box().v).toBe(1);
  const { id, ...rest } = u as any;
  expect(Object.keys(rest)).toContain("name");
});
`
	mustPass(t, Run(src, baseData(), Options{Filename: "user.test.ts"}))
}

func TestRun_AsyncTest(t *testing.T) {
	src := `
test("async ok", async () => { await Promise.resolve(1); expect(1).toBe(1); });
test("async fail", async () => { await null; throw new Error("boom"); });
`
	r := Run(src, baseData(), Options{})
	if len(r.Results) != 2 || !r.Results[0].Passed || r.Results[1].Passed || r.Results[1].Message != "boom" {
		t.Errorf("results = %+v (err %s)", r.Results, r.ScriptError)
	}
}

func TestRun_ConsoleAndVars(t *testing.T) {
	data := baseData()
	data.Vars = map[string]string{}
	src := `
console.log("id is", response.json().id, { a: 1 });
console.warn("careful");
vars.set("userId", response.json().id);
env.set("SESSION", "s1");
test("noop", () => {});
`
	r := Run(src, data, Options{})
	mustPass(t, r)
	if len(r.Logs) != 2 || r.Logs[0].Message != `id is 7 {"a":1}` || r.Logs[1].Level != "warn" {
		t.Errorf("logs = %+v", r.Logs)
	}
	if data.Vars["userId"] != "7" || data.Env["SESSION"] != "s1" {
		t.Errorf("vars=%v env=%v", data.Vars, data.Env)
	}
}

func TestRun_Send(t *testing.T) {
	data := baseData()
	var gotTarget string
	var gotOpts SendOptions
	data.Send = func(target string, opts SendOptions) (*Response, error) {
		gotTarget, gotOpts = target, opts
		if target == "missing" {
			return nil, errors.New("operation not found")
		}
		resp := okResponse(`{"token":"abc"}`)
		resp.Ref = &model.OpenAPIRef{Path: "/login", Method: "POST"}
		return resp, nil
	}
	data.Contract = func(ref model.OpenAPIRef, resp *Response) error {
		if ref.Path != "/login" {
			return errors.New("wrong ref " + ref.Path)
		}
		return nil
	}
	src := `
const login = lazyapi.send("POST /login", { body: { user: "ana" }, headers: { "X-A": "1" } });
test("chained", () => {
  expect(login.json().token).toBe("abc");
  expect(login).toMatchSchema();
});
test("send error", () => lazyapi.send("missing"));
`
	r := Run(src, data, Options{})
	if gotTarget != "missing" && gotTarget != "POST /login" {
		t.Errorf("target = %q", gotTarget)
	}
	if r.Results[0].Name != "chained" || !r.Results[0].Passed {
		t.Errorf("chained = %+v", r.Results[0])
	}
	if r.Results[1].Passed || !strings.Contains(r.Results[1].Message, "operation not found") {
		t.Errorf("send error = %+v", r.Results[1])
	}
	_ = gotOpts
}

func TestRun_SendOptionsDecoded(t *testing.T) {
	data := baseData()
	var got SendOptions
	data.Send = func(_ string, opts SendOptions) (*Response, error) {
		got = opts
		return okResponse("{}"), nil
	}
	Run(`lazyapi.send("getUser", { params: { id: "1" }, query: { q: "x" }, body: { a: 1 } })`, data, Options{})
	if got.Params["id"] != "1" || got.Query["q"] != "x" {
		t.Errorf("opts = %+v", got)
	}
	if m, ok := got.Body.(map[string]any); !ok || m["a"] != float64(1) {
		t.Errorf("body = %#v", got.Body)
	}
}

func TestRun_NoResponse(t *testing.T) {
	data := baseData()
	data.Response = nil
	r := Run(`test("is null", () => expect(response).toBeNull());
test("needs response", () => expect(response.status).toBe(200));`, data, Options{})
	if !r.Results[0].Passed || r.Results[1].Passed || !strings.Contains(r.Results[1].Message, "status") {
		t.Errorf("results = %+v", r.Results)
	}
}

func TestRun_Timeout(t *testing.T) {
	start := time.Now()
	r := Run(`while (true) {}`, baseData(), Options{Timeout: 50 * time.Millisecond})
	if !strings.Contains(r.ScriptError, "timed out") {
		t.Errorf("ScriptError = %q", r.ScriptError)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("timeout not enforced")
	}
}

func TestRun_SandboxHasNoNodeOrFS(t *testing.T) {
	mustPass(t, Run(`test("no process", () => expect(typeof process).toBe("undefined"));`, baseData(), Options{}))

	r := Run(`const fs = require("fs");`, baseData(), Options{})
	if !strings.Contains(r.ScriptError, `bare import "fs"`) {
		t.Errorf("require(fs) should be rejected at bundle time, got %q", r.ScriptError)
	}
}

func TestRun_LocalImports(t *testing.T) {
	root := t.TempDir()
	testsDir := filepath.Join(root, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(testsDir, "helpers.ts"), `export const isOk = (r: { status: number }) => r.status < 300;`)
	write(filepath.Join(root, "shared.js"), `export const two = 2;`)
	write(filepath.Join(filepath.Dir(root), "outside.js"), `export const x = 1;`)

	opts := Options{Filename: "a.test.js", ResolveDir: testsDir, Root: root}
	mustPass(t, Run(`
import { isOk } from "./helpers";
import { two } from "../shared.js";
test("imports", () => { expect(isOk(response)).toBe(true); expect(two).toBe(2); });
`, baseData(), opts))

	r := Run(`import { x } from "../../outside.js";`, baseData(), opts)
	if !strings.Contains(r.ScriptError, "outside the allowed directory") {
		t.Errorf("escape error = %q", r.ScriptError)
	}

	r = Run(`import { z } from "zod";`, baseData(), opts)
	if !strings.Contains(r.ScriptError, "https://esm.sh/zod") {
		t.Errorf("bare import error = %q", r.ScriptError)
	}

	r = Run(`import { z } from "https://esm.sh/zod";`, baseData(), opts)
	if !strings.Contains(r.ScriptError, "URL imports are disabled") {
		t.Errorf("disabled URL import error = %q", r.ScriptError)
	}
}

func TestFormat(t *testing.T) {
	out := Format(Result{
		Results:   []TestResult{{Name: "ok", Passed: true}, {Name: "bad", Message: "why"}},
		Logs:      []LogEntry{{Level: "log", Message: "hi"}},
		PassCount: 1, FailCount: 1,
	})
	for _, want := range []string{"[PASS] ok", "[FAIL] bad", "why", "1 passed, 1 failed", "[log] hi"} {
		if !strings.Contains(out, want) {
			t.Errorf("Format missing %q:\n%s", want, out)
		}
	}
}
