package cli

import (
	"fmt"

	"github.com/githiago-f/lazyapi/internal/env"
)

func SmokeTests(args []string) error {
	var server, envFile string
	var file string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--server":
			i++
			if i < len(args) {
				server = args[i]
			}
		case "--env":
			i++
			if i < len(args) {
				envFile = args[i]
			}
		default:
			if file == "" {
				file = args[i]
			}
		}
	}

	if file == "" {
		return fmt.Errorf("usage: lazyapi smoke tests <file> [--server url] [--env file]")
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
