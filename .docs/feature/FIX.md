# FIX — Code Quality Issues

Based on a systematic review of `./internal/` and `./cmd/` against DRY, SOLID, and KISS principles, plus `golangci-lint` output.

---

## 🔴 Errors (will cause bugs or data loss)

### E1 — `LoadForDuplicate`: file closed before read

**File:** `internal/store/file_system.go:279-317`

**Problem:** The draft file is opened, immediately closed, then `yaml.NewDecoder(file)` is called on the closed handle. The decode always fails silently.

```go
file, err := os.Open(item.DraftPath)  // opened
// ...
err = file.Close()                     // closed
// ...
decoder := yaml.NewDecoder(file)       // BUG: closed file
if err := decoder.Decode(&req); err != nil {
```

**Fix:** Move the `Close()` call to after the `Decode` completes (preferably a `defer`).

---

### E2 — `SaveFile` (else branch): file closed before write

**File:** `internal/store/file_system.go:362-381`

**Problem:** Same pattern as E1 — the created file is closed, then `yaml.NewEncoder(file)` writes to the closed handle. Data in this branch is never saved.

```go
file, err := os.Create(data.FileName)  // created
// ...
err = file.Close()                      // closed
// ...
encoder := yaml.NewEncoder(file)        // BUG: closed file
err = encoder.Encode(data)
```

**Fix:** Move `Close()` after the `Encode` completes.

---

### E3 — Highlight styling reset every frame

**Files:** `internal/app/pane/editor/header.go:50-51`, `internal/app/pane/editor/params.go:85-86`

**Problem:** Both `header.View()` and `params.View()` create fresh `lipgloss.NewStyle()` for each field's name/value each frame, then later set `BorderForeground` for the focused field. Because the style is brand-new, the `BorderForeground` applied earlier in the same loop is lost — focus highlighting only works because `View()` is called in a specific order. Fragile and wasteful.

**Fix:** Set the base width style once in `Update` (on `tea.WindowSizeMsg`) instead of recreating in `View`.

---

## 🟡 Warnings (design debt, duplication, dead code)

### W1 — Method string duplication (DRY)

**Files:**
- `internal/model/method.go:54-72` — `Label()` switch
- `internal/model/method.go:28-52` — `UnmarshalYAML()` switch
- `internal/store/openapi.go:79-98` — `methodFromLabel()` switch
- `internal/store/openapi.go:287-303` — `RemoveOperationFromSpec()` hardcoded strings
- `internal/store/openapi.go:314-352` — `AddOperationToSpec` parameter creation

**Impact:** Adding a new HTTP method (e.g. `TRACE`) requires changes in 5 locations. String literals like `"GET"`, `"POST"` appear in `RemoveOperationFromSpec` instead of using `model.Method` constants.

**Suggested fix:** Eliminate `methodFromLabel()` — use `model.Method.Label()` instead. Replace hardcoded strings in `RemoveOperationFromSpec` with `model.Method` reflection or a lookup map.

---

### W2 — Parameter-ref creation duplicated

**File:** `internal/store/openapi.go`

**Problem:** `AddOperationToSpec` (lines 325-352) and `ApplyRequestToOperation` (lines 209-249) both create `openapi3.ParameterRef` for path/query/header params with identical schema setup (`{Type: &openapi3.Types{"string"}}`). ~60 lines of duplication.

**Suggested fix:** Extract a helper like `makeParamRef(name, in string, required bool)`.

---

### W3 — Env line-parsing logic duplicated

**File:** `internal/env/loader.go`

**Problem:** `Load()` (lines 30-41) and `mergeDotenv()` (lines 113-125) have identical `SplitN("=", 2)` / `TrimSpace` / `Trim("\"'")` logic.

**Suggested fix:** Extract a `parseEnvLine(line string) (key, value string, ok bool)` helper.

---

### W4 — Auto-save on every Update

**File:** `internal/app/pane/editor/editor.go:415-417`

**Problem:** `store.SaveTempFile` is called on every `Update()` that doesn't match a keybinding — including `tea.WindowSizeMsg`, mouse events, etc. Constant disk writes.

**Suggested fix:** Track a dirty flag; only save when data has actually changed.

---

### W5 — Map iteration order for param substitution

**File:** `internal/model/request.go:73-75`

**Problem:** `strings.ReplaceAll` is called in a `for range` over a map, so param substitution order is non-deterministic. If one param name is a substring of another (e.g. `id` → `1` and `user_id` → `foo`), result varies.

**Suggested fix:** Sort param keys before iterating.

---

### W6 — Dead / stub code

| File | Status |
|------|--------|
| `internal/components/modal.go` | Empty (package declaration only) |
| `internal/app/pane/editor/tests.go` | Stub — returns static `"Tests"` string |
| `internal/app/pane/responses/preview.go` | Never used; editor uses `viewport.Model` directly |
| `internal/components/view.go` | `View` struct + `InitView` — never imported |

---

### W7 — `inmath.Circle` typo

**File:** `internal/inmath/counters.go`

**Problem:** Function is named `Circle` (should be `Circle`). The `AGENTS.md` already documents this.

---

### W8 — `Selector.Value()` fails silently

**File:** `internal/components/selector.go:23-28`

**Problem:** If `Cursor` is out of bounds, returns `""` with no indication. Callers assume valid data.

---

### W9 — Auth secrets co-located with schema metadata

**File:** `internal/model/auth.go`

**Problem:** `AuthScheme` holds both schema-definition fields (Type, SchemeName, KeyIn, GrantType, etc.) and runtime secrets (Password, Token, AccessToken, ClientSecret, KeyValue) in the same struct. `SaveFile`'s third branch writes the full `model.Request` to disk, potentially leaking secrets into temp files.

---

## ℹ️ Linter Output

```
golangci-lint run → 3 issues (all SA1019 deprecations)
```

**File:** `internal/components/scrollable/scrollable.go:44-48`

```
msg.Type is deprecated → use MouseAction & MouseButton instead
tea.MouseWheelUp is deprecated → use MouseAction & MouseButton instead
tea.MouseWheelDown is deprecated → use MouseAction & MouseButton instead
```

**Fix:** Migrate from the deprecated `tea.MouseWheelUp`/`tea.MouseWheelDown` constants to the new `tea.MouseAction`/`tea.MouseButton` API.

---

## 📊 Priority Matrix

| ID | Severity | Effort | Impact | Suggested Order |
|----|----------|--------|--------|-----------------|
| E1 | Error | Small | High — duplicates silently fail | 1 |
| E2 | Error | Small | High — saves silently fail | 2 |
| E3 | Error | Small | Medium — visual glitch | 3 |
| W1 | Warning | Medium | Medium — maintenance burden | 4 |
| W2 | Warning | Medium | Medium — duplicate code | 5 |
| W4 | Warning | Tiny | Low — performance | 6 |
| W5 | Warning | Tiny | Low — edge case | 7 |
| W6 | Warning | Tiny | Low — cleanup | 8 |
| W3 | Warning | Tiny | Low — readability | 9 |
| Lint | Info | Small | Low — future-proofing | 10 |
| W7-9 | Warning | Various | Low — design debt | 11 |
