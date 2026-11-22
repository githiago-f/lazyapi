package mock

import (
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

const spec = `openapi: 3.0.0
info: {title: t, version: "1"}
servers: [{url: "https://api.example.com/v1"}]
paths:
  /users:
    get:
      operationId: listUsers
      responses:
        "200":
          description: OK
          headers:
            X-Total: {schema: {type: integer, example: 2}}
          content:
            application/json:
              example: [{id: 1, name: ana}, {id: 2, name: bia}]
    post:
      operationId: createUser
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties: {name: {type: string}}
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties: {id: {type: integer, minimum: 1}, name: {type: string}}
        "422": {description: Invalid}
  /users/me:
    get:
      operationId: me
      responses:
        "200":
          description: OK
          content:
            application/json:
              examples:
                admin: {value: {id: 0, role: admin}}
                guest: {value: {id: 0, role: guest}}
  /users/{id}:
    get:
      operationId: getUser
      parameters:
        - {in: path, name: id, required: true, schema: {type: integer}}
      responses:
        "200":
          description: OK
          content:
            application/json:
              example: {id: 1, name: ana}
        "404":
          description: Missing
          content:
            application/json:
              example: {error: not found}
    delete:
      operationId: deleteUser
      parameters:
        - {in: path, name: id, required: true, schema: {type: integer}}
      responses:
        "204": {description: Deleted}
`

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	p := filepath.Join(t.TempDir(), "api.yml")
	if err := os.WriteFile(p, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := store.ParseSpec(p)
	if err != nil {
		t.Fatal(err)
	}
	m := New(s, p)
	if m.Operations() != 5 {
		t.Errorf("operations = %d", m.Operations())
	}
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, method, url, body string, headers ...string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return resp, string(data)
}

func TestMock_Examples(t *testing.T) {
	srv := newServer(t)

	resp, body := do(t, "GET", srv.URL+"/users", "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"name": "bia"`) || resp.Header.Get("X-Total") != "2" {
		t.Errorf("list: %d %s %v", resp.StatusCode, body, resp.Header)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content type = %q", resp.Header.Get("Content-Type"))
	}

	// Server base path (/v1) is accepted too; literal /users/me wins over /users/{id}.
	resp, body = do(t, "GET", srv.URL+"/v1/users/me", "")
	if resp.StatusCode != 200 || !strings.Contains(body, `"admin"`) {
		t.Errorf("me: %d %s", resp.StatusCode, body)
	}
	_, body = do(t, "GET", srv.URL+"/users/me", "", "Prefer", "example=guest")
	if !strings.Contains(body, `"guest"`) {
		t.Errorf("Prefer example: %s", body)
	}

	resp, body = do(t, "GET", srv.URL+"/users/7", "", "Prefer", "code=404")
	if resp.StatusCode != 404 || !strings.Contains(body, "not found") {
		t.Errorf("Prefer code: %d %s", resp.StatusCode, body)
	}

	resp, _ = do(t, "DELETE", srv.URL+"/users/7", "")
	if resp.StatusCode != 204 {
		t.Errorf("delete = %d", resp.StatusCode)
	}

	// No example: a sample generated from the schema that honours minimum.
	resp, body = do(t, "POST", srv.URL+"/users", `{"name":"ana"}`, "Content-Type", "application/json")
	var created map[string]any
	if resp.StatusCode != 201 || json.Unmarshal([]byte(body), &created) != nil || created["id"] != float64(1) {
		t.Errorf("create: %d %s", resp.StatusCode, body)
	}
}

func TestMock_ValidationAndRouting(t *testing.T) {
	srv := newServer(t)

	resp, body := do(t, "POST", srv.URL+"/users", `{"nome":"ana"}`, "Content-Type", "application/json")
	if resp.StatusCode != 422 || !strings.Contains(body, `property \"name\" is missing`) {
		t.Errorf("invalid body: %d %s", resp.StatusCode, body)
	}
	resp, body = do(t, "GET", srv.URL+"/users/abc", "")
	if resp.StatusCode != 422 || !strings.Contains(body, `path parameter \"id\"`) {
		t.Errorf("invalid path param: %d %s", resp.StatusCode, body)
	}

	resp, _ = do(t, "GET", srv.URL+"/nope", "")
	if resp.StatusCode != 404 {
		t.Errorf("unknown path = %d", resp.StatusCode)
	}
	resp, _ = do(t, "PATCH", srv.URL+"/users/1", "")
	if resp.StatusCode != 405 || resp.Header.Get("Allow") != "DELETE, GET" {
		t.Errorf("method not allowed = %d %q", resp.StatusCode, resp.Header.Get("Allow"))
	}
	resp, _ = do(t, "OPTIONS", srv.URL+"/users/1", "")
	if resp.StatusCode != 204 || resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Errorf("preflight = %d", resp.StatusCode)
	}
}

func TestParsePrefer(t *testing.T) {
	p := parsePrefer(`code=404, example="empty"; dynamic=true`)
	if p["code"] != "404" || p["example"] != "empty" || p["dynamic"] != "true" {
		t.Errorf("prefer = %v", p)
	}
}
