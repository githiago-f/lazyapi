import DefaultTheme from "vitepress/theme"
import type { Theme } from "vitepress"
import Layout from "./Layout.vue"
import Method from "./components/Method.vue"
import Pane from "./components/Pane.vue"
import Interfaces from "./components/Interfaces.vue"
import Checks from "./components/Checks.vue"
import Screen from "./components/Screen.vue"
import Gallery from "./components/Gallery.vue"
import "./tui.css"

export default {
  extends: DefaultTheme,
  Layout,
  enhanceApp({ app }) {
    app.component("Method", Method)
    app.component("Pane", Pane)
    app.component("Interfaces", Interfaces)
    app.component("Checks", Checks)
    app.component("Screen", Screen)
    app.component("Gallery", Gallery)
  },
} satisfies Theme
