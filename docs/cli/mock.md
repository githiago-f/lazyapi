# `lazyapi mock`

Serves a spec as a fake API, so tests, examples, CI and agents have a server to talk to before (or without) the real one.

```bash
lazyapi mock <spec.yml> [--port 4010] [--host 127.0.0.1] [--no-validate] [--quiet]
```

## Responses

For each request, the mock finds the operation (literal paths win over templates: `/users/me` before `/users/{id}`). Path prefixes from the spec's `servers` (e.g. `/v1`) are accepted too. The mock then answers with:

1. **Status**: the code asked for with `Prefer: code=404`, otherwise the lowest declared 2xx, otherwise the lowest declared code.
2. **Media type**: one that matches `Accept`, preferring JSON.
3. **Body**: the named example asked for with `Prefer: example=<name>`. Otherwise, in order: the media type `example`, the first named example, a sample generated from the schema (honouring `example`, `default`, `enum`, `format` and minimums).
4. **Headers**: declared response headers that have an example or schema.

`HEAD` gets headers only. `OPTIONS` on a path without an `OPTIONS` operation answers a CORS preflight.

## Validation

Requests are validated against the operation (parameters and body). A request that does not match gets **422**:

```json
{"error": "request does not match the OpenAPI spec", "details": "body: name: property \"name\" is missing", "mock": "lazyapi"}
```

Pass `--no-validate` to answer anyway. Unknown paths get 404, undeclared methods 405 (with `Allow`). Each request is logged to stderr: `GET /users/1 → 200 getUser (41µs)`.

## Testing against the mock

```bash
lazyapi mock api.yml &
lazyapi run api.yml --server http://127.0.0.1:4010 --methods all
```

Inside test scripts, `Prefer` lets you exercise error paths the real server is hard to trigger:

```js
const missing = lazyapi.send("getUser", { params: { id: "999" }, headers: { Prefer: "code=404" } });
expect(missing).toHaveStatus(404);
expect(missing).toMatchSchema();
```

`examples/openapi.yml` and its `tests/` run this way in CI.
