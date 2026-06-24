package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
	"github.com/spf13/cobra"
)

func newSendCmd() *cobra.Command {
	var serverURL, envFile string
	var saveExample bool

	cmd := &cobra.Command{
		Use:   "send <file> <path> <method>",
		Short: "Send an HTTP request defined in the spec",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			return sendRequest(args[0], args[1], args[2], serverURL, envFile, saveExample)
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "Base server URL (URL or index)")
	cmd.Flags().StringVar(&envFile, "env", "", "Environment file")
	cmd.Flags().BoolVar(&saveExample, "save-example", false, "Save response as an example in the spec")

	return cmd
}

func sendRequest(file, path, method, serverURL, envFile string, saveExample bool) error {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fmt.Errorf("file %q not found", file)
	}

	if !store.IsOpenAPIFile(file) {
		return fmt.Errorf("%q is not a valid OpenAPI file", file)
	}

	spec, err := store.ParseSpec(file)
	if err != nil {
		return fmt.Errorf("error parsing spec: %w", err)
	}

	methodUpper := strings.ToUpper(method)
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
		return fmt.Errorf("no server URL available; specify one with --server <url>")
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
