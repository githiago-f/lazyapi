# Environments & Auth

Everything LazyAPI keeps for a spec lives in a **`.lazyapi/` directory next to it**, so it is versioned with the API (git-native). Only credentials and machine-local state stay out of git:

```
api/
  openapi.yml
  .lazyapi/
    config.yml           # environments: server, vars, http, auth   (committed)
    envs/<name>.env      # variables of an environment               (committed)
    secrets/<name>.env   # credentials of an environment             (git-ignored)
    tests/<op>.js        # test scripts (x-lazyapi-tests)            (committed)
    flows/*.js           # flows                                     (committed)
    data/*.csv|json      # data files for data-driven runs           (committed)
    lazyapi.lock         # pinned URL imports                        (committed)
    .local/              # drafts, history, caches                   (git-ignored)
    .gitignore           # secrets/ and .local/ (written by lazyapi)
```

LazyAPI writes `.lazyapi/.gitignore` when it creates the directory (for example on `lazyapi import`), and never overwrites it. LazyAPI looks for `.lazyapi/config.yml` next to the spec, then in the working directory.

`.lazyapi/config.yml` defines named environments. Each one sets a server, initial variables, HTTP options and credentials per security scheme:

```yaml
default: local                 # used when no --env-name is given ($LAZYAPI_ENV also works)
environments:
  local:
    server: http://127.0.0.1:4010
    vars:                      # initial {{var.X}} values
      tenant: demo
  staging:
    server: https://staging.example.com
    auth:                      # keys are security scheme names from the spec
      bearerAuth:
        token: "{{env.API_TOKEN}}"      # API_TOKEN in secrets/staging.env
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
    env: ../shared/prod.env    # optional extra env file, relative to this file
    http:                      # TLS, proxy and redirects for this environment
      caCert: certs/internal-ca.pem     # paths are relative to this file
      clientCert: certs/client.pem
      clientKey: certs/client-key.pem
      proxy: http://proxy.internal:3128
      noRedirects: false
      maxRedirects: 5
      insecure: false
```

## Variables and secrets

The values of <code v-pre>{{env.NAME}}</code> for an environment come, in this order (later files win), from:

1. the process environment;
2. `.lazyapi/envs/<name>.env`: plain variables (base URLs, tenant ids, feature flags). **Commit it**;
3. `.lazyapi/secrets/<name>.env`: tokens, passwords, client secrets. **Git-ignored**; written with mode `600` by LazyAPI;
4. the environment's `env:` file, if any;
5. `--env <file>` on the command line.

So the config and the variables are code that reviews and CI see, and each developer (or the CI secret store) provides `secrets/<name>.env`. In CI, write the file from the secret store before running, or export the variables in the process environment.

CLI flags such as `--insecure` or `--proxy` are combined with the environment's `http` settings.

## Using an environment

| Where | How |
|-------|-----|
| CLI | `lazyapi run api.yml --env-name staging`. `--server` wins over the environment; `--env` is read last |
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
