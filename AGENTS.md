# LazyAPI

OpenAPI-driven API exploration, testing, and automation from the terminal.

## Architecture

```
cmd/
  lazyapi/main.go    # Root entry — dispatches to TUI (default) or CLI (subcommands)
  tui/main.go        # TUI-only entry (kept for backward compat)
internal/
  app/               # TUI application (bubbletea model + views)
    tui.go           #   Main Tui struct, Init/Update/View
    pane/            #   UI panes
      editor/        #     Request editor (method, URL, headers, body, params, tests)
      requests/      #     Request list (tree, grouping, CRUD messages)
  cli/               # CLI commands (create, remove, add, smoke)
  components/        # Reusable UI components (button, field, modal, tabs, etc.)
  config/            # Colors, keybindings, page constants
  env/               # Environment variable loading/resolution
  inmath/            # Math utilities
  model/             # Core types: Request, Method, Body, Response, OpenAPIRef
  store/             # File system + OpenAPI spec operations
```

## Key Types

- **`model.Request`** — The central data type. Contains URI, Method, Body, Headers, Params, Query, About, ServerURL, Servers, OpenAPIRef, DraftPath, FileName.
- **`model.Method`** — Enum (GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD) with Label() string method.
- **`model.OpenAPIRef`** — Reference to an operation in an OpenAPI file: FilePath, Path, Method.
- **`model.About`** — Summary + Description for documentation.
- **`model.Body`** — MimeType + Raw content.
- **`model.Method`** — iota enum, marshals/unmarshals to/from YAML strings.

## Store Layer

The `store` package is the data access layer:

- **OpenAPI as source of truth** — every `.yml`/`.yaml` file is parsed as an OpenAPI 3.x spec via `openapi3.Loader`. Operations are extracted as `requests.RequestItem`.
- **Temp/draft files** — edits are saved alongside the spec file as `.lazyapi.tmp.METHOD.PATH` (for OpenAPI-ref operations) or `.lazyapi.draft.*` (for new unsaved requests). On "save", the temp data is merged back into the spec via `AddOperationToSpec` or `ApplyRequestToOperation`.
- **Key functions** — `ParseSpec`, `SaveSpec`, `ListOperations`, `OperationToRequest`, `AddOperationToSpec`, `RemoveOperationFromSpec`, `ApplyRequestToOperation`, `LoadServers`, `IsOpenAPIFile`.
- **All are synchronous** — no bubbletea dependency; `tea.Cmd` wrappers exist in `file_system.go` for TUI integration, but the core logic can be called directly (used by CLI).

## TUI vs CLI

- **TUI** (`lazyapi` with no subcommand) — interactive terminal UI using bubbletea. Default mode.
- **CLI** (`lazyapi create|remove|add|smoke ...`) — headless commands for scripting/automation. Subcommands: `create file`, `add request`, `add server`, `remove request`, `smoke tests`.
- Both are separate code paths; the root `cmd/lazyapi/main.go` dispatches based on `os.Args`.

## Building & Running

```bash
go build -o lazyapi ./cmd/lazyapi   # Single binary
go run ./cmd/tui/main.go            # TUI only
go run ./cmd/lazyapi create file    # CLI
```

## Conventions

- Method strings are uppercase (GET, POST, etc.)
- OpenAPI refs use `{FilePath, Path, Method}` structure
- YAML files use `openapi: 3.0.0` at the root
- No external CLI framework — use `os.Args` + `flag` package
- No database — everything is file-based
