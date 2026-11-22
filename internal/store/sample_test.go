package store

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSchemaSampleValidates(t *testing.T) {
	min5 := 5.0
	node := &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"object"}}}
	node.Value.Properties = openapi3.Schemas{
		"name":     {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, MinLength: 3}},
		"children": {Value: &openapi3.Schema{Type: &openapi3.Types{"array"}, Items: node}}, // recursive
	}
	schema := &openapi3.SchemaRef{Value: &openapi3.Schema{
		Type:     &openapi3.Types{"object"},
		Required: []string{"id", "email", "status", "tree"},
		Properties: openapi3.Schemas{
			"id":      {Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}, Min: &min5}},
			"email":   {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "email"}},
			"status":  {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Enum: []any{"active", "blocked"}}},
			"score":   {Value: &openapi3.Schema{Type: &openapi3.Types{"number"}, Example: 9.5}},
			"tags":    {Value: &openapi3.Schema{Type: &openapi3.Types{"array"}, Items: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}}}},
			"tree":    node,
			"created": {Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "date-time"}},
		},
	}}

	v := SchemaSample(schema)
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("sample = %#v", v)
	}
	if obj["id"] != int64(5) || obj["email"] != "user@example.com" || obj["status"] != "active" || obj["score"] != 9.5 {
		t.Errorf("sample = %#v", obj)
	}
	// The sample must satisfy the schema it came from (round-trip through JSON types).
	if err := schema.Value.VisitJSON(toJSONTypes(v)); err != nil {
		t.Errorf("sample does not validate: %v", err)
	}
	if SchemaSample(nil) != nil {
		t.Error("nil schema")
	}
}

// toJSONTypes converts int64 to float64 like encoding/json would.
func toJSONTypes(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range t {
			out[k] = toJSONTypes(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = toJSONTypes(x)
		}
		return out
	case int64:
		return float64(t)
	}
	return v
}
