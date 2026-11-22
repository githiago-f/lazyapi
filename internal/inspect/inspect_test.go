package inspect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/store"
)

const refSpec = `openapi: 3.0.0
info: {title: t, version: "1"}
servers: [{url: "https://api.example.com"}]
paths:
  /nodes/{id}:
    parameters:
      - {in: path, name: id, required: true, schema: {type: string}, example: n1}
    get:
      operationId: getNode
      tags: [tree]
      security: [{bearer: []}]
      parameters:
        - {in: query, name: depth, schema: {type: integer, default: 1}}
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: {$ref: "#/components/schemas/Node"}
        "404":
          description: Missing
  /nodes:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: "#/components/schemas/NewNode"}
            example: {name: root}
      responses:
        "201": {description: Created}
components:
  securitySchemes:
    bearer: {type: http, scheme: bearer}
  schemas:
    NewNode:
      type: object
      properties: {name: {type: string}}
    Node:
      allOf:
        - $ref: "#/components/schemas/NewNode"
        - type: object
          properties:
            children:
              type: array
              items: {$ref: "#/components/schemas/Node"}
`

func writeRefSpec(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "api.yml")
	if err := os.WriteFile(p, []byte(refSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDescribe_ResolvesReferencedSchemas(t *testing.T) {
	p := writeRefSpec(t)
	spec, err := store.ParseSpec(p)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Describe(spec, p, "getNode")
	if err != nil {
		t.Fatal(err)
	}
	if d.Method != "GET" || d.Path != "/nodes/{id}" || d.Servers[0] != "https://api.example.com" {
		t.Errorf("detail = %+v", d.OpSummary)
	}
	if len(d.Parameters) != 2 || d.Parameters[0].Name != "id" || d.Parameters[0].Example != "n1" || d.Parameters[1].Example != "1" {
		t.Errorf("params = %+v", d.Parameters)
	}
	if len(d.Responses) != 2 || d.Responses[0].Status != "200" || !strings.Contains(string(d.Responses[0].Content[0].Schema), "#/components/schemas/Node") {
		t.Errorf("responses = %+v", d.Responses)
	}
	if _, ok := d.Schemas["Node"]; !ok {
		t.Errorf("Node schema missing: %v", d.Schemas)
	}
	if _, ok := d.Schemas["NewNode"]; !ok {
		t.Errorf("nested NewNode schema missing: %v", d.Schemas)
	}
	if len(d.Security) != 1 || d.Security[0] != "bearer" {
		t.Errorf("security = %v", d.Security)
	}
	if _, err := json.Marshal(d); err != nil {
		t.Fatalf("detail must be JSON-serializable: %v", err)
	}

	post, err := Describe(spec, p, "POST /nodes")
	if err != nil {
		t.Fatal(err)
	}
	if post.RequestBody == nil || !post.RequestBody.Required || string(post.RequestBody.Content[0].Example) != `{"name":"root"}` {
		t.Errorf("request body = %+v", post.RequestBody)
	}

	if _, err := Describe(spec, p, "nope"); err == nil {
		t.Error("unknown operation should fail")
	}
}

func TestListOperationsAndValidate(t *testing.T) {
	p := writeRefSpec(t)
	spec, err := store.ParseSpec(p)
	if err != nil {
		t.Fatal(err)
	}
	if ops := ListOperations(spec, p, []string{"tree"}); len(ops) != 1 || ops[0].OperationID != "getNode" {
		t.Errorf("tag filter = %+v", ops)
	}
	if ops := ListOperations(spec, p, nil); len(ops) != 2 {
		t.Errorf("all = %+v", ops)
	}

	v := Validate(p)
	if !v.Valid {
		t.Errorf("spec should be valid: %+v", v.Issues)
	}
	found := false
	for _, i := range v.Issues {
		if i.Operation == "POST /nodes" && strings.Contains(i.Message, "missing operationId") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing operationId warning: %+v", v.Issues)
	}

	broken := filepath.Join(t.TempDir(), "broken.yml")
	if err := os.WriteFile(broken, []byte("openapi: 3.0.0\ninfo: {title: t}\npaths: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := Validate(broken); v.Valid {
		t.Errorf("missing info.version should be invalid: %+v", v)
	}
}
