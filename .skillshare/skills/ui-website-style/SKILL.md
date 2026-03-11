---
name: skillshare-ui-website-style
description: >-
  Skillshare frontend design system for the React dashboard (ui/) and Docusaurus
  website (website/). Use this skill whenever you: build or modify a dashboard page
  or component in ui/src/, style or layout website pages or custom CSS in website/,
  create new React components for the dashboard, add pages to the dashboard, fix
  visual bugs in either frontend, or need to know which design tokens, components,
  or patterns to use. This skill covers color tokens, typography, component API,
  page structure, accessibility, keyboard shortcuts, animations, and anti-patterns
  for both frontends. Even if the user just says "fix the styling" or "add a card",
  use this skill to ensure consistency.
targets: [claude, codex, cursor]
---

Enforce the skillshare design system across the two frontends. $ARGUMENTS is the file or area being worked on.

**Companion skill**: For UX design decisions beyond token/component usage — layout strategy, information hierarchy, interaction patterns, micro-copy, or when designing a new page from scratch — also invoke `/ui-ux-pro-max` for higher-level design guidance. This skill handles *what components and tokens to use*; `/ui-ux-pro-max` handles *how to design the experience*.

The project has **two distinct design systems** sharing semantic color names but with different visual treatments:

| Aspect | UI Dashboard (`ui/`) | Website (`website/`) |
|--------|---------------------|----------------------|
| Stack | React 19 + Vite + Tailwind CSS v4 | Docusaurus 3 + custom CSS |
| Font body | DM Sans | IBM Plex Sans |
| Font heading | DM Sans | Inter |
| Font mono | SFMono-Regular, Menlo | JetBrains Mono |
| Border-radius | Clean: `4px`/`8px`/`12px`/pill | Wobbly: `255px 15px 225px 15px / ...` |
| Shadows | Subtle blur: `0 1px 3px rgba(...)` | Hard offset: `4px 4px 0px 0px #2d2d2d` |
| Background | Flat `#f7f6f3` | Dot grid on `#fdfbf7` |
| Philosophy | oMLX.ai-inspired minimal | Hand-drawn sketchy organic |

---

## UI Dashboard (`ui/`)

Reference implementation: `ui/src/pages/LogPage.tsx`

> For the full human-readable style guide (design philosophy, visual rules, anti-patterns), see `references/STYLE_GUIDE.md` bundled with this skill.

### Design Tokens

Defined in `ui/src/index.css` (@theme) and `ui/src/design.ts`:

```ts
// ui/src/design.ts
import { radius, shadows, palette } from '../design';

radius.sm   // '4px'  — badges, chips
radius.md   // '8px'  — cards, containers
radius.lg   // '12px' — modals, panels
radius.btn  // '9999px' — pill buttons (oMLX style)
radius.full // '9999px' — avatars

shadows.sm / .md / .lg / .hover / .active / .accent / .blue
palette.accent / .info / .success / .warning / .danger
```

### Color Tokens (Tailwind classes)

| Role | Class | When |
|------|-------|------|
| Primary text | `text-pencil` | Titles, names, values |
| Secondary | `text-pencil-light` | Descriptions, timestamps, labels |
| Tertiary | `text-muted-dark` | Hints, placeholders |
| Success | `text-success` | Passed, synced, clean |
| Warning | `text-warning` | Dirty, behind, partial |
| Danger | `text-danger` | Failed, blocked, critical |
| Info / Blue | `text-blue` | Links, info badges, repo URLs |
| Background | `bg-surface` | Cards, inputs |
| Page bg | `bg-paper` | Root background |
| Borders | `border-muted` | Default borders |

**Rule**: Max one accent color per visual region. If a card has a colored left-border stripe, don't add a colored badge for the same status inside it.

### Typography

- Body: DM Sans (via `--font-hand`)
- `font-mono`: timestamps, file paths, durations, code, hashes
- `uppercase tracking-wider`: command names, stat labels
- Size: `text-2xl` > `text-xl` > `text-base` > `text-sm` > `text-xs`

### Page Structure (mandatory order)

Every page follows this layout:

```tsx
<div className="space-y-5 animate-fade-in">
  <PageHeader icon={<Icon />} title="..." subtitle="..." actions={<>...</>} />

  {/* Toolbar: tabs + filters */}
  <div className="flex flex-wrap items-end gap-3">
    <SegmentedControl ... />
    <Select ... />
  </div>

  {/* Summary line or stat cards */}
  <SummaryLine ... />

  {/* Content: table, card list, or card grid */}
  {empty ? <EmptyState ... /> : <ContentArea />}

  {/* Dialogs (rendered at bottom, portal via DialogShell) */}
  <ConfirmDialog ... />
</div>
```

