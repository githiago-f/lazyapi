package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

const sampleSpec = `openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /pets:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name: {type: string}
                age: {type: integer, minimum: 1}
                email: {type: string, format: email}
      responses:
        "201": {description: ok}
    put:
      requestBody:
        content:
          application/json:
            example: {name: rex}
      responses:
        "200": {description: ok}
`

func TestSampleBody(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromData([]byte(sampleSpec))
	if err != nil {
		t.Fatal(err)
	}
	post := model.OpenAPIRef{Path: "/pets", Method: "POST"}

	if _, raw := BodyExample(FindOperation(spec, post)); raw != "" {
		t.Fatalf("BodyExample must not invent a body, got %q", raw)
	}
	mt, raw := SampleBody(FindOperation(spec, post))
	if mt != "application/json" {
		t.Fatalf("media type %q", mt)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("sample is not JSON: %v\n%s", err, raw)
	}
	if _, ok := body["name"].(string); !ok {
		t.Fatalf("missing name in %s", raw)
	}

	req := model.Request{}
	FillSampleBody(spec, &req, model.OpenAPIRef{Path: "/pets", Method: "PUT"})
	if req.Body.Raw == "" || !json.Valid([]byte(req.Body.Raw)) {
		t.Fatalf("declared example not used: %q", req.Body.Raw)
	}
	req.Body.Raw = "kept"
	FillSampleBody(spec, &req, post)
	if req.Body.Raw != "kept" {
		t.Fatal("FillSampleBody overwrote an existing body")
	}
}

func TestResponseExample(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromData([]byte(`openapi: 3.0.0
info: {title: t, version: "1"}
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              example: {id: 1}
        "4XX":
          description: bad
          content:
            application/json:
              examples:
                b: {value: {error: nope}}
`))
	if err != nil {
		t.Fatal(err)
	}
	op := FindOperation(spec, model.OpenAPIRef{Path: "/pets", Method: "GET"})
	if _, body, ok := ResponseExample(op, 200); !ok || body != "{\n  \"id\": 1\n}" {
		t.Errorf("200 example %q", body)
	}
	if _, body, ok := ResponseExample(op, 404); !ok || !strings.Contains(body, "nope") {
		t.Errorf("4XX example %q", body)
	}
	if _, _, ok := ResponseExample(op, 500); ok {
		t.Error("no example for 500")
	}
}
