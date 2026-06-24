package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add resources to an OpenAPI spec",
	}
	cmd.AddCommand(newAddRequestCmd())
	cmd.AddCommand(newAddServerCmd())
	return cmd
}

func newAddRequestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "request <file> <path> <method>",
		Short: "Add a new request to a spec file",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			return addRequest(args[0], args[1], args[2])
		},
	}
}

func newAddServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server <file> <url>",
		Short: "Add a server URL to a spec file",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return addServer(args[0], args[1])
		},
	}
}

func addRequest(filePath, path, method string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file %q not found", filePath)
	}

	if !store.IsOpenAPIFile(filePath) {
		return fmt.Errorf("%q is not a valid OpenAPI file", filePath)
	}

	spec, err := store.ParseSpec(filePath)
	if err != nil {
		return fmt.Errorf("error parsing spec: %w", err)
	}

	methodUpper := strings.ToUpper(method)
	req := model.Request{
		URI: path,
	}

	if err := store.AddOperationToSpec(spec, path, methodUpper, req); err != nil {
		return fmt.Errorf("error adding operation: %w", err)
	}

	if err := store.SaveSpec(filePath, spec); err != nil {
		return fmt.Errorf("error saving file: %w", err)
	}

	fmt.Printf("Added %s %s to %s\n", methodUpper, path, filePath)
	return nil
}
