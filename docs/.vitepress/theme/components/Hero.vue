<script setup lang="ts">
// Home hero: brand, tagline, actions and a live mock of the TUI request list
// (examples/openapi.yml). Focus the mock and use j/k, s and tab like in lazyapi.
import { computed, ref } from "vue"
import { withBase } from "vitepress"

type Op = {
  tag: string
  method: string
  path: string
  summary: string
  status: number
  text: string
  ms: number
  body: string
}

const ops: Op[] = [
  {
    tag: "auth", method: "POST", path: "/auth/login", summary: "Exchange credentials for a token",
    status: 200, text: "OK", ms: 3,
    body: '{\n  "token": "eyJhbGciOiJIUzI1NiJ9…",\n  "expiresIn": 3600\n}',
  },
  {
    tag: "system", method: "GET", path: "/health", summary: "Service health",
    status: 200, text: "OK", ms: 1,
    body: '{\n  "status": "ok",\n  "version": "1.4.0"\n}',
  },
  {
    tag: "users", method: "GET", path: "/users", summary: "List users",
    status: 200, text: "OK", ms: 2,
    body: '[\n  { "id": 1, "name": "Ana", "email": "ana@example.com" },\n  { "id": 2, "name": "Bruno", "email": "bruno@example.com" }\n]',
  },
  {
    tag: "users", method: "POST", path: "/users", summary: "Create a user",
    status: 201, text: "Created", ms: 4,
    body: '{\n  "id": 3,\n  "name": "Carla",\n  "email": "carla@example.com"\n}',
  },
  {
    tag: "users", method: "DELETE", path: "/users/{id}", summary: "Delete a user",
    status: 204, text: "No Content", ms: 2,
    body: "",
  },
  {
    tag: "users", method: "GET", path: "/users/{id}", summary: "Get a user",
    status: 200, text: "OK", ms: 1,
    body: '{\n  "id": 1,\n  "name": "Ana",\n  "email": "ana@example.com"\n}',
  },
]

const groups = computed(() => {
  const out: { tag: string; items: { op: Op; i: number }[] }[] = []
  ops.forEach((op, i) => {
    let g = out.find((x) => x.tag === op.tag)
    if (!g) out.push((g = { tag: op.tag, items: [] }))
    g.items.push({ op, i })
  })
  return out
})

const sel = ref(5)
const sent = ref<number | null>(5)
const sending = ref(false)
const tab = ref(0)
const tabs = ["Body", "Headers", "Checks", "Timeline"]
const current = computed(() => ops[sel.value])
const response = computed(() => (sent.value === sel.value ? ops[sel.value] : null))

const install = "go install github.com/githiago-f/lazyapi/cmd/lazyapi@latest"
const copied = ref(false)

function copy() {
  navigator.clipboard?.writeText(install)
  copied.value = true
  setTimeout(() => (copied.value = false), 1500)
}

function select(i: number) {
  sel.value = i
}

function send() {
  sending.value = true
  sent.value = null
  const i = sel.value
  setTimeout(() => {
    sending.value = false
    sent.value = i
  }, 260)
}

function onKey(e: KeyboardEvent) {
  let handled = true
  switch (e.key) {
    case "j":
    case "ArrowDown":
      sel.value = (sel.value + 1) % ops.length
      break
    case "k":
    case "ArrowUp":
      sel.value = (sel.value - 1 + ops.length) % ops.length
      break
    case "s":
    case "Enter":
      send()
      break
    case "]":
    case "Tab":
      if (e.shiftKey) tab.value = (tab.value - 1 + tabs.length) % tabs.length
      else tab.value = (tab.value + 1) % tabs.length
      break
    case "[":
      tab.value = (tab.value - 1 + tabs.length) % tabs.length
      break
    default:
      handled = false
  }
  if (handled) {
    e.preventDefault()
    e.stopPropagation()
  }
}

function methodClass(m: string) {
  return "t-method " + m.toLowerCase()
}

function statusClass(s: number) {
  if (s >= 500) return "st err"
  if (s >= 400) return "st warn"
  return "st ok"
}

