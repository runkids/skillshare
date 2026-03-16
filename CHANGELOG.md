# Changelog

## [0.17.3] - 2026-03-16

### New Features

#### Centralized Skills Repo

- **`--config local` for project init** — `skillshare init -p --config local` gitignores `config.yaml` so each developer manages their own targets independently. Skills are shared via git, config stays local:
  ```bash
  # Creator: set up shared skills repo
  skillshare init -p --config local --targets claude
  skillshare install <skill> -p && git push

  # Teammate: clone and configure own targets
  git clone <repo> && cd <repo>
  skillshare init -p
  skillshare target add myproject ~/DEV/myproject/.claude/skills -p
  ```
- **Smart shared repo detection** — when a teammate clones a shared skills repo and runs `skillshare init -p`, skillshare auto-detects the shared repo pattern (config.yaml in .gitignore) and creates an empty config with guided next steps. No `--config local` flag needed for cloners

#### Init Source Path Prompt

- **Interactive source path customization** — `skillshare init` now asks whether you want to customize the source directory path instead of silently using the default (`~/.config/skillshare/skills/`). Use `--source` to skip the prompt in scripts:
  ```bash
  skillshare init                          # Prompts for source path
  skillshare init --source ~/my-skills     # Skips prompt
  ```

#### Target List Interactive TUI

- **Interactive target browser** — `skillshare target list` now launches a full-screen TUI with a split panel layout (target list on the left, detail panel on the right). Includes fuzzy filtering via `/` and keyboard navigation:
  ```bash
  skillshare target list           # Interactive TUI (default on TTY)
  skillshare target list --no-tui  # Plain text output
  ```
- **Mode picker** — press `M` on any target to change its sync mode (merge, copy, symlink) without leaving the TUI. Changes are saved to config immediately
- **Include/Exclude editor** — press `I` or `E` to open an inline pattern editor for the selected target. Add patterns with `a`, delete with `d` — changes persist to config on each action

### Bug Fixes

- **Web UI network error guidance** — the web dashboard now shows a clear "restart `skillshare ui`" message when the API server is unreachable, instead of a generic "Failed to fetch" error
- **`init --help` completeness** — `skillshare init --help` now shows the `--subdir` flag and lists flags in the same order as the documentation
- **Project trash gitignore** — `skillshare init -p` now automatically adds `trash/` to `.skillshare/.gitignore`, preventing soft-deleted skills from being accidentally committed. Existing projects are patched on the next `uninstall` run

## [0.17.2] - 2026-03-14

### New Features

#### Web UI Git Sync Enhancements

- **Repository Info Card** — the Git Sync page now displays a card showing the current repository URL, branch, and latest commit at the top of the page
- **Branch switcher** — switch between git branches directly from the Git Sync page without leaving the web dashboard

#### Web UI Backup & Sidebar

- **Backup page restore UX** — improved restore flow with clearer action buttons and confirmation dialogs
- **Collapsible sidebar tools** — sidebar tool sections can now be collapsed/expanded to reduce visual clutter

#### CLI TUI Improvements

- **Trash TUI split panel** — the trash list now uses the same left-right split panel layout as other TUIs, with a detail panel showing item info alongside the list

### Bug Fixes

- **Backup command spinner** — `skillshare backup` now shows a progress spinner during backup operations instead of appearing frozen
- **Git Sync footer layout** — Push/Pull action buttons are now pinned to the bottom of the page and no longer shift when content changes

### Performance

- **Restore TUI async size calculation** — backup version sizes are now computed asynchronously in the background instead of blocking the TUI. Large backups with many versions no longer freeze the interface when browsing or selecting versions. Detail panel I/O is capped at 20 skills to prevent lag on large backups

## [0.17.1] - 2026-03-13

### New Features

#### Web UI Theme System

- **Multi-theme support** — the web dashboard now offers two visual styles and three color modes, switchable via the **Theme** button in the sidebar:
  - **Styles**: `Clean` (professional, minimal) and `Playful` (hand-drawn borders, organic shapes)
  - **Modes**: `Light`, `Dark`, and `System` (follows OS preference)
  - Preferences persist in localStorage across sessions. Default: Playful + Light

#### Init Source Subdirectory

- **Subdirectory prompt during init** — `skillshare init` now prompts whether to store skills in a subdirectory instead of the repository root. Useful when embedding skills inside a dotfiles or monorepo:
  ```bash
  skillshare init --remote git@github.com:you/dotfiles.git --subdir skills
  ```
  This sets the source path to `~/.config/skillshare/skills/skills/`, keeping the repo root free for README, CI config, and other non-skill files

### Bug Fixes

- **Collect skips `.git/` directories** — `skillshare collect` now excludes `.git/` when copying skills from target to source. Previously, collecting a git-cloned repo (e.g., `obra/superpowers` in `~/.cursor/skills/`) could produce only empty directories because `filepath.Walk` would abort on `.git/` pack files
- **Actionable git error messages** — `skillshare install` and `skillshare update` now show context-specific guidance instead of raw exit codes when git operations fail:
  - Authentication failures suggest token env vars (`GITHUB_TOKEN`, `GITLAB_TOKEN`, etc.), SSH URLs, or `gh auth login`
  - SSL certificate errors suggest custom CA bundle, SSH, or `GIT_SSL_NO_VERIFY`
  - Token rejections distinguish between missing auth and expired/invalid tokens
  - Divergent branch conflicts show the `fatal:` line instead of just `exit status 128`

## [0.17.0] - 2026-03-11

### Breaking Changes

- **Extras directory structure** — extras source files are now stored under `extras/<name>/` instead of directly under the config root. Existing directories are **auto-migrated** on first `sync extras` run. No manual action required

### New Features

#### First-Class Extras Command Group

Extras (non-skill resources like rules, prompts, commands) are now a first-class feature with their own command group:

- **`extras init`** — create a new extra with interactive TUI wizard or CLI flags:
  ```bash
  skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules
  skillshare extras init prompts --target .claude/prompts --mode copy -p
  ```
- **`extras list`** — view all configured extras with sync status (`synced`, `drift`, `not synced`, `no source`). Interactive TUI with split-pane detail view, or `--json` / `--no-tui` output
- **`extras mode`** — change sync mode of an extra's target from CLI, TUI (`M` key), or Web UI:
  ```bash
  skillshare extras rules --mode copy                   # single target auto-resolved
  skillshare extras mode rules --target ~/.claude/rules --mode copy
  ```
- **`extras remove`** — remove an extra from config (source files and synced targets are preserved)
- **`extras collect`** — reverse-sync local files from a target back into the extras source directory:
  ```bash
  skillshare extras collect rules --from ~/.claude/rules --dry-run
  ```
- **Project mode** — all extras commands support `--project`/`-p` for `.skillshare/` scoped extras

#### Extras Integration with Existing Commands

- **`status`** — shows extras file count and target count per extra
- **`doctor`** — checks that extras source directories exist and target parent directories are reachable
- **`diff`** — automatically includes per-file diff for extras targets when configured
- **`sync extras --json`** — structured JSON output for programmatic consumption
- **`sync --all -p`** — project-mode `--all` now includes extras sync

#### Web UI Redesign

