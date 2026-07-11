package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "my-api.yml")

	err := CreateFile(path, []string{"https://api.example.com"})
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "openapi:") {
		t.Errorf("file should contain 'openapi:', got: %s", content)
	}
	if !strings.Contains(content, "https://api.example.com") {
		t.Errorf("file should contain server URL, got: %s", content)
	}
}

func TestCreateFile_NoServers(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no-servers.yml")

	err := CreateFile(path, nil)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if strings.Contains(string(data), "servers") {
		t.Error("should not have servers section when none specified")
	}
}

func TestCreateFile_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "existing.yml")
	os.WriteFile(path, []byte("existing"), 0644)

	err := CreateFile(path, nil)
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists' message", err.Error())
	}
}

func TestAddRequest_Success(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "spec.yml")
	if err := CreateFile(specPath, nil); err != nil {
		t.Fatal(err)
	}

	err := AddRequest(specPath, "/pets", "GET")
	if err != nil {
		t.Fatalf("AddRequest: %v", err)
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/pets") {
		t.Errorf("spec should contain /pets, got: %s", string(data))
	}
}

func TestAddRequest_FileNotFound(t *testing.T) {
	err := AddRequest("/nonexistent/file.yml", "/test", "GET")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestAddRequest_NotOpenAPI(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "not-openapi.yml")
	os.WriteFile(path, []byte("name: test\n"), 0644)

	err := AddRequest(path, "/test", "GET")
	if err == nil {
		t.Fatal("expected error for non-OpenAPI file")
	}
}

func TestAddRequest_InvalidMethod(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "spec.yml")
	if err := CreateFile(specPath, nil); err != nil {
		t.Fatal(err)
	}

	err := AddRequest(specPath, "/test", "")
	if err == nil {
		t.Fatal("expected error for empty method")
	}
}

func TestAddServer_Success(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "spec.yml")
	if err := CreateFile(specPath, nil); err != nil {
		t.Fatal(err)
	}

	err := AddServer(specPath, "https://api.example.com")
	if err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "https://api.example.com") {
		t.Errorf("spec should contain server URL, got: %s", string(data))
	}
}

func TestAddServer_FileNotFound(t *testing.T) {
	err := AddServer("/nonexistent/file.yml", "https://api.example.com")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestAddServer_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "spec.yml")
	if err := CreateFile(specPath, []string{"https://api.example.com"}); err != nil {
		t.Fatal(err)
	}

	err := AddServer(specPath, "https://api.example.com")
	if err == nil {
		t.Fatal("expected error for duplicate server URL")
	}
}

func TestRemoveRequest_Success(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "spec.yml")
	if err := CreateFile(specPath, nil); err != nil {
		t.Fatal(err)
	}
	if err := AddRequest(specPath, "/pets", "GET"); err != nil {
		t.Fatal(err)
	}

	err := RemoveRequest(specPath, "GET", "/pets")
	if err != nil {
		t.Fatalf("RemoveRequest: %v", err)
	}
}

func TestRemoveRequest_FileNotFound(t *testing.T) {
	err := RemoveRequest("/nonexistent/file.yml", "GET", "/test")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestRemoveRequest_NotOpenAPI(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "not-openapi.yml")
	os.WriteFile(path, []byte("name: test\n"), 0644)

	err := RemoveRequest(path, "GET", "/test")
	if err == nil {
		t.Fatal("expected error for non-OpenAPI file")
	}
}

func TestRemoveRequest_OperationNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "spec.yml")
	if err := CreateFile(specPath, nil); err != nil {
		t.Fatal(err)
	}

	err := RemoveRequest(specPath, "GET", "/nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent operation")
	}
}

func TestSmokeTests_Stub(t *testing.T) {
	// SmokeTests is a stub - just verify it doesn't crash
	err := SmokeTests([]string{"test-file.yml"})
	if err != nil {
		t.Fatalf("SmokeTests: %v", err)
	}
}

func TestSmokeTests_NoFile(t *testing.T) {
	err := SmokeTests([]string{})
	if err == nil {
		t.Fatal("expected error when no file specified")
	}
}

func TestSmokeTests_WithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	os.WriteFile(envFile, []byte("TEST=val\n"), 0644)

	err := SmokeTests([]string{"test-file.yml", "--server", "https://example.com", "--env", envFile})
	if err != nil {
		t.Fatalf("SmokeTests: %v", err)
	}
}

func TestRun_Create(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "run-test.yml")

	err := run([]string{"create", "file", path, "https://api.example.com"})
	if err != nil {
		t.Fatalf("run create: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file was not created")
	}
}

func TestRun_AddRequest(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "run-test.yml")
	if err := CreateFile(path, nil); err != nil {
		t.Fatal(err)
	}

	err := run([]string{"add", "request", path, "/users", "POST"})
	if err != nil {
		t.Fatalf("run add request: %v", err)
	}
}

func TestRun_AddServer(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "run-test.yml")
	if err := CreateFile(path, nil); err != nil {
		t.Fatal(err)
	}

	err := run([]string{"add", "server", path, "https://api.example.com"})
	if err != nil {
		t.Fatalf("run add server: %v", err)
	}
}

func TestRun_RemoveRequest(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "run-test.yml")
	if err := CreateFile(path, nil); err != nil {
		t.Fatal(err)
	}
	if err := AddRequest(path, "/users", "POST"); err != nil {
		t.Fatal(err)
	}

	err := run([]string{"remove", "request", path, "POST", "/users"})
	if err != nil {
		t.Fatalf("run remove request: %v", err)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestRun_NoArgs(t *testing.T) {
	err := run([]string{})
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestRun_CreateInvalid(t *testing.T) {
	err := run([]string{"create"})
	if err == nil {
		t.Fatal("expected error for incomplete create command")
	}
}

func TestRun_AddInvalid(t *testing.T) {
	err := run([]string{"add"})
	if err == nil {
		t.Fatal("expected error for incomplete add command")
	}
}

func TestRun_RemoveInvalid(t *testing.T) {
	err := run([]string{"remove"})
	if err == nil {
		t.Fatal("expected error for incomplete remove command")
	}
}

func TestRun_SendInvalid(t *testing.T) {
	err := run([]string{"send"})
	if err == nil {
		t.Fatal("expected error for incomplete send command")
	}
}

func TestRun_SmokeInvalid(t *testing.T) {
	err := run([]string{"smoke"})
	if err == nil {
		t.Fatal("expected error for incomplete smoke command")
	}
}

func TestCreateFile_DefaultName(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// CreateFile with default name (called from Run, not directly)
	err = CreateFile("openapi.yml", nil)
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if _, err := os.Stat("openapi.yml"); os.IsNotExist(err) {
		t.Error("default openapi.yml was not created")
	}
}