// A tiny JSON highlighter (keys, strings, numbers) for the mock.
function highlight(json: string): string {
  const esc = json.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
  return esc.replace(
    /("(?:[^"\\]|\\.)*")(\s*:)?|\b(-?\d+(?:\.\d+)?)\b|\b(true|false|null)\b/g,
    (m, str, colon, num, lit) => {
      if (str && colon) return `<span class="j-key">${str}</span>${colon}`
      if (str) return `<span class="j-str">${str}</span>`
      if (num) return `<span class="j-num">${num}</span>`
      if (lit) return `<span class="j-lit">${lit}</span>`
      return m
    },
  )
}

function size(op: Op) {
  return op.body ? `${new TextEncoder().encode(op.body).length} B` : "0 B"
}
</script>

<template>
  <section class="t-hero">
    <div class="intro">
      <p class="eyebrow"><span class="dot" /> OpenAPI-driven · terminal-first · agent-ready</p>
      <h1 class="brand"><span class="p">&gt;_</span> lazyapi<span class="cursor" aria-hidden="true">█</span></h1>
      <p class="tagline">
        Explore, test and automate any API <strong>straight from its OpenAPI spec</strong>.
        A terminal UI for humans, a CLI for CI, an MCP server for agents.
      </p>
      <p class="sub">No collections to keep in sync: every response is checked against the spec, and your tests are JavaScript files committed next to it.</p>

      <div class="actions">
        <a class="t-btn primary" :href="withBase('/getting-started')">▶ Get started</a>
        <a class="t-btn" :href="withBase('/introduction')">What is LazyAPI?</a>
        <a class="t-btn" href="https://github.com/githiago-f/lazyapi">GitHub ↗</a>
      </div>

      <button class="install" type="button" :title="copied ? 'Copied' : 'Copy'" @click="copy">
        <span class="ps">$</span>
        <code>{{ install }}</code>
        <span class="cp">{{ copied ? "✓ copied" : "copy" }}</span>
      </button>
    </div>

    <div
      class="t-term"
      tabindex="0"
      aria-label="Interactive preview of the LazyAPI terminal UI. Use j and k to move, s to send, tab to switch response tabs."
      @keydown="onKey"
    >
      <div class="chrome">
        <span class="b r" /><span class="b y" /><span class="b g" />
        <span class="ttl">lazyapi examples/openapi.yml</span>
      </div>

      <div class="titlebar">
        <span><span class="brandc">&gt;_ lazyapi</span>  ·  openapi.yml</span>
        <span class="env">env: local</span>
        <span class="url">→ http://127.0.0.1:4010</span>
      </div>

      <div class="body">
        <div class="list">
          <div class="filter">🔍 press / to filter</div>
          <template v-for="g in groups" :key="g.tag">
            <div class="group">── {{ g.tag }} ({{ g.items.length }}) ──</div>
            <button
              v-for="{ op, i } in g.items"
              :key="i"
              type="button"
              class="row"
              :class="{ selected: i === sel }"
              tabindex="-1"
              @click="select(i)"
              @dblclick="send"
            >
              <span class="caret">{{ i === sel ? "▸" : " " }}</span>
              <span :class="methodClass(op.method)" class="m">{{ op.method }}</span>
              <span class="path">{{ op.path }}</span>
            </button>
          </template>
        </div>

        <div class="preview">
          <div class="req">
            <span :class="methodClass(current.method)">{{ current.method }}</span>
            <span class="path">{{ current.path }}</span>
            <button type="button" class="send" tabindex="-1" @click="send">▶ Send</button>
          </div>
          <div class="summary">{{ current.summary }}</div>
          <div class="server">Server: http://127.0.0.1:4010</div>

          <div class="resp">
            <template v-if="sending">
              <div class="spin">⠋ sending {{ current.method }} {{ current.path }}…</div>
            </template>
            <template v-else-if="response">
              <div class="status">
                <span :class="statusClass(response.status)">{{ response.status }} {{ response.text }}</span>
                <span class="meta">{{ response.ms }}ms · {{ size(response) }}</span>
              </div>
              <div class="tabs">
                <button
                  v-for="(t, i) in tabs"
                  :key="t"
                  type="button"
                  tabindex="-1"
                  :class="{ active: i === tab }"
                  @click="tab = i"
                >{{ t }}<span v-if="t === 'Checks'" class="count">4</span></button>
              </div>
              <pre v-if="tab === 0" class="out" v-html="response.body ? highlight(response.body) : '<span class=&quot;dim&quot;>(empty body)</span>'" />
              <pre v-else-if="tab === 1" class="out"><span class="j-key">Content-Type</span>: application/json
