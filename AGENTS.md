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
        tests.go           #     Tests tab — JS/TS script editor, F5 runs it via internal/script; default template is not persisted
        field.go           #     paramField helper (name+value pair)
      requests/            #   Request browser (compact list + preview + tags)
        list.go            #     RequestList — custom compact list with filter, tag grouping, scroll
        group.go           #     GroupByTags — groups items by first OpenAPI tag
        item.go            #     RequestItem (Method, URI, About, Tags, RenderCompact)
        preview.go         #     PreviewModel — request header only (the TUI renders the response below it)
        tags.go            #     TagsOverlay — cursor-positioned tag editor (add/delete/change)
      responses/           #   Response view shared by editor and list preview
        model.go           #     Model — status line, tabs (Body/Headers/Checks/Console), raw toggle, search, spinner
        format.go          #     Pretty (json.Indent, no HTML escaping), Highlight (chroma catppuccin-mocha), HumanSize
        exchange.go        #     SendMsg / ResultMsg, Exchange(ctx, SendMsg) → runner.Exchange with the spec as context
  cli/                     # CLI commands — thin wrappers over runner/inspect/report/mcp
    cli.go                 #   Commands list, run() dispatcher, printUsage
    options.go             #   ExitError + ExitCode (0/1/2/3), runFlags shared by run/send, writeReport
    flags.go               #   stdlib flag helpers: parseInterleaved, kvFlag (--param k=v), splitList
    run.go                 #   RunTests (`run`; `smoke tests` is an alias): selection, --parallel, --retries, --bail
    send.go                #   SendRequest: <path> <method> | --op, --body @file, --output json, --save-example
    inspect.go             #   ListOps, Describe, ValidateSpec
    mcp.go                 #   ServeMCP (`lazyapi mcp --root`), Version (set by goreleaser ldflags)
    add.go / create.go / remove.go / server.go  # spec editing commands
  runner/                  # Executes operations → structured results (no printing)
    result.go              #   Outcome, Check (status|contract|test|script), OpResult, Report, Summarize, exit codes
    runner.go              #   Filter, ParseMethods, Run (ordered OnResult, parallel, bail, retries), RunOne
  report/                  # Renders runner results: pretty (FormatOp/WriteSummary), json, junit, tap; body policy
  inspect/                 # ListOperations, Describe (with referenced schemas), Validate (OpenAPI + scripts compile)
  mcp/                     # MCP server (JSON-RPC 2.0 over stdio, hand-rolled) + Tools confined to a root dir
  script/                  # JS/TS test engine: esbuild bundling + goja sandbox
    script.go              #   Run(source, ExecData, Options) → Result; native bridge; Format
    prelude.js             #   JS side of the API: test, expect matchers, console, LazyResponse
    bundle.go              #   Bundle() — esbuild plugin: https imports, local imports confined to Root, bare imports rejected
    modules.go             #   ModuleLoader — https fetch, disk cache, lazyapi.lock integrity, Offline/Reload/Frozen/AllowedHosts
    spec.go                #   SpecContext (lazyapi.send + contract), ResolveTarget, OperationOptions
    lazyapi.d.ts           #   Public script API (embedded; printed by `lazyapi types`)
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
    request.go             #   Request, OpenAPIRef, Send()/SendContext(), ResolvedURL, MissingPathParams (no bubbletea)
    method.go              #   Method iota (POST=0..HEAD), MarshalYAML/UnmarshalYAML/Label
    body.go                #   Body {MimeType, Raw}, MimeType constants
    auth.go                #   AuthType iota, AuthScheme {all secrets + schema metadata}
    about.go               #   About {Summary, Description}
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
    tests.go               #   x-lazyapi-tests: LoadOperationTests, SaveOperationTests, OperationTestsPath, TestScriptName (rejects .lua)
    contract.go            #   ValidateResponse (openapi3filter), ResponseSchemaDeclared
    examples.go            #   ParamExample, DeclaredStatuses, StatusDeclared, FindOperation
    openapi_test.go        #   Auth roundtrip tests using testdata fixtures
    testdata/              #   Static .yml fixtures (minimal.yml, global_security.yml, params.yml)
