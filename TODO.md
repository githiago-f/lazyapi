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

**O que ainda falta:** Fase 17 (requests salvos como código e `lazyapi.run`) e o release v1.4.0.

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
- [x] `after(res)` para limpeza (Fase 9).

## Fase 7 — Completude HTTP e migração (P1) ✅

Objetivo: cobrir as APIs que hoje não dá para chamar e facilitar a vinda de quem usa Postman ou cURL.

- [x] `--insecure`, `--ca-cert`, `--client-cert`/`--client-key` (mTLS), `--proxy`, `--no-redirects`/`--max-redirects`, `--max-body`; `http:` por ambiente no config; `insecure` no MCP.
- [x] Bodies `form-urlencoded` e `multipart/form-data` (objeto JSON → campos), `--form`/`--file`. Arquivos só via `--file`, nunca a partir do body. O editor do TUI mantém o tipo do body.
- [x] **cURL**: `send --output curl`, `lazyapi import curl`, `alt+c` no TUI, MCP `import_curl`/`export_curl`.
- [x] **Import de Postman** (v2.0/v2.1: pastas → tags, auth → security schemes, variáveis → `.env.postman`, scripts mantidos com a camada `pm`) e **HAR** (só tráfego de API, um exemplo por status).
- [x] Abas **Cookies** e **Timeline** (DNS/TCP/TLS/TTFB/download) na tela de resposta; `timing` e `cookies` no report.
- [x] `pm.sendRequest` (Fase 10).
- [x] Import de environments do Postman (Fase 10).
- [x] Editor de campos de form no TUI (Fase 10).

## Fase 8 — Polimento da UI (P2) ✅

- [x] Paleta de comandos (`ctrl+k`, busca fuzzy): abrir operação, rodar spec/tag/fluxos, trocar env, histórico, copiar como cURL.
- [x] Tela do runner (`R` na lista, ou a paleta): resultados em streaming com barra de progresso, cursor na primeira falha, `F` só falhas, `e` abre no editor, `R` roda de novo; detalhes reutilizam a tela de resposta. Escritas e fluxos pedem y/N.
- [x] Fluxos no TUI (paleta → runner; o detalhe lista os passos).
- [x] Histórico por spec (`H`), sem credenciais em disco: replay no editor, diff com a resposta anterior (`d`) e com o example do spec (`x`), JSON comparado com chaves ordenadas.
- [x] Filtro jq (`gojq`): `f` na resposta, `send --jq`, `jq` no MCP `send_request`.
- [x] `{{env.X}}`/`{{var.X}}` sem valor listadas no rodapé do editor; autocomplete inline após `{{` (`→` aceita).
- [x] Body sintético a partir do schema no editor quando não há example (`store.SampleBody`; `run` nunca inventa body).
- [x] Destacar as referências dentro do próprio campo: campos e editores próprios com cores (Fase 13).
- [x] Limpar o histórico pelo TUI (`X` com confirmação).
- [x] Ver o request e a resposta de cada passo de um fluxo (runner: `enter` expande; JSON: `steps[].request/response`).

## Fase 9 — Testes repetíveis e TUI testado ✅

- [x] `export function after(res)`: roda depois dos testes, mesmo quando falham ou o script lança erro, com a API completa (`lazyapi.send` para apagar o que o teste criou). Vira o check `after() ran without errors`. Em fluxos, os requests do `after` aparecem como passos. O exemplo `createUser` apaga o usuário que cria.
- [x] Testes end-to-end do TUI com `teatest` (`internal/app/tui_test.go`): o programa inteiro contra um `httptest`, cobrindo confirmação de envio destrutivo, filtro + envio, paleta → runner, run com escritas (y/N), `R` + `esc`, histórico + diff + replay, editor + `alt+enter`, troca de env nomeado e `esc` cancelando um request.

## Fase 10 — Migração do Postman completa ✅

- [x] `lazyapi import postman-env <env.json>`: valores em `.env.<nome>` (0600), ambiente novo no `lazyapi.config.yml` (criado se não existir, comentários preservados, nada sobrescrito), `server` tirado de `baseUrl`/`host`/…; `import postman --environment` resolve hosts `{{baseUrl}}` que só o environment define.
- [x] Credenciais que são só referência (`Bearer {{token}}`, basic com variáveis, API key) vão para `auth:` do ambiente: o request importado já autentica. Segredos literais continuam fora do spec e do config.
- [x] `pm.sendRequest(url | {url, method, header, body}, callback)` e `lazyapi.send("METHOD https://…")`: a URL é mapeada para a operação do spec (path params e query vêm da URL) e recebe os mesmos checks; URLs fora do spec são rejeitadas com dica.
- [x] Aba Body com **campos de form** (urlencoded/multipart): linhas nome/valor, `ctrl+g` adiciona, nomes repetidos viram array, ordem preservada.
- [x] `.env` aceita valores entre aspas duplas com escapes.

