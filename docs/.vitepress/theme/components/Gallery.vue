<script setup lang="ts">
// Home: tabs over the real TUI captures, like the TUI's own tab bar.
import { ref } from "vue"
import { withBase } from "vitepress"

const shots = [
  { name: "list-preview", tab: "Request list", text: "Operations grouped by tag. s sends the selected one and the response shows up right there." },
  { name: "request-editor", tab: "Editor", text: "Params, auth, headers, body and tests. alt+enter sends from any field." },
  { name: "list-checks", tab: "Checks", text: "Every response is checked: request, status, contract and your test() calls." },
  { name: "runner", tab: "Runner", text: "R runs the spec, a tag or the flows. Results stream in, failures first." },
  { name: "palette", tab: "Palette", text: "ctrl+k: open any operation, run the spec, a tag or a flow, switch environment." },
  { name: "history", tab: "History", text: "H: replay past requests and diff a response with the previous one or the spec example." },
]
const active = ref(0)

function onKey(e: KeyboardEvent) {
  if (e.key === "ArrowRight" || e.key === "]") active.value = (active.value + 1) % shots.length
  else if (e.key === "ArrowLeft" || e.key === "[") active.value = (active.value - 1 + shots.length) % shots.length
  else return
  e.preventDefault()
  e.stopPropagation()
}
</script>

<template>
  <div class="t-gallery" tabindex="0" @keydown="onKey">
    <div class="tabs" role="tablist">
      <button
        v-for="(s, i) in shots"
        :key="s.name"
        type="button"
        role="tab"
        :aria-selected="i === active"
        :class="{ active: i === active }"
        @click="active = i"
      >{{ s.tab }}</button>
    </div>
    <p class="text">{{ shots[active].text }}</p>
    <div class="frame">
      <img
        v-for="(s, i) in shots"
        v-show="i === active"
        :key="s.name"
        :src="withBase(`/screens/${s.name}.svg`)"
        :alt="`LazyAPI TUI: ${s.tab}`"
        loading="lazy"
      />
    </div>
    <p class="hint">real captures of <code>lazyapi examples/openapi.yml</code> · ← → or [ ] to switch</p>
  </div>
</template>

<style scoped>
.t-gallery {
  margin: 28px 0 8px;
  outline: none;
}

.tabs {
  display: flex;
  flex-wrap: wrap;
  border-bottom: 1px solid var(--t-rule);
}

.tabs button {
  font-family: var(--t-font-ui);
  font-size: 13px;
  padding: 6px 14px;
  color: var(--t-muted);
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  cursor: pointer;
}

.tabs button:hover {
  color: var(--ctp-text);
}

.tabs button.active {
  color: var(--t-accent);
  border-bottom-color: var(--t-accent);
  font-weight: 700;
}

.text {
  margin: 14px 0 !important;
  color: var(--ctp-subtext1);
  font-size: 15px;
}

.frame img {
  width: 100%;
  height: auto;
  border-radius: 10px;
  box-shadow: 0 30px 70px -30px color-mix(in srgb, var(--ctp-crust) 85%, transparent);
}

.hint {
  margin-top: 10px !important;
  font-family: var(--t-font-ui);
  font-size: 12px;
  color: var(--t-muted);
  text-align: center;
}
</style>
