package cli

import (
	"fmt"
	"os"
)

func Run(args []string) {
	if err := run(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		printUsage()
		return fmt.Errorf("no command specified")
	}

	switch args[0] {
	case "create":
		if len(args) < 2 || args[1] != "file" {
			fmt.Println("Usage: lazyapi create file [filename] [servers...]")
			return fmt.Errorf("invalid create command")
		}
		filename := "openapi.yml"
		servers := []string{}
		if len(args) > 2 {
			filename = args[2]
			servers = args[3:]
		}
		return CreateFile(filename, servers)

	case "remove":
		if len(args) < 5 || args[1] != "request" {
			fmt.Println("Usage: lazyapi remove request <file> <method> <path>")
			return fmt.Errorf("invalid remove command")
		}
		return RemoveRequest(args[2], args[3], args[4])

	case "add":
		if len(args) < 2 {
			fmt.Println("Usage: lazyapi add request|server ...")
			return fmt.Errorf("invalid add command")
		}
		switch args[1] {
		case "request":
			if len(args) < 5 {
				fmt.Println("Usage: lazyapi add request <file> <path> <method>")
				return fmt.Errorf("invalid add request command")
			}
			return AddRequest(args[2], args[3], args[4])
		case "server":
			if len(args) < 4 {
				fmt.Println("Usage: lazyapi add server <file> <url>")
				return fmt.Errorf("invalid add server command")
			}
			return AddServer(args[2], args[3])
		default:
			fmt.Println("Usage: lazyapi add request|server ...")
			return fmt.Errorf("invalid add subcommand: %s", args[1])
		}

	case "send":
		if len(args) < 2 || args[1] != "request" {
			fmt.Println("Usage: lazyapi send request <file> <path> <method> [--server url]")
			return fmt.Errorf("invalid send command")
		}
		return SendRequest(args[2:])

	case "smoke":
		if len(args) < 3 || args[1] != "tests" {
			fmt.Println("Usage: lazyapi smoke tests <file> [--server url] [--env file]")
			return fmt.Errorf("invalid smoke command")
		}
		return SmokeTests(args[2:])

	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printUsage() {
	fmt.Println(`LazyAPI - OpenAPI-driven API exploration and testing

Usage:
  lazyapi                                Start the TUI (interactive mode)
  lazyapi create file [name] [urls...]   Create a new OpenAPI template file
  lazyapi remove request <file> <method> <path>  Remove a request from a spec file
  lazyapi add request <file> <path> <method>     Add a new request to a spec file
  lazyapi add server <file> <url>        Add a server URL to a spec file
  lazyapi send request <file> <path> <method> [--server url] [--env file]  Send an HTTP request
  lazyapi smoke tests <file> [flags]     Run smoke tests (not yet implemented)

Flags:
  --server url   Base server URL for smoke tests and send request
  --save-example Persist the response as an example in the OpenAPI spec
  --env file     Environment file for smoke tests`)
}
