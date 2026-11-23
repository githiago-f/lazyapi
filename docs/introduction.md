# What is LazyAPI?

> OpenAPI-driven API exploration, testing and automation from the terminal, for humans and AI agents.

LazyAPI uses your OpenAPI spec as the single source of truth, so there are no collections to keep in sync. You can:

- explore the API in a terminal UI;
- send requests and see every response checked against the spec;
- write tests in JavaScript or TypeScript;
- run everything in CI;
- let an AI agent drive it through MCP or the CLI's JSON output.

```text
             ┌── TUI ─── explore, send, edit, debug                    (humans)
OpenAPI  ────┼── CLI ─── run · send · describe · validate              (CI, scripts)
+ .lazyapi/  └── MCP ─── list · describe · send · write_test · run     (agents)
                  │
                  └─ every response is checked: request · status · contract · your tests
```

## Why?

Most API tools make you recreate what is already in the OpenAPI document: endpoints, parameters, bodies, auth. Then the copy drifts from the spec. LazyAPI reads the spec directly, keeps tests and environments in a `.lazyapi/` directory next to it, and validates every exchange against it.

## Features

- **OpenAPI-first**: load any OpenAPI 3.x spec and browse its operations immediately.
- **Checked exchanges**: request, status, contract and your tests, on every send.
- **Interactive TUI**: request list grouped by tag, editor, response view with jq, history and diff, runner.
- **CLI for CI**: `run`, `send`, `describe`, `validate`, `mock`, `import`; JSON, JUnit and TAP reports.
- **Scripts**: JavaScript/TypeScript tests and flows, URL imports pinned in a lock file, `before`/`after` hooks, fake data.
- **Authentication**: Basic, Bearer, API key and OAuth2 (client credentials and password, tokens cached).
- **Environments**: named environments in `.lazyapi/config.yml`, variables committed, secrets git-ignored.
- **Agents**: an MCP server confined to a root directory.
- **Portable**: a single binary for Linux, macOS and Windows.

## Next steps

- [Install LazyAPI](/installation)
- [Getting started](/getting-started) with the bundled example and the mock server
- [The TUI](/tui/) and its [keyboard shortcuts](/tui/keyboard-shortcuts)
- [The CLI reference](/cli/)
