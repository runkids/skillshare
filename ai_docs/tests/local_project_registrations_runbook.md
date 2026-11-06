# CLI E2E Runbook: Where project registrations are saved

Projects are saved in `config.yaml` by default, machine-local projects are opt-in, and each edit or removal stays in the layer that owns the project.

**Origin**: #792 — v0.29.0 made every project add machine-local; this restores `config.yaml` as the default, adds `--machine-local`, `projects_storage`, `share`, and `remove --shared`.

## Scope

- Shared config symlink survives localize, share, and every shared write.
- Preview and repeated localize/share do not write registrations.
- Removing a local override keeps the `config.yaml` version; `remove --shared` drops both.
- Independent state directories see independent registrations.
- Default add writes `config.yaml`; `--machine-local` and `projects_storage: machine-local` keep it local.
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
- jq: .project.origin == "shared" and .project.sharedCopy == false

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
- jq: .project.origin == "local" and .project.sharedCopy == true and (.project.originPath | endswith(".yaml"))

### 4. Edit the local copy, then remove only this machine's copy

```bash
set -eu
ss project edit shared --name private --json >/dev/null
cmp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
ss project remove private --yes --json > "$HOME/remove.json"
jq -e '.remaining.origin == "shared" and .remaining.name == "shared"' "$HOME/remove.json" >/dev/null
cmp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
ss project show shared --json
```

**Expected:**
- exit_code: 0
- jq: .project.origin == "shared" and .project.sharedCopy == false

### 5. A second machine still sees the shared declaration

```bash
set -eu
XDG_STATE_HOME="$HOME/second-machine-state" ss project show shared --json
```

**Expected:**
- exit_code: 0
- jq: .project.name == "shared" and .project.origin == "shared"

### 6. Add a machine-local project, sync it, then share it

```bash
set -eu
mkdir -p "$HOME/scratch"
ss project add "$HOME/scratch" --target claude --skills-mode copy --machine-local --json > "$HOME/add-local.json"
jq -e '.project.origin == "local"' "$HOME/add-local.json" >/dev/null
cmp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
ss sync -g >/dev/null
cmp "$HOME/source/demo/SKILL.md" "$HOME/scratch/.claude/skills/demo/SKILL.md"
ss project share scratch --dry-run --json > "$HOME/share-preview.json"
jq -e '.project.origin == "shared" and .applied == false' "$HOME/share-preview.json" >/dev/null
cmp "$HOME/shared.yaml" "$HOME/shared-before.yaml"
ss project share scratch --json >/dev/null
ss project share scratch --json > "$HOME/share-repeat.json"
jq -e '.unchanged == true' "$HOME/share-repeat.json" >/dev/null
grep -F "$HOME/scratch:" "$HOME/shared.yaml" >/dev/null
test -L "$SKILLSHARE_CONFIG"
XDG_STATE_HOME="$HOME/second-machine-state" ss project show scratch --json
```

**Expected:**
- exit_code: 0
- jq: .project.origin == "shared" and .project.skills.mode == "copy"

### 7. Default add writes config.yaml; the setting switches it

```bash
set -eu
mkdir -p "$HOME/other" "$HOME/third"
ss project add "$HOME/other" --target claude --json > "$HOME/add-default.json"
jq -e '.project.origin == "shared"' "$HOME/add-default.json" >/dev/null
grep -F "$HOME/other:" "$HOME/shared.yaml" >/dev/null
printf 'projects_storage: machine-local\n' >> "$HOME/shared.yaml"
cp "$HOME/shared.yaml" "$HOME/shared-setting.yaml"
ss project add "$HOME/third" --target claude --json > "$HOME/add-setting.json"
jq -e '.project.origin == "local"' "$HOME/add-setting.json" >/dev/null
cmp "$HOME/shared.yaml" "$HOME/shared-setting.yaml"
ss project list --json
```

**Expected:**
- exit_code: 0
- jq: [.projects[] | select(.name == "third") | .origin] == ["local"] and [.projects[] | select(.name == "other") | .origin] == ["shared"]

### 8. Remove --shared drops both copies of an overridden project

```bash
set -eu
ss project localize other --json >/dev/null
ss project remove other --shared --yes --json >/dev/null
if grep -F "$HOME/other:" "$HOME/shared.yaml"; then exit 1; fi
test -L "$SKILLSHARE_CONFIG"
ss project list --json
```

**Expected:**
- exit_code: 0
- jq: [.projects[].name] | index("other") == null

## Pass Criteria

All eight steps pass. Shared bytes stay unchanged for every local-only operation, the config symlink survives every shared write, each state directory keeps its own view, and sync uses the local view.
