# Using LazyAPI from AI agents

LazyAPI gives agents the same loop a developer uses in Postman: read the API, send requests, write tests, and run them. Results come back as structured data.

## Install for your agent

The fastest setup is a package: skills that teach the agent the workflow, plus the MCP server. Install the binary first (`go install github.com/githiago-f/lazyapi/cmd/lazyapi@latest`, or a [release](https://github.com/githiago-f/lazyapi/releases)).

| Agent | Install |
|-------|---------|
| Claude Code | `/plugin marketplace add githiago-f/lazyapi` then `/plugin install lazyapi@lazyapi` (skills + MCP server) |
| Gemini CLI | `gemini extensions install https://github.com/githiago-f/lazyapi` (skills + MCP server) |
| Codex | `npx skills add githiago-f/lazyapi -a codex -y` and `codex mcp add lazyapi -- lazyapi mcp --root .` |
| DeepSeek Harness, OpenCode, Cursor, … | `npx skills add githiago-f/lazyapi` (or copy `skills/lazyapi-*` into `~/.agents/skills`) and add the MCP server below |

### Skills

Four [Agent Skills](https://agentskills.io) live in [`skills/`](https://github.com/githiago-f/lazyapi/tree/main/skills). Each is a short `SKILL.md`; details sit in `references/` and are read only when needed, so the agent learns the API from the spec instead of reading documentation.

| Skill | For |
|-------|-----|
| `lazyapi-setup` | install the binary and connect the MCP server |
| `lazyapi-explore` | find operations, read one operation's contract, call it; build a spec from curl / Postman / HAR |
| `lazyapi-test` | contract checks, test scripts, flows, data-driven runs, mock, CI |
| `lazyapi-integrate` | write client code from verified requests and real schemas |

## Permissions: the agent only calls servers you allow

The MCP server never sends a request to a server you have not allowed. The first request to a new server is held while **you** are asked, in the agent's own UI (MCP elicitation):

| Server | Question | Answers |
|--------|----------|---------|
| local (`localhost`, `127.0.0.1`, `::1`, `*.localhost`, any port) | once | allow (remembered) · deny |
| any other origin (`scheme://host:port`) | per kind of access: reads (GET, HEAD, OPTIONS), then writes (POST, PUT, PATCH, DELETE…) | only this request · this session · always · deny |

- "Only this request" lasts for the tool call, so `run_tests` over 50 operations asks once; a refusal also holds for the rest of the call.
- Everything the server sends is covered: operations, `lazyapi.send` in test scripts and flows, OAuth token calls, every redirect hop, and `https://` imports of test scripts (a script cannot leak data through an import URL).
- Clients that cannot show the question get a `lazyapi blocked …` result telling the agent to ask you. The skills and the server's instructions tell agents never to grant themselves access.
- "Always" answers are stored in your config directory (`~/.config/lazyapi/permissions.json` on Linux, `$LAZYAPI_PERMISSIONS` overrides), never in the project: a repository cannot pre-approve servers for whoever opens it. Manage them with [`lazyapi allow`](/cli/allow).

Per server setup instead of answers: `--allow <origin>` (reads) and `--allow-writes <origin>` on the `lazyapi mcp` command; `--allow localhost` for local servers. `--permissions off` disables the check (trusted automation only).

This guards the network. File-writing tools (`write_test`, `write_flow`, `import_curl`) only write inside `--root`; approving MCP tool calls themselves is your agent's own permission setting. The CLI (`lazyapi run`, `send`) is not gated: it is what you run yourself, and an agent running shell commands is governed by its harness.

## MCP server

```bash
lazyapi mcp [--root dir] [--allow origin]... [--allow-writes origin]... [--permissions ask|off]
```

This starts a [Model Context Protocol](https://modelcontextprotocol.io) server on stdio. Paths in tool arguments are relative to `--root` (default: the current directory). Paths that leave the root are rejected.

Register it with Claude Code:

```bash
claude mcp add lazyapi -- lazyapi mcp --root /path/to/project
```

Or with any client that reads an `mcpServers` config:

```json
{ "mcpServers": { "lazyapi": { "command": "lazyapi", "args": ["mcp", "--root", "/path/to/project"] } } }
```

### Tools

| Tool | What it does |
|------|--------------|
| `list_specs` | OpenAPI files under the root |
| `list_operations` | Operations of a spec (optionally by tag) |
| `describe_operation` | Parameters with examples, request/response schemas, referenced components, security, current test script |
| `send_request` | Send one operation (params, query, headers, body) and get request, response and every check |
| `run_tests` | Run many operations. Only GET/HEAD/OPTIONS run unless `methods` is set (`["all"]` includes writes). Bodies are included only for operations that did not pass |
| `run_flow` / `write_flow` | Write and run `.lazyapi/flows/<name>.js` scenarios across operations; every step is checked against the spec |
| `import_curl` / `export_curl` | Add an operation from a curl command, or get the exact request as curl |
| `read_test` / `write_test` | Read or write an operation's JavaScript test script; `write_test` links it via `x-lazyapi-tests` and reports compile errors |
| `validate_spec` | OpenAPI and LazyAPI checks (operationIds, responses, scripts compile) |
| `script_api` | The TypeScript definitions of the script API |

`send_request` accepts `jq` (e.g. `".items[0] | {id, status}"`) to return only part of a large body; checks still see the whole response.

`send_request`, `run_tests`, `run_flow` and `export_curl` accept `env_name` to use a named environment from `.lazyapi/config.yml` (server, env file, variables and credentials).

Tool failures (network errors, unknown operations) come back as results with `isError: true` and a readable message, so the agent can fix the call and retry.

## A server to test against

Data-driven checks: `run --data rows.csv` (MCP `run_tests` with `data_file`) runs the selection once per row with the row in the env; results carry `iteration`.

When a test creates data against a real API, export `after(res)` from its script to delete it: it runs even when the tests fail, so runs stay repeatable. See [scripting](/scripting#cleaning-up-after-res).

`lazyapi mock api.yml` serves the spec's examples and validates incoming requests (422 with the reason). Agents can build and check tests before the real API exists, or reach error paths with `Prefer: code=404`. See [mock](/cli/mock).

## CLI equivalents

Agents without MCP can use the CLI with JSON output:

```bash
lazyapi list ops api.yml --json
lazyapi describe api.yml getUser --json
lazyapi send request api.yml --op getUser --param id=1 --output json
lazyapi types > .lazyapi/tests/lazyapi.d.ts
lazyapi run api.yml --output json            # exit 0 ok · 1 failed · 2 usage · 3 could not run
```

## Validate an API in five steps

1. `list ops` / `list_operations`: see what exists.
2. `describe` / `describe_operation`: learn the parameters and the expected schemas.
3. `send` / `send_request`: call it. The `contract` check already tells you whether the response matches the spec.
4. `write_test`: encode the behavior you expect:
   ```js
   test("returns the user", () => {
     expect(response).toHaveStatus(200);
     expect(response.json()).toHaveProperty("email");
   });
   ```
5. `run` / `run_tests`: run everything. Commit the spec, `tests/` and `.lazyapi/lazyapi.lock` so CI runs the same checks with `--frozen-lockfile`.
