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
| `--form k=v` | Form field (repeatable): `application/x-www-form-urlencoded`, or multipart with `--file` (the operation's declared media type wins) |
| `--file name=path` | Multipart file part (repeatable) |
| `--env-name <name>` | Named environment from [`.lazyapi/config.yml`](/environments) |
| `--server <url\|index>` | Server URL, or index into the spec's `servers` (default: first) |
| `--env <file>` | `.env` file for `{{env.X}}` |
| `--timeout <d>` | Request timeout (default `30s`) |
| `--script <file>` | Test script; overrides the operation's `x-lazyapi-tests` |
| `--no-contract` | Skip the response schema check |
| `--output pretty\|json\|junit\|tap` | `json` prints the [operation result](/cli/run#json-report) with full bodies |
| `--output-file <path>`, `--bodies <failed\|all\|none>` | As in [`run`](/cli/run#output); `send` defaults to `--bodies all` |
| `--save-example` | Save the response as an example in the spec |
| `--dry-run` | Resolve the request (URL, headers, body) without sending |
| `--output curl` | Print the exact request (after `before()`) as a curl command instead of sending it |
| `--jq filter` | Show only this [jq](https://jqlang.org/manual/) slice of the response body (in every output format). Checks still run on the full body |
| `--insecure` | Skip TLS certificate verification |
| `--ca-cert <pem>` | Extra trusted CA certificates |
| `--client-cert <pem>` / `--client-key <pem>` | Mutual TLS |
| `--proxy <url>` | http, https or socks5 proxy (default: `$HTTPS_PROXY`/`$HTTP_PROXY`) |
| `--no-redirects` / `--max-redirects <n>` | Return 3xx responses as-is / limit redirect chains (default 10) |
| `--max-body <size>` | Maximum response body, e.g. `10MB` (default `50MB`) |
| `--script-timeout`, `--offline`, `--reload`, `--frozen-lockfile`, `--allow-import` | See [Test Scripts](/scripting#imports) |

## Examples

```bash
lazyapi send request api.yml /users GET
lazyapi send request api.yml --op getUser --param id=42 --header "X-Trace=abc"
lazyapi send request api.yml --op createUser --body @new-user.json --output json
echo '{"name":"ana"}' | lazyapi send request api.yml --op createUser --body @-
lazyapi send request api.yml --op getUser --param id=1 --save-example
lazyapi send request api.yml --op login --form username=ana --form password=pw
lazyapi send request api.yml --op uploadAvatar --param id=1 --file avatar=./me.png --form caption=hi
lazyapi send request api.yml --op getUser --param id=1 --output curl   # share or debug the exact request
lazyapi send request api.yml --op listUsers --jq '.[] | {id, email}' --output json   # only what you need
```

## Exit status

0 when every check passes, 1 when a check fails, 2 for usage errors, and 3 when the request could not be sent (network error, missing path parameter, no server).
