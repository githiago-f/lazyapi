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
      palette/             #   Command palette (ctrl+k): fuzzy list of palette.Command{Group, Title, Hint, Msg}; ChosenMsg/ClosedMsg
      runview/             #   Runner view: runner.Run in a goroutine, results streamed through a channel (run id ignores stale msgs), list + responses.Model details
      historyview/         #   History list, replay (ReplayMsg), diff with previous response / spec example
      editor/              #   Request editor (method, URL, headers, body, params, tests, auth)
        editor.go          #     RequestPane — top-level Update/View, cursor field cycle
        documentation.go   #     Summary + Description fields
        body.go            #     Body textarea
        grid.go            #     kvGrid — name/value rows shared by headers, params and form fields; view renders focus on copies
        header.go          #     Header key/value pairs (ctrl+g to add)
        params.go          #     Query + Path params (n+q / n+p to add)
        auth.go            #     Auth schemes (n+a / x+a to add/remove)
        tests.go           #     Tests tab — JS/TS script editor, F5 runs it via internal/script; default template is not persisted
        field.go           #     paramField helper (name+value pair)
      requests/            #   Request browser (compact list + preview + tags)
        list.go            #     RequestList — custom compact list with filter, tag grouping, scroll
        group.go           #     GroupByTags — groups items by first OpenAPI tag
        item.go            #     RequestItem = model.RequestItem; RenderCompact, methodStyle
        preview.go         #     PreviewModel — request header only (the TUI renders the response below it)
        tags.go            #     TagsOverlay — cursor-positioned tag editor (add/delete/change)
      responses/           #   Response view shared by editor and list preview
        model.go           #     Model — status line, tabs (Body/Headers/Cookies/Timeline/Checks/Console), raw toggle, search (matches found on ansi.Strip text, mapped back with rawOffsets), jq filter (f), flow steps, spinner
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
    mock.go                #   ServeMock (`lazyapi mock <spec> --port 4010`)
    add.go / create.go / remove.go / server.go  # spec editing commands
  runner/                  # Executes operations → structured results (no printing)
    result.go              #   Outcome, Check (status|contract|test|script), OpResult, Report, Summarize, exit codes
    runner.go              #   Filter, ParseMethods, Run (reads→POST→PUT/PATCH→DELETE, parallel, bail, retries), RunOne, Exchange
    flow.go                #   RunFlow (flows/*.js; every lazyapi.send step gets request/status/contract checks), FindFlows
  report/                  # Renders runner results: pretty (FormatOp/WriteSummary), json, junit, tap; body policy
  inspect/                 # ListOperations, Describe (with referenced schemas), Validate (OpenAPI + scripts compile)
  mcp/                     # MCP server (JSON-RPC 2.0 over stdio, hand-rolled) + Tools confined to a root dir
  project/                 # lazyapi.config.yml: named environments (server, env file, vars, auth secrets), Find/Select/Resolve
  history/                 # TUI request history per spec (user cache dir, jsonl, 0600): no env/auth secrets, sensitive headers/query redacted, body templates kept; Replay
  diff/                    # Line diff (LCS with prefix/suffix trimming), hunks with folded context
  jq/                      # gojq wrapper: Apply(filter, body) with timeout, Check(filter)
  importer/                # curl / Postman (+ environments: WriteEnvironment → .env.<name> + project.AddEnvironment) / HAR → spec operations (Apply: path templating, inferred schemas, security, tests); pm compat scripts
  mock/                    # Mock server: routes spec paths, answers with examples or SchemaSample, validates requests (422), Prefer: code=/example=
  script/                  # JS/TS test engine: esbuild bundling + goja sandbox
    script.go              #   Run (post-response), Before (export function before(req)), native bridge, Format
    prelude.js             #   JS side of the API: test, expect matchers, console, LazyResponse
    postman.js             #   `pm` compatibility layer (pm.test/expect with Chai subset, pm.response, variable scopes)
    bundle.go              #   Bundle() — CommonJS wrapped in (function(module, exports){...}) so exports exist before the body; esbuild plugin: https imports, local imports confined to Root, bare imports rejected
    modules.go             #   ModuleLoader — https fetch, disk cache, lazyapi.lock integrity, Offline/Reload/Frozen/AllowedHosts
    spec.go                #   SpecContext (lazyapi.send + contract; "METHOD https://…" targets mapped with store.MatchPath and sent to that host), ResolveTarget, OperationOptions
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
    request.go             #   Request, OpenAPIRef, Send()/SendContext(), HTTPRequest (the exact *http.Request sent; empty credentials are skipped), ResolvedURL, MissingPathParams
    oauth.go               #   AuthSecrets, ApplySecrets (by scheme name), OAuth2 client_credentials/password token fetch + cache
    client.go              #   HTTPOptions (insecure, CA, mTLS, proxy, redirects) → *http.Client; Timing
    body.go                #   Body encoding: JSON object → form-urlencoded / multipart; file parts only from Body.Files (never from Raw)
    curl.go                #   Request.Curl — exact request as a curl command
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
    contract.go            #   ValidateRequest / ValidateResponse (openapi3filter, readable errors), ResponseSchemaDeclared
    sample.go              #   SchemaSample — plausible value from a schema (example/default/enum/format/min)
    examples.go            #   ParamExample, BodyExample, DeclaredStatuses, StatusDeclared, FindOperation
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
- **`model.RequestItem`** — List item data: Method, URI, About, Tags, FileName, DraftPath, OpenAPIRef, RequestTime, `FilterValue()`. `requests.RequestItem` is an alias; `requests.RenderCompact(item, width, selected)` renders one line.
- **`config.PageIndex`** — `RequestList` (0) or `RequestEditor` (1).

