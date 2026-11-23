---
layout: home
title: LazyAPI
titleTemplate: OpenAPI-driven API testing from the terminal
---

## One spec, three interfaces

The OpenAPI document is the single source of truth. There are no collections to export, no environments to rebuild: point LazyAPI at your spec and every operation is ready to explore, send and test.

<Interfaces />

## See it in action

<Gallery />

## Every response is checked

Each exchange — from the TUI, the CLI, a flow or an agent — goes through the same checks. `--no-contract` turns off `request` and `contract` for deliberately invalid calls.

<Checks />

## Two minutes, no server needed

```bash
git clone https://github.com/githiago-f/lazyapi.git && cd lazyapi
go build -o lazyapi ./cmd/lazyapi

./lazyapi mock examples/openapi.yml &          # fake API from the spec's examples
./lazyapi run examples/openapi.yml --methods all --flows
```

```text
Passed GET /users/{id} — 200 OK (1ms)
  [PASS] request: request matches the OpenAPI spec
  [PASS] status: status 200 is declared
  [PASS] contract: response matches the OpenAPI schema
  [PASS] returns the requested user
========================================
Operations: 7 passed, 0 failed, 0 errors, 0 skipped (20ms)
Checks:     40 passed, 0 failed
```

Then open the TUI on the same spec: `./lazyapi examples/openapi.yml`. More in [Getting Started](/getting-started).

## Tests are JavaScript next to your spec

::: code-group

```js [.lazyapi/tests/getUser.js]
import { z } from "https://esm.sh/zod@3.23.8"   // URL imports, pinned in .lazyapi/lazyapi.lock
import { login, bearer } from "./helpers.js"    // relative imports

test("returns the user", () => {
  expect(response).toHaveStatus(200)
  expect(response.json()).toHaveProperty("email")
})

test("chained call with a token", () => {
  const me = lazyapi.send("getUser", { params: { id: "1" }, headers: bearer(login()) })
  expect(me).toMatchSchema()
})
```

```yaml [openapi.yml]
paths:
  /users/{id}:
    get:
      operationId: getUser
      x-lazyapi-tests: .lazyapi/tests/getUser.js
```

```js [.lazyapi/flows/userLifecycle.js]
const created = lazyapi.send("createUser", { body: { name: "Ana", email: "ana@example.com" } })
test("created", () => expect(created).toHaveStatus(201))

const removed = lazyapi.send("deleteUser", { params: { id: String(created.json().id) } })
test("deleted", () => expect(removed).toHaveStatus(204))
```

:::

Scripts run in a sandbox with `request`, `response`, `env`, `vars`, `lazyapi.send`, `faker`, `test`, `expect` and `console`. See [Test Scripts](/scripting).

## Git-native

Everything lives in `.lazyapi/` next to the spec and is versioned with it. Secrets and local state stay out of git.

```text
api/
  openapi.yml
  .lazyapi/
    config.yml          named environments: server, vars, HTTP options, credentials
    envs/staging.env    variables            (committed)
    secrets/staging.env tokens, passwords    (git-ignored, mode 600)
    tests/  flows/  data/
    lazyapi.lock        pinned URL imports
    .local/             drafts and history   (git-ignored)
```

Read [Environments & Auth](/environments) for the details.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | everything passed |
| `1` | a check failed |
| `2` | usage error |
| `3` | something could not run (spec, network, config) |
