# Configuration

## Theme

LazyAPI uses the [Catppuccin Mocha](https://catppuccin.com/) color palette. The 29 named colors are defined in `internal/config/config.go`.

Primary accent colors:

| Color | Hex |
|-------|-----|
| Rosewater | `#f5e0dc` |
| Flamingo | `#f2cdcd` |
| Pink | `#f5c2e7` |
| Mauve | `#cba6f7` |
| Red | `#f38ba8` |
| Maroon | `#eba0ac` |
| Peach | `#fab387` |
| Yellow | `#f9e2af` |
| Green | `#a6e3a1` |
| Teal | `#94e2d5` |
| Blue | `#89b4fa` |
| Base | `#1e1e2e` |
| Mantle | `#181825` |
| Crust | `#11111b` |

## Keymap

All keybindings are defined in `internal/config/keymap.go` and can be remapped in the source. The default bindings are:

| Action | Keys |
|--------|------|
| Move up | `k`, `↑` |
| Move down | `j`, `↓` |
| Move left | `h`, `←` |
| Move right | `l`, `→` |
| Select | `Enter`, `Space` |
| Back | `Esc` |
| Next field | `Tab` |
| Prev field | `Shift+Tab` |
| Save | `ctrl+o` |
| Save example | `Ctrl+E` |
| New request | `Ctrl+N` |
| Duplicate | `d` |
| Delete | `x` |
| Filter | `/` |
| Quit | `q` |
| Kill | `Ctrl+C` |

## Interface texts

The texts of the TUI's tabs (tab names, list titles and hints, field labels and placeholders, empty states, the response view's messages) live in [`internal/config/messages.yml`](https://github.com/githiago-f/lazyapi/blob/main/internal/config/messages.yml), embedded in the binary at build time.

- **Change the defaults**: edit `messages.yml` and rebuild (`go build ./cmd/lazyapi`). `go test ./internal/config` fails when the code uses a key the file lacks.
- **Override at run time**: point `LAZYAPI_MESSAGES` at a YAML file with the same layout and only the keys you change, e.g. a translation:

```yaml
# pt-BR.yml
editor:
  tabs:
    auth: Autenticação
    headers: Cabeçalhos
  auth:
    empty: "Nenhuma autenticação. {add} adiciona um scheme."
```

```bash
LAZYAPI_MESSAGES=pt-BR.yml lazyapi api.yml
```

`{name}` placeholders (such as `{add}`, the key that adds a row) are filled in by LazyAPI: keep them in your texts. An unknown key or invalid YAML is reported when the TUI starts, and the built-in texts are used.

