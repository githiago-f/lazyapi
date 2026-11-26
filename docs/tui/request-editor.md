# Request Editor

Open a request from the list to enter the editor. It shows the request details across six tabs.

## Layout

<Screen name="request-editor" caption="The editor after alt+enter: request tabs on the left, the response on the right" />

## Top controls

- **Method** — select from GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD
- **Server URL** — picked from the spec's `servers` list
- **URI** — the endpoint path, with `{{env.X}}` support
- **Send** — press `Enter` on the Send button, or `alt+enter` from any field. The response, status/contract checks and test results appear on the right ([Response View](./response-preview.md))

Opening an operation pre-fills path parameters, required query/header parameters and the request body from the spec's examples. Bodies can be edited in `$EDITOR` with `ctrl+x`.

When the spec declares a request body schema but no example, the editor builds one from the schema (examples, defaults, enums, formats and minimums of each property) so you start from a valid shape. `lazyapi run` never invents bodies: this only fills the editor.

## Mouse and selects

Everything can be clicked: the method and server selects, the URI (places the cursor), **Send**, the tab labels, fields inside a tab (focuses them), the code editors (click places the cursor, drag selects, shift+click extends) and the response. The mouse wheel scrolls the tab under the pointer, an open list, or the response. In the request list, a click selects and a double click opens.

Selects show `▾`. `enter`/`space` or a click opens the list; `↑`/`↓`, `home`/`end`, `pgup`/`pgdown`, the wheel or typing the first letters (`p` → POST, `p` again → PATCH) move; `enter`, `tab` or a click picks; `esc` or a click outside cancels. Closed, `↑`/`↓` change the value directly.

## Switching rows on and off

Every header, query parameter, path parameter and form field has a checkbox (☑ on, ☐ off), and every auth scheme an **on/off** switch on its first line. `ctrl+t` on the focused row (or a click on the box) toggles it. A row that is off keeps its name and value but is not sent; auth schemes that are off are not applied.

The state is saved with the request's draft in `.lazyapi/.local/drafts/` (under `disabled:`, and `disabled: true` on auth schemes), so it survives restarts and is used by `s` in the list. Saving to the spec (`ctrl+o`) keeps the parameter and scheme definitions of rows that are off; `lazyapi run` reads the spec and is not affected.

## Templates

`{{env.NAME}}` and `{{var.NAME}}` work in every field and editor, and are colored as you type: teal when the active environment has a value, **red underlined** when it does not.

- Type `{{` or the shortcut `$` (`$TOK`) to open the completion menu: the env file's variables and the environment's vars, with values masked when they look like credentials. `↑`/`↓` choose, `tab` (or `enter`) inserts `{{env.TOKEN}}`, `esc` closes the menu.
- References without a value are also listed in the bottom bar (`⚠ no value: {{env.TOKEN}}`), because they would be sent as empty strings. Vars set by scripts at run time (`vars.set`, `before()`) are not known before sending.
- A password field shows its value when it is only references (`{{env.PASSWORD}}` holds no secret).
- The URL field also colors the host, `{path}` parameters and query keys.

## Tabs

### Documentation

A free-form **Summary** (single line) and **Description** (multi-line) for the operation. These are persisted back to the OpenAPI spec on save.

### Params

Two sub-sections:

- **Query parameters** — key/value pairs sent as URL query string. Add with `n` + `q`.
- **Path parameters** — key/value pairs that replace `{param}` placeholders in the URI. Add with `n` + `p`.

### Auth

Authentication schemes of the request. `ctrl+r` adds one, `⌫`/`del` on its **Scheme** select removes it. **Scheme** reuses a scheme the spec already defines (so several requests share one definition) or creates a new one. Supports:

- **Basic** — Username + Password
- **Bearer** — Token
- **API Key** — Key name + In (header/query/cookie) + Value
- **OAuth2** — Grant, Client ID/secret, Authorization/Token URL, Scopes
- **OpenID Connect** — Issuer URL (endpoints discovered from `.well-known/openid-configuration`), Grant, Client ID/secret, Scopes

OAuth schemes have a **Login** button (`ctrl+l`) and show the stored session. Sending without a session logs in first. See [Authentication](/authentication).

### Header

Custom HTTP headers as key/value pairs. Add with `n` + `h`. Common headers like `Content-Type` are set automatically from the body type.

### Body

The request body. A select at the top of the tab picks its type: `none`, the types the operation declares in the spec, then `application/json`, `application/xml`, `text/plain`, `application/x-www-form-urlencoded`, `multipart/form-data` and `application/octet-stream`. `alt+t` (or a click) opens it from anywhere in the tab; in forms and file bodies `tab` also reaches it. Changing the type carries the content over: a JSON object becomes form rows, and form rows become a JSON object.

- **none** — no body is sent (nor a `requestBody` added to the spec on save).
- **JSON, text, XML, …** — the code editor (below): JSON is colored, validated as you type (templates count as values) and formatted with `alt+f`. `ctrl+x` opens it in `$EDITOR`.
- **`application/x-www-form-urlencoded` and `multipart/form-data`** — **Form fields**: one name/value row per field, like the Headers tab. `tab` / `shift+tab` move between cells, `ctrl+g` adds a field, a row without a name is dropped, and a repeated name sends the field several times. The rows are kept as a JSON object, which is encoded as form fields when sending.
  In a multipart form, `ctrl+f` turns the focused row into a **file part** (its value, marked `@`, is the path of a local file) and back into text. Dropping files on the form makes the focused row a file part and adds a row for each other file.
- **Binary types** (`application/octet-stream`, `image/png`, `application/pdf`, …, shown with `(file)`) — a file path: the file is sent as the whole body, with the type as its `Content-Type`. The tab shows its size, or `✗ file not found`. Dropping a file on a body set to `none` makes it a binary body with that file.

