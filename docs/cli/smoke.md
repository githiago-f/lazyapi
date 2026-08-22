# `lazyapi smoke tests`

Run automated smoke tests against all operations in a spec. Optionally include a Lua script to perform deeper assertions on each response.

## Usage

```bash
lazyapi smoke tests <file> [--server <url>] [--env <file>] [--script <file>]
```

## Arguments

| Argument | Description |
|----------|-------------|
| `file` | Path to an OpenAPI YAML file |

## Flags

| Flag | Description |
|------|-------------|
| `--server <url>` | Override the server URL for all requests |
| `--env <file>` | Path to a `.env` file |
| `--script <file>` | Path to a Lua test script (see [Lua Test Scripts](../tui/lua-tests.md)) |

## How it works

1. Loads the OpenAPI spec and enumerates all operations (paths × methods).
2. For each operation, sends an HTTP request to the combined server+path URL.
3. Prints each operation's status code and pass/fail status.
4. If `--script` is provided, the Lua script is executed against each response. Test results from the script are shown alongside the request status.
5. Returns a non-zero exit code if any operation fails or any script assertion fails — suitable for CI/CD.
6. If any operation lacks an example request body, an empty body is sent (servers typically handle this gracefully).

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All operations responded successfully |
| 1 | One or more operations failed |

## Example

```bash
lazyapi smoke tests openapi.yml --server https://api.example.com --script tests.lua
```
