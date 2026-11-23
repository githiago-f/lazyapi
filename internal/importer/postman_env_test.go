package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/project"
)

const pmEnvJSON = `{
  "name": "Staging EU",
  "_postman_variable_scope": "environment",
  "values": [
    {"key": "baseUrl", "value": "https://staging.example.com/v1/", "type": "default", "enabled": true},
    {"key": "token", "value": "s3cr3t", "type": "secret", "enabled": true},
    {"key": "note", "value": "line1\nline2", "enabled": true},
    {"key": "retries", "value": 3, "enabled": true},
    {"key": "old", "value": "x", "enabled": false}
  ]
}`

func TestParsePostmanEnvironment(t *testing.T) {
	e, err := ParsePostmanEnvironment([]byte(pmEnvJSON))
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Staging EU" || e.Values["token"] != "s3cr3t" || e.Values["retries"] != "3" || len(e.Disabled) != 1 || len(e.Secret) != 1 {
		t.Errorf("parsed = %+v", e)
	}
	if EnvironmentName(e.Name) != "staging-eu" {
		t.Errorf("name = %q", EnvironmentName(e.Name))
	}
	if _, err := ParsePostmanEnvironment([]byte(`{"info":{}}`)); err == nil {
		t.Error("a collection is not an environment")
	}
}

func TestWriteEnvironment(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")
	cfg := filepath.Join(dir, project.FileName)
	_ = os.MkdirAll(filepath.Dir(cfg), 0o755)
	if err := os.WriteFile(cfg, []byte("# keep me\ndefault: local\nenvironments:\n  local:\n    server: http://localhost:8080 # dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, _ := ParsePostmanEnvironment([]byte(pmEnvJSON))
	var res Result
	if err := WriteEnvironment(spec, "staging-eu", e, &res); err != nil {
		t.Fatal(err)
	}

	// Plain variables are committed, secrets go to the git-ignored file.
	plainPath := filepath.Join(dir, ".lazyapi", "envs", "staging-eu.env")
	secretPath := filepath.Join(dir, ".lazyapi", "secrets", "staging-eu.env")
	if fi, err := os.Stat(secretPath); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("secrets file: %v %v", fi, err)
	}
	if data, _ := os.ReadFile(plainPath); strings.Contains(string(data), "s3cr3t") || !strings.Contains(string(data), "retries=3") {
		t.Errorf("envs file:\n%s", data)
	}
	if ig, _ := os.ReadFile(filepath.Join(dir, ".lazyapi", ".gitignore")); !strings.Contains(string(ig), "secrets/") {
		t.Errorf(".gitignore = %q", ig)
	}
	vals, err := env.NewStore(plainPath, secretPath).Load()
	if err != nil || vals["token"] != "s3cr3t" || vals["note"] != "line1\nline2" || vals["baseUrl"] != "https://staging.example.com/v1/" {
		t.Errorf("env values = %v, %v", vals, err)
	}

	raw, _ := os.ReadFile(cfg)
	for _, want := range []string{"# keep me", "# dev", "local:", "staging-eu:", "server: https://staging.example.com/v1"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("config missing %q:\n%s", want, raw)
		}
	}
	if strings.Contains(string(raw), "s3cr3t") {
		t.Error("secrets must not go to the config")
	}
	c, err := project.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := c.Select("staging-eu"); err != nil || got.Server != "https://staging.example.com/v1" {
		t.Errorf("select = %+v, %v", got, err)
	}

	// Never overwrite.
	if err := WriteEnvironment(spec, "staging-eu", e, &res); err == nil {
		t.Error("second write must fail (env file exists)")
	}
	_ = os.Remove(plainPath)
	_ = os.Remove(secretPath)
	if err := WriteEnvironment(spec, "staging-eu", e, &res); err == nil || !strings.Contains(err.Error(), "already defines") {
		t.Errorf("existing environment: %v", err)
	}
}

func TestAddEnvironmentCreatesConfig(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), project.FileName)
	created, err := project.AddEnvironment(cfg, "qa", project.Environment{Env: ".env.qa"})
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	c, err := project.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if e, err := c.Select("qa"); err != nil || e.Env == "" {
		t.Errorf("qa = %+v, %v", e, err)
	}
}

func TestParsePostmanWithEnvironmentHost(t *testing.T) {
	coll := `{"info":{"name":"c","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
"item":[{"name":"list","request":{"method":"GET","url":"{{baseUrl}}/users"}}]}`
	reqs, extras, err := ParsePostmanWith([]byte(coll), map[string]string{"baseUrl": "https://api.example.com"})
	if err != nil || len(reqs) != 1 {
		t.Fatalf("%v %v", reqs, err)
	}
	if reqs[0].URL != "https://api.example.com/users" {
		t.Errorf("url = %q", reqs[0].URL)
	}
	if len(extras.Variables) != 0 {
		t.Errorf("environment values must not leak into collection variables: %v", extras.Variables)
	}
}

func TestApply_TemplatedCredentialsBecomeConfigAuth(t *testing.T) {
	coll := `{"info":{"name":"c","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
"item":[
 {"name":"a","request":{"method":"GET","url":"https://api.example.com/a","auth":{"type":"basic","basic":[{"key":"username","value":"{{user}}"},{"key":"password","value":"{{pass}}"}]}}},
 {"name":"b","request":{"method":"GET","url":"https://api.example.com/b","header":[{"key":"X-API-Key","value":"{{apiKey}}"}]}},
 {"name":"c","request":{"method":"GET","url":"https://api.example.com/c","header":[{"key":"Authorization","value":"Bearer literal-secret"}]}}
]}`
	reqs, _, err := ParsePostman([]byte(coll))
	if err != nil {
		t.Fatal(err)
	}
	spec := NewSpec("t")
	res, err := Apply(spec, "", reqs)
	if err != nil {
		t.Fatal(err)
	}
	if a := res.Auth["basicAuth"]; a.Username != "{{env.user}}" || a.Password != "{{env.pass}}" {
		t.Errorf("basic = %+v", a)
	}
	if a := res.Auth["apiKeyAuth"]; a.Value != "{{env.apiKey}}" {
		t.Errorf("api key = %+v", a)
	}
	if _, ok := res.Auth["bearerAuth"]; ok {
		t.Error("a literal token must not be carried anywhere")
	}
	if !strings.Contains(strings.Join(res.Notes, "\n"), "bearer tokens were not copied") {
		t.Errorf("notes = %v", res.Notes)
	}
}
