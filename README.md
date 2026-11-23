# LazyAPI

> OpenAPI-driven API exploration, testing and automation from the terminal, for humans and AI agents.

LazyAPI uses your OpenAPI spec as the single source of truth, so there are no collections to keep in sync. You can:

- explore the API in a terminal UI;
- send requests and see every response checked against the spec;
- write tests in JavaScript or TypeScript;
- run everything in CI;
- let an AI agent drive it through MCP or the CLI's JSON output.

```text
             ┌── TUI ─────── explore, send, edit, debug        (humans)
OpenAPI ─────┼── CLI ─────── run · send · describe · validate   (CI, scripts)
+ tests/*.js └── MCP ─────── list · describe · send · write_test · run   (agents)
                  │
                  └─ every response is checked: request · status · contract · your tests
```

## Quickstart (2 minutes, no server needed)

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
  [PASS] the request was authenticated by before()
  [PASS] unknown users answer 404 with an error body
========================================
Operations: 7 passed, 0 failed, 0 errors, 0 skipped (20ms)
Checks:     40 passed, 0 failed
```

Then open the TUI on the same spec: `./lazyapi examples/openapi.yml`.

## What every request is checked for

| Check | Passes when |
|-------|-------------|
| `request` | The request sent matches the spec (required params, param types, body schema) |
| `status` | The status code is declared in the operation's `responses` |
| `contract` | Response headers and body match the declared schema |
| `test` | Each `test()` in the operation's script passes |

`--no-contract` turns off the `request` and `contract` checks (useful for deliberately invalid calls).

## Tests are JavaScript next to your spec

```yaml
# openapi.yml
paths:
  /users/{id}:
    get:
      operationId: getUser
      x-lazyapi-tests: tests/getUser.js
```

```js
// tests/getUser.js
import { z } from "https://esm.sh/zod@3.23.8";   // URL imports, pinned in lazyapi.lock
import { login, bearer } from "./helpers.js";     // relative imports

test("returns the user", () => {
  expect(response).toHaveStatus(200);
  expect(response.json()).toHaveProperty("email");
});

test("chained call with a token", () => {
  const me = lazyapi.send("getUser", { params: { id: "1" }, headers: bearer(login()) });
  expect(me).toMatchSchema();
});
```

A script can also `export function before(req)` to change the request before it is sent (auth, signatures, bodies).

Scenarios across operations go in **flows** (`flows/*.js`). A flow calls `lazyapi.send()` in order, and every step is checked against the spec automatically:

```js
// flows/userLifecycle.js
const created = lazyapi.send("createUser", { body: { name: "Ana", email: "ana@example.com" } });
test("created", () => expect(created).toHaveStatus(201));
const removed = lazyapi.send("deleteUser", { params: { id: String(created.json().id) } });
test("deleted", () => expect(removed).toHaveStatus(204));
```

Scripts run in a sandbox with `request`, `response`, `env`, `vars`, `lazyapi.send`, `test`, `expect` (Jest-style matchers) and `console`. Run `lazyapi types` for the TypeScript definitions. Details: [docs/scripting.md](docs/scripting.md).

## CLI

```bash
lazyapi run api.yml [--tag users] [--op getUser] [--methods all] [--parallel 8] [--retries 2] [--bail]
lazyapi run api.yml --output json|junit|tap [--output-file report.xml]
lazyapi run api.yml --flows                  # operations, then every flows/*.js
lazyapi run api.yml --env-name staging       # server, env file, vars, credentials from lazyapi.config.yml
lazyapi send request api.yml --op createUser --body @user.json --param id=1 --output json
lazyapi list ops api.yml --json
lazyapi describe api.yml getUser --json      # params, bodies, responses, referenced schemas
lazyapi validate spec api.yml                # OpenAPI + operationIds + test scripts compile
lazyapi mock api.yml --port 4010             # fake API; Prefer: code=404 picks a response
lazyapi import postman shop.json --into api.yml   # also: curl '<command>', har capture.har
lazyapi send request api.yml --op getUser --output curl
lazyapi send request api.yml --op upload --file avatar=./me.png --form caption=hi
lazyapi types                                # script API (TypeScript definitions)
```

TLS (`--insecure`, `--ca-cert`, mTLS), proxies, redirects and body limits can be set per run or per environment. Results include timing (DNS, connect, TLS, TTFB) and cookies.

Coming from Postman? `lazyapi import postman` converts a collection into operations and translates its scripts to the native `test`/`expect` API. Folders become tags, and tokens go to an env file, not into the spec.

Named environments and credentials (bearer, basic, API key, OAuth2 client credentials/password with automatic tokens) live in `lazyapi.config.yml`. See [docs/environments.md](docs/environments.md).

By default, `run` only sends `GET`, `HEAD` and `OPTIONS` (reads first, deletes last); pass `--methods all` to include writes. Path parameters come from `--param` or the spec's examples, and the CLI never invents values.

Exit status is the same for every command: **0** all passed · **1** a check failed · **2** usage error · **3** something could not run (spec, network, config).

Full reference: [docs/cli/](docs/cli/index.md).

## AI agents (MCP)

```bash
claude mcp add lazyapi -- lazyapi mcp --root /path/to/project
```

The MCP server exposes these tools: `list_specs`, `list_operations`, `describe_operation`, `send_request` (with an optional `jq` filter), `run_tests`, `run_flow`, `write_flow`, `read_test`, `write_test`, `validate_spec`, `script_api`, `import_curl` and `export_curl`. Paths are confined to `--root`, and failures come back as readable tool results. See [docs/agents.md](docs/agents.md).

## TUI

```bash
lazyapi                       # discovers every OpenAPI file under the current directory
lazyapi api.yml --env .env    # one spec, one env file
```

- Request list grouped by tag, with a filter (`/`) and a response preview (`s` sends; writes ask for confirmation).
- Editor with params, auth, headers, body and tests. `alt+enter` sends; `F5` runs the tests; `ctrl+x` edits in `$EDITOR`.
- Response view: status, time and size; Body (highlighted, `r` for raw, `f` for a jq filter), Headers, Cookies, Timeline, Checks and Console tabs; `/` searches.
- `ctrl+k` command palette: open any operation, run the spec, a tag or flows, switch environment.
- Runner view (`R`): results stream in with a progress bar, failures first, details for each one.
- History (`H`): replay past requests and diff a response with the previous one or with the spec example. Credentials are never written to disk.
- `{{env.X}}` completion after `{{`, and a warning for references without a value.
- `ctrl+v` switches environment; `esc` cancels a request in flight; `?` shows every key.

See [docs/tui/](docs/tui/index.md).

## Install

```bash
go install github.com/githiago-f/lazyapi/cmd/lazyapi@latest
```

Or download a release binary, or build from source with `go build -o lazyapi ./cmd/lazyapi`.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, conventions and the pull request checklist. [AGENTS.md](AGENTS.md) describes the architecture for coding agents. The roadmap is in [TODO.md](TODO.md).

```bash
go test ./...        # includes the examples against the mock
golangci-lint run
```

## License

GPLv3.
