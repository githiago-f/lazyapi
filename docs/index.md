# LazyAPI

> OpenAPI-driven API exploration, testing, and automation from your terminal.

LazyAPI is a Terminal User Interface (TUI) that lets you explore, execute, and validate API operations directly from OpenAPI specifications. Instead of manually recreating collections and environments, LazyAPI uses your OpenAPI document as the source of truth.

## Features

- **OpenAPI-first** — load any OpenAPI 3.x spec and browse operations immediately
- **Interactive TUI** — explore endpoints, edit requests, send calls, view responses
- **CLI automation** — create specs, add endpoints, send requests from scripts
- **Authentication** — Basic, Bearer, API Key, and OAuth2 support
- **Environment variables** — `&#123;&#123;env.X&#125;&#125;` and `&#123;&#123;var.X&#125;&#125;` template resolution
- **Session persistence** — edits survive between sessions via temp files
- **Portable** — single binary for Linux, macOS, and Windows

## Quick start

```bash
# Build
go build -o lazyapi ./cmd/lazyapi

# Launch TUI with a spec
./lazyapi examples/openapi.yml

# Or use CLI commands
./lazyapi create file my-api.yml
./lazyapi add request my-api.yml /users GET
./lazyapi send request my-api.yml /users GET --server http://localhost:3000
```

## Why LazyAPI?

Most API tools require you to manually recreate information that already exists in your OpenAPI specification. LazyAPI eliminates that duplication:

```
OpenAPI
    │
    ▼
 LazyAPI
    │
    ├── Explore Operations
    ├── Execute Requests
    ├── Validate Responses
    └── Run Automated Tests
```
