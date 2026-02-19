---
sidebar_position: 8
---

# hub

Manage skill hubs — saved hub sources for organization-wide skill discovery.

## When to Use

- Set up your organization's skill catalog for `search` to query
- Switch between multiple hubs (e.g., company-wide vs team-specific)
- List or remove saved hub sources

## hub add

Save a hub source to config for reuse in [`search --hub`](./search.md).

```bash
skillshare hub add <url> [options]
```

| Flag | Description |
|------|-------------|
| `--label`, `-l` | Custom label (default: derived from URL hostname) |
| `--project`, `-p` | Save to project config |
| `--global`, `-g` | Save to global config |

The first hub added is automatically set as the default. Labels are case-insensitive.

```bash
skillshare hub add https://internal.corp/hub.json --label team
skillshare hub add ./local-hub.json                # label derived: "local-hub"
```

## hub list

List saved hubs. `*` marks the default hub.

```bash
skillshare hub list [options]
```

| Flag | Description |
|------|-------------|
| `--project`, `-p` | Show project hubs |
| `--global`, `-g` | Show global hubs |

```
$ skillshare hub list
  * team   https://internal.corp/hub.json
    local  ./local-hub.json
```

Alias: `hub ls`

## hub remove

Remove a saved hub by label.

```bash
skillshare hub remove <label> [options]
```

| Flag | Description |
|------|-------------|
| `--project`, `-p` | Remove from project config |
| `--global`, `-g` | Remove from global config |

If the removed hub was the default, the default is cleared.

Alias: `hub rm`

## hub default

Show or set the default hub used by `search --hub` (bare flag).

```bash
skillshare hub default [label] [options]
```

| Flag | Description |
|------|-------------|
| `--reset` | Clear the default (revert to community hub) |
| `--project`, `-p` | Use project config |
| `--global`, `-g` | Use global config |

```bash
skillshare hub default              # Show current default
skillshare hub default team         # Set default to "team"
skillshare hub default --reset      # Clear default → community hub
```

## hub index

Build a `skillshare-hub.json` index file from installed skills. The generated index can be consumed by [`search --hub`](./search.md#private-index-search) for private, offline skill discovery.

### Usage

```bash
skillshare hub index [options]
```

### Options

| Flag | Description |
|------|-------------|
| `--source`, `-s` | Source directory to scan (default: auto-detect) |
| `--output`, `-o` | Output file path (default: `<source>/skillshare-hub.json`) |
| `--full` | Include full metadata (flatName, type, version, etc.) |
| `--project`, `-p` | Use project mode (`.skillshare/`) |
| `--global`, `-g` | Use global mode (`~/.config/skillshare`) |
| `--help`, `-h` | Show help |

### Output Modes

**Minimal (default)** — Only essential fields for search and install:

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "A useful skill",
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow"]
    }
  ]
}
```

**Full (`--full`)** — Includes metadata for auditing and management:

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "github.com/owner/repo/.claude/skills/my-skill",
  "tags": ["workflow"],
  "flatName": "my-skill",
  "type": "github-subdir",
  "repoUrl": "https://github.com/owner/repo.git",
  "version": "abc1234",
  "installedAt": "2026-02-10T03:49:06Z",
  "isInRepo": false
}
```

Metadata fields use `omitempty` — redundant values are suppressed:
- `flatName` omitted when equal to `name`
- `relPath` omitted when equal to `source`
- `isInRepo` omitted when `false`

### Examples

```bash
# Build minimal index (default)
skillshare hub index

# Build with full metadata
skillshare hub index --full

# Custom output path
skillshare hub index -o /shared/team/skillshare-hub.json

# Custom source directory
skillshare hub index -s ~/my-skills

# Project mode
skillshare hub index -p
```

### Workflow

A typical private hub workflow:

```
1. Install skills           → skillshare install ...
2. Build index              → skillshare hub index
3. Share the index file     → Copy/host skillshare-hub.json
4. Teammates search         → skillshare search --hub [path-or-url]
```

For more details, see the [Hub Index Guide](../guides/hub-index.md).

## Config Format

Saved hubs are stored under the `hub:` key in `config.yaml`:

```yaml
hub:
  default: team
  hubs:
    - label: team
      url: https://internal.corp/hub.json
    - label: local
      url: ./local-hub.json
```

The [public hub](https://github.com/runkids/skillshare-hub) is the built-in default and doesn't need to be saved. When no custom default is set, `search --hub` falls back to it automatically. Fork this repo to bootstrap your own organization's hub.