## Store Layer

`internal/store/` — data access. Core functions are synchronous; `tea.Cmd` wrappers in `file_system.go` for TUI.

- **OpenAPI as source of truth** — every `.yml`/`.yaml` file parsed as OpenAPI 3.x via `openapi3.Loader`. `IsOpenAPIFile()` checks for `openapi` or `swagger` root keys via YAML unmarshal.
- **Test scripts** — JavaScript/TypeScript files next to the spec (default `tests/<operationId|method_path>.js`) and referenced per operation by the `x-lazyapi-tests` extension. Paths must stay inside the spec directory. `OperationToRequest` loads them into `Request.Tests`; `SaveFile` writes them via `SaveOperationTests`. The CLI never reads test scripts from temp files.
- **Errors** — `tea.Cmd` wrappers return `store.ErrorMsg{Message}` on failure; the TUI shows it as a notification. Never return `tea.Quit`/`tea.Println` as a `tea.Msg` and never `panic` in store code.
- **Temp/draft files** — stored in `os.TempDir()/lazyapi/<sanitized-abs-path>/`. Format: `tmp.<METHOD>.<sanitized-path>` for spec-ref'd operations, `draft.new.<N>` for new unsaved requests. On "save", merged into spec via `AddOperationToSpec` (new) or `ApplyRequestToOperation` (existing).
- **Key functions**: `ParseSpec`, `SaveSpec`, `ListOperations` (→ `[]model.RequestItem` with Tags), `OperationToRequest` (pre-fills path params and required query/header params from spec examples), `AddOperationToSpec`, `RemoveOperationFromSpec`, `ApplyRequestToOperation` (persists Summary/Description/content type/param defs/auth schemes — does NOT persist Body.Raw, header/param/query values), `LoadServers`, `SaveResponseExample`, `IsOpenAPIFile`, `UpdateOperationTags`.
- **`Glob`** — custom double-star (`**`) glob in `filepath.go` using `strings.Split` + `filepath.Walk`.

## TUI vs CLI

