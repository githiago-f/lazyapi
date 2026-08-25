# `lazyapi send request`

Execute a request directly from the command line using an OpenAPI spec.

## Usage

```bash
lazyapi send request <file> <path> <method> [--server <url>] [--env <file>] [--save-example] [--script <file>]
```

## Arguments

| Argument | Description |
|----------|-------------|
| `file` | Path to an OpenAPI YAML file |
| `path` | URI path |
| `method` | HTTP method |

## Flags

| Flag | Description |
|------|-------------|
| `--server <url>` | Override the server URL |
| `--env <file>` | Path to a `.env` file for `{{ '{{' }}env.X{{ '}}' }}` resolution |
| `--save-example` | Save the response as an OpenAPI example in the spec |
| `--script <file>` | Path to a Lua test script (see [Lua Test Scripts](../tui/lua-tests.md)) |

## Examples

```bash
# Basic request
lazyapi send request my-api.yml /users GET

# With server override
lazyapi send request my-api.yml /users GET --server http://localhost:3000

# With environment variables
lazyapi send request my-api.yml /users POST --env .env

# Save response as example
lazyapi send request my-api.yml /users/{id} GET --save-example

# Run Lua test script against response
lazyapi send request my-api.yml /users GET --script tests.lua
```

## Output

The response status, headers, and body are printed to stdout. If `--script` is provided, test results are printed after the response.
