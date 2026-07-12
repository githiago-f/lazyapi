package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/model"
	"gopkg.in/yaml.v3"
)

func TestNewDraftPath_CreatesDir(t *testing.T) {
	path := NewDraftPath("test-spec.yml")
	if path == "" {
		t.Fatal("NewDraftPath returned empty")
	}
	if !strings.Contains(path, "draft.new.") {
		t.Errorf("NewDraftPath = %q, want 'draft.new.' in path", path)
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("parent dir was not created: %s", dir)
	}
	_ = os.RemoveAll(dir)
}

func TestNewDraftPath_Increments(t *testing.T) {
	p1 := NewDraftPath("test-spec.yml")
	p2 := NewDraftPath("test-spec.yml")
	if p1 == p2 {
		t.Errorf("expected different draft paths, got same: %s", p1)
	}
	dir := filepath.Dir(p1)
	_ = os.RemoveAll(dir)
}

func TestTempPath_Format(t *testing.T) {
	path := TempPath("spec.yml")
	if !strings.Contains(path, "lazyapi") {
		t.Errorf("TempPath = %q, should contain 'lazyapi'", path)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("TempPath = %q, should be absolute", path)
	}
}

func TestSaveTempFile_Roundtrip(t *testing.T) {
	data := model.Request{
		FileName: "test-spec.yml",
		URI:      "/test",
		Method:   model.GET,
		About:    model.About{Summary: "Test endpoint"},
	}

	cmd := SaveTempFile(data)
	msg := cmd()
	if msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}

	// Verify file was created using TempPath
	tempFile := TempPath(data.FileName)
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatal("SaveTempFile did not create a temp file at", tempFile)
	}
	defer func() { _ = os.Remove(tempFile) }()

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("temp file is empty")
	}
	if !strings.Contains(string(content), "/test") {
		t.Errorf("temp file should contain URI /test, got: %s", string(content))
	}
}

func TestListDrafts_Empty(t *testing.T) {
	items := ListDrafts("nonexistent.yml")
	if items != nil {
		t.Errorf("ListDrafts returned non-nil for nonexistent pattern, got %d items", len(items))
	}
}

func TestListDrafts_WithDrafts(t *testing.T) {
	// Create a draft file where ListDrafts will look
	draftDir := tempDirForFile("spec.yml")
	if err := os.MkdirAll(draftDir, 0755); err != nil {
		t.Fatal(err)
	}

	draftContent := "uri: /items\nmethod: get\nabout:\n  summary: Test\n"
	draftFile := filepath.Join(draftDir, "draft.new.1")
	if err := os.WriteFile(draftFile, []byte(draftContent), 0644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(draftDir) }()

	items := ListDrafts("spec.yml")
	if len(items) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(items))
	}
	if items[0].URI != "/items" {
		t.Errorf("draft URI = %q, want %q", items[0].URI, "/items")
	}
	if items[0].Method != model.GET {
		t.Errorf("draft Method = %v, want GET", items[0].Method)
	}
}

