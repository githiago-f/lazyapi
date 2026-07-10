# Environment Variables

LazyAPI supports template resolution in URLs, headers, body, params, and auth fields using `&#123;&#123;...&#125;&#125;` syntax.

## Syntax

| Pattern | Source | Example |
|---------|--------|---------|
| `&#123;&#123;env.VAR_NAME&#125;&#125;` | System environment or `.env` file | `&#123;&#123;env.API_KEY&#125;&#125;` |
| `&#123;&#123;var.VAR_NAME&#125;&#125;` | Request-scoped variables | `&#123;&#123;var.userId&#125;&#125;` |
| `&#123;&#123;lazyapi.*&#125;&#125;` | Reserved for future use | — |

## Usage

```bash
# Load from .env file
./lazyapi my-api.yml --env .env

# In the TUI editor, type directly into any field:
https://&#123;&#123;env.BASE_URL&#125;&#125;/api/users

# In headers:
Authorization: Bearer &#123;&#123;env.TOKEN&#125;&#125;

# In request body:
{"apiKey": "&#123;&#123;env.API_KEY&#125;&#125;"}
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
3. Request-scoped `&#123;&#123;var.X&#125;&#125;` values (set programmatically)
