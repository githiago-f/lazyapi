package store

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI spec %s: %w", path, err)
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(data)
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
				Method: model.MethodFromLabel(method),
				URI:    pathKey,
				About: model.About{
					Summary:     op.Summary,
					Description: op.Description,
				},
				Tags:     op.Tags,
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
		Method:   model.MethodFromLabel(ref.Method),
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

	extractParams := func(params []*openapi3.ParameterRef) {
		for _, paramRef := range params {
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
	}

	extractParams(op.Parameters)
	extractParams(pathItem.Parameters)

	for _, segment := range strings.Split(req.URI, "/") {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := segment[1 : len(segment)-1]
			if _, ok := req.Params[name]; !ok {
				req.Params[name] = ""
			}
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

	if opSec := ExtractOperationSecurity(op, spec); len(opSec) > 0 {
		req.Auth = opSec
	} else {
		req.Auth = ExtractGlobalSecurity(spec)
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

	existing := make(map[string]*openapi3.Parameter)
	for _, pref := range op.Parameters {
		if pref.Value != nil {
			key := pref.Value.In + ":" + pref.Value.Name
			existing[key] = pref.Value
		}
	}

	keep := make(map[string]bool)

	for name := range data.Params {
		key := "path:" + name
		keep[key] = true
		if _, ok := existing[key]; !ok {
			op.Parameters = append(op.Parameters, makeParamRef(name, "path", true))
		}
	}

	for name := range data.Query {
		key := "query:" + name
		keep[key] = true
		if _, ok := existing[key]; !ok {
			op.Parameters = append(op.Parameters, makeParamRef(name, "query", false))
		}
	}

	for name := range data.Headers {
		key := "header:" + name
		keep[key] = true
		if _, ok := existing[key]; !ok {
			op.Parameters = append(op.Parameters, makeParamRef(name, "header", false))
		}
	}

	filtered := make([]*openapi3.ParameterRef, 0, len(op.Parameters))
	for _, pref := range op.Parameters {
		if pref.Value != nil {
			key := pref.Value.In + ":" + pref.Value.Name
			if keep[key] {
				filtered = append(filtered, pref)
			}
		}
	}
	op.Parameters = filtered

	if data.Body.Type != "" {
		if op.RequestBody == nil || op.RequestBody.Value == nil {
			op.RequestBody = &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{},
				},
			}
		}
		contentType := string(data.Body.Type)
		if _, exists := op.RequestBody.Value.Content[contentType]; !exists {
			op.RequestBody.Value.Content[contentType] = &openapi3.MediaType{}
		}
	}

	return applyAuthSchemes(spec, ref, data.Auth)
}

func RemoveOperationFromSpec(spec *openapi3.T, ref model.OpenAPIRef) error {
	pathItem := spec.Paths.Find(ref.Path)
	if pathItem == nil {
		return fmt.Errorf("path %q not found", ref.Path)
	}

	pathItem.SetOperation(ref.Method, nil)

	if pathItem.Operations() == nil || len(pathItem.Operations()) == 0 {
		spec.Paths.Delete(ref.Path)
	}

	return nil
}

func UpdateOperationTags(ref model.OpenAPIRef, tags []string) error {
	spec, err := ParseSpec(ref.FilePath)
	if err != nil {
		return fmt.Errorf("failed to parse spec: %w", err)
	}

	pathItem := spec.Paths.Find(ref.Path)
	if pathItem == nil {
		return fmt.Errorf("path %q not found", ref.Path)
	}

	op := pathItem.GetOperation(ref.Method)
	if op == nil {
		return fmt.Errorf("operation %s %s not found", ref.Method, ref.Path)
	}

	op.Tags = tags
	return SaveSpec(ref.FilePath, spec)
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
		op.Parameters = append(op.Parameters, makeParamRef(name, "path", true))
	}
	for name := range data.Query {
		op.Parameters = append(op.Parameters, makeParamRef(name, "query", false))
	}
	for name := range data.Headers {
		op.Parameters = append(op.Parameters, makeParamRef(name, "header", false))
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

	ref := model.OpenAPIRef{Path: path, Method: method}
	return applyAuthSchemes(spec, ref, data.Auth)
}

