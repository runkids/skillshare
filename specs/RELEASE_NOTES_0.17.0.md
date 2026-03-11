# skillshare v0.17.0 Release Notes

Release date: 2026-03-11

## TL;DR

v0.17.0 promotes **extras** from a sync sub-feature to a **first-class citizen** with its own command group, project mode support, and Web UI page:

1. **`extras` command group** — `init`, `list`, `remove`, `collect` subcommands for managing non-skill resources (rules, prompts, commands)
2. **Interactive TUI wizard** — `extras init` with step-by-step name → target → mode flow
3. **Project mode** — all extras commands support `-p` for `.skillshare/`-scoped extras
4. **Deep integration** — `status`, `doctor`, `diff`, `sync` all gain extras awareness
5. **Web UI** — new Extras management page + Dashboard card + REST API endpoints
6. **Auto-migration** — legacy flat directories (`configDir/rules/`) automatically move to `configDir/extras/rules/`

### Breaking Change

Extras source directory structure changed from flat to nested:
```
# Before (v0.16.x)
~/.config/skillshare/rules/

# After (v0.17.0)
~/.config/skillshare/extras/rules/
```
**Auto-migration** runs on first `sync extras` — no manual action needed.

---

## New `extras` Command Group

### `extras init`

Create a new extra resource type. Two modes:

```bash
# Interactive wizard
skillshare extras init

# CLI flags
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules
skillshare extras init prompts --target .claude/prompts --mode copy -p
```

The TUI wizard walks through: name → target path → sync mode → add more targets → confirm.

### `extras list`

View all extras with sync status per target:

```bash
skillshare extras list
skillshare extras list --json -p
```

Status values: `synced`, `drift`, `not synced`, `no source`.

### `extras remove`

Remove an extra from configuration. Source files and synced targets are preserved.

```bash
skillshare extras remove rules
skillshare extras remove prompts --force -p
```

### `extras collect`

Reverse-sync: copy local files from a target back into the extras source directory, replacing originals with symlinks.

```bash
skillshare extras collect rules
skillshare extras collect rules --from ~/.claude/rules --dry-run
```

---

## Integration with Existing Commands

### `status`

Now shows an Extras section with file count and target count per configured extra.

### `doctor`

Checks that extras source directories exist and target parent directories are reachable. Reports warnings for missing sources.

### `diff --extras`

Per-file diff for extras targets. Shows which files need to be created, updated, or pruned.

```bash
skillshare diff --extras          # extras only
skillshare diff --all             # skills + extras
skillshare diff --extras --json   # JSON output
```

### `sync extras --json`

Structured JSON output for programmatic consumption:

```bash
skillshare sync extras --json
```

### `sync --all -p`

Project-mode `--all` now includes extras sync alongside skills.

---

## Web UI

### Extras Page

New page at `/extras` in the web dashboard:
- List all extras with source path, file count, and per-target sync status
- Sync individual extras or all at once
- Add new extras via modal dialog (name + targets with path/mode)
- Remove extras from config

### Dashboard Card

New stat card showing extras count, total files, and total targets with a link to the Extras page.

### REST API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/extras` | List extras with sync status |
| GET | `/api/extras/diff` | Per-file diff (optional `?name=`) |
| POST | `/api/extras` | Create new extra |
| POST | `/api/extras/sync` | Sync extras |
| DELETE | `/api/extras/{name}` | Remove extra |

---

## Configuration

### Global config (`~/.config/skillshare/config.yaml`)

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: prompts
    targets:
      - path: ~/.claude/prompts
```

### Project config (`.skillshare/config.yaml`)

```yaml
extras:
  - name: rules
    targets:
      - path: .claude/rules
```

### Sync modes

| Mode | Behavior |
|------|----------|
| `merge` (default) | Per-file symlinks from target to source |
| `copy` | Per-file copies |
| `symlink` | Entire directory symlink |

---

## Migration from v0.16.x

The extras directory layout changed from flat to nested. On first `sync extras` run, skillshare automatically migrates:

1. Detects `configDir/<name>/` matching a configured extra
2. Creates `configDir/extras/<name>/`
3. Moves all files to the new location
4. Removes the old directory

The migration is idempotent — running it multiple times is safe. A warning is printed during migration.

---

## Custom GitLab Domain Support

Also included from v0.16.15:

- **JihuLab auto-detection** — hosts containing `jihulab` (e.g., `jihulab.com`) are now auto-detected alongside `gitlab` for nested subgroup support
- **`gitlab_hosts` config** — declare self-managed GitLab hostnames so URLs are parsed with nested subgroup support:
  ```yaml
  gitlab_hosts:
    - git.company.com
    - code.internal.io
  ```
- **`SKILLSHARE_GITLAB_HOSTS` env var** — comma-separated list for CI/CD pipelines without a config file; merged with config values
- **GitLab nested subgroup URL fix** — URLs like `gitlab.com/group/subgroup/project` are now treated as the full repo path
- **HTTPS fallback fix** — platform-aware HTTPS URL parsing no longer misroutes GitHub Enterprise and Gitea URLs

---

## Upgrade

```bash
skillshare self-update
# or
brew upgrade skillshare
```

No manual steps required. Extras directories are auto-migrated on first use.
