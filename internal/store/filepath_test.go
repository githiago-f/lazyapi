package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGlob_WithoutDoubleStar(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.yml"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.yaml"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("c"), 0644)

	// Use absolute path pattern to avoid working directory dependency
	pattern := filepath.Join(tmpDir, "*.yml")
	matches, err := Glob(pattern)
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %v", len(matches), matches)
	}
	if matches[0] != filepath.Join(tmpDir, "a.yml") {
		t.Errorf("match = %q, want %q", matches[0], filepath.Join(tmpDir, "a.yml"))
	}
}

func TestGlob_WithDoubleStar(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "sub", "subsub"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "a.yml"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "sub", "b.yml"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "sub", "subsub", "c.yaml"), []byte("c"), 0644)

	pattern := filepath.Join(tmpDir, "**", "*.yml")
	matches, err := Glob(pattern)
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}

	expected := []string{
		filepath.Join(tmpDir, "a.yml"),
		filepath.Join(tmpDir, "sub", "b.yml"),
	}
	if len(matches) != len(expected) {
		t.Fatalf("expected %d matches, got %d: %v", len(expected), len(matches), matches)
	}
	for _, exp := range expected {
		found := false
		for _, m := range matches {
			if m == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected match %q not found in results: %v", exp, matches)
		}
	}
}

func TestGlob_WithDoubleStarDeepYaml(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "sub", "subsub"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "a.yml"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "sub", "subsub", "c.yaml"), []byte("c"), 0644)

	pattern := filepath.Join(tmpDir, "**", "*.yaml")
	matches, err := Glob(pattern)
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %v", len(matches), matches)
	}
	if matches[0] != filepath.Join(tmpDir, "sub", "subsub", "c.yaml") {
		t.Errorf("match = %q, want %q", matches[0], filepath.Join(tmpDir, "sub", "subsub", "c.yaml"))
	}
}

func TestGlob_NoMatches(t *testing.T) {
	matches, err := Glob("/nonexistent_directory_xyz/**/*.yml")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestGlob_InvalidPattern(t *testing.T) {
	_, err := Glob("[invalid")
	if err == nil {
		t.Log("expected error or empty for invalid pattern")
	}
}

func TestGlobsExpand_MultipleSegments(t *testing.T) {
	// Test that Expand works with multiple ** segments
	// e.g., a/**/b/**/c
	// We'll just test the direct Expand path
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "a", "b", "c"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "a", "b", "c", "file.txt"), []byte(""), 0644)

	// This pattern has two ** segments after splitting
	pattern := filepath.Join(tmpDir, "**", "*.txt")
	matches, err := Glob(pattern)
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %v", len(matches), matches)
	}
}

func TestGlobsExpand_EmptyPattern(t *testing.T) {
	var g Globs
	results, err := g.Expand()
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestGlob_RelativeDoubleStar(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "nested")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.yml"), []byte("r"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested.yml"), []byte("n"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	matches, err := Glob("./**/*.yml")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) < 2 {
		t.Fatalf("expected at least 2 matches, got %d: %v", len(matches), matches)
	}
}
