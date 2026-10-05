# CLI E2E Runbook: Oh My Pi Hooks

## Scope

Validate issue #409's hooks adapter for Oh My Pi (OMP): verbatim extension files
under `<agent dir>/extensions/skillshare-<entry>.ts`, the global, project and
explicit account directories, disable/remove, backup and restore, stale revision
and conflict handling, read-only listing of OMP's own `hooks/pre` and
`hooks/post` factories, and the shared `PI_CODING_AGENT_DIR` block between Pi
and OMP. No OMP process is launched and no hook code is executed.

## Environment

Run in the devcontainer with mdproof's isolated HOME, or a fresh ssenv for manual
execution. Every step creates its own config and agent directory through
`SKILLSHARE_CONFIG` and `PI_CODING_AGENT_DIR`. Never run against real Agent
configuration. Named-profile routing (#259) is out of scope.

## Steps

### Step 1: Global lifecycle keeps the user's extensions and hook factories

```bash
set -eu
CASE=$(mktemp -d "$HOME/omp-hooks-global.XXXXXX")
export SKILLSHARE_CONFIG="$CASE/config.yaml"
export PI_CODING_AGENT_DIR="$CASE/omp/agent"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
mkdir -p "$PI_CODING_AGENT_DIR/extensions" "$PI_CODING_AGENT_DIR/hooks/pre"
printf 'export default function () {}\n' > "$PI_CODING_AGENT_DIR/extensions/mine.ts"
printf 'export default function () {}\n' > "$PI_CODING_AGENT_DIR/hooks/pre/guard.ts"
printf 'bindings:\n  omp:\n    code: |\n      export default function (pi) {\n        pi.on("tool_call", async () => {});\n      }\n' > "$CASE/entry.yaml"
OUT="$PI_CODING_AGENT_DIR/extensions/skillshare-tool-log.ts"
ss hooks add tool-log --file "$CASE/entry.yaml" --dry-run -g >/dev/null
test ! -e "$OUT"
ss hooks add tool-log --file "$CASE/entry.yaml" --sync -g >/dev/null
grep -q 'pi.on("tool_call"' "$OUT"
ss hooks sync -g --json | jq -e '.applied == []' >/dev/null
ss hooks import --from omp -g --json | jq -e 'map(.name) == ["mine"]' >/dev/null
ss hooks list -g --json > "$CASE/list.json"
ss hooks disable tool-log --sync -g >/dev/null
test ! -e "$OUT"
ss hooks enable tool-log --sync -g >/dev/null
test -f "$OUT"
ss hooks remove tool-log --sync -g >/dev/null
test ! -e "$OUT"
test -f "$PI_CODING_AGENT_DIR/extensions/mine.ts"
test -f "$PI_CODING_AGENT_DIR/hooks/pre/guard.ts"
jq . "$CASE/list.json"
```

Expected:
- exit_code: 0
- jq: .source.entries["tool-log"].bindings.omp.code | test("tool_call")
- jq: [.unmanaged[] | select(.target == "omp") | .names[]] | sort == ["guard.ts", "mine.ts"]
- jq: [.unmanaged[] | select(.target == "omp") | .path | endswith("hooks/pre")] | any

### Step 2: Backups, stale revision and conflicts never overwrite silently

```bash
set -eu
CASE=$(mktemp -d "$HOME/omp-hooks-safety.XXXXXX")
export SKILLSHARE_CONFIG="$CASE/config.yaml"
export PI_CODING_AGENT_DIR="$CASE/omp/agent"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
printf 'bindings:\n  omp:\n    code: |\n      export default function (pi) {\n        pi.on("tool_call", async () => {});\n      }\n' > "$CASE/entry.yaml"
OUT="$PI_CODING_AGENT_DIR/extensions/skillshare-tool-log.ts"
mkdir -p "$PI_CODING_AGENT_DIR/extensions"
printf '// mine\n' > "$OUT"
ss hooks add tool-log --file "$CASE/entry.yaml" -g >/dev/null
if ss hooks sync -g >/dev/null 2>&1; then echo "conflict not reported"; exit 1; fi
test "$(cat "$OUT")" = "// mine"
ss hooks sync tool-log --replace -g >/dev/null
grep -q 'tool_call' "$OUT"
sed -i 's/tool_call/tool_result/' "$CASE/entry.yaml"
ss hooks edit tool-log --file "$CASE/entry.yaml" -g >/dev/null
ss hooks sync -g --json > "$CASE/sync.json"
jq -e '.backupIds | length > 0' "$CASE/sync.json" >/dev/null
grep -q 'tool_result' "$OUT"
BACKUP=$(jq -r '.backupIds[0]' "$CASE/sync.json")
ss hooks restore "$BACKUP" --dry-run -g >/dev/null
grep -q 'tool_result' "$OUT"
if ss hooks sync --revision stale -g >/dev/null 2>&1; then echo "stale revision accepted"; exit 1; fi
grep -q 'tool_result' "$OUT"
jq . "$CASE/sync.json"
```

Expected:
- exit_code: 0
- jq: .plan.changes[0].target == "omp" and .plan.changes[0].action == "update"
- jq: .plan.blocked == false

### Step 3: Project hooks land in .omp/extensions only

```bash
set -eu
CASE=$(mktemp -d "$HOME/omp-hooks-project.XXXXXX")
export PI_CODING_AGENT_DIR="$CASE/omp/agent"
cd "$CASE"
mkdir -p .skillshare
printf 'targets: []\n' > .skillshare/config.yaml
printf 'bindings:\n  omp:\n    code: |\n      export default function (pi) {\n        pi.on("tool_call", async () => {});\n      }\n' > "$CASE/entry.yaml"
ss hooks add tool-log --file "$CASE/entry.yaml" --sync -p >/dev/null
grep -q 'tool_call' .omp/extensions/skillshare-tool-log.ts
test ! -e "$PI_CODING_AGENT_DIR"
test ! -e .pi
ss hooks list -p --json
```

Expected:
- exit_code: 0
- jq: .source.entries["tool-log"].bindings.omp != null
- jq: .paths.omp | endswith("/.omp/extensions")

### Step 4: Explicit account stays pinned; a shared agent directory blocks Pi and OMP

```bash
set -eu
CASE=$(mktemp -d "$HOME/omp-hooks-account.XXXXXX")
export SKILLSHARE_CONFIG="$CASE/config.yaml"
export PI_CODING_AGENT_DIR="$CASE/shared/agent"
mkdir -p "$CASE/work/agent"
printf 'targets:\n  omp-work:\n    agent: omp\n    config_dir: %s/work/agent\n    skills: {enabled: false}\n' "$CASE" > "$SKILLSHARE_CONFIG"
printf 'bindings:\n  omp-work:\n    code: |\n      export default function (pi) {\n        pi.on("tool_call", async () => {});\n      }\n' > "$CASE/account.yaml"
ss hooks add tool-log --file "$CASE/account.yaml" --sync -g >/dev/null
grep -q 'tool_call' "$CASE/work/agent/extensions/skillshare-tool-log.ts"
test ! -e "$PI_CODING_AGENT_DIR"
printf 'bindings:\n  pi: &code\n    code: |\n      export default function (pi) {\n        pi.on("tool_call", async () => {});\n      }\n  omp: *code\n' > "$CASE/both.yaml"
ss hooks add both --file "$CASE/both.yaml" --sync -g --json > "$CASE/both.json" || true
test ! -e "$PI_CODING_AGENT_DIR"
jq . "$CASE/both.json"
```

Expected:
- exit_code: 0
- jq: .error | test("omp and pi both write") and test("PI_CODING_AGENT_DIR")

## Pass Criteria

- All four steps pass without launching OMP or executing any hook code.
- Dry runs, blocked plans, stale revisions and conflicts leave native files unchanged.
- The user's own `extensions/*.ts` and `hooks/pre|post` factories survive every sync and are listed, never imported from `hooks/pre|post`.
- Account directories receive the extension while the default and shared OMP homes stay untouched.
