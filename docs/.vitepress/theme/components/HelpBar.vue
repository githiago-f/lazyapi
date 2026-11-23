<script setup lang="ts">
// The TUI's help bar, with the same kind of keys: / search, [ ] prev/next
// page, j/k scroll, g/G top/bottom, t theme, ? every key.
import { onBeforeUnmount, onMounted, ref } from "vue"
import { useData, useRouter, withBase } from "vitepress"

const { isDark, page } = useData()
const router = useRouter()
const help = ref(false)
let lastG = 0

const keys: [string, string][] = [
  ["/  ctrl+k", "search the docs"],
  ["[  ]", "previous / next page"],
  ["j  k", "scroll down / up"],
  ["g g  G", "top / bottom of the page"],
  ["h", "home"],
  ["t", "dark / light theme"],
  ["?", "this help"],
  ["esc", "close"],
]

function editing(e: KeyboardEvent): boolean {
  const el = e.target as HTMLElement | null
  if (!el) return false
  const tag = el.tagName
  return el.isContentEditable || tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT"
}

function pager(dir: "prev" | "next") {
  const a = document.querySelector<HTMLAnchorElement>(`.VPDocFooter .pager-link.${dir}`)
  if (a) a.click()
}

function onKey(e: KeyboardEvent) {
  if (e.metaKey || e.ctrlKey || e.altKey || editing(e)) return
  if (document.querySelector(".VPLocalSearchBox")) return
  if (help.value) {
    if (e.key === "Escape" || e.key === "?" || e.key === "q") {
      help.value = false
      e.preventDefault()
    }
    return
  }
  switch (e.key) {
    case "[":
      pager("prev")
      break
    case "]":
      pager("next")
      break
    case "j":
      window.scrollBy({ top: 80 })
      break
    case "k":
      window.scrollBy({ top: -80 })
      break
    case "G":
      window.scrollTo({ top: document.body.scrollHeight })
      break
    case "g": {
      const now = Date.now()
      if (now - lastG < 500) window.scrollTo({ top: 0 })
      lastG = now
      break
    }
    case "h":
      router.go(withBase("/"))
      break
    case "t":
      isDark.value = !isDark.value
      break
    case "?":
      help.value = true
      break
    default:
      return
  }
  e.preventDefault()
}

onMounted(() => window.addEventListener("keydown", onKey))
onBeforeUnmount(() => window.removeEventListener("keydown", onKey))
</script>

<template>
  <div class="t-helpbar" role="note" aria-label="Keyboard shortcuts">
    <span><span class="k">/</span> search</span>
    <span class="sep">•</span>
    <span><span class="k">[ ]</span> prev/next</span>
    <span class="sep">•</span>
    <span><span class="k">j/k</span> scroll</span>
    <span class="sep">•</span>
    <span><span class="k">h</span> home</span>
    <span class="sep">•</span>
    <span><span class="k">t</span> theme</span>
    <span class="sep">•</span>
    <button type="button" @click="help = true"><span class="k">?</span> help</button>
    <span class="right">{{ page.relativePath.replace(/\.md$/, "").replace(/(^|\/)index$/, "$1") || "home" }}</span>
  </div>

  <Teleport to="body">
    <div v-if="help" class="t-overlay" @click.self="help = false">
      <div class="t-pane" role="dialog" aria-label="Keyboard shortcuts">
        <span class="t-pane-title">keys</span>
        <table>
          <tbody>
            <tr v-for="[k, d] in keys" :key="k">
              <td>{{ k }}</td>
              <td>{{ d }}</td>
            </tr>
          </tbody>
        </table>
        <div class="hint">the TUI has its own: press ? inside lazyapi</div>
      </div>
    </div>
  </Teleport>
</template>
