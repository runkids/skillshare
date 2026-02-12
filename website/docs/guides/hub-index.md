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

Generate the index locally, then upload it to your hosting:

```bash
# Step 1: Generate
skillshare hub index -o ./skillshare-hub.json

# Step 2: Upload (use your preferred method)
scp ./skillshare-hub.json server:/var/www/skills/
# or: aws s3 cp ./skillshare-hub.json s3://my-bucket/
# or: rsync, FTP, etc.
```

Teammates search with:
```bash
skillshare search --hub https://skills.company.com/skillshare-hub.json
```

### Git Repository

Commit the index to a shared repo so teammates can pull it:

```bash
skillshare hub index -o ./skillshare-hub.json
git add skillshare-hub.json && git commit -m "Update skill index"
git push
```

Teammates can search via the raw URL or clone locally:
```bash
# Via raw URL
skillshare search --hub https://raw.githubusercontent.com/team/skills/main/skillshare-hub.json

# Or clone and search locally
git pull
skillshare search --hub ./skillshare-hub.json
```

## Web Dashboard

The web dashboard (`skillshare ui`) supports hub search:

1. Open the **Search** page
2. Click **Hub** tab
3. Click **Manage** to add hub sources (URL or local path)
4. Select a hub from the dropdown and search
5. Install directly from the UI

Saved hubs persist in browser localStorage.

### Hub Search

<p align="center">
  <img src="/img/web-hub-search-demo.png" alt="Hub search page" width="720" />
</p>

Select a hub source from the dropdown and search for skills.

### Switch Between Hubs

<p align="center">
  <img src="/img/web-hub-dropdown-demo.png" alt="Hub dropdown selector" width="720" />
</p>

Use the dropdown to switch between multiple hub sources.

### Manage Hubs

<p align="center">
  <img src="/img/web-hub-manage-demo.png" alt="Manage hubs modal" width="720" />
</p>

Click **Manage** to add, view, or remove hub sources. Enter a URL or local file path to a `skillshare-hub.json` file.

### Delete Confirmation

<p align="center">
  <img src="/img/web-hub-delete-confirm-demo.png" alt="Hub delete confirmation" width="720" />
</p>

Removing a hub requires confirmation to prevent accidental deletion.

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
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow", "productivity"]
    }
  ]
}
```

### Essential Fields (Consumer Contract)

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill display name |
| `source` | Yes | Install source (GitHub shorthand, URL, or local path) |
| `description` | Recommended | Short description for search matching |
| `skill` | No | Specific skill name within a multi-skill repo (used with `install -s`) |
| `tags` | No | Classification tags for filtering and grouping |

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
      "source": "github.com/company/skills/company-style",
      "tags": ["quality", "workflow"]
    },
    {
      "name": "deploy-helper",
      "description": "Deployment automation",
      "source": "gitlab.com/ops/skills/deploy-helper",
      "tags": ["devops"]
    }
  ]
}
```

Tips for hand-written indexes:
- `sourcePath` is optional — omit if all sources are absolute
- `tags` is optional — useful for filtering on the website or in search
- Skills with empty `name` are skipped
- Results are sorted by name alphabetically

## Community Hub

The [skillshare-hub](https://github.com/runkids/skillshare-hub) is a community-maintained index of curated skills. You can search it directly:

```bash
skillshare search --hub https://raw.githubusercontent.com/runkids/skillshare-hub/main/skillshare-hub.json
```

Want to share your skills with the community? [Open a PR](https://github.com/runkids/skillshare-hub) to add your skill to the catalog. CI automatically runs `skillshare audit` on every submission to check for security issues before merging.

## Tips

- **Automate index generation** — Add `skillshare hub index` to your CI pipeline after skill changes
- **Use `--full` for auditing** — Full mode includes version, install date, and type information
- **Combine with project mode** — `skillshare hub index -p` indexes only project-level skills
