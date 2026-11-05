# CLI E2E Runbook: Machine-local project registrations

Routine registrations preserve shared config, while shared declarations remain usable on other machines.

**Origin**: local project registry fix — prevent registration writes from republishing shared intent.

## Scope

- Shared config symlink survives localize, edit, remove, and re-add.
- Preview and repeated localize do not write registrations.
- Independent state directories see independent registrations.
- Normal project sync consumes local registrations.

## Environment

Run inside the devcontainer with a fresh ssenv HOME and XDG directories. `ss` is the current worktree binary. The runbook uses fixture files under the isolated HOME only. A preinitialized ssenv is fine: step 1 replaces only its fixture config. No network is needed.

## Steps

### 1. Set up a symlinked shared config and fixture skill

```bash
set -eu
mkdir -p "$HOME/source/demo" "$HOME/app" "$(dirname "$SKILLSHARE_CONFIG")"
printf '%s\n' '---' 'name: demo' 'description: Fixture skill' '---' '# Demo' > "$HOME/source/demo/SKILL.md"
printf 'source: %s/source\nmode: copy\ntargets: {}\nprojects:\n  %s/app: {name: shared, targets: [claude], skills: {}}\n' "$HOME" "$HOME" > "$HOME/shared.yaml"
if test -L "$SKILLSHARE_CONFIG" || test -f "$SKILLSHARE_CONFIG"; then rm "$SKILLSHARE_CONFIG"; fi
ln -s "$HOME/shared.yaml" "$SKILLSHARE_CONFIG"
cp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
ss project show shared --json
```

**Expected:**
- exit_code: 0
- jq: .project.origin == "shared"

### 2. Preview localization without changing the effective origin

```bash
set -eu
ss project localize shared --dry-run --json > "$HOME/preview.json"
jq -e '.project.origin == "local" and .applied == false' "$HOME/preview.json" >/dev/null
cmp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
ss project show shared --json
```

**Expected:**
- exit_code: 0
- jq: .project.origin == "shared"

### 3. Localize and repeat without rewriting shared intent

```bash
set -eu
ss project localize shared --json >/dev/null
ss project localize shared --json > "$HOME/repeat.json"
jq -e '.unchanged == true and .applied == false' "$HOME/repeat.json" >/dev/null
cmp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
test -L "$SKILLSHARE_CONFIG"
ss project show shared --json
```

**Expected:**
- exit_code: 0
- jq: .project.origin == "local" and (.project.originPath | endswith(".yaml"))

### 4. Edit then remove only on this machine

```bash
set -eu
ss project edit shared --name private --json >/dev/null
ss project remove private --yes --json >/dev/null
cmp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
ss project list --json
```

**Expected:**
- exit_code: 0
- jq: .projects | length == 0

### 5. A second machine still sees the shared declaration

```bash
set -eu
XDG_STATE_HOME="$HOME/second-machine-state" ss project show shared --json
```

**Expected:**
- exit_code: 0
- jq: .project.name == "shared" and .project.origin == "shared"

### 6. Re-add clears the exclusion and normal sync writes the skill

```bash
set -eu
ss project add "$HOME/app" --target claude --skills-mode copy --json >/dev/null
ss sync -g >/dev/null
cmp "$HOME/source/demo/SKILL.md" "$HOME/app/.claude/skills/demo/SKILL.md"
cmp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
test -L "$SKILLSHARE_CONFIG"
ss project show app --json
```

**Expected:**
- exit_code: 0
- jq: .project.origin == "local" and .project.skills.mode == "copy"

## Pass Criteria

All six steps pass. Shared bytes and the config symlink survive every registration-only operation; each state directory keeps its own view, and sync uses the local view.
