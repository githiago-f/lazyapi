# LazyAPI

OpenAPI-driven API exploration, testing, and automation from the terminal.

## Architecture

```
cmd/
  lazyapi/main.go          # Entry — TUI (default) or CLI (subcommands)
internal/
  app/                     # bubbletea model + views
    tui.go                 #   Main Tui struct, Init/Update/View, state machine
    pane/
      editor/              #   Request editor (method, URL, headers, body, params, tests, auth)
        editor.go          #     RequestPane — top-level Update/View, cursor field cycle
        documentation.go   #     Summary + Description fields
        body.go            #     Body textarea
        header.go          #     Header key/value pairs (n+a to add)
        params.go          #     Query + Path params (n+q / n+p to add)
        auth.go            #     Auth schemes (n+a / x+a to add/remove)
        tests.go           #     Stub — returns "Tests" string
        field.go           #     paramField helper (name+value pair)
      requests/            #   Request browser (compact list + preview + tags)
        list.go            #     RequestList — custom compact list with filter, tag grouping, scroll
        group.go           #     GroupByTags — groups items by first OpenAPI tag
        item.go            #     RequestItem (Method, URI, About, Tags, RenderCompact)
        preview.go         #     PreviewModel — request info, send action, response viewport
        tags.go            #     TagsOverlay — cursor-positioned tag editor (add/delete/change)
      responses/           #   Response preview (stub — not used; editor uses viewport.Model directly)
  response/                # Shared response formatting
    formatter.go           #   BuildContent, FormatContent — status/headers/body formatting
  cli/                     # CLI commands (create, remove, add, send, smoke)
    cli.go                 #   Run() dispatcher + printUsage
    add.go                 #   AddRequest, AddServer
    create.go              #   CreateFile
    remove.go              #   RemoveRequest
    send.go                #   SendRequest (parses --server, --env, --save-example ad-hoc)
    server.go              #   AddServer
    smoke.go               #   SmokeTests (not implemented — prints stub message)
  components/              # Reusable UI components
    button.go              #   Button with click debounce
    field.go               #   Field wraps textinput.Model
    modal.go               #   Empty file (package decl only)
    passfield.go           #   PassField — Field with ctrl+p toggle
    prompt.go              #   PromptModel — overlay input for answers
    selector.go            #   Selector — up/down cycle through Labels
    text.go                #   Text wraps textarea.Model
    title_bar.go           #   TitleBar — top bar with config.Name()
    view.go                #   View — generic bordered content (unused)
    scrollable/            #   Scrollable model wrapper with pgup/pgdown/mouse
      scrollable.go
    tabs/                  #   Tabbed container
      tab.go               #     Tab struct (label + Content tea.Model)
      view.go              #     Model — renders tab bar + content, SetActiveTabMsg
  config/                  # Catppuccin Mocha color palette + keybindings + page constants
    config.go              #   Config struct, DefaultConfig, color constants
    keymap.go              #   DefaultKeyMap, KeyMap, ShortHelp/FullHelp
    pages.go               #   PageIndex enum (RequestList, RequestEditor)
  env/                     # Environment variable loading/resolution
    loader.go              #   Load() — reads .env, merges with system env; Store with hash-based caching
    resolver.go            #   Resolve() — replaces {{env.X}} / {{var.X}} in strings
  inmath/                  # Math utilities
    counters.go            #   Circle() — wraps int within [min, max] (typo: Circle → Circle)
  model/                   # Core types
    request.go             #   Request, OpenAPIRef, Send(), RunRequest() tea.Cmd
    method.go              #   Method iota (POST=0..HEAD), MarshalYAML/UnmarshalYAML/Label
    body.go                #   Body {MimeType, Raw}, MimeType constants
    auth.go                #   AuthType iota, AuthScheme {all secrets + schema metadata}
    about.go               #   About {Summary, Description}
    response.go            #   Response {StatusCode, Body} (unused)
  store/                   # File system + OpenAPI spec operations
    openapi.go             #   ParseSpec, SaveSpec, ListOperations, OperationToRequest,
                           #   AddOperationToSpec, RemoveOperationFromSpec,
                           #   ApplyRequestToOperation, SaveResponseExample,
                           #   auth helpers (applyAuthSchemes, extractSecurityFromSpec,
                           #   ExtractGlobalSecurity, ExtractOperationSecurity)
    file_system.go         #   tea.Cmd wrappers: FindRequestFiles, LoadRequestsList,
                           #   OpenRequestFile, OpenDraftFile, SaveTempFile, SaveFile,
                           #   DeleteRequestFile, LoadForDuplicate, SaveResponseExampleCmd
    filepath.go            #   Glob() with ** support (Globs type + Expand method)
    openapi_test.go        #   Auth roundtrip tests using testdata fixtures
    testdata/              #   Static .yml fixtures (minimal.yml, global_security.yml)
```

## Key Types

- **`model.Request`** — Central data type: URI, Method, Body, Headers, Params, Query, About, ServerURL, Servers, OpenAPIRef, DraftPath, FileName, Env, Vars, Auth. Has `Send()` (synchronous) and `RunRequest()` (wraps Send in `tea.Cmd`).
- **`model.Method`** — iota enum (POST=0, GET, PATCH, PUT, DELETE, OPTIONS, HEAD). `MarshalYAML`/`UnmarshalYAML` for lowercase strings. `Label()` returns uppercase.
- **`model.OpenAPIRef`** — `{FilePath, Path, Method}` referencing an OpenAPI operation.
- **`model.About`** — `{Summary, Description}` for operation documentation.
- **`model.Body`** — `{MimeType, Raw}`. Constants: `ApplicationJSON` (`"application/json"`), `PlainText` (`"text/plain"`).
- **`model.AuthScheme`** — Single struct for all types (Basic, Bearer, API Key, OAuth2) with both schema-definition fields and secret fields.
- **`requests.RequestItem`** — Display item: Method, URI, About, Tags, FileName, DraftPath, OpenAPIRef, RequestTime. Has `RenderCompact()` for single-line list rendering. No longer implements `list.Item`.
- **`config.PageIndex`** — `RequestList` (0) or `RequestEditor` (1).

