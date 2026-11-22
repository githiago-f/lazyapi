# TODO — LazyAPI para agents e humanos

Prioridades: **P0** bloqueia uso real · **P1** alto valor · **P2** polimento.

## Onde estamos (branch `feat/agent-ready`)

| Fase | Commit | Entregou |
|------|--------|----------|
| 1 | `ba62cc7` | Testes versionados (`x-lazyapi-tests`), erros sem derrubar o TUI, timeout HTTP, flags com stdlib, smoke seguro |
| 2 | `1ad8ead` | Engine JS/TS (goja + esbuild), imports `https://` com `lazyapi.lock`, `expect`, `lazyapi.send`, contrato automático; Lua removido |
| 3 | `a45e115` | `runner` estruturado, `run` com json/junit/tap, exit codes 0/1/2/3, `list`/`describe`/`validate`, servidor MCP |
| 4 | `8131547` | UI: tela de resposta com abas e checks, highlight, busca, cancelamento, confirmação, envs, `$EDITOR`, rodapé de atalhos |
| 5 | `9cc29dd` | README, `lazyapi mock`, exemplos executáveis no CI, check `request`, `llms.txt`, skill de testes de API |
| 6 | `f404bf7` | `before(req)`, fluxos, ordem do `run`, ambientes nomeados, credenciais e OAuth2 automático |
| 7 | (este) | Opções HTTP/TLS/proxy, form e multipart, timing e cookies, import de curl/Postman/HAR, export curl, camada `pm` |

**O que já funciona hoje:** um agent consegue descobrir, chamar, testar e validar uma API via MCP ou CLI JSON. Um humano consegue fazer o mesmo no TUI, vendo os mesmos checks.

**O que ainda falta:** com as fases 1–7 o fluxo para agents, CI e migração está completo. O que sobra é polimento da UI (fase 8) e qualidade de código.

---

## Fase 5 — Fechar o ciclo de entrada (P0) ✅

Objetivo: alguém (pessoa ou agent) clona o repositório e em 2 minutos roda testes de verdade, sem servidor externo.

- [x] **README reescrito**: quickstart com mock, tabela de checks, testes JS, CLI, MCP, TUI.
- [x] **`lazyapi mock <spec>`**: responde com os examples (ou `SchemaSample`), escolhe a resposta com `Prefer: code=`/`example=`, valida o request (422), aceita prefixo de `servers`, CORS preflight.
- [x] **`examples/` executável**: API de usuários com 6 operações, `tests/*.js` (helpers, encadeamento com login, teste de 404 via `Prefer`) e `.env.example`. Roda no `go test` e num job do CI contra o mock, com report JUnit.
- [x] **Check `request`**: o request enviado é validado contra o spec (params, body) no CLI, MCP e TUI; desligado por `--no-contract`.
- [x] **`llms.txt`** (`docs/public/`) e skill **`.agents/skills/api-testing/`**.
- [x] AGENTS.md: mock, check `request`, exemplos como teste; item de código morto removido.
- [ ] `lazyapi.lock` de exemplo: os exemplos não usam import por URL, para o CI não depender de rede. Avaliar um exemplo opcional com cache.

## Fase 6 — Cenários reais: auth e fluxos (P1) ✅

Objetivo: validar APIs que exigem login e sequências de CRUD, que é onde testes por operação isolada não bastam.

- [x] **`export function before(req)`** no arquivo de testes: muda headers, query, params e body antes do envio. Pode usar `lazyapi.send` e `vars`. Se lança erro, o request não é enviado (check `script`).
- [x] **Fluxos `flows/*.js`**: `run --flow <file>` (só o fluxo) e `run --flows` (operações e depois todos os fluxos). Cada `lazyapi.send` vira um passo com checks de request, status e contract. MCP `run_flow`/`write_flow`, e `include_flows` no `run_tests`.
- [x] **Ordem do `run`**: leituras, POST, PUT/PATCH e por último DELETE (antes o DELETE rodava antes do GET).
- [x] **Ambientes nomeados** em `lazyapi.config.yml` (server, env file, vars, credenciais por security scheme) com `--env-name`, `$LAZYAPI_ENV`, `env_name` no MCP e `ctrl+v` no TUI. O ambiente aparece no report.
- [x] **Credenciais**: bearer, basic, API key (header, query, cookie) e OAuth2 (`accessToken` fixo, ou client credentials/password com token automático e cache até expirar). Credenciais vazias não são mais enviadas (antes saía `Authorization: Bearer ` vazio).
- [x] Exemplos: `before()` com login no `getUser.js`, fluxo `userLifecycle.js`, `lazyapi.config.yml`; tudo roda no `go test` e no CI.
- [x] Fluxos no TUI (paleta e tela do runner, Fase 8).
- [ ] `after(res)` para limpeza (ex.: apagar o que um teste criou).

## Fase 7 — Completude HTTP e migração (P1) ✅

Objetivo: cobrir as APIs que hoje não dá para chamar e facilitar a vinda de quem usa Postman ou cURL.

