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
SSH hub sources are cloned with your SSH agent/keys, so they work for private repos and GitHub Enterprise (GHE) hosts where raw HTTPS URLs redirect to a login page. The index path inside the repo comes from the `//path` suffix and defaults to `skillshare-hub.json` at the repo root. Both scp-style (`git@host:org/repo.git`) and scheme-style (`ssh://git@host/org/repo.git`) URLs work. Save it once with [`hub add`](/docs/reference/commands/hub#hub-add) to search by label instead.

When a GitHub/GHE hub is loaded over SSH, same-host domain-prefixed skill sources inherit the hub's SSH identity. For example, a hub URL of `acme@acme.ghe.com:Org/skills.git//hubs/team.json` lets an entry source of `acme.ghe.com/Org/skills/skills/reviewer` install over SSH. If the hub is loaded over HTTP, a local file, or a different host, domain-prefixed sources remain HTTPS sources.
:::

## Web Dashboard

### Create a Hub without writing JSON

Open **Skills → Hubs** in the dashboard (`skillshare ui`), then choose **Add or create a Hub → Create a new Hub**. The new Hub opens for editing.

1. Give the Hub a **Name** and optional **Description**. The name becomes the `--label` of the `skillshare hub add` command you share; neither is included in the exported index.
2. Choose **Add skill**. On **Paste URL**, enter a **Git URL**, choose **Find**, pick a **Version**, and select the skills to add. On **Installed**, select skills installed on this machine. Or choose **Can’t find it? Enter the source yourself** to add an empty row.
3. Edit each skill's **Name**, **Source**, and **Version**. For example, `runkids/demo-skills/skills/pdf` identifies a skill inside a remote repository. Expand a row for **Skill description**, **Tags (comma-separated)**, and **Skill selector (optional)**, which selects a skill in a repository containing multiple skills.
4. Choose **Save**. The page checks every entry. If a skill cannot be installed by others, the editor stays open and marks that row.
5. Choose **Share → Download skillshare-hub.json**. The download is disabled until the marked skills are fixed with **Edit**.
6. Commit the downloaded file to your own Git repository or upload it to an HTTP server. Paste that URL in the **Share** dialog to copy a `skillshare hub add` command for recipients. The URL is saved with the Hub.

Downloading does **not** publish anything. The catalog references skills; it does not bundle their files. Source validation checks syntax, not whether a repository exists or whether recipients have permission. Private repositories still require access.

:::tip Local skills can stay in your Hub
An installed skill with no known remote origin keeps its local source. You can save it in your Hub. The download is blocked until you provide a remote install source or remove that entry; the builder never silently leaves it out.
:::

### Resume or import a catalog

Your own Hubs are marked **Mine** in the Hub list. They are stored on the machine running the dashboard in `hub-drafts/` next to the active configuration file. Global and project configurations have separate Hubs. Choose **Save** before reloading. While you edit, the Hub list is locked; leaving with unsaved changes prompts you to discard them. Saves from an outdated window are rejected so they cannot overwrite a newer revision. **Cancel** reloads the latest saved version.

Use **Add or create a Hub → Import skillshare-hub.json** for an existing v1 `skillshare-hub.json` (up to 4 MB). Unsupported versions and invalid field types produce errors. Entries with the same display name remain separate. Extra JSON fields and `skill` selectors are preserved. If an older index includes `sourcePath`, relative sources are resolved as local paths, matching the existing index reader; they must be changed to remote sources before export.

The portable export removes the author's `sourcePath` and known local metadata (`relPath`, `flatName`, `installedAt`, `isInRepo`). It contains the index, not the Hub's name, description, IDs, revisions, or hosting URL. Changing an entry's source or skill selector clears its previous audit score, label, and timestamp. URL credentials, query strings, and fragments are rejected; configure repository authentication separately.

**More actions → Delete Hub** asks for confirmation and deletes only that Hub. It does not uninstall skills, delete a hosted index, or remove a subscribed Hub.

### Search a shared Hub

1. Open **Skills → Install** and choose **Search**.
2. Choose a Hub in the **In** selector. To add a URL, SSH repository, or local index path, choose **Manage hubs** next to the selector, then **Add or create a Hub → Add an existing Hub** on the Hubs page.
3. Search, preview, and install skills.

You can also pick a Hub on the Hubs page to filter its skills and install them. Subscribed Hub sources are saved in the active skillshare configuration and shared with the CLI. They are separate from your own Hubs.

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

### Pinning a Source to a Tag or Commit

To pin an entry to a specific version, use a web URL with the ref in the path. The branch, tag or commit SHA after `tree/` or `blob/` (GitHub), `-/tree/` or `-/blob/` (GitLab), or `src/` (Bitbucket) is used as the install ref, the same as `install --branch`:

```json
{
  "name": "reviewer",
  "source": "github.com/owner/repo/tree/v1.2.0/skills/reviewer"
}
```

Everyone who installs from the hub gets that revision, and `skillshare update` keeps it. Move the pin by editing the ref in the index. A ref that the remote does not have fails the install instead of falling back to the default branch.

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

A private hub gives teammates a searchable catalog of reviewed skills. Keep the catalog and skill sources on infrastructure your organization controls; the Git host or HTTP server supplies authentication and access control.

### 1. Curate skills and their sources

Keep skill changes and catalog changes in reviewed PRs. For an SSH-only Git host, use explicit SSH sources in the [index entries](#hand-written-indexes), for example:

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "code-review",
      "description": "Team code-review checklist",
      "source": "git@ghe.example.com:platform/ai-skills.git//skills/code-review"
    }
  ]
}
```

You can also generate the catalog from installed remote skills with `skillshare hub index --audit`. Before publishing, check that each source is reachable by teammates. An index built from local files can contain machine-local paths; replace those with shared sources. Audit badges describe a scan at a point in time, not permanent approval.

### 2. Audit changes with a reviewed CLI version

In the skill repository, gate PRs with a pinned CLI version and a severity threshold:

```yaml
name: Validate shared skills
on:
  pull_request:
    paths: ['skills/**', 'skillshare-hub.json']

