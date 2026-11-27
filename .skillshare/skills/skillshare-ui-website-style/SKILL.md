---
name: skillshare-ui-website-style
description: >-
  Skillshare frontend design system for the React dashboard (ui/) and Docusaurus
  website (website/). Use this skill whenever you: build or modify a dashboard page
  or component in ui/src/, style or layout website pages or custom CSS in website/,
  create new React components for the dashboard, add pages to the dashboard, fix
  visual bugs in either frontend, or need to know which design tokens, ss-* classes,
  components, or patterns to use. Covers the dashboard styles (Clean / Playful, and
  Custom on top of either) in light and dark, design tokens, the ss-* class system, component API, page
  structure, accessibility, keyboard shortcuts, and anti-patterns. Even if the user
  just says "fix the styling" or "add a card", use this skill to ensure consistency.
metadata: 
  targets: [claude, universal]
---

Enforce the skillshare design system across the two frontends. $ARGUMENTS is the file or area being worked on.

Before acting, run `python3 scripts/ai-context.py frontend`. That topic and the CSS/source files it identifies are the source of truth; this skill retains the component catalog and visual QA checklist.

| Aspect | UI Dashboard (`ui/`) | Website (`website/`) |
|--------|---------------------|----------------------|
| Stack | React 19 + Vite + Tailwind CSS v4 | Docusaurus 3 + custom CSS |
| Source of truth | `ui/src/styles/*.css` (tokens + `ss-*` classes), `ui/src/index.css` (Tailwind mapping) | `website/src/css/custom.css` (docs), `website/src/pages/*.module.css` (homepage, features) |
| Looks | Two styles, **Clean** and **Playful**, each in light and dark; **Custom** recolours either | Docs: clean. Homepage: hand-drawn string board |

**This file names tokens and classes and says when to use them. It does not copy their values.** Colours, radii, fonts and shadows change; read them from the CSS when you need one.

---

## UI Dashboard (`ui/`)

Reference pages: `ui/src/pages/TargetsPage.tsx` (list page), `ui/src/pages/HubPage.tsx` (tabs), `ui/src/pages/ResourcesPage.tsx` (list + tiles + bulk toolbar).

> Design rules and the reasoning behind them: `references/STYLE_GUIDE.md`.

### Two styles, two modes

Style and mode are independent, so every screen has four looks.

| Axis | Values | How it is set |
|------|--------|---------------|
| Style | Clean (default), Playful, Custom | `html[data-theme="playful"]`; attribute absent = Clean. Custom adds `html[data-custom]` on either base, and `data-layout="full"` when a Clean base drops the inset panel |
| Mode | light, dark, system | `html.dark` |

Set by `ui/src/context/ThemeContext.tsx`, switched in `ThemePopover.tsx`. `?theme=clean|playful|dark|light` in the URL forces one, which is handy for screenshots.

A third style, Custom, recolours one of the two through `html[data-custom]` and a generated stylesheet (`ui/src/lib/customTheme.ts`). It only changes token values, so a page that stays on tokens works in it too; code that branches on the look reads `baseStyle`, not `style`.

All four looks come from CSS variables alone. A page that uses only tokens and `ss-*` classes gets all four for free; a hardcoded colour, radius or shadow breaks three of them.

