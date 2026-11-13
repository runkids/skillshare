# CLI E2E Runbook: Profiles

Verify named selections, safe switching, pending recovery and verified discard.

**Origin**: profiles #202 core — global and project skill selection.

## Scope

- Definition CRUD and explicit empty selection.
- Copy/merge switches, refusal, pending retry and desired/observed counts.
- JSON/text discard with snapshots and restore; source content stays intact.

## Environment

Run in the hosted Linux verification environment using the repository's devcontainer image and mdproof setup. Each step is self-contained and uses only synthetic files under an isolated HOME. Clear inherited agent overrides. No real accounts, network sources or user skills are used. This runbook was authored from source; no execution result is claimed.

## Steps

### 1. Global copy switch, preview, and pending recovery

```bash
set -eu
unset CODEX_HOME CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR
fixtureRoot=$(mktemp -d "$HOME/profiles-core.XXXXXX")
trap 'rm -rf "$fixtureRoot"' EXIT
export SKILLSHARE_CONFIG="$fixtureRoot/config.yaml"
mkdir -p "$fixtureRoot/source/alpha" "$fixtureRoot/source/beta"
printf '%s\n' '---' 'name: alpha' '---' '# alpha' > "$fixtureRoot/source/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' '---' '# beta' > "$fixtureRoot/source/beta/SKILL.md"
printf 'source: %s/source\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: %s/target\n' "$fixtureRoot" "$fixtureRoot" > "$SKILLSHARE_CONFIG"
ss profile create a --add-include alpha -g >/dev/null
ss profile create b --add-include beta -g >/dev/null
ss profile use a --dry-run --json -g | jq -e '.complete == true' >/dev/null
test ! -e "$fixtureRoot/target"
ss profile use a --no-sync -g >/dev/null
ss status --json -g | jq -e '.profile.pending == true and .profile.selected == 1' >/dev/null
ss sync --json -g >/dev/null
test -f "$fixtureRoot/target/alpha/SKILL.md"
test ! -e "$fixtureRoot/target/beta"
ss profile use b -g >/dev/null
ss profile use a -g >/dev/null
cmp "$fixtureRoot/source/alpha/SKILL.md" "$fixtureRoot/target/alpha/SKILL.md"
ss status --json -g
```

**Expected**:
- exit_code: 0
- jq: .profile.name == "a" and .profile.pending == false and .profile.selected == 1 and .profile.total == 2

### 2. Project merge selection and inactive definition deletion

```bash
set -eu
unset CODEX_HOME CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR
fixtureRoot=$(mktemp -d "$HOME/profiles-project.XXXXXX")
trap 'rm -rf "$fixtureRoot"' EXIT
mkdir -p "$fixtureRoot/.skillshare/skills/alpha"
printf '%s\n' '---' 'name: alpha' '---' '# alpha' > "$fixtureRoot/.skillshare/skills/alpha/SKILL.md"
printf 'targets:\n  - name: custom\n    skills:\n      path: %s/target\n      mode: merge\n' "$fixtureRoot" > "$fixtureRoot/.skillshare/config.yaml"
cd "$fixtureRoot"
ss profile create work --add-include alpha -p >/dev/null
ss profile use work -p >/dev/null
test -L "$fixtureRoot/target/alpha"
ss profile rename work renamed -p >/dev/null
if ss profile delete renamed -p >/dev/null 2>&1; then exit 1; fi
ss profile use --none -p >/dev/null
ss profile delete renamed -p >/dev/null
ss profile list --json -p
```

**Expected**:
- exit_code: 0
- jq: .profile.name == "" and (.profiles | length) == 0

### 3. Refuse an edited copy, snapshot in JSON, and restore

