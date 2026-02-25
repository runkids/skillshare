---
sidebar_position: 3
---

# Recipe: Private Enterprise Skills

> Install skills from private repositories using token authentication.

## Scenario

Your organization hosts internal skills in a private GitHub/GitLab repository. You need to install and update these skills without exposing credentials in config files.

## Solution

### Step 1: Set up authentication

skillshare uses the same token resolution as `git`:

```bash
# Option A: GitHub CLI (recommended)
gh auth login

# Option B: Environment variable
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxx

# Option C: For Azure DevOps
export AZURE_DEVOPS_TOKEN=your-pat-here
```

### Step 2: Install from private repo

```bash
skillshare install your-org/internal-skills
```

skillshare detects the token automatically from `gh auth token`, `GITHUB_TOKEN`, `GH_TOKEN`, or `AZURE_DEVOPS_TOKEN`.

### Step 3: Verify tracking

```bash
skillshare list
```

The installed repo appears with the `_` prefix (tracked repository):

```
_your-org__internal-skills/
├── code-review/
├── testing-standards/
└── deployment-checklist/
```

### Step 4: Update cycle

```bash
skillshare check    # Detect upstream changes
skillshare update   # Pull latest
skillshare sync     # Push to targets
```

## Verification

- `skillshare list` shows the tracked repo
- `skillshare check` can reach the remote and compare hashes
- `skillshare sync` creates symlinks in all targets

## Variations

- **Selective install**: `skillshare install your-org/internal-skills --skill code-review` installs only one skill
- **CI/CD token**: In pipelines, use `GITHUB_TOKEN` from CI secrets
- **Self-hosted GitLab**: Use HTTPS URL directly: `skillshare install https://gitlab.internal.com/team/skills.git`
- **Gitee / AtomGit**: Supported via HTTPS URLs with token auth

## Related

- [`install` command reference](/docs/reference/commands/install)
- [`update` command reference](/docs/reference/commands/update)
- [Organization sharing guide](/docs/how-to/sharing/organization-sharing)
- [URL formats reference](/docs/reference/appendix/url-formats)
