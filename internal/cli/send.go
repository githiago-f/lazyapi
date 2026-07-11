package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

func SendRequest(args []string) error {
	var file, path, method, serverURL, envFile string
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
		return fmt.Errorf("usage: lazyapi send request <file> <path> <method> [--server url] [--env file] [--save-example]")
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
