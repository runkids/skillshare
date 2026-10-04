# .skillfollow Runbook

Verifies the `.skillfollow` declaration file end to end in global mode: declared first-level links become visible to discovery, sync links them through logical paths, `.skillignore` still filters followed trees (including a tracked repo nested in a followed group), an unavailable entry pauses prune, followed repositories refuse forced updates, unfollowing prunes managed links, Git staging refuses an indexed declared link, and the dashboard API shares the same view. Proposal: `proposals/274-skillfollow.md`. Reference: `website/docs/reference/skillfollow.md`.

## Scope

- `internal/sourcewalk` — `FollowSet`, declaration parsing, entry states
- `internal/sync` — follow-aware discovery, `.skillignore` inside nested repos, ownership and prune pause
- `internal/install/followed_update.go` — followed repository update policy
- `internal/git/followed_links.go` — staging guard
- Commands affected: `list`, `status`, `doctor`, `sync`, `diff`, `update`, `commit`, `ui`

## Environment

- Devcontainer with `ss` binary (`make build`)
- mdproof isolation (`isolation: per-runbook`); `ss init -g --force --all-targets --no-git --no-skill` runs before each step, so the source directory persists across steps but `config.yaml` is reset
- External trees live under `/tmp/sf-ext`; Step 1 removes and recreates them
- Dashboard step uses port **49837**
- Not covered here: an unreadable declaration file (`chmod 000`). The container runs as root, which can read it anyway. `tests/integration/skillfollow_unreadable_test.go` covers it when run as a non-root user.

## Steps

### Step 1: Undeclared first-level links stay invisible

Builds two external trees and links them into the source without a declaration file: `group` (plain directory with skills `alpha`, `beta`, and a nested tracked repo `sub/_nested` whose `.skillignore` drops one skill) and `team` (tracked repo with `.skillignore` dropping `wip`). A third link `undeclared` is never declared.

```bash
ss extras remove rules --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/rules 2>/dev/null || true
rm -rf /tmp/sf-ext /tmp/sf-src-git
SOURCE=~/.config/skillshare/skills
rm -rf "$SOURCE/group" "$SOURCE/_team" "$SOURCE/_off" "$SOURCE/undeclared" "$SOURCE/.skillfollow" "$SOURCE/.git"
rm -rf ~/.claude/skills/*

mk() { mkdir -p "$(dirname "$1")"; printf -- "---\nname: %s\ndescription: Fixture %s\n---\n# %s\n" "$2" "$2" "$2" > "$1"; }
mk /tmp/sf-ext/group/alpha/SKILL.md alpha
mk /tmp/sf-ext/group/beta/SKILL.md beta
mk /tmp/sf-ext/group/sub/_nested/keep/SKILL.md keep
mk /tmp/sf-ext/group/sub/_nested/drop/SKILL.md drop
printf "drop\n" > /tmp/sf-ext/group/sub/_nested/.skillignore
git -C /tmp/sf-ext/group/sub/_nested init -q
mk /tmp/sf-ext/team/review/SKILL.md review
mk /tmp/sf-ext/team/wip/SKILL.md wip
printf "wip\n" > /tmp/sf-ext/team/.skillignore
git -C /tmp/sf-ext/team init -q
git -C /tmp/sf-ext/team -c user.email=e2e@example.com -c user.name=e2e add -A
git -C /tmp/sf-ext/team -c user.email=e2e@example.com -c user.name=e2e commit -q -m init
mk /tmp/sf-ext/hidden/secret/SKILL.md secret

ln -s /tmp/sf-ext/group "$SOURCE/group"
ln -s /tmp/sf-ext/team "$SOURCE/_team"
ln -s /tmp/sf-ext/hidden "$SOURCE/undeclared"

ss list --json -g | jq -r '[.[] | select(.disabled != true) | .name] | length'
ss status --json -g | jq -r '.source | has("skillfollow")'
```

Expected:
- 0
- false
- exit_code: 0

### Step 2: Declaring the links makes their skills visible

