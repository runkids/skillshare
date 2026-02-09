#!/usr/bin/env bash
# Start a persistent Docker playground for interactive skillshare usage.
set -euo pipefail

source "$(dirname "$0")/_sandbox_common.sh"
SERVICE="sandbox-playground"

require_docker
cd "$PROJECT_ROOT"

# Resolve GitHub token from env vars or gh CLI (for search command inside playground).
SKILLSHARE_PLAYGROUND_GITHUB_TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-}}"
if [ -z "$SKILLSHARE_PLAYGROUND_GITHUB_TOKEN" ] && command -v gh &>/dev/null; then
  SKILLSHARE_PLAYGROUND_GITHUB_TOKEN="$(gh auth token 2>/dev/null || true)"
fi
export SKILLSHARE_PLAYGROUND_GITHUB_TOKEN

docker compose -f "$COMPOSE_FILE" --profile playground build "$SERVICE"

# Prepare shared volumes for host UID/GID access.
docker compose -f "$COMPOSE_FILE" --profile playground run --rm --user "0:0" "$SERVICE" \
  bash -c "mkdir -p /go/pkg/mod /go/build-cache /sandbox-home /tmp && chmod -R 0777 /go/pkg/mod /go/build-cache /sandbox-home /tmp"

docker compose -f "$COMPOSE_FILE" --profile playground up -d "$SERVICE"

# Build skillshare binary and set up aliases.
docker compose -f "$COMPOSE_FILE" --profile playground exec --user "$(id -u):$(id -g)" "$SERVICE" \
  bash -c '
    mkdir -p /sandbox-home/.local/bin
    go build -o /sandbox-home/.local/bin/skillshare ./cmd/skillshare
    ln -sf /sandbox-home/.local/bin/skillshare /sandbox-home/.local/bin/ss
    touch /sandbox-home/.bashrc
    grep -qxF "alias ss='"'"'skillshare'"'"'" /sandbox-home/.bashrc || echo "alias ss='"'"'skillshare'"'"'" >> /sandbox-home/.bashrc
    grep -qxF "alias skillshare-ui='"'"'skillshare ui -g --host 0.0.0.0 --no-open'"'"'" /sandbox-home/.bashrc || echo "alias skillshare-ui='"'"'skillshare ui -g --host 0.0.0.0 --no-open'"'"'" >> /sandbox-home/.bashrc
    grep -qxF "alias skillshare-ui-p='"'"'cd /sandbox-home/demo-project && skillshare ui -p --host 0.0.0.0 --no-open'"'"'" /sandbox-home/.bashrc || echo "alias skillshare-ui-p='"'"'cd /sandbox-home/demo-project && skillshare ui -p --host 0.0.0.0 --no-open'"'"'" >> /sandbox-home/.bashrc
  '

# Initialize global mode (needed for skillshare-ui alias).
docker compose -f "$COMPOSE_FILE" --profile playground exec --user "$(id -u):$(id -g)" "$SERVICE" \
  bash -c '
    if [ ! -f /sandbox-home/.config/skillshare/config.yaml ]; then
      mkdir -p /sandbox-home/.claude/skills
      skillshare init -g --no-copy --all-targets --no-git
    fi
  '

# Set up a demo project for project mode.
docker compose -f "$COMPOSE_FILE" --profile playground exec --user "$(id -u):$(id -g)" "$SERVICE" \
  bash -c '
    DEMO=/sandbox-home/demo-project
    if [ ! -f "$DEMO/.skillshare/config.yaml" ]; then
      mkdir -p "$DEMO"
      cd "$DEMO"

      # Initialize project mode with claude-code target (non-interactive)
      skillshare init -p --targets claude-code

      # Create a sample skill
      mkdir -p .skillshare/skills/hello-world
      cat > .skillshare/skills/hello-world/SKILL.md << '\''SKILL_EOF'\''
---
name: hello-world
description: A sample project skill for the playground demo
---

# Hello World

This is a sample project-level skill created by the playground setup.

## When to Use

Use this skill when greeting a user or starting a new conversation.

## Instructions

1. Greet the user warmly
2. Ask what they need help with
3. Offer relevant suggestions based on the project context
SKILL_EOF

      # Sync skill to target
      skillshare sync
    fi
  '

echo "══════════════════════════════════════════════════════════"
echo "  Playground is running!"
echo "  Enter with:  make sandbox-shell"
echo "══════════════════════════════════════════════════════════"
echo ""
echo "Available commands: skillshare (alias: ss)"
echo ""
echo "Quick start (global mode — ready to use):"
echo "  skillshare status       # check current state"
echo "  skillshare-ui           # start web dashboard (port 19420)"
echo ""
echo "Quick start (project mode — ready to use):"
echo "  cd ~/demo-project       # pre-configured demo project"
echo "  skillshare status       # auto-detects project mode"
echo "  skillshare-ui-p         # project mode web dashboard (port 19420)"
echo ""
if [ -n "$SKILLSHARE_PLAYGROUND_GITHUB_TOKEN" ]; then
  echo "GitHub token: detected (search command will work)"
else
  echo "GitHub token: not set (export GITHUB_TOKEN=... to enable 'skillshare search')"
fi
