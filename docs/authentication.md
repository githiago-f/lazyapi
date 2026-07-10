# Authentication

LazyAPI supports four authentication types extracted from your OpenAPI spec's `securitySchemes` and `security` blocks.

## Types

| Type | Description |
|------|-------------|
| **Basic Auth** | Username + password |
| **Bearer Token** | Single token value |
| **API Key** | Key name + value, passed in header or query |
| **OAuth2** | Client credentials grant (Client ID, Client Secret, Token URL) |

## TUI: editing auth

Open a request in the editor and navigate to the **Authorize** tab. Use `n` + `a` to add a new scheme, then fill in the fields.

## Security: secrets never persist

When you save a request back to the spec, LazyAPI writes the security scheme *definition* (type, key name, grant type, etc.) but **not** the runtime values (passwords, tokens, secrets). These are kept in session temp files only.

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
