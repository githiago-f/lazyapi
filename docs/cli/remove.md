# `lazyapi remove request`

Removes an operation from an OpenAPI spec.

## Usage

```bash
lazyapi remove request <file> <method> <path>
```

## Arguments

| Argument | Description |
|----------|-------------|
| `file` | Path to an OpenAPI YAML file |
| `method` | HTTP method: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`, `HEAD` |
| `path` | URI path to remove |

## Examples

```bash
lazyapi remove request my-api.yml GET /users
lazyapi remove request my-api.yml POST /sales
```

If no other methods remain on that path, the entire path entry is removed from the spec.
