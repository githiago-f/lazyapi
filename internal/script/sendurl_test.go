package script

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func urlContext(t *testing.T) (*SpecContext, *[]string) {
	t.Helper()
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.RequestURI()+" "+r.Header.Get("X-Trace")+" "+r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"abc","id":42}`))
	}))
	t.Cleanup(srv.Close)
	spec, err := openapi3.NewLoader().LoadFromData([]byte(`openapi: 3.0.0
info: {title: t, version: "1"}
servers: [{url: "` + srv.URL + `/v1"}]
paths:
  /users/{id}:
    get:
      parameters: [{in: path, name: id, required: true, schema: {type: integer}}]
      responses: {"200": {description: ok}}
  /login:
    post:
      responses: {"200": {description: ok}}
`))
	if err != nil {
		t.Fatal(err)
	}
	return &SpecContext{Spec: spec, SpecPath: "spec.yml", Env: map[string]string{"base": srv.URL + "/v1"}}, &hits
}

func TestSend_URLTargets(t *testing.T) {
	c, hits := urlContext(t)
	resp, err := c.Send("GET {{env.base}}/users/42?verbose=1", SendOptions{Headers: map[string]string{"X-Trace": "t1"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Ref == nil || resp.Ref.Path != "/users/{id}" || resp.Status != 200 {
		t.Errorf("resp = %+v", resp)
	}
	if got := (*hits)[0]; got != "GET /v1/users/42?verbose=1 t1 " {
		t.Errorf("hit = %q", got)
	}

	_, err = c.Send("GET https://elsewhere.example.com/users/42", SendOptions{})
	if err == nil || !strings.Contains(err.Error(), "is not an operation of the spec") {
		t.Errorf("foreign host: %v", err)
	}
	if _, err := c.Send("DELETE {{env.base}}/users/42", SendOptions{}); err == nil {
		t.Error("undeclared method must fail")
	}
}

func TestPostmanSendRequest(t *testing.T) {
	c, hits := urlContext(t)
	data := baseData()
	data.Env = c.Env
	c.Bind(&data)
	src := `
var seen = null;
pm.sendRequest({
  url: "{{base}}/login",
  method: "POST",
  header: [{ key: "X-Trace", value: "t2" }],
  body: { mode: "urlencoded", urlencoded: [{ key: "user", value: "ana" }] },
}, function (err, res) {
  seen = err ? String(err) : res.json().token;
  pm.environment.set("token", res.json().token);
});
pm.test("callback ran", function () { pm.expect(seen).to.equal("abc"); });

var direct = pm.sendRequest(pm.environment.get("base") + "/users/7");
pm.test("returns the response", function () { pm.expect(direct.code).to.equal(200); direct.to.have.status(200); });

pm.sendRequest("https://elsewhere.example.com/x", function (err, res) {
  pm.test("errors go to the callback", function () { pm.expect(String(err)).to.include("not an operation of the spec"); pm.expect(res).to.equal(null); });
});
`
	res := Run(src, data, Options{})
	mustPass(t, res)
	if res.PassCount != 3 {
		t.Errorf("tests = %+v", res.Results)
	}
	if data.Env["token"] != "abc" {
		t.Errorf("env token = %q", data.Env["token"])
	}
	if (*hits)[0] != "POST /v1/login t2 application/x-www-form-urlencoded" || !strings.HasPrefix((*hits)[1], "GET /v1/users/7") {
		t.Errorf("hits = %q", *hits)
	}
}
