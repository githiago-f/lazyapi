---
name: lazyapi-test
description: Test an HTTP API against its OpenAPI spec with lazyapi — contract checks (status and response schema validated automatically), JavaScript test scripts per operation, multi-step flows (login → create → read → delete), data-driven runs, a mock server when the API is not running, and CI reports (JUnit, TAP, JSON). Use when asked to test, validate, smoke-test or check an API or its contract, write API tests, or find why an endpoint fails.
---

# Test an API with lazyapi

Use the lazyapi MCP tools when listed; otherwise the CLI with `--output json`. If neither exists, use the lazyapi-setup skill. To learn an operation first, see lazyapi-explore (`describe_operation`).

## The loop

1. **Run what exists** — `run_tests {spec}` · `lazyapi run <spec> --output json`. Every operation gets: request matches the spec → status is declared → response matches the schema (`contract`) → its test script.
2. **Read failures only** — the report has `summary` and per-operation `outcome` + `checks`; bodies are included only for operations that did not pass. Decide from the `message` whether the API or the spec is wrong before changing either.
3. **Encode expectations** — `write_test {spec, operation, source}` saves `.lazyapi/tests/<operationId>.js` and links it (`x-lazyapi-tests`); compile errors come back at once. API cheat sheet: [references/scripts.md](references/scripts.md); full types: `script_api`.
   ```js
   test("returns the user", () => {
     expect(response).toHaveStatus(200);
     expect(response.json()).toHaveProperty("email");
   });
   ```
4. **Scenarios across operations** — `write_flow {spec, name, source}` then `run_flow {spec, flow}` · `lazyapi run <spec> --flow .lazyapi/flows/<name>.js`. See [references/flows.md](references/flows.md).
5. **Re-run** until green; then everything: `run_tests {spec, include_flows: true}` · `lazyapi run <spec> --flows`.

## Selecting

`tags`, `operations` (ids or `"METHOD /path"`), `grep` (regex over `METHOD /path operationId summary`), `params` (path params for every operation), `headers`, `env_name`, `server`, `bail`, `dry_run`. CLI: `--tag`, `--op`, `--grep`, `--param k=v`, `--header k=v`, `--env-name`, `--server`, `--bail`, `--dry-run`, `--retries n`, `--parallel n`.

## Safety rules

- Only GET/HEAD/OPTIONS run unless `methods` is set. Use `methods: ["all"]` (`--methods all`) only when the user agreed that writes may hit that server — never production without explicit consent.
- Operations whose path parameters have no value are skipped, never guessed. Give `params`, or add an `example` to the parameter.
- Tests that create data must clean up: `export function after(res) { … }` runs even when tests fail.
- "lazyapi blocked …" means the user has not allowed that server: ask them; never run `lazyapi allow` yourself.
- Secrets only through environments (`{{env.NAME}}`, `.lazyapi/config.yml` `auth:`); never in specs, scripts or reports you paste.

## No server, or error paths

```bash
lazyapi mock <spec> --port 4010    # answers with the spec's examples; invalid requests → 422 with the reason
```
Then `server: "http://127.0.0.1:4010"`. Force a status or example with the header `Prefer: code=404` or `Prefer: example=<name>`.

## Data-driven

`run_tests {spec, data_file: "rows.csv"}` · `lazyapi run <spec> --data rows.csv` runs the selection once per row (CSV with a header, or a JSON array); row values are `{{env.<column>}}`, results carry `iteration`.

## CI

```bash
lazyapi validate spec <spec>
lazyapi run <spec> --frozen-lockfile --output junit --output-file report.xml   # or --output tap|json
```
Exit codes: 0 passed · 1 a check failed · 2 usage error · 3 could not run. Commit the spec, `.lazyapi/tests/`, `.lazyapi/flows/` and `.lazyapi/lazyapi.lock`.
