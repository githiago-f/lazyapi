package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/importer"
	"github.com/githiago-f/lazyapi/internal/store"
)

const collection = `{
  "info": {"name": "Shop API", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "variable": [{"key": "baseUrl", "value": "%s"}, {"key": "token", "value": "s3cret"}],
  "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{token}}"}]},
  "item": [{
    "name": "Products",
    "item": [{
      "name": "List products",
      "event": [
        {"listen": "prerequest", "script": {"exec": ["pm.request.headers.add({key: 'X-Trace', value: 'from-pre'});"]}},
        {"listen": "test", "script": {"exec": [
          "pm.test('status is 200', () => pm.response.to.have.status(200));",
          "const json = pm.response.json();",
          "pm.test('has products', () => {",
          "  pm.expect(json.items).to.be.an('array').that.is.not.empty;",
          "  pm.expect(json.items[0]).to.have.property('sku').that.equals('a-1');",
          "  pm.expect(pm.response.responseTime).to.be.below(5000);",
          "});",
          "pm.environment.set('firstSku', json.items[0].sku);"
        ]}}
      ],
      "request": {
        "method": "GET",
        "header": [{"key": "Accept", "value": "application/json"}],
        "url": {"raw": "{{baseUrl}}/products?page=1", "host": ["{{baseUrl}}"], "path": ["products"], "query": [{"key": "page", "value": "1"}]}
      },
      "response": [{"code": 200, "header": [{"key": "Content-Type", "value": "application/json"}], "body": "{\"items\":[{\"sku\":\"a-1\"}]}"}]
    }, {
      "name": "Get product",
      "request": {"method": "GET", "url": "{{baseUrl}}/products/:sku"}
    }, {
      "name": "Create product",
      "request": {
        "method": "POST",
        "body": {"mode": "raw", "raw": "{\"sku\": \"{{newSku}}\", \"price\": 10}", "options": {"raw": {"language": "json"}}},
        "url": "{{baseUrl}}/products"
      }
    }]
  }]
}`

