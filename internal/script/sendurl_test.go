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
