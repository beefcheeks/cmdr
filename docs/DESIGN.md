# Design System

**Theme: "Dark Bourbon"** — warm, cozy dark UI inspired by the cmd+r brand.

Reference mockup: [theme-mockup.html](theme-mockup.html) (open in browser to preview)

## Palette

All colors defined as Tailwind v4 `@theme` tokens in `web/src/app.css`.

### Bourbon (warm grays)

The foundation. Used for backgrounds, borders, and text hierarchy.

| Token | Hex | Usage |
|---|---|---|
| `bourbon-950` | `#1a1510` | Page background |
| `bourbon-900` | `#241e16` | Content panel, card backgrounds |
| `bourbon-800` | `#332a1f` | Borders, dividers (`<hr>`) |
| `bourbon-700` | `#4a3d2e` | Checkbox borders, subtle UI |
| `bourbon-600` | `#655440` | Secondary text, metadata |
| `bourbon-500` | `#897562` | Card labels, muted text |
| `bourbon-400` | `#a89580` | Descriptions |
| `bourbon-300` | `#c4b5a2` | Body text (default) |
| `bourbon-200` | `#ddd2c4` | Emphasized body text |
| `bourbon-100` | `#f0ebe4` | Headings, primary text |

### Cmd Purple (⌘ key accent)

Interactive elements — buttons, tags, highlights.

| Token | Hex | Usage |
|---|---|---|
| `cmd-700` | `#26215C` | Tag backgrounds (`cmd-700/40`) |
| `cmd-500` | `#534AB7` | Button backgrounds |
| `cmd-400` | `#7F77DD` | Tag text, hover states |
| `cmd-300` | `#EEEDFE` | Button text |

### Run Amber (R key accent)

Status, active states, section labels, callout borders.

| Token | Hex | Usage |
|---|---|---|
| `run-700` | `#633806` | Dark amber accents |
| `run-500` | `#EF9F27` | Section labels, callout borders, active nav |
| `run-400` | `#FAC775` | Active nav text, status highlights |

## Typography

### Orbitron (`font-display`)

The command-line aesthetic. Used for all structural/heading text.

> **Important:** Orbitron only renders cleanly at **even pixel sizes** (10px, 12px, 14px, etc.). Odd sizes like 11px or 13px cause subpixel artifacting. Stick to even values.

- **Page headings:** `font-display text-3xl font-bold text-bourbon-100 lowercase`
- **Section labels:** `font-display text-xs font-bold uppercase tracking-widest text-run-500`
- **Card labels:** `font-display text-xs font-bold uppercase tracking-widest text-bourbon-500`
- **Stat values:** `font-display text-3xl font-bold text-bourbon-100`
- **Nav items:** `font-display text-[11px] font-bold uppercase tracking-widest`
- **Buttons:** `font-display text-[11px] font-bold uppercase tracking-widest`

### Space Grotesk (`font-body` / `font-sans`)

The readable workhorse. Set as the default sans-serif font.

- Body text, descriptions, metadata, secondary content.

## Layout

```
┌─ bourbon-950 (page bg) ─────────────────────────┐
│  nav: logo + links                               │
│  ┌─ bourbon-900 (content panel) ────────────────┐│
│  │  rounded-2xl border border-bourbon-800       ││
│  │                                              ││
│  │  ┌─ bourbon-950/30 (card) ──┐ ┌─ card ──┐   ││
│  │  │  border border-bourbon-800│ │         │   ││
│  │  │  rounded-xl              │ │         │   ││
│  │  └──────────────────────────┘ └─────────┘   ││
│  │                                              ││
│  └──────────────────────────────────────────────┘│
└──────────────────────────────────────────────────┘
```

- Outer page: `bourbon-950`
- Content panel: `bg-bourbon-900 rounded-2xl border border-bourbon-800 p-8`
- Cards: `bg-bourbon-950/30 border border-bourbon-800 rounded-xl p-5`
- Stat cards: `bg-bourbon-950/50 border border-bourbon-800 rounded-xl p-5`
- Max width: `max-w-7xl`

## Components

### Section Header

Amber Orbitron label that introduces each content block.

```html
<h2 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mb-4">Section Name</h2>
```

### Stat Card

```html
<div class="bg-bourbon-950/50 border border-bourbon-800 rounded-xl p-5">
  <h3 class="font-display text-[11px] font-bold uppercase tracking-widest text-bourbon-500 mb-2">Label</h3>
  <p class="font-display text-3xl font-bold text-bourbon-100">Value</p>
  <p class="text-sm text-bourbon-600 mt-1">subtitle</p>
</div>
```

### Tag / Badge

Purple pill for labels and categories.

```html
<span class="text-xs font-medium text-cmd-400 bg-cmd-700/40 px-2.5 py-0.5 rounded-full">tag</span>
```

### Callout / Note

Left-bordered block for important information.

```html
<div class="border-l-2 border-run-500 bg-bourbon-950/50 rounded-r-lg px-5 py-4">
  <h3 class="font-display text-xs font-bold uppercase tracking-widest text-run-500 mb-2">Note</h3>
  <p class="text-bourbon-400">Content here.</p>
</div>
```

### List Item (task/todo row)

```html
<div class="flex items-center justify-between bg-bourbon-950/30 border border-bourbon-800 rounded-lg px-5 py-3.5">
  <div class="flex items-center gap-3">
    <div class="w-4 h-4 rounded border-2 border-bourbon-700 shrink-0"></div>
    <span class="text-bourbon-200">Item text</span>
  </div>
  <span class="text-xs font-medium text-cmd-400 bg-cmd-700/40 px-2.5 py-0.5 rounded-full">tag</span>
</div>
```

### Button

Standard text button:

```html
<button class="px-4 py-1.5 font-display text-xs font-bold uppercase tracking-widest
  bg-cmd-500 text-cmd-300 rounded-lg hover:bg-cmd-400 transition-colors cursor-pointer">
  Action
</button>
```

### Chiclet Button (keyboard key style)

Icon-only action button matching the logo's keycap aesthetic. Use for primary actions where possible.

```html
<button class="shrink-0 w-10 h-10 flex items-center justify-center
  rounded-lg bg-cmd-500 text-cmd-300
  border-t border-t-cmd-400/50
  shadow-[0_3px_0_0_var(--color-cmd-700)]
  hover:brightness-110
  active:translate-y-0.5 active:shadow-none
  transition-all cursor-pointer">
  <!-- lucide icon at size={18} -->
</button>
```

Key elements:
- **Top highlight:** `border-t border-t-cmd-400/50` — subtle 1px edge catch
- **Stepped shadow:** `shadow-[0_3px_0_0_var(--color-cmd-700)]` — the dark underside
- **Press effect:** `active:translate-y-0.5 active:shadow-none` — shadow disappears as button pushes down
- Use `opacity-0 group-hover:opacity-100` when the button should appear on row hover

### Divider

```html
<hr class="border-bourbon-800" />
```

## Logo

- **Full logo:** `web/static/cmdr-logo.svg` — two keyboard keys (⌘ purple + R amber) with `+` separator
- **Favicon:** `web/static/favicon.svg` — square version with both keys and "cmd+r" text
