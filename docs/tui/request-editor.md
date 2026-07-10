# Request Editor

Open a request from the list to enter the editor. It shows the request details across six tabs.

## Layout

```
┌───────────────────────────────────────────┐
│ Method    Server URL    URI       [Send]  │
├───────────────────────────────────────────┤
│ [Doc] [Params] [Auth] [Header] [Body] [Tests] │
├───────────────────────────────────────────┤
│                                           │
│  Tab Content                              │
│                                           │
├───────────────────────────────────────────┤
│  Response Preview                         │
└───────────────────────────────────────────┘
```

## Top controls

- **Method** — select from GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD
- **Server URL** — picked from the spec's `servers` list
- **URI** — the endpoint path, with `&#123;&#123;env.X&#125;&#125;` support
- **Send** — press `Enter` on the Send button to execute the request

## Tabs

### Documentation

A free-form **Summary** (single line) and **Description** (multi-line) for the operation. These are persisted back to the OpenAPI spec on save.

### Params

Two sub-sections:

- **Query parameters** — key/value pairs sent as URL query string. Add with `n` + `q`.
- **Path parameters** — key/value pairs that replace `{param}` placeholders in the URI. Add with `n` + `p`.

### Authorize

Configure authentication schemes extracted from the spec. Add a scheme with `n` + `a`, remove with `x` + `a`. Supports:

- **Basic** — Username + Password
- **Bearer** — Token
- **API Key** — Key Name + Key Value + In (header/query)
- **OAuth2** — Client ID + Client Secret + Token URL

### Header

Custom HTTP headers as key/value pairs. Add with `n` + `h`. Common headers like `Content-Type` are set automatically from the body type.

### Body

The request body textarea. Supports two content types:

- **`application/json`** — format your JSON here
- **`text/plain`** — plain text

Select the type from the dropdown.

### Tests

Planned feature for response assertions. Currently a placeholder.

## Field navigation

- `Tab` / `Shift+Tab` cycles through: Method → Server → URI → Send → Tabs → Response
- `Enter` inside a tab activates tab content (Blocker mode)
- `Esc` blurs tab content; a second `Esc` exits the tab back to field cycling
- Within tab content (e.g., Params rows), Tab/Shift+Tab wraps cyclically

## Saving

- `Ctrl+S` — saves changes back to the spec (Summary, Description, param definitions, body content type, auth schemes)
- `Ctrl+E` — saves the last response as an example in the spec
- Runtime values (body content, header values, param values) are saved to session temp files
