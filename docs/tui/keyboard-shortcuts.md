# Keyboard Shortcuts

The bottom bar always shows the keys that work in the current context. Press `?` for the full list.

## Global

| Key | Action |
|-----|--------|
| `?` | Toggle help (ignored while typing in a field) |
| `ctrl+v` | Switch environment: cycles `.env`, `.env.<name>`, `<name>.env` in the current directory, and "none" |
| `esc` | Cancel the request in flight (while the spinner is shown) |
| `ctrl+c` | Quit |

## Request List

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | Move |
| `enter` | Open in the editor |
| `s` | Send the selected request. POST/PUT/PATCH/DELETE ask for confirmation |
| `[` / `]` | Previous / next response tab (Body, Headers, Checks, Console) |
| `pgup` / `pgdown` | Scroll the response |
| `o` | Cycle the server |
| `/` | Filter by method, path, summary or tag (`esc` clears) |
| `ctrl+n` | New draft request |
| `d` | Duplicate |
| `x` | Delete |
| `ctrl+t` | Edit tags |
| `q` | Quit |

## Request Editor

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Next / previous field: method → server → URI → Send → tabs → response |
| `alt+enter` | Send from any field |
| `enter` (on Send) | Send |
| `F5` | Run the test script on the last response (sends first if there is none) |
| `ctrl+o` | Save to the spec (definitions, test script) |
| `ctrl+e` | Save the last response as an example in the spec |
| `ctrl+x` | Open the Body or Tests tab in `$VISUAL` / `$EDITOR` |
| `esc` | Leave tab content, then back to the list |

### Tabs

| Key | Tab | Action |
|-----|-----|--------|
| `←` / `→` | (tab bar) | Switch tab |
| `enter` | (tab bar) | Edit the tab content |
| `ctrl+q` | Params | Add query parameter |
| `ctrl+p` | Params | Add path parameter |
| `ctrl+g` | Headers | Add header |
| `ctrl+r` | Auth | Add auth scheme |
| `del` / `⌫` | Auth | Delete the selected auth scheme |

### Response panel (when focused)

| Key | Action |
|-----|--------|
| `[` / `]` or `1`–`4` | Body / Headers / Checks / Console |
| `r` | Toggle raw / pretty body |
| `/` | Search the response; `n` / `N` jump between matches |
| `ctrl+y` | Copy the response body |
| `↑` `↓` `pgup` `pgdown`, mouse wheel | Scroll |
