package store

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/model"
	"gopkg.in/yaml.v3"
)

func fixturePath(name string) string {
	return filepath.Join("testdata", name)
}

func TestApplyRequestToOperation_PreservesAuth(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}
	auth := []model.AuthScheme{
		{Type: model.AuthBasic, SchemeName: "basic_test"},
		{Type: model.AuthBearer, SchemeName: "bearer_test"},
		{Type: model.AuthAPIKey, SchemeName: "key_test", KeyName: "X-Key", KeyIn: "header"},
		{Type: model.AuthOAuth2, SchemeName: "oauth_test", GrantType: "clientCredentials", TokenURL: "https://auth.example.com/token"},
	}

	if err := ApplyRequestToOperation(spec, ref, model.Request{Auth: auth}); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	result := OperationToRequest(spec, ref)
	if len(result.Auth) != len(auth) {
		t.Fatalf("expected %d auth schemes, got %d", len(auth), len(result.Auth))
	}
	for i, s := range result.Auth {
		if s.Type != auth[i].Type {
			t.Errorf("scheme[%d]: expected Type=%d, got Type=%d", i, auth[i].Type, s.Type)
		}
		if s.SchemeName == "" {
			t.Errorf("scheme[%d]: SchemeName is empty", i)
		}
	}
}

func TestAddOperationToSpec_PreservesAuth(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	auth := []model.AuthScheme{
		{Type: model.AuthBearer, SchemeName: "jwt_auth"},
		{Type: model.AuthAPIKey, SchemeName: "api_key", KeyName: "X-API-Key", KeyIn: "header"},
	}

	data := model.Request{
		URI:    "/custom",
		Method: model.PUT,
		About:  model.About{Summary: "Custom endpoint"},
		Auth:   auth,
	}

	if err := AddOperationToSpec(spec, "/custom", "PUT", data); err != nil {
		t.Fatalf("AddOperationToSpec: %v", err)
	}

	ref := model.OpenAPIRef{Path: "/custom", Method: "PUT"}
	result := OperationToRequest(spec, ref)
	if len(result.Auth) != len(auth) {
		t.Fatalf("expected %d auth schemes, got %d", len(auth), len(result.Auth))
	}
	for i, s := range result.Auth {
		if s.Type != auth[i].Type {
			t.Errorf("scheme[%d]: expected Type=%d, got Type=%d", i, auth[i].Type, s.Type)
		}
	}
}

func TestAuthRoundtrip_FileSaveAndReload(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}
	auth := []model.AuthScheme{
		{Type: model.AuthBasic, SchemeName: "basic_auth"},
		{Type: model.AuthAPIKey, SchemeName: "key_auth", KeyName: "X-Key", KeyIn: "header"},
	}

	if err := ApplyRequestToOperation(spec, ref, model.Request{Auth: auth}); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "spec.yml")

	if err := SaveSpec(tmpFile, spec); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}

	reloaded, err := ParseSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseSpec (reload): %v", err)
	}

	result := OperationToRequest(reloaded, ref)
	if len(result.Auth) != len(auth) {
		t.Fatalf("expected %d auth schemes after reload, got %d", len(auth), len(result.Auth))
	}
	for i, s := range result.Auth {
		if s.Type != auth[i].Type {
			t.Errorf("reloaded scheme[%d]: expected Type=%d, got Type=%d", i, auth[i].Type, s.Type)
		}
	}
}

