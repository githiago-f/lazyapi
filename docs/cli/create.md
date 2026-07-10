# `lazyapi create file`

Creates a minimal OpenAPI 3.0 template file.

## Usage

```bash
lazyapi create file [name] [servers...]
```

## Arguments

| Argument | Description | Default |
|----------|-------------|---------|
| `name` | Filename for the new spec | `openapi.yml` |
| `servers...` | One or more server URLs | — |

## Examples

```bash
# Create default openapi.yml
lazyapi create file

# Create named file
lazyapi create file my-api.yml

# Create with server URLs
lazyapi create file my-api.yml https://api.example.com http://localhost:3000
```

## Output

```yaml
openapi: 3.0.0
info:
  title: My API
  version: 0.0.1
servers:
  - url: https://api.example.com
  - url: http://localhost:3000
paths: {}
```
