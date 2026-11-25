import { defineConfig, type DefaultTheme } from "vitepress"

// The site is served from the root of https://lazyapi.ia.br. The current
// docs live at "/" and every major version is also archived at "/v<major>/".
// The workflow builds twice: DOCS_BASE=/ and DOCS_BASE=/v<major>/.
const base = normalizeBase(process.env.DOCS_BASE || "/")
const version = process.env.DOCS_VERSION || "dev"
const major = version.replace(/^v/, "").split(".")[0]
const site = "https://lazyapi.ia.br"
const repo = "https://github.com/githiago-f/lazyapi"

function normalizeBase(b: string): string {
  if (!b.startsWith("/")) b = "/" + b
  if (!b.endsWith("/")) b += "/"
  return b
}

// Old links: github.io/lazyapi/latest/… and lazyapi.ia.br/lazyapi/latest/…
// Strip the "/lazyapi" and "/latest" prefixes before anything else loads,
// so they land on the page that exists (also from the 404 page).
const legacyRedirect = `(function(){var p=location.pathname;var n=p.replace(/^\\/lazyapi(?=\\/|$)/,"").replace(/^\\/latest(?=\\/|$)/,"");if(n!==p){location.replace((n||"/")+location.search+location.hash)}})();`

const guide: DefaultTheme.SidebarItem[] = [
  {
    text: "Guide",
    items: [
      { text: "What is LazyAPI?", link: "/introduction" },
      { text: "Installation", link: "/installation" },
      { text: "Getting Started", link: "/getting-started" },
    ],
  },
  {
    text: "Topics",
    items: [
      { text: "Environments & Auth", link: "/environments" },
      { text: "Authentication", link: "/authentication" },
      { text: "Environment Variables", link: "/environment-variables" },
      { text: "Configuration", link: "/configuration" },
      { text: "Test Scripts", link: "/scripting" },
      { text: "AI Agents (MCP)", link: "/agents" },
      { text: "Architecture", link: "/architecture" },
    ],
  },
  {
    text: "TUI",
    collapsed: false,
    items: [
      { text: "Overview", link: "/tui/" },
      { text: "Request List", link: "/tui/request-list" },
      { text: "Request Editor", link: "/tui/request-editor" },
      { text: "Response Preview", link: "/tui/response-preview" },
      { text: "Runner, Flows & History", link: "/tui/runner" },
      { text: "Keyboard Shortcuts", link: "/tui/keyboard-shortcuts" },
    ],
  },
  {
    text: "CLI",
    collapsed: false,
    items: [
      { text: "Overview", link: "/cli/" },
      { text: "run", link: "/cli/run" },
      { text: "send", link: "/cli/send" },
      { text: "list / describe / validate", link: "/cli/inspect" },
      { text: "mock", link: "/cli/mock" },
      { text: "import", link: "/cli/import" },
      { text: "login", link: "/cli/login" },
      { text: "fake", link: "/cli/fake" },
      { text: "create", link: "/cli/create" },
      { text: "add", link: "/cli/add" },
      { text: "remove", link: "/cli/remove" },
      { text: "smoke", link: "/cli/smoke" },
    ],
  },
]

export default defineConfig({
  title: "lazyapi",
  titleTemplate: ":title · LazyAPI",
  description: "OpenAPI-driven API exploration, testing and automation from the terminal, for humans and AI agents.",
  lang: "en-US",

  base,
  cleanUrls: true,
  lastUpdated: true,
  appearance: "dark",

  sitemap: base === "/" ? { hostname: site } : undefined,

  head: [
    ["script", {}, legacyRedirect],
    ["link", { rel: "icon", type: "image/svg+xml", href: `${base}favicon.svg` }],
    ["meta", { name: "theme-color", content: "#1e1e2e" }],
    ["meta", { property: "og:title", content: "LazyAPI" }],
    ["meta", { property: "og:description", content: "OpenAPI-driven API testing from the terminal." }],
    ["meta", { property: "og:url", content: site }],
    // Archived versions point search engines at the current docs.
    ...(base === "/" ? [] : [["meta", { name: "robots", content: "noindex" }] as [string, Record<string, string>]]),
  ],

  markdown: {
    // Inline code is never a Vue template: `{{env.X}}` shows as written.
    config(md) {
      const inline = md.renderer.rules.code_inline!
      md.renderer.rules.code_inline = (tokens, idx, options, env, self) => {
        tokens[idx].attrSet("v-pre", "")
        return inline(tokens, idx, options, env, self)
      }
    },
    theme: {
      light: "catppuccin-latte",
      dark: "catppuccin-mocha",
    },
  },

  themeConfig: {
    siteTitle: "lazyapi",
    logo: false,

    nav: [
      { text: "Guide", link: "/getting-started", activeMatch: "^/(introduction|installation|getting-started|environments|authentication|environment-variables|configuration|scripting|agents|architecture)" },
      { text: "TUI", link: "/tui/", activeMatch: "^/tui/" },
      { text: "CLI", link: "/cli/", activeMatch: "^/cli/" },
      { text: "MCP", link: "/agents" },
      {
        text: version === "dev" ? "dev" : `v${version.replace(/^v/, "")}`,
        items: [
          { text: "Latest", link: `${site}/` },
          ...(major && major !== "dev" ? [{ text: `v${major}.x`, link: `${site}/v${major}/` }] : []),
          { text: "Releases", link: `${repo}/releases` },
        ],
      },
    ],

    sidebar: {
      "/": guide,
    },

    outline: { level: [2, 3], label: "On this page" },

    search: {
      provider: "local",
    },

    editLink: {
      pattern: `${repo}/edit/main/docs/:path`,
      text: "Edit this page on GitHub",
    },

    lastUpdated: {
      text: "Updated",
      formatOptions: { dateStyle: "medium" },
    },

    docFooter: { prev: "prev", next: "next" },

    socialLinks: [{ icon: "github", link: repo }],

    footer: {
      message: "Released under the GPLv3 License.",
      copyright: "LazyAPI · the OpenAPI spec is the source of truth",
    },
  },
})
