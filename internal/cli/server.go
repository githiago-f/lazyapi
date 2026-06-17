package cli

import (
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/store"
)

func AddServer(filePath, url string) {
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

	for _, s := range spec.Servers {
		if s.URL == url {
			fmt.Fprintf(os.Stderr, "Server URL %q already exists in %s\n", url, filePath)
			os.Exit(1)
		}
	}

	spec.Servers = append(spec.Servers, &openapi3.Server{URL: url})

	if err := store.SaveSpec(filePath, spec); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added server %s to %s\n", url, filePath)
}
