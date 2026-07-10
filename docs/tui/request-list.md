# Request List

The request list displays all operations discovered from your loaded OpenAPI files.

## Grouping

Operations are grouped by resource using `GroupByResource`. The first path segment becomes the group header:

```
Sales
  GET    /sales          List all sales
  POST   /sales          Create a sale
  GET    /sales/{id}     Get sale by ID

Users
  GET    /users          List all users
  POST   /users          Create a user
  DELETE /users/{id}     Delete a user
```

Each list item shows:

- **Method badge** — color-coded (GET=green, POST=blue, DELETE=red, etc.)
- **URI** — the endpoint path
- **Summary** — from the OpenAPI `summary` field

## Actions

| Key | Action |
|-----|--------|
| `Enter` | Open the selected request for editing |
| `Ctrl+N` | Create a new draft request |
| `d` | Duplicate the selected request |
| `x` | Delete the selected request |
| `/` | Filter by method, path, or summary |

## Filtering

Press `/` to enter filter mode. Type to narrow down items by method name, path, or summary text. Press `Esc` to clear the filter.