`.skillignore` inside `_team` and inside the nested `group/sub/_nested` repo both apply. `undeclared` stays invisible.

```bash
SOURCE=~/.config/skillshare/skills
printf "group\n_team\n" > "$SOURCE/.skillfollow"
ss list --json -g | jq -r '[.[] | select(.disabled != true) | .name] | sort | join(",")'
```

Expected:
- _team__review,group__alpha,group__beta,group__sub___nested__keep
- Not wip
- Not drop
- Not secret
- exit_code: 0

### Step 3: status reports the declaration

```bash
ss status --json -g | jq -c '.source.skillfollow | {active, local_active, entry_count, followed_count, skipped_count}'
ss status --json -g | jq -r '[.source.skillfollow.entries[] | "\(.name)=\(.state)"] | sort | join(",")'
```

Expected:
- {"active":true,"local_active":false,"entry_count":2,"followed_count":2,"skipped_count":0}
- _team=followed,group=followed
- exit_code: 0

### Step 4: doctor reports entry states and the undeclared link

```bash
ss doctor --json -g | jq -r '[.checks[] | select(.name == "skillfollow")] | length'
ss doctor --json -g | jq -r '[.checks[] | select(.name == "undeclared_source_links") | .message] | join(" ")'
```

Expected:
- regex: ^[1-9]
- undeclared
- exit_code: 0

### Step 5: sync links followed skills through logical paths

Links are created through the source (`.../skills/group/alpha`), not to `/tmp/sf-ext`, so a later unfollow can still attribute them.

```bash
ss sync -g --json | jq -r '[.details[] | select(.name == "claude") | .linked][0]'
ls ~/.claude/skills | sort | tr '\n' ','; echo
readlink ~/.claude/skills/group__alpha
```

Expected:
- 4
- _team__review,group__alpha,group__beta,group__sub___nested__keep,
- regex: /skills/group/alpha$
- Not sf-ext
- exit_code: 0

### Step 6: A missing entry pauses prune

`_off` is declared but its link target does not exist. A stale managed-looking link into `_off` would normally be pruned; while the entry is missing it is kept and sync says so.

```bash
SOURCE=~/.config/skillshare/skills
printf "group\n_team\n_off\n" > "$SOURCE/.skillfollow"
rm -f "$SOURCE/_off"
ln -s /tmp/sf-ext/offline "$SOURCE/_off"
rm -f ~/.claude/skills/_off__c
ln -s "$SOURCE/_off/c" ~/.claude/skills/_off__c
ss sync -g 2>&1
test -L ~/.claude/skills/_off__c && echo "STALE LINK KEPT"
```

Expected:
- prune paused; unavailable .skillfollow entry: _off
- STALE LINK KEPT
- exit_code: 0

### Step 7: status, doctor, and diff name the blocking entry

```bash
ss status --json -g | jq -r '.source.skillfollow.prune_paused[0]'
ss doctor --json -g | jq -r '[.checks[] | select(.name == "skillfollow_prune")] | length'
ss diff -g 2>&1 | grep "prune paused"
```

Expected:
- regex: ^prune paused: _off is missing; restore or fix .*/_off, or remove _off from \.skillfollow\[\.local\], to resume cleanup$
- 1
- prune paused; unavailable .skillfollow entry: _off (missing)
- exit_code: 0

### Step 8: Restoring the entry resumes prune

```bash
SOURCE=~/.config/skillshare/skills
mkdir -p /tmp/sf-ext/offline/d
printf -- "---\nname: d\ndescription: Fixture d\n---\n# d\n" > /tmp/sf-ext/offline/d/SKILL.md
ss sync -g 2>&1 | grep -c "prune paused" || true
test -L ~/.claude/skills/_off__c || echo "STALE LINK PRUNED"
test -L ~/.claude/skills/_off__d && echo "RESTORED ENTRY LINKED"
```

Expected:
- 0
- STALE LINK PRUNED
- RESTORED ENTRY LINKED
- exit_code: 0

### Step 9: A followed repository refuses --force updates

The checkout is the user's working copy. `--force` is refused even as a dry run, and HEAD does not move.

