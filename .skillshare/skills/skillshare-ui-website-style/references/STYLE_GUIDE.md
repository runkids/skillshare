# Skillshare Web Dashboard — Style Guide

The rules behind the dashboard, and why they exist. Token names, the `ss-*` class list and component APIs are in `../SKILL.md`; values are in `ui/src/components.css`.

Reference pages: `ui/src/pages/TargetsPage.tsx`, `ui/src/pages/ResourcesPage.tsx`.

---

## 1. Philosophy

- **One layout, two skins.** Clean and Playful share the same markup, spacing and information hierarchy. A style changes how things are drawn, never what is on the page or where. The Dashboard pin board is the single exception, and it is marked with `.ss-only-playful`.
- **Flat.** One framed container per group of things (`.ss-list`, `.ss-box`). No card inside a card, no page made of stacked cards.
- **Typography before colour.** Hierarchy comes from size, weight and the three ink levels. Colour is for status and for resource kind, nothing else.
- **It should look made by a person.** Real tool logos (`AgentIcon`), one deliberate icon per concept, no gradient fills, no decorative icon above every heading, no emoji, no rows of identical stat cards.
- **Speak the CLI's language.** sync, target, collect, tracked, merge. The dashboard is another face of the same tool, so a label should match what the terminal prints.
- **Every element earns its place.** If removing it does not hurt comprehension, remove it.

---

## 2. Clean and Playful

| | Clean | Playful (default) |
|-|-------|-------------------|
| Feel | Quiet, professional | Notebook, stationery |
| Headings | System font | Kalam |
| Borders | 1px hairline | 2px ink line |
| Shadows | Soft, barely there | Hard offset, no blur |
| Separators | Solid hairline | Dashed |
| Primary | Ink | Yellow with an ink border |
| Accents | None | Pastels, highlighter under the page title, dot-grid background |
| Tiles | Plain boxes | Sticky notes with tape |

Playful gets its character from line weight, hard shadows and the heading font. Nothing in the dashboard is rotated or tilted; rotation belongs to the website homepage.

Both styles ship in light and dark. Dark is not an inversion: Playful dark replaces yellow with amber and keeps pastel notes light enough for dark text, so check it rather than assuming.

---

## 3. Colour

| Role | Token | When |
|------|-------|------|
| Primary text | `ink` | Titles, names, values |
| Secondary | `ink-2` | Descriptions, subtitles |
| Tertiary | `ink-3` | Hints, placeholders, column headers, timestamps |
| Good | `ok` | Synced, passed, clean |
| Attention | `warn` | Behind, partial, dirty |
| Problem | `bad` | Failed, blocked, critical |
| Link, focus | `link` (`--accent`) | Links, focus rings, info notes |
| Kind | `--c-skill` `--c-agent` `--c-extra` `--c-mcp` `--c-target` | Resource kind, through `.ss-cat` only |

- One status signal per element: a `.ss-st` dot or a toned `.ss-tag`, not both.
- A kind colour means the same kind on every page. Do not borrow one for decoration.
- Status is never colour alone; `.ss-st` always carries text.

---

## 4. Typography

- Body is the system font at 14px; most UI text is 13px.
- Mono (`font-mono`): resource names, paths, hashes, versions, commands, durations. `.ss-tag` is mono by design.
- Headings: `.ss-h1` for the page title (`PageHeader` does it), `.ss-h2` for section titles. Do not size headings with Tailwind text utilities, or they miss Kalam in Playful.
- Numbers are tabular already (`tnum` on `body`).

---

## 5. Layout

- Root: `.ss-wrap animate-fade-in`. 1080px column, 28px between sections.
- Order: `PageHeader` → tabs or toolbar → notes → content → dialogs.
- Spacing between sections comes from `.ss-wrap`. Spacing inside a section comes from the `ss-*` class. Reach for a Tailwind margin last.
- Toolbar: `flex flex-wrap items-center gap-3`; search first, then `SegmentedControl`, then `Select`.
- Desktop width only. The dashboard is a local tool; narrow layouts are not checked.

---

## 6. Choosing a pattern

### Lists

| Pattern | When |
|---------|------|
| `.ss-list` + `.ss-r` | Default. Uniform items; add `.ss-lh` when columns need labels, `.ss-gh` for groups |
| `.ss-tiles` + `.ss-tile` | The user picks by description rather than by name (Skills cards view) |
| `.ss-kv` | Properties of one thing |
| `.ss-setrow` | Settings: label and help on the left, control on the right |

