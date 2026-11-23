package runner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/store"
)

func TestRun_DynamicValues(t *testing.T) {
	var bodies []map[string]string
	var headers []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var m map[string]string
		_ = json.Unmarshal(data, &m)
		bodies = append(bodies, m)
		headers = append(headers, r.Header.Get("X-Request-Id"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.yml")
	if err := os.WriteFile(spec, []byte(`openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /users:
    post:
      operationId: createUser
      x-lazyapi-tests: tests/createUser.js
      parameters:
        - {in: header, name: X-Request-Id, required: true, schema: {type: string}, example: "{{$guid}}"}
      requestBody:
        content:
          application/json:
            example: {email: "{{$randomEmail}}", other: "{{$randomEmail}}", code: "{{$code}}", first: "{{$randomFirstName}}"}
      responses: {"201": {description: created}}
  /orders:
    post:
      operationId: createOrder
      requestBody:
        content:
          application/json:
            example: {ref: "{{$nope}}"}
      responses: {"201": {description: created}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(dir, "tests"), 0o755)
	src := `
// Helpers: {{$code}} calls code(); a helper named like a built-in wins.
export function code() { return "C-" + faker.int(100, 999); }
export function randomFirstName() { return "Ana"; }

test("the request carries the generated values", () => {
  const body = request.json();
  expect(body.email).toMatch(/@/);
  expect(body.email).not.toBe(body.other); // a new value per occurrence
  expect(body.code).toMatch(/^C-\d{3}$/);
  expect(body.first).toBe("Ana");
  expect(response.json()).toEqual(body);
});
test("faker in scripts", () => {
  expect(faker.uuid()).toMatch(/^[0-9a-f-]{36}$/);
  expect(faker.get("$randomInt")).toMatch(/^\d+$/);
  expect(["a", "b"]).toContain(faker.pick(["a", "b"]));
});`
	if err := os.WriteFile(filepath.Join(dir, "tests", "createUser.js"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := store.ParseSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), doc, spec, Options{Server: srv.URL, Filter: Filter{Methods: []string{"POST"}}})
	if err != nil {
		t.Fatal(err)
	}
	var user, order OpResult
	for _, op := range rep.Operations {
		if op.Path == "/users" {
			user = op
		} else {
			order = op
		}
	}
	if user.Outcome != Passed {
		for _, c := range user.Checks {
			t.Logf("%s: %v %s", c.Name, c.Passed, c.Message)
		}
		t.Fatalf("createUser = %s %s", user.Outcome, user.Reason)
	}
	if len(bodies) != 1 || len(headers[0]) != 36 {
		t.Fatalf("sent = %v %v", bodies, headers)
	}
	if user.Request == nil || !strings.Contains(user.Request.Body, bodies[0]["email"]) {
		t.Errorf("reported request differs from the one sent: %+v", user.Request)
	}
	if order.Outcome != Failed || !strings.Contains(order.Reason, "unknown dynamic variable {{$nope}}") {
		t.Errorf("createOrder = %s %s", order.Outcome, order.Reason)
	}
}
