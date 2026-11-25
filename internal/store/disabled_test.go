package store

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

func TestApplyRequestKeepsDisabledParamDefinitions(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromData([]byte(`openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /pets:
    get:
      parameters:
        - {in: query, name: limit, schema: {type: integer}}
        - {in: header, name: X-Trace, schema: {type: string}}
      responses: {"200": {description: ok}}
`))
	if err != nil {
		t.Fatal(err)
	}
	ref := model.OpenAPIRef{Path: "/pets", Method: "GET"}
	data := model.Request{
		Query:    map[string]string{"offset": "0"},
		Disabled: model.Disabled{Query: map[string]string{"limit": "5"}, Headers: map[string]string{"X-Trace": "1"}},
	}
	if err := ApplyRequestToOperation(spec, ref, data); err != nil {
		t.Fatal(err)
	}
	op := spec.Paths.Find("/pets").Get
	for _, p := range [][2]string{{"query", "limit"}, {"query", "offset"}, {"header", "X-Trace"}} {
		if op.Parameters.GetByInAndName(p[0], p[1]) == nil {
			t.Errorf("%s %s must be kept", p[0], p[1])
		}
	}
	if len(op.Parameters) != 3 {
		t.Errorf("parameters: %d", len(op.Parameters))
	}
}