A row that navigates is `.ss-r link`, and the whole row is the target. Controls inside it (`Checkbox`, `IconButton`, menus) must stop the click from navigating, and need a hit area of 24px or more so a near miss does not open the detail page.

### Status

| Pattern | When |
|---------|------|
| `.ss-st` | State of a row or item |
| `.ss-tag` | A label or category (mode, kind, source), neutral unless the label itself is a status |
| `.ss-sev` | Audit severity only |
| `.ss-note` | A message about the whole page or section |

### Numbers

One to three numbers go inline: `.ss-cnt` beside a heading, or a sentence in the subtitle. `.ss-counts` is for the Dashboard and Audit overviews only.

### Buttons

| Variant | When |
|---------|------|
| `primary` | The one main action of the page or dialog |
| `secondary` | Everything else, including reversible removals (uninstall, remove) |
| `danger` | Permanent destruction: empty trash, clear log, delete forever |
| `ghost` | Cancel, reset, low-emphasis toolbar actions |
| `link` | Inline action inside text or a row |

One primary per view. A destructive action always goes through `ConfirmDialog`.

### Dialogs

| Component | When |
|-----------|------|
| `ConfirmDialog` | Yes or no, especially destructive |
| `DialogShell` | Forms, multi-step flows, previews. Use `padding="none"` with `.dh` / `.db` / `.df` so header, scrolling body and footer match every other dialog |

### Empty, loading, error

`EmptyState` with an action that fixes the emptiness; `PageSkeleton` while the first query is pending; `.ss-note bad` for a failed query; a toast for the result of an action.

---

## 7. Motion

| Context | Value |
|---------|-------|
| Page entry | `animate-fade-in` |
| Dialog, dropdown | `animate-dialog-in`, `animate-dropdown-in` |
| Hover and press | Built into `.ss-btn`, `.ss-ib`, `.ss-r.link`, `.ss-counts`. Add none |
| Style or mode switch | `html.theme-transitioning`, applied by `ThemeContext` |

Motion confirms an interaction. Nothing animates on its own except the skeleton shimmer and the spinner.

---

## 8. Accessibility

| Concern | Requirement |
|---------|-------------|
| Focus | The `:focus-visible` outline is built into the `ss-*` controls. Do not remove it; a custom control needs the same outline |
| Target size | 24px minimum (WCAG 2.2 SC 2.5.8). Grow the hit area with `::after` rather than the visual box |
| Contrast | 4.5:1 for text, in all four looks |
| Icon-only buttons | `IconButton` with `label`, or `aria-label` |
| Dialogs | `DialogShell` supplies `role="dialog"`, `aria-modal`, focus trap, Escape and scroll lock. Pass `ariaLabel` |
| Tabs, segments | `aria-current` on the active `.ss-tabs` item; `SegmentedControl` sets `aria-pressed` |
| Form fields | `Input`, `Textarea` and `Select` tie the label to the control. A bare `.ss-inp` needs its own `label` or `aria-label` |
| Hidden checkbox | `sr-only ss-chk-input` directly before `.ss-chk`, so the focus ring lands on the visible box |

---

## 9. Keyboard

Defined in `ui/src/hooks/useGlobalShortcuts.ts`. `SHORTCUT_ENTRIES` feeds the help modal, so adding an entry there is what registers a new shortcut in the UI.

| Key | Action |
|-----|--------|
| `?` | Shortcut help |
| `/` | Focus the page's search input |
| `r` | Refresh, when the page provides a handler |
| `g` then `d` `s` `t` `l` `a` `u` | Dashboard, Skills, Targets, Log, Audit, Updates |
| `Mod+S` | Sync, except inside the code editor, where it saves |

- Single-key shortcuts do not fire while focus is in an input, textarea, select or contenteditable.
- The `g` chord resets after 500ms.
- Never take a browser shortcut. `Mod+S` is the only modifier combination claimed.
- Dialogs close on Escape. `Select` supports arrows, Enter, Space and Escape.

---

## 10. Anti-patterns

The full table is at the end of `../SKILL.md`. The ones that come up most:

- Writing a page with Tailwind colour utilities and legacy token names instead of `ss-*` classes. It renders, but it misses Playful borders and shadows and drifts from every other page.
- `space-y-*` on the page root instead of `.ss-wrap`.
- A Tailwind margin on an element that `.ss-wrap` already spaces. The utility wins over `@layer components` and the gap doubles.
- A hardcoded colour that looks right in the one look you happened to test.