<span class="j-key">Date</span>: Mon, 23 Nov 2026 14:11:49 GMT
<span class="j-key">Content-Length</span>: {{ size(response).replace(" B", "") }}</pre>
              <pre v-else-if="tab === 2" class="out"><span class="pass">[PASS]</span> request: request matches the OpenAPI spec
<span class="pass">[PASS]</span> status: status {{ response.status }} is declared
<span class="pass">[PASS]</span> contract: response matches the schema
<span class="pass">[PASS]</span> {{ response.method === "DELETE" ? "user is gone" : "returns the expected body" }}</pre>
              <pre v-else class="out"><span class="dim">dns     </span> 0.1ms  <span class="bar" style="width: 4%" />
<span class="dim">connect </span> 0.2ms  <span class="bar" style="width: 8%" />
<span class="dim">ttfb    </span> {{ (response.ms - 0.4).toFixed(1) }}ms  <span class="bar" style="width: 60%" />
<span class="dim">download</span> 0.1ms  <span class="bar" style="width: 4%" /></pre>
            </template>
            <template v-else>
              <div class="dim">Press s to send this request · Enter to open it in the editor</div>
            </template>
          </div>
        </div>
      </div>

      <div class="help">
        <span><b>s</b> send</span> • <span><b>j/k</b> move</span> • <span><b>tab</b> response tab</span> •
        <span><b>R</b> run tests</span> • <span><b>ctrl+k</b> commands</span> • <span><b>?</b> help</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.t-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 48px;
  max-width: 1280px;
  margin: 0 auto;
  padding: 48px 24px 24px;
  align-items: center;
}

@media (min-width: 640px) {
  .t-hero {
    padding: 64px 48px 32px;
  }
}

@media (min-width: 1100px) {
  .t-hero {
    grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
    padding: 80px 64px 48px;
  }
}

.eyebrow {
  font-family: var(--t-font-ui);
  font-size: 12px;
  color: var(--t-muted);
  margin: 0 0 18px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.eyebrow .dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--t-go);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--t-go) 25%, transparent);
}

.brand {
  font-family: var(--t-font-ui);
  font-size: clamp(44px, 7vw, 72px);
  line-height: 1.05;
  font-weight: 800;
  letter-spacing: -0.04em;
  color: var(--ctp-text);
  margin: 0 0 20px;
}

.brand .p {
  color: var(--t-brand);
}

.cursor {
  color: var(--t-accent);
  margin-left: 4px;
  animation: blink 1.1s steps(1) infinite;
}

@keyframes blink {
  50% {
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .cursor {
    animation: none;
  }
}

.tagline {
  font-size: 20px;
  line-height: 1.5;
  color: var(--ctp-subtext1);
  margin: 0 0 12px;
  max-width: 36em;
}

.tagline strong {
  color: var(--ctp-text);
}

.sub {
  font-size: 15px;
  line-height: 1.6;
  color: var(--ctp-subtext0);
  margin: 0 0 28px;
  max-width: 38em;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 24px;
}

.install {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  max-width: 560px;
  border: 1px solid var(--t-border);
  border-radius: var(--t-radius);
  background: var(--ctp-mantle);
  padding: 10px 14px;
  font-family: var(--t-font-ui);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s;
}

.install:hover,
.install:focus-visible {
  border-color: var(--t-accent);
}

.install .ps {
  color: var(--t-go);
}

.install code {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ctp-text);
}

.install .cp {
  color: var(--t-muted);
  font-size: 12px;
}

/* ---- terminal mock ---- */

.t-term {
  font-family: var(--t-font-ui);
  font-size: 12.5px;
  line-height: 1.55;
  color: var(--ctp-text);
  background: var(--ctp-base);
  border: 1px solid var(--t-border);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 30px 80px -30px color-mix(in srgb, var(--ctp-crust) 80%, transparent),
    0 0 0 1px color-mix(in srgb, var(--ctp-crust) 40%, transparent);
  outline: none;
  transition: border-color 0.15s;
  min-width: 0;
}

.t-term:focus-visible,
.t-term:focus-within {
  border-color: var(--t-accent);
}

.chrome {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 9px 12px;
  background: var(--ctp-crust);
  border-bottom: 1px solid var(--t-rule);
}

.chrome .b {
  width: 11px;
  height: 11px;
  border-radius: 50%;
}

.chrome .r { background: var(--ctp-red); }
.chrome .y { background: var(--ctp-yellow); }
.chrome .g { background: var(--ctp-green); }