- Clean: system font, 1px hairlines, soft shadows, ink-coloured primary button. At desktop width the page sits on an inset panel (`.ss-main`) over `--shell`, which the transparent sidebar shares. Only layers that float over scrolling content are frosted glass (menus, `.ss-hud`, the stuck `.ss-stickbar`); the page, cards and sidebar stay solid, unless a custom theme sets Glass to Everywhere (`data-glass="surfaces"`), which frosts `.ss-box`, `.ss-list`, `.ss-counts`, the sidebar (on the full layout and the phone drawer; on the inset layout it stays clear on the shell), menus and dialogs over soft light pools on `body` (white in dark; in light, pale pools of the accent's hue, or pastels for a black or grey accent). The surfaces paint their glass on `::before` so they are not backdrop roots: a menu inside one still blurs the page.
- Playful: Kalam headings, 1.5px ink borders, hard offset shadows, dashed separators, terracotta primary, pastel accents, dot-grid background, sticky-note tiles. Text on a pastel fill uses `--on-pa`, not `--sel-ink`.
- Style-only markup: `.ss-only-clean` / `.ss-only-playful` (Dashboard shows a count strip in Clean and a pin board in Playful).

### Design tokens

Defined per look in `ui/src/styles/tokens.css`. `ui/src/index.css` exposes them to Tailwind through `@theme inline`, so `text-ink-2`, `bg-surface`, `border-line` all follow the active look.

| Group | Tokens | Use |
|-------|--------|-----|
| Surfaces | `--bg` `--side` `--surface` `--sunken` `--shell` | Page, sidebar, cards and inputs, recessed headers and footers, Clean's desktop frame behind the inset panel |
| Text | `--ink` `--ink-2` `--ink-3` | Primary, secondary, tertiary and placeholder |
| Lines | `--line` `--line-2` `--line-soft` | Frames, control borders, soft dividers |
| Borders | `--sep` `--frame` `--bw` | Whole `border` values: row separator, box frame, control border width |
| Action | `--pri` `--on-pri` `--accent` `--accent-bg` `--sel` `--sel-ink` | Primary button and on or picked controls (checkbox, switch, tab underline, picked option, progress), links and focus, selected nav and menu items. Text stays `--ink` |
| Status | `--ok` `--warn` `--bad`, each with `-bg` | Text or dot colour, plus its tinted background |
| Kind | `--c-skill` `--c-agent` `--c-extra` `--c-mcp` `--c-plugin` `--c-target`, each with `-bg` | Resource-kind colour, used by `.ss-cat` |
| Pastels | `--pa` `--pb` `--pc` `--pd` `--pe` `--on-pa` | **Playful only.** Never reference outside a `[data-theme="playful"]` rule. `--on-pa` is the text on them |
| Type | `--f` `--fh` `--fm` `--h1` `--h2` | Body, heading (Kalam in Playful), mono, heading shorthands |
| Shape | `--r-ctl` `--r-btn` `--r-box` `--r-tag` | Controls, buttons (pill), boxes, tags |
| Derived shape | `--r-sm` `--r-in` `--r-seg` `--r-inner` `--r-float` `--r-dlg` | Icon buttons and nav items, items inside a control or menu, segmented controls, blocks inside content (tables, notes, code), menus/HUD/toasts, dialogs. Clean derives them from `--r-ctl`/`--r-box`, so a custom theme's corners reach every part; Playful sets them literally. Use these instead of a px radius |
| Shadow | `--sh-box` `--sh-btn` `--sh-float` `--sh-dialog` | Boxes, buttons, menus and toasts, dialogs |
| Motion | `--spring` `--spring-bounce` | `linear()` easings for springy transitions: `--spring` critically damped, `--spring-bounce` for a release that carried momentum. Defined on `:root` beside the components that use them |
| Glass | `--glass-menu` `--glass-bar` `--mat` `--mat-menu`; `--gs-*` for Glass: Everywhere (defined in `materials.css`, not here; the dark light's strength `--gs-pool` comes from `customTheme.ts`, whose contrast check counts it) | **Clean only.** Translucent fill and blur for menus and the sticky toolbar (and cards, lists, the sidebar, dialogs, segmented controls, inputs and secondary buttons with `data-glass="surfaces"`); `prefers-reduced-transparency` turns them solid |

Tailwind names: `bg` `side` `surface` `sunken` `ink` `ink-2` `ink-3` `line` `line-2` `line-soft` `sel` `pri` `on-pri` `ok` `warn` `bad` (with `-bg`) and `link` / `link-bg` for `--accent`.

**Legacy names**: `pencil`, `pencil-light`, `paper`, `paper-warm`, `muted`, `muted-dark`, `success`, `warning`, `danger`, `blue`, `info`, `accent` still resolve as aliases for markup not yet migrated. Do not use them in new code. When you touch a line that has one, replace it:

| Legacy | Use |
|--------|-----|
| `text-pencil` | `text-ink` |
| `text-pencil-light` | `text-ink-2` |
| `text-muted-dark` | `text-ink-3` |
| `bg-paper` / `bg-paper-warm` | `bg-bg` / `bg-side` |
| `border-muted` | `border-line` |
| `text-success` / `text-warning` / `text-danger` | `text-ok` / `text-warn` / `text-bad` |
| `text-blue` / `text-info` | `text-link` |

`ui/src/design.ts` (`radius`, `shadows`, `palette`) forwards to the same variables, for inline styles only.

### The `ss-*` classes

In `ui/src/styles/*.css`: `tokens.css` and `base.css` are unlayered, every other file is one `@layer components` block. The cascade follows the import order in `ui/src/index.css`, so moving or reordering rules can change which one wins. Only `index.css` imports these files: CSS imported from a component loads with its lazy route, in whatever order pages are opened. List what exists today:

```bash
grep -oh '\.ss-[a-z0-9-]*' ui/src/styles/*.css | sort -u
```

State and variant are short modifier classes on the same element: `.on` (selected or checked), `.sel` (selected row or tile), `.ok` `.warn` `.bad` `.inf` (tone), `.sm` `.lg` (size).

| Area | Classes |
|------|---------|
| Page | `.ss-wrap` (1080px column, 28px gap), `.ss-pgh` + `.ss-ph` (header, via `PageHeader`), `.ss-crumb`, `.ss-sec` (section heading row; `h2` inside, `.more` link on the right), `.ss-h1` `.ss-h2`, `.ss-hand` (Kalam aside) |
| Shell | `.ss-side` `.ss-wm` `.ss-nvg` `.ss-nv` `.ss-sidefoot` — `Layout.tsx` only |
| Buttons | `.ss-btn` + `.pri` `.ghost` `.dng` + `.sm` `.lg`; `.ss-ib` (30px icon button); `.ss-more` (text link) |
| Forms | `.ss-fld` (label + control + `.hp` help), `.ss-inp` (+ `.area` `.err`, `.k` key hint), `.ss-chk` (+ `.rad`), `.ss-sw` (switch, via `Switch`; `.on`, `.mix` when only some are on), `.ss-tgl` (icon toggle), `.ss-seg` (+ `.ic` icon-only; give it `ref={segSlide}` from `lib/segSlide` so the Clean pill slides) |
| Navigation | `.ss-tabs` (via `Tabs`), `.ss-tabbar` (tabs with controls on the right), `.ss-pager`, `.ss-menu` (+ `.hv` `.dng`, `hr`, `.k`) |
| Lists | `.ss-list` (framed container), `.ss-lh` (column header), `.ss-gh` (group header), `.ss-r` (row; `.link` clickable, `.sel`, `.fold`; `.nm` name, `.nm.m` mono name), `.ss-plain` (rows without side padding), `.ss-split` (tree view box: `.lp` tree, `.dv` divider, `.rp` detail pane), `.ss-tn` (tree row; `.sel`, `.in` inside a selected folder, `.off`; indent with `--d`) |
| Boxes | `.ss-box` (card, via `Card`), `.ss-tiles` + `.ss-tile` (grid; sticky notes in Playful), `.ss-kv` (`dl` key/value), `.ss-setrow` (settings row), `.ss-counts` (stat strip) |
| Status | `.ss-st` (dot + text; `.ok` `.warn` `.bad` `.off`, `.wrap` for long messages), `.ss-tag` (mono label; `.ok` `.warn` `.bad` `.inf`), `.ss-sev` (audit severity; `.c` `.h` `.md` `.l` `.n`), `.ss-cnt` (count) |
| Icons | `.ss-cat` (kind tile; `.skill` `.agent` `.extra` `.mcp` `.plugin` `.target`, tones, `.sm`), `.ss-at` (agent or tool logo; `.lg`), `.ss-stack` (overlapping logos) |
| Theme editor | `.ss-presets` + `.ss-preset` (preset cards; `.on`), `.ss-tint` + `.ss-tint-sw` (background choice), `.ss-swatch` (colour picker inside `.ss-inp`), `.ss-mini` (live preview; `.sd` sidebar, `.pn` page; its parts transition colour, corners and shadow; `data-part` names the parts an option changes, which then get a brief accent wash (text) or glow (shapes)). Used by `CustomThemeSettings` only |
| Feedback | `.ss-note` (+ `.warn` `.bad` `.inf`), `.ss-empty` (via `EmptyState`), `.ss-prog`, `.ss-skel`, `.ss-toast`, `.ss-tip` |
| Overlays | `.ss-scrim` + `.ss-dlg` with `.dh` `.db` `.df` (via `DialogShell`), `.ss-bulk` (selection toolbar), `.ss-hud` (hint while the shortcut modifier is held), `.ss-top` |
| Content | `.ss-prose` (rendered markdown), `.ss-code` (+ `.ln` `.cur`), `.ss-pre`, `.ss-ed` (editor) |
| Dashboard | `.ss-board` `.ss-pin` `.ss-pinnote` `.ss-squig` — Playful pin board |

Tailwind utilities are for layout inside these (`flex`, `gap-*`, `min-w-0`, `w-[92px]`, `truncate`). Colour, border, radius and shadow come from `ss-*` classes or token utilities.

**Cascade gotcha**: the `ss-*` classes sit in `@layer components`, so a Tailwind utility on the same element always wins, whatever the selector specificity. Do not put `mb-*` on something `.ss-wrap` already spaces. To override an `ss-*` property from markup, use the Tailwind important prefix, as in `className="ss-r link !min-h-[56px]"`.

### Page structure

```tsx
<div className="ss-wrap animate-page-in">
  <PageHeader title={t('x.title')} subtitle={t('x.subtitle')} actions={<>...</>} />

  {/* optional: tabs, or tabs with controls on the right */}
  <Tabs aria-label={t('x.title')}>
    <TabLink to="?tab=x" replace on={on}>...</TabLink>
  </Tabs>

  {error && <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>}

  {empty ? (
    <EmptyState icon={SomeIcon} title="..." description="..." action={...} />
  ) : (
    <div className="ss-list">
      <div className="ss-lh">{/* column labels; widths match the row cells */}</div>
      <Link to="..." className="ss-r link">
        <span className="ss-cat skill"><Puzzle size={16} /></span>
        <span className="nm m min-w-0 flex-1 truncate">name</span>
        <span className="w-[92px] shrink-0"><span className="ss-tag">merge</span></span>
        <span className="ss-st ok">synced</span>
      </Link>
    </div>
  )}

  {/* a second section */}
  <section>
    <div className="ss-sec"><h2>Title</h2><span className="ss-cnt">12</span></div>
    <div className="ss-list">...</div>
  </section>

  {/* dialogs last; DialogShell portals to body */}
</div>
```

- `.ss-wrap` spaces its children with a 28px gap. Do not add `space-y-*` or margins between them.
- A page that is one child of a wider layout, without `.ss-wrap`, still gets header spacing from `.ss-pgh`.
- `PageHeader` has no icon (the `icon` prop is deprecated); do not pass it. Use `backTo` for a sub-page of a nav item, `crumbs` for deeper trails, `mono` when the title is a resource name, `actionsOnTitleRow` to start the actions on the title row and stack them at the right (tabs that switch the page's content on top, their picker under them).
- A sub-page reached from a nav item keeps that item lit through `also` in the `Layout.tsx` nav definition (Skills stays active on `/hubs`).

### Components

Shared components in `ui/src/components/` wrap the `ss-*` classes. Use the component when one exists; write the class directly for things that have none (`.ss-list` rows, `.ss-note`, `.ss-st`, `.ss-tag`, `.ss-kv`).

| Component | Renders | API |
|-----------|---------|-----|
| `PageHeader` | `.ss-pgh` `.ss-ph` | `title`, `subtitle?`, `actions?`, `backTo?`, `crumbs?`, `mono?`, `actionsOnTitleRow?` |
| `Button` | `.ss-btn` | `variant="primary\|secondary\|danger\|warning\|ghost\|link"`, `size="xs\|sm\|md\|lg"`, `loading?` |
| `IconButton` | `.ss-ib` | `icon`, `label` (required, becomes `aria-label`), `size`, `variant="ghost\|danger-outline"` |
| `Card` | `.ss-box` | `padding="none\|sm\|md"`, `variant="default\|outlined"`, `hover?`, `overflow?`, `onClick?`. `tilt` and `skillCard` do nothing |
| `Badge` | `.ss-tag` | `variant="default\|success\|warning\|danger\|info"`, `size`, `dot?` |
| `KindBadge` | `.ss-tag` | `kind="skill\|agent"` |
| `EmptyState` | `.ss-empty` | `icon?` (LucideIcon), `title?`, `description?`, `action?`, `tone="bad"` (failed to load), `compact?` (inside a section), `className?`. A `description` alone renders as one line of body text |
| `Switch`, `SwitchMark` | `.ss-sw` | `Switch`: `checked`, `onChange`, `mixed?` + native button props; name it with `aria-label` or `aria-labelledby`. `SwitchMark` (`checked`, `mixed?`) is the look alone, for a larger button that has `role="switch"` itself; it does not dim on `:disabled`, so dim that button |
| `DialogShell` | `.ss-scrim` `.ss-dlg` | `open`, `onClose`, `maxWidth="sm".."7xl"`, `padding`, `preventClose?`, `ariaLabel`. Use `padding="none"` with `.dh` / `.db` / `.df` children for the standard header, body and footer |
| `ConfirmDialog` | `DialogShell` | `open`, `onConfirm`, `onCancel`, `title`, `message`, `variant="default\|danger"`, `loading?`, `wide?` |
| `Input`, `Textarea` | `.ss-fld` `.ss-inp` | `label?`, `size="sm\|md"` + native props. `Input.tsx` re-exports `Checkbox` and `Select` |
| `Select` | `.ss-inp` + `.ss-menu` | `label?`, `value`, `onChange`, `options[]` (`description?`, `triggerLabel?`, `detail?`), `size`, `prefix?`, `quiet` (footer menu without a box), `up`, `search` (type-to-filter) |
| `Checkbox` | `.ss-chk` | `label` (required), `checked`, `onChange`, `indeterminate?`, `hideLabel?` for row selection |
| `SegmentedControl` | `.ss-seg` | `value`, `onChange`, `options[]` (`count?`, `title?` for icon-only), `colorFn?` |
| `Tabs`, `TabLink`, `TabButton` | `.ss-tabs` | `Tabs`: `slideKey?` (shared by rows a route change replaces). `TabLink`: `on`, `count?` + `Link` props. `TabButton`: `on`, `count?`, inside `<Tabs role="tablist">` |
| `Pagination` | `.ss-pager` | `page`, `totalPages`, `onPageChange`, `rangeText?`, `pageSize?` |
| `Tooltip`, `TruncateTip` | `.ss-tip` | `content`, `side`, `delay`, `followCursor?` |
| `Spinner` | lucide `Loader2` | `size="sm\|md\|lg"` |
| `Skeleton`, `PageSkeleton` | shimmer | `variant="text\|card\|circle"` |
| `useToast()` | `.ss-toast` | `toast(message, 'success'\|'error'\|'warning'\|'info', { title? })` |
| `AgentIcon` | real agent logo | For targets and agents. Do not substitute a generic lucide icon |
| `AgentTile` | `.ss-at` + `AgentIcon` | `target?`, `size="sm\|md\|lg"` (frame/logo: 20/11, 24/14, 30/17; a plain `lg` tile in a stack is 24px), `iconSize?`, `fallback?` (drawn without a target) + span props |
| `AgentStack` | `.ss-stack` of `AgentTile` | `targets`, `max?`, `showRest?` (ends with `+N`), `size?`, `iconSize?` (default 13 without a `size`), `tileClassName?` + span props. When tiles differ from each other, write the `.ss-stack` span with `AgentTile` children |
| `CopyButton`, `CodeView`, `CodeEditor`, `MarkdownView` | `.ss-code` `.ss-prose` | Code and markdown display |

Feature folders (`audit/`, `config/`, `git/`, `hub/`, `mcp/`, `plugins/`, `skill-editor/`, `sync/`, `targets/`, `tour/`) hold page-specific pieces.

### Icons

lucide-react only. 14–16px inline, 16px inside `.ss-cat`, 24px in `EmptyState`. Stroke width comes from `--isw`; do not set `strokeWidth`, except the check mark inside `.ss-chk`. One icon per concept across the app: check `Layout.tsx` and neighbouring pages before picking one.

### Data fetching

```tsx
const { data, error, isPending } = useQuery({
  queryKey: queryKeys.someKey,
  queryFn: () => api.someEndpoint(),
  staleTime: staleTimes.someCategory,
});
if (isPending) return <PageSkeleton />;
```

Keys in `ui/src/lib/queryKeys.ts`, client in `ui/src/api/client.ts`, `useAppContext()` gives `{ isProjectMode, projectRoot }`. Every user-visible string goes through `useT()`; status, mode and kind labels (`merge`, `linked`, `skill`) stay in English.

### Verifying a change

Run the dev server inside the devcontainer with the `ui` command, never on the host. There Vite listens on :45173 (`make ui-dev` on a host with Go uses :5173). Screenshot the page in all four looks, at desktop width only: `?theme=clean`, `?theme=playful`, then each in dark. For theme work, add `?style=custom` with a custom theme saved. Look at the screenshots before reporting done. Do not run prettier in `ui/`; it has no config and rewrites whole files.

---

## Website (`website/`)

Two separate treatments share one palette in `website/src/css/custom.css`.

| Area | Files | Look |
|------|-------|------|
| Docs, navbar, footer | `src/css/custom.css` | Clean: pill buttons, solid 1px borders, soft shadows, plain underlined links, no dot grid |
| Homepage, feature map | `src/pages/index.tsx` + `index.module.css`, `features.tsx` + `features.module.css` | Hand-drawn string board: pinned notes, tape, string lines, wobbly radii, hard offset shadows, Kalam accents, slight rotation |

- Fonts: IBM Plex Sans body, Inter headings, JetBrains Mono code. Kalam appears on the homepage only.
- Palette variables keep the `--color-pencil` / `--color-paper` / `--color-postit` names here. That is current for the website; only the dashboard moved to `--ink` / `--bg`.
- `--radius-wobbly*` and the hard `--shadow-*` are defined globally but belong to the homepage modules. Do not apply them to docs chrome.
- Dark mode swaps the blue primary for amber. Check both modes.
- Docs chrome is styled by overriding Docusaurus classes (`.button--primary`, `.menu__link--active`, `.admonition`, `.table-of-contents`, `.target-badge`). Each block in `custom.css` has a titled banner comment; find the block, edit it there.
- Homepage sections are local components in `index.tsx` (`HeroSection`, `StringBoard`, `PinList`, `SyncTerminal`, `InstallTabs`, `FourMovesSection`, `FeatureMapTeaser`, `CtaSection`), styled from the CSS module.
- Mermaid diagrams use the global `handDrawn` config in `docusaurus.config.ts`: labels of one or two lines, `<br/>` for line breaks, and verify with a screenshot.
- Docs are English only. `~` inside an HTML `<code>` in MDX must be written `&#126;`.

---

## Anti-patterns

| Don't | Do instead |
|-------|------------|
| Hardcoded hex, radius, shadow or font in a page | Token utility (`text-ink-2`) or `ss-*` class |
| Legacy names (`text-pencil-light`, `border-muted`, `text-danger`) in new code | `text-ink-2`, `border-line`, `text-bad` |
| Root `<div className="space-y-5">` | `.ss-wrap` |
| Hand-rolled dashed separators | `.ss-r` inside `.ss-list`, or `border: var(--sep)` |
| `<details>`, bare `<ul>` or `<p role="alert">` for app content | `.ss-list` rows, `.ss-note bad` |
| Playful pastels (`--pa`…`--pe`) in a shared rule | Scope to `:root[data-theme="playful"]` |
| Tilted cards | Nothing tilts in the dashboard. Rotation exists only on the website homepage |
| Dot and tag both carrying the same status | One per element: `.ss-st` for state, `.ss-tag` for a label |
| Left colour stripes (`border-l-*`) | `.ss-st`, `.ss-tag` or `.ss-note` |
| Emoji as icons | lucide, or `AgentIcon` for real tools |
| Stat cards for one to three numbers | Inline text, or `.ss-cnt` beside the heading |
| `window.confirm()` | `ConfirmDialog` |
| Custom empty-state markup | `EmptyState` |
| A small checkbox or icon as the only click target in a clickable row | Keep the hit area at 24px or more; `.ss-chk` already does this with `::after` |
| Dropdown inside a `Card` getting clipped | `overflow` prop on `Card` |
| Wording that drifts from the CLI | Use the CLI's terms: sync, target, collect, tracked, merge |

## Checklist

- [ ] Root is `.ss-wrap animate-page-in`; `PageHeader` first, without `icon`
- [ ] No hardcoded colours, radii or shadows; no legacy token names added
- [ ] Shared components used where they exist; lists are `.ss-list` / `.ss-r`
- [ ] Errors are `.ss-note bad`, empty states are `EmptyState`, destructive actions go through `ConfirmDialog`
- [ ] Icon-only buttons have an accessible name; click targets are 24px or more
- [ ] Strings go through `useT()`; new shortcuts are added to `useGlobalShortcuts.ts`
- [ ] Screenshots checked in Clean and Playful, light and dark (and Custom for theme changes)
