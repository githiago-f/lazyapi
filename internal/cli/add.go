package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

func AddRequest(filePath, path, method string) error {
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
	if methodUpper == "" {
		return fmt.Errorf("invalid HTTP method: %q", method)
	}

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
