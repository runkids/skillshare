# Changelog

## [0.11.6] - 2026-02-11

### Added
- **Auto-pull on `init --remote`** — when remote has existing skills, init automatically fetches and syncs them; no manual `git clone` or `git pull` needed
- **Auto-commit on `git init`** — `init` creates an initial commit (with `.gitignore`) so `push`/`pull`/`stash` work immediately
- **Git identity fallback** — if `user.name`/`user.email` aren't configured, sets repo-local defaults (`skillshare@local`) with a hint to set your own
- **Git remote error hints** — `push`, `pull`, and `init --remote` now show actionable hints for SSH, URL, and network errors
- **Docker sandbox `--bare` mode** — `make sandbox-bare` starts the playground without auto-init for manual testing
- **Docker sandbox `--volumes` reset** — `make sandbox-reset` removes the playground home volume for a full reset

### Changed
- **`init --remote` auto-detection** — global-only flags (`--remote`, `--source`, etc.) now skip project-mode auto-detection, so `init --remote` works from any directory
- **Target multi-select labels** — shortened to `name (status)` for readability; paths shown during detection phase instead

### Fixed
- `init --remote` on second machine no longer fails with "Local changes detected" or merge conflicts
- `init --remote` produces clean linear git history (no merge commits from unrelated histories)
- Pro tip message only shown when built-in skill is actually installed

## [0.11.5] - 2026-02-11

### Added
- **`--into` flag for install** — organize skills into subdirectories (`skillshare install repo --into frontend` places skills under `skills/frontend/`)
- **Nested skill support in check/update/uninstall** — recursive directory walk detects skills in organizational folders; `update` and `uninstall` resolve short names (e.g., `update vue` finds `frontend/vue/vue-best-practices`)
- **Configurable audit block threshold** — `audit.block_threshold` in config sets which severity blocks install (default `CRITICAL`); `audit --threshold <level>` overrides per-command
- **Audit path scanning** — `skillshare audit <path>` scans arbitrary files or directories, not only installed skills
- **Audit JSON output** — `skillshare audit --json` for machine-readable results with risk scores
- **`--skip-audit` flag for install** — bypass security scanning for a single install command
- **Risk scoring** — weighted risk score and label (clean/low/medium/high/critical) per scanned skill
- **LOW and INFO severity levels** — lighter-weight findings that contribute to risk score without blocking
- **IBM Bob target** — added to supported AI CLIs (global: `~/.bob/skills`, project: `.bob/skills`)
- **JS/TS syntax highlighting in file viewer** — Web UI highlights `.js`, `.ts`, `.jsx`, `.tsx` files with CodeMirror
- **Project init agent grouping** — agents sharing the same project skills path (Amp, Codex, Copilot, Gemini, Goose, etc.) are collapsed into a single selectable group entry

### Changed
- **Goose project path** updated from `.goose/skills` to `.agents/skills` (universal agent directory convention)
- **Audit summary includes all severity levels** — LOW/INFO counts, risk score, and threshold shown in summary box and log entries

### Fixed
- Web UI nested skill update now uses full relative path instead of basename only
- YAML block scalar frontmatter (`>-`, `|`, `|-`) parsed correctly in skill detail view
- CodeMirror used for all non-markdown files in file viewer (previously plain `<pre>`)

## [0.11.4] - 2026-02-11

### Added
- **Customizable audit rules** — `audit-rules.yaml` externalizes security rules for user overrides
  - Three-layer merge: built-in → global (`~/.config/skillshare/audit-rules.yaml`) → project (`.skillshare/audit-rules.yaml`)
  - Add custom rules, override severity, or disable built-in rules per-project
  - `skillshare audit --init-rules` to scaffold a starter rules file
- **Web UI Audit Rules page** — create, edit, toggle, and delete rules from the dashboard
- **Log filtering** — filter operation/audit logs by status, command, or keyword; custom dropdown component
- **Docker playground audit demo** — pre-loaded demo skills and custom rules for hands-on audit exploration

### Changed
- **Built-in skill is now opt-in** — `init` and `upgrade` no longer install the built-in skill by default; use `--skill` to include it
- **HIGH findings reclassified as warnings** — only CRITICAL findings block `install`; HIGH/MEDIUM are shown as warnings
- Integration tests split into offline (`!online`) and online (`online`) build tags for faster local runs

## [0.11.0] - 2026-02-10

### Added
- **Security Audit** — `skillshare audit [name]` scans skills for prompt injection, data exfiltration, credential access, destructive commands, obfuscation, and suspicious URLs
  - CRITICAL findings block `skillshare install` by default; use `--force` to override
  - HIGH/MEDIUM findings shown as warnings with file, line, and snippet detail
  - Per-skill progress display with tree-formatted findings and summary box
  - Project mode support (`skillshare audit -p`)
