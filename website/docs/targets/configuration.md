---
sidebar_position: 4
---

# Configuration

Configuration file reference for skillshare.

## Overview

```text
~/.config/skillshare/
├── config.yaml          ← Configuration file
├── skills/              ← Source directory (your skills)
│   ├── my-skill/
│   ├── another/
│   └── _team-repo/      ← Tracked repository
└── backups/             ← Automatic backups
    └── 2026-01-20.../
```

---

## IDE Support (JSON Schema) {#ide-support}

Config files include a YAML Language Server directive that enables **autocompletion**, **validation**, and **hover documentation** in supported editors.

New configs created by `skillshare init` include this automatically:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

### Adding to an existing config

If your config was created before this feature, add the comment as the **first line**:

**Global config** (`~/.config/skillshare/config.yaml`):
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
```

**Project config** (`.skillshare/config.yaml`):
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json
```

Or simply re-run `skillshare init --force` (global) or `skillshare init -p --force` (project) to regenerate the config with the schema comment.

### Supported editors

| Editor | Extension required |
|--------|-------------------|
| VS Code | [YAML](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) by Red Hat |
| JetBrains IDEs | Built-in YAML support |
| Neovim | [yaml-language-server](https://github.com/redhat-developer/yaml-language-server) via LSP |

---

## Config File

**Location:** `~/.config/skillshare/config.yaml`

### Full Example

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
# Source directory (where you edit skills)
source: ~/.config/skillshare/skills

# Default sync mode for new targets
mode: merge

# Targets (AI CLI skill directories)
targets:
  claude:
    path: ~/.claude/skills
    # mode: merge (inherits from default)

  codex:
    path: ~/.codex/skills
    mode: symlink  # Override default mode
    include: [codex-*] # merge/copy mode only

  cursor:
    path: ~/.cursor/skills
    mode: copy  # real files for Cursor
    exclude: [experimental-*] # merge/copy mode only

  # Custom target
  myapp:
    path: ~/apps/myapp/skills

# Remote skills — auto-managed by install/uninstall
skills:
  - name: pdf
    source: anthropics/skills/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true

# Files to ignore during sync
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

---

## Fields

### `source`

Path to your skills directory (single source of truth).

```yaml
source: ~/.config/skillshare/skills
```

**Default:** `~/.config/skillshare/skills`

### `mode`

Default sync mode for all targets.

```yaml
mode: merge
```

| Value | Behavior |
|-------|----------|
| `merge` | Each skill symlinked individually. Local skills preserved. **(default)** |
| `copy` | Each skill copied as real files. For AI CLIs that can't follow symlinks. |
| `symlink` | Entire target directory is one symlink. |

### `targets`

AI CLI skill directories to sync to.

```yaml
targets:
  <name>:
    path: <path>
    mode: <mode>  # optional, overrides default
    include: [<glob>, ...]  # optional, merge/copy mode only
    exclude: [<glob>, ...]  # optional, merge/copy mode only
```

**Example:**
```yaml
targets:
  claude:
    path: ~/.claude/skills

  codex:
    path: ~/.codex/skills
    mode: symlink

  custom:
    path: ~/my-app/skills
```

### `include` / `exclude` (target filters)

Use per-target filters to control which skills are synced in **merge and copy modes**.

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*]
  claude:
    path: ~/.claude/skills
    exclude: [codex-*]
```

Rules:
- Matching is against target flat names (for example `team__frontend__ui`)
- `include` is applied first
- `exclude` is applied after include
- Pattern syntax uses Go `filepath.Match` (`*`, `?`, `[...]`)
- In `symlink` mode, include/exclude is ignored
- If a previously synced source link becomes excluded, `sync` removes that target entry
- Local non-symlink folders that already existed in target are preserved

#### Pattern cheat sheet

| Pattern | Matches | Typical use |
|---------|---------|-------------|
| `codex-*` | `codex-agent`, `codex-rag` | Prefix-based grouping |
| `team__*` | `team__frontend__ui` | Repo/group namespace |
| `*-experimental` | `rag-experimental` | Suffix-based cleanup |
| `core-?` | `core-a`, `core-1` | Single-character variant |
| `[ab]-tool` | `a-tool`, `b-tool` | Small explicit set |

#### Scenario A: include only

Use `include` when a target should receive only a focused subset.

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*, shared-*]
```

