# `lazyapi add`

Add operations or servers to an existing OpenAPI spec.

## `add request`

```bash
lazyapi add request <file> <path> <method>
```

| Argument | Description |
|----------|-------------|
| `file` | Path to an OpenAPI YAML file |
| `path` | URI path (e.g., `/users`, `/users/{id}`) |
| `method` | HTTP method: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`, `HEAD` |

### Examples

```bash
lazyapi add request my-api.yml /users GET
lazyapi add request my-api.yml /users POST
lazyapi add request my-api.yml /users/{id} GET
lazyapi add request my-api.yml /users/{id} DELETE
```

## `add server`

```bash
lazyapi add server <file> <url>
```

| Argument | Description |
|----------|-------------|
| `file` | Path to an OpenAPI YAML file |
| `url` | Server URL (e.g., `https://api.example.com`) |

### Examples

```bash
lazyapi add server my-api.yml https://api.example.com
lazyapi add server my-api.yml http://localhost:3000
```
