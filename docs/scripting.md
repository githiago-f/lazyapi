# Test Scripts (JavaScript / TypeScript)

Every operation can have a test script. Scripts are JavaScript or TypeScript ES modules that run after the response arrives. They can import local helpers and `https://` modules, call other operations of the same spec, and validate responses against the OpenAPI schema.

```ts
import { z } from "https://esm.sh/zod@3.23.8";
import { bearer } from "./helpers.js";

const User = z.object({ id: z.number(), email: z.string().email() });

test("status is 200", () => expect(response).toHaveStatus(200));
test("matches the OpenAPI schema", () => expect(response).toMatchSchema());
test("zod shape", () => expect(User.safeParse(response.json()).success).toBe(true));

// Chain another operation (operationId or "METHOD /path")
const me = lazyapi.send("getMe", { headers: bearer(env.get("TOKEN")) });
test("me works", () => expect(me.json().id).toBe(response.json().id));
vars.set("userId", response.json().id); // available as {{var.userId}} later in the run
```

## Where scripts live

Scripts are files next to the spec, referenced per operation by `x-lazyapi-tests`:

```yaml
paths:
  /users:
    get:
      operationId: listUsers
      x-lazyapi-tests: tests/listUsers.js
```

- The TUI writes `tests/<operationId>.js` (or `tests/<method>_<path>.js`) when you save a request.
- You and agents can also create or edit these files directly. Commit them with the spec.
- Supported extensions: `.js`, `.mjs`, `.ts`, `.mts`. The path must stay inside the spec's directory.
- Lua scripts (`.lua`) are no longer supported; LazyAPI reports an error that names the file. Saving the request in the TUI moves the reference to a new `.js` file.

Run them with:

```bash
lazyapi send request api.yml /users GET      # one operation
lazyapi run api.yml                          # every GET/HEAD/OPTIONS operation
lazyapi run api.yml --script all.js          # one script for every operation
```

In the TUI, open the **Tests** tab and press `F5`.

## Changing the request: `before(req)`

A test script can export `before`, which runs **before** the request is sent. Use it to authenticate, sign or build the request:

```js
import { login } from "./helpers.js";

export function before(req) {
  req.headers.Authorization = "Bearer " + login();   // login() uses lazyapi.send
  req.query.trace = "1";
  req.body = { ...JSON.parse(req.body || "{}"), requestedAt: Date.now() };
}

test("authorized", () => expect(response).toHaveStatus(200));
```

- `req` has `method`, `url` and `path` (read-only), plus `headers`, `query`, `params` and `body`, which you can change. Change it in place or return a new object. A non-string `body` is JSON-encoded.
- `before` can call `lazyapi.send()` and `vars.set()`, and its `console` output is kept.
- The top level of the file (everything outside `before`) still runs **after** the response. To reach the export, the file is evaluated once before the request. In that pass `test()`, `console`, `vars.set()` and top-level `lazyapi.send()` calls are ignored, and errors are swallowed, because the top level normally reads `response`.
- If `before` throws, the request is **not sent** and the failure is reported as a `script` check.
- The request check and the request that is sent both use the request as `before` left it.

## Flows

A flow is a scenario across operations: `flows/<name>.js` next to the spec. There is no implicit request or response: the flow calls `lazyapi.send()` in order and asserts with `test()`.

```js
// flows/userLifecycle.js
import { login, bearer } from "../tests/helpers.js";

const auth = bearer(login());
const created = lazyapi.send("createUser", { headers: auth, body: { name: "Ana", email: "ana@example.com" } });
test("created", () => expect(created).toHaveStatus(201));
vars.set("userId", created.json().id);

const removed = lazyapi.send("deleteUser", { params: { id: vars.get("userId") }, headers: auth });
test("deleted", () => expect(removed).toHaveStatus(204));
```

**Every `lazyapi.send()` in a flow is checked automatically**: the request matches the spec, the status is declared, and the response matches the schema. These checks are named after the step, e.g. `#2 POST /users · status 201 is declared`. A flow that makes no requests and no assertions fails.