.chrome .ttl {
  margin-left: 10px;
  color: var(--t-muted);
  font-size: 12px;
}

.titlebar {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 4px 12px;
  border-bottom: 1px solid var(--t-rule);
  white-space: nowrap;
  overflow: hidden;
}

.brandc {
  color: var(--t-brand);
  font-weight: 700;
}

.env {
  background: var(--t-env);
  color: var(--ctp-crust);
  padding: 0 6px;
}

.url {
  color: var(--t-muted);
  overflow: hidden;
  text-overflow: ellipsis;
}

.body {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
  min-height: 340px;
}

@media (max-width: 560px) {
  .body {
    grid-template-columns: 1fr;
  }

  .list {
    border-right: 0 !important;
    border-bottom: 1px solid var(--t-rule);
  }
}

.list {
  border-right: 1px solid var(--t-rule);
  padding: 6px 0;
}

.filter {
  color: var(--t-muted);
  padding: 0 12px 6px;
  border-bottom: 1px solid var(--t-rule);
  margin-bottom: 4px;
  white-space: nowrap;
  overflow: hidden;
}

.group {
  color: var(--t-muted);
  padding: 2px 12px;
}

.row {
  display: flex;
  width: 100%;
  gap: 6px;
  padding: 0 12px;
  text-align: left;
  white-space: nowrap;
  cursor: pointer;
  font: inherit;
  color: inherit;
}

.row:hover {
  background: color-mix(in srgb, var(--ctp-surface0) 60%, transparent);
}

.row.selected {
  background: color-mix(in srgb, var(--t-accent) 14%, transparent);
}

.row .caret {
  width: 1ch;
  color: var(--t-accent);
}

.row .m {
  width: 6.5ch;
  text-align: right;
  font-size: inherit;
}

.row .path {
  overflow: hidden;
  text-overflow: ellipsis;
}

.row.selected .path {
  color: var(--t-accent);
}

.preview {
  padding: 8px 12px;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.req {
  display: flex;
  gap: 10px;
  align-items: center;
}

.req .t-method {
  font-size: inherit;
}

.req .path {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.send {
  border: 1px solid var(--t-accent);
  border-radius: 6px;
  padding: 0 8px;
  color: var(--t-go);
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

.send:hover {
  background: var(--t-accent);
  color: var(--t-on-accent);
}

.summary {
  color: var(--ctp-subtext1);
}

.server {
  color: var(--t-muted);
  padding-bottom: 6px;
  border-bottom: 1px solid var(--t-rule);
}

.resp {
  padding-top: 6px;
  flex: 1;
  min-width: 0;
}

.status {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 4px;
}

.st {
  color: var(--ctp-crust);
  padding: 0 6px;
  font-weight: 700;
}

.st.ok { background: var(--ctp-green); }
.st.warn { background: var(--ctp-yellow); }
.st.err { background: var(--ctp-red); }

.meta {
  color: var(--t-muted);
}

.tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--t-rule);
  margin-bottom: 6px;
  overflow: hidden;
}

.tabs button {
  font: inherit;
  padding: 0 10px;
  color: var(--t-muted);
  border-bottom: 2px solid transparent;
  cursor: pointer;
  white-space: nowrap;
}

.tabs button.active {
  color: var(--t-accent);
  border-bottom-color: var(--t-accent);
  font-weight: 700;
}

.tabs .count {
  margin-left: 4px;
  color: var(--ctp-green);
}

.out {
  margin: 0;
  font: inherit;
  white-space: pre;
  overflow-x: auto;
}

.out :deep(.j-key) { color: var(--ctp-blue); }
.out :deep(.j-str) { color: var(--ctp-green); }
.out :deep(.j-num) { color: var(--ctp-peach); }
.out :deep(.j-lit) { color: var(--ctp-mauve); }
.out :deep(.dim),
.dim { color: var(--t-muted); }

.pass {
  color: var(--ctp-green);
  font-weight: 700;
}

.bar {
  display: inline-block;
  height: 8px;
  background: var(--t-accent);
  vertical-align: middle;
  border-radius: 2px;
}

.spin {
  color: var(--t-accent);
}

.help {
  padding: 4px 12px;
  border-top: 1px solid var(--t-rule);
  color: var(--t-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.help b {
  color: var(--ctp-subtext1);
}
</style>