The web dashboard (`skillshare ui`) received a complete visual overhaul — replacing the hand-drawn aesthetic with a clean, minimal design:

- **Redesigned design system** — new DM Sans typography, clean border-radius, streamlined color palette with proper dark mode support
- **Table view with pagination** — skills and search results now offer a table view alongside the existing card/grouped views, with client-side pagination for large collections
- **Sticky search and filters** — SkillsPage toolbar stays pinned at the top while scrolling, with grouped view sticky headers
- **Keyboard modifier shortcuts** — press `?` to see available shortcuts, with an on-screen HUD overlay showing active modifiers
- **Sync progress animation** — visual feedback during sync operations
- **Onboarding tour** — step-by-step spotlight tour for first-time users, highlighting key features
- **Shared UI components** — new DialogShell, IconButton, Pagination, and SegmentedControl components for consistent interactions across pages

#### Web UI Extras Page

- New **Extras page** in the web dashboard with list, sync, remove, add-extra modal, and inline mode dropdown per target
- **Dashboard card** showing extras count, total files, and total targets
- REST API: `GET /api/extras`, `GET /api/extras/diff`, `POST /api/extras`, `POST /api/extras/sync`, `PATCH /api/extras/{name}/mode`, `DELETE /api/extras/{name}`

#### Custom GitLab Domain Support

- **JihuLab auto-detection** — hosts containing `jihulab` in the name (e.g., `jihulab.com`) are now automatically detected alongside `gitlab`, so nested subgroup URLs work without any config
- **`gitlab_hosts` config** — declare self-managed GitLab hostnames so skillshare treats URLs with nested subgroup paths correctly. Hosts containing `gitlab` or `jihulab` in the name are detected automatically; this config is for other custom domains like `git.company.com`:
  ```yaml
  # ~/.config/skillshare/config.yaml (or .skillshare/config.yaml)
  gitlab_hosts:
    - git.company.com
    - code.internal.io
  ```
  ```bash
  # With config above, full path is treated as repo (not owner/repo + subdir)
  skillshare install git.company.com/team/frontend/ui
  ```
  Without config, append `.git` as a workaround: `git.company.com/team/frontend/ui.git`

- **`SKILLSHARE_GITLAB_HOSTS` env var** — comma-separated list of GitLab hostnames for CI/CD pipelines that don't have a config file:
  ```bash
  SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
  ```
  When both the env var and config file are set, their values are merged (deduplicated). Invalid entries in the env var are silently skipped

### Bug Fixes

- **GitLab subgroup URL parsing** — `skillshare install` now correctly handles GitLab nested subgroup URLs with arbitrary depth. Previously, URLs like `gitlab.com/group/subgroup/project` were misinterpreted as repo `group/subgroup` with subdir `project`. Now the entire path is treated as the repo path:
  ```bash
  # These all work now (previously failed)
  skillshare install gitlab.com/group/subgroup/project
  skillshare install onprem.gitlab.internal/org/sub1/sub2/project
  skillshare install https://gitlab.com/group/subgroup/project.git
  ```
  To specify a subdir within a multi-segment repo, use `.git` as the explicit boundary:
  ```bash
  # Clone group/subgroup/project, install from skills/my-skill subdir
  skillshare install gitlab.com/group/subgroup/project.git/skills/my-skill
  ```
  Non-GitLab hosts (GHE, Gitea, etc.) retain the original `owner/repo` + subdir behavior. GitLab web URLs with `/-/tree/` and Bitbucket `/src/` markers continue to work as before. `--track` mode generates correct names for subgroup paths (e.g., `group-subgroup-project`)
- **HTTPS fallback on non-GitLab hosts** — fixed platform-aware HTTPS URL parsing that could misroute GitHub Enterprise and Gitea URLs with subdirectory paths
- **Skill discovery in projects** — `skillshare install` now skips known AI tool config directories (`.claude/`, `.cursor/`, etc.) when scanning a project directory for skills, preventing circular discovery and false duplicates
- **Sync collision message** — `skillshare sync` now shows both duplicate skill names in collision warning messages for easier troubleshooting
- **Extras mode switch without `--force`** — changing an extra's sync mode (e.g., from `merge` to `copy`) and re-syncing now automatically replaces old symlinks. Previously, leftover symlinks from the old mode were treated as conflicts requiring `--force`

## [0.16.14] - 2026-03-09

### New Features

#### Terminal Rendering Improvements

- **SGR dim for consistent gray text** — all dim/gray text across CLI and TUI now uses the SGR dim attribute (`\x1b[0;2m`) instead of bright-black (`\033[90m`) or fixed 256-color grays. This adapts to any terminal theme — dark, light, or custom — instead of rendering too dark or invisible on certain configurations
- **Progress bar counter visibility** — the file counter (e.g. `0/63947`) now appears at a fixed position right after the percentage, preventing it from being pushed off-screen by long titles on narrow terminals:
  ```
  ■■■■■■■■■■■■･････ 69%  0/63947  Updating files
  ```
- **Progress bar accent color** — progress bar now uses cyan (the project accent color) instead of orange, matching spinners, titles, and other interactive elements

### Bug Fixes

- Fixed progress bar getting stuck at 99% on large scans (e.g. 63k+ skills) — parallel scan workers could race past the final frame, leaving the bar one tick short of 100%
- Fixed skill path segments (e.g. `security/` in `security/sarif-parsing`) rendering as fixed 256-color gray in TUI list and audit views — now uses theme-adaptive dim

## [0.16.13] - 2026-03-06

### New Features

#### TUI Grouped Layout

- **Grouped skill list** — `skillshare list` TUI now groups skills by tracked repo with visual separators. Each group shows the repo name and skill count. Standalone (local) skills appear in their own section. When only one group exists, separators are omitted for a cleaner view
  ```
  ── runkids-my-skills (42) ──────────────
    ✓ security/skill-improver
    ! security/audit-demo-debug-exfil
  ── standalone (27) ─────────────────────
    ! react-best-practices
  ```
- **Grouped audit results** — `skillshare audit` TUI uses the same grouped layout. Panel height dynamically adjusts based on footer content, maximizing visible rows
- **Structured filter tags** — filter skills precisely with `key:value` tags in the `/` filter input:
  ```
  t:tracked g:security audit
  → type=tracked AND group contains "security" AND free text "audit"
  ```
  Available tags: `t:`/`type:` (tracked/remote/local/github), `g:`/`group:` (substring), `r:`/`repo:` (substring). Multiple tags use AND logic. Tracked skills now show a repo-name badge so they remain identifiable even in filtered results without group headers

#### New Targets

- **3 new AI agent targets** — Warp, Purecode AI (`purecode`), and Witsy, bringing supported tools to 55+

### Bug Fixes

- Fixed long skill names wrapping to multiple lines in list and audit TUIs — names now truncate with `…` when exceeding column width
- Fixed items at the bottom of the audit TUI list being hidden behind the footer
- Fixed detail panel showing duplicate information (installed date, repo name repeated across sections)
- Reduced color noise in audit CLI and TUI output — non-zero counts use semantic severity colors, zero counts are dimmed
- Fixed devcontainer wrapper not suppressing redirect banner for `-j` short flag

