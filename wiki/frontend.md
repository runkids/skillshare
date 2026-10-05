# Frontend Development

Use when changing React components, pages, CSS, responsive layouts, interactions, or visual behavior in the `ui/` dashboard or `website/`.

## Shared Workflow

1. Decide whether the target is the dashboard or public website; they use different visual languages.
2. Search neighboring pages/components and existing tokens/classes before creating anything new.
3. Preserve data, loading, error, empty, keyboard, and accessibility states.
4. Build and test inside the devcontainer, then capture and inspect screenshots for visual changes.
5. Check Clean/Playful × light/dark for the dashboard, and light/dark plus affected breakpoints for the website.

## Dashboard Sources of Truth

| Concern | Source |
|---|---|
| Tokens and `ss-*` classes | `ui/src/components.css` |
| Tailwind token mapping | `ui/src/index.css` |
| Theme selection | `ui/src/context/ThemeContext.tsx` |
| Shared components | `ui/src/components/` |
| Query keys | `ui/src/lib/queryKeys.ts` |
| Queries shared across pages (overview, MCP, skills, targets, diff) | `ui/src/hooks/useSharedQueries.ts` |
| API client | `ui/src/api/client.ts` |
| `?kind=` query strings | `kindQuery()` in `ui/src/api/http.ts` |
| Translation strings | `ui/src/i18n/` |

The dashboard supports Clean and Playful styles plus light, dark, and system modes. New UI must use CSS variables, token utilities, and existing `ss-*` classes. Never hardcode colors, radii, shadows, or fonts. Scope Playful-only pastels to the Playful theme.

`python3 scripts/design/tokens.py` exports the Clean tokens from `components.css` in Design System `tokens.json` format, for mockup tools that read a design system instead of the CSS. A new Clean color variable needs a usage note in that script, or the export (and its test) fails.

## Dashboard Composition

- Use `ss-wrap animate-fade-in` on the page root and place `PageHeader` first.
- Prefer `ss-list` and `ss-r` for lists, `ss-note bad` for errors, and `EmptyState` for empty states.
- Use existing shared components such as `Button`, `IconButton`, `Card`, `DialogShell`, `ConfirmDialog`, `Input`, `Select`, `Checkbox`, `Pagination`, and `Tooltip`.
- Route destructive actions through `ConfirmDialog`; never use `window.confirm()`.
- Use `lucide-react` icons. Use `AgentIcon` for real agents/tools rather than emoji or generic icons.
- Icon-only controls require accessible names, and interactive targets must be at least 24 px.
- Route user-visible strings through `useT()`. Preserve existing English technical labels for status, mode, and kind values.
- Use existing TanStack Query keys, client helpers, and stale-time patterns. Invalidate the correct queries after mutations.

Do not run an unconfigured Prettier in `ui/` because it creates broad unrelated diffs. When Tailwind utilities conflict with the `ss-*` component layer, inspect the cascade before adding more complex selectors.

The config editor's Beautify and Save actions organize top-level YAML sections: sources and default settings, targets, skills, agents, extras, MCP, plugins, hooks, other settings, ignore, and audit. Sections have blank lines between them; nested mappings and lists retain their order, and unknown keys remain in their original relative order at the end. Comments and anchors are preserved; the `yaml-language-server` schema directive stays at the top of the document, including when repairing a previously misplaced directive. If reordering would change an alias reference, the original section order is retained.

Git Sync shows both local and remote commit counts when histories diverge and offers **Pull and merge** before pushing. With uncommitted changes and known remote updates, **Commit and pull** saves local changes before merging. A `pull_conflict` response opens a whole-file version comparison: users must choose local or remote for every conflict before applying. Cancel leaves the repository unchanged. The API retries the merge and validates the reviewed local/remote revision hashes; stale choices require a fresh review. Metadata conflicts still merge automatically. Binary files and files over 16 KiB have no text preview. After merging and syncing, users push separately. **Sync both ways** chains commit (when dirty), pull, and push in one action, like `push --pull`, after a confirmation listing each step with the current counts (dry run previews without asking); a conflict stops it at the review before pushing. It pulls with `alwaysSync` so targets sync even when nothing new arrives, and on `remote_empty` it pushes first, then pulls to sync. First-pull `merge_failed` handling and its confirmed force replacement remain separate from this workflow. See `ai_docs/tests/git_ui_conflicts_runbook.md` for verification and an isolated preview fixture.

## Localization Verification

- Check rendered non-English UI, not just locale-key parity. Skillshare-owned API reasons, notes, warnings and guidance are UI copy too; do not display their English messages directly merely because they originate in Go.
- Trace each changed message from its producer to every visible consumer, including file rows, Details, tooltips and dialogs. Reuse existing translation mappings; preserve paths, names, identifiers, native terms and unknown external diagnostics.
- Inspect representative populated states in the requested locale, including read-only/managed rows. A translated heading or a successful build does not establish that backend guidance is translated.
- Copy/translation-only changes normally need existing locale/placeholder checks and rendered verification, not new tests repeating dictionary values or mocked translations. Add coverage only for a concrete behavioral failure that existing checks cannot detect; follow the `testing` topic's test-value rules.

## Website Boundary

This topic also loads `website/AGENTS.md` for website-specific commands, structure, and deployment rules. Additional boundaries:

- Documentation chrome uses the clean treatment. The hand-drawn string-board treatment belongs only to the homepage and feature map.
- The website keeps its existing `--color-pencil` and `--color-paper` token names. Do not import the dashboard's `--ink` and `--bg` naming.
- Mermaid uses the global configuration. Keep labels short, use `<br/>` for line breaks, and verify rendered output with a screenshot.
- Load `documentation` for content changes. Verify public behavior against Go source before writing it.

## Verification

Run all package commands inside the devcontainer. Select checks according to scope:

- Dashboard: targeted Vitest, `pnpm run lint`, `pnpm run build`, and visual screenshots.
- Website: `pnpm run typecheck` and `pnpm run build`; the build validates broken links.
- API-connected UI: compare `internal/server/server.go` routes, handler tests, and client types.

Report the themes, modes, and viewport sizes inspected, plus any visual verification that could not be performed.
