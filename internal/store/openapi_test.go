package store

import (
	"path/filepath"
	"strings"
	"testing"

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