```bash
set -eu
unset CODEX_HOME CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR
fixtureRoot=$(mktemp -d "$HOME/profiles-discard.XXXXXX")
trap 'rm -rf "$fixtureRoot"' EXIT
export SKILLSHARE_CONFIG="$fixtureRoot/config.yaml"
mkdir -p "$fixtureRoot/source/alpha"
printf '%s\n' '---' 'name: alpha' '---' '# alpha' > "$fixtureRoot/source/alpha/SKILL.md"
printf 'source: %s/source\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: %s/target\n' "$fixtureRoot" "$fixtureRoot" > "$SKILLSHARE_CONFIG"
ss profile create a --add-include alpha -g >/dev/null
ss profile create empty -g >/dev/null
ss profile use a -g >/dev/null
printf 'authored\n' > "$fixtureRoot/target/alpha/SKILL.md"
if ss profile use empty --allow-empty -g >/dev/null 2>&1; then exit 1; fi
test "$(cat "$fixtureRoot/target/alpha/SKILL.md")" = authored
ss profile use empty --allow-empty --discard-edits --json -g > "$fixtureRoot/result.json"
snapshotPath=$(jq -r '.snapshot[0].path' "$fixtureRoot/result.json")
test "$(cat "$snapshotPath/alpha/SKILL.md")" = authored
restoreStamp=$(basename "$(dirname "$snapshotPath")")
ss restore custom --from "$restoreStamp" --force -g >/dev/null
test "$(cat "$fixtureRoot/target/alpha/SKILL.md")" = authored
cat "$fixtureRoot/result.json"
```

**Expected**:
- exit_code: 0
- jq: .complete == true and (.snapshot | length) == 1 and (.snapshot[0].restore | contains("restore")) and (.snapshot[0].retention | length) > 0

### 4. Explicit-empty definitions require confirmation

```bash
set -eu
unset CODEX_HOME CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR
fixtureRoot=$(mktemp -d "$HOME/profiles-empty.XXXXXX")
trap 'rm -rf "$fixtureRoot"' EXIT
export SKILLSHARE_CONFIG="$fixtureRoot/config.yaml"
mkdir -p "$fixtureRoot/source/alpha"
printf '# alpha\n' > "$fixtureRoot/source/alpha/SKILL.md"
printf 'source: %s/source\ntargets: {}\n' "$fixtureRoot" > "$SKILLSHARE_CONFIG"
ss profile create empty -g >/dev/null
if ss profile use empty -g >/dev/null 2>&1; then exit 1; fi
ss profile use empty --allow-empty --json -g
```

**Expected**:
- exit_code: 0
- jq: .complete == true and .profile.active == "empty" and .profile.pending == false

### 5. Assign, explain, diagnose and hint in both modes

Both modes select after add and deselect after remove, explain pruning by profile, refuse glob-blocked assignment atomically, report unmatched patterns once, and hint/warn without rewriting definitions. Source skills remain installed. Runtime acceptance of this authored step is unclaimed.

