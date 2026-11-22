package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/githiago-f/lazyapi/internal/store"
)

const apiSpec = `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /users/{id}:
    get:
      operationId: getUser
      parameters:
        - {in: path, name: id, required: true, schema: {type: string}, example: "1"}
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                required: [id]
                properties: {id: {type: integer}}
  /users:
    post:
      operationId: createUser
      responses:
        "201": {description: Created}
`

// client drives a Server over in-memory pipes.
type client struct {
	t   *testing.T
	in  io.WriteCloser
	out *bufio.Scanner
	id  int
}

func startServer(t *testing.T, root string) *client {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	srv := &Server{Name: "lazyapi", Version: "test", Tools: &Tools{Root: root, FindSpecs: store.FindSpecFiles}}
	go func() {
		_ = srv.Serve(context.Background(), inR, outW)
		_ = outW.Close()
	}()
	t.Cleanup(func() { _ = inW.Close() })
	sc := bufio.NewScanner(outR)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	return &client{t: t, in: inW, out: sc}
}

func (c *client) send(msg string) {
	c.t.Helper()
	if _, err := io.WriteString(c.in, msg+"\n"); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) call(method string, params any) map[string]any {
	c.t.Helper()
	c.id++
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.id, "method": method, "params": params})
	c.send(string(data))
	if !c.out.Scan() {
		c.t.Fatalf("no response to %s", method)
	}
	var resp map[string]any
	if err := json.Unmarshal(c.out.Bytes(), &resp); err != nil {
		c.t.Fatalf("bad response %s: %v", c.out.Text(), err)
	}
	if resp["id"] != float64(c.id) {
		c.t.Fatalf("response id = %v, want %d", resp["id"], c.id)
	}
	return resp
}

