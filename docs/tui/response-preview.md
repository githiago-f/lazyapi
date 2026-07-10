# Response Preview

After sending a request, the response appears in the bottom panel of the editor.

## Displayed information

- **Status code** — e.g., `200 OK`, `404 Not Found`
- **Headers** — response headers from the server
- **Body** — raw response body (scrollable)

## Scrolling

Use `PgUp` / `PgDn` or the mouse wheel to scroll through long responses.

## Saving examples

Press `Ctrl+E` to save the last response as an OpenAPI example in your spec. This writes the status code, headers, and body into the operation's `responses.<code>.content.<type>.example` field.