func TestLoadForDuplicate_Draft(t *testing.T) {
	draftDir := tempDirForFile("spec.yml")
	if err := os.MkdirAll(draftDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(draftDir) }()

	draftContent := "uri: /items\nmethod: post\nabout:\n  summary: Create item\n"
	draftFile := filepath.Join(draftDir, "draft.new.1")
	if err := os.WriteFile(draftFile, []byte(draftContent), 0644); err != nil {
		t.Fatal(err)
	}

	item := requests.RequestItem{
		URI:       "/items",
		Method:    model.POST,
		DraftPath: draftFile,
		FileName:  "spec.yml",
	}

	cmd := LoadForDuplicate(item)
	msg := cmd()

	dup, ok := msg.(DuplicateData)
	if !ok {
		t.Fatalf("LoadForDuplicate returned %T, want DuplicateData", msg)
	}
	if dup.Data.URI != "/items" {
		t.Errorf("duplicate URI = %q, want %q", dup.Data.URI, "/items")
	}
	if dup.Data.Method != model.POST {
		t.Errorf("duplicate Method = %v, want POST", dup.Data.Method)
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/items/{id}", "_items__id_"},
		{"simple", "simple"},
		{"path/with/spaces", "path_with_spaces"},
	}
	for _, tt := range tests {
		got := sanitizePath(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- ServerURL YAML serialization ---

func TestServerURL_MarshalUnmarshal(t *testing.T) {
	original := model.Request{
		URI:       "/test",
		Method:    model.GET,
		ServerURL: "https://custom.example.com",
		Servers:   []string{"https://server1.example.com", "https://custom.example.com"},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	var restored model.Request
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if restored.ServerURL != original.ServerURL {
		t.Errorf("ServerURL = %q, want %q", restored.ServerURL, original.ServerURL)
	}

	if len(restored.Servers) != 0 {
		t.Errorf("Servers should not be serialized (yaml:\"-\"), got %v", restored.Servers)
	}
}

func TestServerURL_EmptyOmitted(t *testing.T) {
	req := model.Request{
		URI:    "/test",
		Method: model.GET,
	}

	data, err := yaml.Marshal(req)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	if strings.Contains(string(data), "serverUrl") {
		t.Errorf("serverUrl should be omitted when empty, got: %s", string(data))
	}
}

// --- helpers ---

func fullRequest() model.Request {
	return model.Request{
		FileName:  "testdata/minimal.yml",
		URI:       "/items",
		Method:    model.POST,
		ServerURL: "https://api.example.com/v1",
		About: model.About{
			Summary:     "Test summary",
			Description: "Test description",
		},
		Body: model.Body{
			Type: model.ApplicationJSON,
			Raw:  `{"key":"value"}`,
		},
		Headers: map[string]string{
			"X-Custom":  "header-value",
			"X-Debug":   "true",
		},
		Params: map[string]string{
			"id": "42",
		},
		Query: map[string]string{
			"page":  "1",
			"limit": "10",
		},
		Auth: []model.AuthScheme{
			{Type: model.AuthBasic, Username: "admin", Password: "secret", SchemeName: "test_auth"},
			{Type: model.AuthBearer, Token: "my-token", SchemeName: "bearer_auth"},
		},
	}
}

func TestOpenRequestFile_TempPreservesAllEditableFields(t *testing.T) {
	ref := model.OpenAPIRef{
		FilePath: "testdata/minimal.yml",
		Path:     "/items",
		Method:   "POST",
	}

	original := fullRequest()
	original.OpenAPIRef = &model.OpenAPIRef{
		FilePath: ref.FilePath,
		Path:     ref.Path,
		Method:   ref.Method,
	}

	// Save to temp
	cmd := SaveTempFile(original)
	msg := cmd()
	if msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}
	defer func() { _ = os.RemoveAll(tempDirForFile(ref.FilePath)) }()

	// Load from temp
	loadCmd := OpenRequestFile(ref)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenRequestFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	// Fields that should match
	if got.URI != original.URI {
		t.Errorf("URI = %q, want %q", got.URI, original.URI)
	}
	if got.Method != original.Method {
		t.Errorf("Method = %v, want %v", got.Method, original.Method)
	}
	if got.About.Summary != original.About.Summary {
		t.Errorf("Summary = %q, want %q", got.About.Summary, original.About.Summary)
	}
	if got.About.Description != original.About.Description {
		t.Errorf("Description = %q, want %q", got.About.Description, original.About.Description)
	}
	if got.Body.Raw != original.Body.Raw {
		t.Errorf("Body = %q, want %q", got.Body.Raw, original.Body.Raw)
	}
	if got.Body.Type != original.Body.Type {
		t.Errorf("Body.Type = %q, want %q", got.Body.Type, original.Body.Type)
	}
	if len(got.Headers) != len(original.Headers) {
		t.Errorf("Headers count = %d, want %d", len(got.Headers), len(original.Headers))
	}
	for k, v := range original.Headers {
		if got.Headers[k] != v {
			t.Errorf("Header[%q] = %q, want %q", k, got.Headers[k], v)
		}
	}
	if len(got.Params) != len(original.Params) {
		t.Errorf("Params count = %d, want %d", len(got.Params), len(original.Params))
	}
	for k, v := range original.Params {
		if got.Params[k] != v {
			t.Errorf("Param[%q] = %q, want %q", k, got.Params[k], v)
		}
	}
	if len(got.Query) != len(original.Query) {
		t.Errorf("Query count = %d, want %d", len(got.Query), len(original.Query))
	}
	for k, v := range original.Query {
		if got.Query[k] != v {
			t.Errorf("Query[%q] = %q, want %q", k, got.Query[k], v)
		}
	}
}

func TestOpenRequestFile_TempRestoresServers(t *testing.T) {
	ref := model.OpenAPIRef{
		FilePath: "testdata/minimal.yml",
		Path:     "/items",
		Method:   "POST",
	}

	original := fullRequest()
	original.OpenAPIRef = &model.OpenAPIRef{
		FilePath: ref.FilePath,
		Path:     ref.Path,
		Method:   ref.Method,
	}

	cmd := SaveTempFile(original)
	msg := cmd()
	if msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}
	defer func() { _ = os.RemoveAll(tempDirForFile(ref.FilePath)) }()

	loadCmd := OpenRequestFile(ref)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenRequestFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	// Servers should be loaded from the spec file
	if len(got.Servers) == 0 {
		t.Fatal("Servers should not be empty (loaded from spec)")
	}
	if got.Servers[0] != "https://api.example.com/v1" {
		t.Errorf("Servers[0] = %q, want %q", got.Servers[0], "https://api.example.com/v1")
	}

	// ServerURL should be preserved from original since it's in the spec server list
	if got.ServerURL != original.ServerURL {
		t.Errorf("ServerURL = %q, want %q (preserved user selection)", got.ServerURL, original.ServerURL)
	}
}

func TestOpenRequestFile_TempPreservesAuth(t *testing.T) {
	ref := model.OpenAPIRef{
		FilePath: "testdata/minimal.yml",
		Path:     "/items",
		Method:   "POST",
	}

	original := fullRequest()
	original.OpenAPIRef = &model.OpenAPIRef{
		FilePath: ref.FilePath,
		Path:     ref.Path,
		Method:   ref.Method,
	}

	cmd := SaveTempFile(original)
	msg := cmd()
	if msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}
	defer func() { _ = os.RemoveAll(tempDirForFile(ref.FilePath)) }()

	loadCmd := OpenRequestFile(ref)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenRequestFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	if len(got.Auth) != len(original.Auth) {
		t.Fatalf("Auth count = %d, want %d", len(got.Auth), len(original.Auth))
	}
	for i := range original.Auth {
		if got.Auth[i].Type != original.Auth[i].Type {
			t.Errorf("Auth[%d].Type = %v, want %v", i, got.Auth[i].Type, original.Auth[i].Type)
		}
		if got.Auth[i].SchemeName != original.Auth[i].SchemeName {
			t.Errorf("Auth[%d].SchemeName = %q, want %q", i, got.Auth[i].SchemeName, original.Auth[i].SchemeName)
		}
		if got.Auth[i].Username != original.Auth[i].Username {
			t.Errorf("Auth[%d].Username = %q, want %q", i, got.Auth[i].Username, original.Auth[i].Username)
		}
		if got.Auth[i].Token != original.Auth[i].Token {
			t.Errorf("Auth[%d].Token = %q, want %q", i, got.Auth[i].Token, original.Auth[i].Token)
		}
	}
}

func TestOpenRequestFile_TempRestoresOpenAPIRef(t *testing.T) {
	ref := model.OpenAPIRef{
		FilePath: "testdata/minimal.yml",
		Path:     "/items",
		Method:   "POST",
	}

	original := fullRequest()
	original.OpenAPIRef = &model.OpenAPIRef{
		FilePath: ref.FilePath,
		Path:     ref.Path,
		Method:   ref.Method,
	}

	cmd := SaveTempFile(original)
	msg := cmd()
	if msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}
	defer func() { _ = os.RemoveAll(tempDirForFile(ref.FilePath)) }()

	loadCmd := OpenRequestFile(ref)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenRequestFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	if got.OpenAPIRef == nil {
		t.Fatal("OpenAPIRef should not be nil after loading from temp")
	}
	if got.OpenAPIRef.FilePath != ref.FilePath {
		t.Errorf("OpenAPIRef.FilePath = %q, want %q", got.OpenAPIRef.FilePath, ref.FilePath)
	}
	if got.OpenAPIRef.Path != ref.Path {
		t.Errorf("OpenAPIRef.Path = %q, want %q", got.OpenAPIRef.Path, ref.Path)
	}
	if got.OpenAPIRef.Method != ref.Method {
		t.Errorf("OpenAPIRef.Method = %q, want %q", got.OpenAPIRef.Method, ref.Method)
	}
}

