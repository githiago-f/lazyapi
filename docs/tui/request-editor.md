# Request Editor

Open a request from the list to enter the editor. It shows the request details across six tabs.

## Layout

```
┌───────────────────────────────────────────┐
│ Method    Server URL    URI       [Send]  │
├───────────────────────────────────────────┤
├─────────────────────┬─────────────────────┤
│ Docs Params Auth    │ 200 OK · 41 ms · ✓ 3│
│ Headers Body Tests  │ Body Headers Checks │
│                     │                     │
│  Tab content        │  Response           │
└─────────────────────┴─────────────────────┘
```

## Top controls

- **Method** — select from GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD
- **Server URL** — picked from the spec's `servers` list
- **URI** — the endpoint path, with `&#123;&#123;env.X&#125;&#125;` support
- **Send** — press `Enter` on the Send button, or `alt+enter` from any field. The response, status/contract checks and test results appear on the right ([Response View](./response-preview.md))

Opening an operation pre-fills path parameters, required query/header parameters and the request body from the spec's examples. Bodies can be edited in `$EDITOR` with `ctrl+x`.

When the spec declares a request body schema but no example, the editor builds one from the schema (examples, defaults, enums, formats and minimums of each property) so you start from a valid shape. `lazyapi run` never invents bodies: this only fills the editor.

## Templates

<code v-pre>{{env.NAME}}</code> and <code v-pre>{{var.NAME}}</code> work in every field. After typing <code v-pre>{{</code>, the field shows a completion from the env file and the active environment's vars; `→` accepts it. References without a value are listed in the bottom bar (<code v-pre>⚠ no value: {{env.TOKEN}}</code>), because they would be sent as empty strings. Vars set by scripts at run time (`vars.set`, `before()`) are not known before sending.

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

The request body, in the operation's media type:

- **JSON, text, XML, …** — a text area (`ctrl+x` opens it in `$EDITOR`).
- **`application/x-www-form-urlencoded` and `multipart/form-data`** — **Form fields**: one name/value row per field, like the Headers tab. `tab` / `shift+tab` move between cells, `ctrl+g` adds a field, a row without a name is dropped, and a repeated name sends the field several times. The rows are kept as a JSON object, which is encoded as form fields when sending; `ctrl+x` edits that object in `$EDITOR`. File parts are only sent from the CLI (`--file`).

Select the type from the dropdown.

### Tests

A JavaScript/TypeScript editor for the operation's test script (see [Test Scripts](../scripting.md)). Scripts can use `request`, `response`, `env`, `vars`, `lazyapi.send()`, `test()` and `expect()`, and may import relative files or `https://` modules.

Press `F5` to run the script against the last response. Results, failure messages and `console` output appear below the editor. When you save (`ctrl+o`), the script is written to `tests/<operationId>.js` next to the spec and linked through `x-lazyapi-tests`.

```js
test("status is 200", () => expect(response).toHaveStatus(200));
test("matches the OpenAPI schema", () => expect(response).toMatchSchema());
test("has an id", () => expect(response.json()).toHaveProperty("id"));
```

## Field navigation

- `Tab` / `Shift+Tab` cycles through: Method → Server → URI → Send → Tabs → Response
- `Enter` inside a tab activates tab content (Blocker mode)
- `Esc` blurs tab content; a second `Esc` exits the tab back to field cycling
- Within tab content (e.g., Params rows), Tab/Shift+Tab wraps cyclically

## Saving

- `ctrl+o` — saves changes back to the spec (Summary, Description, param definitions, body content type, auth schemes, test script)
- `Ctrl+E` — saves the last response as an example in the spec
- Runtime values (body content, header values, param values) are saved to session temp files
