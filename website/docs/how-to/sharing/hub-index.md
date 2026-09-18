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

## Audit Enrichment

Add security risk scores to your index so teammates can see skill safety at a glance:

```bash
# Build index with audit scores
skillshare hub index --audit

# Combine with full metadata
skillshare hub index --full --audit
```

When `--audit` is used, each skill is scanned with `skillshare audit` rules and the index includes `riskScore` (0–100), `riskLabel` (clean/low/medium/high/critical), and `auditedAt` timestamp. Skills that fail to scan are included without risk fields.

Search results from an audited index display risk badges:

```
  1. safe-skill               owner/repo/safe-skill         [clean]
  2. risky-skill              owner/repo/risky-skill        [high]
```

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

Teammates can search via the raw URL, over SSH, or by cloning locally:
```bash
# Via raw URL
skillshare search --hub https://raw.githubusercontent.com/team/skills/main/skillshare-hub.json

# Over SSH — clones the repo and reads the index (no manual clone needed)
skillshare search --hub git@github.com:team/skills.git
skillshare search --hub git@ghe.corp.com:team/skills.git//hubs/team.json

# Or clone and search locally
git pull
skillshare search --hub ./skillshare-hub.json
```

:::tip Private & GitHub Enterprise repos
SSH hub sources are cloned with your SSH agent/keys, so they work for private repos and GitHub Enterprise (GHE) hosts where raw HTTPS URLs redirect to a login page. The index path inside the repo comes from the `//path` suffix and defaults to `skillshare-hub.json` at the repo root. Both scp-style (`git@host:org/repo.git`) and scheme-style (`ssh://git@host/org/repo.git`) URLs work. Save it once with [`hub add`](../../reference/commands/hub.md#hub-add) to search by label instead.

When a GitHub/GHE hub is loaded over SSH, same-host domain-prefixed skill sources inherit the hub's SSH identity. For example, a hub URL of `acme@acme.ghe.com:Org/skills.git//hubs/team.json` lets an entry source of `acme.ghe.com/Org/skills/skills/reviewer` install over SSH. If the hub is loaded over HTTP, a local file, or a different host, domain-prefixed sources remain HTTPS sources.
:::

## Web Dashboard

### Create a Hub without writing JSON

Open **Skills → My Hubs → New Hub** in the dashboard (`skillshare ui`).

1. Give the draft a name and optional description. These identify the draft locally; they are not included in the exported index.
2. Choose **Choose installed skills**, select the skills to share, and add them. Or use **Add source manually**.
3. Edit each skill's display name, description, tags, and install source. For example, `runkids/demo-skills/skills/pdf` identifies a skill inside a remote repository. The **Advanced** section preserves an optional `skill` selector for repositories containing multiple skills.
4. Choose **Save draft**. The page checks every entry and displays any export blockers.
5. Choose **Download index** to obtain `skillshare-hub.json`.
6. Commit the downloaded file to your own Git repository or upload it to an HTTP server. Enter that location in the page to copy a `skillshare hub add` command for recipients.

Downloading does **not** publish anything. The catalog references skills; it does not bundle their files. Source validation checks syntax, not whether a repository exists or whether recipients have permission. Private repositories still require access.

:::tip Local skills can stay in drafts
An installed skill with no known remote origin remains visible with its local source. You can save it in a draft. Export is blocked until you provide a remote install source or remove that entry; the builder never silently leaves it out.
:::

### Resume or import a catalog

Drafts are stored on the machine running the dashboard in `hub-drafts/` next to the active configuration file. Global and project configurations have separate drafts. Use **Save draft** before reloading. Leaving with unsaved changes prompts you to discard them; saves from an outdated window are rejected so they cannot overwrite a newer revision. **Reload saved draft** retrieves the latest version.

Use **Import JSON** for an existing v1 `skillshare-hub.json` (up to 4 MB). Unsupported versions and invalid field types produce errors. Entries with the same display name remain separate. Extra JSON fields and `skill` selectors are preserved. If an older index includes `sourcePath`, relative sources are resolved as local paths, matching the existing index reader; they must be changed to remote sources before export.

The portable export removes the author's `sourcePath` and known local metadata (`relPath`, `flatName`, `installedAt`, `isInRepo`). It contains the index, not the draft's name, description, IDs, or revisions. Changing an entry's source or skill selector clears its previous audit score, label, and timestamp. URL credentials, query strings, and fragments are rejected; configure repository authentication separately.

**Delete draft** asks for confirmation and deletes only that draft. It does not uninstall skills, delete a hosted index, or remove a subscribed Hub.

### Search a shared Hub

1. Open **Skills → Install**.
2. Choose a Hub from the search source selector. Use the Hub manager in the install dialog to add a URL, SSH repository, or local index path.
3. Search, preview, and install skills.

Subscribed Hub sources are saved in the active skillshare configuration and shared with the CLI. They are separate from the drafts in **My Hubs**.

The existing `skillshare hub index` command and `/api/hub/index` endpoint continue to generate indexes as before, including support for local sources. The portable-export rules above apply to the dashboard builder.

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
- For SSH-only GitHub Enterprise installs, prefer explicit SSH sources (`user@host:owner/repo.git//path`) or load the hub itself over SSH so same-host GitHub/GHE domain-prefixed entries inherit that SSH identity

## Organization Deployment

A typical end-to-end workflow for rolling out a hub across your organization:

```bash
# 1. A skill admin curates skills from internal repos
skillshare install ghe.internal.company.com/platform/ai-skills/coding-standards
skillshare install ghe.internal.company.com/platform/ai-skills/review-checklist
skillshare install ghe.internal.company.com/security/ai-skills/threat-model

# 2. Generate the hub index (with optional audit scores)
skillshare hub index --audit -o ./skillshare-hub.json

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

- [search](/docs/reference/commands/search) — Search skills from hubs
- [hub](/docs/reference/commands/hub) — Manage hub sources
- [install](/docs/reference/commands/install) — Install discovered skills
