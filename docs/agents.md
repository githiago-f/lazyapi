# Using LazyAPI from AI agents

LazyAPI gives agents the same loop a developer uses in Postman: read the API, send requests, write tests, and run them. Results come back as structured data.

## MCP server

```bash
lazyapi mcp [--root dir]
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
| `read_test` / `write_test` | Read or write an operation's JavaScript test script; `write_test` links it via `x-lazyapi-tests` and reports compile errors |
| `validate_spec` | OpenAPI and LazyAPI checks (operationIds, responses, scripts compile) |
| `script_api` | The TypeScript definitions of the script API |

Tool failures (network errors, unknown operations) come back as results with `isError: true` and a readable message, so the agent can fix the call and retry.

## CLI equivalents

Agents without MCP can use the CLI with JSON output:

```bash
lazyapi list ops api.yml --json
lazyapi describe api.yml getUser --json
lazyapi send request api.yml --op getUser --param id=1 --output json
lazyapi types > tests/lazyapi.d.ts
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
5. `run` / `run_tests`: run everything. Commit the spec, `tests/` and `lazyapi.lock` so CI runs the same checks with `--frozen-lockfile`.