Use case:
- Keep Codex focused on coding workflows only
- Avoid sending writing/research-only skills to this target

#### Scenario B: exclude only

Use `exclude` when a target should receive almost everything except a known subset.

```yaml
targets:
  claude:
    path: ~/.claude/skills
    exclude: [*-experimental, codex-*]
```

Use case:
- Keep one main target broad
- Hide unstable or target-specific skills

#### Scenario C: include + exclude

Use both when you want a broad include, then carve out exceptions.

```yaml
targets:
  cursor:
    path: ~/.cursor/skills
    include: [core-*, team__*]
    exclude: [*-deprecated, team__legacy__*]
```

Evaluation order:
1. Keep only names matching `include`
2. Remove matches from `exclude`

Given source skills:
- `core-auth`
- `core-deprecated`
- `team__frontend__ui`
- `team__legacy__docs`
- `misc-tool`

Result for `cursor`:
- Synced: `core-auth`, `team__frontend__ui`
- Not synced: `core-deprecated`, `team__legacy__docs`, `misc-tool`

#### Managing filters via CLI

Instead of editing YAML manually, use the `target` command:

```bash
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare sync  # Apply changes
```

Duplicate patterns are silently ignored. Invalid glob patterns return an error.

See [target command](/docs/commands/target#target-filters-includeexclude) for full reference.

#### Skill-level targets {#skill-level-targets}

Skills can declare which targets they're compatible with using the `targets` field in SKILL.md:

```yaml
---
name: claude-prompts
targets: [claude]
---
```

This is a **second layer** of filtering that works alongside config-level include/exclude:

```
Source Skills
  │
  ├─ Config include/exclude    ← per-target, set by consumer
  │
  └─ Skill targets field       ← per-skill, set by author
      │
      ▼
  Skills synced to target
```

**Evaluation order:**
1. `include` — keep only matching names
2. `exclude` — remove matching names
3. `targets` field — remove skills whose targets list doesn't include this target

Both layers must pass (AND relationship). Config filters always take priority — even if a skill declares `targets: [claude]`, a config `exclude: [claude-*]` will still exclude it.

**Cross-mode matching:** `targets: [claude]` matches both the global target `claude` and the project target `claude`, because they refer to the same AI CLI. See [supported targets](/docs/targets/supported-targets).

:::tip
Use config filters (`include`/`exclude`) when the **consumer** wants to control what goes where. Use skill-level `targets` when the **author** knows the skill only works with specific AI CLIs.
:::

#### Existing target entries when filters change

When you add or change filters, then run `skillshare sync`:

| Existing item in target | What happens |
|-------------------------|--------------|
| Source-linked symlink/junction that is now filtered out | Removed (unlinked) |
| Managed copy (copy mode) that is now filtered out | Removed |
| Local non-symlink directory created in target | Preserved |
| Unrelated local content | Preserved |

### `skills`

Tracks remotely-installed skills. Auto-managed by `skillshare install` and `skillshare uninstall`.

```yaml
skills:
  - name: pdf
    source: anthropics/skills/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill directory name |
| `source` | Yes | GitHub URL or local path |
| `tracked` | No | `true` if installed with `--track` (default: `false`) |

When you run `skillshare install` with no arguments, all listed skills that aren't already present are installed. This makes `config.yaml` a portable skill manifest — copy it to another machine and run `skillshare install && skillshare sync`.

The `skills:` list is automatically updated after each `install` and `uninstall` operation. You don't need to edit it manually.

### `ignore`

Glob patterns for files to skip during sync.

```yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

**Default patterns:**
- `**/.DS_Store`
- `**/.git/**`

### `audit`

Security audit configuration.

```yaml
audit:
  block_threshold: CRITICAL
```

| Field | Values | Default | Description |
|-------|--------|---------|-------------|
| `block_threshold` | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFO` | `CRITICAL` | Minimum severity to block `skillshare install` |

- `block_threshold` only controls when install is **blocked** — scanning always runs
- Use `--skip-audit` to bypass scanning for a single install
- Use `--force` to override a block (findings are still shown)

---

## Project Config

**Location:** `.skillshare/config.yaml` (in project root)

Project config uses a different format from global config.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json
# Targets — string or object form
targets:
  - claude                    # String: known target with defaults
  - cursor
  - name: custom-ide               # Object: custom path and mode
    path: ./tools/ide/skills
    mode: symlink
  - name: codex                    # Object with filters
    include: [codex-*]
    exclude: [codex-experimental-*]

# Remote skills — auto-managed by install/uninstall
skills:
  - name: pdf
    source: anthropic/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true                  # Cloned with git history

# Audit — same as global
audit:
  block_threshold: HIGH
```

### `targets` (project)

Supports two YAML forms:

| Form | Example | When to use |
|------|---------|-------------|
| **String** | `- claude` | Known target, default path and merge mode |
| **Object** | `- name: x, path: ..., mode: ..., include: [...], exclude: [...]` | Custom path, mode override, or per-target filters |

### `skills` (project)

Same schema as the [global `skills` field](#skills). Auto-managed by `skillshare install -p` and `skillshare uninstall -p`.

:::tip Portable Manifest
`config.yaml` is a portable skill manifest — in both global and project mode. Run `skillshare install && skillshare sync` on a new machine (or `skillshare install -p` in a project) to reproduce the same setup.
:::

---

## Managing Config

### View current config

```bash
skillshare status
# Shows source, targets, modes
```

### Edit config directly

```bash
# Open in editor
$EDITOR ~/.config/skillshare/config.yaml

# Then sync to apply changes
skillshare sync
```

### Reset config

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Custom Audit Rules

**Location:**

| Mode | Path |
|------|------|
| Global | `~/.config/skillshare/audit-rules.yaml` |
| Project | `.skillshare/audit-rules.yaml` |

Rules are merged in order: **built-in → global → project**. You can add new rules, disable built-in rules, or override severity.

```yaml
rules:
  # Add a custom rule
  - id: flag-todo
    severity: MEDIUM
    pattern: todo-comment
    message: "TODO comment found"
    regex: '(?i)\bTODO\b'

  # Disable a built-in rule
  - id: system-writes-0
    enabled: false
```

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique rule identifier |
| `severity` | Yes | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFO` |
| `pattern` | Yes | Pattern category name |
| `message` | Yes | Human-readable finding description |
| `regex` | Yes | Regular expression to match |
| `exclude` | No | Suppress match when line also matches this regex |
| `enabled` | No | Set `false` to disable a built-in rule |

To scaffold a starter file:

```bash
skillshare audit --init-rules       # Global
skillshare audit --init-rules -p    # Project
```

See [audit command](/docs/commands/audit) for full details.

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SKILLSHARE_CONFIG` | Override config file path |
| `GITHUB_TOKEN` | For API rate limit issues |

**Example:**
```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

---

## Skill Metadata

When you install a skill, skillshare creates a `.skillshare-meta.json` file:

```json
{
  "source": "anthropics/skills/skills/pdf",
  "type": "github",
  "installed_at": "2026-01-20T15:30:00Z",
  "repo_url": "https://github.com/anthropics/skills.git",
  "subdir": "skills/pdf",
  "version": "abc1234"
}
```

| Field | Description |
|-------|-------------|
| `source` | Original install source input |
| `type` | Source type (`github`, `local`, etc.) |
| `installed_at` | Installation timestamp |
| `repo_url` | Git clone URL (git sources only) |
| `subdir` | Subdirectory path (monorepo sources only) |
| `version` | Git commit hash at install time |

This is used by `skillshare update` and `skillshare check` to know where to fetch updates from.

**Don't edit this file manually.**

---

## Platform Differences

### macOS / Linux

```yaml
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

Uses symlinks.

### Windows

```yaml
source: %AppData%\skillshare\skills
targets:
  claude:
    path: %USERPROFILE%\.claude\skills
```

Uses NTFS junctions (no admin required).

---

## Related

- [Source & Targets](/docs/concepts/source-and-targets) — Core concepts
- [Sync Modes](/docs/concepts/sync-modes) — Merge vs copy vs symlink
- [Environment Variables](/docs/reference/environment-variables) — All variables
