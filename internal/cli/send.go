package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

func SendRequest(args []string) {
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
		fmt.Fprintln(os.Stderr, "Usage: lazyapi send request <file> <path> <method> [--server url] [--env file] [--save-example]")
		os.Exit(1)
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File %q not found\n", file)
		os.Exit(1)
	}

	if !store.IsOpenAPIFile(file) {
		fmt.Fprintf(os.Stderr, "%q is not a valid OpenAPI file\n", file)
		os.Exit(1)
	}

	spec, err := store.ParseSpec(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing spec: %v\n", err)
		os.Exit(1)
	}

	methodUpper := strings.ToUpper(method)
	ref := model.OpenAPIRef{
		FilePath: file,
		Path:     path,
		Method:   methodUpper,
	}

	req := store.OperationToRequest(spec, ref)
	if req.URI == "" {
		fmt.Fprintf(os.Stderr, "Operation %s %s not found in %s\n", methodUpper, path, file)
		os.Exit(1)
	}

	if serverURL != "" {
		if idx, err := strconv.Atoi(serverURL); err == nil {
			if idx < 0 || idx >= len(req.Servers) {
				fmt.Fprintf(os.Stderr, "Server index %d out of range. Available servers:\n", idx)
				for i, s := range req.Servers {
					fmt.Fprintf(os.Stderr, "  %d: %s\n", i, s)
				}
				os.Exit(1)
			}
			req.ServerURL = req.Servers[idx]
		} else {
			req.ServerURL = serverURL
		}
	} else if len(req.Servers) > 0 {
		req.ServerURL = req.Servers[0]
	}

	if req.ServerURL == "" {
		fmt.Fprintln(os.Stderr, "No server URL available. Specify one with --server <url>")
		os.Exit(1)
	}

	fmt.Printf("Server: %s\n", req.ServerURL)

	envStore := env.NewStore(envFile)
	envMap, err := envStore.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading env file: %v\n", err)
		os.Exit(1)
	}
	req.Env = envMap

	response, body, err := req.Send()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer response.Body.Close()

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
			fmt.Fprintf(os.Stderr, "Error saving example: %v\n", err)
			os.Exit(1)
		}
		if err := store.SaveSpec(file, spec); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing spec: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\n✓ Example saved to spec")
	}
}
