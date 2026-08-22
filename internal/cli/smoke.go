package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/lua"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
	"gopkg.in/yaml.v3"
)

func SmokeTests(args []string) error {
	var (
		serverURL  string
		envFile    string
		scriptFile string
		file       string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--server":
			i++
			if i < len(args) {
				serverURL = args[i]
			}
		case "--env":
			i++
			if i < len(args) {
				envFile = args[i]
			}
		case "--script":
			i++
			if i < len(args) {
				scriptFile = args[i]
			}
		default:
			if file == "" {
				file = args[i]
			}
		}
	}

	if file == "" {
		return fmt.Errorf("usage: lazyapi smoke tests <file> [--server url] [--env file] [--script file]")
	}

	spec, err := store.ParseSpec(file)
	if err != nil {
		return fmt.Errorf("error parsing spec: %w", err)
	}

	envStore := env.NewStore(envFile)
	envMap, err := envStore.Load()
	if err != nil {
		return fmt.Errorf("error loading env file: %w", err)
	}

	var sharedScript string
	if scriptFile != "" {
		data, readErr := os.ReadFile(scriptFile)
		if readErr != nil {
			return fmt.Errorf("error reading script file: %w", readErr)
		}
		sharedScript = string(data)
	}

	ops := store.ListOperations(spec, file)
	if len(ops) == 0 {
		fmt.Println("No operations found in spec")
		return nil
	}

	var totalPass, totalFail, totalErrors int

	for _, op := range ops {
		ref := model.OpenAPIRef{
			FilePath: file,
			Path:     op.URI,
			Method:   op.Method.Label(),
		}

		req := store.OperationToRequest(spec, ref)
		fillEmptyPathParams(req.Params)

		if serverURL != "" {
			req.ServerURL = serverURL
		} else if len(req.Servers) > 0 {
			req.ServerURL = req.Servers[0]
		}

		if req.ServerURL == "" {
			fmt.Printf("SKIP  %s %s — no server URL\n", op.Method.Label(), op.URI)
			continue
		}

		req.Env = envMap

		testScript := sharedScript
		if testScript == "" {
			if tempReq := loadTempRequest(ref); tempReq != nil && tempReq.Tests != "" {
				testScript = tempReq.Tests
			}
		}

		response, body, err := req.Send()
		if err != nil {
			totalErrors++
			fmt.Printf("ERR   %s %s — %s\n\n", op.Method.Label(), op.URI, err)
			continue
		}
		defer func() { _ = response.Body.Close() }()

		fmt.Printf("%s %s — Status %d\n", op.Method.Label(), op.URI, response.StatusCode)

		if testScript != "" {
			execData := lua.ExecData{
				Request:        req,
				Env:            envMap,
				HasResponse:    true,
				RespStatus:     response.StatusCode,
				RespStatusText: response.Status,
				RespHeaders:    response.Header,
				RespBody:       body,
			}

			result := lua.RunScript(testScript, execData)

			for _, tr := range result.Results {
				mark := "PASS"
				if !tr.Passed {
					mark = "FAIL"
					totalFail++
				} else {
					totalPass++
				}
				fmt.Printf("  [%s] %s\n", mark, tr.Name)
			}

			if result.ScriptError != "" {
				totalErrors++
				fmt.Printf("  [ERR]  Script error: %s\n", result.ScriptError)
			}

			fmt.Printf("  %d passed, %d failed\n\n", result.PassCount, result.FailCount)
		} else {
			fmt.Println("  (no test script — use --script or define tests in the editor)")
		}
	}

	fmt.Println("========================================")
	fmt.Printf("Total: %d passed, %d failed, %d errors\n", totalPass, totalFail, totalErrors)

	return nil
}

func fillEmptyPathParams(params map[string]string) {
	for k, v := range params {
		if v == "" {
			params[k] = "1"
		}
	}
}

func loadTempRequest(ref model.OpenAPIRef) *model.Request {
	abs, err := filepath.Abs(ref.FilePath)
	if err != nil {
		abs = ref.FilePath
	}
	safe := sanitizePath(abs)
	dir := filepath.Join(os.TempDir(), "lazyapi", safe)
	safeRef := sanitizePath(ref.Path)
	tempPath := filepath.Join(dir, fmt.Sprintf("tmp.%s.%s", ref.Method, safeRef))

	data, err := os.ReadFile(tempPath)
	if err != nil {
		return nil
	}
	var req model.Request
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil
	}
	return &req
}

func sanitizePath(path string) string {
	r := strings.NewReplacer("/", "_", "{", "_", "}", "_", " ", "_")
	return r.Replace(path)
}
