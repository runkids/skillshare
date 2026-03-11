# Skillshare Web Dashboard — Unified Style Guide

Reference implementation: `src/pages/LogPage.tsx`

---

## 1. Design Philosophy

- **skillshare-inspired**: whitespace over decoration, typography hierarchy over color coding
- **"Less is more"**: every visual element must earn its place — if removing it doesn't hurt comprehension, remove it
- LogPage redesign is the canonical reference: clean summary text, dashed separators, minimal color, monospace where data demands it

---

## 2. Color Usage Rules

| Role | Token | When |
|------|-------|------|
| Primary text | `text-pencil` | Titles, names, values |
| Secondary | `text-pencil-light` | Descriptions, timestamps, labels |
| Tertiary | `text-muted-dark` | Hints, placeholder text |
| Success | `text-success` | Passed, synced, clean |
| Warning | `text-warning` | Dirty, behind, partial |
| Danger | `text-danger` | Failed, blocked, critical |
| Info / Blue | `text-blue` | Links, info badge, repo URLs |

**Rule**: Max one accent color per visual region. Don't double up — if a row has a colored dot, skip the colored badge (or vice versa).

---

## 3. Typography

- **Body font**: DM Sans
- **Monospace** (`font-mono`): timestamps, file paths, durations, code snippets, hashes
- **Uppercase** (`uppercase tracking-wider`): command names, stat labels
- **Size hierarchy**: `text-2xl` > `text-xl` > `text-base` > `text-sm` > `text-xs`

---

## 4. Layout Patterns

- **Root container**: `space-y-6 animate-fade-in` (unified across all pages)
- **Separator**: `border-dashed border-pencil-light/30` (unified — replaces any `/20`, `/40` variants)
- **Page structure**:
  1. `PageHeader` — title + optional description
  2. Toolbar — filters, search, actions
  3. Summary — inline stats or stat card grid
  4. Content — table, card list, or card grid

---

## 5. Component Usage Guidelines

### 5.1 Stats — 2 patterns only

| Pattern | When | Example |
|---------|------|---------|
| **Inline summary text** | 1–3 stats | LogPage: "42 operations · 3 errors · last: 2 min ago" |
| **Stat card grid** | Dashboard overview only | Dashboard top-level KPIs |

Do not use stat cards for simple counts — prefer inline summary text.

### 5.2 Status — 3 patterns only

| Pattern | Markup | When |
|---------|--------|------|
| **Colored dot** | `w-2 h-2 rounded-full` | Table rows, list items, audit findings/rules |
| **Badge** | `<Badge variant="...">` | Standalone labels |

Never use left-border colored stripes (`border-l-*`) for status or emphasis — always use colored dots or badges instead.

### 5.3 Lists — choose by data shape

| Pattern | When |
|---------|------|
| **Table** | Uniform rows, sortable columns, many items |
| **Card list** (vertical stack) | Mixed content per row, expandable detail |
| **Card grid** | Visual overview, few fields per item |

### 5.4 Filters

- **Layout**: `flex flex-wrap items-end gap-3`
- **Order**: `[SegmentedControl tabs]` `[Select filters]` `[time range]`
- Keep filters on a single row when possible; they wrap naturally on narrow viewports.

### 5.5 Empty States

Always use the `<EmptyState>` component. Never write custom empty-state markup inline.

### 5.6 Buttons

| Variant | When |
|---------|------|
| `danger` | Permanent destructive action only (clear log, empty trash, delete forever) |
| `secondary` | Reversible removal (uninstall, remove, restore) |
| `ghost` | Cancel, clear filter, reset |
| `primary` | Positive action (save, sync, install, run) |

### 5.7 Dialogs

| Component | When |
|-----------|------|
| `ConfirmDialog` | Destructive or irreversible actions (delete, clear, overwrite) |
| `DialogShell` | Forms, multi-step flows, informational modals |

---

## 6. Animation

| Context | Value |
|---------|-------|
| Card rotation | Alternating `rotate(±0.15deg)` via `:nth-child(odd/even)` |
| Standalone accent card | Max `rotate(±0.3deg)` |
| Hover transition | `transition-all duration-150` |
| Page entry | `animate-fade-in` (0.2s ease-out) |

Keep animations subtle. They should feel hand-drawn, not bouncy.

---

## 7. Accessibility

| Concern | Requirement |
|---------|-------------|
| Focus ring | `focus:ring-2 focus:ring-blue/20` |
| Touch target | Min 44x44px interactive area |
| Color contrast | 4.5:1 minimum (WCAG AA) |
| Icon buttons | `aria-label` required on every icon-only button |

---

## 8. Keyboard & Shortcuts UX

### Standard interactions

| Key | Context | Action |
|-----|---------|--------|
| `Enter` | Search / Install input | Submit form |
| `Escape` | Dialog / Modal | Close |
| `Escape` | Select dropdown | Collapse |
| `Up / Down` | Select dropdown | Navigate options |
| `Space / Enter` | Select dropdown | Select option |
| `Tab / Shift+Tab` | Inside modal | Cycle focus (`useFocusTrap`) |
| `Backspace` | FilterTagInput (empty) | Delete last tag |

### Global shortcuts

| Key | Action |
|-----|--------|
| `?` | Open keyboard shortcuts modal |
| `/` | Focus search input (if page has one) |
| `g d` | Go to Dashboard |
| `g s` | Go to Skills |
| `g t` | Go to Targets |
| `g l` | Go to Log |
| `r` | Refresh (trigger page refresh action) |

### Rules

- Shortcuts only trigger when **no** `<input>` or `<textarea>` is focused.
- Chord shortcuts (`g` then `d`) use a **500ms timeout** reset between keys.
- All shortcut-enabled elements must have the `aria-keyshortcuts` attribute.
- New shortcuts must be registered in `KeyboardShortcutsModal`.
- **Never** override browser-native shortcuts (`Cmd+C`, `Cmd+V`, `Cmd+A`, `Cmd+Z`, etc.).

---

## 9. Anti-Patterns

| Don't | Do instead |
|-------|------------|
| Emojis as status icons | Use colored dots, badges, or semantic icons |
| Stat cards for 1–3 values | Inline summary text |
| Left-border colored stripes (`border-l-*`) | Colored dots or badges — never use left stripes |
| Stripe + badge for same status | Pick one per element |
| Mixed separator opacity (`/20`, `/40`) | Always `border-pencil-light/30` |
| `window.confirm()` | `<ConfirmDialog>` component |
| Custom empty-state markup | `<EmptyState>` component |