### Component Library

| Component | File | API |
|-----------|------|-----|
| `Card` | `Card.tsx` | `variant="default\|accent\|outlined"`, `hover`, `overflow` |
| `Button` | `Button.tsx` | `variant="primary\|secondary\|danger\|ghost\|link"`, `size="sm\|md\|lg"` |
| `Badge` | `Badge.tsx` | `variant="default\|success\|warning\|danger\|info\|accent"` |
| `PageHeader` | `PageHeader.tsx` | `icon`, `title`, `subtitle?`, `actions?` |
| `EmptyState` | `EmptyState.tsx` | `icon` (LucideIcon), `title`, `description?`, `action?` |
| `ConfirmDialog` | `ConfirmDialog.tsx` | `open`, `onConfirm`, `onCancel`, `title`, `message`, `variant="default\|danger"` |
| `DialogShell` | `DialogShell.tsx` | `open`, `onClose`, `maxWidth`, `preventClose` |
| `Input` | `Input.tsx` | `label?` + standard input props |
| `Textarea` | `Input.tsx` | `label?` + standard textarea props |
| `Select` | `Input.tsx` | `label?`, `value`, `onChange`, `options[]`, `size="sm\|md"` |
| `Checkbox` | `Input.tsx` | `label`, `checked`, `onChange` |
| `SegmentedControl` | `SegmentedControl.tsx` | `value`, `onChange`, `options[]`, `connected?`, `colorFn?` |
| `Pagination` | `Pagination.tsx` | `page`, `totalPages`, `onPageChange`, `rangeText?`, `pageSize?` |
| `StatusBadge` | `StatusBadge.tsx` | Status display |
| `Skeleton` / `PageSkeleton` | `Skeleton.tsx` | Loading states |
| `Toast` / `useToast` | `Toast.tsx` | `toast(message, 'success'\|'error')` |
| `FilterTagInput` | `FilterTagInput.tsx` | Tag-based filter input |
| `IconButton` | `IconButton.tsx` | Icon-only button with `aria-label` |

### Stats Patterns (choose one)

| Pattern | When |
|---------|------|
| **Inline summary text** | 1-3 stats: `"42 ops · 3 errors · last: 2m ago"` |
| **Stat card grid** | Dashboard overview KPIs only |

### Status Patterns (choose one per element)

| Pattern | Markup | When |
|---------|--------|------|
| Colored dot | `w-2 h-2 rounded-full` | Table rows, list items |
| Badge | `<Badge variant="...">` | Standalone labels |
| Left-border stripe | `border-l-2 border-{color}` | Audit finding cards only |

### List Patterns

| Pattern | When |
|---------|------|
| Table | Uniform rows, sortable columns, many items |
| Card list (vertical) | Mixed content per row, expandable |
| Card grid | Visual overview, few fields per item |

### Button Variants

| Variant | When |
|---------|------|
| `danger` | Permanent destructive: clear log, empty trash, delete forever |
| `secondary` | Reversible removal: uninstall, remove, restore |
| `ghost` | Cancel, clear filter, reset |
| `primary` | Positive action: save, sync, install, run |

### Separator

Always: `border-dashed border-pencil-light/30` (unified opacity, not `/20` or `/40`)

### Animations

| Context | Value |
|---------|-------|
| Card rotation | `rotate(+-0.15deg)` via `:nth-child(odd/even)` |
| Standalone accent | Max `rotate(+-0.3deg)` |
| Hover | `transition-all duration-150` |
| Page entry | `animate-fade-in` (0.2s ease-out) |

### Keyboard Shortcuts

Registered in `KeyboardShortcutsModal.tsx` and `useGlobalShortcuts.ts`:
- `?` — shortcuts modal
- `/` — focus search
- `g d/s/t/l` — go to Dashboard/Skills/Targets/Log
- `r` — refresh page
- Only fire when no `<input>` / `<textarea>` focused
- Chord timeout: 500ms
- New shortcuts must be added to `KeyboardShortcutsModal`
- Never override browser-native shortcuts (`Cmd+C`, etc.)

### Accessibility

