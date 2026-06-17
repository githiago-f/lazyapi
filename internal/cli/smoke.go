package cli

import (
	"fmt"
	"os"
)

func SmokeTests(args []string) {
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
		fmt.Fprintln(os.Stderr, "Usage: lazyapi smoke tests <file> [--server url] [--env file]")
		os.Exit(1)
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File %q not found\n", file)
		os.Exit(1)
	}

	fmt.Println("Smoke tests are not implemented yet")
	fmt.Printf("  File:     %s\n", file)
	if server != "" {
		fmt.Printf("  Server:   %s\n", server)
	}
	if envFile != "" {
		fmt.Printf("  Env file: %s\n", envFile)
	}
}
