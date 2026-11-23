#!/usr/bin/env node
// ansi2svg: renders a terminal capture (tmux capture-pane -e -p) as an SVG
// "screenshot" with window chrome. No dependencies.
//
//   node ansi2svg.mjs <in.ansi> <out.svg> --cols 140 --rows 36 [--title "…"]
//
// Every cell sits on a fixed grid (each run of text is forced to its exact
// width with textLength), so box-drawing lines up whatever font is used.

import fs from "node:fs"

const args = process.argv.slice(2)
const [inFile, outFile] = args
const opt = (name, def) => {
  const i = args.indexOf("--" + name)
  return i >= 0 ? args[i + 1] : def
}
const COLS = +opt("cols", 140)
const ROWS = +opt("rows", 36)
const TITLE = opt("title", "lazyapi")

// Catppuccin Mocha
const BG = "#1e1e2e"
const FG = "#cdd6f4"
const CHROME = "#11111b"
const BORDER = "#45475a"
const ANSI16 = [
  "#45475a", "#f38ba8", "#a6e3a1", "#f9e2af", "#89b4fa", "#f5c2e7", "#94e2d5", "#bac2de",
  "#585b70", "#f38ba8", "#a6e3a1", "#f9e2af", "#89b4fa", "#f5c2e7", "#94e2d5", "#a6adc8",
]

function xterm256(n) {
  if (n < 16) return ANSI16[n]
  if (n < 232) {
    n -= 16
    const v = [0, 95, 135, 175, 215, 255]
    return hex(v[Math.floor(n / 36)], v[Math.floor(n / 6) % 6], v[n % 6])
  }
  const g = 8 + (n - 232) * 10
  return hex(g, g, g)
}

function hex(r, g, b) {
  return "#" + [r, g, b].map((x) => x.toString(16).padStart(2, "0")).join("")
}

// Cell width: 2 for emoji and wide East Asian, 0 for combining marks.
function width(cp) {
  if (cp === 0x200d || (cp >= 0x300 && cp <= 0x36f) || (cp >= 0xfe00 && cp <= 0xfe0f)) return 0
  if (
    (cp >= 0x1f300 && cp <= 0x1faff) ||
    (cp >= 0x2e80 && cp <= 0xa4cf) ||
    (cp >= 0xac00 && cp <= 0xd7a3) ||
    (cp >= 0xf900 && cp <= 0xfaff) ||
    (cp >= 0xff00 && cp <= 0xff60)
  )
    return 2
  return 1
}


// Box drawing is drawn as geometry: font glyphs leave gaps between rows.
// [up, right, down, left] arm weights (1 light, 2 heavy), "r" = rounded corner.
const BOX = {
  "─": [0, 1, 0, 1], "━": [0, 2, 0, 2], "│": [1, 0, 1, 0], "┃": [2, 0, 2, 0],
  "┌": [0, 1, 1, 0], "┐": [0, 0, 1, 1], "└": [1, 1, 0, 0], "┘": [1, 0, 0, 1],
  "├": [1, 1, 1, 0], "┤": [1, 0, 1, 1], "┬": [0, 1, 1, 1], "┴": [1, 1, 0, 1], "┼": [1, 1, 1, 1],
  "╴": [0, 0, 0, 1], "╶": [0, 1, 0, 0], "╵": [1, 0, 0, 0], "╷": [0, 0, 1, 0],
  "╭": "r", "╮": "r", "╯": "r", "╰": "r",
}