```bash
BEFORE=$(git -C /tmp/sf-ext/team rev-parse HEAD)
ss update _team -g --force --dry-run 2>&1; echo "exit=$?"
AFTER=$(git -C /tmp/sf-ext/team rev-parse HEAD)
test "$BEFORE" = "$AFTER" && echo "HEAD UNCHANGED"
```

Expected:
- followed repository update refused
- --force is not allowed for followed repository _team
- exit=1
- HEAD UNCHANGED
- exit_code: 0

### Step 10: Unfollowing prunes the managed links

Removing `_team` from the declaration makes its managed links orphans through the logical source; sync prunes them and `group` is untouched.

```bash
SOURCE=~/.config/skillshare/skills
printf "group\n_off\n" > "$SOURCE/.skillfollow"
ss sync -g --json | jq -r '[.details[] | select(.name == "claude") | .pruned][0]'
ls ~/.claude/skills | sort | tr '\n' ','; echo
```

Expected:
- 1
- _off__d,group__alpha,group__beta,group__sub___nested__keep,
- Not _team__review
- exit_code: 0

### Step 11: Git staging refuses an indexed declared link

With the source itself a Git repository and the `group` link added to the index, `commit` refuses and prints the exact untrack instruction.

```bash
SOURCE=~/.config/skillshare/skills
rm -rf "$SOURCE/.git"
git -C "$SOURCE" init -q
git -C "$SOURCE" add .skillfollow group
ss commit -g -m "e2e" 2>&1; echo "exit=$?"
```

Expected:
- followed links must be untracked and ignored before staging
- is indexed; run git rm --cached
- exit=1
- exit_code: 0

### Step 12: Dashboard API shares the follow-aware view

```bash
kill $(cat /tmp/sf-ui.pid 2>/dev/null) 2>/dev/null || true
rm -f /tmp/sf-ui.pid
/workspace/bin/skillshare ui --host 127.0.0.1 --port 49837 --no-open -g > /tmp/sf-ui.log 2>&1 &
echo $! > /tmp/sf-ui.pid
for i in $(seq 1 30); do curl -sf http://127.0.0.1:49837/api/health >/dev/null 2>&1 && break; sleep 0.5; done
curl -s http://127.0.0.1:49837/api/resources | jq -r '[.resources[] | select(.disabled != true) | .flatName] | sort | join(",")'
kill $(cat /tmp/sf-ui.pid) 2>/dev/null || true
rm -f /tmp/sf-ui.pid
```

The dashboard lists the nested repo's ignored skill as `disabled` rather than hiding it, so the filter above drops it.

Expected:
- _off__d,group__alpha,group__beta,group__sub___nested__keep
- Not _team__review
- Not drop
- exit_code: 0

### Step 13: Cleanup

```bash
SOURCE=~/.config/skillshare/skills
kill $(cat /tmp/sf-ui.pid 2>/dev/null) 2>/dev/null || true
rm -rf "$SOURCE/.git" "$SOURCE/.skillfollow" "$SOURCE/group" "$SOURCE/_team" "$SOURCE/_off" "$SOURCE/undeclared"
rm -rf /tmp/sf-ext /tmp/sf-ui.log /tmp/sf-ui.pid
rm -rf ~/.claude/skills/*
ss list --json -g | jq -r 'length'
```

Expected:
- 0
- exit_code: 0

## Pass Criteria

- All 13 steps pass
- Undeclared first-level links are invisible; declaring them in `.skillfollow` makes their skills visible under logical paths
- Repo-level `.skillignore` filters followed repos and a tracked repo nested inside a followed group
- Targets receive links through the logical source path, never to the external tree
- A missing entry pauses prune in `sync`, and `status --json`, `doctor --json`, and `diff` all name it; restoring the entry resumes prune
- A followed repository refuses `--force` updates and its HEAD does not move
- Removing a declaration prunes that entry's managed links
- `commit` refuses an indexed declared link and prints the untrack instruction
- The dashboard API lists followed skills under the same names as the CLI
