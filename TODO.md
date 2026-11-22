# TODO — LazyAPI para agents e humanos

Prioridades: **P0** bloqueia uso por agents/CI · **P1** alto valor · **P2** polimento.

Ordem de execução:
1. **Fase 1** — Bugs P0 + persistência dos testes em arquivos versionados.
2. **Fase 2** — Engine JS (goja + esbuild) com imports por URL, lockfile e validação de schema.
3. **Fase 3** — `lazyapi run` com `--output json` + exit codes, depois servidor MCP.
4. **Fase 4** — UI: abas de resposta, highlight, `$EDITOR`, cancelamento, seletor de ambiente.

---

## 1. Bugs e problemas (P0) — Fase 1 ✅

- [x] **Test scripts não são persistidos de forma versionada** — vivem só em `os.TempDir()/lazyapi/...`; o CLI lê o temp da máquina local (não reproduzível em CI).
- [x] **Erros no TUI derrubam o app** — `store` retorna `tea.Batch(tea.Println, tea.Quit)` como `tea.Msg` e faz `panic` em `defer Close`. Mostrar notificação de erro.
- [x] **Sem timeout HTTP** — `http.DefaultClient` sem `context`; request pode travar para sempre.
- [x] **Basic auth não resolve variáveis** (`Username`/`Password` sem `env.Resolve`).
- [x] **Testes com mesmo nome se sobrescrevem** e a ordem dos resultados é não-determinística.
- [x] **`test("x", fn)` descarta a mensagem do erro** — o agent não sabe o motivo do FAIL.
- [x] **Smoke perigoso** — dispara POST/PUT/DELETE em tudo; path params vazios viram `"1"`. Adicionar `--methods`, `--dry-run`, `--tag`, `--op`; usar `example`/`default` do spec.
- [x] **Smoke trata todo status ≥400 como erro** — critério deve vir das `responses` declaradas no spec.
- [x] **Parse de flags frágil** — flag desconhecida vira posicional; `--server` sem valor é ignorado. Migrar para `flag` da stdlib.
- [x] **`os.Remove` do temp falha quando ele não existe** ao salvar/deletar operação.
- [x] **AGENTS.md desatualizado** — não cita `internal/lua`, smoke descrito como stub, known issues já corrigidos.

## 2. Scripting JavaScript com import por URL (P0) — Fase 2

- [ ] Engine: `goja` (JS puro Go) + `esbuild` (lib Go) para transpilar ESM/TS → bundle.
- [ ] `import x from "https://..."`:
  - [ ] resolver esbuild (`onResolve`/`onLoad`) baixando via HTTPS
  - [ ] cache em `~/.cache/lazyapi/modules` por hash
  - [ ] `lazyapi.lock` com integrity sha256 por URL (runs reproduzíveis)
  - [ ] `--offline` / `--reload`
  - [ ] allowlist de hosts; rejeitar `http://`
  - [ ] imports relativos (`./helpers.js`)
- [ ] Sandbox: sem fs/processo, timeout via `vm.Interrupt`, limites configuráveis.
- [ ] API: `test()`, `expect()` (`toBe`, `toEqual`, `toContain`, `toMatch`, `toHaveHeader`, `toMatchSchema`), `res.json()`, `vars`, `env`.
- [ ] Pre-request script além do post-response.
- [ ] `lazyapi.send()` para encadear chamadas (login → criar → buscar → deletar).
- [ ] `console.log` capturado no report.
- [ ] Validação automática de contrato com `openapi3filter` (request + response vs spec).
- [ ] `lazyapi.d.ts` com tipos da API de script.
- [ ] Migrar/remover Lua.

## 3. Persistência pensada para agents (P0)

- [x] Scripts em arquivos versionados ao lado do spec, referenciados via `x-lazyapi-tests: tests/<op>.lua`. *(Fase 1 — trocar extensão para `.js` na Fase 2)*
- [ ] Ambientes nomeados em `lazyapi.config.yml` + `.env.<nome>`; `--env-name staging`.
- [ ] Fluxos multi-operação em `flows/*.js`.

## 4. CLI para agents/CI (P0/P1) — Fase 3

- [ ] `--output json|junit|tap|pretty` (pretty só em TTY).
- [ ] Exit codes documentados: 0 ok · 1 teste falhou · 2 uso · 3 rede/spec.
- [ ] `lazyapi run`: `--tag`, `--op`, `--grep`, `--bail`, `--parallel N`, `--timeout`, `--retries`.
- [ ] Introspecção: `list ops --json`, `describe <op> --json`, `validate spec`.
- [ ] Request ad-hoc: `send --op createUser --param id=1 --header ... --body @file.json`.
- [ ] `lazyapi mcp` — servidor MCP stdio (`list_operations`, `describe_operation`, `send_request`, `run_tests`, `write_test`).
- [ ] `lazyapi mock` — mock server a partir dos examples do spec.
- [ ] Import/export: cURL ↔ request, Postman collection → spec + testes JS, HAR.
- [ ] HTTP: `--insecure`, CA/client cert, proxy, redirects, limite de body, `form-urlencoded`, `multipart`, upload.

## 5. UI para humanos — nível Postman (P1) — Fase 4

- [ ] Abas de resposta: Body (pretty/raw) · Headers · Cookies · Tests · Console · Timeline; status, tempo e tamanho.
- [ ] Syntax highlight (`chroma`) para JSON/XML/HTML/JS.
- [ ] Busca e filtro jq (`gojq`) na resposta.
- [ ] Seletor de ambiente na title bar.
- [ ] Command palette (`ctrl+k`).
- [ ] Histórico de requests com replay e diff.
- [ ] Abrir body/script no `$EDITOR`.
- [ ] Cancelar request em andamento + spinner.
- [ ] Confirmação antes de enviar métodos destrutivos pelo preview.
- [ ] Highlight/autocomplete de `{{env.X}}` (não resolvidas em vermelho).
- [ ] Gerar body a partir do schema/example.
- [ ] Diff contra example salvo + violação de schema visível.
- [ ] Runner view com progresso.
- [ ] Parar auto-save a cada `Update` (debounce / só em mudança real).
- [ ] Corrigir styling de param/header recriado a cada frame.

## 6. Qualidade de código (P1/P2)

- [ ] Centralizar mapeamento método→string.
- [ ] Extrair criação de parameter refs duplicada (`AddOperationToSpec`/`ApplyRequestToOperation`).
- [ ] Unificar parsing de `env.Load()`/`mergeDotenv()`.
- [ ] Rodar `gofmt -w` nos 14 arquivos com import order fora do padrão (pré-existente).
- [ ] Remover código morto: `modal.go`, `view.go`, `responses/`, `model/response.go`.
- [ ] Migrar `scrollable.go` para a API de mouse atual.
- [x] `sanitizePath`/temp path duplicados entre `cli` e `store` (CLI não lê mais temp files).
- [ ] Testes de TUI com `teatest` e e2e de CLI contra `httptest`.
- [ ] `go test -race` + coverage no CI.

## 7. Documentação para agents (P1)

- [ ] Reescrever AGENTS.md + `llms.txt` ("validar uma API em 5 comandos").
- [ ] `lazyapi help scripting`.
- [ ] Skill `.agents/skills/api-testing/`.
- [ ] `examples/` com spec + testes + flow + mock server.
