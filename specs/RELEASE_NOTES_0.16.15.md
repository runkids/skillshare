# skillshare v0.16.15 Release Notes

Release date: 2026-03-11

## TL;DR

v0.16.15 adds **custom GitLab domain support** and fixes **nested subgroup URL parsing** (Issue #72):

1. **JihuLab auto-detection** — hosts containing `jihulab` (e.g., `jihulab.com`) are now auto-detected alongside `gitlab` for nested subgroup support
2. **`gitlab_hosts` config** — declare self-managed GitLab hostnames (e.g., `git.company.com`) so URLs are parsed with nested subgroup support
3. **`SKILLSHARE_GITLAB_HOSTS` env var** — comma-separated list for CI/CD pipelines without a config file; merged with config values, invalid entries silently skipped
4. **GitLab nested subgroups** — URLs like `group/subgroup/project` are now treated as the full repo path instead of being split at the 2nd segment
5. **`.git` boundary** — use `.git` suffix to explicitly mark where the repo path ends and the subdir begins

No breaking changes. Drop-in upgrade from v0.16.14.

---

## JihuLab Auto-Detection

JihuLab (`jihulab.com`) is GitLab's official Chinese distribution with full nested subgroup support, but its hostname doesn't contain `"gitlab"`. Built-in detection now checks for both `"gitlab"` and `"jihulab"` in the hostname via `isGitLabHost()`, so URLs like `jihulab.com/group/sub/project` work without any `gitlab_hosts` config.

Other GitLab-like platforms (e.g., OneDev) that use nested paths but have unrelated hostnames still require `gitlab_hosts` as a workaround.

---

## Custom GitLab Domain Support

### The problem

v0.16.15's subgroup fix introduced platform-aware URL parsing: hosts containing `"gitlab"` treat the full path as repo (for nested subgroups), while other hosts use 2-segment `owner/repo` split. Self-managed GitLab instances on custom domains (e.g., `git.company.com`) without `"gitlab"` in the hostname still fell back to the 2-segment split, breaking nested subgroup URLs.

### Solution

New `gitlab_hosts` config field in both global and project config. Entries must be bare hostnames (no scheme, path, or port), normalized to lowercase.

```yaml
# ~/.config/skillshare/config.yaml
gitlab_hosts:
  - git.company.com
  - code.internal.io
```

For CI/CD without a config file, use the `SKILLSHARE_GITLAB_HOSTS` env var:

```bash
SKILLSHARE_GITLAB_HOSTS=git.company.com skillshare install git.company.com/team/frontend/ui
```

### Design decisions

- **`EffectiveGitLabHosts()` method pattern** — the `GitLabHosts` field on `Config`/`ProjectConfig` stores only config-file values and is safe to persist via `Save()`. The `EffectiveGitLabHosts()` method merges the env var at read time, preventing env-only hosts from leaking into the config file on save
- **`ParseOptions` struct** — `ParseSourceWithOptions(input, opts)` threads `GitLabHosts` through the parser without changing `ParseSource()` call sites that don't need custom hosts
- **`isGitLabHost()` helper** — checked after explicit markers (`.git`, `/-/`, `/src/`) but before the 2-segment fallback, so markers always win over the host heuristic
- **Project mode isolation** — in project mode, `parseOpts()` uses project config unconditionally (never falls back to global config), matching the dual-mode design principle
- **Env var is additive** — merges with config file entries (deduplicated), never replaces them. Invalid entries (scheme, path, port, empty) are silently skipped in env var but hard-error in config file
- **Validation** — `isValidGitLabHostname()` shared by both `normalizeGitLabHosts()` (config file, strict) and `mergeGitLabHostsFromEnv()` (env var, lenient)

### Usage patterns

```bash
# Config-based (persistent)
gitlab_hosts:
  - git.company.com

skillshare install git.company.com/team/frontend/ui
# → clones https://git.company.com/team/frontend/ui.git (full path as repo)

# Env-based (ephemeral, CI/CD)
SKILLSHARE_GITLAB_HOSTS=git.company.com skillshare install git.company.com/team/frontend/ui

# Without config, .git workaround still works
skillshare install git.company.com/team/frontend/ui.git
```

---

## GitLab Subgroup URL Parsing

### The problem

The HTTPS URL parser (`gitHTTPSPattern`) used a regex that hardcoded exactly 2 path segments (`owner/repo`):

```regex
^https?://([^/]+)/([^/]+)/([^/]+?)(?:\.git)?(?:/(.+))?$
```

This worked for GitHub-style `owner/repo` URLs but failed for GitLab, which allows nested groups up to 20 levels deep. A URL like `onprem.gitlab.internal/org-group/subgroup-1/subgroup-2/project` would:
1. Capture `org-group` as owner, `subgroup-1` as repo
2. Treat `subgroup-2/project` as a monorepo subdir
3. Construct clone URL `https://host/org-group/subgroup-1.git` — which doesn't exist

### Solution

Replaced the rigid 2-segment regex with flexible path parsing. The new regex captures host and entire remaining path:

```regex
^https?://([^/]+)/(.+)$
```

The `parseGitHTTPS` function now uses explicit markers to determine where the repo path ends:

| Signal | Behavior | Example |
|--------|----------|---------|
| `.git` suffix | Strip suffix, no subdir | `group/sub/project.git` → clone `group/sub/project` |
| `.git/` in path | Split at `.git/` | `group/sub/project.git/skills/x` → clone `group/sub/project`, subdir `skills/x` |
| `/-/` in path | GitLab web URL | `group/sub/project/-/tree/main/x` → clone `group/sub/project`, subdir `x` |
| `/src/` on Bitbucket | Bitbucket web URL | `team/repo/src/main/x` → clone `team/repo`, subdir `x` |
| `gitlab_hosts` match | Entire path = repo | `team/frontend/ui` → clone `team/frontend/ui` |
| `"gitlab"` in hostname | Entire path = repo | `group/sub/project` → clone `group/sub/project` |
| Other hosts (no markers) | 2-segment owner/repo split | `org/repo/skills/x` → clone `org/repo`, subdir `skills/x` |

### Scope

Key files:
- `internal/install/source.go` — regex, `parseGitHTTPS` rewrite, `ParseOptions`/`ParseSourceWithOptions`, `isGitLabHost`
- `internal/install/source_test.go` — subgroup and `ParseSourceWithOptions` tests
- `internal/config/config.go` — `GitLabHosts` field, `EffectiveGitLabHosts()`, `normalizeGitLabHosts()`, `mergeGitLabHostsFromEnv()`
- `internal/config/project.go` — same for project config
- `internal/server/server.go` — `parseOpts()` with project-mode isolation
- `schemas/config.schema.json`, `schemas/project-config.schema.json` — JSON schemas
