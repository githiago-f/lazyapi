# Contributing to LazyAPI

First off, thanks for taking the time to contribute!

## Code of Conduct

This project is governed by the [Contributor Covenant](CODE_OF_CONDUCT.md). By participating, you agree to uphold this code.

## How can I contribute?

### Reporting bugs

Open an [issue](https://github.com/githiago-f/lazyapi/issues) with:

- A clear title and description
- Steps to reproduce
- Expected vs actual behavior
- Your environment (OS, Go version, terminal emulator)

### Suggesting features

Open an issue with:

- What you're trying to achieve
- Why it doesn't fit the current workflow
- A rough sketch of how it could work

### Pull requests

1. Open an issue first for larger changes — let's discuss before you invest time
2. Keep changes focused — one PR per feature or fix
3. Add or update tests when applicable
4. Run the full test suite before pushing
5. Update docs for user-facing changes

## Development setup

### Prerequisites

- Go 1.25+
- Git

### Clone and build

```bash
git clone https://github.com/githiago-f/lazyapi.git
cd lazyapi
go build -o lazyapi ./cmd/lazyapi
```

### Run tests

```bash
go test ./...
go test -race -count=1 ./...
```

### Lint

```bash
golangci-lint run
```

Config is in `.golangci.yml` at the project root.

### Documentation preview

```bash
cd docs
npm install
npm run extract   # extract Go doc comments into data/
npm run dev       # vitepress dev server with HMR
```

## Project conventions

### Code style

- Follow [Go standard formatting](https://go.dev/doc/effective_go) — `gofmt` / `golangci-lint` will catch most issues
- No comments on exported items unless they add context the name doesn't convey
- Use existing patterns from nearby code

### Architecture

```
cmd/lazyapi/         — Entry point, dispatches TUI or CLI
internal/
  app/               — bubbletea TUI model and views
  cli/               — CLI subcommands
  components/        — Reusable UI widgets
  config/            — Color palette and keybindings
  env/               — Environment variable resolution
  inmath/            — Math helpers
  model/             — Core domain types
  store/             — OpenAPI spec and file system operations
```

### Conventions

- **Method strings** are uppercase (`GET`, `POST`)
- **OpenAPI refs** use `{FilePath, Path, Method}` structs
- **YAML files** use `openapi: 3.0.0` at the root
- **No external CLI framework** — ad-hoc `os.Args` parsing
- **No database** — everything is file-based
- **Tests** live next to the code they test (e.g., `internal/store/openapi_test.go`)
- **Test fixtures** live in `internal/store/testdata/`
- **Avoid `/tmp`** in tests — use `t.TempDir()` for ephemeral files
- **Tab/Shift+Tab wraps** in the TUI — no exit state via Tab, user blurs with Esc

### Testing guidelines

- Focus on **roundtrip tests** (marshal → write → read → unmarshal → verify)
- Prefer programmatic spec construction for small tests
- Use fixture YAML files for realistic multi-operation scenarios
- Don't test UI interactions directly

## Pull request checklist

Before submitting:

- [ ] Build passes (`go build ./...`)
- [ ] Tests pass (`go test ./...`)
- [ ] Lint passes (`golangci-lint run`)
- [ ] Tests added for new functionality
- [ ] User-facing changes are documented (markdown in `docs/`)
- [ ] PR template checklist is filled

## Release process

Maintainers handle releases. A push of a `v*` tag triggers:

1. GoReleaser builds binaries for linux/windows/darwin (amd64 + arm64)
2. Documentation is generated and deployed to GitHub Pages

## Getting help

- Open an issue for questions or discussions
- Check [ARCHITECTURE.md](ARCHITECTURE.md) for a deep-dive into the codebase

## License

By contributing, you agree that your contributions will be licensed under the [GPLv3 License](LICENSE).
