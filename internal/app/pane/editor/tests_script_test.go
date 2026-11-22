package editor

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/script"
)

func TestRunScript_UsesSpecContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")
	if err := os.WriteFile(spec, []byte(`openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: {type: object, required: [id], properties: {id: {type: integer}}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tests", "util.js"), []byte(`export const one = 1;`), 0o644); err != nil {
		t.Fatal(err)
	}

	ref := &model.OpenAPIRef{FilePath: spec, Path: "/items", Method: "GET"}
	req := model.Request{FileName: spec, OpenAPIRef: ref, ServerURL: srv.URL, URI: "/items", Method: model.GET}
	data := script.ExecData{
		Request: req,
		Response: &script.Response{
			Status: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: `{"id":1}`, Ref: ref,
		},
	}

	result := runScript(`
import { one } from "./util.js";
test("contract", () => expect(response).toMatchSchema());
test("send", () => expect(lazyapi.send("listItems").json().id).toBe(one));
`, data, req)

	if result.ScriptError != "" || result.FailCount != 0 || result.PassCount != 2 {
		t.Fatalf("result = %+v", result)
	}
	if out := formatTestResults(result); !strings.Contains(out, "2 passed") {
		t.Errorf("format = %q", out)
	}
}

func TestTestsPaneValue_DefaultTemplateIsEmpty(t *testing.T) {
	tp := TestsTab()
	tp.SetValue("")
	if tp.Value() != "" {
		t.Error("default template should not be persisted")
	}
	tp.SetValue(`test("x", () => {})`)
	if tp.Value() == "" {
		t.Error("custom script should be persisted")
	}
}
