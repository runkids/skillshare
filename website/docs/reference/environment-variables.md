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

## GitHub API

### GITHUB_TOKEN

GitHub personal access token for API requests.

**When needed:**
- Upgrading skillshare CLI
- Installing from private repos
- Hitting rate limits

**Usage:**
```bash
export GITHUB_TOKEN=ghp_your_token_here
skillshare upgrade
```

**Creating a token:**
1. Go to https://github.com/settings/tokens
2. Generate new token (classic)
3. No scopes needed for public repos
4. Copy the token

**Windows:**
```powershell
# Current session
$env:GITHUB_TOKEN = "ghp_your_token"

# Permanent
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

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
| `GITHUB_TOKEN` | GitHub API auth | None |
| `SKILLSHARE_TEST_BINARY` | Test binary path | `bin/skillshare` |

---

## Related

- [Configuration](/docs/targets/configuration) — Config file reference
- [Windows Issues](/docs/troubleshooting/windows) — Windows environment setup