File paths stay in the request's draft (`.lazyapi/.local/drafts/`, git-ignored). They are never written to the spec, and never taken from the body text, scripts or MCP: only you choose which files are sent.

## Copy, paste and dropping files

- `ctrl+c` in a field with a value (URI, headers, params, auth, form rows, summary) copies the whole value; in a code editor it copies the selection. With nothing to copy, `ctrl+c` quits as everywhere.
- `ctrl+v` pastes the system clipboard into the focused field or editor; the terminal's own paste (`ctrl+shift+v`, right click) works too. Line breaks pasted into a one-line field become spaces.
- The clipboard is reached through `wl-clipboard`, `xclip` or `xsel` on Linux. Under WSL without them, lazyapi uses the Windows clipboard through `powershell.exe`. When no clipboard is reachable (SSH, for example), copies go through the terminal (OSC 52), which most terminals support.
- **Drag and drop**: dropping files on the terminal window pastes their paths, and lazyapi recognizes them in the format every terminal uses: quoted (`'/a b.png'`, `"C:\a b.png"`), with escaped spaces (`/a\ b.png`), as `file://` URIs, one per line or space-separated. Under WSL, Windows paths (`C:\…`, `\\wsl.localhost\…`) become WSL paths. Terminals that deliver a drop as typed text rather than as a paste (Windows Terminal, some tmux setups) work too: keys that arrive within a few milliseconds of each other and spell existing files are taken as a drop, so the letters of the path never trigger shortcuts. In a text field the drop inserts the bare path; on the Body tab it sets the files to send (above). Dropped anywhere else in the editor (the method, Send, the tab bar, the response), files go to the Body tab.

  Dropping needs a terminal that turns a drop into text: Windows Terminal, GNOME Terminal, Konsole, kitty, WezTerm, iTerm2 and most others do (inside tmux too). Under WSL, use Windows Terminal: it gives WSL paths (`/mnt/c/…`) for Windows files. Some bridges from Windows, such as kitty running through an adapter, do not deliver drops at all; there, paste the path (`ctrl+v`) or type it.

### Tests

The code editor, in JavaScript, for the operation's test script (see [Test Scripts](../scripting.md)). Scripts can use `request`, `response`, `env`, `vars`, `lazyapi.send()`, `test()` and `expect()`, and may import relative files or `https://` modules.

Press `F5` to run the script against the last response. Results, failure messages and `console` output appear below the editor. When you save (`ctrl+o`), the script is written to `.lazyapi/tests/<operationId>.js` next to the spec and linked through `x-lazyapi-tests`.

```js
test("status is 200", () => expect(response).toHaveStatus(200));
test("matches the OpenAPI schema", () => expect(response).toMatchSchema());
test("has an id", () => expect(response.json()).toHaveProperty("id"));
```

## Code editor

Body, Tests and the Docs description use the same editor:

- syntax colors (Catppuccin), line numbers with the current line highlighted, and a status line with the language, `Ln, Col` and, for JSON, `✓ valid` or the parse error. The line with the error gets a red `●`;
- `{`, `[`, `(` and quotes close themselves; typing the closer steps over it; `backspace` between a pair removes both; `enter` keeps the indentation and opens an indented line between brackets;
- `tab` / `shift+tab` indent and dedent (two spaces);
- `alt+f` formats JSON (key order kept);
- `ctrl+z` / `ctrl+y` undo and redo; pasting keeps line breaks;
- **selection**: `shift` + any movement (arrows, `home`/`end`, `alt+←/→`, `pgup`/`pgdown`) selects, `alt+a` selects everything, `esc` clears it. Typing replaces the selection, and an opening bracket or quote wraps it; `tab` / `shift+tab` indent or dedent the selected lines; `backspace` deletes it;
- **clipboard**: `ctrl+c` copies the selection (with nothing selected it quits, as everywhere), `alt+x` cuts, `ctrl+v` (or `alt+v`) pastes from the system clipboard, replacing the selection (terminal paste works too);
- `home` goes to the indentation, then column 0; `ctrl+home` / `ctrl+end`; `alt+←` / `alt+→` move by word; `ctrl+w` deletes the word before the cursor; `ctrl+u` deletes to the line start;
- in Tests, syntax errors as you type (JavaScript and TypeScript, checked with esbuild: the line gets a red `●`, the message goes to the status line) and the **signature** of the call the cursor is in (`ƒ lazyapi.send(target, options?)`, `toHaveProperty(path, value?)`, …);
- dynamic values: `{{$` (or `$name`) offers the built-in faker variables and the helpers the Tests script exports, with a description; unknown names are drawn as missing and listed in the help bar;
- in Tests, completions for the script API as you type: `response.` (`status`, `json()`, …), `expect(…).` (every matcher), `lazyapi.`, `env.`, `vars.` and the globals. Methods insert `()` with the cursor inside.

`esc` closes an open menu first, then leaves the editor.

## Field navigation

- `Tab` / `Shift+Tab` cycles through: Method → Server → URI → Send → Tabs → Response
- `Enter` inside a tab activates tab content (Blocker mode)
- `Esc` blurs tab content; a second `Esc` exits the tab back to field cycling. On an open select, `Esc` only closes the list
- Within tab content (e.g., Params rows), Tab/Shift+Tab wraps cyclically

## Saving

- `ctrl+o` — saves changes back to the spec (Summary, Description, param definitions, body content type, auth schemes, test script)
- `Ctrl+E` — saves the last response as an example in the spec
- Runtime values (body content, header values, param values) are saved to session temp files
