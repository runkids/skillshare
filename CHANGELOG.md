# Changelog

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
