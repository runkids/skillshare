#!/usr/bin/env bash
# Builds an isolated HOME for the Pi Extensions tab runbook (ai_docs/tests/pi_extensions_runbook.md).
# Run inside the devcontainer. Nothing outside ROOT is written; the fixture packages
# contain no runnable code, and Pi is only asked for --version.
#
#   scripts/pi/extensions-e2e-fixture.sh [ROOT]     # default /tmp/pi-ext-e2e
#   source "$ROOT/env.sh"                           # isolated HOME, config and Pi dirs
set -euo pipefail

ROOT="${1:-/tmp/pi-ext-e2e}"
case "$ROOT" in
  /tmp/pi-ext-e2e*) ;;
  *) echo "refusing ROOT outside /tmp/pi-ext-e2e*: $ROOT" >&2; exit 1 ;;
esac
if [ -e "$ROOT" ]; then
  echo "$ROOT exists; remove it first: rm -rf -- '$ROOT'" >&2
  exit 1
fi

H="$ROOT/home"
AGENT="$H/.pi/agent"
PKG="$H/pkgs/tools"
mkdir -p "$PKG/extensions" "$PKG/skills/review" "$PKG/prompts" "$AGENT/extensions" "$H/.pi-work/agent" \
  "$H/extras/pi-ext" "$H/code/acme/.pi" "$H/.config/skillshare/skills" "$H/pi-skills" "$H/claude-skills"

cat > "$PKG/package.json" <<'EOF'
{"name":"tools","keywords":["pi-package"]}
EOF
for f in guard git-status notify slow-lint; do
  echo "throw new Error('fixture: never imported')" > "$PKG/extensions/$f.ts"
done
printf -- '---\nname: review\ndescription: fixture\n---\n' > "$PKG/skills/review/SKILL.md"
echo fixture > "$PKG/prompts/commit.md"

# An npm package where Pi installs it, with a manifest naming its extensions.
NPM="$AGENT/npm/node_modules/@acme/reviewer"
mkdir -p "$NPM/src"
cat > "$NPM/package.json" <<'EOF'
{"name":"@acme/reviewer","version":"0.4.2","pi":{"extensions":["./src/review.ts","./src/slow-check.ts"]}}
EOF
echo "throw new Error('fixture')" > "$NPM/src/review.ts"
echo "throw new Error('fixture')" > "$NPM/src/slow-check.ts"

# An extra linked into Pi's own folder, and a file Pi found there on its own.
echo "throw new Error('fixture')" > "$H/extras/pi-ext/session-guard.ts"
ln -s "$H/extras/pi-ext/session-guard.ts" "$AGENT/extensions/session-guard.ts"
echo "throw new Error('fixture')" > "$AGENT/extensions/scratch.ts"

cat > "$AGENT/settings.json" <<EOF
{
  "theme": "dark",
  "packages": [
    {
      "source": "$PKG",
      "extensions": ["-extensions/notify.ts", "!extensions/slow-*.ts"],
      "prompts": ["!prompts/commit.md"],
      "themes": [],
      "autoUpdate": false
    },
    "npm:@acme/reviewer@0.4.2",
    "git:github.com/acme/not-installed@v1"
  ]
}
EOF
cat > "$H/.pi-work/agent/settings.json" <<EOF
{"packages": ["$PKG"]}
EOF
# A Pi account that runs another executable, which Skillshare treats as an unverified fork.
mkdir -p "$H/.pi-fork/agent"
cp "$H/.pi-work/agent/settings.json" "$H/.pi-fork/agent/settings.json"
cat > "$H/code/acme/.pi/settings.json" <<EOF
{"packages": [{"source": "$PKG", "autoload": false, "extensions": ["+extensions/notify.ts", "-extensions/guard.ts"]}]}
EOF
cat > "$AGENT/trust.json" <<EOF
{"$H/code/acme": true}
EOF

cat > "$H/.config/skillshare/config.yaml" <<EOF
source: $H/.config/skillshare/skills
mode: merge
targets:
  pi:
    path: $H/pi-skills
  claude:
    path: $H/claude-skills
  pi-work:
    agent: pi
    config_dir: $H/.pi-work/agent
  pi-fork:
    agent: pi
    config_dir: $H/.pi-fork/agent
    cli: /bin/true
extras:
  - name: pi-ext
    source: $H/extras/pi-ext
    targets:
      - path: $AGENT/extensions
projects:
  $H/code/acme:
    targets: [pi]
    skills: {}
EOF

cat > "$ROOT/env.sh" <<EOF
export HOME="$H"
export XDG_CONFIG_HOME="$H/.config" XDG_DATA_HOME="$H/.local/share" XDG_STATE_HOME="$H/.local/state" XDG_CACHE_HOME="$H/.cache"
export SKILLSHARE_CONFIG="$H/.config/skillshare/config.yaml"
export PI_CODING_AGENT_DIR="$AGENT"
export PATH="/opt/agent-clis/bin:\$PATH"
export PKG="$PKG"
EOF
echo "fixture ready: source $ROOT/env.sh"
