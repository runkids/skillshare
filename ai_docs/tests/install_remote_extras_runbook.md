# CLI E2E Runbook: Install and recover remote extras

Validate explicit remote-extra installation, delivery, pins, edit protection,
removal and interrupted-publication repair in global and project mode.

**Origin**: federation slice 1, #434. Written from source; not executed by the coder.

## Scope

- Exact Git directory from a synthetic local `file://` repository, no network
- Full commit pins, explicit teammate restore, `--update`, and content edit refusal
- Add-target, sync, collect, edited retention and unedited deletion
- Real process kill after observing its publication journal, followed by repair
- No use of real user files, credentials, accounts or projects

## Environment

Run only when authorized inside the devcontainer, following
`skillshare-cli-e2e-test` and `.mdproof/lessons-learned.md`. Invoke mdproof directly
for this file: `mdproof --report json /workspace/ai_docs/tests/install_remote_extras_runbook.md`.
Use mdproof's own per-runbook isolation, without nesting it inside `ssenv`.
The coder's source-only authorization does not permit execution. The shared
`ai_docs/tests/mdproof.json` setup initializes mdproof's HOME and may run before
each step; the separate fixture HOME and unique root below remain independent.
Every step reloads its variables from the env file in mdproof's HOME. Require Git,
Python 3 and jq already available; install nothing for this runbook. All Expected
markers are printed only after the step's executable assertions succeed.

## Steps

### 1. Create a fixture repository and isolated configuration

```bash
set -euo pipefail
TASK_ROOT=$(mktemp -d /tmp/skillshare-remote-extras.XXXXXX)
printf 'export TASK_ROOT=%q\n' "$TASK_ROOT" > "$HOME/.remote-extras.env"
cat >> "$HOME/.remote-extras.env" <<'SH'
export REMOTE="file://$TASK_ROOT/remote.git//rules"
export TASK_SS_BIN=/workspace/bin/skillshare
test -x "$TASK_SS_BIN"
ss_fixture() {
  env HOME="$TASK_ROOT/home" XDG_CONFIG_HOME="$TASK_ROOT/home/.config" \
    XDG_STATE_HOME="$TASK_ROOT/home/.local/state" \
    SKILLSHARE_CONFIG="$TASK_ROOT/home/.config/skillshare/config.yaml" "$TASK_SS_BIN" "$@"
}
SH
. "$HOME/.remote-extras.env"
mkdir -p "$TASK_ROOT/home/.claude" "$TASK_ROOT/project" "$TASK_ROOT/seed/rules"
git init -q "$TASK_ROOT/seed"
git init -q "$TASK_ROOT/project"
git -C "$TASK_ROOT/seed" config user.name Fixture
git -C "$TASK_ROOT/seed" config user.email fixture@example.invalid
printf '# Team rule v1\n' > "$TASK_ROOT/seed/rules/team.md"
git -C "$TASK_ROOT/seed" add .
git -C "$TASK_ROOT/seed" commit -qm fixture-v1
git -C "$TASK_ROOT/seed" branch -M main
git clone -q --bare "$TASK_ROOT/seed" "$TASK_ROOT/remote.git"
ss_fixture init -g --no-copy --no-git --no-skill --targets claude
(cd "$TASK_ROOT/project" && ss_fixture init -p --targets claude)
test -f "$TASK_ROOT/home/.config/skillshare/config.yaml"
test -f "$TASK_ROOT/project/.skillshare/config.yaml"
test -d "$TASK_ROOT/remote.git"
printf 'REMOTE_EXTRAS_FIXTURE_READY\n'
```

**Expected:**
- exit_code: 0
- REMOTE_EXTRAS_FIXTURE_READY

The repository and both configs live under `TASK_ROOT`; the env file belongs to
mdproof's isolated HOME.

### 2. Install, add a target and sync in both modes

