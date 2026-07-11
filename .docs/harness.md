# Delivery Harness

This harness is a **mandatory prerequisite** for every code delivery. The agent MUST execute every check below before declaring work complete.

---

## 1. DRY — Don't Repeat Yourself

- [ ] No identical switch/if chains across files (especially method strings, parameter creation blocks)
- [ ] No copy-pasted struct initializations — extract to constructors
- [ ] No duplicated line-parsing, string-munging, or schema-building logic
- [ ] If a pattern appears in ≥3 places, extract it into a helper or type method

## 2. KISS — Keep It Simple, Stupid

- [ ] Each function does exactly one thing (single responsibility within the function)
- [ ] No deeply nested conditionals >3 levels — flatten with early returns or guard clauses
- [ ] No unnecessary abstractions (interfaces with one impl, factories that always return the same type)
- [ ] No channels, goroutines, or mutexes for single-goroutine Bubbletea code
- [ ] The simplest correct solution wins — no premature optimization

## 3. SOLID

### Single Responsibility
- [ ] Every package owns one concern (store owns persistence, model owns types, components own widgets)
- [ ] No package imports from sibling view layers (e.g. `store` must not import `app/pane/editor`)
- [ ] `tea.Cmd` wrappers belong in `store/file_system.go`, not scattered across packages

### Open/Closed
- [ ] Adding a new HTTP method requires touching exactly **one** place (the `model.Method` iota + Label)
- [ ] Adding a new component type doesn't require modifying existing component code
- [ ] New tab content types implement a consistent interface (`Init/Update/View` + `SetActive`/`IsActive`)

### Liskov Substitution
- [ ] All `tea.Model` implementations handle the full message contract — no silent drops
- [ ] Tab content respects the `SetActiveTabMsg` contract (focus/blur correctly)
- [ ] `components.Field`, `components.Text`, `components.Selector` are interchangeable in their role

### Interface Segregation
- [ ] Interfaces are small and focused (no "god" interfaces)
- [ ] Bubbletea's `tea.Model` is the only wide interface — don't add another
- [ ] Tab content uses `SetActive(bool)` / `IsActive() bool`, not a monolithic `TabContent` interface

### Dependency Inversion
- [ ] High-level policies (editor, request list) depend on abstractions, not concrete file IO
- [ ] `tea.Cmd` wrappers in `file_system.go` abstract away file operations
- [ ] Don't call `os.*` directly from UI code — go through `store` package

## 4. Architecture Compliance

### Bubbletea Rules
- [ ] `Model` struct contains only state; side effects go in `tea.Cmd`
- [ ] `Update` returns `(tea.Model, tea.Cmd)` — never blocks, never calls `os.Exit` (except in `main`)
- [ ] `View` is pure — no mutations, no IO, no side effects
- [ ] All background work is wrapped in `func() tea.Msg { ... }` closures, never raw goroutines
- [ ] Window resize messages are forwarded to all child models that need sizing
- [ ] Key events follow the established Tab/Shift+Tab / Esc / Enter navigation pattern

### Lipgloss Rules
- [ ] Styles are built once (in `init()` or constructors) and reused, not rebuilt every frame
- [ ] No `lipgloss.NewStyle()` inside `View()` — mutate existing styles with `.Inherit()`
- [ ] Color constants come from `config/config.go` (Catppuccin Mocha palette) — no hardcoded hex strings
- [ ] Style mutations in `View()` must not discard state set earlier in the same frame

### Store Package Rules
- [ ] All synchronous store functions are in `openapi.go`; `tea.Cmd` wrappers are in `file_system.go`
- [ ] No `tea` imports in `openapi.go` — it must be usable from both TUI and CLI
- [ ] Temp files use `os.TempDir()/lazyapi/` prefix via `tempDirForFile()`
- [ ] Secrets (Password, Token, AccessToken, ClientSecret, KeyValue) are never written to the OpenAPI YAML spec

## 5. Testability

- [ ] Pure logic has unit tests (store roundtrips, model marshaling, env resolution)
- [ ] Tests use `t.TempDir()` — never `/tmp`
- [ ] Tests use `fixturePath()` from `internal/store/testdata/` for YAML fixtures
- [ ] `go test ./...` passes
- [ ] `golangci-lint run` passes (or only known pre-existing warnings remain)
- [ ] No new exported symbols without a test

## 6. System Integrity

- [ ] `go build ./cmd/lazyapi` compiles without errors
- [ ] CLI commands produce correct output for both success and error cases
- [ ] TUI initializes without panic (verify with `timeout 2 ./lazyapi`)
- [ ] Draft→Save→Reopen cycle works (data persists correctly)
- [ ] Temp file close-before-use bug pattern is not reintroduced (see `.docs/feature/FIX.md` E1, E2)

## Pre-Delivery Checklist

The agent MUST run these commands and confirm they pass before marking work complete:

```bash
go build ./cmd/lazyapi
go test ./...
golangci-lint run
```

Then self-review against all sections above and report anything that fails.
