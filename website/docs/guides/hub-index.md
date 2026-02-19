---
sidebar_position: 4
---

# Hub Index Guide

Build a centralized skill catalog for your organization — no GitHub API or token required.

## Why Use a Hub Index?

A hub index is a JSON file (`skillshare-hub.json`) that lists skills with their name, description, and source. Host it internally and every team member can search and install skills from it.

| Use Case | GitHub Search | Hub Index |
|----------|--------------|-----------|
| Organization-wide skill catalog | No | **Yes** |
| Private/internal skills | No | **Yes** |
| Air-gapped / VPN-only environments | No | **Yes** |
| Curated, approved skill sets | No | **Yes** |
| No GitHub token needed | No | **Yes** |

For a real-world example, see the [Public Hub](#public-hub) section.

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

You can create an index manually without using `hub index`. This is especially useful for internal skills hosted on private infrastructure — sources that GitHub Search and public tools can never reach:

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "company-style",
      "description": "Company coding standards and review checklist",
      "source": "ghe.internal.company.com/platform/ai-skills/company-style",
      "tags": ["quality", "workflow"]
    },
    {
      "name": "deploy-helper",
      "description": "Internal deployment automation",
      "source": "gitlab.internal.company.com/ops/skills/deploy-helper",
      "tags": ["devops"]
    },
    {
      "name": "onboarding",
      "description": "New hire onboarding skill for AI assistants",
      "source": "ghe.internal.company.com/hr/ai-skills/onboarding",
      "tags": ["workflow"]
    }
  ]
}
```

:::tip Why not just use GitHub Search?
`skillshare search` only finds public repos on github.com. A hub index can point to **any** source — GitHub Enterprise, private GitLab, internal servers — things that only your employees behind VPN can access. This is what makes hub the go-to solution for organization-wide skill distribution.
:::

Tips for hand-written indexes:
- `sourcePath` is optional — omit if all sources are absolute
- `tags` is optional — useful for filtering on the website or in search
- Skills with empty `name` are skipped
- Results are sorted by name alphabetically

## Organization Deployment

A typical end-to-end workflow for rolling out a hub across your organization:

```bash
# 1. A skill admin curates skills from internal repos
skillshare install ghe.internal.company.com/platform/ai-skills/coding-standards
skillshare install ghe.internal.company.com/platform/ai-skills/review-checklist
skillshare install ghe.internal.company.com/security/ai-skills/threat-model

# 2. Generate the hub index
skillshare hub index -o ./skillshare-hub.json

# 3. Host it (pick one)
#    - Internal Git repo: commit and push
#    - S3/CDN: aws s3 cp ./skillshare-hub.json s3://skills-bucket/
#    - Intranet server: scp to your hosting

# 4. Team members add the hub once
skillshare hub add https://skills.internal.company.com/skillshare-hub.json --label company

# 5. Search and install — only accessible behind VPN
skillshare search coding --hub company
```

To keep the index fresh, add `skillshare hub index` to a CI pipeline that runs after skill changes.

## Public Hub

The [skillshare-hub](https://github.com/runkids/skillshare-hub) is a curated catalog of quality skills. It is the **default hub** — when you run `search --hub` without specifying a source, it searches here:

```bash
skillshare search --hub              # Browse all skills in the public hub
skillshare search react --hub        # Search for "react" skills
```

It also serves as a reference for building your own organization's hub:

- **Index structure** — How to organize `skillshare-hub.json` with names, descriptions, sources, and tags
- **CI validation** — Automated JSON format checks and `skillshare audit` security scans on every PR
- **Contribution workflow** — Fork → add entry → PR, with CI gates

Want to build an internal hub for your team? Fork the repo, replace the skills with your organization's catalog, and customize the CI pipeline to match your security policies.

## Tips

- **Automate index generation** — Add `skillshare hub index` to your CI pipeline after skill changes
- **Use `--full` for auditing** — Full mode includes version, install date, and type information
- **Combine with project mode** — `skillshare hub index -p` indexes only project-level skills

---

## See Also

- [search](/docs/commands/search) — Search skills from hubs
- [hub](/docs/commands/hub) — Manage hub sources
- [install](/docs/commands/install) — Install discovered skills
