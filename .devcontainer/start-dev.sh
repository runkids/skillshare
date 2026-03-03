#!/usr/bin/env bash
# Per-start container init: profile.d scripts, PATH, token detection.
# Runs on every container start (VS Code postStartCommand / devc.sh).
set -euo pipefail

# ── Profile.d: login-shell environment ───────────────────────────────
# These live on the container filesystem (not volume) and must be
# recreated after container recreation (docker compose down → up).

# Token env: .env overrides host-passed env vars.
cat > /etc/profile.d/skillshare-env.sh << 'PROFILE_EOF'
if [ -f /workspace/.devcontainer/.env ]; then
  set -a
  . /workspace/.devcontainer/.env
  set +a
fi
PROFILE_EOF

# Restore Go toolchain paths (login shell resets PATH, dropping Docker ENV values)
# and keep devcontainer command wrappers ahead of /usr/local/bin.
cat > /etc/profile.d/skillshare-path.sh << 'PROFILE_EOF'
case ":$PATH:" in
  *:/usr/local/go/bin:*) ;;
  *) export PATH="/go/bin:/usr/local/go/bin:$PATH" ;;
esac
case ":$PATH:" in
  *:/workspace/.devcontainer/bin:*) ;;
  *) export PATH="/workspace/.devcontainer/bin:/workspace/bin:$PATH" ;;
esac
PROFILE_EOF

# ── Re-assert command entrypoints and shortcuts ──────────────────────
if [ -x /workspace/.devcontainer/ensure-skillshare-linux-binary.sh ]; then
  /workspace/.devcontainer/ensure-skillshare-linux-binary.sh
fi
if [ -x /workspace/.devcontainer/install-ssenv-shortcuts.sh ]; then
  /workspace/.devcontainer/install-ssenv-shortcuts.sh
fi

# Auto-detect GITHUB_TOKEN from gh CLI if not already set
# (via .env, remoteEnv, or manual export).
if [ -z "${GITHUB_TOKEN:-}" ] && command -v gh &>/dev/null; then
  token="$(gh auth token 2>/dev/null || true)"
  if [ -n "$token" ]; then
    export GITHUB_TOKEN="$token"
    echo "GITHUB_TOKEN auto-detected from gh CLI"
  fi
fi

echo "Dev servers ready:"
echo "  ui          # global-mode dashboard → :5173"
echo "  ui -p       # project-mode dashboard → :5173"
echo "  ui stop     # stop dashboard"
echo "  docs        # documentation site → :3000"
echo "  docs stop   # stop docs"
echo "  ss ...      # instant mode (go run, no manual rebuild)"
echo "  ssenv ...   # isolated HOME environments (create/use/enter/list/reset/delete)"
echo "  ssnew X     # create + enter isolated shell"
echo "  ssuse X     # enter existing isolated shell"
echo "  ssback      # helper to leave isolated context"
echo "  sshelp      # show all ssenv shortcuts"
