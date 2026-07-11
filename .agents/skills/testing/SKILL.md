---
name: testing
description: >
  Use this skill for any testing-related work on LazyAPI.
  Covers unit tests, CLI integration tests, TUI smoke tests,
  fixture management, and CI workflows.
  Triggered by requests like "test the system", "add tests",
  "write a test for", "check if tests pass", "verify the build",
  "run tests", "fix tests", "add test coverage", "smoke test".
---

# LazyAPI Testing Skill

This skill describes how to test the LazyAPI system: a Go + Bubbletea CLI/TUI for OpenAPI-driven API exploration.

## When This Skill MUST Be Used

- Writing or running unit tests
- Verifying CLI commands (create, add, remove, send)
- Smoke-testing the TUI (binary launches, renders, exits cleanly)
- Adding or modifying test fixtures in `internal/store/testdata/`
- Setting up CI workflows or test automation
- Debugging test failures

## Project Overview

LazyAPI has two modes:

- **TUI** (default) — Bubbletea interactive UI with a request list and editor
- **CLI** — Headless subcommands: `create`, `add`, `remove`, `send`, `smoke`

The core data model revolves around OpenAPI 3.x specs stored as `.yml`/`.yaml` files.

```
cmd/lazyapi/main.go              # Entry point (TUI or CLI)
internal/
  cli/                           # CLI subcommands
  store/                         # OpenAPI spec CRUD operations
  env/                           # Environment variable resolution
  model/                         # Core types (Request, Method, Auth, etc.)
  app/                           # Bubbletea models and views
  components/                    # Reusable UI widgets
  config/                        # Colors, keybindings, constants
  inmath/                        # Math helpers (Circle for focus cycling)
```

## Unit Tests

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests in a specific package
go test -v ./internal/store/...

# Run a specific test
go test -v -run TestAuthRoundtrip_FileSaveAndReload ./internal/store/

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html  # View in browser
```

### Test Structure

Tests live next to the code they test. Convention: `foo_test.go` tests `foo.go`.

```go
// File: internal/store/openapi_test.go
package store

import (
    "path/filepath"
    "testing"
    "github.com/githiago-f/lazyapi/internal/model"
)

func fixturePath(name string) string {
    return filepath.Join("testdata", name)
}
```

### Test Fixtures

Fixtures are static `.yml` files in `internal/store/testdata/`.

| File | Purpose |
|------|---------|
| `minimal.yml` | Basic CRUD spec with `/items`, `/items/{id}` paths and a server URL |
| `global_security.yml` | Spec with global `ApiKeyAuth` + `BearerAuth` and per-operation overrides on `/public` |

**Rules:**
- Use `fixturePath(name)` to reference fixtures, never hardcode paths
- Prefer programmatic spec construction for small isolated tests
- Use fixtures for realistic multi-operation specs and auth scenarios
- **Never use `/tmp`** — use `t.TempDir()` for ephemeral file roundtrips
- All fixtures must have `openapi: 3.0.0` at the root

**Adding a new fixture:**
1. Create a `.yml` file in `internal/store/testdata/`
2. Ensure it starts with `openapi: 3.0.0` and has at least an `info` and `paths` section
3. Reference it in tests with `fixturePath("your-fixture.yml")`

### Test Patterns

The codebase uses **roundtrip patterns** — marshal to YAML, write, read back, unmarshal, and verify.

```go
// Example: auth roundtrip
spec, _ := ParseSpec(fixturePath("minimal.yml"))
// Apply auth
ApplyRequestToOperation(spec, ref, model.Request{Auth: auth})
// Save to temp file
tmpDir := t.TempDir()
tmpFile := filepath.Join(tmpDir, "spec.yml")
SaveSpec(tmpFile, spec)
// Reload and verify
reloaded, _ := ParseSpec(tmpFile)
result := OperationToRequest(reloaded, ref)
```

Tests validate the **store layer** and **model layer** — never UI interaction.

### Auth Secrets Test

There is a specific test (`TestAuthSecrets_NotPersistedInSpec`) that verifies sensitive fields (`Password`, `Token`, `AccessToken`, `ClientSecret`, `KeyValue`) are NOT written to the OpenAPI YAML spec when serialized. When adding new secret fields, add them to this test's list.

## Linting

```bash
golangci-lint run
```

The project uses `golangci-lint` v2 with `standard` linters. The config is in `.golangci.yml` at the project root.

**Known linter issues (3 SA1019 deprecations in `internal/components/scrollable/scrollable.go`):**
- `msg.Type` → use `MouseAction` & `MouseButton`
- `tea.MouseWheelUp` → use new API
- `tea.MouseWheelDown` → use new API

## Building

```bash
go build -o lazyapi ./cmd/lazyapi
# Or just check it compiles without producing a binary
go build ./cmd/lazyapi
```

## CLI Integration Tests

These test the headless CLI subcommands without a terminal. They verify parsing, file operations, and error handling.

### Smoke Test Scenarios

#### 1. Create a new spec file

```bash
go build -o lazyapi ./cmd/lazyapi
TMPFILE=$(mktemp /tmp/lazyapi-test-XXXX.yml)
./lazyapi create file "$TMPFILE" https://api.example.com