| Concern | Requirement |
|---------|-------------|
| Focus ring | `focus:ring-2 focus:ring-blue/20` |
| Touch target | Min 44x44px |
| Color contrast | 4.5:1 (WCAG AA) |
| Icon buttons | `aria-label` required |
| Modals | `role="dialog"` + `aria-modal="true"` + `useFocusTrap` |
| Select | `role="combobox"` + `aria-expanded` + `role="listbox/option"` |

### Card Overflow Gotcha

`Card.tsx` has `overflow-hidden` by default for border-radius clipping. Absolute-positioned children (dropdowns, tooltips) get clipped. Fix: pass `overflow` prop or add `className="!overflow-visible"`.

### Data Fetching Pattern

Pages use `@tanstack/react-query`:
```tsx
const { data, isPending } = useQuery({
  queryKey: queryKeys.someKey(...),
  queryFn: () => api.someEndpoint(...),
  staleTime: staleTimes.someCategory,
});
```
- Query keys: `ui/src/lib/queryKeys.ts`
- API client: `ui/src/api/client.ts`
- App context: `ui/src/context/AppContext.tsx` provides `{ isProjectMode, projectRoot }`

---

## Website (`website/`)

Docusaurus with hand-drawn "sketchy organic" design in `website/src/css/custom.css`.

### Key Differences from UI Dashboard

- **Wobbly borders**: `border-radius: var(--radius-wobbly)` (not clean px values)
- **Hard shadows**: `4px 4px 0px 0px #2d2d2d` (no blur)
- **Dot grid bg**: `radial-gradient(var(--color-muted) 1px, transparent 1px)` on body
- **Post-it yellow**: `--color-postit: #fff9c4` for highlights
- **Dashed borders everywhere**: navbar, sidebar, tables, pagination, code blocks
- **Button hover**: `transform: translate(2px, 2px)` + shadow shrinks (press-down feel)
- **Links**: wavy underline (`text-decoration-style: wavy`)
- **Dark mode**: amber/gold primary (`#e8a84c`) instead of blue

### CSS Variables

See `website/src/css/custom.css` for full list. Key additions beyond shared palette:
- `--radius-wobbly[-sm|-md|-btn]` — hand-drawn border-radius
- `--color-postit[-dark]` — yellow highlight
- `--shadow-md: 4px 4px 0px 0px #2d2d2d` — hard offset
- `--card-bg`, `--card-border`, `--install-bg` — component-specific

### Docusaurus Classes

- `.button--primary` — green bg, pencil border, hard shadow
- `.button--secondary` — dashed border, no fill
- `.markdown h2` — dashed bottom border
- `.admonition` — wobbly border-radius, left stripe
- `.menu__link--active` — post-it yellow background
- `.target-badge` — wobbly border for target grid
- Code blocks get `box-shadow: 4px 4px 0px 0px #101827`

### Homepage Components (`website/src/pages/index.tsx`)

- `CARD_ROTATIONS` array for random hand-drawn tilt
- `WavyDivider` — SVG dashed wavy line between sections
- `InstallTabs` — tabbed install commands with copy button
- Hero has hand-drawn SVG connector + underline

---

## Anti-Patterns (Both Systems)

| Don't | Do Instead |
|-------|------------|
| Emojis as status icons | Colored dots, badges, or semantic icons |
| Stat cards for 1-3 values | Inline summary text |
| Stripe + badge for same status | Pick one per element |
| Mixed separator opacity (`/20`, `/40`) | Always `border-pencil-light/30` |
| `window.confirm()` | `<ConfirmDialog>` component |
| Custom empty-state markup | `<EmptyState>` component |
| Inline styles for design tokens | Use Tailwind classes or `design.ts` exports |
| `overflow-visible` on Card without `overflow` prop | Pass `overflow` prop to Card |
| Dropdowns inside `flex-wrap` title rows | Put dropdowns on their own row |

## Checklist Before Submitting

- [ ] Uses existing components (Card, Badge, Button, etc.) — no custom markup for solved patterns
- [ ] Page follows PageHeader → Toolbar → Summary → Content structure
- [ ] Color tokens from Tailwind (`text-pencil`, not hardcoded `#141312`)
- [ ] Separator is `border-dashed border-pencil-light/30`
- [ ] Empty states use `<EmptyState>`
- [ ] Destructive actions use `<ConfirmDialog>`
- [ ] All icon buttons have `aria-label`
- [ ] New shortcuts registered in `KeyboardShortcutsModal`
- [ ] `animate-fade-in` on root container
- [ ] Dark mode works (check both themes)
