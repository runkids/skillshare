# CLI E2E Runbook: update --merge

Validates that `update --merge` 3-way merges upstream changes into a skill or
agent edited since install, and that a conflict changes nothing (issue #890).

**Scenario**: A skill and an agent are installed from a local bare repo, edited
locally, and changed upstream. A plain `update` skips both; `--merge` keeps the
local edits and takes upstream's; a second upstream change to the same line
conflicts and leaves the local files alone.

## Scope

- `check` reports the edited skill as `modified` with `update available upstream` and hints `--merge`
- Plain `update` skips the edited skill and suggests `--merge`
- `update --merge --json` reports the skill as `merged` and keeps both edits
- The merged skill is still `modified`
- `--merge` with `--force` is rejected
- A conflicting upstream change fails, names the file, and leaves the skill unchanged
- `check agents --json` flags the edited agent with `upstreamChanged`
- `update agents <name> --merge --json` merges the agent

## Environment

Run inside devcontainer with `ssenv` HOME isolation.
Offline test: uses a `file://` bare repo, no network required.
Setup hook handles `ss init -g`.

## Steps

### 1. Create a bare remote with a skill and an agent

```bash
REMOTE=~/remote-merge.git
WORK=~/work-merge
rm -rf "$REMOTE" "$WORK"
git init --bare "$REMOTE"
git clone "$REMOTE" "$WORK"
cd "$WORK"
git config user.email "test@test.com"
git config user.name "test"
mkdir -p skills/merge-skill agents
printf -- '---\nname: merge-skill\n---\nfirst\nmiddle\nlast\n' > skills/merge-skill/SKILL.md
printf -- '# Reviewer\nfirst\nmiddle\nlast\n' > agents/reviewer.md
git add -A
git commit -m "v1"
git push origin HEAD
echo "=== Remote ready ==="
```

Expected:
- exit_code: 0
- === Remote ready ===

### 2. Install both, edit locally, change upstream

```bash
REMOTE=~/remote-merge.git
WORK=~/work-merge
ss install "file://$REMOTE//skills/merge-skill" -g --skip-audit >/dev/null
ss install "file://$REMOTE//agents/reviewer.md" -g --skip-audit >/dev/null
printf -- '---\nname: merge-skill\n---\nmine\nmiddle\nlast\n' > ~/.config/skillshare/skills/merge-skill/SKILL.md
printf -- '# Reviewer\nmine\nmiddle\nlast\n' > ~/.config/skillshare/agents/reviewer.md
cd "$WORK"
printf -- '---\nname: merge-skill\n---\nfirst\nmiddle\ntheirs\n' > skills/merge-skill/SKILL.md
printf -- '# Reviewer\nfirst\nmiddle\ntheirs\n' > agents/reviewer.md
git commit -qam "v2"
git push -q origin HEAD
echo "=== Edited on both sides ==="
```

Expected:
- exit_code: 0
- === Edited on both sides ===

### 3. check reports the skill as modified with an upstream update

```bash
ss check merge-skill -g
```

Expected:
- exit_code: 0
- update available upstream
- skillshare update merge-skill --merge

### 4. Plain update skips the edited skill

```bash
ss update merge-skill -g --skip-audit
echo "exit=$?"
cat ~/.config/skillshare/skills/merge-skill/SKILL.md
```

Expected:
- exit=1
- local changes since install
- update merge-skill --merge
- mine
- Not theirs

### 5. --merge with --force is rejected

```bash
ss update merge-skill -g --merge --force
```

Expected:
- exit_code: 1
- --merge cannot be used with --force

### 6. update --merge keeps both edits

```bash
ss update merge-skill -g --merge --skip-audit --json
```

Expected:
- exit_code: 0
- jq: .updated == 1
- jq: .items[0].status == "merged"

### 7. The merged skill has both edits and is still modified

```bash
cat ~/.config/skillshare/skills/merge-skill/SKILL.md
ss check merge-skill -g --json
```

Expected:
- exit_code: 0
- mine
- theirs
- "status": "modified"

### 8. A conflicting upstream change leaves the skill unchanged

```bash
WORK=~/work-merge
cd "$WORK"
printf -- '---\nname: merge-skill\n---\nupstream v3\nmiddle\ntheirs\n' > skills/merge-skill/SKILL.md
git commit -qam "v3"
git push -q origin HEAD
ss update merge-skill -g --merge --skip-audit
echo "exit=$?"
cat ~/.config/skillshare/skills/merge-skill/SKILL.md
```

Expected:
- exit=1
- conflicting changes
- SKILL.md
- Not upstream v3
- Not <<<<<<<

### 9. check agents flags the edited agent as mergeable

```bash
ss check agents -g --json
```

Expected:
- exit_code: 0
- jq: .[] | select(.name == "reviewer") | .upstreamChanged == true

### 10. update agents --merge merges the agent

```bash
ss update agents reviewer -g --merge --skip-audit --json >/dev/null
cat ~/.config/skillshare/agents/reviewer.md
```

Expected:
- exit_code: 0
- mine
- theirs

## Pass Criteria

- Steps 3–4: the edited skill is reported and skipped, with a `--merge` hint
- Step 6: `--merge` reports `merged` and counts as updated
- Step 7: both edits are present and the skill stays `modified`
- Step 8: a conflict exits non-zero, names `SKILL.md`, and writes no conflict markers
- Steps 9–10: the edited agent is flagged and merged
