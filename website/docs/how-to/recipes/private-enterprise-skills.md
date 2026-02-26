---
sidebar_position: 3
---

# Recipe: Private Enterprise Skills

> Install skills from private repositories using token authentication.

## Scenario

Your organization hosts internal skills in a private GitHub/GitLab repository. You need to install and update these skills without exposing credentials in config files.

## Solution

### Step 1: Set up authentication

skillshare detects tokens from environment variables, with platform-specific vars taking priority over the generic fallback:

| Platform | Environment Variable |
|----------|---------------------|
| GitHub / GitHub Enterprise | `GITHUB_TOKEN` |
| GitLab / Self-hosted GitLab | `GITLAB_TOKEN` |
| Bitbucket | `BITBUCKET_TOKEN` (+ optional `BITBUCKET_USERNAME`) |
| Azure DevOps | `AZURE_DEVOPS_TOKEN` |
| Any platform (fallback) | `SKILLSHARE_GIT_TOKEN` |

```bash
# Option A: Git credential helper (recommended for GitHub)
gh auth login   # sets up git credential helper for HTTPS

# Option B: Platform-specific environment variable
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxx      # GitHub
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxx    # GitLab
export AZURE_DEVOPS_TOKEN=your-pat-here    # Azure DevOps

# Option C: Generic fallback (works with any HTTPS host)
export SKILLSHARE_GIT_TOKEN=your-token-here
```

### Step 2: Install from private repo

```bash
skillshare install your-org/internal-skills --track
```

skillshare detects the token automatically from the environment variables listed above.

### Step 3: Verify tracking

```bash
skillshare list
```

The installed repo appears with the `_` prefix (tracked repository):

```
_your-org-internal-skills/
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

- **Selective install**: `skillshare install your-org/internal-skills --track --skill code-review` installs only one skill
- **CI/CD token**: In pipelines, set the platform-specific env var (e.g., `GITHUB_TOKEN`) from CI secrets
- **Self-hosted GitLab**: Set `GITLAB_TOKEN` and use HTTPS URL: `skillshare install https://gitlab.internal.com/team/skills.git --track`
- **Gitee / AtomGit**: Supported via HTTPS URLs with `SKILLSHARE_GIT_TOKEN`

## Related

- [`install` command reference](/docs/reference/commands/install)
- [`update` command reference](/docs/reference/commands/update)
- [Organization sharing guide](/docs/how-to/sharing/organization-sharing)
- [URL formats reference](/docs/reference/appendix/url-formats)
