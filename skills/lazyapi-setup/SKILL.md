---
name: lazyapi-setup
description: Install the lazyapi binary and connect its MCP server (API exploration, sending requests, contract checks, API tests) to the current agent — Claude Code, Codex, Gemini CLI, DeepSeek Harness or any MCP client. Use when a lazyapi skill is needed but the `lazyapi` command or the lazyapi MCP tools (list_specs, describe_operation, send_request, run_tests) are missing, or when the user asks to set up lazyapi.
---

# Set up lazyapi

lazyapi is one binary: a TUI, a CLI (`lazyapi run|send|describe|mock|import …`) and an MCP server (`lazyapi mcp`). Everything works on OpenAPI 3 specs; there is no account or server to run.

## 1. Is it already there?

```bash
lazyapi --help >/dev/null 2>&1 && echo installed
```

If the agent already lists MCP tools named `list_specs`, `describe_operation`, `send_request`, skip to the other lazyapi skills.

## 2. Install

Pick the first that works:

```bash
# Go 1.26+ (any OS)
go install github.com/githiago-f/lazyapi/cmd/lazyapi@latest

# Release binary (Linux/macOS; GitHub CLI)
gh release download --repo githiago-f/lazyapi --pattern "lazyapi_*_$(uname -s | tr A-Z a-z)_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" -O - | tar -xz lazyapi
install -m 0755 lazyapi ~/.local/bin/lazyapi   # or any directory on PATH
```

Windows: download the `windows_amd64.zip` (or `arm64`) asset from https://github.com/githiago-f/lazyapi/releases and put `lazyapi.exe` on `PATH`. Ask before installing anything system-wide.

## 3. Connect the MCP server

The server speaks MCP over stdio. `--root` confines every path to a directory (default: the working directory); use the project that holds the OpenAPI spec.

| Agent | Command |
|---|---|
| Claude Code | `claude mcp add lazyapi -- lazyapi mcp --root .` |
| Codex | `codex mcp add lazyapi -- lazyapi mcp --root .` |
| Gemini CLI | `gemini mcp add lazyapi lazyapi mcp --root .` |
| Anything else | the JSON below in its MCP config |

```json
{ "mcpServers": { "lazyapi": { "command": "lazyapi", "args": ["mcp", "--root", "/abs/path/to/project"] } } }
```

Codex `~/.codex/config.toml` equivalent:

```toml
[mcp_servers.lazyapi]
command = "lazyapi"
args = ["mcp", "--root", "."]
```

Restart the agent session after adding a server. Packaged installs (skills and MCP server together) are listed in the lazyapi repository's `skills/README.md`.

## 4. Permissions

The MCP server only sends requests to servers the user allowed. The first request to a new server is held while the user is asked in the agent's UI (MCP elicitation): local servers once, other servers for one request, the session or always — reads and writes separately. Clients that cannot ask get a "lazyapi blocked …" result instead.

Only the user grants access. Tell them the options; do not run these yourself:

- `lazyapi allow localhost` / `lazyapi allow https://api.example.com [--writes]` — remembered in their config directory (`lazyapi allow --list`, `--revoke <origin>`);
- `--allow <origin>` / `--allow-writes <origin>` added to the MCP server command, for that server only.

## 5. Check

Call the `list_specs` tool (or run `lazyapi list ops <spec> --json`). An empty list means there is no OpenAPI file under the root: see the lazyapi-explore skill ("No spec yet").

## Without MCP

Every MCP tool has a CLI equivalent with JSON output, so the lazyapi skills also work through a shell: `lazyapi list ops <spec> --json`, `lazyapi describe <spec> <op> --json`, `lazyapi send request <spec> --op <id> --output json`, `lazyapi run <spec> --output json`.
