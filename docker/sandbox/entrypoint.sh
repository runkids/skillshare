#!/bin/bash
# Docker entrypoint: populate UI cache from pre-built /ui-dist if available.
# This allows "skillshare ui" to serve the full React SPA in the playground
# even though the binary version is "dev" (no runtime download needed).
if [ -d /ui-dist ] && [ -n "$HOME" ]; then
  cache_dir="$HOME/.cache/skillshare/ui/dev"
  if [ ! -f "$cache_dir/index.html" ]; then
    mkdir -p "$cache_dir"
    cp -r /ui-dist/* "$cache_dir/"
  fi
fi
exec "$@"
