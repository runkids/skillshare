# CLI E2E Runbook: push --pull

Validates the two-way `skillshare push --pull` round trip between two machines sharing one git remote.

**Origin**: `push --pull` — one explicit command to commit, merge the remote, push, and sync targets.

## Scope

- First run against an empty remote pushes and sets upstream.
- Changes from another machine are merged, pushed back, and synced to targets.
- A content conflict pushes nothing and keeps the local commit.
- `--dry-run` changes neither the local repository nor the remote.
- `commit --pull` is rejected instead of silently ignored.

## Environment

Run inside the devcontainer with `ssenv` isolation. The bare remote and the "other machine" clone live under `~/pp` inside the isolated HOME. Steps resolve the skills source and the claude target from `ss status --json`, because mdproof isolates `HOME` separately from the ssenv config paths.

## Steps

### 1. Setup: connect the source to an empty bare remote

```bash
rm -rf ~/pp && mkdir -p ~/pp
git init -q --bare ~/pp/remote.git
ss init -g --remote ~/pp/remote.git
git -C "$(ss status --json | jq -r '.source.path')" remote get-url origin
```

**Expected**:
- exit_code: 0
- pp/remote.git

### 2. First push --pull against the empty remote

```bash
SRC=$(ss status --json | jq -r '.source.path')
mkdir -p $SRC/local-skill && printf -- '---\nname: local-skill\n---\n# Local\n' > $SRC/local-skill/SKILL.md
ss push --pull -m "Add local skill"
git -C $SRC rev-parse --abbrev-ref --symbolic-full-name '@{u}'
git -C ~/pp/remote.git ls-tree -r --name-only HEAD
```

**Expected**:
- exit_code: 0
- Push complete
- local-skill/SKILL.md
- regex: origin/\S+

### 3. Merge another machine's change, push, and sync

```bash
SRC=$(ss status --json | jq -r '.source.path')
BRANCH=$(git -C $SRC rev-parse --abbrev-ref HEAD)
rm -rf ~/pp/other && git clone -q ~/pp/remote.git ~/pp/other
git -C ~/pp/other config user.email other@example.com
git -C ~/pp/other config user.name other
mkdir -p ~/pp/other/remote-skill && printf -- '---\nname: remote-skill\n---\n# Remote\n' > ~/pp/other/remote-skill/SKILL.md
git -C ~/pp/other add -A && git -C ~/pp/other commit -qm "other machine" && git -C ~/pp/other push -q origin HEAD:$BRANCH
mkdir -p $SRC/second-skill && printf -- '---\nname: second-skill\n---\n# Second\n' > $SRC/second-skill/SKILL.md
ss push --pull -m "Add second skill"
git -C ~/pp/remote.git ls-tree -r --name-only HEAD
CLAUDE=$(ss status --json | jq -r '.targets[] | select(.name=="claude") | .path')
test -e "$CLAUDE/remote-skill/SKILL.md" && echo REMOTE_SKILL_SYNCED
```

**Expected**:
- exit_code: 0
- Push complete
- remote-skill/SKILL.md
- second-skill/SKILL.md
- REMOTE_SKILL_SYNCED

### 4. Dry-run changes nothing

```bash
SRC=$(ss status --json | jq -r '.source.path')
printf '\nedit\n' >> $SRC/local-skill/SKILL.md
HEAD_BEFORE=$(git -C $SRC rev-parse HEAD)
REMOTE_BEFORE=$(git -C ~/pp/remote.git rev-parse HEAD)
ss push --pull --dry-run
[ "$(git -C $SRC rev-parse HEAD)" = "$HEAD_BEFORE" ] && [ "$(git -C ~/pp/remote.git rev-parse HEAD)" = "$REMOTE_BEFORE" ] && echo NOTHING_CHANGED
git -C $SRC checkout -q -- local-skill/SKILL.md
```

**Expected**:
- exit_code: 0
- Would pull from remote (merge)
- Would sync targets
- NOTHING_CHANGED

### 5. A conflict pushes nothing and keeps the local commit

```bash
SRC=$(ss status --json | jq -r '.source.path')
BRANCH=$(git -C $SRC rev-parse --abbrev-ref HEAD)
git -C ~/pp/other pull -q --no-rebase origin $BRANCH
printf -- '---\nname: local-skill\n---\n# Remote edit\n' > ~/pp/other/local-skill/SKILL.md
git -C ~/pp/other commit -qam "remote edit" && git -C ~/pp/other push -q origin HEAD:$BRANCH
REMOTE_BEFORE=$(git -C ~/pp/remote.git rev-parse HEAD)
printf -- '---\nname: local-skill\n---\n# Local edit\n' > $SRC/local-skill/SKILL.md
ss push --pull -m "Local edit" && echo UNEXPECTED_SUCCESS || echo FAILED_AS_EXPECTED
[ "$(git -C ~/pp/remote.git rev-parse HEAD)" = "$REMOTE_BEFORE" ] && echo REMOTE_UNCHANGED
git -C $SRC log -1 --format=%s
```

**Expected**:
- exit_code: 0
- FAILED_AS_EXPECTED
- REMOTE_UNCHANGED
- Local edit

### 6. commit rejects --pull

```bash
ss commit --pull && echo UNEXPECTED_SUCCESS || echo REJECTED
```

**Expected**:
- exit_code: 0
- REJECTED
- --pull is only supported by push

## Pass Criteria

- All steps marked PASS.
- Step 5 leaves the remote untouched and the local "Local edit" commit in place.
