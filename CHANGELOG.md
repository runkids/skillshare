# Changelog

## [0.15.2] - 2026-02-22

### Added
- **`--audit` flag for `hub index`** — `skillshare hub index --audit` enriches the index with per-skill risk scores (0–100) and risk labels so teammates can assess skill safety before installing; `search` displays risk badges in hub results; schema stays v1 with optional fields (omitted when `--audit` is not used)

### Changed
- **`hub index --audit` parallel scanning** — audit scans now run concurrently (up to 8 workers) for faster index generation on large skill collections

### Fixed
- **`init --remote` timing** — initial commit is now deferred to after skill installation, preventing "Local changes detected" errors on first `pull`; re-running `init --remote` on existing config handles edge cases with proper timeout and error recovery
- **Auth error messages for `push`/`pull`** — authentication failures now show actionable hints (SSH URL, token env vars, credential helper) instead of misleading "pull first" advice; includes platform-specific syntax (PowerShell on Windows, `export` on Unix) and links to docs with required token scopes per platform (GitLab, Bitbucket)
- **Git output parsing on non-English systems** — `push`, `pull`, and `init` now set `LC_ALL=C` to force English git output, preventing locale-dependent string matching failures (e.g. "nothing to commit" not detected on Chinese/Japanese systems)
- **Skill version double prefix** — versions like `v0.15.0` in SKILL.md frontmatter no longer display as `vv0.15.0`

## [0.15.1] - 2026-02-21

### Added
- **Dangling link detection in audit** — `skillshare audit` now checks `.md` files for broken local relative links (missing files or directories); produces `LOW` severity findings with pattern `dangling-link`; disable via `audit-rules.yaml` with `- id: dangling-link` / `enabled: false`

### Fixed
- **`push`/`pull` first-sync and remote flow** — overhauled `init --remote`, `push`, and `pull` to handle edge cases: re-running `init --remote` on an existing config, pushing/pulling when remote has no commits yet, and conflicting remote URLs
- **Partial project init recovery** — if `.skillshare/` exists but `config.yaml` is missing, commands now repair config instead of failing

## [0.15.0] - 2026-02-21

### Added
- **Copy sync mode** — `skillshare target <name> --mode copy` syncs skills as real files instead of symlinks, for AI CLIs that can't follow symlinks (e.g. Cursor, Copilot CLI); uses SHA256 checksums for incremental updates; `sync --force` re-copies all; existing targets can switch between merge/copy/symlink at any time (#31, #2)
- **Private repo support via HTTPS tokens** — `install` and `update` now auto-detect `GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`, or `SKILLSHARE_GIT_TOKEN` for HTTPS clone/pull; no manual git config needed; tokens are never written to disk
- **Better auth error messages** — auth failures now tell you whether the issue is "no token found" (with setup suggestions) or "token rejected" (check permissions/expiry); token values are redacted in output

### Fixed
- **`diff` now detects content changes in copy mode** — previously only checked symlink presence; now compares file checksums
- **`doctor` no longer flags copy-managed skills as duplicates**
- **`target remove` in project mode cleans up copy manifest**
- **Copy mode no longer fails on stray files** in target directories or missing target paths
- **`update` and `check` now honor HTTPS token auth** — private repo pull/remote checks now auto-detect `GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`, and `SKILLSHARE_GIT_TOKEN` (same as install)
- **Devcontainer project mode no longer pollutes workspace root** — `ss` keeps caller working directory and redirects `-p` from `/workspace` to demo project
- **Project mode auto-repairs partial initialization** — if `.skillshare/` exists but `config.yaml` is missing, commands repair config instead of failing with "project already initialized"

### Changed
- **`agents` target renamed to `universal`** — existing configs using `agents` continue to work (backward-compatible alias); Kimi and Replit paths updated to match upstream docs
- **`GITHUB_TOKEN` now used for HTTPS clone** — previously only used for GitHub API (search, upgrade); now also used when cloning private repos over HTTPS

## [0.14.2] - 2026-02-20

