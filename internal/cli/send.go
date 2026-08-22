package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/lua"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

func SendRequest(args []string) error {
	var file, path, method, serverURL, envFile, scriptFile string
	var saveExample bool

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
		case "--save-example":
			saveExample = true
		case "--script":
			i++
			if i < len(args) {
				scriptFile = args[i]
			}
		default:
			if file == "" {
				file = args[i]
			} else if path == "" {
				path = args[i]
			} else if method == "" {
				method = args[i]
			}
		}
	}

	if file == "" || path == "" || method == "" {
		return fmt.Errorf("usage: lazyapi send request <file> <path> <method> [--server url] [--env file] [--save-example] [--script file]")
	}

	spec, err := store.ParseSpec(file)
	if err != nil {
		return fmt.Errorf("error parsing spec: %w", err)
	}

	methodUpper := strings.ToUpper(method)
	if methodUpper == "" {
		return fmt.Errorf("invalid HTTP method: %q", method)
	}

	ref := model.OpenAPIRef{
		FilePath: file,
		Path:     path,
		Method:   methodUpper,
	}

	req := store.OperationToRequest(spec, ref)
	if req.URI == "" {
		return fmt.Errorf("operation %s %s not found in %s", methodUpper, path, file)
	}

	fillEmptyPathParams(req.Params)

	if serverURL != "" {
		if idx, err := strconv.Atoi(serverURL); err == nil {
			if idx < 0 || idx >= len(req.Servers) {
				return fmt.Errorf("server index %d out of range", idx)
			}
			req.ServerURL = req.Servers[idx]
		} else {
			req.ServerURL = serverURL
		}
	} else if len(req.Servers) > 0 {
		req.ServerURL = req.Servers[0]
	}

	if req.ServerURL == "" {
		return fmt.Errorf("no server URL available. Specify one with --server <url>")
	}

	fmt.Printf("Server: %s\n", req.ServerURL)

	envStore := env.NewStore(envFile)
	envMap, err := envStore.Load()
	if err != nil {
		return fmt.Errorf("error loading env file: %w", err)
	}
	req.Env = envMap

	// Try loading per-operation test script from temp file
	testScript := ""
	if tempReq := loadTempRequest(ref); tempReq != nil {
		testScript = tempReq.Tests
	}

	if scriptFile != "" {
		data, readErr := os.ReadFile(scriptFile)
		if readErr != nil {
			return fmt.Errorf("error reading script file: %w", readErr)
		}
		testScript = string(data)
	}

	response, body, err := req.Send()
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	fmt.Printf("Status: %s\n\n", response.Status)

	fmt.Println("--- Headers ---")
	for name, values := range response.Header {
		for _, v := range values {
			fmt.Printf("  %s: %s\n", name, v)
		}
	}

	fmt.Println("\n--- Body ---")
	fmt.Println(body)

	if testScript != "" {
		fmt.Println("\n--- Tests ---")
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
			}
			fmt.Printf("  [%s] %s\n", mark, tr.Name)
		}

		if result.ScriptError != "" {
			fmt.Printf("  [ERR]  Script error: %s\n", result.ScriptError)
		}

		fmt.Printf("\n%d passed, %d failed\n", result.PassCount, result.FailCount)
	}

	if saveExample {
		if err := store.SaveResponseExample(spec, ref, response.StatusCode, response.Header, body); err != nil {
			return fmt.Errorf("error saving example: %w", err)
		}
		if err := store.SaveSpec(file, spec); err != nil {
			return fmt.Errorf("error writing spec: %w", err)
		}
		fmt.Println("\n✓ Example saved to spec")
	}

	return nil
}
