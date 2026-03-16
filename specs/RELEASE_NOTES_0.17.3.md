# skillshare v0.17.3 Release Notes

Release date: 2026-03-16

## TL;DR

v0.17.3 brings the **target list** command into the interactive TUI family, adds **centralized skills repo** support, and fixes a couple of paper cuts:

1. **Centralized skills repo** — `init -p --config local` enables one repo for skills, each developer manages own targets
2. **Init source path prompt** — `init` now asks whether to customize source directory path instead of silently defaulting
3. **Target list TUI** — full-screen interactive browser with split layout, fuzzy filter, and inline editing
4. **Mode picker** — change a target's sync mode (`M` key) without leaving the TUI
5. **Include/Exclude editor** — add/remove filter patterns (`I`/`E` keys) directly from the TUI
6. **Web UI error guidance** — network failures now show actionable "restart `skillshare ui`" message
7. **`init --help` fix** — `--subdir` flag now visible, flag ordering matches documentation

---

## Target List Interactive TUI

### The problem

`skillshare target list` printed a static plain-text table. To change a target's mode or filters, users had to run separate CLI commands (`target <name> --mode copy`, `target <name> --add-include "pattern"`). This required remembering exact flag syntax and re-typing the target name each time.

### Solution

The target list now launches a full-screen bubbletea TUI (like `list`, `log`, `diff`, `audit`) with:

- **Split panel layout** — target list on the left, detail panel on the right. Falls back to vertical stacking on narrow terminals (`< 70` columns)
- **Fuzzy filter** — press `/` to filter by target name. `Enter` locks the filter, `Esc` clears it
- **Mode picker overlay** — press `M` to open a three-option picker (merge / copy / symlink) with descriptions. `Enter` confirms and saves immediately
- **Include/Exclude editor** — press `I` or `E` to open an inline pattern list. `a` adds a new glob pattern, `d` deletes the selected one. All changes save to config on each action

### Design decisions

- **Immediate persistence** — mode and filter changes write to config (global or project) on each action, matching the CLI behavior where `--add-include` saves immediately
- **Reuses shared TUI infrastructure** — `wrapAndScroll`, `renderHorizontalSplit`, `renderTUIFilterBar`, and the standard color palette (cyan / gray / yellow) are shared across all TUI views
- **Dual-mode support** — the TUI checks `projCfg != nil` to determine global vs project mode. Both modes load config, apply changes, and save to the correct config file

### Usage patterns

```bash
# Interactive (default on TTY)
skillshare target list

# Project mode
skillshare target list -p

# Skip TUI
skillshare target list --no-tui

# JSON for scripting
skillshare target list --json
```

TUI keybindings:
| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate target list |
| `/` | Start fuzzy filter |
| `M` | Open mode picker for selected target |
| `I` | Edit include patterns |
| `E` | Edit exclude patterns |
| `Ctrl+d`/`Ctrl+u` | Scroll detail panel |
| `q` | Quit |

---

## Init Source Path Prompt

### The problem

`skillshare init` silently set the source directory to `~/.config/skillshare/skills/` without telling the user this was customizable. Only users who read the docs or knew about `--source` could change it. First-time users had no opportunity to choose a different location.

### Solution

In interactive mode, `init` now displays the default source path and asks whether to customize it:

```
ℹ Source directory stores your skills (single source of truth)
  Default: /home/user/.config/skillshare/skills
  Customize source path? [y/N]:
```

If the user says yes, they enter a custom path (with `~` expansion). The success message also now includes a hint about `--source` and the config file location.

### Design decisions

- **TTY guard** — `runningInInteractiveTTY()` ensures the prompt only appears in real terminal sessions. Piped stdin (tests, scripts) skips automatically
- **`--source` priority** — when `--source` is provided on the CLI, `promptSourcePath()` is never called. Zero behavior change for non-interactive workflows
- **Same Y/N pattern** — reuses the `bufio.NewReader` + `ReadString('\n')` gate pattern from `useSourceSubdir()`, keeping the UX consistent

---

## Centralized Skills Repo (`--config local`)

### The problem

Teams often want one dedicated repo (Project A) to hold all shared AI skills, while keeping other projects (B, C, D) clean. `init -p` technically worked for this, but the UX was poor:
- `config.yaml` was committed to git, so all developers shared the same target list
- When a teammate cloned the repo and ran `init -p`, `partialInitRepair` auto-selected existing directories without prompting — confusing and wrong for this use case
- No guidance on how to set up per-developer targets

### Solution

Two-part change:

1. **Creator flow**: `skillshare init -p --config local` adds `config.yaml` to `.skillshare/.gitignore`. Skills are shared via git, but each developer gets their own config file with their own target paths.

2. **Teammate flow**: When a teammate clones the repo and runs `skillshare init -p`, skillshare reads `.skillshare/.gitignore` — if it contains `config.yaml`, it enters **shared repo mode**: creates an empty config (no auto-selected targets) and shows `target add` guidance. No `--config local` flag needed.

### Design decisions

- **Detection via `.gitignore`** — uses existing `GitignoreContains()` to check for `config.yaml` in the managed marker block. No new marker files or config fields needed
- **`UpdateGitIgnoreFiles` vs `UpdateGitIgnoreBatch`** — new `UpdateGitIgnoreFiles()` function for file-mode entries (no trailing `/`). Both delegate to a shared `addGitIgnoreEntries()` helper to avoid code duplication
- **`partialInitRepair` fix** — the old behavior (auto-selecting existing directories) was replaced with interactive prompt for normal partial repairs, matching fresh init behavior

### Usage patterns

```bash
# Creator: set up the shared repo
skillshare init -p --config local --targets claude
skillshare install <skill-repo> -p
git add .skillshare/ && git commit && git push

# Teammate: clone and configure
git clone <repo> && cd <repo>
skillshare init -p                                    # Auto-detects shared repo
skillshare target add projB ~/DEV/projB/.cursor/skills -p
skillshare sync -p
```

---

## Bug Fixes

- **Web UI network error** — `ui/src/api/client.ts` now wraps `fetch()` in try-catch, converting the generic `TypeError: Failed to fetch` into a user-friendly message: "Cannot connect to skillshare API server. Please restart `skillshare ui`." This helps when the Go API server has stopped but the browser tab is still open
- **`init --help` completeness** — the `--subdir` flag was parsed and functional since v0.17.1 but was missing from `printInitHelp()`. Now visible in help output, with flag ordering matching the website documentation (contributed by @dstotijn)
- **Project trash gitignore** — `ensureProjectGitignore` now adds both `logs/` and `trash/` to `.skillshare/.gitignore` via a single `UpdateGitIgnoreBatch` call. Previously, `trash/` was missing, so soft-deleted skills (from `uninstall -p`) could appear in `git status` and be accidentally committed. Existing projects created before v0.17.3 are patched on the next `uninstall` run via a backward-compat call to `ensureProjectGitignore(root)`
