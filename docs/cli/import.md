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
| `Authorization: Bearer/Basic`, `X-API-Key`-like headers | a security scheme (`bearerAuth`, `basicAuth`, `apiKeyAuth`) on the operation. **The secret is not copied**: set it in [`lazyapi.config.yml`](/environments). When the credential is only a variable (Postman <code v-pre>Bearer {{token}}</code>), the reference goes to the environment's `auth` section, so requests authenticate without extra setup |
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

  - `pm.sendRequest(url | {url, method, header, body}, callback)`: the URL is mapped to the spec operation it belongs to and sent with `lazyapi.send`, so it gets the same checks. The callback runs right away (scripts are synchronous) and the response is also returned. A URL that is not an operation of the spec fails with a message telling you to add it (for example with `lazyapi import curl`). `formdata` bodies are not supported.

### Environments

A Postman environment export (`*.postman_environment.json`) becomes a named environment:

- the values go to `.env.<name>` next to the spec (mode 600; add it to `.gitignore`). Secret-type values stay there and never reach the config;
- `lazyapi.config.yml` gets `environments.<name>` with that env file, and `server` from a `baseUrl`-like variable (`baseUrl`, `base_url`, `host`, `url`, `apiUrl`, …) when it is a URL. The file is created if needed; comments and other environments are kept;
- disabled values are skipped. Existing env files and environments are never overwritten (use `--name` for another name).

`--environment` on `import postman` does the same and also uses the values to resolve hosts like <code v-pre>{{baseUrl}}/users</code> that only the environment defines. Then: `lazyapi run api.yml --env-name staging`, or `ctrl+v` in the TUI.

## HAR

Export from the browser DevTools (Network → Save all as HAR). Only API traffic is imported: page assets (images, CSS, JS, fonts, HTML), CORS preflights and failed requests are dropped. Requests with the same method and path template are merged, keeping one response example per status.

## After importing

```bash
lazyapi validate spec api.yml
lazyapi mock api.yml &           # the imported examples become a fake API
lazyapi run api.yml --server http://127.0.0.1:4010
```

MCP agents can use `import_curl` and `export_curl`.
