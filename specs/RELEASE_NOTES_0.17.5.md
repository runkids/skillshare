# skillshare v0.17.5 Release Notes

Release date: 2026-03-18

## TL;DR

v0.17.5 adds **fail-fast config validation**, **sync safety**, **reverse proxy support**, and a **skill design pattern wizard**:

1. **Config Save Validation** — saving config now validates semantics (source path, sync modes, target paths), returning HTTP 400 for invalid configs instead of silently saving broken configurations
2. **Sync Safety — No Auto-Create** — sync no longer auto-creates missing target directories. A typo like `~/.cusor/skills` now fails immediately instead of silently creating the wrong directory
3. **Dry-run Path Validation** — `sync --dry-run` now detects missing target paths, matching the behavior of a real sync
4. **Config Save → Sync Preview** — after saving config, a banner offers to preview what sync will do via a dry-run modal before committing
5. **Web UI Base Path** — serve the Web UI under a sub-path behind a reverse proxy with `--base-path` or `SKILLSHARE_UI_BASE_PATH`
6. **Skill Design Patterns** — `skillshare new` now offers an interactive wizard with five design pattern templates (tool-wrapper, generator, reviewer, inversion, pipeline) and category tagging

---

## Config Save Validation

### The problem

`PUT /api/config` only validated YAML syntax. Users could save a config with a nonexistent source path, an invalid sync mode like `"invalid"`, or a misspelled target path — and get HTTP 200 OK. The error only appeared later when running sync, with a cryptic filesystem error message.

### Solution

Config save now performs semantic validation after YAML parsing:

- **Source path** must exist and be a directory
- **Sync mode** must be `merge`, `symlink`, or `copy` (both global and per-target)
- **Target paths** must exist and be directories

Fatal validation errors return HTTP 400 with a descriptive message. Non-fatal warnings (e.g., project mode source not created yet) return HTTP 200 with `{ success: true, warnings: [...] }`.

The same validation runs in the CLI: `skillshare sync` validates config before starting, so manually-edited `config.yaml` files get the same protection.

### Design decisions

- **Shared validation** — `config.ValidateConfig()` and `config.ValidateProjectConfig()` are used by both the API handler and CLI sync command, ensuring identical behavior
- **Warnings vs errors** — two-tier severity: errors block the operation (400), warnings let it proceed but surface the issue. This matches the compiler warning/error model
- **Project mode leniency** — built-in target paths (like `.claude/skills`) skip existence checks in project mode because the user can't control whether the tool is installed. Only custom paths with explicit `path:` are validated
- **Path expansion** — `ValidateConfig` handles `~` expansion internally, so callers don't need to pre-process paths

---

## Sync Safety — No Auto-Create

### The problem

When sync encountered a missing target directory, it silently ran `mkdir -p` to create it. This was convenient for first-time setup but dangerous for typos — `~/.cusor/skills` (missing `r`) would silently create the wrong directory, and skills would sync there unnoticed.

### Solution

Sync now fails fast with a clear error when a target directory doesn't exist:

```
Error: target directory does not exist: /home/user/.cusor/skills
```

This applies to:
- All sync modes (merge, copy)
- Symlink-to-merge/copy conversions (the only case where `mkdir` is still used — after removing an existing symlink)
- `--dry-run` mode (previously skipped existence checks entirely)
- Both CLI and API sync endpoints

### Design decisions

- **Symlink conversion is the exception** — when converting from symlink mode to merge/copy, the code removes the directory-level symlink and recreates it as a real directory. This is not auto-creation from scratch — it's a deliberate mode conversion on an already-configured target
- **Dry-run must also validate** — users run dry-run to check what will happen. If dry-run silently ignores a missing path but real sync fails, the dry-run output was misleading
- **Extracted shared helper** — `ensureRealTargetDir()` handles both symlink conversion and existence verification in a single function, used by both merge and copy modes

---

## Config Save → Sync Preview

### The problem

After editing config (adding/removing targets, changing sync mode) or updating `.skillignore`, users had to manually navigate to the Sync page and click Sync to apply changes. This extra step was friction — but running sync automatically on save felt risky because users might not realize what would change.

### Solution

A preview-first flow that gives users visibility before action:

