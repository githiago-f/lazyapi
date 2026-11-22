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

func TestLoadData(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	rows, err := LoadData(write("u.csv", "\xef\xbb\xbfid, name\n1, ana\n2,\"bia, b\"\n"))
	if err != nil || len(rows) != 2 || rows[0]["id"] != "1" || rows[0]["name"] != "ana" || rows[1]["name"] != "bia, b" {
		t.Errorf("csv = %v, %v", rows, err)
	}
	rows, err = LoadData(write("u.json", `[{"id": 7, "ok": true, "tags": ["a"]}, {"id": "x", "n": null}]`))
	if err != nil || rows[0]["id"] != "7" || rows[0]["ok"] != "true" || rows[0]["tags"] != `["a"]` || rows[1]["n"] != "" {
		t.Errorf("json = %v, %v", rows, err)
	}
	for name, content := range map[string]string{"e.csv": "id\n", "e.json": `{"id": 1}`, "e.txt": "x", "h.csv": "id,\n1,2\n"} {
		if _, err := LoadData(write(name, content)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRun_DataDriven(t *testing.T) {
	var mu sync.Mutex
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.yml")
	if err := os.WriteFile(spec, []byte(`openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /users/{id}:
    get:
      operationId: getUser
      x-lazyapi-tests: tests/getUser.js
      parameters: [{in: path, name: id, required: true, schema: {type: string}}]
      responses: {"200": {description: ok}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(dir, "tests"), 0o755)
	script := `
test("iteration " + lazyapi.iteration + "/" + lazyapi.iterations, () => {
  expect(env.get("name")).toBe(lazyapi.iteration === 0 ? "ana" : "bia");
  expect(pm.info.iteration).toBe(lazyapi.iteration);
  expect(pm.iterationData.get("id")).toBe(request.params.id);
});
env.set("leak", "from " + lazyapi.iteration);
test("rows are isolated", () => expect(env.get("leak")).toBe("from " + lazyapi.iteration));`
	if err := os.WriteFile(filepath.Join(dir, "tests", "getUser.js"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := store.ParseSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	var streamed []int
	opts := Options{
		Server: srv.URL, Params: map[string]string{"id": "{{env.id}}"},
		Data:     []map[string]string{{"id": "1", "name": "ana"}, {"id": "2", "name": "bia"}},
		OnResult: func(r OpResult) { streamed = append(streamed, r.Iteration) },
	}
	if n := PlannedCount(doc, spec, opts); n != 2 {
		t.Errorf("planned = %d", n)
	}
	rep, err := Run(context.Background(), doc, spec, opts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(hits, ",") != "/users/1,/users/2" {
		t.Errorf("hits = %v", hits)
	}
	if rep.Iterations != 2 || len(rep.Operations) != 2 || rep.Summary.Passed != 2 {
		for _, op := range rep.Operations {
			for _, c := range op.Checks {
				if !c.Passed {
					t.Logf("%s: %s: %s", op.Label(), c.Name, c.Message)
				}
			}
		}
		t.Fatalf("report = %+v", rep.Summary)
	}
	for i, op := range rep.Operations {
		if op.Iteration != i+1 || !strings.HasSuffix(op.Label(), "[#"+string(rune('1'+i))+"]") {
			t.Errorf("op %d: iteration %d label %q", i, op.Iteration, op.Label())
		}
		for _, c := range op.Checks {
			if !c.Passed {
				t.Errorf("op %d: %s: %s", i, c.Name, c.Message)
			}
		}
	}
	if len(streamed) != 2 || streamed[0] != 1 || streamed[1] != 2 {
		t.Errorf("OnResult iterations = %v", streamed)
	}
}
