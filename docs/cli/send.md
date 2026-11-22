# `lazyapi send request`

Send a single operation from an OpenAPI spec and run its tests.

## Usage

```bash
lazyapi send request <file> <path> <method> [flags]
```

Flags may appear before, between, or after the positional arguments.

## Arguments

| Argument | Description |
|----------|-------------|
| `file` | Path to an OpenAPI YAML file |
| `path` | Path as written in the spec, e.g. `/users/{id}` |
| `method` | HTTP method (case-insensitive) |

## Flags

| Flag | Description |
|------|-------------|
| `--server <url\|index>` | Server URL, or index into the spec's `servers` (default: first server) |
| `--env <file>` | Path to a `.env` file for `{{env.X}}` resolution |
| `--param k=v` | Path parameter (repeatable). Defaults to the spec's example |
| `--query k=v` | Query parameter (repeatable) |
| `--header k=v` | Header (repeatable) |
| `--script <file>` | Test script; overrides the operation's `x-lazyapi-tests` file |
| `--timeout <duration>` | Request timeout (default `30s`) |
| `--save-example` | Save the response as an OpenAPI example in the spec |
| `--no-contract` | Skip validating the response against the OpenAPI schema |
| `--script-timeout <duration>` | Maximum script run time (default `30s`) |
| `--offline` / `--reload` / `--frozen-lockfile` / `--allow-import <hosts>` | URL import behavior, see [Test Scripts](../scripting.md#imports) |

If a path parameter has neither a `--param` value nor an example in the spec, the command fails and does not send anything.

## Examples

```bash
lazyapi send request my-api.yml /users GET
lazyapi send request my-api.yml /users GET --server http://localhost:3000
lazyapi send request my-api.yml /users/{id} GET --param id=42 --header "X-Trace=abc"
lazyapi send request my-api.yml /users POST --env .env --script tests/create_user.js
lazyapi send request my-api.yml /users/{id} GET --param id=1 --save-example
```

## Output and exit code

The status (with round-trip time), headers and body are printed to stdout. Then come the contract check (when the response has a declared schema) and the script results. A warning is printed if the status code is not declared in the spec. The command exits with status 1 if the request fails, the contract check fails, or a test fails.
