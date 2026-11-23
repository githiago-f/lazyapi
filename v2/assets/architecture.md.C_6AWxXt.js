import{_ as s,o as n,c as e,a3 as t}from"./chunks/framework.BJ2rlPXv.js";const h=JSON.parse('{"title":"Architecture","description":"","frontmatter":{},"headers":[],"relativePath":"architecture.md","filePath":"architecture.md","lastUpdated":1795454575000}'),p={name:"architecture.md"};function i(l,a,o,c,r,d){return n(),e("div",null,[...a[0]||(a[0]=[t(`<h1 id="architecture" tabindex="-1">Architecture <a class="header-anchor" href="#architecture" aria-label="Permalink to &quot;Architecture&quot;">​</a></h1><p>LazyAPI is structured as a Go monolith with a clean internal package layout. All core logic is synchronous; TUI interactions are wrapped in <code>tea.Cmd</code> closures for the bubbletea framework.</p><h2 id="package-layout" tabindex="-1">Package layout <a class="header-anchor" href="#package-layout" aria-label="Permalink to &quot;Package layout&quot;">​</a></h2><div class="language- vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang"></span><pre class="shiki shiki-themes catppuccin-latte catppuccin-mocha vp-code" tabindex="0"><code><span class="line"><span>cmd/</span></span>
<span class="line"><span>  lazyapi/main.go          # Entry point — dispatches to TUI or CLI</span></span>
<span class="line"><span></span></span>
<span class="line"><span>internal/</span></span>
<span class="line"><span>  app/                     # bubbletea model + views</span></span>
<span class="line"><span>    tui.go                 #   Main Tui struct, Init/Update/View</span></span>
<span class="line"><span>    pane/</span></span>
<span class="line"><span>      editor/              #   Request editor (method, URL, headers, body, params, tests, auth)</span></span>
<span class="line"><span>      requests/            #   Request list (tree with GroupByResource, CRUD messages)</span></span>
<span class="line"><span>      responses/           #   Response preview</span></span>
<span class="line"><span>  cli/                     # CLI commands (create, remove, add, send, smoke)</span></span>
<span class="line"><span>  components/              # Reusable UI components (button, field, modal, tabs, selector, etc.)</span></span>
<span class="line"><span>  config/                  # Catppuccin Mocha color palette + keybindings + page constants</span></span>
<span class="line"><span>  env/                     # Environment variable loading/resolution</span></span>
<span class="line"><span>  inmath/                  # Math utilities (circular field cycling)</span></span>
<span class="line"><span>  model/                   # Core types: Request, Method, Body, Response, OpenAPIRef, About</span></span>
<span class="line"><span>  store/                   # File system + OpenAPI spec operations</span></span></code></pre></div><h2 id="data-flow" tabindex="-1">Data flow <a class="header-anchor" href="#data-flow" aria-label="Permalink to &quot;Data flow&quot;">​</a></h2><div class="language-text vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang">text</span><pre class="shiki shiki-themes catppuccin-latte catppuccin-mocha vp-code" tabindex="0"><code><span class="line"><span>OpenAPI YAML file</span></span>
<span class="line"><span>       │</span></span>
<span class="line"><span>       ▼</span></span>
<span class="line"><span>   store.ParseSpec()      ─── openapi3.Loader</span></span>
<span class="line"><span>       │</span></span>
<span class="line"><span>       ▼</span></span>
<span class="line"><span>   store.ListOperations()  ─── sorted by URI</span></span>
<span class="line"><span>       │</span></span>
<span class="line"><span>       ▼</span></span>
<span class="line"><span>   requests.RequestItem    ─── wraps Method, URI, About, FileName, OpenAPIRef</span></span>
<span class="line"><span>       │</span></span>
<span class="line"><span>       ▼</span></span>
<span class="line"><span>   editor.RequestPane      ─── 6 tabs (Documentation, Params, Authorize, Header, Body, Tests)</span></span>
<span class="line"><span>       │</span></span>
<span class="line"><span>       ▼</span></span>
<span class="line"><span>   model.Request.Send()    ─── http.DefaultClient.Do</span></span></code></pre></div><h2 id="key-principles" tabindex="-1">Key principles <a class="header-anchor" href="#key-principles" aria-label="Permalink to &quot;Key principles&quot;">​</a></h2><ul><li><strong>OpenAPI is the source of truth</strong> — every <code>.yml</code>/<code>.yaml</code> file is parsed as OpenAPI 3.x</li><li><strong>No database</strong> — everything is file-based</li><li><strong>Git-native workspace</strong> — everything for a spec lives in <code>.lazyapi/</code> next to it (config, envs, tests, flows, data, lockfile; secrets and local state git-ignored). See <a href="/v2/environments">Environments</a></li><li><strong>Session state in <code>.lazyapi/.local/</code></strong> — unsaved edits, drafts and history (git-ignored)</li><li><strong>Auth secrets never persist</strong> — security scheme definitions are saved, but runtime values (passwords, tokens) are session-only</li></ul><h2 id="local-state" tabindex="-1">Local state <a class="header-anchor" href="#local-state" aria-label="Permalink to &quot;Local state&quot;">​</a></h2><div class="language- vp-adaptive-theme"><button title="Copy Code" class="copy"></button><span class="lang"></span><pre class="shiki shiki-themes catppuccin-latte catppuccin-mocha vp-code" tabindex="0"><code><span class="line"><span>&lt;spec dir&gt;/.lazyapi/.local/drafts/</span></span>
<span class="line"><span>  tmp.&lt;METHOD&gt;.&lt;sanitized-path&gt;   # OpenAPI-ref operations</span></span>
<span class="line"><span>  draft.new.&lt;N&gt;                    # New unsaved requests</span></span></code></pre></div>`,10)])])}const m=s(p,[["render",i]]);export{h as __pageData,m as default};
