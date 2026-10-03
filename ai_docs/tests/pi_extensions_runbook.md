# CLI E2E Runbook: Pi Target Extensions Tab

Validates the dashboard's Pi Extensions API against a real Pi 0.99.2 on an isolated HOME.
Pi 1.0.0 is covered by the version matrix (step 12).

**Origin**: #342 — the tab edits Pi's `settings.json`, so the preview/apply guards, lock
handling and read-only gates need a check against Pi itself, beyond the Go and Vitest tests.

## Scope

- GET returns the global, account and project views without raw settings values
- Preview writes nothing; apply changes only the touched `extensions` list
- Pi's settings lock refuses an apply and is left in place; a lock Skillshare holds stays
  fresh for Pi's own proper-lockfile past its 10s stale age
- An apply against an old revision is refused as stale
- A fork account is read-only
- A project saves only its own `.pi/settings.json`, as `pi config` writes it; the global
  settings and `trust.json` stay byte for byte, and Pi applies the result only when it
  trusts the project
- A non-Pi target answers 404
- The view's selection matches Pi's own resolver, and every extension Pi resolves is
  listed (no extension is imported)
- Apply writes an oplog entry and a backup

## Environment

Run inside the devcontainer, with the Pi CLI at `/opt/agent-clis/bin/pi` (0.99.2). The
fixture (`scripts/pi/extensions-e2e-fixture.sh`) writes only under `/tmp/pi-ext-e2e`; its
extension files throw if imported. Do not run the server from `/workspace`.

```bash
CONTAINER=$(docker compose -f .devcontainer/docker-compose.yml ps -q skillshare-devcontainer)
docker exec -it "$CONTAINER" bash
```

## Steps

### 1. Setup: build, create the fixture and start the server

```bash
cd /workspace && make build
rm -rf -- /tmp/pi-ext-e2e
scripts/pi/extensions-e2e-fixture.sh
source /tmp/pi-ext-e2e/env.sh
pi --version
cd /tmp/pi-ext-e2e && /workspace/bin/skillshare ui --no-open --port 49423 > /tmp/pi-ext-e2e/ui.log 2>&1 &
sleep 2
API=http://127.0.0.1:49423/api
S=$PI_CODING_AGENT_DIR/settings.json
```

**Expected**: `pi --version` prints `0.99.2`; the server answers on port 49423.

### 2. Global view without raw values

```bash
curl -s $API/targets/pi/pi-extensions > /tmp/pi-ext-e2e/view.json
jq -c '{scope, editable, version, n: [.packages[].rows[]] | length}' /tmp/pi-ext-e2e/view.json
jq -c '.packages[0] | {otherKeys, rows: [.rows[] | {path, selection, editable}]}' /tmp/pi-ext-e2e/view.json
grep -c '"dark"' /tmp/pi-ext-e2e/view.json || true
```

**Expected**: `scope` is `global`, `editable` true, `version` `0.99.2` and 6 rows (the git
package is not installed, so it has none). The local package lists `extensions/notify.ts` as
skipped by its exact rule and `extensions/slow-lint.ts` as skipped by the `!` glob; both stay
editable, because an exact rule wins over a glob. `otherKeys` lists the names `autoUpdate`,
`prompts` and `themes` only. The `"dark"` theme value appears 0 times.

### 3. Preview writes nothing

```bash
CH='[{"index":0,"source":"'$PKG'","path":"extensions/git-status.ts","action":"exclude"}]'
cp $S /tmp/pi-ext-e2e/before.json
curl -s -X POST $API/targets/pi/pi-extensions/preview -d '{"changes":'"$CH"'}' > /tmp/pi-ext-e2e/plan.json
jq -c '.entries[0] | {before, after, keptKeys}' /tmp/pi-ext-e2e/plan.json
REV=$(jq -r .revision /tmp/pi-ext-e2e/plan.json)
test "$REV" != "$(jq -r .revision /tmp/pi-ext-e2e/view.json)" && echo bound-to-changes
cmp $S /tmp/pi-ext-e2e/before.json && echo unchanged
```