// tool calls a tool and returns its decoded JSON text and isError flag.
func (c *client) tool(name string, args map[string]any) (map[string]any, string, bool) {
	c.t.Helper()
	resp := c.call("tools/call", map[string]any{"name": name, "arguments": args})
	if resp["error"] != nil {
		c.t.Fatalf("%s: rpc error %v", name, resp["error"])
	}
	result := resp["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	var out map[string]any
	_ = json.Unmarshal([]byte(text), &out)
	return out, text, result["isError"].(bool)
}

func setupRoot(t *testing.T, serverURL string) string {
	t.Helper()
	root := t.TempDir()
	spec := strings.Replace(apiSpec, "paths:", "servers: [{url: \""+serverURL+"\"}]\npaths:", 1)
	if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", "spec.yml"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestProtocol(t *testing.T) {
	c := startServer(t, t.TempDir())

	init := c.call("initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "1"}})
	result := init["result"].(map[string]any)
	if result["protocolVersion"] != "2025-03-26" {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
	if result["capabilities"].(map[string]any)["tools"] == nil {
		t.Error("tools capability missing")
	}

	c.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`) // no response expected

	if r := c.call("ping", nil); r["result"] == nil {
		t.Errorf("ping = %v", r)
	}

	list := c.call("tools/list", nil)["result"].(map[string]any)["tools"].([]any)
	names := map[string]bool{}
	for _, tl := range list {
		m := tl.(map[string]any)
		names[m["name"].(string)] = true
		if m["inputSchema"].(map[string]any)["type"] != "object" {
			t.Errorf("%s: inputSchema must be an object schema", m["name"])
		}
	}
	for _, want := range []string{"list_specs", "list_operations", "describe_operation", "send_request", "run_tests",
		"run_flow", "write_flow", "import_curl", "export_curl", "read_test", "write_test", "validate_spec", "script_api"} {
		if !names[want] {
			t.Errorf("missing tool %s", want)
		}
	}

	if r := c.call("nope/method", nil); r["error"].(map[string]any)["code"] != float64(errMethodNotFound) {
		t.Errorf("unknown method = %v", r)
	}
	if r := c.call("tools/call", map[string]any{"name": "nope"}); r["error"] == nil {
		t.Errorf("unknown tool should be an rpc error: %v", r)
	}

	c.send(`{not json`)
	if !c.out.Scan() || !strings.Contains(c.out.Text(), "-32700") {
		t.Errorf("parse error = %s", c.out.Text())
	}

	// Unknown version falls back to the newest supported one.
	c2 := startServer(t, t.TempDir())
	r := c2.call("initialize", map[string]any{"protocolVersion": "1999-01-01"})
	if r["result"].(map[string]any)["protocolVersion"] != ProtocolVersions[0] {
		t.Errorf("fallback version = %v", r)
	}
}

func TestTools_AgentLoop(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(201)
			return
		}
		_, _ = w.Write([]byte(`{"id":"wrong-type"}`))
	}))
	defer api.Close()
	root := setupRoot(t, api.URL)
	c := startServer(t, root)

	specs, _, _ := c.tool("list_specs", map[string]any{})
	if got := specs["specs"].([]any); len(got) != 1 || got[0] != "api/spec.yml" {
		t.Fatalf("specs = %v", specs)
	}

	ops, _, _ := c.tool("list_operations", map[string]any{"spec": "api/spec.yml"})
	if len(ops["operations"].([]any)) != 2 {
		t.Errorf("operations = %v", ops)
	}

	d, _, isErr := c.tool("describe_operation", map[string]any{"spec": "api/spec.yml", "operation": "getUser"})
	if isErr || d["path"] != "/users/{id}" {
		t.Errorf("describe = %v", d)
	}

	// The API returns a string id: the contract check must fail with a reason.
	res, text, isErr := c.tool("send_request", map[string]any{"spec": "api/spec.yml", "operation": "getUser"})
	if isErr || res["outcome"] != "failed" || !strings.Contains(text, "value must be an integer") {
		t.Errorf("send_request = %s", text)
	}
	if res["response"].(map[string]any)["body"] != `{"id":"wrong-type"}` {
		t.Errorf("body missing: %s", text)
	}
	if res, text, _ := c.tool("send_request", map[string]any{"spec": "api/spec.yml", "operation": "getUser", "jq": ".id"}); res["response"].(map[string]any)["body"] != `"wrong-type"` {
		t.Errorf("jq body: %s", text)
	}
	if _, text, isErr := c.tool("send_request", map[string]any{"spec": "api/spec.yml", "operation": "getUser", "jq": ".["}); !isErr || !strings.Contains(text, "jq:") {
		t.Errorf("invalid jq must be an error: %s", text)
	}

	// Write a test, read it back, run everything.
	w, text, isErr := c.tool("write_test", map[string]any{"spec": "api/spec.yml", "operation": "getUser",
		"source": `test("has id", () => expect(response.json()).toHaveProperty("id"));`})
	if isErr || w["path"] != "api/tests/getUser.js" || w["compileError"] != nil {
		t.Errorf("write_test = %s", text)
	}
	r, _, _ := c.tool("read_test", map[string]any{"spec": "api/spec.yml", "operation": "getUser"})
	if r["exists"] != true || !strings.Contains(r["source"].(string), "has id") {
		t.Errorf("read_test = %v", r)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "api", "spec.yml"))
	if !strings.Contains(string(raw), "x-lazyapi-tests: tests/getUser.js") {
		t.Errorf("spec not linked:\n%s", raw)
	}

	rep, text, _ := c.tool("run_tests", map[string]any{"spec": "api/spec.yml"})
	summary := rep["summary"].(map[string]any)
	if summary["failed"] != float64(1) || summary["skipped"] != float64(1) {
		t.Errorf("run_tests summary = %v\n%s", summary, text)
	}
	if !strings.Contains(text, `"name": "has id"`) {
		t.Errorf("script check missing from report: %s", text)
	}

	rep, _, _ = c.tool("run_tests", map[string]any{"spec": "api/spec.yml", "methods": []string{"all"}, "operations": []string{"createUser"}})
	if rep["summary"].(map[string]any)["passed"] != float64(1) {
		t.Errorf("POST with methods=all = %v", rep["summary"])
	}

	// Compile errors are reported, not fatal.
	w, _, _ = c.tool("write_test", map[string]any{"spec": "api/spec.yml", "operation": "getUser", "source": `test("x", () => {`})
	if w["compileError"] == nil {
		t.Errorf("expected compileError: %v", w)
	}
	v, _, _ := c.tool("validate_spec", map[string]any{"spec": "api/spec.yml"})
	if v["valid"] != false {
		t.Errorf("validate should report the broken script: %v", v)
	}

	_, apiTypes, _ := c.tool("script_api", map[string]any{})
	if !strings.Contains(apiTypes, "declare function expect") {
		t.Error("script_api should return the d.ts")
	}
}

func TestTools_Confinement(t *testing.T) {
	root := setupRoot(t, "http://127.0.0.1:1")
	c := startServer(t, root)

	for _, bad := range []string{"../outside.yml", "/etc/passwd", "api/../../x.yml"} {
		_, text, isErr := c.tool("list_operations", map[string]any{"spec": bad})
		if !isErr || !strings.Contains(text, "outside the server root") {
			t.Errorf("%s: %s", bad, text)
		}
	}
	_, text, isErr := c.tool("send_request", map[string]any{"spec": "api/spec.yml", "operation": "getUser", "env_file": "../.env"})
	if !isErr || !strings.Contains(text, "outside the server root") {
		t.Errorf("env_file escape: %s", text)
	}
	_, text, isErr = c.tool("list_operations", map[string]any{"spec": "api/spec.yml", "unknown": 1})
	if !isErr || !strings.Contains(text, "unknown field") {
		t.Errorf("unknown argument should be rejected: %s", text)
	}
	// A network failure is an in-band tool result, not a crash.
	res, text, isErr := c.tool("send_request", map[string]any{"spec": "api/spec.yml", "operation": "getUser"})
	if isErr || res["outcome"] != "error" || !strings.Contains(text, "request failed") {
		t.Errorf("network error = %s", text)
	}
}

func TestTools_Flows(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(201)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer api.Close()
	root := setupRoot(t, api.URL)
	c := startServer(t, root)

	w, text, isErr := c.tool("write_flow", map[string]any{"spec": "api/spec.yml", "name": "signup",
		"source": `const r = lazyapi.send("createUser"); test("created", () => expect(r).toHaveStatus(201));
const u = lazyapi.send("getUser", { params: { id: "1" } }); test("readable", () => expect(u.json().id).toBe(1));`})
	if isErr || w["path"] != "api/flows/signup.js" || w["compileError"] != nil {
		t.Fatalf("write_flow = %s", text)
	}

	res, text, isErr := c.tool("run_flow", map[string]any{"spec": "api/spec.yml", "flow": "signup"})
	if isErr || res["outcome"] != "passed" || len(res["steps"].([]any)) != 2 {
		t.Errorf("run_flow = %s", text)
	}
	if !strings.Contains(text, "#2 GET /users/{id} · response matches the OpenAPI schema") {
		t.Errorf("each step should be contract-checked: %s", text)
	}

	rep, _, _ := c.tool("run_tests", map[string]any{"spec": "api/spec.yml", "include_flows": true})
	ops := rep["operations"].([]any)
	if last := ops[len(ops)-1].(map[string]any); last["kind"] != "flow" {
		t.Errorf("include_flows should append the flow: %v", last)
	}

	for _, bad := range []string{"../x", "a/b", ""} {
		if _, _, isErr := c.tool("write_flow", map[string]any{"spec": "api/spec.yml", "name": bad, "source": "x"}); !isErr {
			t.Errorf("name %q should be rejected", bad)
		}
	}
	if _, text, isErr := c.tool("run_flow", map[string]any{"spec": "api/spec.yml", "flow": "../../etc/passwd"}); !isErr || !strings.Contains(text, "outside the server root") {
		t.Errorf("flow path escape: %s", text)
	}
}

func TestTools_Curl(t *testing.T) {
	root := setupRoot(t, "https://api.example.com")
	c := startServer(t, root)

	res, text, isErr := c.tool("import_curl", map[string]any{"spec": "api/new.yml",
		"command": `curl -X POST https://api.example.com/orders -H 'Authorization: Bearer x' -d '{"sku":"a"}'`})
	if isErr || len(res["added"].([]any)) != 1 || res["added"].([]any)[0] != "POST /orders" {
		t.Fatalf("import_curl = %s", text)
	}
	if raw, _ := os.ReadFile(filepath.Join(root, "api", "new.yml")); strings.Contains(string(raw), "Bearer x") {
		t.Error("credentials must not be copied")
	}

	out, text, isErr := c.tool("export_curl", map[string]any{"spec": "api/spec.yml", "operation": "getUser", "params": map[string]any{"id": "9"}})
	if isErr || out["curl"] != "curl https://api.example.com/users/9" {
		t.Errorf("export_curl = %s", text)
	}
	if _, text, isErr := c.tool("import_curl", map[string]any{"spec": "../x.yml", "command": "curl https://a.b"}); !isErr || !strings.Contains(text, "outside the server root") {
		t.Errorf("import_curl confinement: %s", text)
	}
}

func TestTools_RunTestsWithData(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()
	root := setupRoot(t, srv.URL)
	if err := os.WriteFile(filepath.Join(root, "api", "ids.csv"), []byte("uid\n5\n6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := startServer(t, root)
	res, text, isErr := c.tool("run_tests", map[string]any{"spec": "api/spec.yml", "operations": []string{"getUser"},
		"params": map[string]any{"id": "{{env.uid}}"}, "data_file": "api/ids.csv"})
	if isErr || res["iterations"] != float64(2) {
		t.Fatalf("run_tests = %s", text)
	}
	if strings.Join(paths, ",") != "/users/5,/users/6" {
		t.Errorf("paths = %v", paths)
	}
	if _, text, isErr := c.tool("run_tests", map[string]any{"spec": "api/spec.yml", "data_file": "../x.csv"}); !isErr || !strings.Contains(text, "outside the server root") {
		t.Errorf("data_file must stay inside the root: %s", text)
	}
}