func TestOpenRequestFile_ServerURLFallback(t *testing.T) {
	ref := model.OpenAPIRef{
		FilePath: "testdata/minimal.yml",
		Path:     "/items",
		Method:   "POST",
	}

	// ServerURL not in spec servers list
	original := fullRequest()
	original.OpenAPIRef = &model.OpenAPIRef{
		FilePath: ref.FilePath,
		Path:     ref.Path,
		Method:   ref.Method,
	}
	original.ServerURL = "https://stale.example.com"

	cmd := SaveTempFile(original)
	msg := cmd()
	if msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}
	defer func() { _ = os.RemoveAll(tempDirForFile(ref.FilePath)) }()

	loadCmd := OpenRequestFile(ref)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenRequestFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	// Should fall back to first spec server
	if got.ServerURL != "https://api.example.com/v1" {
		t.Errorf("ServerURL = %q, want %q (fallback to spec default)", got.ServerURL, "https://api.example.com/v1")
	}
}

func TestOpenRequestFile_NoTempFile_FallsBackToSpec(t *testing.T) {
	ref := model.OpenAPIRef{
		FilePath: "testdata/minimal.yml",
		Path:     "/items",
		Method:   "POST",
	}

	// Ensure no temp file exists
	_ = os.RemoveAll(tempDirForFile(ref.FilePath))

	loadCmd := OpenRequestFile(ref)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenRequestFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	if got.URI != "/items" {
		t.Errorf("URI = %q, want %q (from spec)", got.URI, "/items")
	}
	if got.Method != model.POST {
		t.Errorf("Method = %v, want %v", got.Method, model.POST)
	}
	if len(got.Servers) == 0 || got.Servers[0] != "https://api.example.com/v1" {
		t.Errorf("Servers = %v, want [https://api.example.com/v1] (from spec)", got.Servers)
	}
	if got.OpenAPIRef == nil {
		t.Error("OpenAPIRef should be set from OperationToRequest")
	}
}

