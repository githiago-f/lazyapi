# LazyAPI

OpenAPI-driven API exploration, testing, and automation from the terminal.

## Architecture

```
cmd/
  lazyapi/main.go          # Entry — TUI (default) or CLI (subcommands)
internal/
  app/                     # bubbletea model + views
    tui.go                 #   Main Tui struct, Init/Update/View
    pane/
      editor/              #   Request editor (method, URL, headers, body, params, tests, auth)
      requests/            #   Request list (tree with GroupByResource, CRUD messages)
      responses/           #   Response preview
  cli/                     # CLI commands (create, remove, add, smoke)
  components/              # Reusable UI components (button, field, modal, tabs, selector, etc.)
  config/                  # Catppuccin Mocha color palette + keybindings + page constants
  env/                     # Environment variable loading/resolution (STUB — both functions empty)
  inmath/                  # Math utilities (Cicle for cycling field focus)
  model/                   # Core types: Request, Method, Body, Response, OpenAPIRef, About
  store/                   # File system + OpenAPI spec operations
```

## Key Types

- **`model.Request`** — Central data type: URI, Method, Body, Headers, Params, Query, About, ServerURL, Servers, OpenAPIRef, DraftPath, FileName. Has `RunRequest()` (sends via `http.DefaultClient`).
- **`model.Method`** — iota enum (POST=0, GET, PATCH, PUT, DELETE, OPTIONS, HEAD). Marshals/unmarshals to/from YAML lowercase strings. `Label()` returns uppercase.
- **`model.OpenAPIRef`** — `{FilePath, Path, Method}` referencing an OpenAPI operation.
- **`model.About`** — `{Summary, Description}` for documentation.
- **`model.Body`** — `{MimeType, Raw}`. MimeType constants: `ApplicationJSON` (`"application/json"`), `PlainText` (`"plain/txt"` — non-standard).
- **`requests.RequestItem`** — Display item wrapping Method, URI, About, FileName, DraftPath, OpenAPIRef, RequestTime.

## Store Layer

`internal/store/` — data access. All core functions are synchronous; `tea.Cmd` wrappers in `file_system.go` for TUI.

- **OpenAPI as source of truth** — every `.yml`/`.yaml` file is parsed as OpenAPI 3.x via `openapi3.Loader`. `IsOpenAPIFile()` checks for `openapi` or `swagger` root keys.
- **Temp/draft files** — stored in `os.TempDir()/lazyapi/<sanitized-abs-path>/`. Format: `tmp.<METHOD>.<sanitized-path>` for OpenAPI-ref operations, `draft.new.<N>` for new unsaved requests. On "save", merged back into the spec via `AddOperationToSpec` or `ApplyRequestToOperation`.
- **Key functions** — `ParseSpec`, `SaveSpec`, `ListOperations`, `OperationToRequest`, `AddOperationToSpec`, `RemoveOperationFromSpec`, `ApplyRequestToOperation` (only updates Summary/Description — body/params/query NOT applied back), `LoadServers`, `IsOpenAPIFile`.
- **`Glob`** — custom double-star (`**`) glob implementation in `filepath.go`.

## TUI vs CLI

- **TUI** (`lazyapi` with no subcommand or non-CLI arg) — interactive bubbletea UI. Default mode.
- **CLI** (`lazyapi create|remove|add|smoke ...`) — headless commands. Subcommands: `create file [name] [servers...]`, `add request <file> <path> <method>`, `add server <file> <url>`, `remove request <file> <method> <path>`, `smoke tests <file> [--server url] [--env file]` (not yet implemented).
- **Dispatch** — `cmd/lazyapi/main.go` checks `os.Args[1]`; CLI verbs → `cli.Run()`, else → `tea.NewProgram(app.NewTui(...))`.
- **The `--server` and `--env` flag parsing in `smoke.go` is ad-hoc** (manual loop, no `flag` package).

## Building & Running

```bash
go build -o lazyapi ./cmd/lazyapi   # Single binary (also goreleaser entry)
./lazyapi                            # TUI (default)
./lazyapi examples/openapi.yml       # TUI with a spec preloaded
./lazyapi create file my-api.yml     # CLI
```

## Conventions

- Method strings are uppercase (GET, POST, etc.)
- OpenAPI refs use `{FilePath, Path, Method}` structure
- YAML files use `openapi: 3.0.0` at the root
- No external CLI framework — ad-hoc `os.Args` parsing
- No database — everything is file-based
- Release: goreleaser v2, builds for linux/windows/darwin (amd64 + arm64)
- **Tab/Shift+Tab within tab content always wraps** — cycling through sub-fields goes from last back to first (and vice versa). There is no exit state via Tab; the user blurs the tab with Esc.

## Known Quirks

- **`ApplyRequestToOperation`** persists Summary, Description, Body content type, and parameter definitions (names + `in` type). Runtime values (Body.Raw, param/query/header values) are session-only and stored in temp files, not in the OpenAPI spec.
- **`RunRequest`** sends the request via `http.DefaultClient.Do`.
- **`env/`** package has empty stubs (`Load()` and `Resolve()` do nothing).
- **`smoke tests`** subcommand prints "not implemented yet".
- **No tests exist** in the repo — `go test ./...` produces nothing.
- **No CI workflows** configured yet.
- **`inmath.Cicle`** has a typo (should be `Circle`).
- **Editor `BlockTab`** uses a two-stage Esc pattern: first Esc blurs the tab's inner content, second Esc exits the tab field.
