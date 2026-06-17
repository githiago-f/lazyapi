package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

func RemoveRequest(filePath, method, path string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File %q not found\n", filePath)
		os.Exit(1)
	}

	if !store.IsOpenAPIFile(filePath) {
		fmt.Fprintf(os.Stderr, "%q is not a valid OpenAPI file\n", filePath)
		os.Exit(1)
	}

	spec, err := store.ParseSpec(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing spec: %v\n", err)
		os.Exit(1)
	}

	ref := model.OpenAPIRef{
		FilePath: filePath,
		Path:     path,
		Method:   strings.ToUpper(method),
	}

	if err := store.RemoveOperationFromSpec(spec, ref); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing operation: %v\n", err)
		os.Exit(1)
	}

	if err := store.SaveSpec(filePath, spec); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed %s %s from %s\n", method, path, filePath)
}