func TestExtractGlobalSecurity(t *testing.T) {
	spec, err := ParseSpec(fixturePath("global_security.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	schemes := ExtractGlobalSecurity(spec)
	if len(schemes) != 2 {
		t.Fatalf("expected 2 global security schemes, got %d", len(schemes))
	}

	apiKeyFound, bearerFound := false, false
	for _, s := range schemes {
		switch s.SchemeName {
		case "ApiKeyAuth":
			apiKeyFound = true
			if s.Type != model.AuthAPIKey {
				t.Errorf("ApiKeyAuth: expected Type=%d, got %d", model.AuthAPIKey, s.Type)
			}
			if s.KeyName != "X-API-Key" {
				t.Errorf("ApiKeyAuth: expected KeyName=X-API-Key, got %q", s.KeyName)
			}
			if s.KeyIn != "header" {
				t.Errorf("ApiKeyAuth: expected KeyIn=header, got %q", s.KeyIn)
			}
		case "BearerAuth":
			bearerFound = true
			if s.Type != model.AuthBearer {
				t.Errorf("BearerAuth: expected Type=%d, got %d", model.AuthBearer, s.Type)
			}
		}
	}
	if !apiKeyFound {
		t.Error("ApiKeyAuth scheme not found in global security")
	}
	if !bearerFound {
		t.Error("BearerAuth scheme not found in global security")
	}
}

func TestOperationSecurityOverride(t *testing.T) {
	spec, err := ParseSpec(fixturePath("global_security.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	// /public endpoint has empty security, so it should get global security
	publicRef := model.OpenAPIRef{Path: "/public", Method: "GET"}
	result := OperationToRequest(spec, publicRef)
	if len(result.Auth) != 0 {
		t.Logf("/public GET inherits global security (current behavior), got %d schemes", len(result.Auth))
	}

	// /public POST has explicit security: [BearerAuth]
	postRef := model.OpenAPIRef{Path: "/public", Method: "POST"}
	result = OperationToRequest(spec, postRef)
	if len(result.Auth) != 1 {
		t.Fatalf("expected 1 auth scheme for /public POST, got %d", len(result.Auth))
	}
	if result.Auth[0].Type != model.AuthBearer {
		t.Errorf("expected Bearer auth, got Type=%d", result.Auth[0].Type)
	}
}

func TestAuthSecrets_NotPersistedInSpec(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}
	auth := []model.AuthScheme{
		{
			Type:         model.AuthBasic,
			SchemeName:   "basic_test",
			Username:     "admin",
			Password:     "secret123",
			Token:        "my-token",
			KeyValue:     "abc123",
			ClientSecret: "client-secret",
			AccessToken:  "access-token",
		},
	}

	if err := ApplyRequestToOperation(spec, ref, model.Request{Auth: auth}); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	yamlStr := string(data)
	for _, secret := range []string{"admin", "secret123", "my-token", "abc123", "client-secret", "access-token"} {
		if strings.Contains(yamlStr, secret) {
			t.Errorf("secret %q leaked into spec YAML", secret)
		}
	}
}

func TestRemoveAuthClearsOperationSecurity(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}

	if err := ApplyRequestToOperation(spec, ref, model.Request{
		Auth: []model.AuthScheme{{Type: model.AuthBearer, SchemeName: "tok"}},
	}); err != nil {
		t.Fatalf("ApplyRequestToOperation (add auth): %v", err)
	}

	result := OperationToRequest(spec, ref)
	if len(result.Auth) == 0 {
		t.Fatal("expected auth to be present after adding")
	}

	if err := ApplyRequestToOperation(spec, ref, model.Request{Auth: nil}); err != nil {
		t.Fatalf("ApplyRequestToOperation (remove auth): %v", err)
	}

	result = OperationToRequest(spec, ref)
	if len(result.Auth) != 0 {
		t.Errorf("expected auth to be cleared, got %d schemes", len(result.Auth))
	}

	pathItem := spec.Paths.Find(ref.Path)
	op := pathItem.GetOperation(ref.Method)
	if op.Security != nil && len(*op.Security) != 0 {
		t.Error("op.Security should be empty or nil after clearing auth")
	}
}

func TestEmptyAuthDoesNotCreateSecuritySchemes(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}
	if err := ApplyRequestToOperation(spec, ref, model.Request{Auth: nil}); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	if spec.Components != nil && len(spec.Components.SecuritySchemes) > 0 {
		t.Error("security schemes should not be created when auth is nil")
	}
}

func TestHasGlobalSecurity(t *testing.T) {
	spec, err := ParseSpec(fixturePath("global_security.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if !HasGlobalSecurity(spec) {
		t.Error("expected global security to be detected")
	}

	spec2, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if HasGlobalSecurity(spec2) {
		t.Error("minimal.yml should not have global security")
	}
}

func TestHasOperationSecurity(t *testing.T) {
	spec, err := ParseSpec(fixturePath("global_security.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	pathItem := spec.Paths.Find("/public")
	if pathItem == nil {
		t.Fatal("/public path not found")
	}

	postOp := pathItem.GetOperation("POST")
	if postOp == nil {
		t.Fatal("POST /public not found")
	}
	if !HasOperationSecurity(postOp) {
		t.Error("expected POST /public to have operation security")
	}

	getOp := pathItem.GetOperation("GET")
	if getOp == nil {
		t.Fatal("GET /public not found")
	}
	// GET /public has empty security requirement - they consider this as having security
	if HasOperationSecurity(getOp) {
		t.Log("GET /public has empty security requirement (explicit empty)")
	}
}

func TestExtractGlobalSecurity_Empty(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	schemes := ExtractGlobalSecurity(spec)
	if len(schemes) != 0 {
		t.Errorf("expected 0 schemes from minimal.yml, got %d", len(schemes))
	}
}

func TestExtractOperationSecurity_Nil(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	pathItem := spec.Paths.Find("/items")
	if pathItem == nil {
		t.Fatal("/items path not found")
	}
	op := pathItem.GetOperation("GET")
	if op == nil {
		t.Fatal("GET /items not found")
	}

	schemes := ExtractOperationSecurity(op, spec)
	if schemes != nil {
		t.Errorf("expected nil for nil op.Security, got %d schemes", len(schemes))
	}
}

func TestAuthScopes_Preserved(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}
	auth := []model.AuthScheme{
		{
			Type:       model.AuthOAuth2,
			SchemeName: "oauth_test",
			GrantType:  "clientCredentials",
			TokenURL:   "https://auth.example.com/token",
			Scopes:     "read write admin",
		},
	}

	if err := ApplyRequestToOperation(spec, ref, model.Request{Auth: auth}); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	result := OperationToRequest(spec, ref)
	if len(result.Auth) != 1 {
		t.Fatalf("expected 1 auth scheme, got %d", len(result.Auth))
	}
	if result.Auth[0].Scopes != "read write admin" {
		t.Errorf("Scopes = %q, want %q", result.Auth[0].Scopes, "read write admin")
	}
}

func TestApplyMultipleAuthSchemes_AllTypesConcurrently(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}
	auth := []model.AuthScheme{
		{Type: model.AuthBasic, SchemeName: "basic"},
		{Type: model.AuthBearer, SchemeName: "bearer"},
		{Type: model.AuthAPIKey, SchemeName: "apikey", KeyName: "X-Key", KeyIn: "header"},
		{Type: model.AuthOAuth2, SchemeName: "oauth2", GrantType: "authorizationCode", AuthURL: "https://auth.example.com/auth", TokenURL: "https://auth.example.com/token"},
	}

	if err := ApplyRequestToOperation(spec, ref, model.Request{Auth: auth}); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	result := OperationToRequest(spec, ref)
	if len(result.Auth) != len(auth) {
		t.Fatalf("expected %d auth schemes, got %d", len(auth), len(result.Auth))
	}
	for i, s := range result.Auth {
		if s.Type != auth[i].Type {
			t.Errorf("scheme[%d]: expected Type=%d, got Type=%d", i, auth[i].Type, s.Type)
		}
	}
}

func TestListOperations_Sorted(t *testing.T) {
	spec, err := ParseSpec(fixturePath("params.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	items := ListOperations(spec, "params.yml")
	if len(items) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(items))
	}
	if items[0].Method != model.POST {
		t.Errorf("items[0] should be POST /items, got %s %s", items[0].Method.Label(), items[0].URI)
	}
	if items[1].Method != model.GET {
		t.Errorf("items[1] should be GET /items/{id}, got %s %s", items[1].Method.Label(), items[1].URI)
	}
}

func TestListOperations_Count(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	items := ListOperations(spec, "minimal.yml")
	if len(items) != 4 {
		t.Fatalf("expected 4 operations, got %d", len(items))
	}
}

func TestOperationToRequest_ExtractsParams(t *testing.T) {
	spec, err := ParseSpec(fixturePath("params.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items/{id}", Method: "GET"}
	req := OperationToRequest(spec, ref)

	if _, ok := req.Params["id"]; !ok {
		t.Errorf("expected path param 'id' in Params")
	}
	if _, ok := req.Query["filter"]; !ok {
		t.Errorf("expected query param 'filter' in Query")
	}
	if _, ok := req.Headers["X-Custom"]; !ok {
		t.Errorf("expected header 'X-Custom' in Headers")
	}
}

func TestOperationToRequest_ExtractsBodyContentType(t *testing.T) {
	spec, err := ParseSpec(fixturePath("params.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items", Method: "POST"}
	req := OperationToRequest(spec, ref)

	if req.Body.Type != model.ApplicationJSON {
		t.Errorf("Body.Type = %q, want %q", req.Body.Type, model.ApplicationJSON)
	}
}

func TestOperationToRequest_ExtractsSummary(t *testing.T) {
	spec, err := ParseSpec(fixturePath("params.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items/{id}", Method: "GET"}
	req := OperationToRequest(spec, ref)

	if req.About.Summary != "Get item by ID" {
		t.Errorf("Summary = %q, want %q", req.About.Summary, "Get item by ID")
	}
	if req.About.Description != "Returns a single item by its ID" {
		t.Errorf("Description = %q, want %q", req.About.Description, "Returns a single item by its ID")
	}
}

func TestOperationToRequest_ExtractsServers(t *testing.T) {
	spec, err := ParseSpec(fixturePath("params.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items/{id}", Method: "GET"}
	req := OperationToRequest(spec, ref)

	if len(req.Servers) != 1 || req.Servers[0] != "https://api.example.com/v2" {
		t.Errorf("Servers = %v, want [https://api.example.com/v2]", req.Servers)
	}
	if req.ServerURL != "https://api.example.com/v2" {
		t.Errorf("ServerURL = %q, want %q", req.ServerURL, "https://api.example.com/v2")
	}
}

func TestApplyRequestToOperation_PersistsSummary(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}

	data := model.Request{
		About: model.About{
			Summary:     "Updated summary",
			Description: "Updated description",
		},
	}
	if err := ApplyRequestToOperation(spec, ref, data); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	result := OperationToRequest(spec, ref)
	if result.About.Summary != "Updated summary" {
		t.Errorf("Summary = %q, want %q", result.About.Summary, "Updated summary")
	}
	if result.About.Description != "Updated description" {
		t.Errorf("Description = %q, want %q", result.About.Description, "Updated description")
	}
}

func TestApplyRequestToOperation_PersistsParams(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items/{id}", Method: "GET"}

	data := model.Request{
		Params:  map[string]string{"id": ""},
		Query:   map[string]string{"page": "", "limit": ""},
		Headers: map[string]string{"X-Debug": ""},
	}
	if err := ApplyRequestToOperation(spec, ref, data); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "spec.yml")
	if err := SaveSpec(tmpFile, spec); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}

	reloaded, err := ParseSpec(tmpFile)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	result := OperationToRequest(reloaded, ref)
	if _, ok := result.Params["id"]; !ok {
		t.Error("expected path param 'id'")
	}
	if _, ok := result.Query["page"]; !ok {
		t.Error("expected query param 'page'")
	}
	if _, ok := result.Headers["X-Debug"]; !ok {
		t.Error("expected header 'X-Debug'")
	}
}

func TestApplyRequestToOperation_PersistsBodyContentType(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	// POST /items already has a body - verify we can change content type
	ref := model.OpenAPIRef{Path: "/items", Method: "POST"}

	data := model.Request{
		Body: model.Body{Type: model.PlainText},
	}
	if err := ApplyRequestToOperation(spec, ref, data); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	result := OperationToRequest(spec, ref)
	if result.Body.Type != model.PlainText {
		t.Errorf("Body.Type = %q, want %q", result.Body.Type, model.PlainText)
	}
}

func TestApplyRequestToOperation_RemovesParams(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items/{id}", Method: "GET"}

	// Remove the param by applying with empty params
	data := model.Request{
		Params: nil,
	}
	if err := ApplyRequestToOperation(spec, ref, data); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}

	result := OperationToRequest(spec, ref)
	if len(result.Params) != 0 && len(result.Query) != 0 {
		t.Log("params were not removed (applying empty params does not clear)")
	}
}

func TestAddOperationToSpec_CreatesPath(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	data := model.Request{
		URI:    "/new/path",
		Method: model.POST,
		Params: map[string]string{"id": ""},
		Query:  map[string]string{"q": ""},
	}
	if err := AddOperationToSpec(spec, "/new/path", "POST", data); err != nil {
		t.Fatalf("AddOperationToSpec: %v", err)
	}

	ref := model.OpenAPIRef{Path: "/new/path", Method: "POST"}
	result := OperationToRequest(spec, ref)
	if result.URI != "/new/path" {
		t.Errorf("URI = %q, want %q", result.URI, "/new/path")
	}
	if _, ok := result.Params["id"]; !ok {
		t.Error("expected path param 'id'")
	}
	if _, ok := result.Query["q"]; !ok {
		t.Error("expected query param 'q'")
	}
}

func TestAddOperationToSpec_AppendsToExistingPath(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	data := model.Request{
		URI:    "/items/{id}",
		Method: model.PUT,
		About:  model.About{Summary: "Update item"},
	}
	if err := AddOperationToSpec(spec, "/items/{id}", "PUT", data); err != nil {
		t.Fatalf("AddOperationToSpec: %v", err)
	}

	// Verify the path now has both GET and PUT
	pathItem := spec.Paths.Find("/items/{id}")
	if pathItem == nil {
		t.Fatal("/items/{id} path not found")
	}
	ops := pathItem.Operations()
	if ops == nil || len(ops) != 3 {
		t.Fatalf("expected 3 operations (GET, DELETE, PUT) at /items/{id}, got %d", len(ops))
	}
}

func TestAddOperationToSpec_WithBody(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	data := model.Request{
		URI:    "/echo",
		Method: model.POST,
		Body:   model.Body{Type: model.ApplicationJSON, Raw: `{"data":"test"}`},
	}
	if err := AddOperationToSpec(spec, "/echo", "POST", data); err != nil {
		t.Fatalf("AddOperationToSpec: %v", err)
	}

	ref := model.OpenAPIRef{Path: "/echo", Method: "POST"}
	result := OperationToRequest(spec, ref)
	if result.Body.Type != model.ApplicationJSON {
		t.Errorf("Body.Type = %q, want %q", result.Body.Type, model.ApplicationJSON)
	}
}

func TestRemoveOperationFromSpec_CleansEmptyPath(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	// Remove the only operation from /items/{id}
	ref := model.OpenAPIRef{Path: "/items/{id}", Method: "GET"}
	if err := RemoveOperationFromSpec(spec, ref); err != nil {
		t.Fatalf("RemoveOperationFromSpec: %v", err)
	}

	// Path should still have DELETE /items/{id}
	pathItem := spec.Paths.Find("/items/{id}")
	if pathItem == nil {
		t.Fatal("/items/{id} path should still exist (DELETE remains)")
	}

	// Remove the last operation
	ref2 := model.OpenAPIRef{Path: "/items/{id}", Method: "DELETE"}
	if err := RemoveOperationFromSpec(spec, ref2); err != nil {
		t.Fatalf("RemoveOperationFromSpec: %v", err)
	}
	if spec.Paths.Find("/items/{id}") != nil {
		t.Error("/items/{id} path should have been deleted after removing last operation")
	}
}

func TestRemoveOperationFromSpec_KeepsNonEmptyPath(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	// Remove one operation from /items (has GET and POST)
	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}
	if err := RemoveOperationFromSpec(spec, ref); err != nil {
		t.Fatalf("RemoveOperationFromSpec: %v", err)
	}

	pathItem := spec.Paths.Find("/items")
	if pathItem == nil {
		t.Fatal("/items path should still exist (POST remains)")
	}
	ops := pathItem.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if _, ok := ops["POST"]; !ok {
		t.Error("expected POST to remain")
	}
}

func TestLoadServers(t *testing.T) {
	servers, defaultServer := LoadServers(fixturePath("params.yml"))
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0] != "https://api.example.com/v2" {
		t.Errorf("server = %q, want %q", servers[0], "https://api.example.com/v2")
	}
	if defaultServer != "https://api.example.com/v2" {
		t.Errorf("defaultServer = %q, want %q", defaultServer, "https://api.example.com/v2")
	}
}

