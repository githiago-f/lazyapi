<script setup lang="ts">
import { withBase } from "vitepress"
</script>

<template>
  <div class="t-notfound">
    <div class="t-pane">
      <span class="t-pane-title">404</span>
      <p class="status"><span class="code">404</span> Not Found <span class="muted">· 0 B</span></p>
      <p class="msg">[FAIL] status: this page is not declared in the docs</p>
      <p class="hint">It may have moved: the docs now live at the root of the site, without <code>/lazyapi</code> or <code>/latest</code>.</p>
      <div class="actions">
        <a class="t-btn primary" :href="withBase('/')">▶ Home</a>
        <a class="t-btn" :href="withBase('/getting-started')">Getting started</a>
      </div>
    </div>
  </div>
</template>

<style scoped>
.t-notfound {
  display: grid;
  place-items: center;
  padding: 96px 24px;
}

.t-pane {
  width: min(560px, 100%);
  font-family: var(--t-font-ui);
}

.status {
  font-size: 18px;
  font-weight: 700;
  margin: 0 0 12px;
}

.code {
  background: var(--ctp-red);
  color: var(--ctp-crust);
  padding: 0 8px;
  border-radius: 3px;
}

.muted {
  color: var(--t-muted);
  font-weight: 400;
}

.msg {
  color: var(--ctp-red);
  font-size: 13px;
}

.hint {
  color: var(--ctp-subtext0);
  font-family: var(--vp-font-family-base);
  font-size: 14px;
  margin: 12px 0 20px;
}

.actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
</style>
