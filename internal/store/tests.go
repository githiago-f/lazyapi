package store

import (
	"fmt"
	"github.com/githiago-f/lazyapi/internal/workspace"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/model"
)

// TestsExtension is the operation-level OpenAPI extension that points to the
// operation's test script, relative to the spec file's directory.
const TestsExtension = "x-lazyapi-tests"

// TestsDir is the directory (relative to the spec) where new scripts go.
const TestsDir = workspace.TestsDir

// TestScriptExt is the extension used for newly created test scripts.
const TestScriptExt = ".js"

// supportedScriptExts are the script types the engine can run.
var supportedScriptExts = []string{".js", ".mjs", ".ts", ".mts"}

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
	// Credentials and local state (secrets, OAuth sessions) are never scripts.
	slash := filepath.ToSlash(inside)
	for _, private := range []string{workspace.Dir + "/secrets", workspace.Dir + "/.local"} {
		if slash == private || strings.HasPrefix(slash, private+"/") {
			return "", fmt.Errorf("%s may not point into %s/: %q", TestsExtension, private, rel)
		}
	}
	return full, nil
}

// OperationTestsPath returns the absolute path of the operation's script:
// the x-lazyapi-tests file when set, otherwise where a new script would be
// created. It is used to resolve relative imports inside the script.
func OperationTestsPath(specPath string, op *openapi3.Operation, ref model.OpenAPIRef) (string, error) {
	rel := testsRef(op)
	if rel == "" {
		rel = TestsDir + "/" + TestScriptName(op, ref)
	}
	return resolveTestsPath(specPath, rel)
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
	if err := checkScriptExt(rel); err != nil {
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
	if rel == "" || strings.EqualFold(filepath.Ext(rel), ".lua") {
		// New script, or migrating away from a legacy Lua file.
		rel = TestsDir + "/" + TestScriptName(op, ref)
	}
	full, err := resolveTestsPath(specPath, rel)
	if err != nil {
		return err
	}
	if err := checkScriptExt(rel); err != nil {
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

// checkScriptExt rejects script types the engine cannot run.
func checkScriptExt(rel string) error {
	ext := strings.ToLower(filepath.Ext(rel))
	if ext == ".lua" {
		return fmt.Errorf("%s: Lua test scripts are no longer supported; rewrite it in JavaScript "+
			"(run `lazyapi types` for the script API) and point %s at the .js file", rel, TestsExtension)
	}
	if !slices.Contains(supportedScriptExts, ext) {
		return fmt.Errorf("%s: unsupported test script type %q (use %s)", rel, ext, strings.Join(supportedScriptExts, ", "))
	}
	return nil
}
