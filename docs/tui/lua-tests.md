# Lua Test Scripts

LazyAPI supports Lua scripts for writing test assertions against HTTP requests and responses. Scripts are embedded via [gopher-lua](https://github.com/yuin/gopher-lua).

## Using Test Scripts

- **TUI** — In the Tests tab of the Request Editor, write Lua directly and press `F5` to run.
- **CLI `send` / `smoke`** — Each operation's saved script runs automatically. Pass `--script <file.lua>` to use a different script instead.

## Where scripts are stored

When you save a request in the TUI, its script is written to a file next to the spec and referenced from the operation:

```yaml
paths:
  /users:
    get:
      operationId: listUsers
      x-lazyapi-tests: tests/listUsers.lua
```

The file name is the `operationId`, or `<method>_<path>` when there is none. You can also create these files by hand, or point `x-lazyapi-tests` at any path **inside** the spec's directory (absolute paths and `..` are rejected). Commit them with the spec so CI and other machines run the same tests. Saving an empty script (or the untouched default template) removes the reference but keeps the file.

## Available Globals

### `request` table

Details of the outgoing HTTP request.

| Method | Returns |
|--------|---------|
| `request.method()` | HTTP method as a string (`"GET"`, `"POST"`, etc.) |
| `request.uri()` | The URI path |
| `request.url()` | Full URL (server + path) |
| `request.body()` | Request body string |
| `request.header(name)` | Header value by name (case-insensitive), or `nil` |
| `request.headers()` | Table of all headers |
| `request.query(name)` | Query parameter value, or `nil` |
| `request.param(name)` | Path parameter value, or `nil` |

### `response` table

Available after a request has been sent. If no request has been sent, the `response` table is present but returns empty/zero values.

| Method | Returns |
|--------|---------|
| `response.status()` | HTTP status code (e.g. `200`), or `0` if no response |
| `response.statusText()` | Status text (e.g. `"OK"`), or `""` if no response |
| `response.body()` | Response body string |
| `response.header(name)` | Header value by name, or `nil` |
| `response.headers()` | Table of all headers |
| `response.json()` | Body parsed as JSON (table), raises error if invalid JSON |

### `env` table

Environment variables loaded from `.env` files.

| Method | Returns |
|--------|---------|
| `env.get(name)` | Environment variable value, or `nil` |
| `env.has(name)` | Boolean |
| `env.set(name, value)` | Stores a variable |
| `env.vars()` | Table of all variables |

### `tests` table

Register boolean results. Any value can be assigned; it is truthy if:
- It's `true` (boolean)
- It's a non-zero number
- It's a non-empty string (excluding `"false"`, `"0"`, `"nil"`)

Example:

```lua
tests["Status is 200"] = response.status() == 200
tests["Content-Type is JSON"] = response.header("Content-Type"):find("json") ~= nil
```

Results are reported in the order they are assigned. Assigning the same name twice records two results; it does not overwrite the first one.

## Helper Functions

| Function | Description |
|----------|-------------|
| `json_decode(str)` | Parse a JSON string into a Lua table |
| `json_encode(table)` | Serialize a Lua table as a JSON string |
| `test(name, fn)` | Run `fn()` in protected mode; records pass/fail under `name`. On failure the error message (e.g. from `assert(cond, "msg")`) is shown next to the result |

## Example Script

```lua
-- Simple boolean assertions
tests["Status is 200"] = response.status() == 200

tests["Content-Type is JSON"] = response.header("Content-Type"):find("json") ~= nil

tests["Response has no errors"] = response.json().error == nil

-- Protected block with assertions
test("Body contains valid user data", function()
    local body = response.json()
    assert(body.id, "missing id field")
    assert(body.email, "missing email field")
    assert(body.email:find("@"), "email is invalid")
end)

-- Using env variables
tests["API key was configured"] = env.has("API_KEY")
```

## Script Errors

If the script has a syntax error or raises an error, the error message is captured and displayed in the results panel. The script does not block or crash the application.
