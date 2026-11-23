# Environment Variables

LazyAPI supports template resolution in URLs, headers, body, params, and auth fields using `{{...}}` syntax.

## Syntax

| Pattern | Source | Example |
|---------|--------|---------|
| `{{env.VAR_NAME}}` | System environment or `.env` file | `{{env.API_KEY}}` |
| `{{var.VAR_NAME}}` | Request-scoped variables | `{{var.userId}}` |
| `{{lazyapi.*}}` | Reserved for future use | — |

## Usage

```bash
# Load from .env file
./lazyapi my-api.yml --env .env

# In the TUI editor, type directly into any field:
https://{{env.BASE_URL}}/api/users

# In headers:
Authorization: Bearer {{env.TOKEN}}

# In request body:
{"apiKey": "{{env.API_KEY}}"}
```

## .env file format

```
BASE_URL=api.example.com
API_KEY=sk-abc123
TOKEN=eyJhbGci...
```

## Resolution order

1. System environment variables
2. `.env` file variables (overrides system if duplicate)
3. Request-scoped `{{var.X}}` values (set programmatically)
