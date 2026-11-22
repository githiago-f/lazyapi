package cli

import (
	"fmt"
	"os"

	"github.com/githiago-f/lazyapi/internal/script"
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
			return fmt.Errorf("invalid send command\n%s", sendUsage)
		}
		return SendRequest(args[2:])

	case "smoke":
		if len(args) < 3 || args[1] != "tests" {
			return fmt.Errorf("invalid smoke command\n%s", smokeUsage)
		}
		return SmokeTests(args[2:])

	case "types":
		fmt.Print(script.Types)
		return nil

	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printUsage() {
	fmt.Println(`LazyAPI - OpenAPI-driven API exploration and testing

Usage:
  lazyapi [spec.yml] [--env file]                Start the TUI (interactive mode)
  lazyapi create file [name] [urls...]           Create a new OpenAPI template file
  lazyapi add request <file> <path> <method>     Add a new request to a spec file
  lazyapi add server <file> <url>                Add a server URL to a spec file
  lazyapi remove request <file> <method> <path>  Remove a request from a spec file
  lazyapi send request <file> <path> <method>    Send one request and run its tests
  lazyapi smoke tests <file>                     Run every selected operation and its tests
  lazyapi types                                  Print the test script API (TypeScript definitions)

Flags (send and smoke):
  --server url|index  Server URL, or index into the spec's servers (default: first)
  --env file          Environment file ({{env.NAME}} values)
  --script file       Test script (.js/.ts); overrides the operation's x-lazyapi-tests file
  --param k=v         Path parameter value (repeatable; default: spec example)
  --timeout 30s       Per-request timeout
  --script-timeout d  Maximum run time of a test script (default 30s)
  --no-contract       Do not validate responses against the OpenAPI schema
  --offline           Never download URL imports; use the module cache only
  --reload            Re-download URL imports and update lazyapi.lock
  --frozen-lockfile   Fail if a URL import is not pinned in lazyapi.lock (CI)
  --allow-import list Hosts URL imports may come from (default: any https host)

Flags (send only):
  --query k=v         Query parameter (repeatable)
  --header k=v        Header (repeatable)
  --save-example      Persist the response as an example in the spec

Flags (smoke only):
  --methods list      Methods to run (default GET,HEAD,OPTIONS; "all" for every method)
  --tag list          Only operations with one of these tags
  --op list           Only these operations (operationId or "METHOD /path")
  --dry-run           Print what would be sent without sending

Test scripts are JavaScript/TypeScript files stored next to the spec and
referenced per operation via x-lazyapi-tests (e.g. tests/listItems.js). They
may import relative files and https:// modules (pinned in lazyapi.lock).
Responses are validated against the OpenAPI schema automatically.
Commands exit with status 1 when a request fails, a status code is not
declared in the spec, the response breaks the schema, or a test fails.`)
}
