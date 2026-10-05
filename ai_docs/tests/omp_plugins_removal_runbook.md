# OMP Plugin Removal and Reinstall E2E

## Scope

Use native OMP 18.6.1 and an inert plugin to verify scoped removal, retained
shared cache used by an invisible project, and reinstall into a new cache
identity. No Agent session, extension code or real user configuration is used.

## Environment

Run in the devcontainer with a fresh `ssenv` and the pinned OMP/Bun installation
on PATH. Keep the environment and JSON report for inspection. Each step is
self-contained; the native `plugin` commands do not start an Agent session.

## Steps

### 1. Remove a user install without damaging an invisible project, then reinstall

```bash
set -eu
cd /workspace
go build -o "$HOME/omp-removal-skillshare" ./cmd/skillshare
ss="$HOME/omp-removal-skillshare"
cd "$HOME"
mkdir -p "$HOME/removal-source/.claude-plugin" "$HOME/removal-source/skills/removal" "$HOME/foreign-project/.omp"
cat > "$HOME/removal-source/.claude-plugin/plugin.json" <<'EOF'
{"name":"removal-probe","version":"1.0.0","description":"An inert fixture with a long description for native scoped removal and retained shared cache verification."}
EOF
cat > "$HOME/removal-source/package.json" <<'EOF'
{"name":"removal-runtime","version":"1.0.0","omp":{"extensions":["index.ts"]}}
EOF
cat > "$HOME/removal-source/skills/removal/SKILL.md" <<'EOF'
---
name: removal
description: Inert removal fixture
---
Never execute anything.
EOF
cat > "$HOME/removal-source/index.ts" <<EOF
import { writeFileSync } from 'node:fs';
writeFileSync('$HOME/extension-executed', 'unsafe');
export default function () {}
EOF
"$ss" plugin add "$HOME/removal-source" --target omp --no-tui -g > "$HOME/add.log"
omp plugin list --json > "$HOME/native-before.json"
id=$(jq -r '.marketplace[] | select(.scope == "user") | .id' "$HOME/native-before.json")
cache=$(jq -r '.marketplace[] | select(.scope == "user") | .entries[0].installPath' "$HOME/native-before.json")
test -n "$id" && test -d "$cache"
(cd "$HOME/foreign-project" && omp plugin install "$id" --scope project > "$HOME/foreign-install.log")
foreign="$HOME/foreign-project/.omp/plugins"
sha256sum "$foreign/installed_plugins.json" "$foreign/omp-plugins.lock.json" > "$HOME/foreign-before.sha"
(cd "$cache" && find . -type f -print0 | sort -z | xargs -0 sha256sum) > "$HOME/cache-before.sha"
"$ss" plugin remove removal-probe --dry-run --json -g > "$HOME/remove-plan.json"
jq -e '.blocked == false and .changes[0].action == "remove" and .changes[0].messageKey == "plugins.note.ompRemoval"' "$HOME/remove-plan.json" >/dev/null
revision=$(jq -r '.revision' "$HOME/remove-plan.json")
"$ss" plugin remove removal-probe --revision "$revision" --no-tui -g > "$HOME/remove.log"
omp plugin list --json > "$HOME/native-removed.json"
jq -e '.marketplace | all(.scope != "user")' "$HOME/native-removed.json" >/dev/null
test ! -e "$HOME/.omp/plugins/node_modules/removal-runtime"
jq -e '.plugins | has("removal-runtime") | not' "$HOME/.omp/plugins/omp-plugins.lock.json" >/dev/null
sha256sum -c "$HOME/foreign-before.sha" > /dev/null
(cd "$cache" && find . -type f -print0 | sort -z | xargs -0 sha256sum) > "$HOME/cache-after.sha"
cmp "$HOME/cache-before.sha" "$HOME/cache-after.sha"
test -f "$foreign/node_modules/removal-runtime/index.ts"
(cd "$HOME/foreign-project" && omp plugin list --json) | jq -e --arg id "$id" '.marketplace | any(.id == $id and .scope == "project")' > /dev/null
"$ss" plugin add "$HOME/removal-source" --target omp --dry-run --json -g > "$HOME/reinstall-plan.json"
jq -e --arg id "$id" '.blocked == false and .changes[0].id != $id and .changes[0].messageKey == "plugins.note.ompFreshCache"' "$HOME/reinstall-plan.json" > /dev/null
"$ss" plugin add "$HOME/removal-source" --target omp --no-tui -g > "$HOME/reinstall.log"
omp plugin list --json | jq -e --arg id "$id" --arg cache "$cache" '.marketplace | any(.scope == "user" and .id != $id and .entries[0].installPath != $cache)' > /dev/null
sha256sum -c "$HOME/foreign-before.sha" > /dev/null
test -f "$foreign/node_modules/removal-runtime/index.ts"
test ! -e "$HOME/extension-executed"
printf '{"removed":true,"foreignProjectIntact":true,"reinstalledFresh":true,"executed":false}\n'
```