func makeParamRef(name, in string, required bool) *openapi3.ParameterRef {
	return &openapi3.ParameterRef{
		Value: &openapi3.Parameter{
			Name:     name,
			In:       in,
			Required: required,
			Schema:   &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
		},
	}
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

func SaveResponseExample(spec *openapi3.T, ref model.OpenAPIRef, statusCode int, header http.Header, body string) error {
	pathItem := spec.Paths.Find(ref.Path)
	if pathItem == nil {
		return fmt.Errorf("path %q not found", ref.Path)
	}

	op := pathItem.GetOperation(ref.Method)
	if op == nil {
		return fmt.Errorf("operation %s %s not found", ref.Method, ref.Path)
	}

	if op.Responses == nil {
		op.Responses = openapi3.NewResponses()
	}

	statusStr := strconv.Itoa(statusCode)
	responseRef := op.Responses.Value(statusStr)
	if responseRef == nil {
		desc := http.StatusText(statusCode)
		responseRef = &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: &desc,
			},
		}
		op.Responses.Set(statusStr, responseRef)
	}

	resp := responseRef.Value
	if resp.Content == nil {
		resp.Content = openapi3.Content{}
	}

	contentType := header.Get("Content-Type")
	if idx := strings.IndexByte(contentType, ';'); idx > 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	if contentType == "" {
		contentType = "application/json"
	}

	mt, exists := resp.Content[contentType]
	if !exists {
		mt = &openapi3.MediaType{}
		resp.Content[contentType] = mt
	}

	var exampleValue any
	if strings.Contains(contentType, "json") {
		if err := json.Unmarshal([]byte(body), &exampleValue); err != nil {
			exampleValue = body
		}
	} else {
		exampleValue = body
	}

	mt.Example = exampleValue
	return nil
}

func applyAuthSchemes(spec *openapi3.T, ref model.OpenAPIRef, schemes []model.AuthScheme) error {
	if spec.Components == nil {
		spec.Components = &openapi3.Components{}
	}
	if spec.Components.SecuritySchemes == nil {
		spec.Components.SecuritySchemes = make(openapi3.SecuritySchemes)
	}

	secReqs := openapi3.NewSecurityRequirements()

	for i := range schemes {
		s := &schemes[i]
		name := s.SchemeName
		if name == "" {
			name = generateSchemeName(spec)
		}

		switch s.Type {
		case model.AuthBasic:
			spec.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{
				Value: &openapi3.SecurityScheme{
					Type:   "http",
					Scheme: "basic",
				},
			}
		case model.AuthBearer:
			spec.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{
				Value: &openapi3.SecurityScheme{
					Type:   "http",
					Scheme: "bearer",
				},
			}
		case model.AuthAPIKey:
			in := s.KeyIn
			if in == "" {
				in = "header"
			}
			spec.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{
				Value: &openapi3.SecurityScheme{
					Type: "apiKey",
					In:   in,
					Name: s.KeyName,
				},
			}
		case model.AuthOAuth2:
			flows := &openapi3.OAuthFlows{}
			scopeMap := openapi3.StringMap[string]{}
			for _, sc := range strings.Fields(s.Scopes) {
				scopeMap[sc] = ""
			}

			switch s.GrantType {
			case "clientCredentials":
				flows.ClientCredentials = &openapi3.OAuthFlow{
					TokenURL: s.TokenURL,
					Scopes:   scopeMap,
				}
			case "implicit":
				flows.Implicit = &openapi3.OAuthFlow{
					AuthorizationURL: s.AuthURL,
					Scopes:           scopeMap,
				}
			case "password":
				flows.Password = &openapi3.OAuthFlow{
					TokenURL: s.TokenURL,
					Scopes:   scopeMap,
				}
			default: // authorizationCode
				flows.AuthorizationCode = &openapi3.OAuthFlow{
					AuthorizationURL: s.AuthURL,
					TokenURL:         s.TokenURL,
					Scopes:           scopeMap,
				}
			}

			spec.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{
				Value: &openapi3.SecurityScheme{
					Type:  "oauth2",
					Flows: flows,
				},
			}
		}

		req := openapi3.NewSecurityRequirement()
		var reqScopes []string
		if s.Type == model.AuthOAuth2 {
			reqScopes = strings.Fields(s.Scopes)
		}
		req.Authenticate(name, reqScopes...)
		secReqs.With(req)
	}

	if len(schemes) == 0 {
		if ref.Path == "" {
			spec.Security = nil
		} else {
			pathItem := spec.Paths.Find(ref.Path)
			if pathItem != nil {
				op := pathItem.GetOperation(ref.Method)
				if op != nil {
					op.Security = nil
				}
			}
		}
	} else if ref.Path == "" {
		spec.Security = *secReqs
	} else {
		pathItem := spec.Paths.Find(ref.Path)
		if pathItem != nil {
			op := pathItem.GetOperation(ref.Method)
			if op != nil {
				op.Security = secReqs
			}
		}
	}

	return nil
}

