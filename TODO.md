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

## 2. Scripting JavaScript com import por URL (P0) — Fase 2 ✅ (faltam pre-request e validação de request)

- [x] Engine: `goja` (JS puro Go) + `esbuild` (lib Go) para transpilar ESM/TS → bundle.
- [x] `import x from "https://..."`:
  - [x] resolver esbuild (`onResolve`/`onLoad`) baixando via HTTPS
  - [x] cache em `~/.cache/lazyapi/modules` por hash
  - [x] `lazyapi.lock` com integrity sha256 por URL (runs reproduzíveis)
  - [x] `--offline` / `--reload`
  - [x] allowlist de hosts; rejeitar `http://`
  - [x] imports relativos (`./helpers.js`)
- [x] Sandbox: sem fs/processo, timeout via `vm.Interrupt`, limites configuráveis.
- [x] API: `test()`, `expect()` (`toBe`, `toEqual`, `toContain`, `toMatch`, `toHaveHeader`, `toMatchSchema`), `res.json()`, `vars`, `env`.
- [ ] Pre-request script além do post-response. *(adiado: definir formato — arquivo separado `x-lazyapi-pre-request` ou hook no mesmo arquivo)*
- [x] `lazyapi.send()` para encadear chamadas (login → criar → buscar → deletar).
- [x] `console.log` capturado no report.
- [x] Validação automática de contrato da **response** com `openapi3filter` (`--no-contract` desliga).
- [ ] Validação do **request** enviado contra o spec (params obrigatórios, body schema).
- [x] `lazyapi.d.ts` com tipos da API de script (`lazyapi types`).
- [x] Remover Lua (`.lua` referenciado gera erro explícito; salvar no TUI migra para `.js`).

## 3. Persistência pensada para agents (P0)

- [x] Scripts em arquivos versionados ao lado do spec, referenciados via `x-lazyapi-tests: tests/<op>.js`.
- [ ] Ambientes nomeados em `lazyapi.config.yml` + `.env.<nome>`; `--env-name staging`.
- [ ] Fluxos multi-operação em `flows/*.js`.

## 4. CLI para agents/CI (P0/P1) — Fase 3 ✅ (faltam mock, import/export e opções HTTP)

- [x] `--output json|junit|tap|pretty` + `--output-file` (pretty no stdout) + `--bodies failed|all|none`.
- [x] Exit codes documentados: 0 ok · 1 check falhou · 2 uso · 3 não executou (rede/spec/config).
- [x] `lazyapi run`: `--tag`, `--op`, `--grep`, `--bail`, `--parallel N`, `--timeout`, `--retries` (`smoke tests` virou alias).
- [x] Introspecção: `list ops --json`, `describe <op> --json` (com schemas referenciados), `validate spec` (inclui compilar scripts).
- [x] Request ad-hoc: `send --op createUser --param id=1 --header ... --body @file.json|@-` + `--output json` + `--dry-run`.
- [x] `lazyapi mcp --root` — servidor MCP stdio com 9 tools (`list_specs`, `list_operations`, `describe_operation`, `send_request`, `run_tests`, `read_test`, `write_test`, `validate_spec`, `script_api`), caminhos confinados ao root.
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

- [x] AGENTS.md atualizado + `docs/agents.md` ("validar uma API em 5 passos", setup MCP).
- [ ] `llms.txt` na raiz do site de docs.
- [x] `lazyapi types` (referência da API de script) + `docs/scripting.md`.
- [ ] Skill `.agents/skills/api-testing/`.
- [ ] `examples/` com spec + testes JS + flow + mock server (o spec atual aponta para `api.example.com`, não é executável).
