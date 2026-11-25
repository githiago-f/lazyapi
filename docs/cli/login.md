# login / logout

Log in to an OAuth2 or OpenID Connect security scheme once; `run`, `send`, flows, MCP and the TUI then use the stored session and refresh it on their own.

```bash
lazyapi login <spec.yml> [scheme] [--env-name name] [--device] [--no-browser]
lazyapi login <spec.yml> --status
lazyapi logout <spec.yml> [scheme]
```

The scheme is one of the spec's `oauth2` / `openIdConnect` schemes, with the environment's `auth:` applied (or an `auth:` entry with an `issuer` that the spec does not declare). With a single scheme the name can be left out.

| Flag | Meaning |
|------|---------|
| `--env-name` | environment from `.lazyapi/config.yml` (default: its `default`, or `$LAZYAPI_ENV`) |
| `--env` | extra env file for `{{env.X}}` |
| `--device` | use the device code grant: prints a URL and a code to enter on any device |
| `--no-browser` | only print the login URL |
| `--status` | list the stored sessions and when they expire |

```console
$ lazyapi login api.yml --env-name dev
keycloak: log in in the browser — https://sso.dev.example.com/realms/acme/protocol/openid-connect/auth?…
✓ logged in to keycloak [dev] (expires in 5m0s, refreshable)

$ lazyapi login api.yml --status
keycloak             https://sso.dev.example.com/realms/acme            expires in 4m31s, refreshable
```

Sessions live in `.lazyapi/.local/tokens.json` next to the spec (git-ignored, mode `600`). `logout` removes them (all, or one scheme's); it does not end the session at the provider.

See [Authentication](/authentication#oauth2-and-openid-connect) for grants, discovery and a Keycloak example.
