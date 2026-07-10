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
| Save | `Ctrl+S` |
| Save example | `Ctrl+E` |
| New request | `Ctrl+N` |
| Duplicate | `d` |
| Delete | `x` |
| Filter | `/` |
| Quit | `q` |
| Kill | `Ctrl+C` |
