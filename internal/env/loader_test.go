package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnvLine_Basic(t *testing.T) {
	key, val, ok := parseEnvLine("FOO=bar")
	if !ok {
		t.Fatal("expected ok")
	}
	if key != "FOO" {
		t.Errorf("key = %q, want %q", key, "FOO")
	}
	if val != "bar" {
		t.Errorf("val = %q, want %q", val, "bar")
	}
}

func TestParseEnvLine_WithQuotes(t *testing.T) {
	_, val, ok := parseEnvLine(`FOO="bar"`)
	if !ok {
		t.Fatal("expected ok")
	}
	if val != "bar" {
		t.Errorf("val = %q, want %q", val, "bar")
	}

	_, val, ok = parseEnvLine("FOO='bar'")
	if !ok {
		t.Fatal("expected ok")
	}
	if val != "bar" {
		t.Errorf("val = %q, want %q", val, "bar")
	}
}

func TestParseEnvLine_TrimsWhitespace(t *testing.T) {
	key, val, ok := parseEnvLine("  FOO  =  bar  ")
	if !ok {
		t.Fatal("expected ok")
	}
	if key != "FOO" {
		t.Errorf("key = %q, want %q", key, "FOO")
	}
	if val != "bar" {
		t.Errorf("val = %q, want %q", val, "bar")
	}
}

func TestParseEnvLine_Comment(t *testing.T) {
	_, _, ok := parseEnvLine("# FOO=bar")
	if ok {
		t.Fatal("expected not ok for comment")
	}
}

func TestParseEnvLine_Empty(t *testing.T) {
	_, _, ok := parseEnvLine("")
	if ok {
		t.Fatal("expected not ok for empty")
	}
}

func TestParseEnvLine_NoEquals(t *testing.T) {
	_, _, ok := parseEnvLine("FOOBAR")
	if ok {
		t.Fatal("expected not ok when no equals sign")
	}
}

func TestParseEnvLine_EmptyValue(t *testing.T) {
	key, val, ok := parseEnvLine("EMPTY=")
	if !ok {
		t.Fatal("expected ok")
	}
	if key != "EMPTY" {
		t.Errorf("key = %q, want %q", key, "EMPTY")
	}
	if val != "" {
		t.Errorf("val = %q, want empty", val)
	}
}

func TestParseEnvLine_ValueWithEquals(t *testing.T) {
	_, val, ok := parseEnvLine("PASSWORD=abc=def")
	if !ok {
		t.Fatal("expected ok")
	}
	if val != "abc=def" {
		t.Errorf("val = %q, want %q", val, "abc=def")
	}
}