1. Save config or skillignore
2. A banner appears above the editor: **"Config updated — preview what sync will do?"**
3. Click "Preview Sync" to open a modal that auto-runs `sync --dry-run`
4. The modal shows per-target results: which skills will be linked, updated, or pruned (compact badge-count view)
5. Click "Sync Now" to execute, or Cancel to walk away

### Design decisions

- **Inline banner, not toast** — a banner is a better fit because the "save confirmation" and "sync prompt" are different responsibilities. Toasts are for notifications; banners are for suggested follow-up actions
- **Banner above editor** — positioned between the tab selector and the code editor so it's visible without scrolling, and doesn't get covered by the toast notification at the bottom-right
- **Auto-dismiss on edit** — if the user starts editing again, the banner disappears (they're making more changes, so previewing now is premature)
- **Dry-run first, confirm to execute** — two-step confirmation prevents accidental syncs when config isn't quite right
- **Warnings in preview** — the dry-run modal now shows pre-check warnings (empty source, missing targets) as a yellow banner above the results

### Edge cases handled

- **No targets configured** — shows "No targets configured" (distinct from "everything up to date")
- **All targets in sync** — shows "Everything is up to date" and hides the Sync Now button
- **API errors** — shows error message with a Retry button; modal stays open
- **Refresh while open** — a refresh icon in the modal header re-runs dry-run
- **Double-click prevention** — Sync Now button shows loading state and disables during execution; backdrop/Escape are blocked during sync

---

## Web UI — Base Path for Reverse Proxy

### The problem

Organizations that run multiple internal tools behind a reverse proxy (Nginx, Caddy, Traefik) need to serve each tool under a sub-path like `/skillshare/`. Without base path support, API routes, static assets, and client-side navigation all break when the UI is not served from the root.

### Solution

A `--base-path` flag and `SKILLSHARE_UI_BASE_PATH` environment variable let you host the Web UI under any sub-path:

```bash
skillshare ui --base-path /skillshare
```

All API routes, static assets, and React Router navigation automatically adjust to the configured prefix.

### Design decisions

- **Server-side injection** — the base path is injected into the SPA's `index.html` at serve time via a `__BASE_PATH__` placeholder, so no frontend rebuild is needed
- **Environment variable fallback** — `SKILLSHARE_UI_BASE_PATH` is supported alongside the CLI flag, making it easy to configure in Docker Compose or systemd environments
- **Trailing slash normalization** — the server handles both `/skillshare` and `/skillshare/` consistently

---

## Skill Design Patterns — `skillshare new` Wizard

### The problem

`skillshare new` generated a single generic SKILL.md template regardless of what kind of skill you were building. A skill that wraps a library API needs a very different structure from one that runs a multi-step pipeline or reviews code against a checklist. Users had to restructure the template manually every time.

### Solution

`skillshare new` now offers five structural design patterns, each producing a tailored SKILL.md body and recommended directory structure:

| Pattern | What it does | Scaffold dirs |
|---------|-------------|---------------|
| `tool-wrapper` | Teaches agent library/API conventions | `references/` |
| `generator` | Produces formatted output from templates | `assets/`, `references/` |
| `reviewer` | Scores/audits against a checklist | `references/` |
| `inversion` | Agent interviews user before acting | `assets/` |
| `pipeline` | Multi-step workflow with checkpoints | `references/`, `assets/`, `scripts/` |

Without flags, an interactive TUI wizard guides through pattern → category → scaffold directory selection. With `-P <pattern>`, it skips the wizard and auto-creates scaffold directories.

### Usage patterns

```bash
# Interactive wizard (Esc = back to previous step)
skillshare new my-skill

# Direct pattern selection
skillshare new my-reviewer -P reviewer

# Plain template (previous behavior)
skillshare new my-skill -P none
```

### Design decisions

- **Patterns and categories are separate concerns** — patterns describe *how* a skill is structured (workflow), categories describe *what* it's for (domain). Both are optional frontmatter fields (`pattern:`, `category:`)
- **Wizard back-navigation** — Esc at category goes back to pattern, Esc at scaffold goes back to category. Breadcrumbs in the help bar footer show previous selections
- **`-P` auto-scaffolds** — when using the flag, scaffold directories are always created (sensible default for non-interactive use). The wizard asks separately
- **Nine categories from community research** — based on Anthropic's skill taxonomy (library, verification, data, automation, scaffold, quality, cicd, runbook, infra)
