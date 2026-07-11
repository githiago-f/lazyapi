package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/app/pane/requests"
	"github.com/githiago-f/lazyapi/internal/model"
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
	os.RemoveAll(dir)
}

func TestNewDraftPath_Increments(t *testing.T) {
	p1 := NewDraftPath("test-spec.yml")
	p2 := NewDraftPath("test-spec.yml")
	if p1 == p2 {
		t.Errorf("expected different draft paths, got same: %s", p1)
	}
	dir := filepath.Dir(p1)
	os.RemoveAll(dir)
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
	defer os.Remove(tempFile)

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
	os.MkdirAll(draftDir, 0755)

	draftContent := "uri: /items\nmethod: get\nabout:\n  summary: Test\n"
	draftFile := filepath.Join(draftDir, "draft.new.1")
	if err := os.WriteFile(draftFile, []byte(draftContent), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(draftDir)

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
	os.MkdirAll(draftDir, 0755)
	defer os.RemoveAll(draftDir)

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
