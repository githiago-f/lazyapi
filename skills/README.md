# LazyAPI skills for agents

Plug-and-play API work for coding agents: an MCP server that calls APIs from their OpenAPI spec, plus four short skills that tell the agent how to use it — so it explores, tests or integrates with an API without reading the API's documentation.

| Skill | Use it to |
|---|---|
| [`lazyapi-setup`](lazyapi-setup/SKILL.md) | install `lazyapi` and connect its MCP server to the agent |
| [`lazyapi-explore`](lazyapi-explore/SKILL.md) | find what an API can do, read one operation's contract, call it; build a spec from curl / Postman / HAR |
| [`lazyapi-test`](lazyapi-test/SKILL.md) | contract checks, test scripts, flows, data-driven runs, mock server, CI reports |
| [`lazyapi-integrate`](lazyapi-integrate/SKILL.md) | write client code from verified requests and real schemas, develop against the mock |

The skills follow the [Agent Skills](https://agentskills.io) format (`SKILL.md` with `name` and `description`), which Claude Code, Codex, Gemini CLI, DeepSeek Harness, OpenCode, Cursor and others read. Each `SKILL.md` is short; details sit in `references/` and are opened only when needed.

## 1. Install the binary

```bash
go install github.com/githiago-f/lazyapi/cmd/lazyapi@latest
```

or download a release from https://github.com/githiago-f/lazyapi/releases and put `lazyapi` on your `PATH`.

## 2. Install the skills and the MCP server

### Any agent — `npx skills`

```bash
npx skills add githiago-f/lazyapi                       # pick agents and skills interactively
npx skills add githiago-f/lazyapi -a claude-code -a codex -a gemini-cli -y
```

This installs the skills only; connect the MCP server with your agent's command below (or let the `lazyapi-setup` skill do it).

### Claude Code — plugin (skills + MCP server)

```
/plugin marketplace add githiago-f/lazyapi
/plugin install lazyapi@lazyapi
```

### Gemini CLI — extension (skills + MCP server)

```bash
gemini extensions install https://github.com/githiago-f/lazyapi
```

### Codex

```bash
npx skills add githiago-f/lazyapi -a codex -y           # skills in .agents/skills
codex mcp add lazyapi -- lazyapi mcp --root .
```

### DeepSeek Harness, OpenCode and other agents

Copy (or `npx skills add`) the `skills/lazyapi-*` folders into the agent's skills directory — most read `~/.agents/skills` or `.agents/skills` — and register the server in its MCP config:

```json
{ "mcpServers": { "lazyapi": { "command": "lazyapi", "args": ["mcp", "--root", "/abs/path/to/project"] } } }
```

## Security: the agent only calls servers you allow

The MCP server holds every request to a server you have not allowed and asks **you**, in the agent's UI (MCP elicitation):

- **local servers** (`localhost`, `127.0.0.1`, `::1`, any port): asked once, then remembered;
- **other servers**: allow *only this request*, *for this session* or *always* — reads (GET, HEAD, OPTIONS) and writes (POST, PUT, PATCH, DELETE…) separately;
- refusals hold for the rest of the tool call, so a run of 50 operations asks once.

This covers everything the server sends: operations, `lazyapi.send` in test scripts and flows, OAuth token calls, redirects and `https://` imports in scripts. Agents whose client cannot show the question get a refusal telling them to ask you. Grant or review access yourself:

```bash
lazyapi allow localhost
lazyapi allow https://api.example.com            # reads only
lazyapi allow https://staging.example.com --writes
lazyapi allow --list
lazyapi allow --revoke https://api.example.com
```

Remembered answers live in your config directory (`~/.config/lazyapi/permissions.json` on Linux), never in the project, so a repository cannot pre-approve servers. For a single MCP setup, add `--allow <origin>` / `--allow-writes <origin>` to its `lazyapi mcp` command. `--permissions off` turns the check off (trusted automation only).

The skills also tell agents to read before they write, never to invent IDs, to keep secrets in environments and never to grant themselves access.

More: https://lazyapi.ia.br/agents