## Próximas ideias

- [x] Runs com data files: `run --data rows.csv|json` (uma iteração por linha, linha no env, `lazyapi.iteration`, `pm.iterationData`, `[#n]` nos relatórios), MCP `data_file`.
- [~] ~~`formdata` (arquivos) no `pm.sendRequest`~~: o global `pm` foi removido (Fase 14).
- [x] Runner no TUI com data file: a paleta lista CSV e JSON (listas) ao lado do spec e em `data/`.

## Fase 13 — Editores de verdade

- [x] Editor de código para Body, Tests e Description: syntax highlight (chroma, Catppuccin), números de linha, pares automáticos, auto-indent, `tab`/`shift+tab`, `alt+f` formata JSON, validação de JSON ao vivo (erro na linha), desfazer/refazer, colar com várias linhas. Buffer próprio (o `textarea` redesenhava tudo a cada tecla e cortava conteúdo em 99 linhas).
- [x] Campos de uma linha com cores: templates (resolvido/faltando), host, `{params}`, query.
- [x] Menu de autocomplete: variáveis após `{{` ou `$nome` (valores mascarados), API dos scripts no editor de Tests (`response.`, `expect(…).`, `lazyapi.`, `pm.`…).
- [x] Visual: bordas arredondadas e apagadas, destaque só no foco, aba ativa com sublinhado grosso.
- [x] Assinatura da chamada sob o cursor e erros de sintaxe JS/TS ao vivo (esbuild) no editor de Tests.
- [x] Seleção (shift+movimentos, alt+a), copiar (ctrl+c só com seleção), recortar (alt+x), colar (alt+v); digitar substitui, abrir aspas/parênteses envolve, tab indenta as linhas.

## Fase 14 — Sem `pm`: scripts do Postman traduzidos ✅

- [x] O global `pm` (camada de compatibilidade) foi removido do runtime, das completions e da doc.
- [x] `lazyapi import postman` traduz os scripts para a API nativa: `pm.test` → `test`, cadeias Chai (`equal`, `eql`, `deep.equal`, `include`, `property().that.equals`, `an('array')`, `above/below`, `lengthOf`, `empty`, `oneOf`, `not`…) → matchers do `expect`, `pm.response.*` → `response`, `pm.environment`/`iterationData` → `env`, escopos de variáveis → `vars`, headers do pre-request → `req.headers`.
- [x] O que não traduz vira `// TODO(postman):` (contado no cabeçalho); se comentar quebra a sintaxe (checado com esbuild), o script original fica inteiro comentado.

## Fase 15 — Dados falsos e helpers: `{{$nome}}` ✅

- [x] `internal/fake` (gofakeit) com os nomes do Postman: `{{$guid}}`, `{{$timestamp}}`, `{{$randomEmail}}`, `{{$randomFirstName}}`… (~90). `lazyapi fake [nome] [--json]` lista ou gera.
- [x] Valor novo a cada ocorrência, materializado uma vez por envio (antes do `before(req)`): o report, o `request` dos testes e o que vai na rede batem. Nome desconhecido = operação falha sem enviar.
- [x] Helpers: toda função exportada pelo script do request (`export function randomSku()`) vira `{{$randomSku}}` e sobrescreve o builtin de mesmo nome.
- [x] Global `faker` nos scripts (`faker.email()`, `faker.uuid()`, `int`, `float`, `pick`, `get`), tipado no `lazyapi.d.ts` (teste garante que bate com a tabela).
- [x] TUI: `{{$` completa builtins e helpers do script com descrição, cores de resolvido/faltando, nomes desconhecidos no rodapé.
- [ ] `--seed` / `LAZYAPI_SEED` para valores reproduzíveis (`fake.Seed` já existe).
- [ ] Helpers com argumentos: `{{$randomInt(1, 10)}}`.

## Fase 16 — Git-native: `.lazyapi/` ao lado do spec ✅

