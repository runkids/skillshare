#!/bin/bash
# Production entrypoint: initialise config if missing, then exec CMD.
set -e

config_path=${SKILLSHARE_CONFIG:-}
if [ -z "$config_path" ]; then
  # Go's filepath.Join cleans the base path without resolving symlinks.
  config_path=$(realpath -ms "${XDG_CONFIG_HOME:-$HOME/.config}/skillshare/config.yaml")
fi
if [ ! -f "$config_path" ]; then
  echo "First run: initialising skillshare config..."
  skillshare init -g --no-copy --no-git --skill 2>/dev/null || true
fi

exec "$@"