- [x] `--insecure`, `--ca-cert`, `--client-cert`/`--client-key` (mTLS), `--proxy`, `--no-redirects`/`--max-redirects`, `--max-body`; `http:` por ambiente no config; `insecure` no MCP.
- [x] Bodies `form-urlencoded` e `multipart/form-data` (objeto JSON → campos), `--form`/`--file`. Arquivos só via `--file`, nunca a partir do body. O editor do TUI mantém o tipo do body.
- [x] **cURL**: `send --output curl`, `lazyapi import curl`, `alt+c` no TUI, MCP `import_curl`/`export_curl`.
- [x] **Import de Postman** (v2.0/v2.1: pastas → tags, auth → security schemes, variáveis → `.env.postman`, scripts mantidos com a camada `pm`) e **HAR** (só tráfego de API, um exemplo por status).
- [x] Abas **Cookies** e **Timeline** (DNS/TCP/TLS/TTFB/download) na tela de resposta; `timing` e `cookies` no report.
- [ ] `pm.sendRequest` (callback) na camada `pm`.
- [ ] Import de environments do Postman (`*.postman_environment.json`) para `lazyapi.config.yml`.
- [ ] Editor de campos de form no TUI (hoje o body form é editado como objeto JSON).

## Fase 8 — Polimento da UI (P2) ✅

- [x] Paleta de comandos (`ctrl+k`, busca fuzzy): abrir operação, rodar spec/tag/fluxos, trocar env, histórico, copiar como cURL.
- [x] Tela do runner (`R` na lista, ou a paleta): resultados em streaming com barra de progresso, cursor na primeira falha, `F` só falhas, `e` abre no editor, `R` roda de novo; detalhes reutilizam a tela de resposta. Escritas e fluxos pedem y/N.
- [x] Fluxos no TUI (paleta → runner; o detalhe lista os passos).
- [x] Histórico por spec (`H`), sem credenciais em disco: replay no editor, diff com a resposta anterior (`d`) e com o example do spec (`x`), JSON comparado com chaves ordenadas.
- [x] Filtro jq (`gojq`): `f` na resposta, `send --jq`, `jq` no MCP `send_request`.
- [x] `{{env.X}}`/`{{var.X}}` sem valor listadas no rodapé do editor; autocomplete inline após `{{` (`→` aceita).
- [x] Body sintético a partir do schema no editor quando não há example (`store.SampleBody`; `run` nunca inventa body).
- [ ] Destacar as referências dentro do próprio campo (cor no texto). Hoje os componentes de input do bubbles não aceitam estilo por trecho.
- [ ] Limpar o histórico pelo TUI (hoje: apagar o arquivo ou `LAZYAPI_NO_HISTORY`).
- [ ] Rodar um único passo de um fluxo / ver o request de cada passo no detalhe.

## Próximas ideias

- `after(res)` para limpeza; `pm.sendRequest`; import de environments do Postman; editor de campos de form.
- Testes de TUI end-to-end com `teatest` (hoje: testes por componente + validação manual no tmux).

## Contínuo — qualidade de código

Fazer junto com as fases, quando tocar no código em questão:

- [ ] **Tirar a dependência `store` → `app/pane/requests`** (mover o tipo de dados de `RequestItem` para `model`). Hoje ela força o preview a ficar sem o componente de resposta e cria risco de import cycle.
- [ ] Testes de TUI com `teatest` (os fluxos validados manualmente no tmux: envio, confirmação, busca, env).
- [ ] Coverage no CI (o `-race` já roda).
- [ ] `gofmt -w` nos 9 arquivos antigos fora do padrão (num commit só de formatação).
- [ ] Estilo de param/header recriado a cada frame (o foco fica frágil).
- [ ] Criação duplicada de parameter refs (`AddOperationToSpec`/`ApplyRequestToOperation`).
- [ ] Parsing duplicado entre `env.Load()` e `mergeDotenv()`.
- [ ] Mapeamento método→string ainda em 3 lugares.
- [ ] `scrollable.go` usando a API de mouse depreciada.
- [ ] Highlight de busca: hoje a busca mostra texto sem cores por causa de um bug no viewport do bubbles. Reportar ou corrigir upstream.

---

<details>
<summary>Itens concluídos nas fases 1–4 (histórico)</summary>

- Fase 1: testes versionados · notificações de erro no lugar de quit/panic · timeout + context · Basic auth com templates · resultados ordenados com mensagem · smoke com `--methods/--tag/--op/--dry-run` e exemplos do spec · status validado contra `responses` · flags stdlib · `os.Remove` tolerante · AGENTS.md.
- Fase 2: goja + esbuild · imports https com cache, lockfile, `--offline/--reload/--frozen-lockfile/--allow-import` · imports relativos confinados · sandbox com timeout · `test/expect` (18 matchers) · `lazyapi.send` · console · contrato da response · `lazyapi types` · Lua removido.
- Fase 3: `run` com `--grep/--bail/--parallel/--retries` · `--output pretty|json|junit|tap` · `--output-file` · `--bodies` · headers sensíveis ocultos · exit codes · `send --op/--body/--dry-run` · `list/describe/validate` · MCP com 9 ferramentas.
- Fase 4: tela de resposta (status, tempo, tamanho, Body/Headers/Checks/Console, raw, busca) · highlight chroma · TUI envia pelo runner · `esc` cancela · confirmação de métodos destrutivos · `ctrl+v` env · `alt+enter`/F5 · `$EDITOR` · rodapé de atalhos · body pré-preenchido · bugs (script não carregava, Content-Type, auto-save, layout).

</details>
