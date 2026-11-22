package cli

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/lua"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

// selectServer applies the --server value to req. The value may be a URL or
// an index into the spec's servers list; empty falls back to the first server.
func selectServer(req *model.Request, value string) error {
	if value == "" {
		if len(req.Servers) > 0 {
			req.ServerURL = req.Servers[0]
		}
		return nil
	}
	if idx, err := strconv.Atoi(value); err == nil {
		if idx < 0 || idx >= len(req.Servers) {
			return fmt.Errorf("server index %d out of range (spec declares %d servers)", idx, len(req.Servers))
		}
		req.ServerURL = req.Servers[idx]
		return nil
	}
	req.ServerURL = value
	return nil
}

// applyOverrides copies name=value overrides into target. When onlyKnown is
// true, names not already present in target are ignored (used by smoke, where
// one --param applies to many operations).
func applyOverrides(target map[string]string, overrides kvFlag, onlyKnown bool) {
	for name, value := range overrides {
		if _, ok := target[name]; ok || !onlyKnown {
			target[name] = value
		}
	}
}

// readScriptFile returns the content of path, or "" when path is empty.
func readScriptFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error reading script file: %w", err)
	}
	return string(data), nil
}

// operationScript picks the test script for an operation: an explicit
// --script wins, otherwise the file referenced by x-lazyapi-tests.
func operationScript(override, specPath string, op *openapi3.Operation) (string, error) {
	if override != "" {
		return override, nil
	}
	return store.LoadOperationTests(specPath, op)
}

// runTests executes script against a received response and prints each
// result with an indent prefix.
func runTests(script string, req model.Request, envMap map[string]string, resp *http.Response, body, indent string) lua.ScriptResult {
	result := lua.RunScript(script, lua.ExecData{
		Request:        req,
		Env:            envMap,
		HasResponse:    true,
		RespStatus:     resp.StatusCode,
		RespStatusText: resp.Status,
		RespHeaders:    resp.Header,
		RespBody:       body,
	})
	for _, tr := range result.Results {
		mark := "PASS"
		if !tr.Passed {
			mark = "FAIL"
		}
		fmt.Printf("%s[%s] %s\n", indent, mark, tr.Name)
		if tr.Message != "" {
			fmt.Printf("%s       %s\n", indent, tr.Message)
		}
	}
	if result.ScriptError != "" {
		fmt.Printf("%s[ERR]  Script error: %s\n", indent, result.ScriptError)
	}
	return result
}
