# TODO — LazyAPI para agents e humanos

Prioridades: **P0** bloqueia uso real · **P1** alto valor · **P2** polimento.

## Onde estamos (branch `feat/agent-ready`)

| Fase | Commit | Entregou |
|------|--------|----------|
| 1 | `ba62cc7` | Testes versionados (`x-lazyapi-tests`), erros sem derrubar o TUI, timeout HTTP, flags com stdlib, smoke seguro |
| 2 | `1ad8ead` | Engine JS/TS (goja + esbuild), imports `https://` com `lazyapi.lock`, `expect`, `lazyapi.send`, contrato automático; Lua removido |
| 3 | `a45e115` | `runner` estruturado, `run` com json/junit/tap, exit codes 0/1/2/3, `list`/`describe`/`validate`, servidor MCP |
| 4 | `8131547` | UI: tela de resposta com abas e checks, highlight, busca, cancelamento, confirmação, envs, `$EDITOR`, rodapé de atalhos |

**O que já funciona hoje:** um agent consegue descobrir, chamar, testar e validar uma API via MCP ou CLI JSON. Um humano consegue fazer o mesmo no TUI, vendo os mesmos checks.

**O que ainda impede o uso real:** a documentação de entrada está errada, não existe nada executável "de fábrica" e cenários com autenticação ou várias etapas dão trabalho. Por isso a próxima fase é a 5.

---

## Fase 5 — Fechar o ciclo de entrada (P0) ← próxima

Objetivo: alguém (pessoa ou agent) clona o repositório e em 2 minutos roda testes de verdade, sem servidor externo.

- [ ] **README reescrito.** Hoje diz que o smoke "não foi implementado" e não cita `run`, MCP nem JS. Incluir quickstart, `run --output json`, setup do MCP e link para `docs/scripting.md`.
- [ ] **`lazyapi mock <spec>`**: servidor HTTP que responde com os examples do spec (status, headers, body) e valida o request recebido. Dá um alvo local para os examples, para o CI e para os próprios testes.
- [ ] **`examples/` executável.** O spec atual aponta para `api.example.com`. Trocar por um spec com `tests/*.js`, `lazyapi.lock` e `.env.example` que passe contra o `lazyapi mock`, e adicionar esse cenário ao CI.
- [ ] **Validação do request contra o spec** (params obrigatórios, body schema) antes de enviar, como check `request`. Pega o erro do lado do cliente, que é o caso mais comum quando um agent monta a chamada.
- [ ] **`llms.txt`** no site de docs e uma **skill `.agents/skills/api-testing/`** com o fluxo list → describe → send → write_test → run.
- [ ] Corrigir no AGENTS.md o item de código morto (`modal.go`/`view.go` já não existem).

## Fase 6 — Cenários reais: auth e fluxos (P1)

Objetivo: validar APIs que exigem login e sequências de CRUD, que é onde testes por operação isolada não bastam.

- [ ] **Pre-request script**, por exemplo `x-lazyapi-pre-request` ou `export function before(req)` no mesmo arquivo. Serve para gerar token, assinar o request ou montar o body. Precisa decidir o formato primeiro.
- [ ] **Fluxos `flows/*.js`**: cenários com várias operações (`login → create → get → delete`) usando `lazyapi.send` e `vars`. Comando `lazyapi run --flow <file>`, ferramenta MCP `run_flow`, e reports no mesmo formato do `run`.
- [ ] **Ambientes nomeados** em `lazyapi.config.yml` (`environments: {local, staging}` → server + arquivo env) com `--env-name staging`. O `ctrl+v` do TUI passa a usar essa lista.
- [ ] **OAuth2 client-credentials/password automático**: buscar e cachear o token pelo `tokenUrl` do security scheme.

## Fase 7 — Completude HTTP e migração (P1)

Objetivo: cobrir as APIs que hoje não dá para chamar e facilitar a vinda de quem usa Postman ou cURL.

- [ ] `--insecure`, CA e certificado de cliente, proxy, política de redirects, limite de tamanho de body.
- [ ] Bodies `application/x-www-form-urlencoded`, `multipart/form-data` e upload de arquivo (CLI, MCP e TUI).
- [ ] **cURL ↔ request**: `lazyapi import curl`, e no TUI um "copy as cURL" (`ctrl+y` no request).
- [ ] **Import de Postman collection e HAR** → operações no spec + testes JS (convertendo `pm.test` / `pm.expect` para `test` / `expect`).
- [ ] Abas **Cookies** e **Timeline** (DNS/TLS/TTFB via `httptrace`) na tela de resposta.

## Fase 8 — Polimento da UI (P2)

- [ ] Paleta de comandos (`ctrl+k`): buscar operação, trocar env, rodar testes, copiar como cURL.
- [ ] Tela do runner no TUI: rodar uma tag ou a spec inteira com progresso e lista navegável de falhas.
- [ ] Histórico de requests com replay, e diff entre respostas ou contra o example salvo.
- [ ] Filtro jq (`gojq`) na resposta.
- [ ] Destacar `{{env.X}}`/`{{var.X}}` não resolvidas, com autocomplete.
- [ ] Gerar body sintético a partir do schema quando não há example.

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