// --- OpenDraftFile tests ---

func TestOpenDraftFile_RoundtripAllFields(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yml")
	specContent := `openapi: "3.0.0"
info:
  title: Test
  version: 1.0.0
paths: {}
servers:
  - url: https://api.example.com/v1
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	draftPath := NewDraftPath(specFile)
	original := fullRequest()
	original.FileName = specFile
	original.DraftPath = draftPath
	original.OpenAPIRef = nil

	// Save draft
	saveCmd := SaveTempFile(original)
	if msg := saveCmd(); msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}
	defer func() { _ = os.RemoveAll(tempDirForFile(specFile)) }()

	// Load draft
	loadCmd := OpenDraftFile(draftPath, specFile)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenDraftFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	if got.URI != original.URI {
		t.Errorf("URI = %q, want %q", got.URI, original.URI)
	}
	if got.Method != original.Method {
		t.Errorf("Method = %v, want %v", got.Method, original.Method)
	}
	if got.About.Summary != original.About.Summary {
		t.Errorf("Summary = %q, want %q", got.About.Summary, original.About.Summary)
	}
	if got.Body.Raw != original.Body.Raw {
		t.Errorf("Body = %q, want %q", got.Body.Raw, original.Body.Raw)
	}
	if len(got.Headers) != len(original.Headers) {
		t.Errorf("Headers count = %d, want %d", len(got.Headers), len(original.Headers))
	}
	if len(got.Auth) != len(original.Auth) {
		t.Errorf("Auth count = %d, want %d", len(got.Auth), len(original.Auth))
	}
	if len(got.Servers) == 0 {
		t.Error("Servers should be loaded from spec")
	}
	if got.DraftPath != draftPath {
		t.Errorf("DraftPath = %q, want %q", got.DraftPath, draftPath)
	}
}

func TestOpenDraftFile_PreservesServerURL(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yml")
	specContent := `openapi: "3.0.0"
info:
  title: Test
  version: 1.0.0
paths: {}
servers:
  - url: https://api.example.com/v1
  - url: https://staging.example.com
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	draftPath := NewDraftPath(specFile)
	original := model.Request{
		FileName:  specFile,
		DraftPath: draftPath,
		URI:       "/items",
		Method:    model.GET,
		ServerURL: "https://staging.example.com",
	}

	saveCmd := SaveTempFile(original)
	if msg := saveCmd(); msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}
	defer func() { _ = os.RemoveAll(tempDirForFile(specFile)) }()

	loadCmd := OpenDraftFile(draftPath, specFile)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenDraftFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	if got.ServerURL != "https://staging.example.com" {
		t.Errorf("ServerURL = %q, want %q (preserved user selection)", got.ServerURL, "https://staging.example.com")
	}
	if len(got.Servers) != 2 {
		t.Errorf("Servers count = %d, want 2", len(got.Servers))
	}
}

