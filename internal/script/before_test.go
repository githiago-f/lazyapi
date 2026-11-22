package script

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/model"
)

func TestBefore_ChangesRequest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.js"), []byte(`export const token = () => "t-" + env.get("TOKEN");`), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `
import { token } from "./auth.js";
// Top level runs after the response: it must not break before().
const user = response.json();
console.log("top level");
vars.set("leak", "no");
lazyapi.send("shouldNotRun");
test("post-response test", () => expect(user.id).toBe(1));

export function before(req) {
  req.headers.Authorization = "Bearer " + token();
  req.query.trace = "1";
  req.params.id = "42";
  req.body = { name: "ana", at: 1 };
  vars.set("seen", req.method);
  console.log("before ran");
}
`
	data := baseData()
	data.Vars = map[string]string{}
	sends := 0
	data.Send = func(string, SendOptions) (*Response, error) { sends++; return okResponse("{}"), nil }

	res := Before(src, data, Options{ResolveDir: dir})
	if res.Error != "" || !res.Defined {
		t.Fatalf("Before = %+v", res)
	}
	r := res.Request
	if r.Headers["Authorization"] != "Bearer t-t0k" || r.Query["trace"] != "1" || r.Params["id"] != "42" {
		t.Errorf("request = %+v", r)
	}
	if r.Body.Raw != `{"at":1,"name":"ana"}` {
		t.Errorf("body = %q", r.Body.Raw)
	}
	if sends != 0 {
		t.Errorf("top-level lazyapi.send must not run while preparing before(): %d calls", sends)
	}
	if data.Vars["seen"] != "POST" || data.Vars["leak"] != "" {
		t.Errorf("vars = %v", data.Vars)
	}
	if len(res.Logs) != 1 || res.Logs[0].Message != "before ran" {
		t.Errorf("logs = %+v (top-level output must be dropped)", res.Logs)
	}
}

func TestBefore_ReturnsNewObjectAndCanSend(t *testing.T) {
	data := baseData()
	data.Send = func(target string, _ SendOptions) (*Response, error) {
		return okResponse(`{"token":"from-login"}`), nil
	}
	src := `export function before(req) {
  const token = lazyapi.send("login").json().token;
  return { ...req, headers: { ...req.headers, Authorization: "Bearer " + token } };
}`
	res := Before(src, data, Options{})
	if res.Error != "" || res.Request.Headers["Authorization"] != "Bearer from-login" {
		t.Fatalf("Before = %+v", res)
	}
	if res.Request.Headers["Content-Type"] != "application/json" {
		t.Errorf("existing headers must be kept: %v", res.Request.Headers)
	}
}

func TestBefore_NotDefinedAndErrors(t *testing.T) {
	if res := Before(`test("x", () => {});`, baseData(), Options{}); res.Defined || res.Error != "" {
		t.Errorf("no before: %+v", res)
	}
	res := Before(`export function before(req) { throw new Error("no credentials"); }`, baseData(), Options{Filename: "t.js"})
	if !res.Defined || !strings.Contains(res.Error, "before(): Error: no credentials") {
		t.Errorf("throwing before: %+v", res)
	}
	if res := Before(`export function before( {`, baseData(), Options{}); !strings.Contains(res.Error, "Unexpected") && !strings.Contains(res.Error, "Expected") {
		t.Errorf("syntax error: %+v", res)
	}
	// The original request is returned unchanged on error.
	if res.Request.Method != model.POST {
		t.Errorf("request lost: %+v", res.Request)
	}
}

func TestRun_IgnoresBeforeExport(t *testing.T) {
	src := `export function before(req) { throw new Error("must not run"); }
test("ok", () => expect(response.status).toBe(200));`
	mustPass(t, Run(src, baseData(), Options{}))
}
