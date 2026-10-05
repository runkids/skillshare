# OMP Extension Selection E2E

## Scope

Exercise the real dashboard API with an installed OMP 18.6.1 package: discover a
standalone extension, preview and apply a disable, reject a stale preview, and
verify preservation of unrelated YAML and non-execution of extension code.

## Environment

Run inside the devcontainer in an isolated `ssenv`. Build `ui/dist` first. Put the
real OMP 18.6.1 npm/Bun launcher on `PATH`; a wrapper or standalone binary without
verifiable package metadata is intentionally read-only. The runbook never invokes
OMP or imports its extensions. Port 19431 must be free. Keep the environment and
report for inspection; no real Agent configuration is used.

## Steps

### 1. Preview, apply, and stale-write protection without executing code

```bash
set -eu
command -v omp >/dev/null
cd /workspace
go build -o "$HOME/omp-selection-skillshare" ./cmd/skillshare
cd "$HOME"
mkdir -p "$XDG_CONFIG_HOME/skillshare/skills" "$HOME/.omp/agent/extensions"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<EOF
source: $XDG_CONFIG_HOME/skillshare/skills
targets:
  omp:
    path: $HOME/.omp/agent/skills
EOF
cat > "$HOME/.omp/agent/config.yml" <<'EOF'
# Preserve this native configuration comment.
customValue: 'keep quoted'
disabledExtensions:
  - extension-module:unrelated
EOF
cat > "$HOME/.omp/agent/extensions/selection-probe.ts" <<EOF
import { writeFileSync } from 'node:fs';
writeFileSync('$HOME/extension-executed', 'unsafe');
export default function () {}
EOF
mkdir -p "$XDG_CACHE_HOME/skillshare/ui"
ln -s /workspace/ui/dist "$XDG_CACHE_HOME/skillshare/ui/dev"
"$HOME/omp-selection-skillshare" ui --host 127.0.0.1 --port 19431 --no-open -g > "$HOME/omp-selection-server.log" 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true' EXIT
api=http://localhost:19431/api/targets/omp/omp-extensions
ready=0
for attempt in $(seq 1 100); do
  if curl -fsS "$api" > "$HOME/view.json" 2>/dev/null; then ready=1; break; fi
  kill -0 "$pid"
  sleep 0.1
done
test "$ready" = 1
jq -e '.rows[] | select(.name == "selection-probe") | .selectable and .enabled' "$HOME/view.json" >/dev/null
jq '{revision, changes: [.rows[] | select(.name == "selection-probe") | {key, enabled:false}]}' "$HOME/view.json" > "$HOME/request.json"
cp "$HOME/.omp/agent/config.yml" "$HOME/before.yml"
curl -fsS -H 'Content-Type: application/json' --data-binary @"$HOME/request.json" "$api/preview" > "$HOME/plan.json"
cmp "$HOME/before.yml" "$HOME/.omp/agent/config.yml"
jq -e '.rows | length == 1 and .[0].before == true and .[0].after == false' "$HOME/plan.json" >/dev/null
jq --slurpfile plan "$HOME/plan.json" '.revision = $plan[0].revision' "$HOME/request.json" > "$HOME/apply.json"
curl -fsS -H 'Content-Type: application/json' --data-binary @"$HOME/apply.json" "$api/apply" > "$HOME/applied.json"
jq -e '.backupId | length > 0' "$HOME/applied.json" >/dev/null
grep -Fx '# Preserve this native configuration comment.' "$HOME/.omp/agent/config.yml" >/dev/null
grep -Fx "customValue: 'keep quoted'" "$HOME/.omp/agent/config.yml" >/dev/null
grep -F 'extension-module:unrelated' "$HOME/.omp/agent/config.yml" >/dev/null
grep -F 'extension-module:selection-probe' "$HOME/.omp/agent/config.yml" >/dev/null
curl -fsS "$api" > "$HOME/disabled.json"
jq -e '.rows[] | select(.name == "selection-probe") | .enabled == false and .selection == "disabled"' "$HOME/disabled.json" >/dev/null
jq '{revision, changes: [.rows[] | select(.name == "selection-probe") | {key, enabled:true}]}' "$HOME/disabled.json" > "$HOME/enable-request.json"
curl -fsS -H 'Content-Type: application/json' --data-binary @"$HOME/enable-request.json" "$api/preview" > "$HOME/enable-plan.json"
jq --slurpfile plan "$HOME/enable-plan.json" '.revision = $plan[0].revision' "$HOME/enable-request.json" > "$HOME/enable-apply.json"
printf '\n# Concurrent native edit\n' >> "$HOME/.omp/agent/config.yml"
cp "$HOME/.omp/agent/config.yml" "$HOME/concurrent.yml"
status=$(curl -sS -o "$HOME/stale.json" -w '%{http_code}' -H 'Content-Type: application/json' --data-binary @"$HOME/enable-apply.json" "$api/apply")
test "$status" = 409
jq -e '.error_code == "omp_extensions_stale"' "$HOME/stale.json" >/dev/null
cmp "$HOME/concurrent.yml" "$HOME/.omp/agent/config.yml"
test ! -e "$HOME/extension-executed"
printf '{"preview_read_only":true,"disabled":true,"preserved_yaml":true,"stale_rejected":true,"code_not_executed":true}\n'
```

**Expected**
- exit_code: 0
- jq: .preview_read_only and .disabled and .preserved_yaml and .stale_rejected and .code_not_executed

## Pass Criteria

The approved selection is written, unrelated settings and comments remain, stale
application does not overwrite a concurrent edit, and no extension is executed.
Native lock interoperability and unsupported-row refusal are covered by the Go
regression suite; this runbook does not establish runtime loading or Windows behavior.