```bash
set -eu
unset CODEX_HOME CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR
fixtureRoot=$(mktemp -d "$HOME/profiles-membership.XXXXXX")
trap 'rm -rf "$fixtureRoot"' EXIT
for scopeFlag in -g -p; do
  case "$scopeFlag" in -g) scope=global ;; -p) scope=project ;; esac
  scopeRoot="$fixtureRoot/$scope"
  mkdir -p "$scopeRoot/source/alpha" "$scopeRoot/source/beta" "$scopeRoot/.skillshare"
  printf '# alpha\n' > "$scopeRoot/source/alpha/SKILL.md"
  printf '# beta\n' > "$scopeRoot/source/beta/SKILL.md"
  export SKILLSHARE_CONFIG="$scopeRoot/global.yaml"
  configPath="$SKILLSHARE_CONFIG"
  printf 'source: %s/source\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: %s/target\n' "$scopeRoot" "$scopeRoot" > "$configPath"
  if [ "$scopeFlag" = -p ]; then
    configPath="$scopeRoot/.skillshare/config.yaml"
    printf 'sources:\n  skills: %s/source\ntargets:\n  - name: custom\n    skills:\n      path: %s/target\n      mode: copy\n' "$scopeRoot" "$scopeRoot" > "$configPath"
  fi
  cd "$scopeRoot"
  ss profile create work --add-include alpha "$scopeFlag" >/dev/null
  ss profile use work --no-sync "$scopeFlag" >/dev/null
  ss profile add work beta --json "$scopeFlag" | jq -e '.changed and .skills[0].result == "added"' >/dev/null
  ss sync "$scopeFlag" >/dev/null
  test -f "$scopeRoot/target/beta/SKILL.md"
  ss profile remove work beta --json "$scopeFlag" | jq -e '.changed and .skills[0].result == "removed"' >/dev/null
  ss diff --json "$scopeFlag" | jq -e '[.targets[].items[] | select(.reason == "not in profile work")] | length == 1' >/dev/null
  ss sync "$scopeFlag" >/dev/null
  test ! -e "$scopeRoot/target/beta"
  ss profile edit work --add-include absent --add-exclude 'beta*' "$scopeFlag" >/dev/null
  cp "$configPath" "$scopeRoot/before.yaml"
  if ss profile add work beta "$scopeFlag" > "$scopeRoot/refused.txt" 2>&1; then exit 1; fi
  grep -F 'beta*' "$scopeRoot/refused.txt"
  cmp "$configPath" "$scopeRoot/before.yaml"
  ss sync --dry-run "$scopeFlag" > "$scopeRoot/sync.txt"
  test "$(grep -c 'Profile work: unmatched patterns' "$scopeRoot/sync.txt")" = 1
  ss doctor --json "$scopeFlag" | jq -e 'any(.checks[]; .name == "profile_unmatched")' >/dev/null
  ss new fresh -P none "$scopeFlag" | grep -F 'skillshare profile add work fresh'
  ss move alpha archive --dry-run --json "$scopeFlag" | jq -e 'any(.warnings[]; contains("profile \"work\""))' >/dev/null
  cmp "$configPath" "$scopeRoot/before.yaml"
done
```

**Expected**:
- exit_code: 0

### 6. Recover a project skills snapshot

Preview writes no target content; restore recovers the authored bytes without switching the active profile. The destination is replaced as a whole. Manual `backup -p` still requires `agents`; use `backup --list -p` to find profile snapshots. Restore of a recognized moved key returns bytes to its declared former folder, leaving the current folder untouched. Save edits into source before resync. The dry run uses the same force authorization as the proposed restore because managed metadata can remain in the destination.

```bash
set -eu
unset CODEX_HOME CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR
fixtureRoot=$(mktemp -d "$HOME/profiles-project-restore.XXXXXX")
trap 'rm -rf "$fixtureRoot"' EXIT
mkdir -p "$fixtureRoot/.skillshare/skills/alpha"
printf '%s\n' '---' 'name: alpha' '---' '# alpha' > "$fixtureRoot/.skillshare/skills/alpha/SKILL.md"
printf 'targets:\n  - name: custom\n    skills:\n      path: target\n      mode: copy\n' > "$fixtureRoot/.skillshare/config.yaml"
cd "$fixtureRoot"
ss profile create a --add-include alpha -p >/dev/null
ss profile create empty -p >/dev/null
ss profile use a -p >/dev/null
printf 'authored\n' > target/alpha/SKILL.md
ss profile use empty --allow-empty --discard-edits --json -p > result.json
snapshotPath=$(jq -r '.snapshot[0].path' result.json)
restoreStamp=$(basename "$(dirname "$snapshotPath")")
ss restore custom --from "$restoreStamp" -p --force --dry-run >/dev/null
test ! -e target/alpha
ss restore custom --from "$restoreStamp" -p --force >/dev/null
cmp "$snapshotPath/alpha/SKILL.md" target/alpha/SKILL.md
ss status --json -p | jq -e '.profile.name == "empty"' >/dev/null
```

