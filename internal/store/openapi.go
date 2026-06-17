package store

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/model"
	"gopkg.in/yaml.v3"
)

func IsOpenAPIFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false
	}
	_, hasOpenAPI := root["openapi"]
	_, hasSwagger := root["swagger"]
	return hasOpenAPI || hasSwagger
}

func ParseSpec(path string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec %s: %w", path, err)
	}
	return doc, nil
}

func ListOperations(spec *openapi3.T, filePath string) []requests.RequestItem {
	var items []requests.RequestItem

	for _, pathKey := range spec.Paths.InMatchingOrder() {
		pathItem := spec.Paths.Find(pathKey)
		if pathItem == nil {
			continue
		}
		for method, op := range pathItem.Operations() {
			items = append(items, requests.RequestItem{
				Method: methodFromLabel(method),
				URI:    pathKey,
				About: model.About{
					Summary:     op.Summary,
					Description: op.Description,
				},
				FileName: filePath,
				OpenAPIRef: &model.OpenAPIRef{
					FilePath: filePath,
					Path:     pathKey,
					Method:   method,
				},
			})
		}
	}

	slices.SortStableFunc(items, func(a, b requests.RequestItem) int {
		if c := strings.Compare(strings.ToLower(a.URI), strings.ToLower(b.URI)); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.Method.Label()), strings.ToLower(b.Method.Label()))
	})

	return items
}

func methodFromLabel(label string) model.Method {
	switch label {
	case "GET":
		return model.GET
	case "POST":
		return model.POST
	case "PATCH":
		return model.PATCH
	case "PUT":
		return model.PUT
	case "DELETE":
		return model.DELETE
	case "OPTIONS":
		return model.OPTIONS
	case "HEAD":
		return model.HEAD
	default:
		return model.GET
	}
}

func OperationToRequest(spec *openapi3.T, ref model.OpenAPIRef) model.Request {
	pathItem := spec.Paths.Find(ref.Path)
	if pathItem == nil {
		return model.Request{}
	}

	op := pathItem.GetOperation(ref.Method)
	if op == nil {
		return model.Request{}
	}

	req := model.Request{
		FileName: ref.FilePath,
		URI:      ref.Path,
		Method:   methodFromLabel(ref.Method),
		About: model.About{
			Summary:     op.Summary,
			Description: op.Description,
		},
		Body: model.Body{
			Type: model.ApplicationJSON,
			Raw:  "",
		},
		Headers: map[string]string{},
		Params:  map[string]string{},
		Query:   map[string]string{},
		OpenAPIRef: &model.OpenAPIRef{
			FilePath: ref.FilePath,
			Path:     ref.Path,
			Method:   ref.Method,
		},
	}

	for _, paramRef := range op.Parameters {
		if paramRef.Value == nil || paramRef.Ref != "" {
			continue
		}
		p := paramRef.Value
		switch p.In {
		case "path":
			req.Params[p.Name] = ""
		case "query":
			req.Query[p.Name] = ""
		case "header":
			req.Headers[p.Name] = ""
		}
	}

	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for contentType := range op.RequestBody.Value.Content {
			req.Body.Type = model.MimeType(contentType)
			break
		}
	}

	for _, s := range spec.Servers {
		req.Servers = append(req.Servers, s.URL)
	}
	if len(spec.Servers) > 0 {
		req.ServerURL = spec.Servers[0].URL
	}

	return req
}

func ApplyRequestToOperation(spec *openapi3.T, ref model.OpenAPIRef, data model.Request) error {
	pathItem := spec.Paths.Find(ref.Path)
	if pathItem == nil {
		return fmt.Errorf("path %q not found in spec", ref.Path)
	}

	op := pathItem.GetOperation(ref.Method)
	if op == nil {
		return fmt.Errorf("operation %s %s not found", ref.Method, ref.Path)
	}

	op.Summary = data.About.Summary
	op.Description = data.About.Description

	if data.Body.Type != "" && op.RequestBody != nil && op.RequestBody.Value != nil {
		contentType := string(data.Body.Type)
		if _, exists := op.RequestBody.Value.Content[contentType]; !exists {
			op.RequestBody.Value.Content[contentType] = &openapi3.MediaType{}
		}
	}

	return nil
}

func RemoveOperationFromSpec(spec *openapi3.T, ref model.OpenAPIRef) error {
	pathItem := spec.Paths.Find(ref.Path)
	if pathItem == nil {
		return fmt.Errorf("path %q not found", ref.Path)
	}

	switch ref.Method {
	case "GET":
		pathItem.Get = nil
	case "POST":
		pathItem.Post = nil
	case "PUT":
		pathItem.Put = nil
	case "PATCH":
		pathItem.Patch = nil
	case "DELETE":
		pathItem.Delete = nil
	case "OPTIONS":
		pathItem.Options = nil
	case "HEAD":
		pathItem.Head = nil
	default:
		return fmt.Errorf("unknown method %q", ref.Method)
	}

	if pathItem.Get == nil && pathItem.Post == nil && pathItem.Put == nil &&
		pathItem.Patch == nil && pathItem.Delete == nil && pathItem.Options == nil &&
		pathItem.Head == nil {
		spec.Paths.Delete(ref.Path)
	}

	return nil
}

func AddOperationToSpec(spec *openapi3.T, path, method string, data model.Request) error {
	pathItem := spec.Paths.Find(path)
	if pathItem == nil {
		pathItem = &openapi3.PathItem{}
		spec.Paths.Set(path, pathItem)
	}

	op := &openapi3.Operation{}
	op.Summary = data.About.Summary
	op.Description = data.About.Description

	for name := range data.Params {
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				Name:     name,
				In:       "path",
				Required: true,
				Schema:   &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
			},
		})
	}
	for name := range data.Query {
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				Name:   name,
				In:     "query",
				Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
			},
		})
	}
	for name := range data.Headers {
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				Name: name,
				In:   "header",
				Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
			},
		})
	}

	if data.Body.Raw != "" {
		op.RequestBody = &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: openapi3.Content{
					string(data.Body.Type): &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &openapi3.Types{"object"}},
						},
					},
				},
			},
		}
	}

	op.Responses = openapi3.NewResponses()
	desc := "OK"
	op.Responses.Set("200", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: &desc,
		},
	})

	pathItem.SetOperation(method, op)
	return nil
}

func LoadServers(filePath string) ([]string, string) {
	spec, err := ParseSpec(filePath)
	if err != nil {
		return nil, ""
	}
	var servers []string
	for _, s := range spec.Servers {
		servers = append(servers, s.URL)
	}
	if len(servers) > 0 {
		return servers, servers[0]
	}
	return servers, ""
}

func SaveSpec(path string, spec *openapi3.T) error {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAPI spec: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
