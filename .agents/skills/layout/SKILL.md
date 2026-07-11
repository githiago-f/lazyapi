---
name: layout
description: >
  Use this skill for any TUI layout work on LazyAPI. Covers sizing calculations,
  WindowSizeMsg propagation through the component tree, border/padding overhead
  accounting, and rendering validation with DebugRenderPane.
  Triggered by requests like "fix layout", "borders overflow", "sizing is wrong",
  "element doesn't fit", "align this component", "adjust the layout".
---

# LazyAPI Layout Validation Skill

This skill guides agents through validating and fixing TUI layout in a Bubbletea/lipgloss application. The core idea: **every component is a box within a box**. To size a component correctly, trace from the outermost box (terminal) inward, accounting for each container's border/padding/margin overhead and each sibling's consumed space.

## The "div > input, input" Mental Model

Think of the layout as:

```
div                ← parent container (terminal, tab pane, etc.)
├── border(2)      ← container's own overhead (border + padding)
├── sibling1       ← label row, toolbar, etc. (takes N lines)
└── sibling2       ← main content (gets remaining space)
```

**Rule:** A child's available space = parent's allocated space − parent's overhead (borders, padding) − preceding siblings' sizes.

For two siblings side by side:
```
terminal width = 100
div border(1 each side) = 2
├── input1 width = 40
└── input2 width = 100 − 2(border) − 40 = 58
```

For vertical stacking:
```
terminal height = 50
div border(top + bottom) = 2
├── titleBar  = 2 lines (text + bottom border)
├── toolRow   = 3 lines (bordered controls)
└── content   = 50 − 2 − 2 − 3 = 43 lines
```

## When This Skill MUST Be Used

- Adding a new TUI page or view
- Adding or removing elements from an existing View()
- Changing border styles, padding, or margins on any component
- Sizing changes to selectors, fields, textareas, viewports
- Any `WindowSizeMsg` handler that computes child dimensions
- Debugging layout overflow, truncated content, or misaligned borders
- Modifying scrollable containers and their content sizing

## Layout Validation Workflow

### Step 1: Map the Element Tree

Read every `View()` method from the outermost `Tui.View()` down to the innermost content. Build a tree like this:

```
Terminal (msg.Width × msg.Height)
└── Tui.View()
    ├── prompt (0 lines when closed)
    ├── titleBar (2 lines: text + bottom border)
    ├── editor/requestList (fill remaining)
    │   ├── requestURL row (3 lines: border top/content/bottom)
    │   │   ├── Method (NormalBorder = 3 lines)
    │   │   ├── Server (NormalBorder = 3 lines)
    │   │   ├── URI (NormalBorder = 3 lines)
    │   │   └── Send (InnerHalfBlockBorder = 3 lines)
    │   └── tabs + response (tabsHeight lines)
    │       ├── RequestTabs (tabsHeight, bordered)
    │       │   ├── top border (1 line)
    │       │   ├── labels text (1 line)
    │       │   ├── labels bottom border (1 line)
    │       │   ├── scrollable content (tabsHeight - 4 lines)
    │       │   └── bottom border (1 line)
    │       └── ResponsePreview (tabsHeight, bordered)
    └── help (1+ lines)
```

Each node records its **allocated size**, its **own overhead**, and the **remaining space** passed to children.

### Step 2: Identify Every Overhead Source

For each element, determine how many lines/columns it consumes beyond its content:

| lipgloss Style | Vertical Overhead | Horizontal Overhead |
|----------------|-------------------|---------------------|
| `Border(NormalBorder())` | 2 (top + bottom) | 2 (left + right) |
| `Border(InnerHalfBlockBorder())` | 2 | 2 |
| `Border(RoundedBorder())` | 2 | 2 |
| `Border(DoubleBorder())` | 2 | 2 |
| `BorderBottom(true)` | 1 (bottom only) | 0 |
| `BorderTop(true)` | 1 (top only) | 0 |
| `Padding(0, N)` | 0 | N*2 (left + right) |
| `Padding(N, 0)` | N*2 (top + bottom) | 0 |
| `Margin(0, N)` | 0 | N*2 |
| `Margin(N, 0)` | N*2 | 0 |

### Step 3: Trace the `WindowSizeMsg` Chain

Each component's `Update()` handles `tea.WindowSizeMsg` and may forward a modified message to children.

1. **Read each `Update()` method** in the chain.
2. **Identify what dimensions are stored** (`m.Width`, `m.Height`, etc.)
3. **Determine what's forwarded** — is it the raw `msg` or a derived `childrenMsg`?
4. **Verify the forward calculation** subtracts ALL overhead.

Key pattern — the `childrenMsg` in `editor.go:483`:

```go
// Good: width subtracts border
childrenMsg := tea.WindowSizeMsg{
    Width:  rp.RequestTabs.Width - 2,    // 2 for left+right border
    Height: max(0, tabsHeight - 4),      // MUST subtract: top border(1) + labels(1) + labels border(1) + bottom border(1)
}
```

