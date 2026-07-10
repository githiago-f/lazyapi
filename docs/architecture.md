# Architecture

LazyAPI is structured as a Go monolith with a clean internal package layout. All core logic is synchronous; TUI interactions are wrapped in `tea.Cmd` closures for the bubbletea framework.

## Package layout

```
cmd/
  lazyapi/main.go          # Entry point — dispatches to TUI or CLI

internal/
  app/                     # bubbletea model + views
    tui.go                 #   Main Tui struct, Init/Update/View
    pane/
      editor/              #   Request editor (method, URL, headers, body, params, tests, auth)
      requests/            #   Request list (tree with GroupByResource, CRUD messages)
      responses/           #   Response preview
  cli/                     # CLI commands (create, remove, add, send, smoke)
  components/              # Reusable UI components (button, field, modal, tabs, selector, etc.)
  config/                  # Catppuccin Mocha color palette + keybindings + page constants
  env/                     # Environment variable loading/resolution
  inmath/                  # Math utilities (circular field cycling)
  model/                   # Core types: Request, Method, Body, Response, OpenAPIRef, About
  store/                   # File system + OpenAPI spec operations
```

## Data flow

```text
OpenAPI YAML file
       │
       ▼
   store.ParseSpec()      ─── openapi3.Loader
       │
       ▼
   store.ListOperations()  ─── sorted by URI
       │
       ▼
   requests.RequestItem    ─── wraps Method, URI, About, FileName, OpenAPIRef
       │
       ▼
   editor.RequestPane      ─── 6 tabs (Documentation, Params, Authorize, Header, Body, Tests)
       │
       ▼
   model.Request.Send()    ─── http.DefaultClient.Do
```

## Key principles

- **OpenAPI is the source of truth** — every `.yml`/`.yaml` file is parsed as OpenAPI 3.x
- **No database** — everything is file-based
- **Session state in temp files** — unsaved edits live in `os.TempDir()/lazyapi/`
- **Auth secrets never persist** — security scheme definitions are saved, but runtime values (passwords, tokens) are session-only

## Temp file system

```
os.TempDir()/lazyapi/<sanitized-abs-path>/
  tmp.<METHOD>.<sanitized-path>   # OpenAPI-ref operations
  draft.new.<N>                    # New unsaved requests
```
