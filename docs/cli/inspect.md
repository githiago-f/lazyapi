# Inspecting specs

These commands help agents (and humans) understand an API before calling it. Each accepts `--json`.

## `list ops`

```bash
lazyapi list ops <spec.yml> [--tag t1,t2] [--json]
```

Lists every operation: method, path, `operationId`, summary, tags and test script file.

## `describe`

```bash
lazyapi describe <spec.yml> <operationId | "METHOD /path"> [--json]
```

Prints everything needed to call and test one operation:

- parameters, with location, required flag, schema and a sample value (`example`, `examples`, schema `example`/`default`/`enum`)
- request body media types, schemas and examples
- responses by status, with schemas
- `schemas`: every component schema referenced above, followed recursively and keyed by name (`$ref`s are kept so recursive schemas stay finite)
- security scheme names, servers, and the current test script source

## `validate spec`

```bash
lazyapi validate spec <spec.yml> [--json]
```

Reports **errors** (exit 1):
- invalid OpenAPI
- duplicate `operationId`
- a test script that is missing, outside the spec directory, written in Lua, or does not compile

Reports **warnings**:
- a missing `operationId`
- operations without declared responses
- a spec without servers

URL imports are resolved from the module cache only; `validate` never downloads.
