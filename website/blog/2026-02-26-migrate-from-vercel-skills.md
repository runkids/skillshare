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

- Install skills from Git repositories
- Sync to multiple AI tool targets
- Support for symlink and copy modes
- Project-level and global skill management

## Where vercel/skills Shines

**Best for quick, curated installs:**

- Runs via `npx skills` — no binary installation needed if you have Node.js
- Curated skill discovery via `npx skills find` with interactive selection
- Strong Vercel/Next.js ecosystem integration
- Familiar npm-based workflow for JavaScript developers

**Use vercel/skills when:**
- You're already in the Node.js ecosystem
- All your skill repos are on GitHub (it currently only supports GitHub)
- You want a curated, community-driven skill catalog
- You prefer `npx`-based tooling with no permanent install
- Your workflow is primarily single-machine, single-project

## Where skillshare Shines

**Best for multi-tool sync, multi-platform, and team workflows:**

- Single binary — no Node.js, npm, or runtime dependencies
- **Any Git host** — GitHub, GitLab, Bitbucket, Gitea, Azure DevOps, AtomGit, Codeberg, self-hosted, and any HTTPS/SSH git server
- Bidirectional sync: collect skills from targets back to source
- Cross-machine sync via `push`/`pull`
- Built-in security audit (15+ detection patterns, auto-block on install)
- Backup/restore with timestamped snapshots
- Web dashboard (`skillshare ui`)
- Organization-wide skill distribution via tracked repos
- Works offline (core operations need no network)

**Use skillshare when:**
- You use multiple AI tools and need one source of truth
- Your skills live on GitLab, Bitbucket, Azure DevOps, or self-hosted Git — not just GitHub
- You work across multiple machines
- Your team needs standardized skills via git
- You need security scanning for untrusted skill sources
- You want zero runtime dependencies (CI/CD, Docker, air-gapped environments)

## Feature Comparison

| Feature | vercel/skills | skillshare |
|---------|--------------|------------|
| Install method | `npx` (Node.js) | Single binary |
| Git platform support | GitHub only | GitHub, GitLab, Bitbucket, Gitea, GHE, Azure DevOps, AtomGit, Codeberg, any HTTPS/SSH host |
| Sync modes | Symlink, copy | Merge (per-skill symlink), symlink, copy |
| Multi-tool sync | Yes | Yes |
| Collect (target → source) | No | Yes |
| Cross-machine sync | No | Yes (`push`/`pull`) |
| Security audit | No | Yes (15+ patterns) |
| Backup/restore | No | Yes |
| Web UI | No | Yes |
| Hub/registry | Community catalog | Self-hosted hub index |
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
