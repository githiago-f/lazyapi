# `lazyapi send request`

Sends one operation and runs its checks: declared status, response contract and the operation's test script.

```bash
lazyapi send request <spec.yml> <path> <method> [flags]
lazyapi send request <spec.yml> --op <operationId> [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--op <id>` | Operation to send, instead of `<path> <method>` (`"METHOD /path"` also works) |
| `--param k=v` | Path parameter (repeatable). Defaults to the spec's example; a missing value is an error |
| `--query k=v`, `--header k=v` | Query parameter / header (repeatable) |
| `--body <value\|@file\|@->` | Request body: literal, read from a file, or from stdin |
| `--server <url\|index>` | Server URL, or index into the spec's `servers` (default: first) |
| `--env <file>` | `.env` file for <code v-pre>{{env.X}}</code> |
| `--timeout <d>` | Request timeout (default `30s`) |
| `--script <file>` | Test script; overrides the operation's `x-lazyapi-tests` |
| `--no-contract` | Skip the response schema check |
| `--output pretty\|json\|junit\|tap` | `json` prints the [operation result](/cli/run#json-report) with full bodies |
| `--output-file <path>`, `--bodies <failed\|all\|none>` | As in [`run`](/cli/run#output); `send` defaults to `--bodies all` |
| `--save-example` | Save the response as an example in the spec |
| `--dry-run` | Resolve the request (URL, headers, body) without sending |
| `--script-timeout`, `--offline`, `--reload`, `--frozen-lockfile`, `--allow-import` | See [Test Scripts](/scripting#imports) |

## Examples

```bash
lazyapi send request api.yml /users GET
lazyapi send request api.yml --op getUser --param id=42 --header "X-Trace=abc"
lazyapi send request api.yml --op createUser --body @new-user.json --output json
echo '{"name":"ana"}' | lazyapi send request api.yml --op createUser --body @-
lazyapi send request api.yml --op getUser --param id=1 --save-example
```

## Exit status

0 when every check passes, 1 when a check fails, 2 for usage errors, and 3 when the request could not be sent (network error, missing path parameter, no server).