## [0.16.12] - 2026-03-06

### New Features

#### Structured JSON Output

- **`--json` flag on 8 more commands** — structured JSON output for agent and CI/CD consumption, bringing total coverage to 12 commands:
  - Mutating: `sync`, `install`, `update`, `uninstall`, `collect`
  - Read-only: `target list`, `status`, `diff`
  ```bash
  skillshare status --json                          # overview as JSON
  skillshare list --json | jq '.[].name'            # extract skill names
  skillshare sync --json | jq '.details'            # per-target sync details
  skillshare install github.com/user/repo --json    # non-interactive install
  ```
  - For mutating commands, `--json` implies `--force` (skips interactive prompts)
  - Fully silent: no spinners, no stderr progress — only pure JSON on stdout
  - Previously supported: `audit --format json`, `log --json`, `check --json`, `list --json`
- **`status --project --json`** — project-mode status now supports `--json` output

### Bug Fixes

- Fixed `--json` mode leaking spinner and progress text to stderr, breaking `2>&1 | jq .` pipelines
- Fixed non-zero exit codes being swallowed in `--json` error paths
- Fixed `status --json` showing hardcoded analyzer list instead of actual active analyzers
- Fixed argument validation being skipped in `status --project` mode

### Performance

- **Parallelized git dirty checks** — `status --json` now runs git status checks concurrently across tracked repos

## [0.16.11] - 2026-03-05

### New Features

#### Supply-Chain Trust Verification

- **Metadata analyzer** — new audit analyzer that cross-references SKILL.md metadata against the actual git source URL to detect social-engineering attacks:
  - `publisher-mismatch` (HIGH): skill claims an organization (e.g., "by Anthropic") but repo owner differs
  - `authority-language` (MEDIUM): skill uses authority words ("official", "verified") from an unrecognized source
  ```bash
  skillshare audit                         # metadata analyzer runs by default
  skillshare audit --analyzer metadata     # run metadata analyzer only
  ```

#### Hardcoded Secret Detection

- **10 new audit rules** (`hardcoded-secret-0` through `hardcoded-secret-9`) detect inline API keys, tokens, and passwords embedded in skill files:
  - Google API keys, AWS access keys, GitHub PATs (classic + fine-grained), Slack tokens, OpenAI keys, Anthropic keys, Stripe keys, PEM private key blocks, and generic `api_key`/`password` assignments
  - Severity: HIGH — blocks installation at default threshold
  ```bash
  skillshare audit                         # hardcoded secrets detected automatically
  skillshare audit rules --pattern hardcoded-secret  # list all secret rules
  ```

#### Skill Integrity Verification

- **`doctor` integrity check** — verifies installed skills haven't been tampered with by comparing current file hashes against stored `.skillshare-meta.json` hashes:
  ```
  ✓ Skill integrity: 5/6 verified
  ⚠ _team-repo__api-helper: 1 modified
  ⚠ Skill integrity: 1 skill(s) unverifiable (no metadata)
  ```

#### Web UI Streaming & Virtualization

- **Real-time SSE streaming** — all long-running web dashboard operations (audit, update, check, diff) now stream results via Server-Sent Events with per-item progress bars instead of waiting for the full batch
- **Per-skill audit** — audit individual skills directly from the skill detail page
- **Virtualized scrolling** — audit results and diff item lists now use virtual scrolling for smooth performance with large datasets (replaces "Show more" pagination)

### Improvements

- **SSL error guidance** — `skillshare install` now detects SSL certificate errors and shows actionable options (custom CA bundle, SSH, or skip verification)
- **Cleaner TUI layout** — removed detail panel box borders in list/log views for a cleaner, less cluttered appearance

## [0.16.10] - 2026-03-04

### New Features

#### Sync Extras

- **`sync extras` subcommand** — sync non-skill resources (rules, commands, memory files, etc.) from your config directory to arbitrary target paths:
  ```bash
  skillshare sync extras              # sync all configured extras
  skillshare sync extras --dry-run    # preview without changes
  skillshare sync extras --force      # overwrite existing files
  ```
  Each extra supports per-target sync modes (`symlink`, `copy`, or `merge`). Configure in `config.yaml`:
  ```yaml
  extras:
    - name: rules
      targets:
        - path: ~/.claude/rules
        - path: ~/.cursor/rules
          mode: copy
  ```
- **`sync --all` flag** — run skill sync and extras sync together in one command:
  ```bash
  skillshare sync --all
  ```

#### TUI Preferences

- **`tui` subcommand** — persistently enable or disable interactive TUI mode:
  ```bash
  skillshare tui          # show current setting
  skillshare tui off      # disable TUI globally
  skillshare tui on       # re-enable TUI
  ```
  When disabled, all commands fall back to plain text output. Setting is stored in `config.yaml`.

### Bug Fixes

- Fixed TUI detail panel bottom content being clipped in list view

### Documentation

- Added sync extras documentation to website, built-in skill, and README
- Split monolith audit page into focused sub-pages for easier navigation

## [0.16.9] - 2026-03-03

### New Features

#### Audit Rules Management

- **`audit rules` subcommand** — browse, search, disable, enable, and override severity for individual rules or entire patterns:
  ```bash
  skillshare audit rules                          # interactive TUI browser
  skillshare audit rules --format json             # machine-readable listing
  skillshare audit rules disable credential-access-ssh-private-key
  skillshare audit rules disable --pattern prompt-injection
  skillshare audit rules severity my-rule HIGH
  skillshare audit rules reset                     # restore built-in defaults
  skillshare audit rules init                      # create starter audit-rules.yaml
  ```
- **Audit Rules TUI** — two-level interactive browser with accordion pattern groups, severity tabs (ALL/CRIT/HIGH/MED/LOW/INFO/OFF), text filter, and inline disable/enable/severity-override actions
- **Pattern-level rule overrides** — `audit-rules.yaml` now supports pattern-level entries (e.g., `prompt-injection: disabled: true`) that apply to all rules under a pattern

#### Security Policy & Deduplication

- **`--profile` flag** — preset security profiles that set block threshold and deduplication mode in one flag:
  ```bash
  skillshare audit --profile strict      # blocks on HIGH+, global dedupe
  skillshare audit --profile permissive  # blocks on CRITICAL only, legacy dedupe
  ```
  Profiles: `default` (CRITICAL threshold, global dedupe), `strict` (HIGH threshold, global dedupe), `permissive` (CRITICAL threshold, legacy dedupe)
- **`--dedupe` flag** — control finding deduplication: `global` (default) deduplicates across all skills using SHA-256 fingerprints; `legacy` keeps per-skill behavior
- **Policy display** — active policy (profile, threshold, dedupe mode) shown in audit header, summary box, and TUI footer

#### Analyzer Pipeline

- **`--analyzer` flag** — run only specific analyzers (repeatable): `static`, `dataflow`, `tier`, `integrity`, `structure`, `cross-skill`:
  ```bash
  skillshare audit --analyzer static --analyzer dataflow
  ```