- **Web UI Audit page** — scan all skills from the dashboard, view findings with severity badges
  - Install flow shows `ConfirmDialog` on CRITICAL block with "Force Install" option
  - Warning dialog displays HIGH/MEDIUM findings after successful install
- **Audit API** — `GET /api/audit` and `GET /api/audit/{name}` endpoints
- **Operation log (persistent audit trail)** — JSONL-based operations/audit logging across CLI + API + Web UI
  - CLI: `skillshare log` (`--audit`, `--tail`, `--clear`, `-p/-g`)
  - API: log list/clear endpoints for operations and audit streams
  - Web UI: Log page with tabs, filters, status/duration formatting, and clear/refresh actions
- **Sync drift detection** — `status` and `doctor` warn when targets have fewer linked skills than source
  - Web UI shows drift badges on Dashboard and Targets pages
- **Trash (soft-delete) workflow** — uninstall now moves skills to trash with 7-day retention
  - New CLI commands: `skillshare trash list`, `skillshare trash restore <name>`, `skillshare trash delete <name>`, `skillshare trash empty`
  - Web UI Trash page for list/restore/delete/empty actions
  - Trash API handlers with global/project mode support
- **Update preview command** — `skillshare check` shows available updates for tracked repos and installed skills without modifying files
- **Search ranking upgrade** — relevance scoring now combines name/description/stars with repo-scoped query support (`owner/repo[/subdir]`)
- **Docs site local search** — Docusaurus local search integrated for command/doc lookup
- **SSH subpath support** — `install git@host:repo.git//subdir` with `//` separator
- **Docs comparison guide** — new declarative vs imperative workflow comparison page

### Changed
- **Install discovery + selection UX**
  - Hidden directory scan now skips only `.git` (supports repos using folders like `.curated/` and `.system/`)
  - `install --skill` falls back to fuzzy matching when exact name lookup fails
  - UI SkillPicker adds filter input and filtered Select All behavior for large result sets
  - Batch install feedback improved: summary toast always shown; blocked-skill retry targets only blocked items
  - CLI mixed-result installs now use warning output and condensed success summaries
- **Search performance + metadata enrichment** — star/description enrichment is parallelized, and description frontmatter is used in scoring
- **Skill template refresh** — `new` command template updated to a WHAT+WHEN trigger format with step-based instructions
- **Search command UX** — running `search` with no keyword now prompts for input instead of auto-browsing
- **Sandbox hardening** — playground shell defaults to home and mounts source read-only to reduce accidental host edits
- **Project mode clarity** — `(project)` labels added across key command outputs; uninstall prompt now explicitly says "from the project?"
- **Project tracked-repo workflow reliability**
  - `ProjectSkill` now supports `tracked: true` for portable project manifests
  - Reconcile logic now detects tracked repos via `.git` + remote origin even when metadata files are absent
  - Tracked repo naming uses `owner-repo` style (for example, `_openai-skills`) to avoid basename collisions
  - Project `list` now uses recursive skill discovery for parity with global mode and Web UI
- **Privacy-first messaging + UI polish** — homepage/README messaging updated, dashboard quick actions aligned, and website hero/logo refreshed with a new hand-drawn style
- `ConfirmDialog` component supports `wide` prop and hidden cancel button
- Sidebar category renamed from "Utilities" to "Security & Utilities"
- README updated with audit section, new screenshots, unified image sizes
- Documentation links and navigation updated across README/website

### Fixed
- Web UI uninstall handlers now use trash move semantics instead of permanent deletion
- Windows self-upgrade now shows a clear locked-binary hint when rename fails (for example, when `skillshare ui` is still running)
- `mise.toml` `ui:build` path handling fixed so `cd ui` does not leak into subsequent build steps
- Sync log details now include target count, fixing blank details in some entries
- Project tracked repos are no longer skipped during reconcile when metadata is missing

## [0.10.0] - 2026-02-08

### Added
- **Web Dashboard** — `skillshare ui` launches a full-featured React SPA embedded in the binary
  - Dashboard overview with skill/target counts, sync mode, and version check
  - Skills browser with search, filter, SKILL.md viewer, and uninstall
  - Targets page with status badges, add/remove targets
  - Sync controls with dry-run/force toggles and diff preview
  - Collect page to scan and pick skills from targets back to source
  - GitHub skill search with one-click install and batch install
  - Config editor with YAML validation
  - Backup/restore management with cleanup
  - Git sync page with push/pull, dirty-file detection, and force-pull
  - Install page supporting path, git URL, and GitHub shorthand inputs
  - Update tracked repos from the UI with commit/diff details
