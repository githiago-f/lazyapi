package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/store"
)

func TestShellSplit(t *testing.T) {
	got, err := shellSplit("curl 'a b' \"c \\\"d\\\"\" $'e\\nf' g\\ h \\\n  -X POST")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"curl", "a b", `c "d"`, "e\nf", "g h", "-X", "POST"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
	if _, err := shellSplit("curl 'open"); err == nil {
		t.Error("unterminated quote")
	}
}

func TestParseCurl(t *testing.T) {
	r, err := ParseCurl(`curl 'https://api.example.com/v1/users/42?expand=orders' \
  -H 'Authorization: Bearer secret' -H 'X-Request-Id: r1' -H 'content-type: application/json' \
  --data-raw '{"name":"ana","age":30}' --compressed -s`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "POST" || r.URL != "https://api.example.com/v1/users/42?expand=orders" || r.Body != `{"name":"ana","age":30}` {
		t.Errorf("request = %+v", r)
	}
	if len(r.Headers) != 3 {
		t.Errorf("headers = %+v", r.Headers)
	}

	r, _ = ParseCurl(`curl -X PUT https://x.io/a -u ana:pw -F 'caption=hi' -F 'file=@./me.png'`)
	if r.Method != "PUT" || len(r.Form) != 2 || !r.Form[1].File || r.Form[1].Value != "./me.png" || r.Headers[0].Value != "Basic YW5hOnB3" {
		t.Errorf("multipart = %+v", r)
	}

	r, _ = ParseCurl(`curl https://x.io/search -G --data-urlencode 'q=a b' -d page=2`)
	if r.Method != "GET" || r.URL != "https://x.io/search?q=a+b&page=2" {
		t.Errorf("-G = %+v", r)
	}

	r, _ = ParseCurl(`curl https://x.io/login -d 'user=ana&pass=pw'`)
	if r.Method != "POST" || len(r.Form) != 2 || r.Body != "" {
		t.Errorf("form data = %+v", r)
	}

	if _, err := ParseCurl("wget https://x.io"); err == nil {
		t.Error("not curl")
	}
	if _, err := ParseCurl("curl -H 'A: b'"); err == nil {
		t.Error("no URL")
	}
}

func TestApply(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "api.yml")
	spec := NewSpec("Imported")

	reqs := []Request{
		{Method: "GET", URL: "https://api.example.com/v1/users/42/orders/550e8400-e29b-41d4-a716-446655440000?limit=10",
			Headers:   []KV{{Key: "Authorization", Value: "Bearer secret"}, {Key: "X-Tenant", Value: "acme"}, {Key: "User-Agent", Value: "curl"}},
			Responses: []Response{{Status: 200, ContentType: "application/json", Body: `{"id":"550e8400-e29b-41d4-a716-446655440000","total":9.5,"items":[{"sku":"a"}]}`}}},
		{Method: "POST", URL: "https://api.example.com/v1/users", Body: `{"name":"ana","email":"ana@example.com"}`,
			Headers: []KV{{Key: "Content-Type", Value: "application/json"}, {Key: "X-API-Key", Value: "k"}}, Tests: `test("ok", () => {});`},
		{Name: "Upload avatar", Method: "POST", URL: "https://api.example.com/v1/users/:userId/avatar",
			Form: []KV{{Key: "caption", Value: "hi"}, {Key: "file", Value: "me.png", File: true}}, Tags: []string{"Users"}},
		{Method: "GET", URL: "https://api.example.com/v1/users/42/orders/550e8400-e29b-41d4-a716-446655440000"}, // duplicate
		{Method: "GET", URL: "not a url"},
	}
	res, err := Apply(spec, specPath, reqs)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 3 || len(res.Skipped) != 2 {
		t.Fatalf("result = %+v", res)
	}
	if res.Added[0] != "GET /v1/users/{id}/orders/{orderId}" {
		t.Errorf("templated path = %s", res.Added[0])
	}
	if len(spec.Servers) != 1 || spec.Servers[0].URL != "https://api.example.com" {
		t.Errorf("servers = %+v", spec.Servers)
	}
	if len(res.Notes) < 2 || !strings.Contains(strings.Join(res.Notes, " "), "bearerAuth") {
		t.Errorf("notes = %v", res.Notes)
	}

	get := spec.Paths.Find("/v1/users/{id}/orders/{orderId}").Get
	if get.OperationID != "getV1UsersByIdOrdersByOrderId" || len(get.Parameters) != 4 {
		t.Errorf("get op = %s %d params", get.OperationID, len(get.Parameters))
	}
	if get.Parameters[0].Value.Example != int64(42) || get.Parameters[2].Value.Name != "limit" {
		t.Errorf("params: %+v %+v", get.Parameters[0].Value, get.Parameters[2].Value)
	}
	if (*get.Security)[0]["bearerAuth"] == nil {
		t.Error("bearer security")
	}
	body := get.Responses.Value("200").Value.Content["application/json"]
	if body.Schema.Value.Properties["id"].Value.Format != "uuid" || !body.Schema.Value.Properties["total"].Value.Type.Is("number") {
		t.Errorf("inferred schema = %+v", body.Schema.Value)
	}

	post := spec.Paths.Find("/v1/users").Post
	if post.RequestBody.Value.Content["application/json"].Schema.Value.Properties["email"].Value.Format != "email" {
		t.Error("request body schema")
	}
	if post.Extensions[store.TestsExtension] != ".lazyapi/tests/postV1Users.js" {
		t.Errorf("tests link = %v", post.Extensions)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lazyapi", "tests", "postV1Users.js")); err != nil {
		t.Error("tests file not written")
	}

	up := spec.Paths.Find("/v1/users/{userId}/avatar").Post
	mt := up.RequestBody.Value.Content["multipart/form-data"]
	if up.OperationID != "uploadAvatar" || up.Tags[0] != "Users" || mt == nil || mt.Schema.Value.Properties["file"].Value.Format != "binary" {
		t.Errorf("upload op = %+v", up)
	}

	// The result must be a valid spec that round-trips.
	if err := store.SaveSpec(specPath, spec); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Validate(t.Context()); err != nil {
		t.Errorf("imported spec is invalid: %v", err)
	}
	data, _ := json.Marshal(res)
	if !strings.Contains(string(data), `"added"`) {
		t.Error("result json")
	}
}