- **Finding enrichment** — JSON, SARIF, and Markdown outputs now include `ruleId`, `analyzer`, `category`, `confidence`, and `fingerprint` fields per finding
- **Category-based threat breakdown** — summary now shows threat counts by category (injection, exfiltration, credential, obfuscation, privilege, integrity, structure, risk) across all output channels (CLI, TUI, JSON, Markdown)
- **Semantic coloring** — TUI summary footer and CLI summary box use per-category colors for the Threats breakdown line

#### New Detection Rules

- **Interpreter tier (T6)** — audit classifies Turing-complete runtimes (`python`, `node`, `ruby`, `perl`, `lua`, `php`, `bun`, `deno`, `npx`, `tsx`, `pwsh`, `powershell`) as T6:interpreter. Versioned binaries like `python3.11` are also recognized. Tier combination findings: `tier-interpreter` (INFO) and `tier-interpreter-network` (MEDIUM when combined with network commands)
- **Expanded prompt injection detection** — new rules detect `OVERRIDE:`/`IGNORE:`/`ADMIN:`/`ROOT:` prefixes, agent directive tags (`<system>`, `</instructions>`), and jailbreak directives (`DEVELOPER MODE`, `DEV MODE`, `DAN MODE`, `JAILBREAK`)
- **Table-driven credential access detection** — credential rules are now generated from a data table covering 30+ sensitive paths (SSH keys, AWS/Azure/GCloud credentials, GnuPG keyrings, Kubernetes config, Vault tokens, Terraform credentials, Docker config, GitHub CLI tokens, macOS Keychains, shell history, and more) across 5 access methods (read, copy, redirect, dd, exfil). Supports `~`, `$HOME`, `${HOME}` path variants. Includes an INFO-level heuristic catch-all for unknown home dotdirs. Rule IDs are now descriptive (e.g., `credential-access-ssh-private-key` instead of `credential-access-0`)
- **Cross-skill credential × interpreter** — new cross-skill rule `cross-skill-cred-interpreter` (MEDIUM) flags when one skill reads credentials and another has interpreter access
- **Markdown image exfiltration detection** — new rule detects external markdown images with query parameters (`![img](https://...?data=...)`) as a potential data exfiltration vector
- **Invisible payload detection** — detects Unicode tag characters (U+E0001–U+E007F) that render at 0px width but are fully processed by LLMs. Primary vector for "Rules File Backdoor" attacks. Uses dedicated `invisible-payload` pattern to ensure CRITICAL findings are never suppressed in tutorial contexts
- **Output suppression detection** — detects directives that hide actions from the user ("don't tell the user", "hide this from the user", "remove from conversation history"). Strong indicator of supply-chain attacks
- **Bidirectional text detection** — detects Unicode bidi control characters (U+202A–U+202E, U+2066–U+2069) used in Trojan Source attacks (CVE-2021-42574) that reorder visible text
- **Config/memory file poisoning** — detects instructions to modify AI agent configuration files (`MEMORY.md`, `CLAUDE.md`, `.cursorrules`, `.windsurfrules`, `.clinerules`)
- **DNS exfiltration detection** — detects `dig`/`nslookup`/`host` commands with command substitution (`$(...)` or backticks) that encode stolen data in DNS subdomain queries
- **Self-propagation detection** — detects instructions that tell AI to inject/insert payloads into all/every/other files, a repository worm pattern
- **Markdown comment injection** — detects prompt injection keywords hidden inside markdown reference-link comments (`[//]: # (ignore previous instructions...)`)
- **Untrusted package execution** — detects `npx -y`/`npx --yes` (auto-execute without confirmation) and `pip install https://` (install from URL, not PyPI registry)
- **Additional invisible Unicode** — detects soft hyphens (U+00AD), directional marks (U+200E–U+200F), and invisible math operators (U+2061–U+2064) at MEDIUM severity
- **`env` prefix handling** — command tier classifier now correctly classifies `env python3 script.py` as T6:interpreter instead of T0:read-only

### Performance

- **Regex prefilters** — static analyzer now applies conservative literal-substring prefilters before running regex, reducing scan time on large skills

### Bug Fixes

- **Regex bypass vulnerabilities closed** — fixed prompt injection rules that could be bypassed with leading whitespace or mixed case; fixed data-exfiltration image rule whose exclude pattern allowed `.png?stolen_data` to pass; fixed `dd if=/etc/shadow` being mislabeled as `destructive-commands` instead of `credential-access`
- **SSH public key false positive** — `~/.ssh/id_rsa.pub` and other `.pub` files no longer trigger CRITICAL credential-access findings (only private keys are flagged)
- **Catch-all regex bypass** — fixed heuristic catch-all rule that could be silenced when a known credential path appeared on the same line as an unknown dotdir
- **Structured output ANSI leak** — `audit --format json/sarif/markdown` no longer leaks pterm cursor hide/show ANSI codes into stdout
- **Severity-only merge no longer wipes rules** — editing only severity in `audit-rules.yaml` no longer drops the rule's regex patterns
- **Profile threshold fallback** — profile presets now correctly set block threshold when config has no explicit `block_threshold`
- **TreeSpinner ghost cursor** — fixed missing `WithWriter` that caused cursor hide/show codes to leak on structured output
- **TUI summary overflow** — category threat breakdown now renders on a separate line to prevent horizontal overflow on narrow terminals

## [0.16.8] - 2026-03-02

### New Features

- **`audit --format`** — new `--format` flag supports `text` (default), `json`, `sarif`, and `markdown` output formats. `--json` is now deprecated:
  ```bash
  skillshare audit --format sarif     # SARIF 2.1.0 for GitHub Code Scanning
  skillshare audit --format markdown  # Markdown report for GitHub Issues/PRs
  skillshare audit --format json      # Machine-readable JSON
  ```