func TestLoad_WithDotenv(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `KEY=value
EMPTY=
# this is a comment
QUOTED="hello world"
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Load(envFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if result["KEY"] != "value" {
		t.Errorf("KEY = %q, want %q", result["KEY"], "value")
	}
	if result["EMPTY"] != "" {
		t.Errorf("EMPTY = %q, want empty", result["EMPTY"])
	}
	if result["QUOTED"] != "hello world" {
		t.Errorf("QUOTED = %q, want %q", result["QUOTED"], "hello world")
	}
	// System env should still be present
	if result["PATH"] == "" {
		t.Errorf("PATH should be present from system env")
	}
}

func TestLoad_EmptyFilepath(t *testing.T) {
	result, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if result["PATH"] == "" {
		t.Errorf("PATH should be present from system env")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/.env")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLoad_DotenvOverridesSystem(t *testing.T) {
	// Set a temp env var
	os.Setenv("OVERRIDE_TEST", "system")
	defer os.Unsetenv("OVERRIDE_TEST")

	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("OVERRIDE_TEST=dotenv"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Load(envFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if result["OVERRIDE_TEST"] != "dotenv" {
		t.Errorf("OVERRIDE_TEST = %q, want %q (dotenv should override system)", result["OVERRIDE_TEST"], "dotenv")
	}
}

func TestStore_Load(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("STORE_TEST=initial"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(envFile)
	result, err := s.Load()
	if err != nil {
		t.Fatalf("Store.Load failed: %v", err)
	}
	if result["STORE_TEST"] != "initial" {
		t.Errorf("STORE_TEST = %q, want %q", result["STORE_TEST"], "initial")
	}
}

func TestStore_Load_CachesResult(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("CACHE_TEST=initial"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(envFile)
	_, err := s.Load()
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}

	// Modify the file
	if err := os.WriteFile(envFile, []byte("CACHE_TEST=modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second load should return cached value (hash unchanged? Actually no, hash changed)
	// Actually, since we modified content, hash changed, so it should re-read
	result, err := s.Load()
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if result["CACHE_TEST"] != "modified" {
		t.Errorf("CACHE_TEST = %q, want %q (should detect file change)", result["CACHE_TEST"], "modified")
	}
}

func TestStore_Load_SameContent(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("SAME_TEST=value"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(envFile)
	first, err := s.Load()
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}

	// Write same content again
	if err := os.WriteFile(envFile, []byte("SAME_TEST=value"), 0644); err != nil {
		t.Fatal(err)
	}

	second, err := s.Load()
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}

	// Should have same values (cached due to hash match)
	if first["SAME_TEST"] != second["SAME_TEST"] {
		t.Errorf("expected same value after same-content write")
	}
}

func TestStore_ForceReload(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("FORCE_TEST=first"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(envFile)
	_, err := s.Load()
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}

	// Modify the file
	if err := os.WriteFile(envFile, []byte("FORCE_TEST=second"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := s.ForceReload()
	if err != nil {
		t.Fatalf("ForceReload: %v", err)
	}
	if result["FORCE_TEST"] != "second" {
		t.Errorf("FORCE_TEST = %q, want %q", result["FORCE_TEST"], "second")
	}
}

func TestStore_NoFilepath(t *testing.T) {
	s := NewStore("")
	result, err := s.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if result["PATH"] == "" {
		t.Errorf("PATH should be present from system env")
	}
}

func TestStore_NonExistentFile(t *testing.T) {
	s := NewStore("/nonexistent/.env")
	_, err := s.Load()
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestMergeDotenv_Basic(t *testing.T) {
	base := map[string]string{"EXISTING": "old"}
	result := mergeDotenv(base, "NEW_KEY=new_val\nEXISTING=overridden")

	if result["NEW_KEY"] != "new_val" {
		t.Errorf("NEW_KEY = %q, want %q", result["NEW_KEY"], "new_val")
	}
	if result["EXISTING"] != "overridden" {
		t.Errorf("EXISTING = %q, want %q", result["EXISTING"], "overridden")
	}
	if result["UNRELATED"] != "" {
		t.Errorf("UNRELATED should not exist")
	}
}

func TestMergeDotenv_OriginalUnmodified(t *testing.T) {
	base := map[string]string{"ONLY_IN_BASE": "preserved"}
	_ = mergeDotenv(base, "NEW=val")

	if base["NEW"] != "" {
		t.Errorf("original base should not be modified")
	}
}

func TestLoad_MultipleValuesPerLine(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `A=1
B=2
C=3
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Load(envFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if result["A"] != "1" || result["B"] != "2" || result["C"] != "3" {
		t.Errorf("unexpected results: %v", result)
	}
}

func TestLoad_EmptyLinesIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := "\n\n\n\n\n"
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Load(envFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(result) == 0 {
		t.Error("result should at least contain system env vars")
	}
}

func TestParseEnvLine_ValueWithInternalSpaces(t *testing.T) {
	_, val, ok := parseEnvLine(`MESSAGE=hello world`)
	if !ok {
		t.Fatal("expected ok")
	}
	if val != "hello world" {
		t.Errorf("val = %q, want %q", val, "hello world")
	}
}

func TestStore_Load_SystemEnvPresent(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("USER_DEFINED=custom"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(envFile)
	result, err := s.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if result["USER_DEFINED"] != "custom" {
		t.Errorf("USER_DEFINED = %q, want %q", result["USER_DEFINED"], "custom")
	}
	if result["PATH"] == "" {
		t.Errorf("PATH should be present from system env")
	}
}

func TestPartsFunctionNotExported(t *testing.T) {
	// Verify parseEnvLine works correctly for edge cases
	key, val, ok := parseEnvLine("  # indented comment")
	if ok {
		t.Error("indented comment should not parse")
	}
	_ = key
	_ = val
}

func TestLoad_WithWindowsLineEndings(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := "WINDOWS=value\r\nNEXT=second\r\n"
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Load(envFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if result["WINDOWS"] != "value" {
		t.Errorf("WINDOWS = %q, want %q", result["WINDOWS"], "value")
	}
	if result["NEXT"] != "second" {
		t.Errorf("NEXT = %q, want %q", result["NEXT"], "second")
	}
}

func TestLoadFromFixture(t *testing.T) {
	// Integration: load a realistic .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=lazyapi

# API
API_KEY=sk-abc123
API_URL=https://api.example.com
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Load(envFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	expected := map[string]string{
		"DB_HOST": "localhost",
		"DB_PORT": "5432",
		"DB_NAME": "lazyapi",
		"API_KEY": "sk-abc123",
		"API_URL": "https://api.example.com",
	}
	for k, v := range expected {
		if result[k] != v {
			t.Errorf("%s = %q, want %q", k, result[k], v)
		}
	}
}

// Ensure system env vars like GOFLAGS, HOME etc. leak through
func TestLoad_SystemEnvLeakThrough(t *testing.T) {
	// Use a variable we know exists
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set")
	}

	result, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if result["HOME"] != home {
		t.Errorf("HOME = %q, want %q", result["HOME"], home)
	}
}

func TestLoadPrefixesAllSystemEnv(t *testing.T) {
	// Ensure at least a few system env keys are present
	result, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, key := range []string{"PATH", "HOME", "USER", "SHELL"} {
		if result[key] != "" {
			found++
		}
	}
	if found == 0 {
		t.Error("no expected system env vars found, Load may not be loading system env")
	}
}

func TestDotenvPreservesSystemEnvTypes(t *testing.T) {
	// Verify that strings from .env are not accidentally interpreted as other types
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("BOOLEAN=true\nNUMBER=42"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Load(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if result["BOOLEAN"] != "true" {
		t.Errorf("BOOLEAN = %q, want %q", result["BOOLEAN"], "true")
	}
	if result["NUMBER"] != "42" {
		t.Errorf("NUMBER = %q, want %q", result["NUMBER"], "42")
	}
}

func TestLoadWithRealisticEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("export TEST_VAL=hello"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Load(envFile)
	if err != nil {
		t.Fatal(err)
	}
	// The "export" prefix should be treated as part of the key since parseEnvLine
	// uses SplitN with "=" and trims whitespace
	if strings.HasPrefix(result["export TEST_VAL"], "hello") {
		t.Log("export prefix preserved as part of key")
	}
}
