# `lazyapi send request`

Execute a request directly from the command line using an OpenAPI spec.

## Usage

```bash
lazyapi send request <file> <path> <method> [--server <url>] [--env <file>] [--save-example]
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
| `--env <file>` | Path to a `.env` file for `&#123;&#123;env.X&#125;&#125;` resolution |
| `--save-example` | Save the response as an OpenAPI example in the spec |

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
```

## Output

The response status, headers, and body are printed to stdout.
