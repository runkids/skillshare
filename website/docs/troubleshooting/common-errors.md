---
sidebar_position: 2
---

# Common Errors

Error messages and their solutions.

## Config Errors

### `config not found: run 'skillshare init' first`

**Cause:** No configuration file exists.

**Solution:**
```bash
skillshare init
```

Add `--source` if you want a custom path:
```bash
skillshare init --source ~/my-skills
```

---

### `failed to load project config: ...`

**Cause:** `.skillshare/config.yaml` exists but cannot be parsed (malformed YAML, wrong types, etc.). Mutating commands (`uninstall`, `new`, `enable`/`disable`, `check`) refuse to proceed in this state so they don't accidentally touch the default `.skillshare/skills/` directory when you have a custom `sources` configuration.

**Solution:** Fix the YAML and re-run the command. Common issues:

```yaml
# WRONG — targets must be a list
targets: {}

# RIGHT
targets: []
```

```yaml
# WRONG — skills must be a list
skills: my-skill

# RIGHT
skills:
  - name: my-skill
    source: github.com/org/my-skill
```

Validate the file with any YAML linter, or temporarily restore from `.skillshare/backups/` if you have one.

---

### `target "<name>": skills target path X overlaps skills source Y`

**Cause:** Your `sources.skills` resolves to the same directory as a target's skills path (or one contains the other). For example, configuring `sources.skills: .claude/skills` together with a `claude` target — both point to `.claude/skills/`. Without this guard, `sync --force` would treat the source as a target directory and delete its contents.

**Solution:** Choose a source path that does not alias any target. Common safe choices:

```yaml
# Co-locate with project docs
sources:
  skills: ./docs/skills

# Keep under .skillshare/ (default — remove the sources key entirely)
```

The same check applies to `sources.agents` against agent target paths.

---

## Target Errors

### `target add: path does not exist`

**Cause:** The skills directory doesn't exist yet.

**Solution:**
```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### `target path does not end with 'skills'`

**Cause:** Warning that path doesn't follow convention.

**Solution:** This is a warning, not an error. Proceed if your path is intentional, or fix it:
```bash
skillshare target add myapp ~/.myapp/skills  # Preferred
```

### `target directory already exists with files`

**Cause:** Target has existing files that might be overwritten.

**Solution:**
```bash
skillshare backup
skillshare sync
```

---

## Sync Errors

### `deleting a symlinked target removed source files`

**Cause:** You ran `rm -rf` on a target in symlink mode.

**Solution:**
```bash
# If git is initialized
cd ~/.config/skillshare/skills
git checkout -- .

# Or restore from backup
skillshare restore <target>
```

**Prevention:** Use `skillshare target remove` instead of manual deletion.

### `sync seems stuck or slow`

**Cause:** Large files in skills directory.

**Solution:** Add ignore patterns:
```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

---

## Git Errors

### `Could not read from remote repository`

**Cause:** SSH key not set up, or the remote URL is wrong.

**Solution:**
```bash
# Check SSH access
ssh -T git@github.com

# If SSH isn't set up, use HTTPS instead
git -C ~/.config/skillshare/skills remote set-url origin https://github.com/you/my-skills.git

# Or set up SSH keys
ssh-keygen -t ed25519 -C "you@example.com"
# Then add the public key to GitHub → Settings → SSH keys
```

### `push: remote has changes`

**Cause:** Remote repository is ahead of local.

**Solution:**
```bash
skillshare pull   # Get remote changes first
skillshare push   # Now push works
```

### `pull: local has uncommitted changes`

**Cause:** You have local changes that haven't been pushed.

**Solution:**
```bash
# Option 1: Push your changes first
skillshare push -m "Local changes"
skillshare pull

# Option 2: Discard local changes
cd ~/.config/skillshare/skills
git checkout -- .
skillshare pull
```

### `merge conflicts`

**Cause:** Same file was edited on multiple machines.

**Solution:**
```bash
cd ~/.config/skillshare/skills
git status                    # See conflicted files
# Edit files to resolve conflicts
git add .
git commit -m "Resolve conflicts"
skillshare sync
```

### `Git identity not configured`

**Cause:** No `user.name` / `user.email` in git config. skillshare uses a local fallback (`skillshare@local`) so init can complete, but you should set your own.

**Solution:**
```bash
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
```

### `Git root mismatch`

