# `lazyapi import`

Brings requests from elsewhere into an OpenAPI spec: curl commands, Postman collections and HAR captures.

```bash
lazyapi import curl '<curl command>' --into api.yml      # or: --into api.yml -- curl ...   or: - (stdin)
lazyapi import postman shop.postman_collection.json --into api.yml
lazyapi import har capture.har --into api.yml
```

`--into` creates the spec when it does not exist. Operations that are already in the spec (same method and path) are skipped. Use `--dry-run` to see what would change and `--json` for a machine-readable summary.

## What gets created

| From the request | In the spec |
|------------------|-------------|
| URL origin | a `servers` entry (an existing server with the same origin and base path is reused) |
| `/users/42`, `/users/:id`, `/users/{{id}}`, UUIDs | `/users/{id}` with the observed value as `example`; a second id becomes `{orderId}` and so on |
| Query string | query parameters with examples (integers detected) |
| Custom headers | header parameters (transport headers such as `Accept`, `User-Agent` and `Cookie` are skipped) |
| `Authorization: Bearer/Basic`, `X-API-Key`-like headers | a security scheme (`bearerAuth`, `basicAuth`, `apiKeyAuth`) on the operation. **The secret is not copied**: set it in [`lazyapi.config.yml`](/environments) |
| Body | `requestBody` with the example and a schema inferred from it (form and multipart fields included) |
| Responses (Postman examples, HAR) | one response per status with example and inferred schema |
| Request name (Postman) | `summary` and a camelCase `operationId` (`List products` → `listProducts`); otherwise derived from method and path |
| Folder (Postman) | tag |

Inferred schemas list the observed properties but do not mark them required: one example cannot tell which fields are optional. Tighten the schemas afterwards.

## Postman

- Collections v2.0 and v2.1. Auth is taken from the collection, the folder or the request (bearer, basic, apikey). Bodies can be raw, urlencoded, formdata or graphql.
- `{{name}}` becomes <code v-pre>{{env.name}}</code>. Collection variables are written to `.env.postman` next to the spec (mode 600; add it to `.gitignore`). A `postman` environment is created in `lazyapi.config.yml` when there is no config yet.
- **Scripts run unchanged.** Pre-request scripts go into `before(req)` and test scripts into the operation's test file. The `pm` compatibility layer provides:
  - `pm.test`;
  - `pm.expect` with Chai-style chains: `to.be.ok`, `.to.equal`, `.eql`, `.include`, `.property`, `.an('array')`, `.above`, `.lengthOf`, `.not`, …;
  - `pm.response` (`code`, `status`, `json()`, `text()`, `headers.get`, `responseTime`, `to.have.status`, `to.have.header`);
  - `pm.request` (headers `add`/`upsert`/`remove` in the pre-request phase);
  - `pm.environment` / `pm.variables` / `pm.collectionVariables` / `pm.globals`, all backed by `env`.

  `pm.sendRequest` is not supported; use `lazyapi.send(operationId, options)`.

## HAR

Export from the browser DevTools (Network → Save all as HAR). Only API traffic is imported: page assets (images, CSS, JS, fonts, HTML), CORS preflights and failed requests are dropped. Requests with the same method and path template are merged, keeping one response example per status.

## After importing

```bash
lazyapi validate spec api.yml
lazyapi mock api.yml &           # the imported examples become a fake API
lazyapi run api.yml --server http://127.0.0.1:4010
```

MCP agents can use `import_curl` and `export_curl`.