func TestOpenDraftFile_FallbackWhenServerURLStale(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yml")
	specContent := `openapi: "3.0.0"
info:
  title: Test
  version: 1.0.0
paths: {}
servers:
  - url: https://api.example.com/v1
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	draftPath := NewDraftPath(specFile)
	original := model.Request{
		FileName:  specFile,
		DraftPath: draftPath,
		URI:       "/items",
		Method:    model.GET,
		ServerURL: "https://stale.example.com",
	}

	saveCmd := SaveTempFile(original)
	if msg := saveCmd(); msg != nil {
		t.Fatalf("SaveTempFile returned msg: %v", msg)
	}
	defer func() { _ = os.RemoveAll(tempDirForFile(specFile)) }()

	loadCmd := OpenDraftFile(draftPath, specFile)
	loadMsg := loadCmd()
	loaded, ok := loadMsg.(LoadedFile)
	if !ok {
		t.Fatalf("OpenDraftFile returned %T, want LoadedFile", loadMsg)
	}

	got := loaded.Data

	if got.ServerURL != "https://api.example.com/v1" {
		t.Errorf("ServerURL = %q, want %q (fallback to default)", got.ServerURL, "https://api.example.com/v1")
	}
}

// --- LoadForDuplicate tests ---

func TestLoadForDuplicate_DraftPreservesServers(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.yml")
	specContent := `openapi: "3.0.0"
info:
  title: Test
  version: 1.0.0
paths: {}
servers:
  - url: https://api.example.com/v1
  - url: https://staging.example.com
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	draftDir := tempDirForFile(specFile)
	if err := os.MkdirAll(draftDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(draftDir) }()

	draftFile := filepath.Join(draftDir, "draft.new.1")
	draftReq := model.Request{
		URI:       "/items",
		Method:    model.POST,
		ServerURL: "https://staging.example.com",
	}
	draftData, err := yaml.Marshal(draftReq)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draftFile, draftData, 0644); err != nil {
		t.Fatal(err)
	}

	item := requests.RequestItem{
		URI:       "/items",
		Method:    model.POST,
		DraftPath: draftFile,
		FileName:  specFile,
	}

	cmd := LoadForDuplicate(item)
	msg := cmd()

	dup, ok := msg.(DuplicateData)
	if !ok {
		t.Fatalf("LoadForDuplicate returned %T, want DuplicateData", msg)
	}

	if len(dup.Data.Servers) != 2 {
		t.Errorf("Servers count = %d, want 2", len(dup.Data.Servers))
	}
	if dup.Data.ServerURL != "https://staging.example.com" {
		t.Errorf("ServerURL = %q, want %q (preserved)", dup.Data.ServerURL, "https://staging.example.com")
	}
}

func TestLoadForDuplicate_OpenAPIRefPreservesServers(t *testing.T) {
	item := requests.RequestItem{
		URI:    "/items",
		Method: model.POST,
		OpenAPIRef: &model.OpenAPIRef{
			FilePath: "testdata/minimal.yml",
			Path:     "/items",
			Method:   "POST",
		},
		FileName: "testdata/minimal.yml",
	}

	cmd := LoadForDuplicate(item)
	msg := cmd()

	dup, ok := msg.(DuplicateData)
	if !ok {
		t.Fatalf("LoadForDuplicate returned %T, want DuplicateData", msg)
	}

	if len(dup.Data.Servers) == 0 {
		t.Error("Servers should be populated from spec")
	}
	if dup.Data.Servers[0] != "https://api.example.com/v1" {
		t.Errorf("Servers[0] = %q, want %q", dup.Data.Servers[0], "https://api.example.com/v1")
	}
	if dup.Data.ServerURL != "https://api.example.com/v1" {
		t.Errorf("ServerURL = %q, want %q", dup.Data.ServerURL, "https://api.example.com/v1")
	}
}