function boxPath(ch, x, y, cw, chh, color) {
  const cx = x + cw / 2
  const cy = y + chh / 2
  const def = BOX[ch]
  if (def === "r") {
    const r = Math.min(cw, chh) / 2
    let d
    if (ch === "╭") d = `M${cx} ${y + chh}V${cy + r}Q${cx} ${cy} ${cx + r} ${cy}H${x + cw}`
    if (ch === "╮") d = `M${x} ${cy}H${cx - r}Q${cx} ${cy} ${cx} ${cy + r}V${y + chh}`
    if (ch === "╰") d = `M${cx} ${y}V${cy - r}Q${cx} ${cy} ${cx + r} ${cy}H${x + cw}`
    if (ch === "╯") d = `M${x} ${cy}H${cx - r}Q${cx} ${cy} ${cx} ${cy - r}V${y}`
    return `<path d="${d}" fill="none" stroke="${color}" stroke-width="1.2"/>`
  }
  const [u, r, dn, l] = def
  const seg = (w, x1, y1, x2, y2) =>
    w ? `<line x1="${x1.toFixed(1)}" y1="${y1}" x2="${x2.toFixed(1)}" y2="${y2}" stroke="${color}" stroke-width="${w === 2 ? 2.6 : 1.2}"/>` : ""
  return (
    seg(u, cx, y, cx, cy) + seg(dn, cx, cy, cx, y + chh) + seg(l, x, cy, cx, cy) + seg(r, cx, cy, x + cw, cy)
  )
}

const blank = () => ({ ch: " ", fg: null, bg: null, bold: false, faint: false, italic: false, under: false })

