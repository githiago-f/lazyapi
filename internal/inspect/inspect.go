// Package inspect describes OpenAPI specs for agents: operation listings,
// full operation details with referenced schemas, and spec validation.
package inspect

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/script"
	"github.com/githiago-f/lazyapi/internal/store"
)

// OpSummary is one line of `list ops`.
type OpSummary struct {
	OperationID string   `json:"operationId,omitempty"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Summary     string   `json:"summary,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	TestsFile   string   `json:"testsFile,omitempty"`
}

// ListOperations returns every operation, optionally limited to some tags.
func ListOperations(spec *openapi3.T, specPath string, tags []string) []OpSummary {
	var out []OpSummary
	for _, item := range store.ListOperations(spec, specPath) {
		ref := model.OpenAPIRef{FilePath: specPath, Path: item.URI, Method: item.Method.Label()}
		op := store.FindOperation(spec, ref)
		if op == nil {
			continue
		}
		if len(tags) > 0 && !slices.ContainsFunc(op.Tags, func(t string) bool { return slices.Contains(tags, t) }) {
			continue
		}
		s := OpSummary{OperationID: op.OperationID, Method: ref.Method, Path: ref.Path, Summary: op.Summary, Tags: op.Tags}
		if v, ok := op.Extensions[store.TestsExtension].(string); ok {
			s.TestsFile = v
		}
		out = append(out, s)
	}
	return out
}

// Param describes a parameter.
type Param struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Required    bool            `json:"required"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Example     string          `json:"example,omitempty"`
}

// Content is a media type with its schema and example.
type Content struct {
	MediaType string          `json:"mediaType"`
	Schema    json.RawMessage `json:"schema,omitempty"`
	Example   json.RawMessage `json:"example,omitempty"`
}

// Body is a request body.
type Body struct {
	Required bool      `json:"required"`
	Content  []Content `json:"content"`
}

// Response is a declared response.
type Response struct {
	Status      string    `json:"status"`
	Description string    `json:"description,omitempty"`
	Content     []Content `json:"content,omitempty"`
}

// OpDetail is the output of `describe`.
type OpDetail struct {
	OpSummary
	Description string                     `json:"description,omitempty"`
	Servers     []string                   `json:"servers,omitempty"`
	Parameters  []Param                    `json:"parameters,omitempty"`
	RequestBody *Body                      `json:"requestBody,omitempty"`
	Responses   []Response                 `json:"responses"`
	Security    []string                   `json:"security,omitempty"`
	TestsSource string                     `json:"testsSource,omitempty"`
	Schemas     map[string]json.RawMessage `json:"schemas,omitempty"` // components referenced above
}

// Describe returns everything an agent needs to call and test an operation.
// target is an operationId or "METHOD /path".
func Describe(spec *openapi3.T, specPath, target string) (OpDetail, error) {
	ref, err := script.ResolveTarget(spec, specPath, target)
	if err != nil {
		return OpDetail{}, err
	}
	op := store.FindOperation(spec, ref)
	pathItem := spec.Paths.Find(ref.Path)

	d := OpDetail{
		OpSummary:   OpSummary{OperationID: op.OperationID, Method: ref.Method, Path: ref.Path, Summary: op.Summary, Tags: op.Tags},
		Description: op.Description,
	}
	if v, ok := op.Extensions[store.TestsExtension].(string); ok {
		d.TestsFile = v
		d.TestsSource, _ = store.LoadOperationTests(specPath, op)
	}
	for _, s := range spec.Servers {
		d.Servers = append(d.Servers, s.URL)
	}

	refs := map[string]bool{}
	params := append(openapi3.Parameters{}, pathItem.Parameters...)
	params = append(params, op.Parameters...)
	for _, pr := range params {
		if pr == nil || pr.Value == nil {
			continue
		}
		p := pr.Value
		d.Parameters = append(d.Parameters, Param{
			Name: p.Name, In: p.In, Required: p.Required, Description: p.Description,
			Schema: marshalSchema(p.Schema, refs), Example: store.ParamExample(p),
		})
	}

	if op.RequestBody != nil && op.RequestBody.Value != nil {
		d.RequestBody = &Body{Required: op.RequestBody.Value.Required, Content: contents(op.RequestBody.Value.Content, refs)}
	}

	if op.Responses != nil {
		codes := make([]string, 0, op.Responses.Len())
		for code := range op.Responses.Map() {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			r := op.Responses.Value(code)
			if r == nil || r.Value == nil {
				continue
			}
			resp := Response{Status: code, Content: contents(r.Value.Content, refs)}
			if r.Value.Description != nil {
				resp.Description = *r.Value.Description
			}
			d.Responses = append(d.Responses, resp)
		}
	}

	security := op.Security
	if security == nil {
		security = &spec.Security
	}
	for _, req := range *security {
		for name := range req {
			d.Security = append(d.Security, name)
		}
	}
	sort.Strings(d.Security)

	d.Schemas = resolveRefs(spec, refs)
	return d, nil
}

func contents(c openapi3.Content, refs map[string]bool) []Content {
	types := make([]string, 0, len(c))
	for mt := range c {
		types = append(types, mt)
	}
	sort.Strings(types)
	var out []Content
	for _, mt := range types {
		m := c[mt]
		if m == nil {
			continue
		}
		item := Content{MediaType: mt, Schema: marshalSchema(m.Schema, refs)}
		if m.Example != nil {
			item.Example, _ = json.Marshal(m.Example)
		}
		out = append(out, item)
	}
	return out
}

// marshalSchema keeps $ref pointers (so recursive schemas stay finite) and
// records the component names they point to.
func marshalSchema(s *openapi3.SchemaRef, refs map[string]bool) json.RawMessage {
	if s == nil {
		return nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	collectRefs(data, refs)
	return data
}

const schemaPrefix = "#/components/schemas/"

func collectRefs(data []byte, refs map[string]bool) {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return
	}
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if r, ok := t["$ref"].(string); ok && strings.HasPrefix(r, schemaPrefix) {
				refs[strings.TrimPrefix(r, schemaPrefix)] = true
			}
			for _, child := range t {
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	walk(v)
}

// resolveRefs returns referenced component schemas, following nested refs.
func resolveRefs(spec *openapi3.T, refs map[string]bool) map[string]json.RawMessage {
	if len(refs) == 0 || spec.Components == nil {
		return nil
	}
	out := map[string]json.RawMessage{}
	for {
		var pending []string
		for name := range refs {
			if _, done := out[name]; !done {
				pending = append(pending, name)
			}
		}
		if len(pending) == 0 {
			return out
		}
		for _, name := range pending {
			s := spec.Components.Schemas[name]
			if s == nil || s.Value == nil {
				out[name] = json.RawMessage(`null`)
				continue
			}
			data, err := json.Marshal(s.Value)
			if err != nil {
				out[name] = json.RawMessage(`null`)
				continue
			}
			collectRefs(data, refs)
			out[name] = data
		}
	}
}

// Issue is one validation finding.
type Issue struct {
	Severity  string `json:"severity"` // "error" or "warning"
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message"`
}