```bash
lazyapi run api.yml --flow flows/userLifecycle.js   # just this flow
lazyapi run api.yml --methods all --flows           # operations, then every flow
```

Flows run sequentially after the operations and share `env` and `vars` with them. `--flow-timeout` (default `5m`) bounds each flow. The report lists every step (method, path, status, time) and the checks.

## Editor support

```bash
lazyapi types > tests/lazyapi.d.ts
```

Add `/// <reference path="./lazyapi.d.ts" />` to the top of a script for autocompletion and type checking. The same file is the complete API reference.

## Globals

| Global | Description |
|--------|-------------|
| `request` | The request sent: `method`, `url`, `path`, `headers` (lower-cased), `query`, `params`, `body`, `json()` |
| `response` | `status`, `statusText`, `headers` (lower-cased), `body`, `time` (ms), `header(name)`, `json()`, `text()`. `null` in the TUI before a request is sent |
| `env` | `get`, `set`, `has`, `all` over environment variables (<code v-pre>{{env.X}}</code>). Changes last for the current run |
| `vars` | Same API for run variables (<code v-pre>{{var.X}}</code>). During `run` (without `--parallel`), later operations see them |
| `lazyapi.send(target, opts?)` | Sends another operation synchronously and returns its response. `opts`: `params`, `query`, `headers`, `body` (objects are JSON-encoded) |
| `test(name, fn)` | Registers a test. It fails if `fn` throws or returns a rejected promise |
| `expect(value)` | Assertions (see below) |
| `console.log/info/warn/error/debug` | Captured and printed with the results |

## Matchers

`toBe`, `toEqual` (deep), `toBeTruthy`, `toBeFalsy`, `toBeDefined`, `toBeUndefined`, `toBeNull`, `toBeGreaterThan`, `toBeGreaterThanOrEqual`, `toBeLessThan`, `toBeLessThanOrEqual`, `toContain`, `toMatch`, `toHaveLength`, `toHaveProperty(path, value?)`, `toHaveStatus(code)`, `toHaveHeader(name, value?)` and `toMatchSchema(schema?)`. Prefix a matcher with `.not` to invert it.

`expect(response).toMatchSchema()` with no argument validates the response against the operation's OpenAPI definition. With an argument it validates any value against that JSON/OpenAPI schema.

Failure messages are explicit, e.g. `expected 404 to be 200` or `schema validation failed: id: value must be an integer`.

## Automatic contract check

`send`, `run`, flows and the TUI also check, even without a script, that every request matches the operation (`request` check) and that every response with a declared schema matches it (`contract` check). Use `--no-contract` to turn both off.

## Imports

| Import | Behavior |
|--------|----------|
| `./x.js`, `../shared/x.ts` | Resolved from the script's directory; must stay inside the spec directory. The extension may be omitted |
| `https://…` | Downloaded once, cached in the user cache dir (`lazyapi/modules`), and pinned in `lazyapi.lock` next to the spec |
| `zod`, `fs`, … (bare names) | Rejected. Use a CDN URL such as `https://esm.sh/zod@3.23.8`. Node built-ins are not available |

`lazyapi.lock` records a `sha256` integrity for every URL. Commit it. If a module's content changes upstream, the run fails with `integrity mismatch`. To accept the change, pin a version in the URL or rerun with `--reload`.

| Flag | Use |
|------|-----|
| `--frozen-lockfile` | CI: fail if a URL import is not already pinned |
| `--offline` | Never hit the network; modules must be cached |
| `--reload` | Re-download all modules and rewrite their lock entries |
| `--allow-import esm.sh,cdn.jsdelivr.net` | Restrict which hosts imports may come from |
| `--script-timeout 30s` | Abort scripts that run too long |

Only `https://` imports are accepted. Imported code runs with the same sandbox as your script: no filesystem, no process and no network, except through `lazyapi.send`.

## Errors

Syntax and runtime errors are reported with file, line and column, e.g. `ReferenceError: user is not defined (listUsers.js:4:3)`. They are reported as a failed `script` check, so `send` and `run` exit with status 1.
