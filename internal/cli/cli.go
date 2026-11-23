package cli

import (
	"fmt"
	"os"

	"github.com/githiago-f/lazyapi/internal/script"
)

// Commands lists the first words that select CLI mode instead of the TUI.
var Commands = []string{"create", "remove", "add", "send", "smoke", "run", "list", "describe", "validate", "types", "fake", "mcp", "mock", "import", "help"}

// Run executes a CLI command and exits with its status code
// (0 ok, 1 a check failed, 2 usage error, 3 something could not run).
func Run(args []string) {
	if err := run(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitCode(err))
	}
}

func run(args []string) error {
	if len(args) < 1 {
		printUsage()
		return usageErr("no command specified")
	}

	sub := func(name string) bool { return len(args) >= 2 && args[1] == name }

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return nil

	case "create":
		if !sub("file") {
			return usageErr("usage: lazyapi create file [filename] [servers...]")
		}
		filename := "openapi.yml"
		servers := []string{}
		if len(args) > 2 {
			filename = args[2]
			servers = args[3:]
		}
		return CreateFile(filename, servers)

	case "remove":
		if len(args) < 5 || !sub("request") {
			return usageErr("usage: lazyapi remove request <file> <method> <path>")
		}
		return RemoveRequest(args[2], args[3], args[4])

	case "add":
		switch {
		case sub("request") && len(args) >= 5:
			return AddRequest(args[2], args[3], args[4])
		case sub("server") && len(args) >= 4:
			return AddServer(args[2], args[3])
		default:
			return usageErr("usage: lazyapi add request <file> <path> <method> | lazyapi add server <file> <url>")
		}

	case "send":
		if !sub("request") {
			return usageErr("invalid send command\n%s", sendUsage)
		}
		return SendRequest(args[2:])

	case "run":
		return RunTests(args[1:])

	case "smoke":
		if len(args) < 3 || !sub("tests") {
			return usageErr("invalid smoke command (smoke tests is an alias of run)\n%s", runUsage)
		}
		return RunTests(args[2:])

	case "list":
		if !sub("ops") {
			return usageErr("%s", listUsage)
		}
		return ListOps(args[2:])

	case "describe":
		return Describe(args[1:])

	case "validate":
		if !sub("spec") {
			return usageErr("%s", validateUsage)
		}
		return ValidateSpec(args[2:])

	case "types":
		fmt.Print(script.Types)
		return nil

	case "fake":
		return ListFake(args[1:])

	case "mcp":
		return ServeMCP(args[1:])

	case "mock":
		return ServeMock(args[1:])

	case "import":
		return Import(args[1:])

	default:
		printUsage()
		return usageErr("unknown command: %s", args[0])
	}
}

func printUsage() {
	fmt.Println(`LazyAPI - OpenAPI-driven API exploration and testing

Usage:
  lazyapi [spec.yml] [--env file]                   Start the TUI (interactive mode)

  Testing
  lazyapi run <spec.yml> [flags]                    Run operations, contract checks and test scripts
  lazyapi send request <spec.yml> <path> <method>   Send one operation (or: --op <operationId>)
  lazyapi smoke tests <spec.yml> [flags]            Alias of run

  Inspecting
  lazyapi list ops <spec.yml> [--json]              List operations
  lazyapi describe <spec.yml> <operation> [--json]  Parameters, bodies, responses and schemas of an operation
  lazyapi validate spec <spec.yml> [--json]         Validate the spec and its test scripts
  lazyapi types                                     Print the test script API (TypeScript definitions)
  lazyapi fake [name] [--json]                      List the {{$dynamic}} variables (faker), or generate one

  Agents and local development
  lazyapi mcp [--root dir]                          Serve the tools above over MCP (stdio)
  lazyapi mock <spec.yml> [--port 4010]             Fake API from the spec's examples (validates requests)

  Editing
  lazyapi import curl|postman|har <source> --into <spec.yml>   Add operations from curl, Postman or a HAR capture
  lazyapi create file [name] [urls...]              Create a new OpenAPI template file
  lazyapi add request <file> <path> <method>        Add a new request to a spec file
  lazyapi add server <file> <url>                   Add a server URL to a spec file
  lazyapi remove request <file> <method> <path>     Remove a request from a spec file

Run "lazyapi run" or "lazyapi send request" without arguments for their flags.
Common flags: --server, --env, --param k=v, --query k=v, --header k=v, --timeout,
--script, --no-contract, --output pretty|json|junit|tap, --output-file, --dry-run.

Test scripts are JavaScript/TypeScript files next to the spec, referenced per
operation by x-lazyapi-tests. They may import relative files and https://
modules (pinned in lazyapi.lock). Responses are validated against the OpenAPI
schema automatically.

Exit status: 0 all passed, 1 a check failed, 2 usage error, 3 something could not run.`)
}