# Verify
head -5 "$TMPFILE"
# Should show openapi: 3.0.0 with servers section

# Verify invalid usage exits with error
./lazyapi create 2>/dev/null && echo "FAIL: should have exited" || echo "PASS: exits on missing args"
```

#### 2. Add a request

```bash
./lazyapi add request "$TMPFILE" /pets GET

# Verify the operation was added
./lazyapi send request "$TMPFILE" /pets GET --server https://api.example.com 2>&1 | head -3
# Should show "Server: https://api.example.com" and "Status:" output

# Verify error on non-existent file
./lazyapi add request /nonexistent/file.yml /test GET 2>/dev/null && echo "FAIL" || echo "PASS"
```

#### 3. Add a server

```bash
./lazyapi add server "$TMPFILE" https://staging.example.com

# Verify via send output
./lazyapi send request "$TMPFILE" /pets GET 2>&1 | grep -q "Server:" && echo "PASS" || echo "FAIL"
```

#### 4. Send a request

```bash
# Dry run check — the actual HTTP request will fail if the server isn't running,
# but the parsing and setup should work.
./lazyapi send request "$TMPFILE" /pets GET --server https://httpbin.org 2>&1
# With a real server, use --save-example to persist the response
```

#### 5. Remove a request

```bash
./lazyapi remove request "$TMPFILE" GET /pets
# Verify removed
./lazyapi send request "$TMPFILE" /pets GET --server https://api.example.com 2>&1 | grep -q "not found" && echo "PASS" || echo "FAIL"
```

#### 6. Smoke tests (stub)

```bash
./lazyapi smoke tests "$TMPFILE" --server https://api.example.com
# Should print "Smoke tests are not implemented yet"
```

### Expected CLI Exit Codes

| Scenario | Exit Code |
|----------|-----------|
| Successful command | 0 |
| Invalid arguments | 1 (via `os.Exit(1)`) |
| File not found | 1 |
| Invalid OpenAPI file | 1 |
| Operation not found | 1 |

## TUI Smoke Tests

The TUI uses Bubbletea, which requires a terminal. Testing strategies:

### Strategy 1: Verify Binary Starts and Exits

```bash
go build -o lazyapi ./cmd/lazyapi

# Run TUI with a timeout and send Ctrl+C to exit
timeout 2 ./lazyapi "$TMPFILE" 2>&1 || true
# If it runs for 2 seconds without crashing, the TUI initializes correctly.

