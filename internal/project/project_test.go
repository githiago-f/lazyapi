package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cfg = `default: local
environments:
  local:
    server: http://127.0.0.1:4010
    env: .env.local
    vars: {tenant: acme}
    auth:
      bearerAuth: {token: "{{env.TOKEN}}"}
  staging:
    server: "1"
    env: /abs/.env.staging
`

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFindLoadSelect(t *testing.T) {
	dir := t.TempDir()
	spec := write(t, dir, "api.yml", "openapi: 3.0.0")
	write(t, dir, FileName, cfg)

	c, err := Find(spec)
	if err != nil || c == nil {
		t.Fatalf("Find = %v, %v", c, err)
	}
	if got := strings.Join(c.Names(), ","); got != "local,staging" {
		t.Errorf("Names = %s", got)
	}

	e, err := c.Select("")
	if err != nil || e.Name != "local" || e.Server != "http://127.0.0.1:4010" || e.Vars["tenant"] != "acme" {
		t.Fatalf("default = %+v, %v", e, err)
	}
	if e.Env != filepath.Join(dir, ".env.local") {
		t.Errorf("env path should be relative to the config: %s", e.Env)
	}
	if e.Auth["bearerAuth"].Token != "{{env.TOKEN}}" {
		t.Errorf("auth = %+v", e.Auth)
	}

	s, _ := c.Select("staging")
	if s.Env != "/abs/.env.staging" || s.Server != "1" {
		t.Errorf("staging = %+v", s)
	}
	if _, err := c.Select("prod"); err == nil || !strings.Contains(err.Error(), "local, staging") {
		t.Errorf("unknown env error = %v", err)
	}

	t.Setenv("LAZYAPI_ENV", "staging")
	if e, _ := c.Select(""); e.Name != "staging" {
		t.Errorf("LAZYAPI_ENV should select staging, got %s", e.Name)
	}
}

func TestNoConfig(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	c, err := Find(filepath.Join(dir, "api.yml"))
	if c != nil || err != nil {
		t.Fatalf("Find = %v, %v", c, err)
	}
	if e, err := c.Select(""); e != nil || err != nil {
		t.Errorf("nil config, no name: %v %v", e, err)
	}
	if _, err := c.Select("staging"); err == nil {
		t.Error("asking for an env without a config should fail")
	}
}

func TestBadDefault(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, FileName, "default: nope\nenvironments: {local: {}}\n")
	if _, err := Load(p); err == nil {
		t.Error("undefined default should fail")
	}
}
