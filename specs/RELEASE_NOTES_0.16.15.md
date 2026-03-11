# skillshare v0.16.15 Release Notes

Release date: 2026-03-11

## TL;DR

v0.16.15 fixes **GitLab subgroup URL parsing** (Issue #72) — the only user-facing change in this release:

1. **GitLab nested subgroups** — URLs like `group/subgroup/project` are now treated as the full repo path instead of being split at the 2nd segment
2. **`.git` boundary** — use `.git` suffix to explicitly mark where the repo path ends and the subdir begins
3. **TrackName** — `--track` mode produces correct hyphenated names for subgroup paths (e.g., `group-subgroup-project`)

No breaking changes. Drop-in upgrade from v0.16.14.

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
| None of above | Entire path = repo | `group/sub/project` → clone `group/sub/project` |

### Design decisions

- **`.git` as the canonical boundary** — without `.git`, it's genuinely ambiguous whether extra segments are subgroups or subdirs. `.git` is the standard git URL suffix and resolves this ambiguity cleanly. This is a behavioral change: `gitlab.com/user/repo/path/skill` previously cloned `user/repo` with subdir `path/skill`; now it clones the entire path as a repo. Users who need subdirs must add `.git` explicitly.
- **Platform-specific markers preserved** — `/-/` (GitLab web) and `/src/` (Bitbucket web) are checked before the fallback, maintaining correct behavior for browser-copied URLs.
- **`url.Parse` in TrackName** — replaced manual `strings.Split` with `url.Parse` for the HTTPS fallback, correctly extracting the full path for any depth of nesting.
- **SSH subgroups** — the existing SSH regex (`git@host:owner/repo.git`) already captures multi-segment repo names via `.+?` non-greedy match. Only the `TrackName` formatting needed updating (`/` → `-` in the repo portion).

### Scope

2 files changed:
- `internal/install/source.go` — regex, `parseGitHTTPS` rewrite, `TrackName` SSH and HTTPS fallback
- `internal/install/source_test.go` — 3 existing tests updated, 8 new tests added (6 subgroup + 2 TrackName)
