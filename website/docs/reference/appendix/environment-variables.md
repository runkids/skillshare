---
sidebar_position: 2
---

# Environment Variables

All environment variables recognized by skillshare.

## Configuration

### SKILLSHARE_CONFIG

Override the config file path.

```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

**Default:** `~/.config/skillshare/config.yaml`

---

### XDG_CONFIG_HOME

Override the base configuration directory per the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/).

```bash
export XDG_CONFIG_HOME=~/my-config
# skillshare will use ~/my-config/skillshare/
```

**Default behavior:**

| Platform | Default |
|----------|---------|
| Linux | `~/.config/skillshare/` |
| macOS | `~/.config/skillshare/` |
| Windows | `%AppData%\skillshare\` |

**Priority:** `SKILLSHARE_CONFIG` > `XDG_CONFIG_HOME` > platform default.

:::note
If you set `XDG_CONFIG_HOME` after initial setup, move your existing `~/.config/skillshare/` directory to the new location manually.
:::

---

### XDG_DATA_HOME

Override the data directory (backups, trash).

```bash
export XDG_DATA_HOME=~/my-data
# skillshare will use ~/my-data/skillshare/backups/ and ~/my-data/skillshare/trash/
```

**Default:** `~/.local/share/skillshare/`

---

### XDG_STATE_HOME

Override the state directory (operation logs).

```bash
export XDG_STATE_HOME=~/my-state
# skillshare will use ~/my-state/skillshare/logs/
```

**Default:** `~/.local/state/skillshare/`

---

### XDG_CACHE_HOME

Override the cache directory (version check cache, UI dist cache).

```bash
export XDG_CACHE_HOME=~/my-cache
# skillshare will use ~/my-cache/skillshare/
```

**Default:** `~/.cache/skillshare/`

:::tip Automatic migration
Starting from v0.13.0, skillshare follows the XDG Base Directory Specification for backups, trash, and logs. If you're upgrading from an older version, these directories are automatically migrated from `~/.config/skillshare/` to their proper XDG locations on first run.
:::

---

## GitHub API

### GITHUB_TOKEN

GitHub personal access token.

**Used for:**
- GitHub API requests (`skillshare search`, `skillshare upgrade`, version check)
- **Git clone authentication** — automatically injected when installing private repos via HTTPS

**Creating a token:**
1. Go to https://github.com/settings/tokens
2. Generate new token (classic)
3. Scope: `repo` for private repos, none for public repos
4. Copy the token

Official docs: [Managing your personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)

**Usage:**
```bash
export GITHUB_TOKEN=ghp_your_token_here
skillshare install https://github.com/org/private-skills.git --track
```

**Windows:**
```powershell
# Current session
$env:GITHUB_TOKEN = "ghp_your_token"

# Permanent
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

### GH_TOKEN

