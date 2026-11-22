# `lazyapi run`

Runs the selected operations of a spec. Each exchange is checked four ways:

1. **request**: the request sent must match the operation (required parameters, parameter schemas, body schema). It is still sent when it fails, because the server's answer is useful too.
2. **status**: the status code must be declared in the operation's `responses` (exact code, `2XX`-style range, or `default`).
3. **contract**: response headers and body must match the declared schema.
4. **tests**: the operation's script (`x-lazyapi-tests`, or `--script`). See [Test Scripts](/scripting).

`--no-contract` skips both the request and contract checks.

```bash
lazyapi run <spec.yml> [flags]
```

## Selection

| Flag | Description |
|------|-------------|
| `--methods <list>` | Methods to run. The default is `GET,HEAD,OPTIONS`; `all` includes POST/PUT/PATCH/DELETE. Other methods are reported as skipped |
| `--tag <list>` | Only operations with one of these tags |
| `--op <list>` | Only these operations, by `operationId` or `"METHOD /path"` |
| `--grep <regex>` | Only operations whose `METHOD /path operationId summary` matches |

## Order

Operations run in spec order within four groups: reads (`GET`/`HEAD`/`OPTIONS`), then `POST`, then `PUT`/`PATCH`, then `DELETE`. A run therefore does not delete what later operations read. For exact sequences, use [flows](/scripting#flows).

## Flows

| Flag | Description |
|------|-------------|
| `--flows` | After the operations, run every `flows/*.js` next to the spec |
| `--flow <file>` | Run only this flow (repeatable); no operations are run |
| `--flow-timeout <d>` | Maximum run time of one flow (default `5m`) |

Flow results appear in the report with `"kind": "flow"`, `"method": "FLOW"`, the flow path, its `steps` and its checks.

## Execution

| Flag | Description |
|------|-------------|
| `--env-name <name>` | Named environment from [`lazyapi.config.yml`](/environments): server, env file, vars and credentials |
| `--server <url\|index>` | Server URL, or index into the spec's `servers` (default: first). Wins over the environment |
| `--env <file>` | `.env` file for <code v-pre>{{env.NAME}}</code>. Wins over the environment |
| `--param k=v` | Path parameter for operations that declare it (repeatable). Otherwise the spec's example is used; with no value at all, the operation is skipped |
| `--query k=v`, `--header k=v` | Added to every request (repeatable) |
| `--timeout <d>` | Per-request timeout (default `30s`) |
| `--parallel <N>` | Run N operations at once. Results stay in spec order. Each operation gets its own copy of `env`/`vars` |
| `--retries <N>` | Retry network errors and 5xx responses up to N times (backoff 250ms × attempt) |
| `--bail` | Stop after the first failed or errored operation; the rest are reported as skipped |
| `--dry-run` | Resolve every request without sending |
| `--insecure` | Skip TLS certificate verification |
| `--ca-cert <pem>` | Extra trusted CA certificates |
| `--client-cert <pem>` / `--client-key <pem>` | Mutual TLS |
| `--proxy <url>` | http, https or socks5 proxy (default: `$HTTPS_PROXY`/`$HTTP_PROXY`) |
| `--no-redirects` / `--max-redirects <n>` | Return 3xx responses as-is / limit redirect chains (default 10) |
| `--max-body <size>` | Maximum response body, e.g. `10MB` (default `50MB`) |

With `--parallel 1` (the default), `vars.set()` in one operation's script is visible to the next operations.

## Scripts

`--script <file>`, `--no-contract`, `--script-timeout <d>`, `--offline`, `--reload`, `--frozen-lockfile`, `--allow-import <hosts>`. See [Test Scripts](/scripting#imports).

## Output

| Flag | Description |
|------|-------------|
| `--output pretty` | Human-readable log, printed as operations complete (default) |
| `--output json` | One JSON report on stdout (nothing else is printed) |
| `--output junit` | JUnit XML: one `<testsuite>` per operation, one `<testcase>` per check |
| `--output tap` | TAP version 14, one test point per check |
| `--output-file <path>` | Write the `--output` report to a file; stdout keeps the pretty log |
| `--bodies failed\|all\|none` | Request/response bodies in JSON (default: only for operations that did not pass; truncated at 64 KiB) |

Sensitive request headers (`Authorization`, `Cookie`, anything containing `token`, `secret`, `password` or `api-key`) are redacted in reports.

### JSON report

```json
{
  "spec": "api.yml",
  "startedAt": "2026-11-22T10:00:00Z",
  "summary": { "operations": 3, "passed": 1, "failed": 1, "errors": 0, "skipped": 1,
               "checksPassed": 4, "checksFailed": 1, "durationMs": 812 },
  "operations": [
    {
      "operationId": "getUser", "method": "GET", "path": "/users/{id}",
      "outcome": "failed", "attempts": 1, "testsFile": "tests/getUser.js",
      "request":  { "method": "GET", "url": "https://api.example.com/users/1" },
      "response": { "status": 200, "statusText": "200 OK", "headers": {"Content-Type": "application/json"},
                    "body": "{\"id\":\"1\"}", "durationMs": 41,
                    "timing": { "dnsMs": 1.2, "connectMs": 3.4, "tlsMs": 12.1, "ttfbMs": 20.3, "totalMs": 41.0, "reusedConnection": false },
                    "cookies": [{ "name": "session", "value": "abc", "path": "/", "httpOnly": true }] },
      "checks": [
        { "kind": "request",  "name": "request matches the OpenAPI spec", "passed": true },
        { "kind": "status",   "name": "status 200 is declared", "passed": true },
        { "kind": "contract", "name": "response matches the OpenAPI schema", "passed": false,
          "message": "id: value must be an integer" },
        { "kind": "test",     "name": "has an email", "passed": true }
      ],
      "logs": [{ "level": "log", "message": "user 1" }]
    }
  ]
}
```

`outcome` is one of `passed`, `failed`, `error` (could not run; see `reason`), `skipped` (see `reason`) or `planned` (`--dry-run`). Check `kind` is `request`, `status`, `contract`, `test` or `script` (the script failed to compile or threw outside `test()`).

## Examples

```bash
# Read-only pass over the whole API
lazyapi run api.yml --server https://staging.example.com

# CI: everything, JUnit for the CI UI, pinned imports
lazyapi run api.yml --methods all --param id=42 --frozen-lockfile --output junit --output-file report.xml

# Agent: machine-readable, fast
lazyapi run api.yml --tag users --parallel 8 --output json
```