**Expected**: `after` adds `-extensions/git-status.ts` to the list; `keptKeys` names the other keys;
`bound-to-changes` (the preview's revision covers these changes, so apply must use it, not the
view's); `unchanged`.

### 4. Pi's lock refuses an apply and is kept

```bash
mkdir $S.lock
curl -s -o /dev/null -w '%{http_code}\n' -X POST $API/targets/pi/pi-extensions/apply -d '{"changes":'"$CH"',"revision":"'$REV'"}'
test -d $S.lock && echo kept && rmdir $S.lock
cmp $S /tmp/pi-ext-e2e/before.json && echo unchanged
```

**Expected**: `409` (`pi_extensions_busy`), then `kept` and `unchanged`.

### 5. Apply edits one list only; Pi reads the result

```bash
curl -s -X POST $API/targets/pi/pi-extensions/apply -d '{"changes":'"$CH"',"revision":"'$REV'"}' | jq -r .backupId
diff /tmp/pi-ext-e2e/before.json $S
pi list > /dev/null && echo pi-reads-it
```

**Expected**: a backup id; the diff shows only the `"extensions"` line of the local entry
changing, with every other byte the same; `pi-reads-it`.

### 6. A stale revision is refused

```bash
curl -s -w '\n%{http_code}\n' -X POST $API/targets/pi/pi-extensions/apply -d '{"changes":'"$CH"',"revision":"'$REV'"}'
```

**Expected**: `409` with code `pi_extensions_stale`; the file is not written again.

### 7. Accounts: verified account editable, fork read-only

```bash
curl -s $API/targets/pi-work/pi-extensions | jq -c '{scope, editable, readOnly}'
curl -s $API/targets/pi-fork/pi-extensions | jq -c '{scope, editable, readOnly}'
```

**Expected**: `{"scope":"account","editable":true,...}` and `{"scope":"account","editable":false,"readOnly":"fork"}`.
The fork's CLI is `/bin/true`; Skillshare refuses before running it (covered by
`TestPiExtensionsAccountUsesItsOwnDirectoryAndCLI`, which fails if the fork's CLI runs).

### 8. A project saves only its own settings file

```bash
P=$HOME/code/acme/.pi/settings.json; T=$PI_CODING_AGENT_DIR/trust.json
sha256sum $S $T > /tmp/pi-ext-e2e/global.sha
curl -s $API/targets/acme@pi/pi-extensions | jq -c '{scope, editable, readOnly, trust, packages: [.packages[] | {scope, index, shape, problem}]}'
CH='[{"scope":"global","index":1,"source":"npm:@acme/reviewer@0.4.2","path":"src/slow-check.ts","action":"exclude"},{"scope":"project","index":0,"source":"'$PKG'","path":"extensions/guard.ts","action":"default"}]'
REV=$(curl -s -X POST $API/targets/acme@pi/pi-extensions/preview -d '{"changes":'"$CH"'}' | jq -r .revision)
sha256sum -c /tmp/pi-ext-e2e/global.sha && cat $P
curl -s -w '\n%{http_code}\n' -X POST $API/targets/pi/pi-extensions/apply -d '{"changes":'"$CH"',"revision":"'$REV'"}'
curl -s -X POST $API/targets/acme@pi/pi-extensions/apply -d '{"changes":'"$CH"',"revision":"'$REV'"}' | jq -c '{backupId}'
cat $P; sha256sum -c /tmp/pi-ext-e2e/global.sha
PIR=/opt/agent-clis/lib/node_modules/@earendil-works/pi-coding-agent
for t in trusted untrusted; do
  PI_ROOT=$PIR node /workspace/scripts/pi/resolve-probe.mjs $HOME/code/acme $PI_CODING_AGENT_DIR $t \
    | jq -c '.extensions | with_entries(select(.key | test("slow-check|guard")))'
done
```

**Expected**: `scope` `project`, `editable` true, no `readOnly`, `trust.saved` `trusted`;
packages: project 0 `delta`, global 1 `global`, global 2 `global` with problem `notInstalled`.
The preview leaves the settings, `trust.json` and the project file as they were. The global
target refuses the project revision with `409` and code `pi_extensions_stale`. The project
apply returns a `backupId`. The project file keeps its first entry with only
`+extensions/notify.ts` and gains
`{"source":"npm:@acme/reviewer@0.4.2","autoload":false,"extensions":["-src/slow-check.ts"]}`.
Both checksums are `OK`. Pi trusted: `slow-check.ts` false, `guard.ts` false (the global
rule); Pi untrusted: `slow-check.ts` true, because Pi ignores the project's settings.

### 9. Other targets answer 404

```bash
curl -s -w '\n%{http_code}\n' $API/targets/claude/pi-extensions
```

**Expected**: `404` with code `pi_extensions_not_pi`.

### 10. Selection matches Pi's resolver

```bash
export PI_ROOT=/opt/agent-clis/lib/node_modules/@earendil-works/pi-coding-agent
node /workspace/scripts/pi/extensions-crosscheck.mjs $API pi $PI_CODING_AGENT_DIR
node /workspace/scripts/pi/extensions-crosscheck.mjs $API pi-work $HOME/.pi-work/agent
node /workspace/scripts/pi/extensions-crosscheck.mjs $API acme@pi $PI_CODING_AGENT_DIR $HOME/code/acme
```

**Expected**: each prints `"mismatches": []` and `"omitted": []` and exits 0; `compared` equals
`resolved` (pi 8, pi-work 4, acme@pi 8). As a negative control,
`node /workspace/scripts/pi/extensions-crosscheck.mjs $API pi-work $PI_CODING_AGENT_DIR` exits 1 and
lists the 4 extensions only `pi` has under `omitted`.

### 11. Oplog and backup

```bash
grep '"pi-extensions"' $XDG_STATE_HOME/skillshare/logs/operations.log | tail -2
ls $XDG_STATE_HOME/skillshare/pi-extensions/backups/
```

**Expected**: entries in order: `error` (busy), `ok`, `error` (stale), then from step 8 `error`
for target `pi` (the project revision) and `ok` for target `acme@pi`; one backup file for each
successful apply (`grep` shows only the last two entries).

### 12. Version matrix and Pi's own lock implementation

```bash
cd /workspace && PI_ROOT=$PI_ROOT go test ./internal/plugin -run TestPiNativeLockHoldsAgainstPi -count=1 -v
```

**Expected**: `PASS` after about 12s: Pi's `proper-lockfile` reports the lock as held after more
than its 10s stale age, and acquires it once Skillshare releases it.

```bash
cd /workspace && scripts/pi/version-matrix.sh 0.99.2 1.0.0 1.0.1 && git diff --exit-code scripts/pi/version-evidence.json
```

**Expected**: for each version, `contract/core`, `contract/bundle`, `native-lock` and
`project-native` pass; the recorded evidence is unchanged and includes
`PiMinVersion`. Versions install only under `/tmp/pi-versions`, never globally.

### 13. Dashboard (manual, desktop width)

Run Vite in `/workspace/ui` with its API proxy pointed at port 49423, then open
`/targets/pi?tab=extensions`, `/targets/pi-fork?tab=extensions` and the Extensions tab of the
project `acme` (`/projects/<encoded path>?tab=extensions`) in Clean and Playful, light and dark.

**Expected**: the table has two columns, Extension and Configured (On, Off or Can't tell
with the reason); a missing file shows a "File missing" badge. Switches appear only on
editable rows; **Remove rule** hides the row's switch and shows "Shown in review" until the
preview; the pending bar and review dialog show the diff. The fork shows a read-only note
with a next step and no switches. The hint line ends in one Info button; hovering or
focusing it (Tab) shows that the page changes settings only and, on the project, the trust
explanation and saved/default hints. Details opens the source and rules of a package. On
the project, rows are tagged as from `pi (global)` or as project overrides, and the review
dialog is "Save project settings" with the new or removed project entry.

### 14. Windows native lock and private-state acceptance

Use [the Windows Pi runbook](windows_pi_extensions_runbook.md) for a pinned kit
with Interactive full/basic tokens. Linux checks and Windows cross-compilation
cannot establish NTFS locking or private ACL behavior. Preserve failed-run evidence;
do not repair existing ACLs or change user settings/trust as part of the test.

## Pass Criteria

- All steps marked PASS
- `settings.json` differs from the fixture only in the edited `extensions` list
- The global settings, `trust.json` and every lock directory Skillshare did not create are
  untouched; the project file changes only in step 8
