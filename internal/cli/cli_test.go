package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut
	defer func() { os.Stdout = oldStdout }()

	cmd.SetArgs(args)
	err := cmd.Execute()

	_ = wOut.Close()
	out, _ := io.ReadAll(rOut)

	return string(out) + buf.String(), err
}

func requireFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file %q to exist", path)
	}
}

// --- create ---

func TestCreateCmd(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "myapi.yml")

	out, err := runCmd(t, "create", spec)
	if err != nil {
		t.Fatal(err)
	}
	requireFile(t, spec)
	if !strings.Contains(out, "Created") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestCreateCmd_WithServers(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "myapi.yml")

	_, err := runCmd(t, "create", spec, "http://localhost:8080", "https://api.example.com")
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "http://localhost:8080") {
		t.Error("expected first server URL in spec")
	}
	if !strings.Contains(string(data), "https://api.example.com") {
		t.Error("expected second server URL in spec")
	}
}

func TestCreateCmd_FileExists(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "myapi.yml")

	_, err := runCmd(t, "create", spec)
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "create", spec)
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", out)
	}
}

func TestCreateCmd_DefaultName(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	out, err := runCmd(t, "create")
	if err != nil {
		t.Fatal(err)
	}
	requireFile(t, filepath.Join(dir, "openapi.yml"))
	if !strings.Contains(out, "openapi.yml") {
		t.Errorf("expected default filename in output, got: %s", out)
	}
}

// --- add request ---

func TestAddRequestCmd(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "add", "request", spec, "/users", "POST")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "POST /users") {
		t.Errorf("expected success message, got: %s", out)
	}

	raw, _ := os.ReadFile(spec)
	if !strings.Contains(string(raw), "/users") {
		t.Error("expected /users path in spec")
	}
}

func TestAddRequestCmd_FileNotFound(t *testing.T) {
	out, err := runCmd(t, "add", "request", "/no/such/file.yml", "/x", "GET")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

// --- add server ---

func TestAddServerCmd(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "add", "server", spec, "http://localhost:9000")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "http://localhost:9000") {
		t.Errorf("expected server URL in output, got: %s", out)
	}
}

func TestAddServerCmd_Duplicate(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, []string{"http://localhost:9000"}); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "add", "server", spec, "http://localhost:9000")
	if err == nil {
		t.Fatal("expected error for duplicate server")
	}
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", out)
	}
}

// --- rm ---

func TestRemoveCmd(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}
	if err := addRequest(spec, "/items", "DELETE"); err != nil {
		t.Fatal(err)
	}

	debugRaw, _ := os.ReadFile(spec)
	t.Logf("spec content before remove:\n%s", debugRaw)

	out, err := runCmd(t, "rm", spec, "DELETE", "/items")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/items") {
		t.Errorf("expected path in output, got: %s", out)
	}

	raw, _ := os.ReadFile(spec)
	if strings.Contains(string(raw), "/items") {
		t.Error("expected /items path to be removed from spec")
	}
}

func TestRemoveCmd_NotFound(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "rm", spec, "GET", "/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing operation")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

// --- send ---

func TestSendCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}
	if err := addRequest(spec, "/test", "GET"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "send", spec, "/test", "GET", "--server", ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "200 OK") {
		t.Errorf("expected '200 OK' in output, got: %s", out)
	}
	if !strings.Contains(out, `{"status":"ok"}`) {
		t.Errorf("expected response body in output, got: %s", out)
	}
}

func TestSendCmd_ServerFlagWithIndex(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, []string{ts.URL}); err != nil {
		t.Fatal(err)
	}
	if err := addRequest(spec, "/test", "GET"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "send", spec, "/test", "GET", "--server", "0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "204 No Content") {
		t.Errorf("expected '204 No Content' in output, got: %s", out)
	}
}

func TestSendCmd_NoServerURL(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}
	if err := addRequest(spec, "/test", "GET"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "send", spec, "/test", "GET")
	if err == nil {
		t.Fatal("expected error for missing server URL")
	}
	if !strings.Contains(out, "server URL") {
		t.Errorf("expected 'server URL' error, got: %s", out)
	}
}

func TestSendCmd_SaveExample(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer ts.Close()

	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}
	if err := addRequest(spec, "/items", "POST"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "send", spec, "/items", "POST", "--server", ts.URL, "--save-example")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Example saved") {
		t.Errorf("expected 'Example saved' in output, got: %s", out)
	}
}

func TestSendCmd_OperationNotFound(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "send", spec, "/missing", "GET", "--server", "http://localhost:9999")
	if err == nil {
		t.Fatal("expected error for missing operation")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

// --- smoke ---

func TestSmokeCmd(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "smoke", spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "not implemented") {
		t.Errorf("expected 'not implemented' message, got: %s", out)
	}
}

func TestSmokeCmd_FileNotFound(t *testing.T) {
	out, err := runCmd(t, "smoke", "/no/such/file.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' error, got: %s", out)
	}
}

func TestSmokeCmd_WithFlags(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yml")

	if err := createFile(spec, nil); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(dir, "test.env")
	if err := os.WriteFile(envFile, []byte("TEST_KEY=value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "smoke", spec, "--server", "http://localhost:8080", "--env", envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "http://localhost:8080") {
		t.Errorf("expected server URL in output, got: %s", out)
	}
	if !strings.Contains(out, "test.env") {
		t.Errorf("expected env file in output, got: %s", out)
	}
}

// --- help ---

func TestHelp_Root(t *testing.T) {
	out, err := runCmd(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "lazyapi") {
		t.Errorf("expected command name in help, got: %s", out)
	}
	if !strings.Contains(out, "create") {
		t.Errorf("expected 'create' in help, got: %s", out)
	}
	if !strings.Contains(out, "rm") {
		t.Errorf("expected 'rm' in help, got: %s", out)
	}
	if !strings.Contains(out, "add") {
		t.Errorf("expected 'add' in help, got: %s", out)
	}
	if !strings.Contains(out, "send") {
		t.Errorf("expected 'send' in help, got: %s", out)
	}
	if !strings.Contains(out, "smoke") {
		t.Errorf("expected 'smoke' in help, got: %s", out)
	}
	if !strings.Contains(out, "TUI") {
		t.Errorf("expected TUI section in help, got: %s", out)
	}
}

func TestHelp_Subcommands(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"create", "--help"}, want: "create"},
		{args: []string{"rm", "--help"}, want: "rm"},
		{args: []string{"add", "--help"}, want: "add"},
		{args: []string{"add", "request", "--help"}, want: "request"},
		{args: []string{"add", "server", "--help"}, want: "server"},
		{args: []string{"send", "--help"}, want: "send"},
		{args: []string{"smoke", "--help"}, want: "smoke"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, "_"), func(t *testing.T) {
			out, err := runCmd(t, tt.args...)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("expected help to contain %q, got: %s", tt.want, out)
			}
		})
	}
}

// --- error on missing positional args ---

func TestMissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"remove_no_args", []string{"rm"}},
		{"remove_partial", []string{"rm", "file.yml"}},
		{"add_request_no_args", []string{"add", "request"}},
		{"add_request_partial", []string{"add", "request", "file.yml"}},
		{"add_server_no_args", []string{"add", "server"}},
		{"add_server_partial", []string{"add", "server", "file.yml"}},
		{"send_no_args", []string{"send"}},
		{"send_partial", []string{"send", "file.yml"}},
		{"smoke_no_args", []string{"smoke"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runCmd(t, tt.args...)
			if err == nil {
				t.Errorf("expected error for args %v", tt.args)
			}
		})
	}
}