```

## Key Types

- **`model.Request`** — Central data type: URI, Method, Body, Headers, Params, Query, About, ServerURL, Servers, OpenAPIRef, DraftPath, FileName, Env, Vars, Auth, Tests, Timeout. `Send()`/`SendContext()` are synchronous; the TUI sends through `responses.Exchange` (runner checks) — never call Send from Update.
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
- **Test scripts** — JavaScript/TypeScript files next to the spec (default `tests/<operationId|method_path>.js`) and referenced per operation by the `x-lazyapi-tests` extension. Paths must stay inside the spec directory. `OperationToRequest` loads them into `Request.Tests`; `SaveFile` writes them via `SaveOperationTests`. The CLI never reads test scripts from temp files.
- **Errors** — `tea.Cmd` wrappers return `store.ErrorMsg{Message}` on failure; the TUI shows it as a notification. Never return `tea.Quit`/`tea.Println` as a `tea.Msg` and never `panic` in store code.
- **Temp/draft files** — stored in `os.TempDir()/lazyapi/<sanitized-abs-path>/`. Format: `tmp.<METHOD>.<sanitized-path>` for spec-ref'd operations, `draft.new.<N>` for new unsaved requests. On "save", merged into spec via `AddOperationToSpec` (new) or `ApplyRequestToOperation` (existing).
- **Key functions**: `ParseSpec`, `SaveSpec`, `ListOperations` (→ `[]requests.RequestItem` with Tags), `OperationToRequest` (pre-fills path params and required query/header params from spec examples), `AddOperationToSpec`, `RemoveOperationFromSpec`, `ApplyRequestToOperation` (persists Summary/Description/content type/param defs/auth schemes — does NOT persist Body.Raw, header/param/query values), `LoadServers`, `SaveResponseExample`, `IsOpenAPIFile`, `UpdateOperationTags`.
- **`Glob`** — custom double-star (`**`) glob in `filepath.go` using `strings.Split` + `filepath.Walk`.

## TUI vs CLI

- **TUI** (`lazyapi` with no subcommand) — interactive bubbletea UI. Default mode.
- **CLI** (`lazyapi run|send|list|describe|validate|types|mcp|create|add|remove ...`) — headless commands. Dispatch: `cmd/lazyapi/main.go` sends anything in `cli.Commands` to `cli.Run()`, else starts `tea.NewProgram(app.NewTui(...))`.
- **Flag parsing** — every command uses the stdlib `flag` package via `newFlagSet` + `parseInterleaved` (flags allowed before/after positionals; unknown flags are errors). `main.go` still parses `--env` ad-hoc.
- **CLI subcommands**: `run <file>`, `send request <file> (<path> <method> | --op id)`, `smoke tests` (alias of run), `list ops`, `describe`, `validate spec`, `types`, `mcp`, plus `create file`, `add request|server`, `remove request`. `main.go` dispatches anything in `cli.Commands`. See `docs/cli/`, `docs/scripting.md`, `docs/agents.md`.
- **Run safety** — runs only `GET,HEAD,OPTIONS` unless `--methods` is given; skips operations whose path params have no value (never invents one); a status not declared in the operation's `responses` is a failed `status` check (exit 1).
- **Execution** — all checks live in `internal/runner`: status declared → contract (`store.ValidateResponse`) → script. CLI and MCP only render `runner.Report`/`OpResult`. Never print from runner.
- **Exit codes** — 0 ok, 1 a check failed, 2 usage, 3 could not run (errors win over failures). Return `*cli.ExitError` (`usageErr`, `runtimeErr`) instead of plain errors from commands.
- **Scripts** — run per operation by `runner`: automatic contract check, then the operation's script via `script.Run`. URL imports are pinned in `lazyapi.lock` next to the spec (`--offline`, `--reload`, `--frozen-lockfile`, `--allow-import`). `lazyapi types` prints the script API. Lua is gone; `.lua` references produce an explicit error.
- **HTTP** — `Request.SendContext` applies `Request.Timeout` (default `model.DefaultTimeout`, 30s). All auth fields (incl. Basic) are template-resolved.

## TUI Request Flow

- Editor (Send button, `alt+enter`, `F5` without a response) and list (`s`) emit `responses.SendMsg`. `Tui.startSend` owns a cancellable context (`esc` cancels), shows the spinner and runs `responses.Exchange` → `runner.Exchange` (status, contract, script checks). The result comes back as `responses.ResultMsg{Source}`.
- List sends use `store.LoadRequest(ref)` (temp edits, else spec with examples). Non-safe methods (POST/PUT/PATCH/DELETE) go through a y/N confirmation overlay.
- `Tui.capturesKeys()` (list filter, editor URI/tab content, response search) disables global shortcuts like `q`, `?`, `ctrl+v`.
- Environment: `ctrl+v` cycles `env.Discover(".")` via `env.Store.SetPath`; the title bar shows spec and env.
- `pane/requests` must not import `responses`/`runner` (store imports `requests` → import cycle). The TUI composes preview header + `previewResp`.

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
- **param/header styling** in `View()` resets `lipgloss.Style` each frame, making focus highlighting fragile.

### Warnings (design debt)
- HTTP-method→string mapping repeated in 5 places (Method.Label, Method.UnmarshalYAML, methodFromLabel, RemoveOperationFromSpec, hardcoded strings).
- Parameter-ref creation blocks duplicated between `AddOperationToSpec` and `ApplyRequestToOperation`.
- `env.Load()` and `mergeDotenv()` share identical line-parsing logic.
- Dead/stub code: `modal.go` (empty), `view.go` (unused).
- `store` imports the UI package `app/pane/requests` (`RequestItem`); move the item data type to `model` to break the dependency.
- bubbles viewport mispositions highlights on ANSI-colored content (`parseMatches` mixes stripped and raw offsets); the response view strips colors while a search is active.

### Linter (3 deprecations)
- `scrollable.go` uses deprecated `msg.Type`, `tea.MouseWheelUp`, `tea.MouseWheelDown` — should migrate to `MouseAction`/`MouseButton`.

## Testing

- **Test files** next to code (`internal/store/openapi_test.go`). CLI tests use `httptest` servers (`internal/cli/runner_test.go`). URL-import tests use an `httptest.NewTLSServer` fake CDN (`internal/script/modules_test.go`) — never hit the real network in tests.
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
