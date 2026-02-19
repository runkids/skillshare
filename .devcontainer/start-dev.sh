#!/usr/bin/env bash
# Auto-start dev servers when devcontainer starts.
# Delegates to dev-servers manager for process tracking.
exec /workspace/.devcontainer/dev-servers.sh start