- **REST API** at `/api/*` — Go `net/http` backend (30+ endpoints) powering the dashboard
- **Single-binary distribution** — React frontend embedded via `go:embed`, no Node.js required at runtime
- **Dev mode** — `go build -tags dev` serves placeholder SPA; use Vite on `:5173` with `/api` proxy for hot reload
- **`internal/git/info.go`** — git operations library (pull with change info, force-pull, dirty detection, stage/commit/push)
- **`internal/version/skill.go`** — local and remote skill version checking
- **Bitbucket/GitLab URL support** — `install` now strips branch prefixes from Bitbucket (`src/{branch}/`) and GitLab (`-/tree/{branch}/`) web URLs
- **`internal/utils/frontmatter.go`** — `ParseFrontmatterField()` utility for reading SKILL.md metadata
- Integration tests for `skillshare ui` server startup
- Docker sandbox support for web UI (`--host 0.0.0.0`, port 19420 mapping)
- CI: frontend build step in release and test workflows
- Website documentation for `ui` command

### Changed
- Makefile updated with `ui-build`, `build-ui`, `ui-dev` targets
- `.goreleaser.yaml` updated to include frontend build in release pipeline
- Docker sandbox Dockerfile uses multi-stage build with Node.js for frontend assets

## [0.9.0] - 2026-02-05

### Added
- **Project-level skills** — scope skills to a single repository, shared via git
  - `skillshare init -p` to initialize project mode
  - `.skillshare/` directory with `config.yaml`, `skills/`, and `.gitignore`
  - All core commands support `-p` flag: `sync`, `install`, `uninstall`, `update`, `list`, `status`, `target`, `collect`
- **Auto-detection** — commands automatically switch to project mode when `.skillshare/config.yaml` exists
- **Per-target sync mode for project mode** — each target can use `merge` or `symlink` independently
- **`--discover` flag** — detect and add new AI CLI targets to existing project config
- **Tracked repos in project mode** — `install --track -p` clones repos into `.skillshare/skills/`
- Integration tests for all project mode commands

### Changed
- Terminology: "Team Sharing" → "Organization-Wide Skills", "Team Edition" → "Organization Skills"
- Documentation restructured with dual-level architecture (Organization + Project)
- Unified project sync output format with global sync

## [0.8.0] - 2026-01-31

### Breaking Changes

**Command Rename: `pull <target>` → `collect <target>`**

For clearer command symmetry, `pull` is now exclusively for git operations:

| Before | After | Description |
|--------|-------|-------------|
| `pull claude` | `collect claude` | Collect skills from target to source |
| `pull --all` | `collect --all` | Collect from all targets |
| `pull --remote` | `pull` | Pull from git remote |

### New Command Symmetry

| Operation | Commands | Direction |
|-----------|----------|-----------|
| Local sync | `sync` / `collect` | Source ↔ Targets |
| Remote sync | `push` / `pull` | Source ↔ Git Remote |

```
Remote (git)
   ↑ push    ↓ pull
Source
   ↓ sync    ↑ collect
Targets
```

### Migration

```bash
# Before
skillshare pull claude
skillshare pull --remote

# After
skillshare collect claude
skillshare pull
```

## [0.7.0] - 2026-01-31

### Added
- Full Windows support (NTFS junctions, zip downloads, self-upgrade)
- `search` command to discover skills from GitHub
- Interactive skill selector for search results

### Changed
- Windows uses NTFS junctions instead of symlinks (no admin required)

## [0.6.0] - 2026-01-20

### Added
- Team Edition with tracked repositories
- `--track` flag for `install` command
- `update` command for tracked repos
- Nested skill support with `__` separator

## [0.5.0] - 2026-01-16

### Added
- `new` command to create skills with template
- `doctor` command for diagnostics
- `upgrade` command for self-upgrade

### Changed
- Improved sync output with detailed statistics

## [0.4.0] - 2026-01-16

### Added
- `diff` command to show differences
- `backup` and `restore` commands
- Automatic backup before sync

### Changed
- Default sync mode changed to `merge`

## [0.3.0] - 2026-01-15

### Added
- `push` and `pull --remote` for cross-machine sync
- Git integration in `init` command

## [0.2.0] - 2026-01-14

### Added
- `install` and `uninstall` commands
- Support for git repo installation
- `target add` and `target remove` commands

## [0.1.0] - 2026-01-14

### Added
- Initial release
- `init`, `sync`, `status`, `list` commands
- Symlink and merge sync modes
- Multi-target support
