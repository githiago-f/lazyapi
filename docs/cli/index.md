# CLI: Command-Line Interface

LazyAPI's subcommands run headless, for scripting, CI/CD and AI agents.

```bash
lazyapi <command> [arguments...] [flags]
```

Flags may appear before or after positional arguments.

## Commands

| Command | Description |
|---------|-------------|
| [`run`](/cli/run) | Run operations with status, contract and script checks; outputs pretty, JSON, JUnit or TAP |
| [`send request`](/cli/send) | Send one operation (`<path> <method>` or `--op <operationId>`) |
| [`smoke tests`](/cli/smoke) | Alias of `run` |
| [`list ops`](/cli/inspect#list-ops) | List operations |
| [`describe`](/cli/inspect#describe) | Parameters, bodies, responses and schemas of one operation |
| [`validate spec`](/cli/inspect#validate-spec) | Validate a spec and its test scripts |
| `types` | Print the test script API ([Test Scripts](/scripting)) |
| [`mcp`](/agents) | Serve these tools to AI agents over MCP |
| [`mock`](/cli/mock) | Fake API from the spec's examples, with request validation |
| [`create file`](/cli/create) | Create a new OpenAPI spec template |
| [`add request`](/cli/add) / [`add server`](/cli/add) | Add an operation or server |
| [`remove request`](/cli/remove) | Remove an operation |

## Exit status

The same codes apply to every command:

| Code | Meaning |
|------|---------|
| 0 | Everything passed |
| 1 | A check failed: invalid request, undeclared status, contract violation, test failure, script error, or an invalid spec (`validate`) |
| 2 | Usage error (unknown flag, missing argument) |
| 3 | Something could not run: spec unreadable, network error, missing server, invalid env file |

When a run has both failures and errors, the code is 3.
