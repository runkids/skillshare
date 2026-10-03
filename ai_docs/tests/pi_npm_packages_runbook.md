# CLI E2E Runbook: npm Packages through Pi

Validates `plugin add npm:<package>` against a real Pi 0.99.2 and the npm registry, on an
isolated HOME.

**Origin**: packages listed on pi.dev are npm packages, which Skillshare can't fetch or review
itself; it hands them to `pi install`. The Go and integration tests use a fake `pi`, so the
real CLI's settings, version replacement, account directories and project trust need a check.

## Scope

- `discover` rejects an npm source and points to `plugin add`
- Non-Pi targets, Pi accounts running another CLI, a missing target and Git-only flags are
  refused, with the reason the CLI prints matching the dashboard's key
- A preview writes nothing; an apply runs `pi install` and records the binding
- `update` skips a package pinned to an exact version
- Adding another version installs it, and Pi replaces the entry's source in place
- An account that already has the same source imports it without running Pi
- `-p` installs into the project's `.pi/settings.json` only, once Pi trusts the project
- `remove` runs `pi remove` on every bound target
- Another version keeps a package's extension filters, and Skillshare records them again so a
  reinstall restores them

## Environment

Run inside the devcontainer, in one shell, so the variables from step 1 carry over. Pi
(`pi --version` 0.99.2) must be on PATH, and the container needs to reach the npm registry.
Everything is written under `/tmp/pi-npm-e2e`. Do not run the CLI from `/workspace`.

The test package is `@evoclock/pi-agentic-driver` (versions 1.0.0 and 1.1.0, about 1.4 MB,
no install scripts). Check `npm view @evoclock/pi-agentic-driver@1.1.0 scripts` prints
nothing before running; if it changed, choose another pi.dev package with two versions.

```bash
CONTAINER=$(docker compose -f .devcontainer/docker-compose.yml ps -q skillshare-devcontainer)
docker exec -it "$CONTAINER" bash
```

## Steps

### 1. Setup: build and create an isolated HOME with two Pi accounts

```bash
cd /workspace && make build
rm -rf -- /tmp/pi-npm-e2e && mkdir -p /tmp/pi-npm-e2e/home && cd /tmp/pi-npm-e2e
export HOME=/tmp/pi-npm-e2e/home XDG_CONFIG_HOME=/tmp/pi-npm-e2e/home/.config
export XDG_STATE_HOME=$HOME/.local/state XDG_DATA_HOME=$HOME/.local/share XDG_CACHE_HOME=$HOME/.cache
export PI_CODING_AGENT_DIR=$HOME/.pi/agent
unset SENPI_CODING_AGENT_DIR OMO_CODING_AGENT_DIR
mkdir -p $PI_CODING_AGENT_DIR $HOME/.pi-work/agent $HOME/.pi-omo/agent $XDG_CONFIG_HOME/skillshare
cat > $XDG_CONFIG_HOME/skillshare/config.yaml <<'EOF'
targets:
  pi-work: {agent: pi, config_dir: ~/.pi-work/agent, skills: {enabled: false}}
  pi-omo: {agent: pi, config_dir: ~/.pi-omo/agent, cli: omo, skills: {enabled: false}}
EOF
ss() { /workspace/bin/skillshare "$@"; }
PKG=npm:@evoclock/pi-agentic-driver
S=$PI_CODING_AGENT_DIR/settings.json
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
pi --version
npm view ${PKG#npm:}@1.1.0 version scripts
```

**Expected**: `0.99.2`, then `1.1.0` with no `scripts` line.

### 2. Discover rejects npm sources

```bash
ss plugin discover $PKG --json -g; echo "exit=$?"
```

**Expected**: `npm packages can't be previewed: Pi downloads them when it installs them. Add
one to a Pi target with: skillshare plugin add npm:@evoclock/pi-agentic-driver --target pi`,
then `exit=1`.

### 3. Targets and flags that can't take npm packages are refused

```bash
ss plugin add $PKG --target opencode --dry-run --json -g | head -1 | jq -c '.changes[] | {action, messageKey, message}'
ss plugin add $PKG --target pi-omo --dry-run --json -g | head -1 | jq -c '.changes[] | {action, messageKey, message}'
ss plugin add $PKG --dry-run --json -g; echo "exit=$?"
ss plugin add $PKG --target pi --entry x --dry-run --json -g; echo "exit=$?"
ls -A $PI_CODING_AGENT_DIR $HOME/.pi-omo/agent
```

