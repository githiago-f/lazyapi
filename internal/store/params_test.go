package store

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

func paramNames(op *openapi3.Operation) []string {
	var out []string
	for _, p := range op.Parameters {
		out = append(out, p.Value.In+":"+p.Value.Name)
	}
	return out
}

func TestSyncParameters(t *testing.T) {
	op := openapi3.NewOperation()
	keep := &openapi3.Parameter{Name: "id", In: "path", Required: true, Description: "kept with its docs",
		Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}}}}
	op.AddParameter(keep)
	op.AddParameter(&openapi3.Parameter{Name: "old", In: "query"})
	op.AddParameter(&openapi3.Parameter{Name: "Authorization", In: "header"}) // author's choice
	op.AddParameter(&openapi3.Parameter{Name: "session", In: "cookie"})

	req := model.Request{
		Params:  map[string]string{"id": "1"},
		Query:   map[string]string{"zeta": "", "alpha": ""},
		Headers: map[string]string{"X-Trace": "1", "Content-Type": "application/json", "Accept": "*/*"},
	}
	for i := 0; i < 5; i++ { // stable across saves
		syncParameters(op, req)
	}
	got := paramNames(op)
	want := []string{"path:id", "header:Authorization", "cookie:session", "query:alpha", "query:zeta", "header:X-Trace"}
	if len(got) != len(want) {
		t.Fatalf("params = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("params = %v, want %v", got, want)
		}
	}
	if op.Parameters[0].Value.Description != "kept with its docs" {
		t.Error("existing definitions must be kept")
	}
}