**Expected:**
- exit_code: 0
- jq: .removed and .foreignProjectIntact and .reinstalledFresh and (.executed == false)

### 2. Deselect and sync a project installation while retaining the user installation

```bash
set -eu
cd /workspace
go build -o "$HOME/omp-project-removal-skillshare" ./cmd/skillshare
ss="$HOME/omp-project-removal-skillshare"
project="$HOME/removal-project"
source="$HOME/project-removal-source"
mkdir -p "$source/.claude-plugin" "$source/skills/removal" "$project/.skillshare/skills" "$project/.omp"
cat > "$source/.claude-plugin/plugin.json" <<'EOF'
{"name":"project-removal-probe","version":"1.0.0"}
EOF
cat > "$source/package.json" <<'EOF'
{"name":"project-removal-runtime","version":"1.0.0"}
EOF
cat > "$source/skills/removal/SKILL.md" <<'EOF'
---
name: removal
description: Inert project removal fixture
---
No executable resources.
EOF
cat > "$project/.skillshare/config.yaml" <<'EOF'
source: ./skills
targets:
  omp:
    path: .omp/skills
EOF
cd "$project"
"$ss" plugin add "$source" --target omp --no-tui -p > "$HOME/project-add.log"
omp plugin list --json > "$HOME/project-native.json"
id=$(jq -r '.marketplace[] | select(.scope == "project") | .id' "$HOME/project-native.json")
cache=$(jq -r '.marketplace[] | select(.scope == "project") | .entries[0].installPath' "$HOME/project-native.json")
(cd "$HOME" && omp plugin install "$id" --scope user > "$HOME/user-install.log")
userRoot="$HOME/.omp/plugins"
sha256sum "$userRoot/installed_plugins.json" "$userRoot/omp-plugins.lock.json" > "$HOME/user-before.sha"
"$ss" plugin disable project-removal-probe --target omp --no-tui -p > "$HOME/project-disable.log"
"$ss" plugin sync project-removal-probe --no-tui -p > "$HOME/project-sync.log"
omp plugin list --json | jq -e --arg id "$id" '.marketplace | all(.id != $id or .scope != "project")' > /dev/null
test ! -e "$project/.omp/plugins/node_modules/project-removal-runtime"
test -f "$userRoot/node_modules/project-removal-runtime/package.json"
test -d "$cache"
sha256sum -c "$HOME/user-before.sha" > /dev/null
printf '{"projectRemoved":true,"userUnchanged":true,"cacheRetained":true}\n'
```

**Expected:**
- exit_code: 0
- jq: .projectRemoved and .userUnchanged and .cacheRetained

## Pass Criteria

The scoped registry, runtime selection and link are removed rather than disabled;
foreign registrations, links and cache bytes remain unchanged. Reinstall uses a
fresh identity, and no extension executes during management.
