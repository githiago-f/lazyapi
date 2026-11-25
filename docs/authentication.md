# Authentication

LazyAPI reads the security schemes of your OpenAPI spec (`components.securitySchemes` and the `security` blocks) and applies them to every request that needs them: in the TUI, `run`, `send`, flows and `lazyapi.send()` in scripts.

## Types

| Type | OpenAPI | What is sent |
|------|---------|--------------|
| **Basic** | `type: http, scheme: basic` | `Authorization: Basic …` from username + password |
| **Bearer** | `type: http, scheme: bearer` | `Authorization: Bearer <token>` |
| **API Key** | `type: apiKey` (header, query or cookie) | the key under its name |
| **OAuth2** | `type: oauth2` | a token from the flow's endpoints |
| **OpenID Connect** | `type: openIdConnect` | a token from endpoints **discovered** from the issuer |

Credentials (tokens, passwords, client secrets) never go into the spec: they come from the environment's `auth:` in [`.lazyapi/config.yml`](/environments#credentials), usually as `{{env.X}}` references to the git-ignored `secrets/<env>.env`.

## OAuth2 and OpenID Connect

### Grants

| Grant | Needs the user | How the token is obtained |
|-------|----------------|---------------------------|
| `clientCredentials` | no | client id + secret, on every token expiry |
| `password` | no | client + username + password |
| `authorizationCode` | yes, once | the browser opens the login page; a local server receives the redirect (PKCE, S256) |
| `deviceCode` | yes, once | you open a URL on any device and type a code (no local browser needed: SSH, containers, WSL) |
| `implicit` | — | not supported: use `authorizationCode` with the same client |

Tokens are stored per project in `.lazyapi/.local/tokens.json` (git-ignored, mode `600`) and **refreshed automatically** with the refresh token. You log in once; every request of every operation using the scheme reuses the session, also after restarting LazyAPI.

### Discovery (`.well-known`)

Give an **issuer** instead of endpoints: LazyAPI fetches `<issuer>/.well-known/openid-configuration` (then `/.well-known/oauth-authorization-server`, RFC 8414) and takes the authorization, token and device endpoints from it. A full `.well-known` URL works too. Explicit `authUrl` / `tokenUrl` win over discovered ones.

### Example: Keycloak

The issuer of a Keycloak realm is `https://<host>/realms/<realm>`. Declare the scheme once in the spec:

```yaml
components:
  securitySchemes:
    keycloak:
      type: openIdConnect
      openIdConnectUrl: https://sso.example.com/realms/acme
security:
  - keycloak: [openid]        # every operation; or per operation
```

and the client per environment (the issuer can differ between environments, it overrides the spec's):

```yaml
# .lazyapi/config.yml
environments:
  dev:
    server: https://api.dev.example.com
    auth:
      keycloak:
        issuer: https://sso.dev.example.com/realms/acme
        clientId: lazyapi-cli                 # a public client with PKCE
        # grantType: deviceCode               # instead of the browser
        # redirectUri: http://localhost:8765/callback   (the default)
        # params: {kc_idp_hint: google}       # extra authorization parameters
  ci:
    auth:
      keycloak:
        grantType: clientCredentials          # a service account in CI
        clientId: "{{env.KC_CLIENT_ID}}"
        clientSecret: "{{env.KC_CLIENT_SECRET}}"
```

In Keycloak, the client needs **Standard flow** (and/or **OAuth 2.0 Device Authorization Grant**) enabled and `http://localhost:8765/*` among its **Valid redirect URIs**. Use another port with `redirectUri`; port `0` picks a free one (only for servers that accept any loopback port).

Then:

- **TUI** — send any request: when there is no session, LazyAPI logs in (opens the browser, or shows the device code, and copies the URL to the clipboard) and resends the request. You can also log in from the Auth tab (**Login** button or `ctrl+l`), or with `ctrl+k` → *Log in · keycloak*.
- **CLI** — `lazyapi login api.yml` once (see [login](/cli/login)), then `run` / `send` use and refresh the session. Without a session, a request fails with `login required … run "lazyapi login"`; in CI use `clientCredentials`.

The same works for Auth0, Okta, Entra ID, Google, Cognito, Zitadel or any provider with a discovery document (`params: {audience: …}` for Auth0).

## TUI: the Auth tab

Open a request, go to the **Auth** tab and press `enter` (or click inside it):

- `ctrl+r` adds a scheme; `⌫`/`del` on its **Scheme** select removes it.
- **Scheme** reuses a scheme of the spec (its definition is copied in, and saving keeps the operation pointing at it) or defines a new one (**Name** sets its name in `components.securitySchemes`).
- The rows follow the **Type** and the **Grant**: an OpenID Connect scheme asks for the issuer; an authorization-code grant for the redirect URI; a password grant for the user.
- `ctrl+t` (or a click on **☑ on**) switches a scheme off for this request without losing it.
- **Login** (or `ctrl+l`) logs in now; the line next to it shows the session (`✓ logged in · expires in 4m`).
- Selects open with `enter`/`space` or a click; `↑`/`↓`, `home`/`end`, the mouse wheel and typing the first letters move in the list; `enter` or a click picks, `esc` or a click outside cancels.

Saving (`ctrl+o`) writes the scheme *definition* to the spec (type, issuer, grant, endpoints, scopes), never the values typed for secrets.

## Global vs per-operation security

LazyAPI reads both:

- **Global security** — defined at the spec root level, applies to all operations
- **Operation security** — defined per path+method, overrides global

```yaml
# Global — applies to all operations
security:
  - ApiKeyAuth: []

# Operation-level override
/users:
  get:
    security:
      - BearerAuth: []
```

## Other ways to log in

- A token you already have: `accessToken` (OAuth2) or `token` (Bearer) in the environment's `auth:`.
- A custom login endpoint (session cookie, JWT from `POST /login`): get the token in [`before(req)`](/scripting#changing-the-request-before-req) with `lazyapi.send()`, or in a [flow](/scripting) step that stores it with `vars.set`.