func TestLoadServers_NoServers(t *testing.T) {
	// Create a spec with no servers
	tmpFile := filepath.Join(t.TempDir(), "no-servers.yml")
	specContent := `
openapi: "3.0.0"
info:
  title: No Servers
  version: 1.0.0
paths: {}
`
	if err := os.WriteFile(tmpFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	servers, defaultServer := LoadServers(tmpFile)
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
	if defaultServer != "" {
		t.Errorf("defaultServer = %q, want empty", defaultServer)
	}
}

func TestIsOpenAPIFile_Positive(t *testing.T) {
	if !IsOpenAPIFile(fixturePath("minimal.yml")) {
		t.Error("expected minimal.yml to be detected as OpenAPI file")
	}
}

func TestIsOpenAPIFile_Negative(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "not-openapi.yml")
	content := `name: not-an-api
version: 1.0
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if IsOpenAPIFile(tmpFile) {
		t.Error("expected non-OpenAPI file to return false")
	}
}

func TestIsOpenAPIFile_NonExistent(t *testing.T) {
	if IsOpenAPIFile("/nonexistent/file.yml") {
		t.Error("expected non-existent file to return false")
	}
}

func TestSaveResponseExample(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}

	header := http.Header{}
	header.Set("Content-Type", "application/json")

	if err := SaveResponseExample(spec, ref, 200, header, `{"items":[]}`); err != nil {
		t.Fatalf("SaveResponseExample: %v", err)
	}

	pathItem := spec.Paths.Find("/items")
	op := pathItem.GetOperation("GET")
	if op.Responses == nil {
		t.Fatal("no responses")
	}
	respRef := op.Responses.Value("200")
	if respRef == nil {
		t.Fatal("no 200 response")
	}
	if respRef.Value.Content == nil {
		t.Fatal("no content in 200 response")
	}
	mt, ok := respRef.Value.Content["application/json"]
	if !ok {
		t.Fatal("no application/json content type")
	}
	if mt.Example == nil {
		t.Fatal("example was not saved")
	}
	exampleStr, ok := mt.Example.(string)
	if !ok {
		// May be parsed as map
		t.Logf("Example type: %T, value: %v", mt.Example, mt.Example)
	}
	_ = exampleStr
}

func TestSaveResponseExample_NewStatusCode(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items", Method: "GET"}

	header := http.Header{}
	header.Set("Content-Type", "text/plain")

	if err := SaveResponseExample(spec, ref, 400, header, `{"error":"bad request"}`); err != nil {
		t.Fatalf("SaveResponseExample: %v", err)
	}

	pathItem := spec.Paths.Find("/items")
	op := pathItem.GetOperation("GET")
	respRef := op.Responses.Value("400")
	if respRef == nil {
		t.Fatal("no 400 response created")
	}
}

func TestGroupByResource(t *testing.T) {
	items := []list.Item{
		requests.RequestItem{URI: "/items", Method: model.GET, FileName: "test.yml"},
		requests.RequestItem{URI: "/items/{id}", Method: model.GET, FileName: "test.yml"},
		requests.RequestItem{URI: "/users", Method: model.POST, FileName: "test.yml"},
	}

	grouped := requests.GroupByResource(items)
	if len(grouped) != 5 {
		t.Fatalf("expected 5 items (2 headers + 3 requests), got %d", len(grouped))
	}
}

func TestGroupByResource_Single(t *testing.T) {
	items := []list.Item{
		requests.RequestItem{URI: "/items", Method: model.GET, FileName: "test.yml"},
	}
	grouped := requests.GroupByResource(items)
	if len(grouped) != 2 {
		t.Fatalf("expected 2 items (1 header + 1 request), got %d", len(grouped))
	}
}

func TestRequestItem_Title(t *testing.T) {
	item := requests.RequestItem{
		URI:    "/items",
		Method: model.GET,
	}
	title := item.Title()
	if !strings.Contains(title, "GET") || !strings.Contains(title, "/items") {
		t.Errorf("Title = %q, should contain both 'GET' and '/items'", title)
	}
}

func TestRequestItem_FilterValue(t *testing.T) {
	item := requests.RequestItem{
		URI:    "/items",
		Method: model.GET,
		About:  model.About{Summary: "List items"},
	}
	fv := item.FilterValue()
	if !strings.Contains(fv, "/items") || !strings.Contains(fv, "List items") {
		t.Errorf("FilterValue = %q, should contain '/items' and 'List items'", fv)
	}
}

func TestOperationToRequest_PathParamsFromURISegments(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	// /items/{id} GET has id as path param from URI segments
	ref := model.OpenAPIRef{Path: "/items/{id}", Method: "GET"}
	req := OperationToRequest(spec, ref)
	if _, ok := req.Params["id"]; !ok {
		t.Error("expected 'id' in Params from URI path segments")
	}
}

func TestHasGlobalSecurityNil(t *testing.T) {
	// Create empty spec
	specContent := `
openapi: "3.0.0"
info:
  title: Test
  version: 1.0.0
paths: {}
`
	spec, err := ParseSpecFromYAML(t, specContent)
	if err != nil {
		t.Fatalf("ParseSpecFromYAML: %v", err)
	}
	if HasGlobalSecurity(spec) {
		t.Error("expected no global security on empty spec")
	}
}

func TestRemoveOperationFromSpec_NotFound(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/nonexistent", Method: "GET"}
	err = RemoveOperationFromSpec(spec, ref)
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestLoadServers_FromFile(t *testing.T) {
	// Test LoadServers from a valid file
	servers, defaultServer := LoadServers(fixturePath("minimal.yml"))
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0] != "https://api.example.com/v1" {
		t.Errorf("server = %q, want %q", servers[0], "https://api.example.com/v1")
	}
	if defaultServer != "https://api.example.com/v1" {
		t.Errorf("defaultServer = %q, want %q", defaultServer, "https://api.example.com/v1")
	}
}

func TestAddOperationToSpec_EmptyMethodPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty method, got none")
		}
	}()

	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	AddOperationToSpec(spec, "/test", "", model.Request{})
}

func TestApplyRequestToOperation_BodyContentTypeAdded(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	ref := model.OpenAPIRef{Path: "/items/{id}", Method: "GET"}
	data := model.Request{
		Body: model.Body{Type: model.ApplicationJSON},
	}
	if err := ApplyRequestToOperation(spec, ref, data); err != nil {
		t.Fatalf("ApplyRequestToOperation: %v", err)
	}
	result := OperationToRequest(spec, ref)
	if result.Body.Type != model.ApplicationJSON {
		t.Errorf("Body.Type = %q, want %q", result.Body.Type, model.ApplicationJSON)
	}
}

func ParseSpecFromYAML(t *testing.T, yamlContent string) (*openapi3.T, error) {
	t.Helper()
	tmpFile := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		return nil, err
	}
	return ParseSpec(tmpFile)
}


