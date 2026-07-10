# CLI: Command-Line Interface

LazyAPI supports headless operations via subcommands. Use these for scripting, automation, and CI/CD.

```bash
./lazyapi <command> [arguments...]
```

## Commands

| Command | Description |
|---------|-------------|
| [`create file`](/cli/create) | Create a new OpenAPI spec template |
| [`add request`](/cli/add) | Add an operation to a spec |
| [`add server`](/cli/add) | Add a server URL to a spec |
| [`remove request`](/cli/remove) | Remove an operation from a spec |
| [`send request`](/cli/send) | Execute a request from a spec |
| [`smoke tests`](/cli/smoke) | Run smoke tests (planned) |

## General flags

| Flag | Description |
|------|-------------|
| `--server <url>` | Override the server URL (used with `send` and `smoke`) |
| `--env <file>` | Path to a `.env` file (used with `send` and `smoke`) |
| `--save-example` | Save the response as an OpenAPI example (used with `send`) |
