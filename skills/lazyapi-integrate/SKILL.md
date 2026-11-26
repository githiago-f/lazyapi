---
name: lazyapi-integrate
description: Integrate an application with an HTTP API using lazyapi as ground truth — read exact request/response schemas, prove each call works before writing client code, generate types and requests from real exchanges, and keep the integration honest with contract tests and a mock server. Use when writing an API client, SDK wrapper, webhook consumer or service-to-service call, adding an endpoint call to an app, or when an integration returns unexpected data.
---

# Integrate with an API using lazyapi

Use the lazyapi MCP tools when listed; otherwise the CLI with JSON output. If neither exists, use the lazyapi-setup skill. The point: **never write client code from guesses** — every request you code was sent and checked first.

## Steps

1. **Find the operations the feature needs** — `list_operations {spec, tags}`; pick by summary/path.
2. **Get the contract** — `describe_operation {spec, operation}`: parameters (required, location, example), body schema, every response status with its schema, referenced components, security. Generate types/structs from these schemas, not from one sample response (optional fields, enums and error shapes matter).
3. **Prove the call** — `send_request` with the values your code will send (`params`, `query`, `headers`, `body`; `env_name` for credentials). It must pass `request` (you sent what the spec expects), `status` and `contract`. Use `jq` to look at the part you will map.
4. **Copy the exact request** — `export_curl {…the same arguments}` gives the resolved URL, headers and body. Translate that into the target language/HTTP client; keep base URL and credentials configurable (env vars), never hard-coded.
5. **Handle the declared errors** — the statuses in `describe_operation` other than 2xx are the error cases to code for. Reach them against the mock with `Prefer: code=<status>` (below).
6. **Lock it in** — `write_test` for each operation you depend on (fields and statuses your code relies on), or a flow (`write_flow`) for the whole sequence; `run_tests` in CI tells you when the provider breaks the contract. See the lazyapi-test skill.

## Develop without the real API

```bash
lazyapi mock <spec> --port 4010
```
Point the app (and `server`) at `http://127.0.0.1:4010`. It answers with the spec's examples, validates incoming requests (422 + reason when the app sends something wrong — a free check of your client), and `Prefer: code=404` / `Prefer: example=<name>` select responses.

## Auth

Named environments in `.lazyapi/config.yml` hold the server, variables and credentials per security scheme (bearer, basic, API key, OAuth2 client credentials/password, OpenID Connect); `env_name` selects one. Interactive OAuth (browser, device code) needs the user: ask them to run `lazyapi login <spec>`. Details: [references/environments.md](references/environments.md).

## Rules

- Writes (POST/PUT/PATCH/DELETE) only against a mock or a test environment the user named, never production without explicit consent.
- "lazyapi blocked …" means the user has not allowed that server; ask them, do not run `lazyapi allow` yourself.
- When the API's real responses disagree with the spec (`contract` fails), tell the user; fix the spec only if they own it, otherwise code defensively and note it.
- Keep secrets in env files referenced as `{{env.NAME}}`; never paste them into code, specs or chat.
