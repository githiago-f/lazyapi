# allow

Allow servers for agents. The MCP server (`lazyapi mcp`) asks before sending a request to a server nobody allowed; `allow` answers ahead of time, lists the answers and takes them back. It is meant for people: agents are told to ask you instead of running it.

```bash
lazyapi allow <origin> [--writes]
lazyapi allow --list
lazyapi allow --revoke <origin>
```

An origin is `scheme://host[:port]` — `https://api.example.com` is `https://api.example.com:443`, a bare `api.example.com:8443` means HTTPS — or `localhost` for every server on this machine (`localhost`, `127.0.0.1`, `::1`, `*.localhost`, any port).

| Flag | Meaning |
|------|---------|
| `--writes` | also allow POST, PUT, PATCH, DELETE… (without it: GET, HEAD and OPTIONS only; `localhost` always allows both) |
| `--list` | list the remembered origins and what they allow |
| `--revoke` | forget an origin |

```console
$ lazyapi allow localhost
Allowed localhost for reads and writes (/home/ana/.config/lazyapi/permissions.json)
$ lazyapi allow https://staging.example.com --writes
Allowed https://staging.example.com:443 for reads and writes (…)
$ lazyapi allow --list
https://staging.example.com:443          reads and writes  since 2026-11-26
localhost                                reads and writes  since 2026-11-26
```

The answers are stored in your config directory (`$XDG_CONFIG_HOME/lazyapi/permissions.json`, `~/Library/Application Support/lazyapi/` on macOS, `%AppData%\lazyapi\` on Windows; `$LAZYAPI_PERMISSIONS` overrides the file), mode `600`, never in the project. A running MCP server sees changes at once. Answers given in the agent ("always") land in the same file.

See [permissions](/agents#permissions-the-agent-only-calls-servers-you-allow) for how the MCP server asks.