- **TUI** (`lazyapi` with no subcommand) — interactive bubbletea UI. Default mode.
- **CLI** (`lazyapi run|send|list|describe|validate|types|mcp|create|add|remove ...`) — headless commands. Dispatch: `cmd/lazyapi/main.go` sends anything in `cli.Commands` to `cli.Run()`, else starts `tea.NewProgram(app.NewTui(...))`.
- **Flag parsing** — every command uses the stdlib `flag` package via `newFlagSet` + `parseInterleaved` (flags allowed before/after positionals; unknown flags are errors). `main.go` still parses `--env` ad-hoc.
- **CLI subcommands**: `run <file>`, `send request <file> (<path> <method> | --op id)`, `smoke tests` (alias of run), `list ops`, `describe`, `validate spec`, `types`, `mcp`, plus `create file`, `add request|server`, `remove request`. `main.go` dispatches anything in `cli.Commands`. See `docs/cli/`, `docs/scripting.md`, `docs/agents.md`.
- **Run safety** — runs only `GET,HEAD,OPTIONS` unless `--methods` is given; skips operations whose path params have no value (never invents one); a status not declared in the operation's `responses` is a failed `status` check (exit 1).
- **Execution** — all checks live in `internal/runner`: request (`store.ValidateRequest` on `Request.HTTPRequest`, does not block sending) → status declared → contract (`store.ValidateResponse`) → script. `--no-contract` skips request + contract. CLI and MCP only render `runner.Report`/`OpResult`. Never print from runner.
- **Exit codes** — 0 ok, 1 a check failed, 2 usage, 3 could not run (errors win over failures). Return `*cli.ExitError` (`usageErr`, `runtimeErr`) instead of plain errors from commands.
- **before(req)** — `script.Before` evaluates the module once with side effects suppressed (test/console/vars/send are no-ops, top-level errors swallowed), then calls the exported `before` with the full API. The runner applies it before the request check and the send; if it throws, the request is not sent (failed `script` check).
- **after(res)** — `script.Run` calls the exported `after` with `response` once the top level finished (also when tests failed or it threw; not after a timeout). `Result.AfterRan/AfterError` → `runner.afterChecks` adds a `script` check "after() ran without errors". `Before` never calls it.
- **Flows** — `run --flow f.js` (only flows) or `--flows` (operations, then `flows/*.js`); MCP `run_flow`/`write_flow`. Results have `kind: "flow"` and `steps`.
- **HTTP** — one client per run from `model.HTTPOptions` (flags merged over the environment's `http:`), set on every request incl. `lazyapi.send`, flows and OAuth2 token calls. `SendTimed` returns DNS/connect/TLS/TTFB timing; results carry `timing` and `cookies`.
- **Import** — never copy credentials into specs (Authorization/API-key headers become security schemes plus a note). Credentials that are only `{{env.X}}` references go to `Result.Auth` → the environment's `auth:` in lazyapi.config.yml (edited with yaml.Node via `project.AddEnvironment`, never overwriting). Multipart files must never be read from request bodies (scripts/MCP could exfiltrate files).
- **Environments** — `project.Resolve(spec, --env-name, --server, --env)`: explicit flags win; secrets go to `runner.Options.Auth` → `model.ApplySecrets` (operations, `lazyapi.send`, flows, TUI). TUI `ctrl+v` cycles named envs when a config exists.
- **Scripts** — run per operation by `runner`: automatic contract check, then the operation's script via `script.Run`. URL imports are pinned in `lazyapi.lock` next to the spec (`--offline`, `--reload`, `--frozen-lockfile`, `--allow-import`). `lazyapi types` prints the script API. Lua is gone; `.lua` references produce an explicit error.
- **HTTP** — `Request.SendContext` applies `Request.Timeout` (default `model.DefaultTimeout`, 30s). All auth fields (incl. Basic) are template-resolved.

## TUI Pages and Overlays

- Pages (`config.PageIndex`): `RequestList`, `RequestEditor`, `Runner`, `History`. The runner view receives its non-key messages on every page (a run continues in the background).
- `ctrl+k` palette: `Tui.paletteCommands()` (internal/app/commands.go) builds the commands; their `Msg` goes back through `Tui.Update`. Add new actions there.
- Runs with writes (every method) and flows go through `Tui.confirmRun` (y/N), like destructive sends through `Tui.confirm`.
- History is recorded on every `responses.ResultMsg` (`ResultMsg.Request` is the unresolved request). Never store `Request.Env`, auth secrets or resolved bodies.
- `components.TemplateNames` feeds `{{` completions in every `components.Field` (→ accepts); `Tui.missingRefs()` lists references without a value for the editor help bar.
- `store.SampleBody`/`FillSampleBody` (schema → body) are for the editor only; `run` must never invent bodies.

## TUI Request Flow

- Editor (Send button, `alt+enter`, `F5` without a response) and list (`s`) emit `responses.SendMsg`. `Tui.startSend` owns a cancellable context (`esc` cancels), shows the spinner and runs `responses.Exchange` → `runner.Exchange` (status, contract, script checks). The result comes back as `responses.ResultMsg{Source}`.
- List sends use `store.LoadRequest(ref)` (temp edits, else spec with examples). Non-safe methods (POST/PUT/PATCH/DELETE) go through a y/N confirmation overlay.
- `Tui.capturesKeys()` (list filter, editor URI/tab content, response search) disables global shortcuts like `q`, `?`, `ctrl+v`.
- Environment: `ctrl+v` cycles `env.Discover(".")` via `env.Store.SetPath`; the title bar shows spec and env.
- Core packages (store, runner, script, mcp, …) must not import `internal/app` or `internal/components` (enforced by `internal/archtest`). `model.RequestItem` is the list item data; `app/pane/requests` aliases it and renders it (`RenderCompact`).

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
./lazyapi mock examples/openapi.yml &   # fake API on :4010
./lazyapi run examples/openapi.yml --methods all
go test ./...                        # Tests
golangci-lint run                    # Lint (config: .golangci.yml)
```

## Known Issues

- `config.DefaultConfig.Active` (current page) is package-level state: only one TUI program may run at a time (tests use `start`, which waits for the program to stop).

## Testing

- **CI** fails on unformatted files (`gofmt -l`) and when total coverage drops below 55% (per-package table in the job summary). `internal/archtest` fails if a core package imports `internal/app`/`internal/components`.

- **TUI end-to-end**: `internal/app/tui_test.go` runs the whole program with `teatest` against an `httptest` server. Use `start(t, spec)` (it waits for the program to stop: the active page is a package-level variable, so two programs must never overlap; it also points TMPDIR at the test dir, where drafts are written), `waitFor` for text that appears, and `finalScreen` for text that changes in place (the renderer redraws only changed cells, e.g. counters).

- **Examples are tests**: `internal/cli/examples_test.go` runs `examples/openapi.yml` + `examples/tests/*.js` against `mock.New` — keep them passing when changing the runner, script API or mock. CI also runs them against `lazyapi mock` and uploads a JUnit report.

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