### Added
- **Multi-name and `--group` for `update`** — `skillshare update a b c` updates multiple skills at once; `--group`/`-G` flag expands a group directory to all updatable skills within it (repeatable); positional names that match a group directory are auto-detected and expanded; names and groups can be mixed freely
- **Multi-name and `--group` for `check`** — `skillshare check a b c` checks only specified skills; `--group`/`-G` flag works identically to `update`; no args = check all (existing behavior preserved); filtered mode includes a loading spinner for network operations
- **Security guide** — new `docs/guides/security.md` covering audit rules, `.skillignore`, and safe install practices; cross-referenced from audit command docs and best practices guide

### Changed
- **Docs diagrams migrated to Mermaid SVG** — replaced ASCII box-drawing diagrams across 10+ command docs with Mermaid `handDrawn` look for better rendering and maintainability
- **Hub docs repositioned** — hub documentation reframed as organization-first with private source examples
- **Docker/devcontainer unified** — consolidated version definitions, init scripts, and added `sandbox-logs` target; devcontainer now includes Node.js 24, auto-start dev servers, and a `dev-servers` manager script

## [0.14.1] - 2026-02-19

### Added
- **Config YAML Schema** — JSON Schema files for both global `config.yaml` and project `.skillshare/config.yaml`; enables IDE autocompletion, validation, and hover documentation via YAML Language Server; `Save()` automatically prepends `# yaml-language-server: $schema=...` directive; new configs from `skillshare init` include the directive out of the box; existing configs get it on next save (any mutating command)

## [0.14.0] - 2026-02-18

### Added
- **Global skill manifest** — `config.yaml` now supports a `skills:` section in global mode (previously project-only); `skillshare install` (no args) installs all listed skills; auto-reconcile keeps the manifest in sync after install/uninstall
- **`.skillignore` file** — repo-level file to hide skills from discovery during install; supports exact match and trailing wildcard patterns; group matching via path-based comparison (e.g. `feature-radar` excludes all skills under that directory)
- **`--exclude` flag for install** — skip specific skills during multi-skill install; filters before the interactive prompt so excluded skills never appear
- **License display in install** — shows SKILL.md `license` frontmatter in selection prompts and single-skill confirmation screen
- **Multi-skill and group uninstall** — `skillshare uninstall` accepts multiple skill names and a repeatable `--group`/`-G` flag for batch removal; groups use prefix matching; problematic skills are skipped with warnings; group directories auto-detected with sub-skill listing in confirmation prompt
- **`group` field in skill manifest** — explicit `group` field separates placement from identity (previously encoded as `name: frontend/pdf`); automatic migration of legacy slash-in-name entries; both global and project reconcilers updated
- **6 new audit security rules** — detection for `eval`/`exec`/`Function` dynamic code, Python shell execution, `process.env` leaking, prompt injection in HTML comments, hex/unicode escape obfuscation; each rule includes false-positive guards
- **Firebender target** — coding agent for JetBrains IDEs; paths: `~/.firebender/skills` (global), `.firebender/skills` (project); target count now 49+
- **Declarative manifest docs** — new concept page and URL formats reference page

### Fixed
- **Agent target paths synced with upstream** — antigravity: `global_skills` → `skills`; augment: `rules` → `skills`; goose project: `.agents/skills` → `.goose/skills`
- **Docusaurus relative doc links** — added `.md` extension to prevent 404s when navigating via navbar

### Changed
- **Website docs restructured** — scenario-driven "What do you want to do?" navigation on all 9 section index pages; standardized "When to Use" and "See Also" sections across all 24 command docs; role-based paths in intro; "What Just Happened?" explainer in getting-started
- **Install integration tests split by concern** — tests reorganized into `install_basic`, `install_discovery`, `install_filtering`, `install_selection`, and `install_helpers` for maintainability

## [0.13.0] - 2026-02-16

