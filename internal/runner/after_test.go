package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/githiago-f/lazyapi/internal/store"
)

const afterSpec = `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /items:
    post:
      operationId: createItem
      x-lazyapi-tests: tests/createItem.js
      responses: {"201": {description: created}}
  /items/{id}:
    parameters: [{in: path, name: id, required: true, schema: {type: string}}]
    delete:
      operationId: deleteItem
      responses: {"204": {description: deleted}}
`

func TestRun_AfterCleansUp(t *testing.T) {
	var mu sync.Mutex
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":"abc"}`))
			return
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "spec.yml")
	if err := os.WriteFile(path, []byte(afterSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := `test("created", () => expect(response.status).toBe(201));
export function after(res) {
  lazyapi.send("deleteItem", { params: { id: res.json().id } });
}`
	if err := os.WriteFile(filepath.Join(dir, "tests", "createItem.js"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := store.ParseSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), spec, path, Options{Server: srv.URL, Filter: Filter{Ops: []string{"createItem"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(hits, ","); got != "POST /items,DELETE /items/abc" {
		t.Errorf("requests = %s", got)
	}
	op := rep.Operations[0]
	found := false
	for _, c := range op.Checks {
		if c.Name == "after() ran without errors" && c.Passed {
			found = true
		}
	}
	if !found || op.Outcome != Passed {
		t.Errorf("result = %+v", op)
	}
}