// Validation is the output of `validate spec`.
type Validation struct {
	Spec   string  `json:"spec"`
	Valid  bool    `json:"valid"`
	Issues []Issue `json:"issues"`
}

// Validate checks the spec against OpenAPI and LazyAPI's own conventions:
// test script references must exist and compile, operationIds should be set
// and unique, and responses should be declared. URL imports are only
// resolved from the module cache (no network access).
func Validate(specPath string) Validation {
	v := Validation{Spec: specPath, Issues: []Issue{}}
	add := func(sev, op, msg string) {
		v.Issues = append(v.Issues, Issue{Severity: sev, Operation: op, Message: msg})
	}

	spec, err := store.ParseSpec(specPath)
	if err != nil {
		add("error", "", err.Error())
		return v
	}
	if err := spec.Validate(context.Background()); err != nil {
		add("error", "", "OpenAPI: "+err.Error())
	}
	if len(spec.Servers) == 0 {
		add("warning", "", "no servers declared; pass --server when running")
	}

	specDir, _ := filepath.Abs(filepath.Dir(specPath))
	modules := script.NewModuleLoader("")
	modules.Offline = true

	seen := map[string]string{}
	for _, s := range ListOperations(spec, specPath, nil) {
		label := s.Method + " " + s.Path
		ref := model.OpenAPIRef{FilePath: specPath, Path: s.Path, Method: s.Method}
		op := store.FindOperation(spec, ref)

		switch {
		case s.OperationID == "":
			add("warning", label, "missing operationId (agents and lazyapi.send() address operations by it)")
		case seen[s.OperationID] != "":
			add("error", label, fmt.Sprintf("duplicate operationId %q (also used by %s)", s.OperationID, seen[s.OperationID]))
		default:
			seen[s.OperationID] = label
		}
		if op.Responses == nil || op.Responses.Len() == 0 {
			add("warning", label, "no responses declared; any status below 400 will be accepted")
		}

		if s.TestsFile == "" {
			continue
		}
		source, err := store.LoadOperationTests(specPath, op)
		if err != nil {
			add("error", label, err.Error())
			continue
		}
		opts, err := script.OperationOptions(specPath, op, ref)
		if err != nil {
			add("error", label, err.Error())
			continue
		}
		opts.Modules, opts.Root = modules, specDir
		if _, err := script.Bundle(source, opts); err != nil {
			msg := "test script does not compile: " + err.Error()
			if strings.Contains(err.Error(), "--offline") {
				msg = "test script imports a URL that is not cached yet (validate never downloads); run the tests once to fetch it: " + err.Error()
			}
			add("error", label, msg)
		}
	}

	v.Valid = !slices.ContainsFunc(v.Issues, func(i Issue) bool { return i.Severity == "error" })
	return v
}
