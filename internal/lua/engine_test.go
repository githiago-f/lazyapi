package lua

import (
	"net/http"
	"testing"

	"github.com/githiago-f/lazyapi/internal/model"
)

func TestRunScript_BasicTests(t *testing.T) {
	data := ExecData{
		Request: model.Request{
			Method: model.POST,
			URI:    "/users",
			Body:   model.Body{Raw: `{"name":"test"}`},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Query:  map[string]string{"page": "1"},
			Params: map[string]string{"id": "123"},
		},
		Env: map[string]string{"BASE_URL": "https://api.example.com"},

		HasResponse:    true,
		RespStatus:     200,
		RespStatusText: "OK",
		RespHeaders:    http.Header{"Content-Type": []string{"application/json"}},
		RespBody:       `{"id":1,"name":"test"}`,
	}

	script := `
tests["Method is POST"] = request.method() == "POST"
tests["URI is /users"] = request.uri() == "/users"
tests["Status is 200"] = response.status() == 200
tests["Body has name"] = response.json().name == "test"
tests["URL contains /users"] = string.find(request.url(), "/users") ~= nil
tests["Env has BASE_URL"] = env.has("BASE_URL")
`

	result := RunScript(script, data)

	if result.ScriptError != "" {
		t.Fatalf("unexpected script error: %s", result.ScriptError)
	}

	if len(result.Results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(result.Results))
	}

	if result.PassCount != 6 {
		t.Errorf("expected 6 passed, got %d passed, %d failed", result.PassCount, result.FailCount)
	}

	if result.FailCount != 0 {
		t.Errorf("expected 0 failed, got %d", result.FailCount)
	}
}

func TestRunScript_FailingTests(t *testing.T) {
	data := ExecData{
		Request: model.Request{
			Method: model.GET,
			URI:    "/items",
		},
		HasResponse: true,
		RespStatus:  404,
		RespBody:    `{"error":"not found"}`,
	}

	script := `
tests["Status is 200"] = response.status() == 200
tests["Method is GET"] = request.method() == "GET"
`

	result := RunScript(script, data)

	if result.ScriptError != "" {
		t.Fatalf("unexpected script error: %s", result.ScriptError)
	}

	if result.PassCount != 1 {
		t.Errorf("expected 1 passed, got %d", result.PassCount)
	}

	if result.FailCount != 1 {
		t.Errorf("expected 1 failed, got %d", result.FailCount)
	}
}

func TestRunScript_ScriptError(t *testing.T) {
	data := ExecData{
		Request: model.Request{Method: model.GET, URI: "/test"},
	}

	script := `
tests["ok"] = true
error("something went wrong")
`

	result := RunScript(script, data)

	if result.ScriptError == "" {
		t.Fatal("expected script error, got none")
	}

	if result.PassCount != 1 {
		t.Errorf("expected 1 passed before error, got %d", result.PassCount)
	}
}

func TestRunScript_TestFunction(t *testing.T) {
	data := ExecData{
		Request: model.Request{
			Method: model.GET,
			URI:    "/test",
		},
		HasResponse: true,
		RespStatus:  200,
	}

	script := `
test("assert passes", function()
    assert(response.status() == 200)
end)

test("assert fails", function()
    assert(response.status() == 404, "expected 404")
end)
`

	result := RunScript(script, data)

	if result.ScriptError != "" {
		t.Fatalf("unexpected script error: %s", result.ScriptError)
	}

	if result.PassCount != 1 {
		t.Errorf("expected 1 passed, got %d", result.PassCount)
	}

	if result.FailCount != 1 {
		t.Errorf("expected 1 failed, got %d", result.FailCount)
	}
}

func TestRunScript_NoResponse(t *testing.T) {
	data := ExecData{
		Request: model.Request{
			Method: model.GET,
			URI:    "/test",
		},
		// HasResponse is false
	}

	script := `
tests["Status is 0"] = response.status() == 0
tests["Body is empty"] = response.body() == ""
`

	result := RunScript(script, data)

	if result.ScriptError != "" {
		t.Fatalf("unexpected script error: %s", result.ScriptError)
	}

	if result.PassCount != 2 {
		t.Errorf("expected 2 passed, got %d passed, %d failed", result.PassCount, result.FailCount)
	}
}

func TestRunScript_EnvSet(t *testing.T) {
	envMap := map[string]string{"existing": "value"}
	data := ExecData{
		Request: model.Request{Method: model.GET, URI: "/test"},
		Env:     envMap,
	}

	script := `
env.set("new_var", "new_value")
env.set("existing", "updated")

tests["new_var set"] = env.get("new_var") == "new_value"
tests["existing updated"] = env.get("existing") == "updated"
tests["new_var in vars"] = env.vars().new_var == "new_value"
`

	result := RunScript(script, data)

	if result.ScriptError != "" {
		t.Fatalf("unexpected script error: %s", result.ScriptError)
	}

	if result.PassCount != 3 {
		t.Errorf("expected 3 passed, got %d passed, %d failed", result.PassCount, result.FailCount)
	}
	_ = envMap
}

func TestRunScript_HeaderLookup(t *testing.T) {
	data := ExecData{
		Request: model.Request{
			Method: model.GET,
			URI:    "/test",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		HasResponse: true,
		RespHeaders: http.Header{
			"Content-Type": []string{"application/json; charset=utf-8"},
		},
		RespBody: "hello",
	}

	script := `
tests["Header exists"] = request.header("Content-Type") == "application/json"
tests["Case insensitive"] = request.header("content-type") == "application/json"
tests["Missing header is nil"] = (request.header("X-Custom") == nil)
tests["Resp header exists"] = response.header("Content-Type"):find("json") ~= nil
`

	result := RunScript(script, data)

	if result.ScriptError != "" {
		t.Fatalf("unexpected script error: %s", result.ScriptError)
	}

	if len(result.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(result.Results))
	}

	for _, r := range result.Results {
		t.Logf("  [%v] %s", r.Passed, r.Name)
	}

	if result.PassCount != 4 {
		t.Errorf("expected 4 passed, got %d passed, %d failed", result.PassCount, result.FailCount)
	}
}

func TestRunScript_JSONDecode(t *testing.T) {
	data := ExecData{
		Request: model.Request{Method: model.GET, URI: "/test"},
		HasResponse: true,
		RespBody:    `{"items":[1,2,3],"count":3}`,
	}

	script := `
local data = json_decode(response.body())
tests["has items"] = type(data.items) == "table"
tests["items count"] = #data.items == 3
tests["count is 3"] = data.count == 3
tests["json_encode roundtrip"] = json_encode(data) ~= nil
`

	result := RunScript(script, data)

	if result.ScriptError != "" {
		t.Fatalf("unexpected script error: %s", result.ScriptError)
	}

	if result.PassCount != 4 {
		t.Errorf("expected 4 passed, got %d passed, %d failed", result.PassCount, result.FailCount)
		for _, r := range result.Results {
			t.Logf("  [%v] %s", r.Passed, r.Name)
		}
	}
}

func TestFormatOutput_NoTests(t *testing.T) {
	data := ExecData{
		Request: model.Request{Method: model.GET, URI: "/test"},
	}

	result := RunScript("", data)

	if result.ScriptError != "" {
		t.Fatalf("unexpected script error: %s", result.ScriptError)
	}

	if len(result.Results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(result.Results))
	}

	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}
