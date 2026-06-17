package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/githiago-f/lazyapi/internal/model"
	"github.com/githiago-f/lazyapi/internal/store"
)

func AddRequest(filePath, path, method string) {
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

	methodUpper := strings.ToUpper(method)
	req := model.Request{
		URI: path,
	}

	if err := store.AddOperationToSpec(spec, path, methodUpper, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error adding operation: %v\n", err)
		os.Exit(1)
	}

	if err := store.SaveSpec(filePath, spec); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added %s %s to %s\n", methodUpper, path, filePath)
}
