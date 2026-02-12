---
sidebar_position: 4
---

# Hub Index Guide

Build and share private skill catalogs without depending on the GitHub API.

## Why Use a Hub Index?

| Scenario | GitHub Search | Hub Index |
|----------|--------------|-----------|
| Public skills on GitHub | Yes | No |
| Private/internal skills | No | Yes |
| Air-gapped environments | No | Yes |
| Custom curated catalogs | No | Yes |
| No GitHub token needed | No | Yes |

A hub index is a JSON file (`skillshare-hub.json`) that lists skills with their name, description, and source. Anyone with access to this file can search and install from it.

## Quick Start

### 1. Build an Index

```bash
# From your global skills
skillshare hub index

# From a project
skillshare hub index -p

# Output: <source>/skillshare-hub.json
```

### 2. Search the Index

```bash
# Local file
skillshare search react --hub ./skillshare-hub.json

# Remote URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# Browse all skills (no query)
skillshare search --hub ./skillshare-hub.json --json
```

### 3. Install from Results

The interactive search flow works the same as GitHub search — select a skill and it gets installed.

## Sharing Strategies

### File Share (Simplest)

Copy the index file to a shared location:

```bash
skillshare hub index -o /shared/team/skillshare-hub.json
```

Teammates search with:
```bash
skillshare search --hub /shared/team/skillshare-hub.json
```

### HTTP Server

Host the index on any web server:

```bash
# Generate and upload
skillshare hub index -o ./skillshare-hub.json
# Upload to your preferred hosting
```

Teammates search with:
```bash
skillshare search --hub https://skills.company.com/skillshare-hub.json
```

### Git Repository

Commit the index to a shared repo:

```bash
skillshare hub index -o ./skillshare-hub.json
git add skillshare-hub.json && git commit -m "Update skill index"
git push
```

## Web Dashboard

The web dashboard (`skillshare ui`) supports private index search:

1. Open the **Search** page
2. Click **Private Index** tab
3. **Current Skills** is built-in — searches your local installed skills
4. Click **Add Index** to add custom index URLs
5. Search and install directly from the UI

Saved indexes persist in browser localStorage.

## Index Schema

The index follows Schema v1:

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "Does something useful",
      "source": "owner/repo/.claude/skills/my-skill"
    }
  ]
}
```

### Essential Fields (Consumer Contract)

Only three fields are required for search and install to work:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill display name |
| `source` | Yes | Install source (GitHub shorthand, URL, or local path) |
| `description` | Recommended | Short description for search matching |

### Document-Level Fields

| Field | Description |
|-------|-------------|
| `schemaVersion` | Always `1` |
| `generatedAt` | RFC 3339 timestamp |
| `sourcePath` | Base path for resolving relative sources |

### Source Path Resolution

When `sourcePath` is set and a skill's `source` is a relative path, the search consumer joins them:

```
sourcePath: /home/user/.config/skillshare/skills
source:     _team/frontend-skill
→ resolved: /home/user/.config/skillshare/skills/_team/frontend-skill
```

This prevents relative paths from being misinterpreted as GitHub shorthand (`owner/repo`).

Absolute paths, URLs, and domain-prefixed paths are never joined:

| Source Pattern | Joined? |
|----------------|---------|
| `_team/my-skill` | Yes |
| `subdir/skill` | Yes |
| `/absolute/path` | No |
| `github.com/owner/repo/skill` | No |
| `https://...` | No |

## Hand-Written Indexes

You can create an index manually without using `hub index`:

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "company-style",
      "description": "Company coding standards",
      "source": "github.com/company/skills/company-style"
    },
    {
      "name": "deploy-helper",
      "description": "Deployment automation",
      "source": "gitlab.com/ops/skills/deploy-helper"
    }
  ]
}
```

Tips for hand-written indexes:
- `sourcePath` is optional — omit if all sources are absolute
- Skills with empty `name` are skipped
- Results are sorted by name alphabetically

## Tips

- **Automate index generation** — Add `skillshare hub index` to your CI pipeline after skill changes
- **Use `--full` for auditing** — Full mode includes version, install date, and type information
- **Combine with project mode** — `skillshare hub index -p` indexes only project-level skills
