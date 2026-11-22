package store

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestMatchPath(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromData([]byte(`openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /users/{id}:
    get: {responses: {"200": {description: ok}}}
  /users/me:
    get: {responses: {"200": {description: ok}}}
  /users/{id}/orders/{orderId}:
    delete: {responses: {"204": {description: ok}}}
`))
	if err != nil {
		t.Fatal(err)
	}
	ref, params, ok := MatchPath(spec, "get", "/users/42")
	if !ok || ref.Path != "/users/{id}" || ref.Method != "GET" || params["id"] != "42" {
		t.Errorf("users/42 → %+v %v %v", ref, params, ok)
	}
	if ref, _, _ := MatchPath(spec, "GET", "/users/me"); ref.Path != "/users/me" {
		t.Errorf("literal segment should win: %s", ref.Path)
	}
	ref, params, ok = MatchPath(spec, "DELETE", "/users/a%20b/orders/7/")
	if !ok || params["id"] != "a b" || params["orderId"] != "7" || ref.Path != "/users/{id}/orders/{orderId}" {
		t.Errorf("nested → %+v %v", ref, params)
	}
	if _, _, ok := MatchPath(spec, "POST", "/users/42"); ok {
		t.Error("method must match")
	}
	if _, _, ok := MatchPath(spec, "GET", "/users"); ok {
		t.Error("segment count must match")
	}
}
