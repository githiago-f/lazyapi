package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githiago-f/lazyapi/internal/model"
)

// copyFixture copies a testdata spec into a temp dir so it can be mutated.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(fixturePath(name))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestSaveOperationTests_Roundtrip(t *testing.T) {
	specPath := copyFixture(t, "minimal.yml")
	ref := model.OpenAPIRef{FilePath: specPath, Path: "/items/{id}", Method: "GET"}
	script := `test("ok", () => expect(response.status).toBe(200));`

	spec, err := ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveOperationTests(spec, specPath, ref, script); err != nil {
		t.Fatalf("SaveOperationTests: %v", err)
	}
	if err := SaveSpec(specPath, spec); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(filepath.Dir(specPath), ".lazyapi", "tests", "get_items_id.js")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("script file not written: %v", err)
	}
	if string(data) != script {
		t.Errorf("script file = %q, want %q", data, script)
	}

	raw, _ := os.ReadFile(specPath)
	if !strings.Contains(string(raw), TestsExtension+": .lazyapi/tests/get_items_id.js") {
		t.Errorf("spec does not reference script:\n%s", raw)
	}

	reloaded, err := ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	req := OperationToRequest(reloaded, ref)
	if req.Tests != script {
		t.Errorf("OperationToRequest Tests = %q, want %q", req.Tests, script)
	}
}

func TestSaveOperationTests_UsesOperationID(t *testing.T) {
	specPath := copyFixture(t, "minimal.yml")
	spec, err := ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	ref := model.OpenAPIRef{FilePath: specPath, Path: "/items", Method: "POST"}
	spec.Paths.Find("/items").Post.OperationID = "createItem"

	if err := SaveOperationTests(spec, specPath, ref, "-- t"); err != nil {
		t.Fatal(err)
	}
	if got := spec.Paths.Find("/items").Post.Extensions[TestsExtension]; got != ".lazyapi/tests/createItem.js" {
		t.Errorf("extension = %v, want tests/createItem.js", got)
	}
}

func TestSaveOperationTests_EmptyRemovesReference(t *testing.T) {
	specPath := copyFixture(t, "minimal.yml")
	spec, err := ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	ref := model.OpenAPIRef{FilePath: specPath, Path: "/items", Method: "GET"}
	if err := SaveOperationTests(spec, specPath, ref, "-- t"); err != nil {
		t.Fatal(err)
	}
	if err := SaveOperationTests(spec, specPath, ref, "   "); err != nil {
		t.Fatal(err)
	}
	if _, ok := spec.Paths.Find("/items").Get.Extensions[TestsExtension]; ok {
		t.Error("expected extension to be removed for empty script")
	}
}

func TestLoadOperationTests_RejectsEscapingPath(t *testing.T) {
	specPath := copyFixture(t, "minimal.yml")
	spec, err := ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	op := spec.Paths.Find("/items").Get
	for _, bad := range []string{"../outside.js", "/etc/passwd"} {
		op.Extensions = map[string]any{TestsExtension: bad}
		if _, err := LoadOperationTests(specPath, op); err == nil {
			t.Errorf("expected error for %q", bad)
		}
		ref := model.OpenAPIRef{FilePath: specPath, Path: "/items", Method: "GET"}
		if err := SaveOperationTests(spec, specPath, ref, "-- t"); err == nil {
			t.Errorf("expected save error for %q", bad)
		}
	}
}

func TestLoadOperationTests_NoReference(t *testing.T) {
	spec, err := ParseSpec(fixturePath("minimal.yml"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadOperationTests(fixturePath("minimal.yml"), spec.Paths.Find("/items").Get)
	if err != nil || got != "" {
		t.Errorf("got (%q, %v), want empty, nil", got, err)
	}
}

func TestLoadOperationTests_RejectsLua(t *testing.T) {
	specPath := copyFixture(t, "minimal.yml")
	spec, err := ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	op := spec.Paths.Find("/items").Get
	op.Extensions = map[string]any{TestsExtension: "tests/old.lua"}
	if _, err := LoadOperationTests(specPath, op); err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Errorf("expected Lua rejection, got %v", err)
	}

	// Saving migrates the reference to a .js file.
	ref := model.OpenAPIRef{FilePath: specPath, Path: "/items", Method: "GET"}
	if err := SaveOperationTests(spec, specPath, ref, "test('x', () => {});"); err != nil {
		t.Fatal(err)
	}
	if got := op.Extensions[TestsExtension]; got != ".lazyapi/tests/get_items.js" {
		t.Errorf("extension = %v, want tests/get_items.js", got)
	}
}

func TestOperationTestsPath(t *testing.T) {
	specPath := copyFixture(t, "minimal.yml")
	spec, err := ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	ref := model.OpenAPIRef{FilePath: specPath, Path: "/items/{id}", Method: "DELETE"}
	got, err := OperationTestsPath(specPath, spec.Paths.Find("/items/{id}").Delete, ref)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(specPath), ".lazyapi", "tests", "delete_items_id.js")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