```bash
set -euo pipefail
. "$HOME/.remote-extras.env"
for mode in g p; do
  cd "$TASK_ROOT/project"
  ss_fixture install "$REMOTE" --kind extra --name team-rules --branch main "-$mode" --json > "$TASK_ROOT/install-$mode.json"
  jq -e '.kind == "extra" and (.commit | test("^[0-9a-f]{40}$")) and .action == "installed"' "$TASK_ROOT/install-$mode.json"
  ss_fixture extras list "-$mode" --json | jq -e '.[] | select(.name == "team-rules") | .remote != "" and .branch == "main" and .targets == []'
  ss_fixture extras team-rules --add-target "$TASK_ROOT/target-$mode" --mode copy "-$mode"
  ss_fixture sync extras team-rules "-$mode"
  cmp "$TASK_ROOT/seed/rules/team.md" "$TASK_ROOT/target-$mode/team.md"
  ss_fixture check extras team-rules "-$mode" --json | jq -e '.[] | .state == "intact" and .message == "remote revision not checked"'
  source_dir=$(ss_fixture extras list "-$mode" --json | jq -r '.[] | select(.name == "team-rules") | .source_dir')
  jq -e --arg remote "$REMOTE" '.entries["team-rules"] | .kind == "extra" and .source == $remote and (.commit | test("^[0-9a-f]{40}$")) and (.file_hashes["team.md"] | length > 0)' "$(dirname "$source_dir")/.metadata.json"
done
jq -e '.version == 1 and (.extras["team-rules"].commit | length == 40)' "$TASK_ROOT/project/.skillshare/skills.lock.json"
git -C "$TASK_ROOT/project" check-ignore .skillshare/extras/team-rules/ .skillshare/extras/.metadata.json .skillshare/extras/.skillshare-staging/
printf 'REMOTE_EXTRAS_DELIVERY_AND_RECORDS_OK\n'
```

**Expected:**
- exit_code: 0
- REMOTE_EXTRAS_DELIVERY_AND_RECORDS_OK

The inline JSON, file comparison and Git assertions verify zero-target admission,
delivery, metadata hashes, version-1 pins and ignored fetched content.

### 3. Advance the hub, restore the pin and explicitly update

```bash
set -euo pipefail
. "$HOME/.remote-extras.env"
printf '# Team rule v2\n' > "$TASK_ROOT/seed/rules/team.md"
git -C "$TASK_ROOT/seed" commit -qam fixture-v2
git -C "$TASK_ROOT/seed" push -q "$TASK_ROOT/remote.git" main
for mode in g p; do
  cd "$TASK_ROOT/project"
  # Simulate a fresh machine's missing fetched copy; retain records and pin.
  source_dir=$(ss_fixture extras list "-$mode" --json | jq -r '.[] | select(.name == "team-rules") | .source_dir')
  case "$source_dir" in "$TASK_ROOT"/*/team-rules) ;; *) exit 1 ;; esac
  rm -r -- "$source_dir"
  ss_fixture sync extras team-rules "-$mode" --json > "$TASK_ROOT/missing-$mode.json"
  grep -q 'install .*--kind extra --name team-rules' "$TASK_ROOT/missing-$mode.json"
  ss_fixture install "$REMOTE" --kind extra --name team-rules --branch main "-$mode" --json > "$TASK_ROOT/restored-$mode.json"
  test "$(jq -r .commit "$TASK_ROOT/restored-$mode.json")" = "$(jq -r .commit "$TASK_ROOT/install-$mode.json")"
  ss_fixture install "$REMOTE" --kind extra --name team-rules --branch main --update "-$mode" --json > "$TASK_ROOT/updated-$mode.json"
  test "$(jq -r .commit "$TASK_ROOT/updated-$mode.json")" != "$(jq -r .commit "$TASK_ROOT/install-$mode.json")"
done
printf 'REMOTE_EXTRAS_PIN_RESTORE_AND_UPDATE_OK\n'
```

**Expected:**
- exit_code: 0
- REMOTE_EXTRAS_PIN_RESTORE_AND_UPDATE_OK

