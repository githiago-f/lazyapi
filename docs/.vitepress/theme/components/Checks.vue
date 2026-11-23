<script setup lang="ts">
// The four checks every exchange goes through, as in the runner view.
const checks = [
  { name: "request", text: "the request sent matches the spec: required params, types, body schema" },
  { name: "status", text: "the status code is declared in the operation's responses" },
  { name: "contract", text: "response headers and body match the declared schema" },
  { name: "test", text: "each test() in the operation's script passes" },
]
</script>

<template>
  <div class="t-checks">
    <div v-for="(c, i) in checks" :key="c.name" class="t-pane">
      <span class="t-pane-title">{{ i + 1 }} · {{ c.name }}</span>
      <span class="pass">[PASS]</span>
      <span class="txt">{{ c.text }}</span>
    </div>
  </div>
</template>

<style scoped>
.t-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 20px;
  margin: 28px 0 8px;
}

.t-pane {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 14px;
}

.pass {
  font-family: var(--t-font-ui);
  font-weight: 700;
  font-size: 13px;
  color: var(--ctp-green);
}

.txt {
  color: var(--ctp-subtext1);
  line-height: 1.55;
}
</style>
