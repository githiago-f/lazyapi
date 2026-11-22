package store

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

func TestParamExample(t *testing.T) {
	schema := func(s *openapi3.Schema) *openapi3.SchemaRef { return &openapi3.SchemaRef{Value: s} }
	cases := []struct {
		name string
		p    *openapi3.Parameter
		want string
	}{
		{"nil", nil, ""},
		{"none", &openapi3.Parameter{Name: "id"}, ""},
		{"example", &openapi3.Parameter{Example: "abc"}, "abc"},
		{"numeric example", &openapi3.Parameter{Example: float64(42)}, "42"},
		{"named examples sorted", &openapi3.Parameter{Examples: openapi3.Examples{
			"b": {Value: &openapi3.Example{Value: "second"}},
			"a": {Value: &openapi3.Example{Value: "first"}},
		}}, "first"},
		{"schema example", &openapi3.Parameter{Schema: schema(&openapi3.Schema{Example: "s"})}, "s"},
		{"schema default", &openapi3.Parameter{Schema: schema(&openapi3.Schema{Default: true})}, "true"},
		{"schema enum", &openapi3.Parameter{Schema: schema(&openapi3.Schema{Enum: []any{"x", "y"}})}, "x"},
	}
	for _, c := range cases {
		if got := ParamExample(c.p); got != c.want {
			t.Errorf("%s: ParamExample = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestStatusDeclared(t *testing.T) {
	cases := []struct {
		declared []string
		code     int
		want     bool
	}{
		{nil, 200, true},
		{nil, 404, false},
		{[]string{"200"}, 200, true},
		{[]string{"200"}, 201, false},
		{[]string{"200", "404"}, 404, true},
		{[]string{"2XX"}, 204, true},
		{[]string{"2xx"}, 204, true},
		{[]string{"default"}, 500, true},
	}
	for _, c := range cases {
		if got := StatusDeclared(c.declared, c.code); got != c.want {
			t.Errorf("StatusDeclared(%v, %d) = %v, want %v", c.declared, c.code, got, c.want)
		}
	}
}

func TestDeclaredStatuses(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatal(err)
	}
	got := DeclaredStatuses(spec, model.OpenAPIRef{Path: "/items/{id}", Method: "delete"})
	if len(got) != 1 || got[0] != "204" {
		t.Errorf("DeclaredStatuses = %v, want [204]", got)
	}
	if DeclaredStatuses(spec, model.OpenAPIRef{Path: "/nope", Method: "GET"}) != nil {
		t.Error("expected nil for unknown operation")
	}
}