### Added
- **Skill-level `targets` field** — SKILL.md frontmatter now accepts a `targets` list to restrict which targets a skill syncs to; `check` validates unknown target names
- **Target filter CLI** — `target <name> --add-include/--add-exclude/--remove-include/--remove-exclude` for inline filter editing; Web UI inline filter editor on Targets page
- **XDG Base Directory support** — respect `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`; backups/trash stored in data dir, logs in state dir; automatic migration from legacy layout on first run
- **Windows legacy path migration** — existing Windows installs at `~\.config\skillshare\` are auto-migrated to `%AppData%\skillshare\` with config source path rewrite
- **Fuzzy subdirectory resolution** — `install owner/repo/skill-name` now fuzzy-matches nested skill directories by basename when exact path doesn't exist, with ambiguity error for multiple matches
- **`list` grouped display** — skills are grouped by directory with tree-style formatting; `--verbose`/`-v` flag for detailed output
- **Runtime UI download** — `skillshare ui` downloads frontend assets from GitHub Releases on first launch and caches at `~/.cache/skillshare/ui/<version>/`; `--clear-cache` to reset; `upgrade` pre-downloads UI assets

### Changed
- **Unified project target names** — project targets now use the same short names as global (e.g. `claude` instead of `claude-code`); old names preserved as aliases for backward compatibility
- **Binary no longer embeds UI** — removed `go:embed` and build tags; UI served exclusively from disk cache, reducing binary size
- **Docker images simplified** — production and CI Dockerfiles no longer include Node build stages

### Fixed
- **Windows `DataDir()`/`StateDir()` paths** — now correctly fall back to `%AppData%` instead of Unix-style `~/.local/` paths
- **Migration result reporting** — structured `MigrationResult` with status tracking; migration outcomes printed at startup
- **Orphan external symlinks after data migration** — `sync` now auto-removes broken external symlinks (e.g. leftover from XDG/Windows path migration); `--force` removes all external symlinks; path comparison uses case-insensitive matching on Windows

### Breaking Changes
- **Windows paths relocated** — config/data moves from `%USERPROFILE%\.config\skillshare\` to `%AppData%\skillshare\` (auto-migrated)
- **XDG data/state split (macOS/Linux)** — backups and trash move from `~/.config/skillshare/` to `~/.local/share/skillshare/`; logs move to `~/.local/state/skillshare/` (auto-migrated)
- **Project target names changed** — `claude-code` → `claude`, `gemini-cli` → `gemini`, etc. (old names still work via aliases)

## [0.12.6] - 2026-02-13

### Added
- **Per-target include/exclude filters (merge mode)** — `include` / `exclude` glob patterns are now supported in both global and project target configs
- **Comprehensive filter test coverage** — added unit + integration tests for include-only, exclude-only, include+exclude precedence, invalid patterns, and prune behavior
- **Project mode support for `doctor`** — `doctor` now supports auto-detect project mode plus explicit `--project` / `--global`

### Changed
- **Filter-aware diagnostics** — `sync`, `diff`, `status`, `doctor`, API drift checks, and Web UI target counts now compute expected skills using include/exclude filters
- **Web UI config freshness** — UI API now auto-reloads config on requests, so browser refresh reflects latest `config.yaml` without restarting `skillshare ui`
- **Documentation expanded** — added practical include/exclude strategy guidance, examples, and project-mode `doctor` usage notes

### Fixed
- **Exclude pruning behavior in merge mode** — when a previously synced source-linked entry becomes excluded, `sync` now unlinks/removes it; existing local non-symlink target folders are preserved
- **Project `doctor` backup/trash reporting** — now uses project-aware semantics (`backups not used in project mode`, trash checked from `.skillshare/trash`)

## [0.12.5] - 2026-02-13

### Fixed
- **`target remove` merge mode symlink cleanup** — CLI now correctly detects and removes all skillshare-managed symlinks using path prefix matching instead of exact name matching; fixes nested/orphaned symlinks being left behind
- **`target remove` in Web UI** — server API now handles merge mode targets (previously only cleaned up symlink mode)

## [0.12.4] - 2026-02-13

### Added
- **Graceful shutdown** — HTTP server handles SIGTERM/SIGINT with 10s drain period, safe for container orchestrators
- **Server timeouts** — ReadHeaderTimeout (5s), ReadTimeout (15s), WriteTimeout (30s), IdleTimeout (60s) prevent slow-client resource exhaustion
- **Enhanced health endpoint** — `/api/health` now returns `version` and `uptime_seconds`
- **Production Docker image** (`docker/production/Dockerfile`) — multi-stage build, `tini` PID 1, non-root user (UID 10001), auto-init entrypoint, healthcheck
- **CI Docker image** (`docker/ci/Dockerfile`) — minimal image for `skillshare audit` in pipelines
- **Docker dev profile** — `make dev-docker-up` runs Go API server in Docker for frontend development without local Go
- **Multi-arch Docker build** — `make docker-build-multiarch` produces linux/amd64 + linux/arm64 images
- **Docker publish workflow** (`.github/workflows/docker-publish.yml`) — auto-builds and pushes production + CI images to GHCR on tag push
- **`make sandbox-status`** — show playground container status

### Changed
- **Compose security hardening** — playground: `read_only`, `cap_drop: ALL`, `tmpfs` with exec; all profiles: `no-new-privileges`, resource limits (2 CPU / 2G)
- **Test scripts DRY** — `test_docker.sh` accepts `--online` flag; `test_docker_online.sh` is now a thin wrapper
- **Compose version check** — `_sandbox_common.sh` verifies Docker Compose v2.20+ with platform-specific install hints
- **`.dockerignore` expanded** — excludes `.github/`, `website/`, editor temp files
- **Git command timeout** — increased from 60s to 180s for constrained Docker/CI networks
- **Online test timeout** — increased from 120s to 300s

### Fixed
- **Sandbox `chmod` failure** — playground volume init now uses `--cap-add ALL` to work with `cap_drop: ALL`
- **Dev profile crash on first run** — auto-runs `skillshare init` before starting UI server
- **Sandbox Dockerfile missing `curl`** — added for playground healthcheck

## [0.12.2] - 2026-02-13

### Fixed
- **Hub search returns all results** — hub/index search no longer capped at 20; `limit=0` means no limit (GitHub search default unchanged)
- **Search filter ghost cards** — replaced IIFE rendering with `useMemo` to fix stale DOM when filtering results

### Added
- **Scroll-to-load in Web UI** — search results render 20 at a time with IntersectionObserver-based incremental loading

## [0.12.1] - 2026-02-13

### Added
- **Hub persistence** — saved hubs stored in `config.yaml` (both global and project), shared between CLI and Web UI
  - `hub add <url>` — save a hub source (`--label` to name it; first add auto-sets as default)
  - `hub list` — list saved hubs (`*` marks default)
  - `hub remove <label>` — remove a saved hub
  - `hub default [label]` — show or set the default hub (`--reset` to clear)
  - All subcommands support `--project` / `--global` mode
- **Hub label resolution in search** — `search --hub <label>` resolves saved hub labels instead of requiring full URLs
  - `search --hub team` looks up the "team" hub from config
  - `search --hub` (bare) uses the config default, falling back to community hub
- **Hub saved API** — REST endpoints for hub CRUD (`GET/PUT/POST/DELETE /api/hub/saved`)
- **Web UI hub persistence** — hub list and default hub now persisted on server instead of browser localStorage
- **Search fuzzy filter** — hub search results filtered by fuzzy match on name + substring match on description and tags
- **Tag badges in search** — `#tag` badges displayed in both CLI interactive selector and Web UI hub search results
- **Web UI tag filter** — inline filter input on hub search cards matching name, description, and tags

