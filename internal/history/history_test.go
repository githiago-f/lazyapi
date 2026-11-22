package history

import (
	"os"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/runner"
)

func sample() (model.Request, runner.OpResult) {
	req := model.Request{
		FileName:   "api/spec.yml",
		Method:     model.POST,
		URI:        "/login",
		Headers:    map[string]string{"Authorization": "Bearer s3cret", "X-Api-Key": "{{env.KEY}}", "Accept": "application/json"},
		Query:      map[string]string{"api_key": "k1", "page": "2"},
		Body:       model.Body{Type: model.ApplicationJSON, Raw: `{"password":"{{env.PASS}}"}`},
		Env:        map[string]string{"PASS": "hunter2", "KEY": "abc"},
		Auth:       []model.AuthScheme{{Type: model.AuthBearer, Token: "tok"}},
		OpenAPIRef: &model.OpenAPIRef{FilePath: "api/spec.yml", Path: "/login", Method: "POST"},
	}
	res := runner.OpResult{
		Method: "POST", Path: "/login", Outcome: runner.Passed,
		Request:  &runner.RequestInfo{Method: "POST", URL: "http://u:p@h/login?api_key=k1&page=2", Body: `{"password":"hunter2"}`},
		Response: &runner.ResponseInfo{Status: 200, Body: `{"token":"t"}`, Headers: map[string]string{"Set-Cookie": "sid=1"}},
	}
	return req, res
}

func TestAppendLoadRedacts(t *testing.T) {
	t.Setenv("LAZYAPI_HISTORY_DIR", t.TempDir())
	req, res := sample()
	if err := Append(New(req, res, "local")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(File(req.FileName))
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	for _, secret := range []string{"hunter2", "s3cret", "k1", "sid=1", "u:p@", `"tok"`, "abc"} {
		if strings.Contains(raw, secret) {
			t.Errorf("history file contains %q:\n%s", secret, raw)
		}
	}
	if !strings.Contains(raw, "{{env.PASS}}") || !strings.Contains(raw, "{{env.KEY}}") {
		t.Errorf("templates must be kept:\n%s", raw)
	}
	if fi, _ := os.Stat(File(req.FileName)); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}

	entries, err := Load(req.FileName)
	if err != nil || len(entries) != 1 || entries[0].Env != "local" || entries[0].Result.Response.Status != 200 {
		t.Fatalf("load = %+v, %v", entries, err)
	}

	// Replay keeps the base's credentials for redacted values.
	base := model.Request{Headers: map[string]string{"Authorization": "Bearer {{env.TOKEN}}"}}
	got := entries[0].Replay(base)
	if got.Method != model.POST || got.URI != "/login" || got.Headers["Authorization"] != "Bearer {{env.TOKEN}}" ||
		got.Headers["Accept"] != "application/json" || got.Query["api_key"] != "" || got.Query["page"] != "2" ||
		got.Body.Raw != `{"password":"{{env.PASS}}"}` || got.OpenAPIRef == nil {
		t.Errorf("replay = %+v", got)
	}
}

func TestAppendKeepsNewest(t *testing.T) {
	t.Setenv("LAZYAPI_HISTORY_DIR", t.TempDir())
	req, res := sample()
	for i := 0; i < Keep+Keep/4+5; i++ {
		res.Response.Status = 200 + i%3
		if err := Append(New(req, res, "")); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := Load(req.FileName)
	if len(entries) > Keep+Keep/4 || len(entries) < Keep {
		t.Errorf("kept %d entries", len(entries))
	}
	if err := Clear(req.FileName); err != nil {
		t.Fatal(err)
	}
	if entries, _ := Load(req.FileName); len(entries) != 0 {
		t.Error("clear")
	}
}

func TestDisabled(t *testing.T) {
	t.Setenv("LAZYAPI_NO_HISTORY", "1")
	req, res := sample()
	if err := Append(New(req, res, "")); err != nil || File("x") != "" {
		t.Error("history must be disabled")
	}
}
