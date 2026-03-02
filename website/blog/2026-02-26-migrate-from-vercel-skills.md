---
slug: migrate-from-vercel-skills
title: skillshare vs. vercel/skills — When to Use Which
authors: [runkids]
tags: [comparison]
---

[vercel/skills](https://github.com/vercel-labs/skills) and skillshare are both CLI tools for managing AI coding skills across multiple agents. If you're choosing between them — or considering a migration — here's an honest comparison.

<!-- truncate -->

## What They Have in Common

Both tools solve the same core problem: managing AI skill files across 40+ coding agents (Claude Code, Cursor, Codex, etc.). Both offer:

- Install skills from Git repositories (GitHub, GitLab, and other hosts)
- Sync to multiple AI tool targets
- Support for symlink and copy modes
- Project-level and global skill management
- Security audit before installation

## Where vercel/skills Shines

**Best for quick, curated installs:**

- Runs via `npx skills` — no binary installation needed if you have Node.js
- Curated skill discovery via `npx skills find` with interactive selection
- [Server-side security audit](https://vercel.com/changelog/automated-security-audits-now-available-for-skills-sh) powered by Snyk — skills on the catalog are automatically scanned and flagged
- Strong Vercel/Next.js ecosystem integration
- Familiar npm-based workflow for JavaScript developers

**Use vercel/skills when:**
- You're already in the Node.js ecosystem
- You primarily install from the skills.sh catalog (which provides pre-audited skills)
- You want a curated, community-driven skill catalog
- You prefer `npx`-based tooling with no permanent install
- Your workflow is primarily single-machine, single-project

## Where skillshare Shines

**Best for multi-tool sync, multi-platform, and team workflows:**

- Single binary — no Node.js, npm, or runtime dependencies
- **Any Git host** — GitHub, GitLab, Bitbucket, Gitea, Azure DevOps, AtomGit, Codeberg, self-hosted, and any HTTPS/SSH git server
- Bidirectional sync: collect skills from targets back to source
- Cross-machine sync via `push`/`pull`
- **Fully customizable security audit** — 15+ detection patterns, configurable block thresholds, [custom rules](/docs/how-to/advanced/security) to enable/disable individual patterns, and multiple output formats (text, JSON, SARIF, Markdown). Runs locally on any skill source — no server dependency
- Pre-commit hook integration via the [pre-commit](https://pre-commit.com/) framework
- Backup/restore with timestamped snapshots
- Web dashboard (`skillshare ui`)
- Organization-wide skill distribution via tracked repos
- Works offline (core operations need no network)

**Use skillshare when:**
- You use multiple AI tools and need one source of truth
- Your skills live on GitLab, Bitbucket, Azure DevOps, or self-hosted Git — not just GitHub
- You work across multiple machines
- Your team needs standardized skills via git
- You need **customizable** security scanning — your own rules, thresholds, and CI integration for any skill source (not just a public catalog)
- You want zero runtime dependencies (CI/CD, Docker, air-gapped environments)

## Feature Comparison

| Feature | vercel/skills | skillshare |
|---------|--------------|------------|
| Install method | `npx` (Node.js) | Single binary |
| Git platform support | GitHub, GitLab, any git URL | GitHub, GitLab, Bitbucket, Gitea, GHE, Azure DevOps, AtomGit, Codeberg, any HTTPS/SSH host |
| Sync modes | Symlink, copy | Merge (per-skill symlink), symlink, copy |
| Multi-tool sync | Yes | Yes |
| Collect (target → source) | No | Yes |
| Cross-machine sync | No | Yes (`push`/`pull`) |
| Security audit | Yes (server-side via Snyk, catalog skills) | Yes (local, 15+ patterns, custom rules, any source) |
| Custom audit rules | No | Yes (enable/disable patterns, configurable thresholds) |
| Pre-commit hook | No | Yes ([pre-commit](https://pre-commit.com/) framework) |
| Audit output formats | CLI display | Text, JSON, SARIF, Markdown |
| Backup/restore | No | Yes |
| Web UI | No | Yes |
| Hub/registry | Community catalog (skills.sh) | Self-hosted hub index |
| Offline operation | Needs npm | Yes (core operations) |
| Project skills | Yes | Yes |

## Migrating from vercel/skills

If you decide to switch, the process is straightforward:

### Step 1: Install skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
skillshare init
```

### Step 2: Collect existing skills

If vercel/skills already synced skills to your AI tool directories:

```bash
skillshare collect
```

This copies skills from your target directories into skillshare's source.

### Step 3: Sync

```bash
skillshare sync
```

### Step 4: Ongoing updates

```bash
skillshare check          # Detect upstream changes
skillshare update --all   # Apply updates
skillshare sync           # Push to all tools
```

## Can They Coexist?

Yes. Both tools use symlinks (or copies) to the same target directories. However, running both simultaneously on the same targets may cause conflicts — one tool's symlinks may be overwritten by the other. If you're evaluating both, use them on separate targets or test one at a time.

## Resources

- [Migration guide](/docs/how-to/advanced/migration)
- [Install command reference](/docs/reference/commands/install)
- [Security audit guide](/docs/how-to/advanced/security)