### Changed
- `search --hub` (bare flag) now defaults to community skillshare-hub instead of requiring a URL
- Web UI SearchPage migrated from localStorage to server API for hub state

### Fixed
- `audit <path>` no longer fails with "config not found" in CI environments without a skillshare config

## [0.12.0] - 2026-02-13

### Added
- **Hub index generation** — `skillshare hub index` builds a `skillshare-hub.json` from installed skills for private or team catalogs
  - `--full` includes extended metadata (flatName, type, version, repoUrl, installedAt)
  - `--output` / `-o` to customize output path; `--source` / `-s` to override scan directory
  - Supports both global and project mode (`-p` / `-g`)
- **Private index search** — `skillshare search --hub <url>` searches a hub index (local file or HTTP URL) instead of GitHub
  - Browse all entries with no query, or fuzzy-match by name/description/tags/source
  - Interactive install prompt with `source` and optional `skill` field support
- **Hub index schema** — `schemaVersion: 1` with `tags` and `skill` fields for classification and multi-skill repo support
- **Web UI hub search** — search private indexes from the dashboard with a hub URL dropdown
  - Hub manager modal for adding, removing, and selecting saved hub URLs (persisted in localStorage)
- **Web UI hub index API** — `GET /api/hub/index` endpoint for generating indexes from the dashboard
- Hub index guide and command reference in documentation

### Fixed
- `hub index` help text referenced incorrect `--index-url` flag (now `--hub`)
- Frontend `SearchResult` TypeScript interface missing `tags` field

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
