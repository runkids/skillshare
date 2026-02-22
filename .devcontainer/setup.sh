#!/usr/bin/env bash
# Devcontainer post-create setup — mirrors sandbox_playground_up.sh
# for an out-of-the-box demo experience inside VS Code / Codespaces.
#
# Environment assumptions (from docker-compose.yml + devcontainer.json):
#   HOME=/tmp  |  binary at /workspace/bin/skillshare  |  PATH includes /workspace/bin
set -euo pipefail

if [ "${HOME:-}" != "/tmp" ] || [ ! -d /workspace ] || [ ! -f /workspace/go.mod ]; then
  echo "Refusing to run: expected devcontainer context (HOME=/tmp, /workspace mounted)." >&2
  exit 1
fi
cd /workspace

# ── 0. Token env: .env overrides host-passed env vars ────────────────
# Docker-compose passes host tokens via environment (fallback).
# If .devcontainer/.env exists, source it in every login shell to override.
cat > /etc/profile.d/skillshare-env.sh << 'PROFILE_EOF'
if [ -f /workspace/.devcontainer/.env ]; then
  set -a
  . /workspace/.devcontainer/.env
  set +a
fi
PROFILE_EOF

# ── 1. Build CLI ────────────────────────────────────────────────────
echo "▸ Building skillshare binary …"
make build

# ── 1a. Install air (Go hot-reload) — optional, `ui` command auto-installs if missing
echo "▸ Installing air (hot-reload) …"
go install github.com/air-verse/air@latest || echo "  ⚠ air install failed (will auto-install on first 'ui' run)"

# ── 1b. Ensure Linux binary + stable command symlinks ─────────────
echo "▸ Verifying skillshare command targets …"
./.devcontainer/ensure-skillshare-linux-binary.sh

# ── 1c. Install frontend dependencies ─────────────────────────────
echo "▸ Installing UI dependencies …"
(cd /workspace/ui && pnpm install --frozen-lockfile)
echo "▸ Installing website dependencies …"
(cd /workspace/website && pnpm install --frozen-lockfile)

# ── 2. Global mode init ────────────────────────────────────────────
echo "▸ Initializing global mode …"
mkdir -p "$HOME/.claude/skills"
GLOBAL_CFG="$HOME/.config/skillshare/config.yaml"
if [ -f "$GLOBAL_CFG" ]; then
  echo "  ✓ Already initialized ($GLOBAL_CFG), skipping init"
elif skillshare status >/dev/null 2>&1; then
  echo "  ✓ Already initialized (detected via 'skillshare status'), skipping init"
else
  skillshare init -g --no-copy --all-targets --no-git --skill
fi

# ── 3. Create demo content (shared with sandbox playground) ───────
echo "▸ Creating demo content …"
SKILLS="$HOME/.config/skillshare/skills"
CFG="$HOME/.config/skillshare"
DEMO="$HOME/demo-project"
/workspace/scripts/create-demo-content.sh "$SKILLS" "$CFG" "$DEMO"

# ── Done ────────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════════════"
echo "  Devcontainer ready!"
echo "══════════════════════════════════════════════════════════"
echo ""
echo "Quick start:"
echo "  ss status               # check current state"
echo "  ss list                 # see all skills"
echo "  ui                      # global-mode dashboard → :5173"
echo "  ui -p                   # project-mode dashboard → :5173"
echo "  ui stop                 # stop dashboard"
echo "  docs                    # documentation site → :3000"
echo "  docs stop               # stop docs"
echo ""
echo "Private repos:"
echo "  credential-helper status                  # check auth state"
echo "  eval \"\$(credential-helper --eval off)\"   # disable all auth"
echo "  eval \"\$(credential-helper --eval on)\"    # restore all auth"
echo ""
echo "Audit playground:"
echo "  ss audit                # scan all skills, see findings"
echo "  cd ~/demo-project && ss audit  # project-level scan"
