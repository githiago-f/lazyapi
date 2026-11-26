# Test script API (cheat sheet)

Scripts are JavaScript or TypeScript, synchronous, one file per operation. Full definitions: MCP `script_api` or `lazyapi types`.

## Globals

| Name | What |
|---|---|
| `response` | `status`, `statusText`, `headers`, `body` (text), `json()`, `text()`, `header(name)`, `time` (ms) |
| `request` | the request that was sent: `method`, `url`, `headers`, `body` |
| `env` / `vars` | `get(name)`, `set(name, value)`, `has(name)`, `all()`; `vars` persist to later operations of a run and are `{{var.name}}` in params, headers and bodies |
| `lazyapi.send(target, {params, query, headers, body})` | another operation (operationId, `"METHOD /path"`, or `"METHOD https://other.host/path"`); returns a response; checked against the spec |
| `lazyapi.iteration` | the data row number in data-driven runs |
| `faker.*` | fake data: `faker.email()`, `faker.uuid()`, `faker.int(1, 9)`, `faker.firstName()`, … (also `{{$email}}`, `{{$uuid}}` … in requests; `lazyapi fake` lists them) |
| `test(name, fn)`, `expect(value)`, `console.log` | assertions and logs (logs appear in the report) |

## Matchers

`toBe`, `toEqual`, `toBeTruthy`, `toBeFalsy`, `toBeDefined`, `toBeUndefined`, `toBeNull`, `toBeGreaterThan`, `toBeGreaterThanOrEqual`, `toBeLessThan`, `toBeLessThanOrEqual`, `toContain`, `toMatch(regex|string)`, `toHaveLength`, `toHaveProperty(path, value?)`, `toHaveStatus(code)`, `toHaveHeader(name, value?)`, `toMatchSchema(schema?)` (no argument: the spec's response schema). Every matcher has `.not`.

## Hooks

```js
// Change the request before it is sent (custom auth, signatures, computed ids).
export function before(req) {
  req.headers["X-Signature"] = sign(req.body);
}

// Runs after the tests, even when they failed: clean up what the test created.
export function after(res) {
  if (res.status === 201) lazyapi.send("deleteUser", { params: { id: res.json().id } });
}
```

## Imports

Relative files (`import { login } from "./helpers.js"`, inside the spec's directory) or `https://` URLs with a pinned version (`https://esm.sh/zod@3.23.8`). Bare package names are rejected. URL imports are locked in `.lazyapi/lazyapi.lock` (commit it; CI uses `--frozen-lockfile`).

## Typical tests

```js
test("lists users", () => {
  expect(response).toHaveStatus(200);
  const users = response.json();
  expect(users.length).toBeGreaterThan(0);
  vars.set("userId", users[0].id);           // later operations: {{var.userId}}
});

test("rejects a missing email", () => {
  const r = lazyapi.send("createUser", { body: { name: "Ana" } });
  expect(r).toHaveStatus(422);
});
```
