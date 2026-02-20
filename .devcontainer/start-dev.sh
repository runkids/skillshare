#!/usr/bin/env bash
# Print available commands on container start.
set -euo pipefail

echo "Dev servers ready:"
echo "  ui          # global-mode dashboard → :5173"
echo "  ui -p       # project-mode dashboard → :5173"
echo "  ui stop     # stop dashboard"
echo "  docs        # documentation site → :3000"
echo "  docs stop   # stop docs"
echo "  ss ...      # instant mode (go run, no manual rebuild)"
