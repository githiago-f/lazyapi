---
name: lazyapi-explore
description: Explore and understand an HTTP API fast with lazyapi — find its OpenAPI spec, list operations, read one operation's parameters and schemas, and call it to see real responses, without reading the API's documentation. Also turns curl commands, Postman collections or HAR captures into a spec when there is none. Use when asked what an API can do, how to call an endpoint, what a response looks like, or to try or debug a request.
---

# Explore an API with lazyapi

Use the lazyapi MCP tools when they are listed; otherwise the CLI in the right column (JSON output). If neither exists, use the lazyapi-setup skill.

| Step | MCP tool | CLI |
|---|---|---|
| Find specs | `list_specs` | `ls`, files with an `openapi:` key |
| See operations | `list_operations {spec, tags?}` | `lazyapi list ops <spec> --json` |
| Learn one | `describe_operation {spec, operation}` | `lazyapi describe <spec> <op> --json` |
| Call it | `send_request {spec, operation, params, query, headers, body, jq}` | `lazyapi send request <spec> --op <id> --param k=v --output json` |
| See the exact request | `export_curl {…same as send}` | `send … --dry-run` |

`operation` is an operationId or `"METHOD /path"` as written in the spec (`"GET /users/{id}"`).

## Read less

- `describe_operation` returns only what one call needs: parameters with example values, request body schema and example, response schemas per status, the component schemas they reference, and security. Do not open the spec file to learn an operation.
- Large responses: pass `jq` to `send_request` (`".items[0] | {id, status}"`, `".items | length"`). Checks still run on the whole body.
- `list_operations` with `tags` narrows big APIs.

## Reading a result

`outcome` is `passed`, `failed` or `error`; `checks` list `status` (declared in the spec?), `contract` (body matches the schema?), `request` (did *we* send what the spec expects?) and test results, each failure with a `message`. `error` means it could not run: read `reason` (missing path parameter, no server, blocked by permissions).

## Rules

- Never invent path parameter values or IDs. Take them from a list call (`GET /users` → an id), from the user, or from the parameter's `example`.
- Reads first. Send POST/PUT/PATCH/DELETE only when the user asked for it or agreed, and never against production without explicit consent.
- "lazyapi blocked …" means the user has not allowed that server. Tell the user what you wanted to call and why; never run `lazyapi allow` yourself.
- Credentials come from the environment (`env_name`, or `{{env.NAME}}` from an env file). Never paste secrets into specs, scripts or chat.

## Which server

The spec's first server is the default. `server` takes a URL or an index into the spec's servers; `env_name` picks a named environment from `.lazyapi/config.yml` (server, variables, credentials). No server at all: run `lazyapi mock <spec> --port 4010` and use `server: "http://127.0.0.1:4010"` — it answers with the spec's examples and rejects invalid requests with 422.

## No spec yet

Build one from what the user has, then explore it as above. Details: [references/import.md](references/import.md).

- One request: `import_curl {spec: "api.yml", command: "curl …"}` (creates the spec if missing).
- A Postman collection or a browser HAR capture: `lazyapi import postman|har <file> --into api.yml`.

Imports infer schemas from examples and never copy credentials into the spec.
