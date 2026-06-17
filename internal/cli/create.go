package cli

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func CreateFile(filename string, servers []string) {
	if _, err := os.Stat(filename); err == nil {
		fmt.Fprintf(os.Stderr, "File %q already exists\n", filename)
		os.Exit(1)
	}

	data := map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":       "API",
			"description": "API description",
			"version":     "1.0.0",
		},
		"paths": map[string]any{},
	}

	if len(servers) > 0 {
		serverList := make([]map[string]any, len(servers))
		for i, s := range servers {
			serverList[i] = map[string]any{"url": s}
		}
		data["servers"] = serverList
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating template: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(filename, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created OpenAPI template: %s\n", filename)
}