**Cause:** `git_root` in `config.yaml` points to a scope directory that has no git repo, but another scope directory does. This happens when you change `git_root` without relocating the repository — switching scope means "start versioning a different directory", not "move the existing history". See [`git_root`](/docs/reference/targets/configuration#git-root).

**Solution:** Pick one of the three options the error prints:
```bash
# Start a fresh repo at the configured scope (no history)
skillshare init --git-root <scope>

# Move the existing repo over, keeping history
mv <old-scope>/.git <new-scope>/.git

# Or keep using the existing repo: set git_root back in config.yaml
#   git_root: <scope-that-has-the-repo>
```

### `nested git repositories must be disabled first`

**Cause:** With `git_root: root`, a subdirectory (e.g. a tracked skill repo under `skills/_org/`) has its own `.git`. Git would upload it as an **empty submodule**, silently dropping its files, so `commit`/`push` abort until each nested repo is disabled.

**Solution:**
```bash
# Disable each reported nested repo (reversible — just rename back to re-enable)
mv ~/.config/skillshare/<dir>/.git ~/.config/skillshare/<dir>/.git.disabled
```
Or use the one-click disable on the web UI Git Sync page. skillshare also keeps `config.yaml` out of a root-scope repo automatically (it holds machine-specific paths).

### `Invalid git_root`

**Cause:** `git_root` in `config.yaml` is set to an unrecognized value (e.g. a typo).

**Solution:** Use one of `skills`, `agents`, `extras`, or `root` — or leave it empty (defaults to `skills`).

---

## Install Errors

### `skill already exists`

**Cause:** A skill with the same name is already installed.

**Solution:**
```bash
# Update the existing skill
skillshare install <source> --update

# Or force overwrite
skillshare install <source> --force
```

### `git failed (exit 128): repository not found or authentication required`

**Cause:** The repository URL is wrong, the repo doesn't exist, or authentication is missing.

skillshare now provides actionable error messages for common git failures instead of raw exit codes. The error message includes suggestions:

```
Error: git failed (exit 128): repository not found or authentication required
```

If a token was used but rejected:

```
Error: git failed (exit 128): authentication token was rejected — check permissions and expiry
```

**Solution:** See the authentication options below.

### `Authentication failed` / `Access denied`

**Cause:** HTTPS credentials are missing, expired, or wrong token type.

**Solution — Option 1: Set a token env var:**

```bash
# GitHub
export GITHUB_TOKEN=ghp_xxxx

# GitLab (must be a Personal Access Token, prefix glpat-)
export GITLAB_TOKEN=glpat-xxxx

# Bitbucket
export BITBUCKET_TOKEN=your_app_password
```

**Windows (PowerShell):**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxx"

# Permanent (survives restarts)
[Environment]::SetEnvironmentVariable("GITLAB_TOKEN", "glpat-xxxx", "User")
```

**Solution — Option 2: Use SSH URL:**
```bash
skillshare install git@github.com:team/private-skills.git
skillshare install git@gitlab.com:team/skills.git
skillshare install git@bitbucket.org:team/skills.git
```

**Solution — Option 3: Git credential helper:**
```bash
gh auth login          # GitHub CLI
git credential approve # or platform-specific credential manager
```

**Required token permissions:**

| Platform | Token type | Scopes / Permissions |
|----------|-----------|---------------------|
| GitHub | Personal Access Token (`ghp_`) | `repo` (private repos), none (public) |
| GitLab | Personal Access Token (`glpat-`) | `read_repository` + `write_repository` |
| Bitbucket | Repository Access Token | Read + Write |
| Bitbucket | App Password + `BITBUCKET_USERNAME` | Repositories: Read + Write |

:::warning GitLab token types
Only **Personal Access Token** (`glpat-`) works for git operations. Feed Tokens (`glft-`) do **not** have git access.
:::

See [Environment Variables](/docs/reference/appendix/environment-variables#git-authentication) and [Private Repositories](/docs/reference/commands/install#private-repositories).

### `SSL certificate problem` / `certificate verification failed`

**Cause:** The Git server uses a self-signed certificate or an internal CA that your system doesn't trust. Common with self-hosted GitLab, Gitea, or Gogs instances.

**Solution — Option 1: Custom CA bundle (recommended):**
```bash
export GIT_SSL_CAINFO=/path/to/company-ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**Solution — Option 2: Use SSH instead (avoids SSL entirely):**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

**Solution — Option 3: Disable SSL verification (not recommended):**
```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

:::warning
Disabling SSL verification is a security risk. Prefer Option 1 or 2.
:::

See [Environment Variables — Git SSL / TLS](/docs/reference/appendix/environment-variables#git-ssl--tls).

### `invalid skill: SKILL.md not found`

**Cause:** The source doesn't have a valid SKILL.md file.

**Solution:** Check the source path is correct and points to a skill directory.

---

## Update Errors

### `git failed: Need to specify how to reconcile divergent branches`

**Cause:** The remote branch has diverged from your local tracked copy.

**Solution:**
```bash
# Force update (replaces local with remote)
skillshare update --force

# Or manually resolve
cd ~/.config/skillshare/skills/_repo-name
git pull --rebase
```

:::tip
`skillshare update` and `skillshare install` now show actionable error messages for git failures (authentication, SSL, divergent branches) instead of raw exit codes.
:::

---

## Audit Errors

### `security audit failed — critical threats detected`

**Cause:** The skill contains patterns matching critical security threats (prompt injection, data exfiltration, credential access).

**Solution:**
```bash
# Review the findings
skillshare audit <skill-name>

# If you trust the source, force install
skillshare install <source> --force
```

### `audit HIGH: Hidden zero-width Unicode characters detected`

**Cause:** The skill contains invisible Unicode characters, which may be a copy-paste artifact or intentional obfuscation.

**Solution:** Open the file in an editor that shows hidden characters and remove them, or force install if you trust the source.

---

## Upgrade Errors

### `GitHub API rate limit exceeded`

**Cause:** Too many unauthenticated API requests.

**Solution:**
```bash
# Option 1: Set a GitHub token (recommended)
export GITHUB_TOKEN=ghp_your_token_here
skillshare upgrade

# Option 2: Force upgrade
skillshare upgrade --cli --force
```

Create a token at: https://github.com/settings/tokens (no scopes needed for public repos)

---

## Skill Errors

### `skill not appearing in AI CLI`

**Causes:**
1. Skill not synced
2. Invalid SKILL.md format
3. AI CLI caching

**Solutions:**
```bash
# 1. Sync
skillshare sync

# 2. Check format
skillshare doctor

# 3. Restart AI CLI
```

### `skill name 'X' is defined in multiple places`

**Cause:** Multiple skills have the same `name` field and land on the same target.

**Solution:** Rename one in SKILL.md or use `include`/`exclude` filters to route them to different targets:
```yaml
# Option 1: Namespace in SKILL.md
name: team-a-skill-name

# Option 2: Route with filters (global config)
targets:
  codex:
    path: ~/.codex/skills
    include: [_team-a__*]
  claude:
    path: ~/.claude/skills
    include: [_team-b__*]

# Option 2: Route with filters (project config)
targets:
  - name: claude
    exclude: [codex-*]
  - name: codex
    include: [codex-*]
```

:::tip
If filters already isolate the duplicates, sync shows an info message instead of a warning — no action needed.
See [Target Filters](/docs/reference/targets/configuration#include--exclude-target-filters) for full syntax.
:::

---

## Agent Errors

### Warning: `target(s) skipped for agents (no agents path)`

**Cause:** You ran `skillshare sync` (or `skillshare sync agents`) and one or more configured targets don't define an agents directory. Only Claude, Cursor, Augment, and OpenCode have built-in agent paths; other targets are silently skipped.

**Solutions:**

1. Ignore the warning if those targets don't need agents.
2. Add an `agents:` sub-key to the target in `config.yaml` to enable agent sync for it:

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

Then re-run `skillshare sync agents`.

### `backup is not supported in project mode (except for agents)`

**Cause:** You ran `skillshare backup -p` (or `skillshare backup -p <target>`) without the `agents` filter. In project mode, only agent backups are supported — skill backups are global-only.

**Solution:** Add the `agents` positional argument or use `--all`:

```bash
skillshare backup -p agents          # Project agent targets
skillshare backup -p agents claude   # Specific target
skillshare backup -p --all           # Same as above (narrows to agents)
```

The same rule applies to `restore`: `restore is not supported in project mode (except for agents)`.

### `agent name 'X' has invalid characters`

**Cause:** An agent filename or `name:` frontmatter field contains characters outside the allowed set.

**Solution:** Agent names must use only `a-z`, `0-9`, `_`, `-`, `.`. Rename the file (and update its `name:` field to match) so they share the same canonical name.

### `.agentignore` patterns not taking effect

**Causes:**

1. The file is in the wrong location. It must live at the agents source root: `~/.config/skillshare/agents/.agentignore` (global) or `.skillshare/agents/.agentignore` (project).
2. Your pattern matches a different segment than you expect — the file uses [gitignore syntax](https://git-scm.com/docs/gitignore).

**Solution:** Confirm the file path with `skillshare doctor` and re-check the pattern. Agents are matched by basename (without `.md`), so `draft-*` matches `draft-experiment.md`. Use `skillshare disable <agent> --kind agent` to let the CLI write the entry for you.

---

## Binary Errors

### `integration tests cannot find the binary`

**Cause:** Binary not built or wrong path.

**Solution:**
```bash
go build -o bin/skillshare ./cmd/skillshare
# Or set
export SKILLSHARE_TEST_BINARY=/path/to/skillshare
```

---

## Still Having Issues?

See [Troubleshooting Workflow](./troubleshooting-workflow.md) for a systematic debugging approach.