# With key input via expect-like pattern (using a temp PTY if available)
# On Linux with `script`:
timeout 2 script -q -c "./lazyapi $TMPFILE" /dev/null </dev/null 2>&1 || true
```

### Strategy 2: Quick-Render Verification

The TUI renders immediately on start. A binary that segfaults or panics on init will fail within milliseconds. Use a short timeout to confirm the view renders:

```bash
# This should NOT hang — if it does, the TUI is stuck at Init
timeout 1 bash -c './lazyapi "$1" & sleep 0.5; kill %1' _ "$TMPFILE" 2>&1
```

### Strategy 3: Structured Testing with `$TERM` and `$COLUMNS`/`$LINES`

Bubbletea reads terminal size. For headless environments, set dummy values:

```bash
COLUMNS=80 LINES=24 TERM=xterm-256color timeout 2 ./lazyapi "$TMPFILE" 2>&1 || true
```

## Full CI Script

Here is a complete script that exercises the full test suite:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=== Build ==="
go build -o lazyapi ./cmd/lazyapi

echo "=== Unit Tests ==="
go test -race -count=1 ./...

echo "=== Lint ==="
golangci-lint run

echo "=== CLI Integration ==="
TMPFILE=$(mktemp /tmp/lazyapi-XXXX.yml)
trap 'rm -f "$TMPFILE"' EXIT

# 1. Create file
./lazyapi create file "$TMPFILE" https://api.example.com
grep -q "openapi: 3.0.0" "$TMPFILE" || { echo "FAIL: create"; exit 1; }
echo "PASS: create"

# 2. Add GET request
./lazyapi add request "$TMPFILE" /items GET
grep -A1 "/items:" "$TMPFILE" | grep -q "get" || { echo "FAIL: add GET"; exit 1; }
echo "PASS: add GET"

# 3. Add POST request
./lazyapi add request "$TMPFILE" /items POST
echo "PASS: add POST"

# 4. Send request (verify it parses correctly — actual HTTP may fail)
./lazyapi send request "$TMPFILE" /items GET --server https://api.example.com 2>&1 | grep -q "Server:" || { echo "FAIL: send"; exit 1; }
echo "PASS: send"

# 5. Remove GET request
./lazyapi remove request "$TMPFILE" GET /items
./lazyapi send request "$TMPFILE" /items GET --server https://api.example.com 2>&1 | grep -q "not found" || { echo "FAIL: remove"; exit 1; }
echo "PASS: remove"

# 6. Add server
./lazyapi add server "$TMPFILE" https://staging.example.com
echo "PASS: add server"

# 7. Smoke tests (stub)
./lazyapi smoke tests "$TMPFILE" 2>&1 | grep -q "not implemented" || { echo "FAIL: smoke"; exit 1; }
echo "PASS: smoke (stub)"

# 8. Verify invalid args
./lazyapi 2>&1 | grep -q "Usage" || { echo "FAIL: no args"; exit 1; }
echo "PASS: usage on no args"

echo ""
echo "=== ALL TESTS PASSED ==="
```

## Known Testing Gaps

- **TUI has no UI tests** — Bubbletea is hard to test programmatically without a terminal emulator
- **No CLI flag validation tests** — ad-hoc arg parsing means edge cases (duplicate flags, missing values) are untested
- **No HTTP roundtrip mocks** — `model.Request.Send()` calls `http.DefaultClient.Do()` directly with no mock injection point
- **No env/scenario tests** — merging system env with `.env` file, hash caching behavior, `ForceReload`
- **No concurrent access tests** — the `Store` type comments say "thread-safe for single-goroutine TUI use" but has no tests proving correctness
- **`smoke tests` is unimplemented** — only prints a stub message

## Writing Good Tests

### DOs

- Use `t.TempDir()` for ephemeral files
- Use `fixturePath()` for testdata references
- Test roundtrip: marshal → write → read → unmarshal → verify
- Use `t.Run()` subtables for multiple related scenarios
- Test both success and error paths

### DON'Ts

- Don't use `/tmp` directly
- Don't test UI rendering in unit tests
- Don't hardcode file paths
- Don't depend on external servers (no network calls in unit tests)

### Example: Adding a new test

```go
func TestMyFeature(t *testing.T) {
    spec, err := ParseSpec(fixturePath("minimal.yml"))
    if err != nil {
        t.Fatalf("ParseSpec: %v", err)
    }

    // ... test logic ...

    tmpDir := t.TempDir()
    tmpFile := filepath.Join(tmpDir, "spec.yml")
    if err := SaveSpec(tmpFile, spec); err != nil {
        t.Fatalf("SaveSpec: %v", err)
    }

    reloaded, err := ParseSpec(tmpFile)
    if err != nil {
        t.Fatalf("ParseSpec (reload): %v", err)
    }

    // ... verify reloaded data matches ...
}
```

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| `IsOpenAPIFile` returns false | Missing `openapi:` or `swagger:` root key | Add `openapi: 3.0.0` to fixture |
| `go test` fails with YAML errors | Invalid fixture or missing `info` field | Validate fixture with `openapi3.Loader` |
| Binary hangs on launch | Terminal size detection fails | Set `COLUMNS=80 LINES=24` |
| Linter not found | `golangci-lint` not installed | Run `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |
| Segfault on TUI init | Bubbletea model returns nil from `Update` | Check that all model fields are initialized |