The assertions compare the original, restored and updated commits in both modes;
bare install is deliberately not used to restore extras.

### 4. Collect, refuse edited reinstall, force and remove

```bash
set -euo pipefail
. "$HOME/.remote-extras.env"
for mode in g p; do
  cd "$TASK_ROOT/project"
  printf '# Local addition\n' > "$TASK_ROOT/target-$mode/local.md"
  ss_fixture extras collect team-rules --from "$TASK_ROOT/target-$mode" "-$mode"
  ss_fixture check extras team-rules "-$mode" --json | jq -e '.[] | .state == "modified"'
  if ss_fixture install "$REMOTE" --kind extra --name team-rules --branch main "-$mode" --json > "$TASK_ROOT/refused-$mode.json"; then exit 1; fi
  jq -e '.error | contains("--force")' "$TASK_ROOT/refused-$mode.json"
  ss_fixture install "$REMOTE" --kind extra --name team-rules --branch main --force "-$mode"
  source_dir=$(ss_fixture extras list "-$mode" --json | jq -r '.[] | select(.name == "team-rules") | .source_dir')
  printf '# Keep my work\n' >> "$source_dir/team.md"
  ss_fixture extras remove team-rules --force "-$mode"
  test -f "$source_dir/team.md"
  grep -q 'Keep my work' "$source_dir/team.md"
  # Re-adopt only this known synthetic retained path, explicitly discarding edits.
  ss_fixture install "$REMOTE" --kind extra --name team-rules --branch main --force "-$mode"
  ss_fixture extras remove team-rules --force "-$mode"
  test ! -e "$source_dir"
  test -f "$TASK_ROOT/target-$mode/team.md"
done
printf 'REMOTE_EXTRAS_EDIT_PROTECTION_AND_REMOVAL_OK\n'
```

**Expected:**
- exit_code: 0
- REMOTE_EXTRAS_EDIT_PROTECTION_AND_REMOVAL_OK

The assertions verify collection, refusal without explicit force, edited-source
retention, unedited deletion and retained copy outputs. A failed removal restores
records/source but does not relink already-cleaned targets; use `sync extras`.

### 5. Observe a real kill and repair in both modes

Add enough benign fixture files to leave an observable incoming-copy interval.
The watcher kills only the subprocess it starts, after observing the marker.
A missed interval is a failed observation, not a successful crash test.