**Expected**:
- opencode is `blocked` with key `plugins.error.npmPiOnly` and the message `npm packages are
  installed by Pi; choose a Pi target.`, even though opencode is not installed here.
- pi-omo is `blocked` with key `plugins.error.npmOtherCli` and a message naming `omo`, without
  running `omo`.
- No target: `npm packages are installed by Pi; choose a Pi target`, `exit=1`.
- `--entry`: `an npm source takes no source ref, entry or plugin selector`, `exit=1`.
- Both directories are empty.

A blocked `--json` preview also prints `✗ plugin changes are blocked` on stdout after the JSON,
hence `head -1`.

### 4. A preview writes nothing

```bash
ss plugin add $PKG@1.0.0 --target pi --dry-run --json -g | jq -c '.changes[] | {name, target, action, messageKey}'
ls -A $PI_CODING_AGENT_DIR; grep -c plugins $CFG
```

**Expected**: `{"name":"pi-agentic-driver","target":"pi","action":"install","messageKey":"plugins.note.npmInstall"}`;
the Pi directory is still empty and `grep` prints `0`.

### 5. Install a pinned version through Pi

```bash
ss plugin add $PKG@1.0.0 --target pi --no-tui -g; echo "exit=$?"
jq -c .packages $S
grep -A3 'pi-agentic-driver:' $CFG
pi list
jq -r .version $PI_CODING_AGENT_DIR/npm/node_modules/@evoclock/pi-agentic-driver/package.json
grep '"plugin add"' $XDG_STATE_HOME/skillshare/logs/operations.log | tail -1
```

**Expected**: `pi-agentic-driver  pi · installed`, `exit=0`;
`["npm:@evoclock/pi-agentic-driver@1.0.0"]`; the binding `pi: id: npm:@evoclock/pi-agentic-driver@1.0.0`
with no `source`; `pi list` shows the package under User packages; version `1.0.0`; an oplog
entry with `"status":"ok"`.

### 6. Update skips the pinned version

```bash
ss plugin update pi-agentic-driver --dry-run --json -g | jq -c '.changes[] | {action, messageKey, messageArgs}'
```

**Expected**: `{"action":"skip","messageKey":"plugins.skip.npmPinned","messageArgs":{"version":"1.0.0"}}`.

### 7. Adding another version replaces Pi's entry

```bash
ss plugin add $PKG@1.1.0 --name pi-agentic-driver --target pi --dry-run --json -g | jq -c '.changes[] | {action, messageKey, messageArgs}'
ss plugin add $PKG@1.1.0 --name pi-agentic-driver --target pi --no-tui -g; echo "exit=$?"
jq -c .packages $S
grep -A3 'pi-agentic-driver:' $CFG
jq -r .version $PI_CODING_AGENT_DIR/npm/node_modules/@evoclock/pi-agentic-driver/package.json
```

**Expected**: the preview is `install` with key `plugins.note.npmReplace` and
`{"from":"npm:@evoclock/pi-agentic-driver@1.0.0"}`; `exit=0`; Pi keeps one entry,
`["npm:@evoclock/pi-agentic-driver@1.1.0"]`; the binding ID is `@1.1.0`; version `1.1.0`.

### 8. An account that already has the package imports it

```bash
PI_CODING_AGENT_DIR=$HOME/.pi-work/agent pi install $PKG@1.1.0 > /dev/null
cp $HOME/.pi-work/agent/settings.json /tmp/pi-npm-e2e/work-before.json
ss plugin add $PKG@1.1.0 --name pi-agentic-driver --target pi-work --dry-run --json -g | jq -c '.changes[] | {target, action}'
ss plugin add $PKG@1.1.0 --name pi-agentic-driver --target pi-work --no-tui -g; echo "exit=$?"
cmp $HOME/.pi-work/agent/settings.json /tmp/pi-npm-e2e/work-before.json && echo unchanged
grep -A5 'pi-agentic-driver:' $CFG
```

