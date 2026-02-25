---
sidebar_position: 4
---

# URL Formats

All source URL patterns recognized by `skillshare install`.

## Quick Reference

| Format | Example | Notes |
|--------|---------|-------|
| GitHub shorthand | `owner/repo` | Expands to `github.com/owner/repo` |
| GitHub with subdir | `owner/repo/path/to/skill` | Installs specific skill from repo |
| Full HTTPS | `https://github.com/owner/repo` | Any Git host |
| Full HTTPS with subdir | `https://github.com/owner/repo/path` | Subdir after host/owner/repo |
| SSH | `git@github.com:owner/repo.git` | Private repos via SSH key |
| SSH with subdir | `git@github.com:owner/repo.git//path` | `//` separates repo from subdir |
| GHE Cloud | `mycompany.github.com/org/repo` | Enterprise Cloud subdomain |
| GHE Server | `github.mycompany.com/org/repo` | Enterprise Server |
| Azure DevOps shorthand | `ado:org/project/repo` | Expands to `dev.azure.com` URL |
| Azure DevOps HTTPS | `https://dev.azure.com/org/proj/_git/repo` | Modern format |
| Azure DevOps SSH | `git@ssh.dev.azure.com:v3/org/proj/repo` | SSH v3 format |
| Local path | `~/my-skill` or `/abs/path` | Copies directory to source |
| Git file URL | `file:///path/to/repo` | Local git clone (for testing) |

## GitHub Shorthand

The simplest format — just `owner/repo`:

```bash
skillshare install anthropics/skills
skillshare install ComposioHQ/awesome-claude-skills
```

This expands to `https://github.com/owner/repo` internally.

### With Subdirectory

Add a path after `owner/repo` to install a specific skill:

```bash
skillshare install anthropics/skills/skills/pdf
skillshare install anthropics/skills/skills/commit
```

When the subdir doesn't match exactly, skillshare scans the repo for a skill with that basename:

```bash
# "pdf" doesn't exist at root, but found at skills/pdf/ — resolves automatically
skillshare install anthropics/skills/pdf
```

## Full HTTPS URLs

Works with any Git host:

```bash
# GitHub
skillshare install https://github.com/owner/repo

# GitLab
skillshare install https://gitlab.com/owner/repo

# Bitbucket
skillshare install https://bitbucket.org/owner/repo

# Self-hosted Gitea
skillshare install https://git.mycompany.com/team/skills
```

## SSH URLs

Use SSH for private repositories:

```bash
# Standard SSH
skillshare install git@github.com:owner/repo.git

# With subdirectory (note the // separator)
skillshare install git@github.com:owner/repo.git//path/to/skill

# GitLab SSH
skillshare install git@gitlab.com:owner/repo.git
```

:::info The `//` separator
For SSH URLs, use `//` to separate the repo from the subdirectory path. This is because the `:` in SSH URLs already serves as a separator, so the standard `/` path convention would be ambiguous.
:::

## GitHub Enterprise

Enterprise hostnames are recognized automatically:

```bash
# Enterprise Cloud (subdomain pattern: *.github.com)
skillshare install mycompany.github.com/org/repo

# Enterprise Server (hostname pattern: github.*.*)
skillshare install github.mycompany.com/org/repo
skillshare install github.internal.corp/team/skills
```

Both patterns support subdirectory paths:

```bash
skillshare install github.mycompany.com/org/repo/path/to/skill
```

## Azure DevOps

### Shorthand

The `ado:` prefix expands to Azure DevOps URLs:

```bash
skillshare install ado:myorg/myproject/myrepo
skillshare install ado:myorg/myproject/myrepo/skills/react
```

### Full URLs

```bash
# Modern format
skillshare install https://dev.azure.com/myorg/myproject/_git/myrepo

# Legacy format (auto-normalized to dev.azure.com)
skillshare install https://myorg.visualstudio.com/myproject/_git/myrepo

# SSH
skillshare install git@ssh.dev.azure.com:v3/myorg/myproject/myrepo
```

## Local Paths

Install from a directory on your filesystem:

```bash
# Absolute path
skillshare install /home/user/my-skill

# Home directory shorthand
skillshare install ~/my-skill

# Relative path
skillshare install ./local-skill
```

Local installs **copy** files (not symlink) and are not updatable via `skillshare update`.

## Authentication

### SSH Keys (Recommended for Private Repos)

```bash
# Ensure your SSH key is loaded
ssh-add ~/.ssh/id_ed25519

# Install via SSH
skillshare install git@github.com:company/private-skills.git
```

### HTTPS with Tokens

For HTTPS URLs, git uses your configured credential helper:

```bash
# Configure git credential helper (one-time)
git config --global credential.helper store

# Or use GH CLI for GitHub
gh auth login

# Then install normally
skillshare install https://github.com/company/private-repo
```

:::tip Private Repos
If you get an authentication error with HTTPS, switch to SSH URLs. skillshare sets `GIT_TERMINAL_PROMPT=0` to prevent hanging credential prompts, so interactive HTTPS auth won't work.
:::

## Platform Support

| Feature | GitHub | GitLab | Bitbucket | Gitea | GHE | Azure DevOps |
|---------|--------|--------|-----------|-------|-----|--------------|
| Shorthand (`owner/repo`) | Yes | No | No | No | Yes | `ado:` prefix |
| Full HTTPS URL | Yes | Yes | Yes | Yes | Yes | Yes |
| SSH URL | Yes | Yes | Yes | Yes | Yes | Yes |
| Subdirectory | Yes | Yes | Yes | Yes | Yes | Yes |
| `skillshare search` | Yes | No | No | No | No | No |

## Related

- [Install command](../commands/install.md) — full install options and examples
