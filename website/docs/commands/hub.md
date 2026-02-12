---
sidebar_position: 8
---

# hub

Manage private skill hubs. Currently provides the `index` subcommand for building searchable skill catalogs.

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
      "source": "owner/repo/.claude/skills/my-skill"
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
4. Teammates search         → skillshare search --hub <path-or-url>
```

For more details, see the [Hub Index Guide](../guides/hub-index.md).
