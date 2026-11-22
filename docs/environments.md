# Environments & Auth

`lazyapi.config.yml` defines named environments. Each one sets a server, an env file, initial variables and credentials per security scheme. LazyAPI looks for the file next to the spec, then in the working directory.

```yaml
default: local                 # used when no --env-name is given ($LAZYAPI_ENV also works)
environments:
  local:
    server: http://127.0.0.1:4010
    env: .env.local            # relative to this file
    vars:                      # initial {{var.X}} values
      tenant: demo
  staging:
    server: https://staging.example.com
    env: .env.staging
    auth:                      # keys are security scheme names from the spec
      bearerAuth:
        token: "{{env.API_TOKEN}}"
      apiKeyAuth:
        value: "{{env.API_KEY}}"
      basicAuth:
        username: "{{env.USER}}"
        password: "{{env.PASSWORD}}"
      oauth:
        clientId: "{{env.CLIENT_ID}}"
        clientSecret: "{{env.CLIENT_SECRET}}"
        scopes: "read write"   # optional: overrides the spec's scopes
  production:
    server: "1"                # an index into the spec's servers also works
    env: .env.production
```

Keep secrets in the env files (gitignored) and reference them with templates. The config file itself can be committed.

## Using an environment

| Where | How |
|-------|-----|
| CLI | `lazyapi run api.yml --env-name staging`. `--server` and `--env` still win over the environment |
| MCP | `env_name` argument on `send_request`, `run_tests`, `run_flow` |
| TUI | Starts on the `default` environment; `ctrl+v` cycles environments (then "none"). The title shows the environment and its server |

Reports include `"environment": "<name>"`.

## Credentials

Credentials are applied to every request whose operation (or the spec, globally) requires that security scheme, including `lazyapi.send()` calls from scripts and flows. Empty values are never sent, so an `Authorization` header set explicitly or by [`before(req)`](/scripting#changing-the-request-before-req) is not overwritten by an empty scheme.

| Scheme type | Fields |
|-------------|--------|
| `http` bearer | `token` |
| `http` basic | `username`, `password` |
| `apiKey` (header, query or cookie) | `value` |
| `oauth2` | `accessToken` (fixed), or `clientId` + `clientSecret` (client credentials) / + `username` + `password` (password grant); optional `scopes`, `tokenUrl` |

For OAuth2 client-credentials and password flows, LazyAPI requests a token from the flow's `tokenUrl` (client authenticated with HTTP Basic). It caches the token until shortly before `expires_in`. A failed token request fails the operation with the token server's answer. Authorization-code and implicit flows need a browser: put a token in `accessToken`.