**Expected**: `{"target":"pi-work","action":"import"}`; `pi-agentic-driver  pi-work · imported`,
`exit=0`; `unchanged`; bindings for both `pi` and `pi-work` with ID `@1.1.0`.

### 9. Remove runs pi remove on every bound target

```bash
ss plugin remove pi-agentic-driver --no-tui -g; echo "exit=$?"
jq -c .packages $S $HOME/.pi-work/agent/settings.json
grep -c pi-agentic-driver $CFG
```

**Expected**: `pi · removed` and `pi-work · removed`, `exit=0`; `[]` twice; `0`.

### 10. Another version keeps the package's filters, and a reinstall restores them

```bash
pi install $PKG@1.0.0 > /dev/null
jq '.packages = [{source: .packages[0], extensions: ["-extensions/pulse.ts"]}]' $S > $S.new && mv $S.new $S
ss plugin import $PKG@1.0.0 --from pi --name pi-agentic-driver --no-tui -g; echo "exit=$?"
ss plugin add $PKG@1.1.0 --name pi-agentic-driver --target pi --no-tui -g; echo "exit=$?"
jq -c .packages $S
grep -c pi_registration $CFG
pi remove $PKG@1.1.0 > /dev/null
ss plugin sync --no-tui -g; echo "exit=$?"
jq -c .packages $S
ss plugin remove pi-agentic-driver --no-tui -g > /dev/null; jq -c .packages $S
```

**Expected**: `pi · imported`, then `pi · installed`, both `exit=0`. Pi moves the entry to the
new version and keeps its filter:
`[{"source":"npm:@evoclock/pi-agentic-driver@1.1.0","extensions":["-extensions/pulse.ts"]}]`,
and the binding has a `pi_registration` (`1`). After the package is removed outside
Skillshare, `sync` reinstalls it (`pi · installed`, `exit=0`) with the same filtered entry.
The final removal leaves `[]`.

### 11. A project installs locally once Pi trusts it

```bash
mkdir -p $HOME/code/acme && cd $HOME/code/acme
ss init -p --targets pi > /dev/null
cp $S /tmp/pi-npm-e2e/global-before.json
ss plugin add $PKG --target pi-work --dry-run --json -p; echo "exit=$?"
ss plugin add $PKG --target pi --no-tui -p; echo "exit=$?"
ls .pi
printf '{"%s": true}\n' "$PWD" > $PI_CODING_AGENT_DIR/trust.json
ss plugin add $PKG --target pi --no-tui -p; echo "exit=$?"
jq -c .packages .pi/settings.json
cmp $S /tmp/pi-npm-e2e/global-before.json && echo global-unchanged
ss plugin remove pi-agentic-driver --no-tui -p; echo "exit=$?"
jq -c .packages .pi/settings.json
cd /tmp/pi-npm-e2e
```

**Expected**:
- An account is not a project target: `unsupported plugin target "pi-work"`, `exit=1`.
- `init -p` created `.pi/skills`, so Pi treats the project as having local config and the
  first install fails with `project packages require trust established in Pi`, `exit=1`;
  `ls .pi` shows only `skills`. Skillshare does not pass `--approve`.
- Once Pi trusts the project, the install (`pi install --local`) prints `pi · installed`,
  `exit=0`; the project file is `["npm:@evoclock/pi-agentic-driver"]` and `global-unchanged`.
- Removal prints `pi · removed`, `exit=0`, and the project file is `[]`.

### 12. Dashboard (manual, desktop width)

Run the API from `/tmp/pi-npm-e2e` with the variables from step 1
(`ss ui --no-open --port 49423`), point Vite in `/workspace/ui` at it, then open
`/targets/pi-work?tab=extensions` in Clean and Playful, light and dark.

**Expected**: the empty Extensions tab offers **Add a package**, which opens Plugins with the
add dialog and `pi-work` selected, and the URL loses `?add=`. Entering
`npm:@evoclock/pi-agentic-driver` shows the install-script notice and lists only `Pi` and
`pi-work` (not `pi-omo`). The preview shows `install` with the npm note; after applying, the
Extensions tab lists the package's extensions.

## Pass Criteria

- All steps marked PASS
- Nothing is written outside `/tmp/pi-npm-e2e`
- Pi's settings change only through `pi install` / `pi remove`; refused and previewed changes
  leave every settings file as it was