- [x] `internal/workspace`: `.lazyapi/config.yml` (ambientes), `envs/<nome>.env` (commitado), `secrets/<nome>.env` (0600, gitignored), `tests/`, `flows/`, `data/`, `lazyapi.lock`, `.local/` (gitignored); `.gitignore` escrito automaticamente, nunca sobrescrito.
- [x] Ambiente lê `envs/<nome>.env` e depois `secrets/<nome>.env` por convenção (`env:` e `--env` por último); `env.Store` mescla vários arquivos.
- [x] Import do Postman separa variáveis comuns (envs/) de segredos (secrets/, por tipo `secret` ou nome sensível).
- [x] Drafts e edições não salvas em `.lazyapi/.local/drafts`, histórico em `.lazyapi/.local/history.jsonl`.
- [x] Sem compatibilidade com o layout antigo (`lazyapi.config.yml`, `tests/`, `flows/` soltos). `examples/` migrado.
- [ ] TUI: editar variáveis do ambiente ativo (gravar em `envs/` ou `secrets/` conforme o nome).
- [ ] Vários specs no mesmo diretório compartilham `.lazyapi/`: avaliar `tests/<spec>/` se virar problema.

## Fase 17 — Tudo scriptável: requests salvos como código (próxima)

Objetivo: chamar um request já definido (com body, headers e query salvos) de qualquer script, sem montar um `send` na mão.

- [ ] Salvar no TUI grava os valores do request (body, headers, query, path params) em `.lazyapi/requests/<operationId>.yml` (commitado). Hoje eles só existem em `.local/drafts` e o spec guarda apenas a definição.
- [ ] `lazyapi.send("createUser")` usa o request salvo (e depois os examples do spec) e os helpers `{{$…}}` do script dele.
- [ ] `lazyapi.run("createUser", opts?)`: roda o request como no `run` — `before()`, envio, checks de request/status/contract, testes e `after()` dele — e devolve a resposta; os resultados aparecem como passos.
- [ ] `lazyapi.requests.createUser(opts)` (proxy por operationId) + `lazyapi types --spec` gerando tipos por operação; completions de operationId em `lazyapi.send("`/`run("`.
- [ ] CLI `run`/MCP usam os requests salvos como fonte dos valores.

## Release

- [ ] **v1.4.0**: fases 13 (seleção, diagnósticos), dados no runner, 14, 15, 16 (breaking: `pm` removido, layout `.lazyapi/`). Tag depois do merge na `main`; o `cd.yml` publica os binários e o `docs.yml` a doc em https://lazyapi.ia.br (atual em `/`, arquivo por major em `/v1/`).

## Fase 11 / Contínuo — qualidade de código

Fazer junto com as fases, quando tocar no código em questão:

- [x] Tirar a dependência `store` → `app/pane/requests` (`model.RequestItem`; `internal/archtest` impede que volte).
- [x] Testes de TUI com `teatest` (Fase 9).
- [x] Coverage no CI (resumo por pacote, piso de 55%; hoje ~61%) e checagem de `gofmt`.
- [x] `gofmt -w` nos arquivos antigos.
- [x] `kvGrid` único para headers, params e campos de form: o View não altera mais o estado; linhas ordenadas por nome.
- [x] `syncParameters` único (ordem estável ao salvar; `Accept`/`Content-Type`/`Authorization` não viram parâmetros).
- [x] `env.Load` e `Store` usam o mesmo parser.
- [x] Uma tabela de métodos (`ParseMethod`); método desconhecido no YAML é erro.
- [x] `scrollable.go` (já usava a API nova; clamp unificado e testes).
- [x] Highlight de busca com cores: corrigido no bubbles v2.2 (atualizado junto com bubbletea v2.1 e lipgloss v2.1); a busca roda no texto sem cores e os offsets são convertidos.

---

<details>
<summary>Itens concluídos nas fases 1–4 (histórico)</summary>

- Fase 1: testes versionados · notificações de erro no lugar de quit/panic · timeout + context · Basic auth com templates · resultados ordenados com mensagem · smoke com `--methods/--tag/--op/--dry-run` e exemplos do spec · status validado contra `responses` · flags stdlib · `os.Remove` tolerante · AGENTS.md.
- Fase 2: goja + esbuild · imports https com cache, lockfile, `--offline/--reload/--frozen-lockfile/--allow-import` · imports relativos confinados · sandbox com timeout · `test/expect` (18 matchers) · `lazyapi.send` · console · contrato da response · `lazyapi types` · Lua removido.
- Fase 3: `run` com `--grep/--bail/--parallel/--retries` · `--output pretty|json|junit|tap` · `--output-file` · `--bodies` · headers sensíveis ocultos · exit codes · `send --op/--body/--dry-run` · `list/describe/validate` · MCP com 9 ferramentas.
- Fase 4: tela de resposta (status, tempo, tamanho, Body/Headers/Checks/Console, raw, busca) · highlight chroma · TUI envia pelo runner · `esc` cancela · confirmação de métodos destrutivos · `ctrl+v` env · `alt+enter`/F5 · `$EDITOR` · rodapé de atalhos · body pré-preenchido · bugs (script não carregava, Content-Type, auto-save, layout).

</details>