Alternative GitHub token variable. Used by the [GitHub CLI (`gh`)](https://cli.github.com/) and recognized by skillshare as a fallback.

**Token resolution order:** `GITHUB_TOKEN` → `GH_TOKEN` → `gh auth token`

If you already use `gh` and have authenticated via `gh auth login`, skillshare picks up the token automatically — no extra env vars needed.

```bash
export GH_TOKEN=ghp_your_token_here
skillshare search "react patterns"
```

---

## Git Authentication

These variables enable HTTPS authentication for private repositories. When set, skillshare automatically injects the token during `install` and `update` — no URL modification needed.

See [Private Repositories](/docs/reference/commands/install#private-repositories) for details and CI/CD examples.


### GITLAB_TOKEN

GitLab personal access or CI job token. Used for HTTPS clone of GitLab-hosted private repos.

**Creating a token:**
1. Go to https://gitlab.com/-/user_settings/personal_access_tokens
2. Add new token
3. Scopes: `read_repository` (pull only) or `read_repository` + `write_repository` (push & pull)
4. Copy the token (prefix `glpat-`)

Official docs: [Token overview](https://docs.gitlab.com/security/tokens/)

:::warning Token types
Only **Personal Access Token** (`glpat-`) and **Project/Group Access Token** work for git operations. Feed Tokens (`glft-`) do **not** have git access.
:::

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
skillshare install https://gitlab.com/org/skills.git --track
```

**Windows:**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxxxxxxxxxxxxxxxxxx"
```

### BITBUCKET_TOKEN

Bitbucket repository access token, or app password. Used for HTTPS clone of Bitbucket-hosted private repos.

**Creating a token (repository access token):**
1. Go to Repository → Settings → Access tokens
2. Create token with **Read** permission (pull only) or **Read + Write** (push & pull)
3. Copy the token — uses `x-token-auth` automatically (no username needed)

**Creating a token (app password):**
1. Go to https://bitbucket.org/account/settings/app-passwords/
2. Create app password with **Repositories: Read** (pull only) or **Repositories: Read + Write** (push & pull)
3. Also set `BITBUCKET_USERNAME` (or include it in the URL as `https://<username>@bitbucket.org/...`)

Official docs: [Access tokens](https://support.atlassian.com/bitbucket-cloud/docs/access-tokens/)

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

**Windows:**
```powershell
$env:BITBUCKET_USERNAME = "your_bitbucket_username"
$env:BITBUCKET_TOKEN = "your_app_password"
```

### BITBUCKET_USERNAME

Bitbucket username used with `BITBUCKET_TOKEN` when that token is an app password.

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

### AZURE_DEVOPS_TOKEN

Azure DevOps Personal Access Token (PAT). Used for HTTPS clone of Azure DevOps-hosted private repos.

**Creating a token:**
1. Go to `https://dev.azure.com/{org}/_usersSettings/tokens`
2. Select **+ New Token**
3. Scopes: **Code → Read** (pull only) or **Code → Read & Write** (push & pull)
4. Copy the token (84-character string with `AZDO` signature)

Official docs: [Use Personal Access Tokens](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo --track
```

**Windows:**
```powershell
$env:AZURE_DEVOPS_TOKEN = "your_pat_here"
```

### SKILLSHARE_GIT_TOKEN

Generic fallback token for any HTTPS git host. Used when no platform-specific token is set.

```bash
export SKILLSHARE_GIT_TOKEN=your_token
skillshare install https://git.example.com/org/skills.git --track
```

**Windows:**
```powershell
$env:SKILLSHARE_GIT_TOKEN = "your_token"
```

**Token priority:** Platform-specific (`GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`, `AZURE_DEVOPS_TOKEN`) > `SKILLSHARE_GIT_TOKEN`.

---

## Testing

### SKILLSHARE_TEST_BINARY

Override the CLI binary path for integration tests.

```bash
SKILLSHARE_TEST_BINARY=/path/to/skillshare go test ./tests/integration
```

**Default:** `bin/skillshare` in project root

---

## Usage Examples

### Temporary override

```bash
# Single command
SKILLSHARE_CONFIG=/tmp/test-config.yaml skillshare status

# Multiple commands
export SKILLSHARE_CONFIG=/tmp/test-config.yaml
skillshare status
skillshare list
unset SKILLSHARE_CONFIG
```

### Permanent setup (macOS/Linux)

Add to `~/.bashrc` or `~/.zshrc`:
```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

### Permanent setup (Windows)

```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

## Summary

| Variable | Purpose | Default |
|----------|---------|---------|
| `SKILLSHARE_CONFIG` | Config file path | `~/.config/skillshare/config.yaml` |
| `XDG_CONFIG_HOME` | Base config directory | `~/.config` (Linux/macOS), `%AppData%` (Windows) |
| `XDG_DATA_HOME` | Data directory (backups, trash) | `~/.local/share` |
| `XDG_STATE_HOME` | State directory (logs) | `~/.local/state` |
| `XDG_CACHE_HOME` | Cache directory (version check, UI) | `~/.cache` |
| `GITHUB_TOKEN` | GitHub API + git clone auth | None |
| `GH_TOKEN` | GitHub API (fallback for `GITHUB_TOKEN`) | None |
| `GITLAB_TOKEN` | GitLab git clone auth | None |
| `BITBUCKET_TOKEN` | Bitbucket git clone auth | None |
| `BITBUCKET_USERNAME` | Bitbucket username for app password auth | None |
| `AZURE_DEVOPS_TOKEN` | Azure DevOps git clone auth | None |
| `SKILLSHARE_GIT_TOKEN` | Generic git clone auth (fallback) | None |
| `SKILLSHARE_TEST_BINARY` | Test binary path | `bin/skillshare` |

---

## Related

- [Configuration](/docs/reference/targets/configuration) — Config file reference
- [Windows Issues](/docs/troubleshooting/windows) — Windows environment setup
