# TUI: Terminal User Interface

The TUI is the default mode. Launch it with:

```bash
./lazyapi
./lazyapi examples/openapi.yml        # Pre-load a spec
./lazyapi examples/openapi.yml --env .env  # With env file
```

## Layout

The screen is divided into three areas:

```
┌─────────────────────────────────────────┐
│  Title Bar                               │
├─────────────────────────────────────────┤
│                                         │
│  Main Content                            │
│  (Request List or Request Editor)       │
│                                         │
├─────────────────────────────────────────┤
│  Help Bar                                │
└─────────────────────────────────────────┘
```

## Pages

| Page | Description |
|------|-------------|
| **Request List** | Browse all operations from loaded specs |
| **Request Editor** | View and edit a single request, send it, see the response |

Switch between pages by opening a request (`Enter`) and closing it (`Esc`).

## Navigation

- `Tab` / `Shift+Tab` — cycle through fields
- `Enter` — select / open
- `Esc` — back / blur
- `q` — quit
