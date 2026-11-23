# `lazyapi import`

Brings requests from elsewhere into an OpenAPI spec: curl commands, Postman collections and HAR captures.

```bash
lazyapi import curl '<curl command>' --into api.yml      # or: --into api.yml -- curl ...   or: - (stdin)
lazyapi import postman shop.postman_collection.json --into api.yml
lazyapi import postman shop.postman_collection.json --environment staging.postman_environment.json --into api.yml
lazyapi import postman-env prod.postman_environment.json --into api.yml   # one more environment
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
| `Authorization: Bearer/Basic`, `X-API-Key`-like headers | a security scheme (`bearerAuth`, `basicAuth`, `apiKeyAuth`) on the operation. **The secret is not copied**: set it in [`.lazyapi/config.yml`](/environments). When the credential is only a variable (Postman `Bearer {{token}}`), the reference goes to the environment's `auth` section, so requests authenticate without extra setup |
| Body | `requestBody` with the example and a schema inferred from it (form and multipart fields included) |
| Responses (Postman examples, HAR) | one response per status with example and inferred schema |
| Request name (Postman) | `summary` and a camelCase `operationId` (`List products` → `listProducts`); otherwise derived from method and path |
| Folder (Postman) | tag |

Inferred schemas list the observed properties but do not mark them required: one example cannot tell which fields are optional. Tighten the schemas afterwards.

## Postman

- Collections v2.0 and v2.1. Auth is taken from the collection, the folder or the request (bearer, basic, apikey). Bodies can be raw, urlencoded, formdata or graphql.
- `{{name}}` becomes `{{env.name}}`. Collection variables are written to the `postman` environment: plain ones to `.lazyapi/envs/postman.env` (committed), ones whose name looks sensitive (`token`, `secret`, `password`, `key`, …) to `.lazyapi/secrets/postman.env` (mode 600, git-ignored). A `postman` environment is created in `.lazyapi/config.yml` when there is no config yet.
- **Scripts are translated** to the native script API (there is no `pm` object). Pre-request scripts go into `before(req)` and test scripts into the operation's test file:
  - `pm.test` → `test`; `pm.expect(x)` with Chai chains (`to.be.ok`, `.equal`, `.deep.equal`, `.eql`, `.include`, `.property(p).that.equals(v)`, `.an('array')`, `.above`, `.below`, `.lengthOf`, `.empty`, `.oneOf`, `.not`, …) → `expect(x)` matchers;
  - `pm.response.to.have.status/header`, `to.be.ok` → `expect(response)…`; `pm.response.code`, `json()`, `text()`, `headers.get`, `responseTime` → `response.*`;
  - `pm.request.headers.add/upsert/remove` (pre-request) → `req.headers`;
  - `pm.environment` / `pm.iterationData` → `env`; `pm.variables` / `pm.collectionVariables` / `pm.globals` → `vars`; `pm.info.iteration` → `lazyapi.iteration`.

  Lines that cannot be translated (for example `pm.sendRequest`; use `lazyapi.send`) are commented out with `TODO(postman)`, and the header of the file says how many. If commenting a line would break the syntax, the original script is kept, commented out.
  Postman dynamic variables such as `{{$guid}}` and `{{$randomEmail}}` are kept as they are.

### Environments

A Postman environment export (`*.postman_environment.json`) becomes a named environment:

- plain values go to `.lazyapi/envs/<name>.env` (committed); secret-type values, and values whose name looks sensitive (`token`, `password`, `key`, …), go to `.lazyapi/secrets/<name>.env` (mode 600, git-ignored). They never reach the config;
- `.lazyapi/config.yml` gets `environments.<name>`, with `server` from a `baseUrl`-like variable (`baseUrl`, `base_url`, `host`, `url`, `apiUrl`, …) when it is a URL. The file is created if needed; comments and other environments are kept;
- disabled values are skipped. Existing env files and environments are never overwritten (use `--name` for another name).

`--environment` on `import postman` does the same and also uses the values to resolve hosts like `{{baseUrl}}/users` that only the environment defines. Then: `lazyapi run api.yml --env-name staging`, or `ctrl+v` in the TUI.

## HAR

Export from the browser DevTools (Network → Save all as HAR). Only API traffic is imported: page assets (images, CSS, JS, fonts, HTML), CORS preflights and failed requests are dropped. Requests with the same method and path template are merged, keeping one response example per status.

## After importing

```bash
lazyapi validate spec api.yml
lazyapi mock api.yml &           # the imported examples become a fake API
lazyapi run api.yml --server http://127.0.0.1:4010
```

MCP agents can use `import_curl` and `export_curl`.