- **Analyzability score** — each audited skill now receives an analyzability percentage (how much of the skill's content can be statically analyzed). Shown per-skill in audit output and as an average in the summary
- **Command safety tiering (T0–T5)** — audit classifies shell commands by behavioral tier: T0 read-only, T1 mutating, T2 destructive, T3 network, T4 privilege, T5 stealth. Tier labels appear alongside pattern-based findings for richer context
- **Dataflow taint tracking** — audit detects cross-line exfiltration patterns: credential reads or environment variable access on one line followed by network sends (`curl`, `wget`, etc.) on a subsequent line
- **Cross-skill interaction detection** — when auditing multiple skills, audit now checks for dangerous capability combinations across skills (e.g., one skill reads credentials while another has network access). Results are also exposed in the REST API (`GET /api/audit`)
- **Audit TUI filter** — the `/` filter in the audit TUI now searches across risk level, status (blocked/warning/clean), max severity, finding pattern names, and file names — not just skill names
- **Pre-commit hook** — `.pre-commit-hooks.yaml` for the [pre-commit](https://pre-commit.com/) framework. Runs `skillshare audit -p` on every commit to catch security issues before they land:
  ```yaml
  repos:
    - repo: https://github.com/runkids/skillshare
      rev: v0.16.8
      hooks:
        - id: skillshare-audit
  ```
- **AstrBot target** — new target for AstrBot AI assistant (`~/.astrbot/data/skills`)
- **Cline target updated** — Cline now uses the universal `.agents/skills` project path

### Performance

- **Cross-skill analysis O(N) rewrite** — cross-skill interaction detection rewritten from O(N²) pair-wise comparison to O(N) capability-bucket approach, significantly faster for large skill collections

### Bug Fixes

- **TUI gray text contrast** — improved gray text readability on dark terminals by increasing ANSI color contrast
- **Spinner on structured output** — `audit` now shows progress spinner on stderr when using `--format json/sarif/markdown`, so structured stdout remains clean for piping
- **SARIF line-0 region** — SARIF output no longer emits an invalid `region` object for findings at line 0

## [0.16.7] - 2026-03-02

### Bug Fixes

- **Preserve external symlinks during sync** — sync (merge/copy mode) no longer deletes target directory symlinks created by dotfiles managers (e.g., stow, chezmoi, yadm). Previously, switching from symlink mode to merge/copy mode would unconditionally remove the target symlink, breaking external link chains. Now skillshare checks whether the symlink points to the source directory before removing it — external symlinks are left intact and skills are synced into the resolved directory
- **Symlinked source directory support across all commands** — all commands that walk the source directory (`sync`, `update`, `uninstall`, `list`, `diff`, `install`, `status`, `collect`) now resolve symlinks before scanning. Skills managed through symlinked `~/.config/skillshare/skills/` (common with dotfiles managers) are discovered correctly everywhere. Chained symlinks (link → link → real dir) are also handled
- **Group operation containment guard** — `uninstall --group` and `update --group` now reject group directories that are symlinks pointing outside the source tree, preventing accidental operations on external directories
- **`status` recognizes external target symlinks** — `CheckStatusMerge` no longer reports external symlinks as "conflict"; it follows the symlink and counts linked/local skills in the resolved directory
- **`collect` scans through external target symlinks** — `FindLocalSkills` now follows non-source symlinks instead of skipping them, so local skills in dotfiles-managed target directories can be collected
- **`upgrade` prompt cleanup** — upgrade prompts ("Install built-in skill?" and "Upgrade to vX?") no longer leave residual lines that break the tree-drawing layout

## [0.16.6] - 2026-03-02

### New Features

- **`diff` interactive TUI** — new bubbletea-based split-panel interface for `skillshare diff`: left panel lists targets with status icons (✓/!/✗), right panel shows categorized file-level diffs for the selected target. Supports fuzzy filter (`/`), detail scrolling (`Ctrl+d/u`), and narrow terminal fallback. Add `--no-tui` for plain text output
- **`diff --patch`** — show unified text diffs for each changed file:
  ```
  skillshare diff --patch
  ```
- **`diff --stat`** — show per-file change summary with added/removed line counts:
  ```
  skillshare diff --stat
  ```
- **`diff` file-level detail** — diff entries now include per-file data (added/removed/modified/renamed), source paths, modification times, and git-style status symbols (`+`/`−`/`≠`/`→`)
- **`diff` statistics summary** — every diff run prints a summary line with total counts by category (e.g., `3 added, 1 modified, 2 removed`)
- **Glob pattern matching** — `install`, `update`, and `uninstall` now accept glob patterns (`*`, `?`, `[...]`) in skill name arguments; matching is case-insensitive:
  ```bash
  skillshare install repo -s "core-*"
  skillshare update "team-*"
  skillshare uninstall "old-??"
  ```
- **`trash` interactive TUI** — bubbletea-based TUI with multi-select, fuzzy filter, and inline restore/delete/empty operations; includes SKILL.md preview in the detail panel
- **`restore` interactive TUI** — two-phase TUI: target picker → version list with left-right split panel, showing skill diffs and descriptions in the detail panel. Add `--help` flag and delete-backup action from TUI
- **`backup` version listing** — `backup` now lists available backup versions per target and correctly follows top-level symlinks in merge-mode targets
- **Homebrew-aware version check** — Homebrew users no longer see false "update available" notifications; `doctor` and post-command checks now query `brew info` instead of the GitHub Release API when installed via Homebrew
- **Devcontainer skill** — new built-in skill that teaches AI assistants when and how to run CLI commands, tests, and debugging inside the devcontainer
- **Red destructive confirmations** — all destructive action confirmations (delete, empty, uninstall) now render in red across trash, restore, and list TUIs

### Fixed

- **`backup`/`restore` mode flags** — `-g` and `-p` flags now work correctly; previously `-g` was misinterpreted as a target name
- **`diff` hides internal metadata** — `.skillshare-meta.json` is no longer shown in file-level diff output
- **`diff --stat` implies `--no-tui`** — `--stat` now correctly skips the TUI and prints to stdout

## [0.16.5] - 2026-02-28

### New Features

- **Web UI: Dark theme** — toggle between light and dark mode via the sun/moon button; persists to localStorage and respects `prefers-color-scheme`
- **Web UI: Update page** — dedicated page for batch-updating tracked skills with select-all, per-item progress tracking, and result summary
- **Web UI: Security overview card** — dashboard now shows a risk-level badge and severity breakdown; highlights critical findings with an accent card
- **Web UI: Sync mode selector** — change a target's sync mode (merge/symlink) directly from the Targets page dropdown
- **Web UI: Install skill picker** — skill descriptions from SKILL.md frontmatter are now shown inline in the picker modal; search also matches descriptions
- **`upgrade` version transition** — `skillshare upgrade` now shows clear before/after versions:
  ```
  Upgraded  v0.16.3 → v0.16.5
  ```
  Works for Homebrew, direct download, and skill installs

### Fixed

- **Custom targets flagged as unknown** — `check` and `doctor` no longer warn about user-defined targets in global or project config (fixes [#57](https://github.com/runkids/skillshare/issues/57))
- **Web UI: Modal scroll-away** — clicking checkboxes in the skill picker no longer causes content to scroll out of view (replaced `overflow-hidden` with `overflow-clip`)
- **Web UI: Subdir URL discovery** — install form now correctly discovers skills from git subdirectory URLs
- **Web UI: Accessibility** — added `aria-labels`, `htmlFor`, focus trap for modals, and `ErrorBoundary` for graceful error recovery

### New Targets

- **omp** — [oh-my-pi](https://github.com/can1357/oh-my-pi) (`~/.omp/agent/skills`, `.omp/skills`; alias: `oh-my-pi`)
- **lingma** — [Lingma](https://help.aliyun.com/zh/lingma/user-guide/skills) (`~/.lingma/skills`, `.lingma/skills`)

## [0.16.4] - 2026-02-28

### New Features

- **Cross-path duplicate detection** — `install` now detects when a repo is already installed at a different location and blocks the operation with a clear hint:
  ```bash
  skillshare install runkids/feature-radar --into feature-radar
  # later...
  skillshare install runkids/feature-radar
  # ✗ this repo is already installed at skills/feature-radar/scan (and 2 more)
  #   Use 'skillshare update' to refresh, or reinstall with --force to allow duplicates
  ```
- **Same-repo skip** — reinstalling a skill from the same repo now shows a friendly `⊘ skipped` indicator instead of an error; skipped skills are grouped by directory with repo label in the summary
- **Web UI install dedup** — the Web UI install endpoints enforce the same cross-path duplicate check as the CLI, returning HTTP 409 when duplicates are found
- **5 new audit rules** — the security scanner now detects 36 patterns (up from 31):
  - `fetch-with-pipe` (HIGH) — detects `curl | bash`, `wget | sh`, and pipes to `python`, `node`, `ruby`, `perl`, `zsh`, `fish`
  - `ip-address-url` (MEDIUM) — URLs with raw IP addresses that bypass DNS-based security; private/loopback ranges excluded
  - `data-uri` (MEDIUM) — `data:` URIs in markdown links that may embed executable content
- **Unified batch summary** — `install`, `uninstall`, and `update` now share a consistent single-line summary format with color-coded counts and elapsed time

### Performance

- **Batch gitignore operations** — `.gitignore` updates during `install` reconciliation and `uninstall` are now batched into a single file read/write instead of one per skill; eliminates hang when `.gitignore` grows large (100K+ lines)
- **`update --all` grouped skip** — skills from the same repo are now skipped when installed metadata already matches remote state (commit or tree-hash match), avoiding redundant reinstall/copy; on large repos this eliminates the majority of work
- **`update --all` batch speed** — removed a fixed 50ms per-skill delay in grouped batch iteration that dominated runtime on large skill sets (~90 min at 108K skills → seconds)
- **`update --all` progress visibility** — batch progress bar now advances per-skill instead of per-repo, so it no longer appears stuck at 0% during large grouped updates; a scanning spinner and phase headers (`[1/3] Pulling N tracked repos...`) show which stage is running
- **`status` and `doctor` at scale** — both commands now run a single skill discovery pass instead of repeating it per-section (status: 7× → 1×, doctor: 5× → 1×); target status checks are cached so drift detection reuses the first result; `doctor` overlaps its GitHub version check with local I/O; a spinner is shown during discovery so the CLI doesn't appear frozen
- **`collect` scan speed** — directory size calculation is no longer run eagerly during skill discovery; deferred to the Web UI handler where it is actually needed

### Fixed

- **`universal` target path** — corrected global path from `~/.config/agents/skills` to `~/.agents/skills` (the shared agent directory used by multiple AI CLIs)
- **`init` auto-includes `universal`** — `init` and `init --discover` now automatically include the `universal` target whenever any AI CLI is detected; labeled as "shared agent directory" so users understand what it is
- **`universal` coexistence docs** — added FAQ section explaining how skillshare and `npx skills` coexist on the same `~/.agents/skills` path, including sync mode differences and name collision caveats
- **`--force` hint accuracy** — the force hint now uses the actual repo URL (not per-skill subpath) and includes `--into` when applicable
- **`update` root-level skills** — root-level skill repos (SKILL.md at repo root) no longer appear as stale/deleted during batch update; fixed `Subdir` normalization mismatch between metadata (`""`) and discovery (`"."`)
- **`pull` project mode leak** — `pull` now forces `--global` for the post-pull sync, preventing unintended project-mode auto-detection when run inside a project directory
- **`list` TUI action safety** — `audit`, `update`, and `uninstall` actions in the skill list TUI now show a confirmation overlay before executing; actions pass explicit `--global`/`--project` mode flags to prevent mode mismatch

### Improvements

- **`update` batch summary** — batch update summary now uses the same single-line stats format as `sync` with color-coded counts
- **Command output spacing** — commands now consistently print a trailing blank line after output for better terminal readability

## [0.16.3] - 2026-02-27

### Improvements

- **`diff` output redesign** — actions are now labeled by what they do (`add`, `remove`, `update`, `restore`) with a grouped summary showing counts per action; overall summary line at the end
- **Install progress output** — config and search installs now show tree-style steps with a summary line (installed/skipped/failed counts + elapsed time) and real-time git clone progress
- **Web UI log stats bar** — Log page now shows a stats bar with success rate and per-command breakdown
- **Hub batch install progress** — multi-skill installs from `search --hub` now show real-time git clone progress (`cloning 45%`, `resolving 67%`) instead of a static "installing..." label; only the active install is shown to keep the display compact
- **Hub risk badge colors** — risk labels in hub search results are now color-coded by severity (green for clean, yellow for low, red for critical) in both the list and detail panel
- **Hub batch failure output** — failure details are classified by type (security / ambiguous / not found) with distinct icons; long audit findings and ambiguous path lists are truncated to 3 lines with a "(+N more)" summary

### Performance

- **Batch install reconcile** — config reconciliation now runs once after all installs complete instead of after each skill, eliminating O(n²) directory walks that caused batch installs of large collections to appear stuck
- **Repo-grouped cloning** — skills from the same git repo are now cloned once and installed from the shared clone, reducing network requests for multi-skill repos

### Fixed

- **Race condition in `sync`** — targets sharing the same filesystem path no longer produce duplicate or missing symlinks
- **Race condition in `sync` group key** — canonicalized group key prevents non-deterministic sync results
- **Web UI stats on "All" tab** — dashboard now computes stats from both ops and audit logs, not just ops
- **Web UI last operation timestamp** — timestamps are compared as dates instead of strings, fixing incorrect "most recent" ordering
- **`log --stats --cmd audit`** — now correctly reads from `audit.log` instead of `operations.log`
- **`log max_entries: 0`** — setting max_entries to 0 now correctly means unlimited instead of deleting all entries
- **Oplog data loss** — rewriteEntries now checks for write errors before truncating the original file
- **TUI content clipping** — detail panels in `list` and `log` TUIs now hard-wrap content and account for padding, preventing text from being clipped at panel edges
- **TUI footer spacing** — list and log TUI footers have proper breathing room between action hints
- **Copy mode symlink handling** — `sync` in copy mode now dereferences directory symlinks instead of copying broken link files; prevents missing content in targets like Windsurf that use file copying
- **`uninstall --all` stale summary** — spinner and confirm prompt now show correct noun type after skipping dirty tracked repos; added skip count message ("1 tracked repo skipped, 2 remaining"); fixed unnatural pluralization ("2 group(s)" → "2 groups")
- **Empty `list` / `log` TUI** — `list` and `log` no longer open a blank interactive screen when there are no skills or log entries; they print a plain-text hint instead
- **`install` quiet mode** — tracked config dry-run messages are now suppressed in quiet mode

### New Targets

- **Verdent** — added [Verdent](https://www.verdent.ai/) AI coding agent (`verdent`)

## [0.16.2] - 2026-02-26

### New Features

- **`diff` command** — new command to preview what `sync` would change without modifying anything; parallel target scanning, grouped output for targets with identical diffs, and an overall progress bar:
  ```bash
  skillshare diff              # all targets
  skillshare diff claude       # single target
  skillshare diff -p           # project mode
  ```
- **Interactive TUI for `audit`** — `skillshare audit` launches a bubbletea TUI with severity-colored results, fuzzy filter, and detail panel; progress bar during scanning; confirmation prompt for large scans (1,000+ skills) (`skillshare audit --no-tui` for plain text)
- **Tree sidebar in `list` TUI** — detail panel now shows the skill's directory tree (up to 3 levels) with glamour-rendered markdown preview; SKILL.md pinned at top for quick reading
- **Log TUI: delete entries** — press `space` to select entries, `d` to delete with confirmation; supports multi-select (`a` to select all)
- **Log `--stats` flag** — aggregated summary with per-command breakdown, success rate, and partial/blocked status tracking:
  ```bash
  skillshare log --stats
  ```
- **Azure DevOps URL support** — install from Azure DevOps repos using `ado:` shorthand, full HTTPS (`dev.azure.com`), legacy HTTPS (`visualstudio.com`), or SSH v3 (`ssh.dev.azure.com`) URLs:
  ```bash
  skillshare install ado:myorg/myproject/myrepo
  skillshare install https://dev.azure.com/org/proj/_git/repo
  skillshare install git@ssh.dev.azure.com:v3/org/proj/repo
  ```
- **`AZURE_DEVOPS_TOKEN` env var** — automatic HTTPS token injection for Azure DevOps private repos, same pattern as `GITHUB_TOKEN` / `GITLAB_TOKEN` / `BITBUCKET_TOKEN`:
  ```bash
  export AZURE_DEVOPS_TOKEN=your_pat
  skillshare install https://dev.azure.com/org/proj/_git/repo --track
  ```
- **`update --prune`** — remove stale skills whose upstream source no longer exists (`skillshare update --prune`)
- **Stale detection in `check`** — `skillshare check` now reports skills deleted upstream as "stale (deleted upstream)" instead of silently skipping them
- **Windows ARM64 cross-compile** — `make build-windows` / `mise run build:windows` produces Windows ARM64 binaries

### Performance (large skill collections)

- **Parallel target sync** — both global and project-mode `sync` now run target syncs concurrently (up to 8 workers) with a live per-target progress display
- **mtime fast-path for copy mode** — repeat syncs skip SHA-256 checksums when source directory mtime is unchanged, making no-op syncs near-instant
- **Cached skill discovery** — skills are discovered once and shared across all parallel target workers instead of rediscovering per target

### Improvements

- **Batch progress for hub installs** — multi-skill installs from `search` now show per-skill status (queued/installing/done/error) with a live progress display
- **Log retention** — operation log auto-trims old entries with configurable limits and hysteresis to avoid frequent rewrites
- **Partial completion tracking** — `sync`, `install`, `update`, and `uninstall` now log `"partial"` status when some targets succeed and others fail, instead of a blanket `"error"`
- **Unified TUI color palette** — all bubbletea TUIs share a consistent color palette via shared `tc` struct

### Website

- **Documentation restructure** — website now follows Diátaxis IA (getting-started / how-to / learn / understand / reference / troubleshooting)
- **Blog launch** — 5 launch posts covering tutorials, recipes, and migration guides

### Fixed

- **`upgrade` spinner nesting** — brew output and GitHub release download steps now render cleanly inside tree spinners instead of breaking the layout

## [0.16.1] - 2026-02-25

### Improvements

- **Async TUI loading for `list`** — skill list now loads inside the TUI with a spinner instead of blocking before rendering; metadata reads use a parallel worker pool (64 workers) for faster startup
- **Unified filter bar across all TUIs** — `list`, `log`, and `search` now share the same filter UX: press `/` to enter filter mode, `Esc` to clear, `Enter` to lock; search TUI suppresses action keys while typing to avoid accidental checkbox toggles
- **Colorized audit output** — severity counts (CRITICAL/HIGH/MEDIUM/LOW/INFO), risk labels, and finding details are now color-coded by severity level
- **Improved install output** — single-skill and tracked-repo installs show inline tree steps (description, license, location) instead of a separate SkillBox; description truncation increased to 100 characters with visible ellipsis (`…`)
- **Parallel uninstall discovery** — `uninstall --all` uses parallel git dirty checks (8 workers) for faster execution

### Fixed

- **Frozen terminal during `check` and `update`** — header and spinners now appear immediately before filesystem scans, so users see feedback instead of a blank screen
- **Spinner flicker during `install` clone** — eliminated visual glitch when transitioning between clone and post-clone phases
- **Large operation log files crash `log` TUI** — JSONL parser now uses streaming `json.Decoder` instead of reading entire lines into memory, handling arbitrarily large log entries

## [0.16.0] - 2026-02-25

### Performance

- **Per-skill tree hash comparison for `check`** — `skillshare check` now uses blobless git fetches (~150-200 KB) and compares per-skill directory tree hashes instead of whole-commit hashes; detects updates to individual skills within monorepos without downloading full history ([#46](https://github.com/runkids/skillshare/issues/46))
- **Parallel checking with bounded concurrency** — `check` and `check --all` run up to 8 concurrent workers; deduplicates `ls-remote` calls for repos hosting multiple skills; progress bar now shows skill count instead of URL count ([#46](https://github.com/runkids/skillshare/issues/46))
- **Sparse checkout for subdir installs** — `install owner/repo/subdir` uses `git sparse-checkout` (git 2.25+) to clone only the needed subdirectory with `--filter=blob:none`; falls back to full clone on older git versions (fixes [#46](https://github.com/runkids/skillshare/issues/46))
- **Batch update progress** — `update --all` now shows a progress bar with the current skill name during batch operations

### New Features

- **Interactive TUI for `list`** — `skillshare list` launches a bubbletea TUI with fuzzy search, filter, sort, and a detail panel showing description, license, and metadata; inline actions: audit, update, and uninstall directly from the list (`skillshare list --no-tui` for plain text)
- **Interactive TUI for `log`** — `skillshare log` launches a bubbletea TUI with fuzzy filter and detail panel for browsing operation history (`skillshare log --no-tui` for plain text)
- **Interactive TUI for `search`** — `skillshare search` results now use a bubbletea multi-select checkbox interface instead of survey prompts
- **Interactive TUI for `init`** — target selection in `skillshare init` now uses a bubbletea checklist with descriptions instead of survey multi-select
- **Skill registry separation** — installed skill metadata moved from `config.yaml` to `registry.yaml`; `config.yaml` remains focused on user settings (targets, audit thresholds, custom targets); silent auto-migration on first v0.16.0 run — no user action required
- **Project-mode skills for this repo** — `.skillshare/skills/` ships 5 built-in project skills for contributors: `cli-e2e-test`, `codebase-audit`, `implement-feature`, `update-docs`, `changelog`; install with `skillshare sync -p` in the repo
- **Restore validation preview** — Web UI restore modal now shows a pre-restore validation with conflict warnings, backup size, and symlink detection before committing (`POST /api/restore/validate`)
- **Expanded detail panel in `list` TUI** — detail view now includes word-wrapped description and license field

### Changed

- **CLI visual language overhaul** — all single-item operations (install, update, check) now use a consistent hierarchical layout with structured labels (`Source:`, `Items:`, `Skill:`) and adaptive spinners; audit findings section only appears when findings exist
- **`check` single-skill output** — single skill/repo checks now use the same hierarchical tree layout as `update` with spinner and step results instead of a progress bar
- **`check` summarizes clean results** — up-to-date and local-only skills are now shown as summary counts (e.g., "3 up to date, 2 local") instead of listing each one individually
- **Symlink compat hint moved to `doctor`** — per-target mode hints removed from `sync` output; `doctor` now shows a universal symlink compatibility notice when relevant targets are configured
- **Web UI migrated to TanStack Query** — all API calls use `@tanstack/react-query` with automatic caching, deduplication, and background refetching; Skills page uses virtual scrolling for large collections
- **Deprecated `openclaude` target removed** — replaced by `openclaw`; existing configs using `openclaude` should update to `openclaw`

### Fixed

- **Infinite loop in directory picker for large repos** — bubbletea directory picker now handles repos with many subdirectories without hanging
- **Leading slash in subdir path breaks tree hash lookup** — `check` now normalizes `//skills/foo` to `skills/foo` for consistent path matching
- **`update --all` in project mode skipped nested skills** — recursive skill discovery now enabled for project-mode `update --all`
- **Batch update path duplication** — `update --all` now uses caller-provided destination paths to prevent doubled path segments
- **`file://` URL subdir extraction** — `install file:///path/to/repo//subdir` now correctly extracts subdirectories via the `//` separator
- **Git clone progress missing in batch update** — progress output now wired through to batch update operations
- **Backup restore with symlinks** — `ValidateRestore` now uses `os.Lstat` to correctly detect symlink targets instead of following them

## [0.15.5] - 2026-02-23

### Added
- **`init --mode` flag** — `skillshare init --mode copy` (or `-m copy`) sets the default sync mode for all targets at init time; in interactive mode (TTY), a prompt offers merge / copy / symlink selection; `init --discover --mode copy` applies the mode only to newly added targets, leaving existing targets unchanged (closes [#42](https://github.com/runkids/skillshare/issues/42))
- **Per-target sync mode hint** — after `sync` and `doctor`, a contextual hint suggests `copy` mode for targets known to have symlink compatibility issues (Cursor, Antigravity, Copilot, OpenCode); suppressed when only symlink-compatible targets are configured
- **`uninstall --all`** — remove all skills from source in one command; requires confirmation unless `--force` is set; works in both global and project mode

### Changed
- **Improved CLI output** — compact grouped audit findings (`× N` dedup), structured section labels, lighter update headers

### Fixed
- **Orphan real directories not pruned after uninstall** — `sync` in merge mode now writes `.skillshare-manifest.json` to track managed skills; after `uninstall`, orphan directories (non-symlinks) that appear in the manifest are safely removed instead of kept with "unknown directory" warnings; user-created directories not in the manifest are still preserved (fixes [#45](https://github.com/runkids/skillshare/issues/45))
- **Exclude filter not removing managed real directories** — changing `exclude` patterns now correctly prunes previously-managed real directories (not just symlinks) from targets; manifest entries are cleaned up to prevent stale ownership
- **MultiSelect filter text cleared after selection** — filter text is now preserved after selecting an item in interactive prompts (e.g., `install` skill picker)

## [0.15.4] - 2026-02-23

### Added
- **Post-update security audit gate** — `skillshare update` now runs a security audit after pulling tracked repositories; findings at or above the active threshold trigger rollback/block; interactive mode prompts for confirmation, non-interactive mode (CI) fails closed; use `--skip-audit` to bypass
- **Post-install audit gate for `--track`** — `skillshare install --track` and tracked repo updates now run the same threshold-based security gate; fresh installs are removed on block, updates are rolled back via `git reset`; use `--skip-audit` to bypass
- **Threshold override flags on `update`** — `skillshare update` now supports `--audit-threshold`, `--threshold`, `-T` (including shorthand aliases like `-T h`) for per-command blocking policy
- **`--diff` flag for `update`** — `skillshare update team-skills --diff` shows a file-level change summary after update; for tracked repos, includes line counts via `git diff`; for regular skills, uses file hash comparison to show added/modified/deleted files
- **Content hash pinning** — `install` and `update` now record SHA-256 hashes of all skill files in `.skillshare-meta.json`; subsequent `audit` runs detect tampering (`content-tampered`), missing files (`content-missing`), and unexpected files (`content-unexpected`)
- **`source-repository-link` audit rule** (HIGH) — detects markdown links labeled "source repo" or "source repository" pointing to external URLs, which may be used for supply-chain redirect attacks
- **Structural markdown link parsing for audit** — audit rules now use a full markdown parser instead of regex, correctly handling inline links with titles, reference-style links, autolinks, and HTML anchors while skipping code fences, inline code spans, and image links; reduces false positives in `external-link` and `source-repository-link` rules (extends link-audit foundation from [#39](https://github.com/runkids/skillshare/pull/39))
- **Severity-based risk floor** — audit risk label is now the higher of the score-based label and a floor derived from the most severe finding (e.g., a single HIGH finding always gets at least a `high` risk label)
- **Severity-based color ramp** — audit output now uses consistent color coding: CRITICAL → red, HIGH → orange, MEDIUM → yellow, LOW/INFO → gray; applies to batch summary, severity counts, and single-skill risk labels
- **Audit risk score in `update` output** — CLI and Web UI now display the risk label and score (e.g., "Security: LOW (12/100)") after updating regular skills; Web UI toast notifications include the same information for all update types

### Fixed
- **Uninstall group directory config cleanup** — uninstalling a group directory (e.g., `frontend/`) now properly removes member skill entries (e.g., `frontend/react`, `frontend/vue`) from `config.yaml` via prefix matching
- **Batch `update --all` error propagation** — repos blocked by the security audit gate now count as "Blocked" in the batch summary and cause non-zero exit code
- **`--skip-audit` passthrough** — the flag is now consistently honored for both tracked repos and regular skills during `update` and `install`
- **Server rollback error reporting** — Web UI update endpoint now implements post-pull threshold gate with automatic rollback on findings at/above threshold
- **Audit rollback error accuracy** — rollback failures now report whether the reset succeeded ("rolled back") or failed ("malicious content may remain") instead of silently ignoring errors
- **Audit error propagation** — file hash computation now propagates walk/hash errors instead of silently skipping, ensuring complete integrity baselines

## [0.15.3] - 2026-02-22

### Added
- **Multi-name and `--group` for `audit`** — `skillshare audit a b c` scans multiple skills at once; `--group`/`-G` flag scans all skills in a group directory (repeatable); names and groups can be mixed freely (e.g. `skillshare audit my-skill -G frontend`)
- **`external-link` audit rule** (closes #38) — new `external-link-0` rule (LOW severity) detects external URLs in markdown links (`[text](https://...)`) that may indicate prompt injection vectors or unnecessary token consumption; localhost and loopback links are excluded; completes #38 together with dangling-link detection from v0.15.1 (supersedes #39)
- **Auth tokens for hub search** — `search --hub` now automatically uses `GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`, or `SKILLSHARE_GIT_TOKEN` when fetching private hub indexes; no extra configuration needed

### Changed
- **`pull` merges by default** — when both local and remote have skills on first pull, `pull` now attempts a git merge instead of failing; if the merge has conflicts, it stops with guidance; `--force` still replaces local with remote
- **Parallel audit scanning** — `skillshare audit` (all-skills scan) now runs up to 8 concurrent workers for faster results in both CLI and Web UI

### Fixed
- **`audit` resolves nested skill names** — `skillshare audit nested__skill` now correctly finds skills by flat name or basename with short-name fallback
- **CodeX SKILL.md description over 1024 chars** (fixes #40) — built-in skill description trimmed to stay within CodeX's 1024-character limit

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
