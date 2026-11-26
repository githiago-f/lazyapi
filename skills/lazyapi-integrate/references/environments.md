# Environments, credentials and servers

`.lazyapi/config.yml` next to the spec (committed) names environments. Secrets stay in env files (`.lazyapi/secrets/*.env` is git-ignored) and are referenced as `{{env.NAME}}`.

```yaml
default: local
environments:
  local:
    server: http://127.0.0.1:4010
  staging:
    server: https://staging.example.com
    env: secrets/staging.env          # API_TOKEN=…, CLIENT_SECRET=…
    vars: { tenant: demo }            # {{var.tenant}}
    auth:                             # keys = security scheme names in the spec
      bearerAuth:   { token: "{{env.API_TOKEN}}" }
      apiKeyAuth:   { value: "{{env.API_KEY}}" }
      basicAuth:    { username: "{{env.USER}}", password: "{{env.PASSWORD}}" }
      oauth:        { clientId: "{{env.CLIENT_ID}}", clientSecret: "{{env.CLIENT_SECRET}}", scopes: "read write" }
      keycloak:     { issuer: https://sso.example.com/realms/acme, clientId: app, grantType: deviceCode }
    http: { caCert: certs/ca.pem, proxy: http://proxy:3128 }
```

- Select with `env_name` (MCP) or `--env-name` (CLI); `default` (or `$LAZYAPI_ENV`) applies otherwise. Explicit `server` / `--server` wins.
- OAuth2 client credentials and password grants fetch and refresh tokens by themselves. Authorization code and device code need a person: the user runs `lazyapi login <spec> [scheme]` once (tokens in `.lazyapi/.local/tokens.json`, 0600).
- Schemes the spec cannot express (HMAC signatures, a token from a login call): export `before(req)` from the operation's test script and set `req.headers`.
- List what an environment resolves to without sending: `send_request {…, dry_run: true}` or `export_curl`.
