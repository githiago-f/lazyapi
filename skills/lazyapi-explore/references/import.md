# Building a spec from what you have

lazyapi needs an OpenAPI 3 file. When the API has none:

| Source | Command | Notes |
|---|---|---|
| curl command | MCP `import_curl {spec, command}` · CLI `lazyapi import curl '<curl …>' --into api.yml` (`-` reads stdin) | path ids become `{id}` parameters |
| Postman collection v2.0/v2.1 | `lazyapi import postman collection.json --into api.yml [--environment env.json]` | `{{baseUrl}}` hosts resolved from the environment; tests are translated to lazyapi scripts, untranslatable lines are marked `TODO(postman)` |
| Postman environment | `lazyapi import postman-env staging.postman_environment.json --into api.yml` | values → `.lazyapi/envs/<name>.env`, secrets → `.lazyapi/secrets/` (git-ignored), and an environment in `.lazyapi/config.yml` |
| HAR (DevTools → Save all as HAR) | `lazyapi import har capture.har --into api.yml` | one operation per distinct method + path |
| Nothing | `lazyapi create file api.yml https://api.example.com` then `lazyapi add request api.yml /users GET` | an empty template to fill |

Every import:

- creates the spec when `--into` does not exist and keeps existing operations;
- turns `Authorization` / API-key headers into security schemes and leaves a note — the values are never copied;
- adds request/response examples and schemas inferred from them (review them: inferred types are a guess);
- accepts `--dry-run --json` to preview.

Then: `validate_spec {spec}` (or `lazyapi validate spec api.yml --json`) and explore as usual.
