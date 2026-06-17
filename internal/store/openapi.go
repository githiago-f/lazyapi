package store

import (
	"fmt"
	"os"

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

func SaveSpec(path string, spec *openapi3.T) error {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAPI spec: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
