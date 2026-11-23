<script setup lang="ts">
// One spec, three interfaces: panes with the title in the border.
import { withBase } from "vitepress"

const items = [
  {
    title: "TUI",
    who: "humans",
    text: "Browse operations grouped by tag, edit params, auth, headers and body, send with alt+enter and read the response with highlighting, jq filters, cookies, timing and checks.",
    cmd: "lazyapi api.yml",
    link: "/tui/",
    keys: ["s send", "R run", "H history", "ctrl+k"],
  },
  {
    title: "CLI",
    who: "CI · scripts",
    text: "Run every operation with its checks and tests, in parallel, with retries. Reports as pretty text, JSON, JUnit or TAP, and the same exit codes everywhere.",
    cmd: "lazyapi run api.yml --output junit",
    link: "/cli/",
    keys: ["run", "send", "describe", "mock"],
  },
  {
    title: "MCP",
    who: "AI agents",
    text: "An MCP server confined to a root directory: agents list and describe operations, send requests, write tests and flows, and run them, with readable failures.",
    cmd: "lazyapi mcp --root .",
    link: "/agents",
    keys: ["send_request", "write_test", "run_tests"],
  },
]
</script>

<template>
  <div class="t-ifaces">
    <a v-for="it in items" :key="it.title" class="t-pane" :href="withBase(it.link)">
      <span class="t-pane-title">{{ it.title }} <span class="who">· {{ it.who }}</span></span>
      <p>{{ it.text }}</p>
      <code class="cmd"><span class="ps">$</span> {{ it.cmd }}</code>
      <div class="keys">
        <span v-for="k in it.keys" :key="k" class="t-kbd">{{ k }}</span>
      </div>
    </a>
  </div>
</template>

<style scoped>
.t-ifaces {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 24px;
  margin: 32px 0 8px;
}

.t-pane {
  display: flex;
  flex-direction: column;
  gap: 12px;
  text-decoration: none !important;
  color: inherit !important;
}

.who {
  color: var(--t-muted);
  font-weight: 400;
}

p {
  margin: 0 !important;
  font-size: 14px;
  line-height: 1.6;
  color: var(--ctp-subtext1);
  flex: 1;
}

.cmd {
  display: block;
  font-size: 12.5px !important;
  padding: 6px 10px !important;
  color: var(--ctp-text) !important;
  background: var(--ctp-mantle) !important;
  border-radius: 6px;
  overflow-x: auto;
  white-space: nowrap;
}

.ps {
  color: var(--t-go);
}

.keys {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