func TestImportPostmanAndRun(t *testing.T) {
	var gotAuth, gotTrace string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products" && r.Method == http.MethodGet {
			gotAuth, gotTrace = r.Header.Get("Authorization"), r.Header.Get("X-Trace")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"sku":"a-1"}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	src := filepath.Join(dir, "shop.postman_collection.json")
	if err := os.WriteFile(src, []byte(fmt.Sprintf(collection, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(dir, "shop.yml")

	out, err := captureStdout(t, func() error { return run([]string{"import", "postman", src, "--into", spec, "--json"}) })
	if err != nil {
		t.Fatalf("import: %v\n%s", err, out)
	}
	var res importer.Result
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatal(jerr)
	}
	if strings.Join(res.Added, ",") != "GET /products,GET /products/{sku},POST /products" {
		t.Errorf("added = %v", res.Added)
	}
	parsed, err := store.ParseSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Info.Title != "Shop API" || parsed.Paths.Find("/products").Get.Tags[0] != "Products" {
		t.Errorf("title/tags: %s %v", parsed.Info.Title, parsed.Paths.Find("/products").Get.Tags)
	}
	raw, _ := os.ReadFile(spec)
	if strings.Contains(string(raw), "s3cret") {
		t.Error("the token must not be copied into the spec")
	}
	envData, _ := os.ReadFile(filepath.Join(dir, ".lazyapi", "secrets", "postman.env"))
	if !strings.Contains(string(envData), "token=s3cret") {
		t.Errorf("secrets/postman.env = %s", envData)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lazyapi", "config.yml")); err != nil {
		t.Error("config with the postman environment should be created")
	}

	// The translated Postman scripts run, with the collection token applied
	// via the bearerAuth scheme from the environment's secrets file.
	cfg := "environments:\n  postman:\n    auth:\n      bearerAuth: {token: \"{{env.token}}\"}\n"
	if err := os.WriteFile(filepath.Join(dir, ".lazyapi", "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = captureStdout(t, func() error {
		return RunTests([]string{spec, "--env-name", "postman", "--op", "listProducts", "--output", "json"})
	})
	if err != nil {
		t.Fatalf("run imported collection: %v\n%s", err, out)
	}
	for _, want := range []string{`"name": "status is 200"`, `"name": "has products"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	if gotAuth != "Bearer s3cret" || gotTrace != "from-pre" {
		t.Errorf("auth=%q trace=%q (pre-request script should add X-Trace)", gotAuth, gotTrace)
	}

	// Re-importing skips existing operations.
	out, _ = captureStdout(t, func() error { return run([]string{"import", "postman", src, "--into", spec, "--json"}) })
	if !strings.Contains(out, "already in the spec") {
		t.Errorf("re-import should skip: %s", out)
	}
}

func TestImportCurlAndHAR(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")
	out, err := captureStdout(t, func() error {
		return run([]string{"import", "curl", "--into", spec, "--", `curl -X POST https://api.example.com/v1/orders -H 'Authorization: Bearer x' -H 'Content-Type: application/json' -d '{"sku":"a","qty":2}'`})
	})
	if err != nil || !strings.Contains(out, "+ POST /v1/orders") || !strings.Contains(out, "bearerAuth") {
		t.Fatalf("curl import: %v\n%s", err, out)
	}
	if out, _ := captureStdout(t, func() error { return run([]string{"validate", "spec", spec}) }); !strings.Contains(out, "is valid") {
		t.Errorf("imported spec should validate:\n%s", out)
	}

	har := `{"log":{"entries":[
	 {"request":{"method":"GET","url":"https://api.example.com/v1/orders/17","headers":[{"name":":authority","value":"x"},{"name":"X-Tenant","value":"t1"}]},
	  "response":{"status":200,"content":{"mimeType":"application/json","text":"{\"id\":17,\"sku\":\"a\"}"}}},
	 {"request":{"method":"GET","url":"https://api.example.com/v1/orders/99","headers":[]},
	  "response":{"status":404,"content":{"mimeType":"application/json","text":"{\"error\":\"not found\"}"}}},
	 {"request":{"method":"GET","url":"https://api.example.com/logo.png","headers":[]},
	  "response":{"status":200,"content":{"mimeType":"image/png","text":""}}}
	]}}`
	harPath := filepath.Join(dir, "capture.har")
	if err := os.WriteFile(harPath, []byte(har), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = captureStdout(t, func() error { return run([]string{"import", "har", harPath, "--into", spec, "--dry-run"}) })
	if err != nil || !strings.Contains(out, "Would add 1 operation") || !strings.Contains(out, "GET /v1/orders/{id}") {
		t.Fatalf("har dry run: %v\n%s", err, out)
	}
	if raw, _ := os.ReadFile(spec); strings.Contains(string(raw), "/v1/orders/{id}") {
		t.Error("--dry-run must not write the spec")
	}
	if _, err := captureStdout(t, func() error { return run([]string{"import", "har", harPath, "--into", spec}) }); err != nil {
		t.Fatal(err)
	}
	parsed, _ := store.ParseSpec(spec)
	op := parsed.Paths.Find("/v1/orders/{id}").Get
	if op.Responses.Value("200") == nil || op.Responses.Value("404") == nil {
		t.Error("both observed statuses should become response examples")
	}

	if err := run([]string{"import", "soap", "x", "--into", spec}); ExitCode(err) != 2 {
		t.Errorf("unknown source: %v", err)
	}
}

func TestImport_PostmanWithEnvironmentRuns(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1}]`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	coll := filepath.Join(dir, "shop.postman_collection.json")
	envJSON := filepath.Join(dir, "staging.postman_environment.json")
	spec := filepath.Join(dir, "api.yml")
	writeFile(t, coll, `{"info":{"name":"Shop","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
"auth":{"type":"bearer","bearer":[{"key":"token","value":"{{token}}"}]},
"item":[{"name":"List users","request":{"method":"GET","url":"{{baseUrl}}/users"}}]}`)
	writeFile(t, envJSON, fmt.Sprintf(`{"name":"Staging","_postman_variable_scope":"environment","values":[
{"key":"baseUrl","value":%q,"enabled":true},{"key":"token","value":"tok-123","type":"secret","enabled":true}]}`, srv.URL))

	if _, err := captureStdout(t, func() error {
		return Import([]string{"postman", coll, "--environment", envJSON, "--into", spec})
	}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{spec, filepath.Join(dir, ".lazyapi", "config.yml"), filepath.Join(dir, ".lazyapi", "envs", "staging.env")} {
		if data, _ := os.ReadFile(f); strings.Contains(string(data), "tok-123") {
			t.Errorf("%s contains the secret:\n%s", f, data)
		}
	}
	out, err := captureStdout(t, func() error {
		return RunTests([]string{spec, "--env-name", "staging"})
	})
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if gotAuth != "Bearer tok-123" {
		t.Errorf("Authorization = %q\n%s", gotAuth, out)
	}

	// postman-env alone adds another environment next to it.
	writeFile(t, envJSON, `{"name":"Prod","values":[{"key":"baseUrl","value":"https://api.example.com","enabled":true}]}`)
	if _, err := captureStdout(t, func() error { return Import([]string{"postman-env", envJSON, "--into", spec}) }); err != nil {
		t.Fatal(err)
	}
	cfg, _ := os.ReadFile(filepath.Join(dir, ".lazyapi", "config.yml"))
	if !strings.Contains(string(cfg), "prod:") || !strings.Contains(string(cfg), "staging:") {
		t.Errorf("config:\n%s", cfg)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
