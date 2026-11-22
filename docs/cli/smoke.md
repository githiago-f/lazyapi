# `lazyapi smoke tests`

Run every selected operation in a spec, check the response status against the spec, and run each operation's test script.

## Usage

```bash
lazyapi smoke tests <file> [flags]
```

Flags may appear before or after `<file>`.

## Flags

| Flag | Description |
|------|-------------|
| `--server <url\|index>` | Server URL, or index into the spec's `servers` (default: first server) |
| `--env <file>` | Path to a `.env` file |
| `--script <file>` | Test script applied to every operation; overrides each operation's `x-lazyapi-tests` |
| `--methods <list>` | Methods to run. Default `GET,HEAD,OPTIONS`; use `all` to include mutating methods |
| `--tag <list>` | Only run operations that have one of these tags |
| `--op <list>` | Only run these operations, by `operationId` or `"METHOD /path"` |
| `--param k=v` | Path parameter value for every operation that has it (repeatable) |
| `--timeout <duration>` | Per-request timeout (default `30s`) |
| `--dry-run` | Print the requests that would be sent, without sending |

## How it works

1. Loads the spec and lists all operations, then applies `--op`, `--tag` and `--methods`.
2. Fills path parameters from `--param`, then from the spec (`example`, `examples`, schema `example`/`default`/`enum`). Operations that still have an empty path parameter are **skipped**; the tool never invents values.
3. Sends the request and checks the status code against the operation's declared `responses` (exact code, `2XX`-style ranges, or `default`). If the operation declares no responses, any status below 400 is accepted.
4. Runs the test script: `--script` if given, otherwise the file referenced by the operation's `x-lazyapi-tests`.
5. Prints a summary: passed, failed, errors, skipped.

Mutating methods (POST, PUT, PATCH, DELETE) are skipped by default and reported as `SKIP`, so running smoke against a shared environment is safe unless you pass `--methods`.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Every executed operation returned a declared status and all tests passed |
| 1 | A request failed, a status was not declared, a script errored, or a test failed |

## Examples

```bash
# Safe read-only pass over the whole API
lazyapi smoke tests openapi.yml --server https://staging.example.com

# Only the "users" tag, including writes, with a fixed id
lazyapi smoke tests openapi.yml --tag users --methods all --param id=42

# See what would be called
lazyapi smoke tests openapi.yml --methods all --dry-run
```