## Store Layer

`internal/store/` — data access. Core functions are synchronous; `tea.Cmd` wrappers in `file_system.go` for TUI.

- **OpenAPI as source of truth** — every `.yml`/`.yaml` file parsed as OpenAPI 3.x via `openapi3.Loader`. `IsOpenAPIFile()` checks for `openapi` or `swagger` root keys via YAML unmarshal.
- **Temp/draft files** — stored in `os.TempDir()/lazyapi/<sanitized-abs-path>/`. Format: `tmp.<METHOD>.<sanitized-path>` for spec-ref'd operations, `draft.new.<N>` for new unsaved requests. On "save", merged into spec via `AddOperationToSpec` (new) or `ApplyRequestToOperation` (existing).
- **Key functions**: `ParseSpec`, `SaveSpec`, `ListOperations` (→ `[]requests.RequestItem` with Tags), `OperationToRequest`, `AddOperationToSpec`, `RemoveOperationFromSpec`, `ApplyRequestToOperation` (persists Summary/Description/content type/param defs/auth schemes — does NOT persist Body.Raw, header/param/query values), `LoadServers`, `SaveResponseExample`, `IsOpenAPIFile`, `UpdateOperationTags`.
- **`Glob`** — custom double-star (`**`) glob in `filepath.go` using `strings.Split` + `filepath.Walk`.

## TUI vs CLI

- **TUI** (`lazyapi` with no subcommand) — interactive bubbletea UI. Default mode.
- **CLI** (`lazyapi create|remove|add|send|smoke ...`) — headless commands. Dispatch: `cmd/lazyapi/main.go` checks `os.Args[1]`; known verbs → `cli.Run()`, else → `tea.NewProgram(app.NewTui(...))`.
- **Flag parsing** — ad-hoc manual loop in `main.go`, `send.go`, and `smoke.go`. No `flag` package.
- **CLI subcommands**: `create file [name] [servers...]`, `add request <file> <path> <method>`, `add server <file> <url>`, `remove request <file> <method> <path>`, `send request <file> <path> <method> [--server url] [--env file] [--save-example]`, `smoke tests <file> [--server url] [--env file]` (stub).

## Editor Field Cycle

The editor (`RequestPane`) has 6 top-level fields cycled by Tab/Shift+Tab: `method → server → uri → [send button] → reqTabs → response → method → ...`

When `reqTabs` is focused, Enter activates the tab content (`BlockTab = true`). Tab/Shift+Tab then cycles within the tab's sub-fields. Esc first blurs the tab content, then a second Esc exits the tab back to top-level fields.

## Building & Running

```bash
go build -o lazyapi ./cmd/lazyapi
./lazyapi                            # TUI
./lazyapi examples/openapi.yml       # TUI with spec preloaded
./lazyapi create file my-api.yml     # CLI
./lazyapi send request spec.yml /pets GET --server http://localhost:8080
go test ./...                        # Tests
golangci-lint run                    # Lint (config: .golangci.yml)
```

## Known Issues (from code review)

### Errors (will cause bugs)
- **`store.LoadForDuplicate`** closes the draft file before reading from it.
- **`store.SaveFile` (else branch)** closes the created file before encoding YAML to it.
- **param/header styling** in `View()` resets `lipgloss.Style` each frame, making focus highlighting fragile.

### Warnings (design debt)
- HTTP-method→string mapping repeated in 5 places (Method.Label, Method.UnmarshalYAML, methodFromLabel, RemoveOperationFromSpec, hardcoded strings).
- Parameter-ref creation blocks duplicated between `AddOperationToSpec` and `ApplyRequestToOperation`.
- `env.Load()` and `mergeDotenv()` share identical line-parsing logic.
- TUI auto-saves on every Update (window resize, mouse move, keystroke).
- Path-param URL substitution iterates a map (non-deterministic order).
- Dead/stub code: `modal.go` (empty), `tests.go` (stub), `view.go` (unused).
- Preview panel auto-sends on `s` without confirmation.

### Linter (3 deprecations)
- `scrollable.go` uses deprecated `msg.Type`, `tea.MouseWheelUp`, `tea.MouseWheelDown` — should migrate to `MouseAction`/`MouseButton`.

## Testing

- **Test files** next to code (`internal/store/openapi_test.go`).
- **Fixtures** in `internal/store/testdata/` as `.yml` files. Use `fixturePath(name)`.
- **Avoid `/tmp`** — use `t.TempDir()` for file roundtrips.
- **Pattern** — roundtrip (marshal → write → read → unmarshal → verify), not UI interaction.
- Run: `go test ./...`

## Conventions

- Method strings uppercase in code, lowercase in YAML.
- OpenAPI refs: `{FilePath, Path, Method}`.
- YAML files: `openapi: 3.0.0` root key.
- No external CLI framework — ad-hoc `os.Args`.
- No database — everything file-based.
- Tab/Shift+Tab within tab content wraps cyclically (last → first, first → last). No exit via Tab; Esc blurs then exits.
- `lipgloss` for all styling; Catppuccin Mocha palette in `config/config.go`.
