package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [name] [servers...]",
		Short: "Create a new OpenAPI template file",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			filename := "openapi.yml"
			servers := []string{}
			if len(args) > 0 {
				filename = args[0]
				servers = args[1:]
			}
			return createFile(filename, servers)
		},
	}
}

func createFile(filename string, servers []string) error {
	if _, err := os.Stat(filename); err == nil {
		return fmt.Errorf("file %q already exists", filename)
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
		return fmt.Errorf("error generating template: %w", err)
	}

	if err := os.WriteFile(filename, out, 0644); err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}

	fmt.Printf("Created OpenAPI template: %s\n", filename)
	return nil
}