jobs:
  audit:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      # Tags shown for readability; pin each action to a reviewed commit SHA
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          version: '0.23.5' # Example: choose a CLI version your team has reviewed
          source: ./skills
          audit: true
          audit-threshold: high
```

This example assumes the skills are under `skills/` in the checked-out repository. For an internal Git server, use your CI runner's checkout and access configuration; the scan commands are the same. See [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) for other CI systems.

The action's `version` input fixes the CLI release, not the Action itself or the skill contents. The example uses tags for readability; pin each action to a reviewed full commit SHA under your organization's policy. Use the [project lockfile](/docs/understand/project-skills#lockfile) to record remote skill commits. Review each kind of update separately. `hub index --audit` adds scan results to a catalog; use `skillshare audit --threshold high` or the pipeline gate above to reject findings at that severity.

### 3. Publish the catalog privately

Commit `skillshare-hub.json` to the root of the internal skill repository after review. Grant teammates read access through the Git host. Internal HTTP hosting is another option if the index can be fetched from your team's environment. You do not need to fork the public hub or expose a public raw URL.

### 4. Register, search and sync

After [initializing skillshare](/docs/getting-started/first-sync), teammates register the private catalog once:

```bash
skillshare hub add git@ghe.example.com:platform/ai-skills.git --label company -g
skillshare search code-review --hub company -g
# Select a skill to install, then distribute it to global targets
skillshare sync -g
```

An SSH hub URL reads `skillshare-hub.json` from the repository root. If the catalog lives elsewhere, append its path, for example `git@ghe.example.com:platform/ai-skills.git//catalog/skillshare-hub.json`. SSH access uses the teammate's existing SSH setup. The Git host must authorize access to both the catalog and each skill source. The hub is a discovery mechanism; it does not prevent installation from other sources.

### 5. Record project dependencies

For skills required by one project, install them in project mode and commit the resulting config and lockfile. Teammates then run `skillshare install -p`, audit and sync after cloning or pulling updates. Follow [Team Onboarding](/docs/how-to/recipes/team-onboarding-recipe) for the sequence. Keep catalog curation, skill updates and CLI upgrades as explicit reviewed changes.

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
