---
name: api-testing
description: >
  Use this skill to validate, test or explore an HTTP API that has an OpenAPI
  spec, using LazyAPI. Covers discovering operations, sending requests,
  writing JavaScript test scripts (x-lazyapi-tests), running them, reading the
  JSON report and using the mock server. Triggered by requests like "test the
  API", "check if the endpoints work", "write API tests", "validate the
  contract", "is the API returning what the spec says", "smoke test the API".
---

# API Testing with LazyAPI

Prefer the MCP tools when the `lazyapi` MCP server is connected (`list_operations`, `describe_operation`, `send_request`, `write_test`, `run_tests`, ...). Otherwise use the CLI with JSON output. Both return the same data.

## Loop

1. **Find the spec**: `list_specs`, or look for YAML files with an `openapi:` key.
2. **See what exists**: `lazyapi list ops <spec> --json`.
3. **Learn an operation**: `lazyapi describe <spec> <operationId> --json`. It gives parameters with sample values, request body schema and example, response schemas, and referenced components.
4. **Call it**: `lazyapi send request <spec> --op <operationId> --param id=1 --body @body.json --output json`.
   - Read `outcome` (`passed` | `failed` | `error`) and every failed entry in `checks` (`request`, `status`, `contract`, `test`, `script`). Each failure has a `message` explaining it.
   - `error` means it could not run (network, missing path parameter, no server). Read `reason`.
5. **Encode the expected behavior** in `tests/<operationId>.js` and link it from the operation with `x-lazyapi-tests: tests/<operationId>.js` (MCP `write_test` does both):
   ```js
   test("returns the user", () => {
     expect(response).toHaveStatus(200);
     expect(response.json()).toHaveProperty("email");
   });
   ```
   Globals: `request`, `response`, `env`, `vars`, `lazyapi.send(op, {params, query, headers, body})`, `test`, `expect`, `console`. Full API: `lazyapi types`.
6. **Scenarios across operations** (login → create → read → delete): write `flows/<name>.js` (MCP `write_flow`), which calls `lazyapi.send()` in order and asserts with `test()`. Every step is checked against the spec. Run it with `lazyapi run <spec> --flow flows/<name>.js` (MCP `run_flow`).
7. **Run everything**: `lazyapi run <spec> --flows --output json`. Exit codes: 0 all passed · 1 a check failed · 2 usage error · 3 could not run.

## Auth

- Prefer `lazyapi.config.yml` environments (`--env-name`, MCP `env_name`). They supply the server, env file and credentials per security scheme (bearer token, basic, API key, OAuth2 client credentials/password, fetched automatically).
- For custom schemes (signatures, tokens from a login call), `export function before(req)` in the test script and set `req.headers`. It runs before the request is sent.
- Never write secrets into the spec or scripts. Use `{{env.NAME}}` from an env file.

## Rules

- `run` only sends GET/HEAD/OPTIONS by default. Pass `--methods all` only when writes are acceptable against that server. Never run writes against production unless the user said so.
- Never invent path parameter values: use `--param`, or add `example` to the parameter in the spec.
- Imports in scripts must be relative (`./helpers.js`) or `https://` (e.g. `https://esm.sh/zod@3.23.8`, always with a version). Bare names (`zod`) are rejected. Commit `lazyapi.lock`.
- Scripts are synchronous: `const r = lazyapi.send("login", {...}); r.json().token`.
- `vars.set()` passes values to later operations of a sequential `run` (`{{var.name}}` in params, headers or bodies).
- To test error paths against the mock, send `Prefer: code=404`.

## No server available

```bash
lazyapi mock <spec> --port 4010 &       # answers with the spec's examples, 422 for invalid requests
lazyapi run <spec> --server http://127.0.0.1:4010 --methods all
```

A failing `request` check means the request did not match the spec. A failing `contract` check means the response did not match the schema. Look at the `message` to tell whether the spec or the implementation is wrong before changing either.

## CI

```bash
lazyapi validate spec <spec>
lazyapi run <spec> --frozen-lockfile --output junit --output-file report.xml
```
