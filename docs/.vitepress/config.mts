import { defineConfig } from "vitepress"

const majorVersion = process.env.DOCS_VERSION || "1"

export default defineConfig({
  title: "LazyAPI",
  description: "OpenAPI-driven API exploration, testing, and automation from the terminal",

  base: `/lazyapi/v${majorVersion}/`,

  cleanUrls: true,

  markdown: {
    theme: {
      light: "catppuccin-latte",
      dark: "catppuccin-mocha",
    },
  },

  themeConfig: {
    logo: false,

    nav: [
      { text: "Home", link: "/" },
      { text: "TUI", link: "/tui/" },
      { text: "CLI", link: "/cli/" },
      { text: "Architecture", link: "/architecture" },
    ],

    sidebar: {
      "/tui/": [
        {
          text: "TUI",
          items: [
            { text: "Overview", link: "/tui/" },
            { text: "Request List", link: "/tui/request-list" },
            { text: "Request Editor", link: "/tui/request-editor" },
            { text: "Keyboard Shortcuts", link: "/tui/keyboard-shortcuts" },
            { text: "Response Preview", link: "/tui/response-preview" },
          ],
        },
      ],
      "/cli/": [
        {
          text: "CLI",
          items: [
            { text: "Overview", link: "/cli/" },
            { text: "create", link: "/cli/create" },
            { text: "add", link: "/cli/add" },
            { text: "remove", link: "/cli/remove" },
            { text: "send", link: "/cli/send" },
            { text: "smoke", link: "/cli/smoke" },
          ],
        },
      ],
      "/": [
        {
          text: "Guide",
          items: [
            { text: "What is LazyAPI?", link: "/" },
            { text: "Installation", link: "/installation" },
            { text: "Getting Started", link: "/getting-started" },
          ],
        },
        {
          text: "Topics",
          items: [
            { text: "Architecture", link: "/architecture" },
            { text: "Authentication", link: "/authentication" },
            { text: "Environment Variables", link: "/environment-variables" },
            { text: "Configuration", link: "/configuration" },
          ],
        },
        {
          text: "Interface",
          items: [
            { text: "TUI", link: "/tui/" },
            { text: "CLI", link: "/cli/" },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: "github", link: "https://github.com/githiago-f/lazyapi" },
    ],

    footer: {
      message: "Released under the GPLv3 License.",
    },
  },
})