### Step 4: Use `DebugRenderPane` to Validate

The `DebugRenderPane(width, height int) string` function in `internal/app/pane/editor/debug.go` renders the full editor at any terminal size. The matching test `TestDebugLayout` in `debug_test.go` measures rendered output.

**To validate a layout change:**

1. Call `DebugRenderPane(W, H)` with known dimensions.
2. Split output by `\n` and measure:
   - Total lines
   - Position of each border character (`┌`, `┐`, `└`, `┘`)
   - Width of each section (find `┐` → `┌` transitions)
   - Content area dimensions
3. Assert that:
   - Total rendered lines ≤ allocated height
   - Tab content + tab overhead == tabsHeight
   - All content fits within border bounds

### Step 5: Verify the Arithmetic

For every `childrenMsg` or propagated size, verify with a concrete example:

```
Terminal: 194 x 46
methodHeight = 3 (NormalBorder selector)
tabsHeight = 46 - (3 + 5) = 38

RequestTabs outer: 38 lines
  top border:       1
  labels + sep:     2
  content area:    34  ← this must equal childrenMsg.Height
  bottom border:    1
  total:           38 ✓
```

## Common Pitfalls

### 1. Not calling `updateFieldWidths()` after adding fields

When a new header/param field is appended, its inner `textinput.Model.Width` defaults to 0 (auto-size). Without an explicit width, typing long content stretches the field, causing lipgloss wrapping inside the tab border.

**Always call `updateFieldWidths()` after appending a new field.**

### 2. Forgetting that `BorderBottom(true)` adds a line

A label rendered with `BorderBottom(true)` is 2 lines tall (text + border). When joined horizontally with other labels, the row height = 2. When joined vertically with content below, the 2 lines reduce available content space.

### 3. Confusing lipgloss `Height` behavior

`lipgloss.Style.Height(N)` sets the box height to **exactly** N rows (truncating content). This includes borders. If content + borders > N, content is truncated — **content is NOT visible beyond the borders even if the scrollable thinks it has more space**.

### 4. Using lipgloss.Size() on a rendered element to get its height

`lipgloss.Size(str)` returns the rendered width and height of a string. For complex bordered elements, always render first, then measure. Never assume a bordered element is 3 lines — verify with `Size()`.

### 5. Hardcoding magic constants without documenting them

The `+5` in `tabsHeight := msg.Height - (methodHeight + 5)` is a magic number. When introducing similar constants, add a comment:

```go
// 5 = 2 (tabs top border) + 2 (tabs bottom border) + 1 (gap)
tabsHeight := msg.Height - (methodHeight + 5)
```

Better yet, derive constants from actual component measurements.

## Reference: Full Editor Layout

### Height Chain (terminal at 46 lines)

| Component | Lines | Formula |
|-----------|-------|---------|
| Terminal | 46 | `msg.Height` |
| prompt (closed) | 0 | empty string |
| titleBar | 2 | text + bottom border |
| help (short) | 1 | bubbles/help default |
| Tui fixed overhead | 3 | titleBar(2) + help(1) |
| Editor receives | 46 | unadjusted `msg.Height` from Tui |
| requestURL row | 3 | methodHeight (max of Method/Server/URI/Send) |
| tabsHeight | 38 | `msg.Height - (methodHeight + 5)` |
| Tabs border box | 38 | `Style.Height(tabsHeight)` |
| Tabs inner content | 34 | `tabsHeight - 4` (border 2 + labels 2) |
| Response preview | 38 | `tabsHeight` |

### Width Chain (terminal at 194 columns)

| Component | Columns | Formula |
|-----------|---------|---------|
| Terminal | 194 | `msg.Width` |
| RequestTabs outer | 97 | `msg.Width / 2` |
| RequestTabs inner | 95 | `tabsWidth - 2` (border) |
| ResponsePreview outer | 97 | `msg.Width - tabsWidth` |
| ResponsePreview inner | 95 | `respWidth - 2` (border) |
| URI field width | dynamic | `msg.Width - (methodWidth + serverWidth + sendWidth + 3)` |

## Example: Fixing Overflow Step by Step

When a user reports "borders get resized after adding many params":

1. **Read the element tree** → tabs contain scrollable → params/header content
2. **Find the `WindowSizeMsg` chain** → `editor.go` computes `tabsHeight`, sends `childrenMsg`
3. **Measure overhead** → 4 lines (2 border + 2 labels)
4. **Compare with forward** → `childrenMsg.Height = tabsHeight` (wrong, should be `tabsHeight - 4`)
5. **Fix** → subtract overhead from childrenMsg height
6. **Validate** → `DebugRenderPane(194, 46)` → count lines, assert bottom border at expected position