func generateSchemeName(spec *openapi3.T) string {
	for i := 0; ; i++ {
		name := fmt.Sprintf("lazyapi_auth_%d", i)
		if spec.Components.SecuritySchemes[name] == nil {
			return name
		}
	}
}

func ExtractGlobalSecurity(spec *openapi3.T) []model.AuthScheme {
	var schemes []model.AuthScheme
	for _, secReq := range spec.Security {
		extracted := extractSecurityFromSpec(spec.Components.SecuritySchemes, secReq)
		schemes = append(schemes, extracted...)
	}
	return schemes
}

func ExtractOperationSecurity(op *openapi3.Operation, spec *openapi3.T) []model.AuthScheme {
	if op.Security == nil {
		return nil
	}
	var schemes []model.AuthScheme
	for _, secReq := range *op.Security {
		extracted := extractSecurityFromSpec(spec.Components.SecuritySchemes, secReq)
		schemes = append(schemes, extracted...)
	}
	return schemes
}

func extractSecurityFromSpec(schemeMap openapi3.SecuritySchemes, secReq openapi3.SecurityRequirement) []model.AuthScheme {
	var schemes []model.AuthScheme
	for schemeName, reqScopes := range secReq {
		schemeRef, ok := schemeMap[schemeName]
		if !ok || schemeRef.Value == nil {
			continue
		}
		ss := schemeRef.Value

		as := model.AuthScheme{
			SchemeName: schemeName,
		}

		switch ss.Type {
		case "http":
			switch ss.Scheme {
			case "basic":
				as.Type = model.AuthBasic
			case "bearer":
				as.Type = model.AuthBearer
			}
		case "apiKey":
			as.Type = model.AuthAPIKey
			as.KeyName = ss.Name
			as.KeyIn = ss.In
		case "oauth2":
			as.Type = model.AuthOAuth2
			if ss.Flows != nil {
				switch {
				case ss.Flows.AuthorizationCode != nil:
					as.GrantType = "authorizationCode"
					as.AuthURL = ss.Flows.AuthorizationCode.AuthorizationURL
					as.TokenURL = ss.Flows.AuthorizationCode.TokenURL
					as.Scopes = scopeKeysJoin(ss.Flows.AuthorizationCode.Scopes, " ")
				case ss.Flows.ClientCredentials != nil:
					as.GrantType = "clientCredentials"
					as.TokenURL = ss.Flows.ClientCredentials.TokenURL
					as.Scopes = scopeKeysJoin(ss.Flows.ClientCredentials.Scopes, " ")
				case ss.Flows.Implicit != nil:
					as.GrantType = "implicit"
					as.AuthURL = ss.Flows.Implicit.AuthorizationURL
					as.Scopes = scopeKeysJoin(ss.Flows.Implicit.Scopes, " ")
				case ss.Flows.Password != nil:
					as.GrantType = "password"
					as.TokenURL = ss.Flows.Password.TokenURL
					as.Scopes = scopeKeysJoin(ss.Flows.Password.Scopes, " ")
				}
			}
		}

		if len(reqScopes) > 0 {
			as.Scopes = strings.Join(reqScopes, " ")
		}

		schemes = append(schemes, as)
	}
	return schemes
}

func scopeKeysJoin(m openapi3.StringMap[string], sep string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, sep)
}

func HasGlobalSecurity(spec *openapi3.T) bool {
	return len(spec.Security) > 0
}

func HasOperationSecurity(op *openapi3.Operation) bool {
	return op.Security != nil
}
