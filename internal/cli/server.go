package cli

import (
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/githiago-f/lazyapi/internal/store"
)

func AddServer(filePath, url string) error {
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

	for _, s := range spec.Servers {
		if s.URL == url {
			return fmt.Errorf("server URL %q already exists in %s", url, filePath)
		}
	}

	spec.Servers = append(spec.Servers, &openapi3.Server{URL: url})

	if err := store.SaveSpec(filePath, spec); err != nil {
		return fmt.Errorf("error saving file: %w", err)
	}

	fmt.Printf("Added server %s to %s\n", url, filePath)
	return nil
}
