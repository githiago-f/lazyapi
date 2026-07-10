# Installation

## Build from source

Requires Go 1.25 or later.

```bash
git clone https://github.com/githiago-f/lazyapi.git
cd lazyapi
go build -o lazyapi ./cmd/lazyapi
```

## Download a release

Pre-built binaries are available on the [releases page](https://github.com/githiago-f/lazyapi/releases) for:

| Platform | Architectures |
|----------|--------------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64 |

## Verify

```bash
./lazyapi --help
```
