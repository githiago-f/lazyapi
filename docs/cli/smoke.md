# `lazyapi smoke tests`

Run automated smoke tests against all operations in a spec.

## Usage

```bash
lazyapi smoke tests <file> [--server <url>] [--env <file>]
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

## Status

**Not yet implemented.** This command is planned for a future release.

The goal is to iterate through every operation in the spec, send a request to each, and report which endpoints respond successfully and which fail — useful for CI/CD pipelines and release validation.
