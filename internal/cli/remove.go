package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <file> <method> <path>",
		Short: "Remove a request from a spec file",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			return removeRequest(args[0], args[1], args[2])
		},
	}
}

func removeRequest(filePath, method, path string) error {
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

	ref := model.OpenAPIRef{
		FilePath: filePath,
		Path:     path,
		Method:   strings.ToUpper(method),
	}

	if err := store.RemoveOperationFromSpec(spec, ref); err != nil {
		return fmt.Errorf("error removing operation: %w", err)
	}

	if err := store.SaveSpec(filePath, spec); err != nil {
		return fmt.Errorf("error saving file: %w", err)
	}

	fmt.Printf("Removed %s %s from %s\n", method, path, filePath)
	return nil
}