function parse(text) {
  const lines = text.replace(/\r/g, "").split("\n").slice(0, ROWS)
  const grid = []
  for (const line of lines) {
    const row = []
    let st = { fg: null, bg: null, bold: false, faint: false, italic: false, under: false, rev: false }
    const push = (ch, w) => {
      const fg = st.rev ? st.bg || BG : st.fg
      const bg = st.rev ? st.fg || FG : st.bg
      row.push({ ch, w, fg, bg, bold: st.bold, faint: st.faint, italic: st.italic, under: st.under })
      for (let k = 1; k < w; k++) row.push(null) // continuation cell
    }
    const visible = (s) => [...s.replace(/\x1b\[[0-9;:]*[A-Za-z]/g, "")].reduce((n, c) => n + width(c.codePointAt(0)), 0)
    for (let i = 0; i < line.length; ) {
      if (line[i] === "\x1b" && line[i + 1] === "[") {
        const m = /^\x1b\[([0-9;:]*)([A-Za-z])/.exec(line.slice(i))
        if (!m) { i++; continue }
        i += m[0].length
        if (m[2] !== "m") continue
        const ps = m[1] === "" ? [0] : m[1].split(/[;:]/).map(Number)
        for (let j = 0; j < ps.length; j++) {
          const p = ps[j]
          if (p === 0) st = { fg: null, bg: null, bold: false, faint: false, italic: false, under: false, rev: false }
          else if (p === 1) st.bold = true
          else if (p === 2) st.faint = true
          else if (p === 3) st.italic = true
          else if (p === 4) st.under = true
          else if (p === 7) st.rev = true
          else if (p === 22) st.bold = st.faint = false
          else if (p === 23) st.italic = false
          else if (p === 24) st.under = false
          else if (p === 27) st.rev = false
          else if (p >= 30 && p <= 37) st.fg = ANSI16[p - 30]
          else if (p >= 90 && p <= 97) st.fg = ANSI16[p - 90 + 8]
          else if (p >= 40 && p <= 47) st.bg = ANSI16[p - 40]
          else if (p >= 100 && p <= 107) st.bg = ANSI16[p - 100 + 8]
          else if (p === 39) st.fg = null
          else if (p === 49) st.bg = null
          else if (p === 38 || p === 48) {
            let c = null
            if (ps[j + 1] === 5) { c = xterm256(ps[j + 2]); j += 2 }
            else if (ps[j + 1] === 2) { c = hex(ps[j + 2], ps[j + 3], ps[j + 4]); j += 4 }
            if (p === 38) st.fg = c
            else st.bg = c
          }
        }
        continue
      }
      const cp = line.codePointAt(i)
      const ch = String.fromCodePoint(cp)
      i += ch.length
      if (ch === "\t") {
        // tmux may compress a run of blanks into a tab: fill up to the
        // width the rest of the line needs to end on the last column.
        const rest = visible(line.slice(i))
        const n = Math.max(1, COLS - row.length - rest)
        for (let k = 0; k < n; k++) push(" ", 1)
        continue
      }
      const w = width(cp)
      if (w === 0) {
        const last = [...row].reverse().find((c) => c)
        if (last) last.ch += ch
        continue
      }
      push(ch, w)
    }
    while (row.length < COLS) row.push(blank())
    grid.push(row.slice(0, COLS))
  }
  while (grid.length < ROWS) grid.push(Array.from({ length: COLS }, blank))
  return grid
}

const esc = (s) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")

function render(grid) {
  const CW = 8.4 // cell width at font-size 14
  const CH = 18 // line height
  const PAD = 14
  const BAR = 34
  const W = Math.round(COLS * CW + PAD * 2)
  const H = Math.round(ROWS * CH + PAD * 2 + BAR)
  const ox = PAD
  const oy = BAR + PAD

  const out = []
  out.push(
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="${esc(TITLE)}">`,
    `<style>text{font-family:"JetBrains Mono","Fira Code","Cascadia Code",ui-monospace,Menlo,Consolas,monospace;font-size:14px;white-space:pre}</style>`,
    `<rect width="${W}" height="${H}" rx="10" fill="${BG}"/>`,
    `<path d="M0 10a10 10 0 0 1 10-10h${W - 20}a10 10 0 0 1 10 10v${BAR - 10}h-${W}z" fill="${CHROME}"/>`,
    `<circle cx="20" cy="17" r="6" fill="#f38ba8"/><circle cx="40" cy="17" r="6" fill="#f9e2af"/><circle cx="60" cy="17" r="6" fill="#a6e3a1"/>`,
    `<text x="${W / 2}" y="22" fill="#7f849c" text-anchor="middle" style="font-size:12px">${esc(TITLE)}</text>`,
    `<rect x="0.5" y="0.5" width="${W - 1}" height="${H - 1}" rx="10" fill="none" stroke="${BORDER}"/>`,
  )

  grid.forEach((row, r) => {
    const y = oy + r * CH
    // backgrounds
    for (let c = 0; c < COLS; ) {
      const cell = row[c]
      if (!cell || !cell.bg) { c++; continue }
      let e = c + 1
      while (e < COLS && (row[e] === null || (row[e] && row[e].bg === cell.bg))) e++
      out.push(`<rect x="${(ox + c * CW).toFixed(1)}" y="${y}" width="${((e - c) * CW).toFixed(1)}" height="${CH}" fill="${cell.bg}"/>`)
      c = e
    }
    // box drawing
    for (let c = 0; c < COLS; c++) {
      const cell = row[c]
      if (cell && BOX[cell.ch]) {
        out.push(boxPath(cell.ch, ox + c * CW, y, CW, CH, cell.fg || FG))
      }
    }
    // text runs with the same style (box drawing becomes a blank)
    for (let c = 0; c < COLS; ) {
      const cell = row[c]
      if (!cell) { c++; continue }
      const key = (x) => x && `${x.fg}|${x.bold}|${x.faint}|${x.italic}|${x.under}`
      let e = c
      let s = ""
      while (e < COLS && (row[e] === null || key(row[e]) === key(cell))) {
        if (row[e]) s += BOX[row[e].ch] ? " " : row[e].ch
        e++
      }
      const trimmed = s.replace(/\s+$/, "")
      if (trimmed.trim()) {
        const cells = [...trimmed].reduce((n, ch) => n + width(ch.codePointAt(0)), 0)
        const lead = trimmed.length - trimmed.trimStart().length
        const text = trimmed.trimStart()
        const x = ox + (c + lead) * CW
        const attrs = [
          `x="${x.toFixed(1)}"`,
          `y="${y + 13.5}"`,
          `fill="${cell.fg || FG}"`,
          `textLength="${((cells - lead) * CW).toFixed(1)}"`,
          `lengthAdjust="spacingAndGlyphs"`,
        ]
        if (cell.bold) attrs.push(`font-weight="700"`)
        if (cell.faint) attrs.push(`opacity="0.6"`)
        if (cell.italic) attrs.push(`font-style="italic"`)
        if (cell.under) attrs.push(`text-decoration="underline"`)
        out.push(`<text ${attrs.join(" ")}>${esc(text)}</text>`)
      }
      c = e
    }
  })

  out.push("</svg>")
  return out.join("\n")
}

fs.writeFileSync(outFile, render(parse(fs.readFileSync(inFile, "utf8"))))
