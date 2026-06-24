package cli

import (
	"fmt"
	"os"

	"github.com/githiago-f/lazyapi/internal/env"
	"github.com/spf13/cobra"
)

func newSmokeCmd() *cobra.Command {
	var serverURL, envFile string

	cmd := &cobra.Command{
		Use:   "smoke <file>",
		Short: "Run smoke tests against an API",
		Long:  "Run smoke tests against an API (not yet implemented)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return smokeTests(args[0], serverURL, envFile)
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "Base server URL for smoke tests")
	cmd.Flags().StringVar(&envFile, "env", "", "Environment file for smoke tests")

	return cmd
}

func smokeTests(file, server, envFile string) error {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fmt.Errorf("file %q not found", file)
	}

	envStore := env.NewStore(envFile)

	envMap, err := envStore.Load()
	if err != nil {
		return fmt.Errorf("error loading env file: %w", err)
	}

	fmt.Println("Smoke tests are not implemented yet")
	fmt.Printf("  File:     %s\n", file)
	if server != "" {
		fmt.Printf("  Server:   %s\n", server)
	}
	if envFile != "" {
		fmt.Printf("  Env file: %s\n", envFile)
	}
	if envMap != nil {
		fmt.Println("  Env vars loaded with hash-based auto-reload")
	}
	return nil
}