```bash
set -euo pipefail
. "$HOME/.remote-extras.env"
python3 - <<'PY'
import os
from pathlib import Path
root = Path(os.environ['TASK_ROOT']) / 'seed/rules/bulk'
root.mkdir()
for i in range(6000):
    (root / f'{i:05d}.txt').write_text('fixture\n' * 128)
PY
git -C "$TASK_ROOT/seed" add .
git -C "$TASK_ROOT/seed" commit -qm crash-fixture
git -C "$TASK_ROOT/seed" push -q "$TASK_ROOT/remote.git" main
python3 - <<'PY'
import json, os, subprocess, time
from pathlib import Path
root = Path(os.environ['TASK_ROOT'])
env = dict(os.environ, HOME=str(root/'home'), XDG_CONFIG_HOME=str(root/'home/.config'), XDG_STATE_HOME=str(root/'home/.local/state'), SKILLSHARE_CONFIG=str(root/'home/.config/skillshare/config.yaml'))
ss = os.environ['TASK_SS_BIN']
assert Path(ss).is_file(), 'built candidate must exist'
for mode, extras in [('g', root/'home/.config/skillshare/extras'), ('p', root/'project/.skillshare/extras')]:
    marker = extras/'.skillshare-staging/publishing-crash-rules.json'
    command = [ss, 'install', os.environ['REMOTE'], '--kind', 'extra', '--name', 'crash-rules', '--branch', 'main', '--skip-audit', '-'+mode]
    with (root/f'kill-{mode}.log').open('w') as output:
        proc = subprocess.Popen(command, cwd=root/'project', env=env, stdout=output, stderr=output)
        deadline = time.monotonic() + 60
        try:
            while not marker.exists():
                assert proc.poll() is None and time.monotonic() < deadline, 'publication interval missed; no crash verification'
                time.sleep(0.001)
            proc.kill()
            proc.wait(timeout=10)
        finally:
            if proc.poll() is None:
                proc.kill(); proc.wait(timeout=10)
    assert marker.exists(), 'marker must survive observed kill'
    checked = subprocess.run([ss, 'check', 'extras', 'crash-rules', '-'+mode, '--json', '--exit-code'], cwd=root/'project', env=env, capture_output=True, text=True)
    assert checked.returncode == 1, (checked.returncode, checked.stdout, checked.stderr)
    rows = json.loads(checked.stdout)
    assert len(rows) == 1 and rows[0]['state'] == 'interrupted', rows
    assert rows[0]['message'] == 'remote revision not checked', rows
    # Same command audits the journaled commit before proceeding; no skip here.
    subprocess.run([arg for arg in command if arg != '--skip-audit'], cwd=root/'project', env=env, check=True)
    assert not marker.exists(), 'repair did not finish'
    assert (extras/'crash-rules/team.md').read_bytes() == (root/'seed/rules/team.md').read_bytes()
    meta = json.loads((extras/'.metadata.json').read_text())['entries']['crash-rules']
    assert meta['source'] == os.environ['REMOTE'] and meta['branch'] == 'main'
    assert len(meta['commit']) == 40 and 'team.md' in meta['file_hashes']
    listed = subprocess.run([ss, 'extras', 'list', '-'+mode, '--json'], cwd=root/'project', env=env, capture_output=True, text=True, check=True)
    entry = next(e for e in json.loads(listed.stdout) if e['name'] == 'crash-rules')
    assert entry['remote'] == meta['source'] and entry['branch'] == meta['branch'] and entry['commit'] == meta['commit'] and entry['state'] == 'intact', entry
    if mode == 'p':
        pin = json.loads((root/'project/.skillshare/skills.lock.json').read_text())['extras']['crash-rules']
        assert pin['source'] == meta['source'] and pin['commit'] == meta['commit'], pin
print('REMOTE_EXTRAS_OBSERVED_KILL_AND_REPAIR_OK')
PY
```

**Expected:**
- exit_code: 0
- REMOTE_EXTRAS_OBSERVED_KILL_AND_REPAIR_OK

The check subprocess intentionally exits 1 under `--exit-code` for interrupted
state; the surrounding step succeeds only after asserting that exit and JSON.
Repair must remove the marker and leave matching content, config, metadata and
project pin. Audit failure, unavailable commits and later edits are covered by
deterministic tests, not this polling interval; this step does not hit every death
boundary.

### 6. Cleanup only the resolved fixture

```bash
set -euo pipefail
. "$HOME/.remote-extras.env"
case "$TASK_ROOT" in /tmp/skillshare-remote-extras.??????) ;; *) exit 1 ;; esac
test -d "$TASK_ROOT/seed/.git"
cd "$HOME"
rm -rf -- "$TASK_ROOT"
rm -- "$HOME/.remote-extras.env"
test ! -e "$TASK_ROOT"
test ! -e "$HOME/.remote-extras.env"
printf 'REMOTE_EXTRAS_FIXTURE_REMOVED\n'
```

**Expected:**
- exit_code: 0
- REMOTE_EXTRAS_FIXTURE_REMOVED

mdproof owns the enclosing isolated HOME cleanup. No daemon was started.

## Pass Criteria

- Every assertion above succeeds in both modes, with exact candidate commit recorded
- Real kill is observed for each mode; missed observation is reported as incomplete
- Metadata, config and project pin agree after repair; no stale staging paths
- CI passes the focused fault/death, link, credential and rollback source tests
- Record commands, result artifacts and limitations; do not infer a pass from source
