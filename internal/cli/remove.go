package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

func RemoveRequest(filePath, method, path string) error {
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

	ref := model.OpenAPIRef{
		FilePath: filePath,
		Path:     path,
		Method:   methodUpper,
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
