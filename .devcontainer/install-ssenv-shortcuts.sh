#!/usr/bin/env bash
set -euo pipefail

SHORTCUTS_SRC=/workspace/.devcontainer/ssenv-shortcuts.sh
SHORTCUTS_DEST=/etc/profile.d/ssenv-shortcuts.sh

if [ ! -f "$SHORTCUTS_SRC" ]; then
  echo "ssenv shortcuts source not found: $SHORTCUTS_SRC" >&2
  exit 0
fi

cp "$SHORTCUTS_SRC" "$SHORTCUTS_DEST"
chmod +x "$SHORTCUTS_DEST"

append_shortcuts_loader() {
  local rc="$1"
  if [ ! -f "$rc" ]; then
    return 0
  fi

  if ! grep -Fq "/etc/profile.d/ssenv-shortcuts.sh" "$rc"; then
    cat >> "$rc" <<'RC_EOF'

# skillshare: ssenv shortcuts
if [ -f /etc/profile.d/ssenv-shortcuts.sh ]; then
  . /etc/profile.d/ssenv-shortcuts.sh
fi
RC_EOF
  fi
}

for rc in /etc/bash.bashrc /root/.bashrc; do
  append_shortcuts_loader "$rc"
done
