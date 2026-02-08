#!/bin/bash
# Docker entrypoint: ensure pre-built frontend assets are available for go:embed.
# The host volume mount (.:/workspace) may not include internal/server/dist/
# (gitignored), so we copy from the Docker-built /ui-dist if needed.
if [ -d /ui-dist ] && [ ! -f internal/server/dist/index.html ]; then
  mkdir -p internal/server/dist
  cp -r /ui-dist/* internal/server/dist/
fi
exec "$@"
