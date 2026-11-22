package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

// TestsExtension is the operation-level OpenAPI extension that points to the
// operation's test script, relative to the spec file's directory.
const TestsExtension = "x-lazyapi-tests"

// TestsDir is the directory (relative to the spec) where new scripts go.
const TestsDir = "tests"

// TestScriptExt is the extension used for newly created test scripts.
const TestScriptExt = ".lua"

var nonIdentChars = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// TestScriptName builds a stable file name for an operation's script:
// the operationId when present, otherwise "<method>_<path>".
func TestScriptName(op *openapi3.Operation, ref model.OpenAPIRef) string {
	base := ""
	if op != nil && op.OperationID != "" {
		base = op.OperationID
	} else {
		base = strings.ToLower(ref.Method) + "_" + ref.Path
	}
	base = strings.Trim(nonIdentChars.ReplaceAllString(base, "_"), "_")
	if base == "" {
		base = "operation"
	}
	return base + TestScriptExt
}

// testsRef returns the script path stored in the operation extension.
func testsRef(op *openapi3.Operation) string {
	if op == nil || op.Extensions == nil {
		return ""
	}
	v, _ := op.Extensions[TestsExtension].(string)
	return v
}

// resolveTestsPath turns a spec-relative script reference into an absolute
// path, refusing references that escape the spec directory.
func resolveTestsPath(specPath, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%s must be relative to the spec: %q", TestsExtension, rel)
	}
	specDir, err := filepath.Abs(filepath.Dir(specPath))
	if err != nil {
		return "", err
	}
	full := filepath.Join(specDir, filepath.FromSlash(rel))
	inside, err := filepath.Rel(specDir, full)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s escapes the spec directory: %q", TestsExtension, rel)
	}
	return full, nil
}

// LoadOperationTests reads the test script referenced by the operation.
// It returns "" with no error when the operation has no script.
func LoadOperationTests(specPath string, op *openapi3.Operation) (string, error) {
	rel := testsRef(op)
	if rel == "" {
		return "", nil
	}
	full, err := resolveTestsPath(specPath, rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("reading test script %s: %w", rel, err)
	}
	return string(data), nil
}

// SaveOperationTests writes script next to the spec and records its path in
// the operation's x-lazyapi-tests extension. An empty script removes the
// reference (the file itself is kept so no user work is deleted).
// The caller is responsible for persisting the spec afterwards.
func SaveOperationTests(spec *openapi3.T, specPath string, ref model.OpenAPIRef, script string) error {
	op := FindOperation(spec, ref)
	if op == nil {
		return fmt.Errorf("operation %s %s not found", ref.Method, ref.Path)
	}

	if strings.TrimSpace(script) == "" {
		delete(op.Extensions, TestsExtension)
		return nil
	}

	rel := testsRef(op)
	if rel == "" {
		rel = TestsDir + "/" + TestScriptName(op, ref)
	}
	full, err := resolveTestsPath(specPath, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("creating tests directory: %w", err)
	}
	if err := os.WriteFile(full, []byte(script), 0o644); err != nil {
		return fmt.Errorf("writing test script %s: %w", rel, err)
	}

	if op.Extensions == nil {
		op.Extensions = map[string]any{}
	}
	op.Extensions[TestsExtension] = rel
	return nil
}