**Expected**:
- exit_code: 0

### 7. Dashboard profile switch, consent and recovery (remote browser)

Use the isolated global copy fixture from step 1, or the project fixture from step 6. Start the dashboard in that same disposable environment; stop it before the fixture is removed.

```text
ss ui --no-open --host 0.0.0.0 --port 9100 -g
# For the project fixture instead, from its root:
# ss ui --no-open --host 0.0.0.0 --port 9100 -p
```

Manual browser checks: open the parent-forwarded remote preview and check these actions in global and project mode, with Clean/Playful light/dark looks and desktop/mobile navigation:
- No profiles and no state hides the picker. Otherwise it shows All skills or the active name; unusable choices are disabled with reasons. Mobile selection closes navigation before confirmation.
- Select a profile. Preview lists per-target added/removed/refreshed entries and unchanged symlink-mode targets. Cancel and compare saved config, state, manifest and target bytes with the pre-preview fixture.
- Edit a managed target copy, then switch away. Switch stays disabled until snapshot-first consent; the project label names the actual resolved configuration folder's backups directory, including visible `skillshare/` projects. Confirm, copy the returned restore command, run it in the fixture and compare the restored edited bytes.
- Choose an empty profile: confirm Allow an empty skill selection separately; inspect unmatched patterns. Change candidate while a preview is delayed: old results and consent must not transfer.
- Make one target fail. The incomplete result and per-target error remain visible after affected queries refresh. Fix the fixture, close the result and use the pending banner's Sync now skills preview to settle it.
- Remove the active definition from the fixture's shared config to simulate receiving a changed configuration (the CLI only deletes inactive definitions). Both unavailable repairs work: Choose profile and All skills. Skills outside a usable active profile stay visible with a muted tag in list, tree and tiles. Inspect a non-English locale, including blocked reasons and snapshot retention text.

This step requires actual parent-run browser, restore and byte-comparison evidence; authored instructions are not a PASS receipt.

### 8. Dashboard definitions and tree membership (manual browser)

Use a retained disposable fixture from step 7 in each scope; capture config, local profile state and target bytes before each action. This manual step has no automatic executor.

1. Open Profiles after Skills. With no definitions, create frontend with an explicit empty include; creation leaves activation and targets unchanged. Inspect a populated list with active counts and an unusable definition reason.
2. Select a folder plus an overlapping child in Skills tree, choose Add to profile, then frontend. Verify each resolved source flat name appears once in the saved definition; agents offer no action. Repeat Remove from profile in list/card view. A glob-blocked batch leaves config unchanged and the selection remains available for retry.
3. Edit patterns, inspect selected names and unmatched warnings, then save. Delay a preview and change the draft: an older response must not replace the newest list. Force a save error; confirm the draft remains and a retry submits it unchanged. Cancel resets the last saved form. Inspect empty includes with inert excludes and a pass-through description edit.
4. Switch to frontend through the header preview. Edit the active definition; verify no target changes until explicitly opening Sync now and applying the shared skills preview. Repeat with pending state and no target changes: explicit apply must settle pending.
5. Rename through its dialog, verify patterns retained and active state follows. Delete an inactive definition only after confirmation; active delete is disabled and a server refusal after an active race retains the editor. Source skills and targets stay intact.
6. Inspect these states in Clean/Playful light/dark, desktop and 390px mobile, and a non-English locale including skill/glob diagnostics. Save actual screenshots and byte comparisons; this authored step claims no browser PASS.

## Pass Criteria

All step exit codes and JSON assertions pass. The discarded bytes are present in the verified snapshot and restored. Refused switches keep edited bytes. Membership refusals leave definitions unchanged in both modes. Temporary fixture roots are removed by each step's trap. Unit/integration CI additionally owns snapshot failure, second-edit races, scope tree comparisons, lock ordering, legacy replay, live skill-server recovery and no-profile baseline comparison.
