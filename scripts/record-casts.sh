#!/usr/bin/env bash
# record-casts.sh — Record asciinema casts inside playground container.
#
# Usage:
#   1. Start playground:  make playground   (or: ./scripts/sandbox.sh up)
#   2. Enter shell:       ./scripts/sandbox.sh shell
#   3. Run this script:   ./scripts/record-casts.sh
#   4. Copy casts out:    docker cp skillshare-playground:/tmp/casts/ website/static/casts/
#
# Prerequisites (inside container):
#   pip3 install asciinema   (or: apt-get install -y asciinema)
#
# Each recording runs a pre-defined command sequence with controlled timing.
set -euo pipefail

CAST_DIR="/tmp/casts"
COLS=120
ROWS=30
mkdir -p "$CAST_DIR"

# Ensure asciinema is available
if ! command -v asciinema &>/dev/null; then
    echo "Installing asciinema..."
    pip3 install asciinema 2>/dev/null || {
        apt-get update -qq && apt-get install -y -qq asciinema
    }
fi

record_command() {
    local name="$1"
    local cmd="$2"
    local desc="${3:-}"

    echo "Recording: $name ($desc)"

    # Use asciinema in non-interactive mode
    asciinema rec "$CAST_DIR/$name.cast" \
        --cols "$COLS" --rows "$ROWS" \
        --title "$desc" \
        --command "$cmd" \
        --overwrite

    echo "  → $CAST_DIR/$name.cast"
}

echo "=== Skillshare Cast Recorder ==="
echo "Output: $CAST_DIR/"
echo ""

# 1. Init
record_command "init" \
    "skillshare init --no-copy --all-targets --no-git --skill" \
    "skillshare init"

# 2. Install a skill
record_command "install" \
    "skillshare install runkids/my-skills 2>&1 || true" \
    "skillshare install"

# 3. List skills (plain mode — TUI doesn't work in non-interactive)
record_command "list" \
    "skillshare list --no-tui" \
    "skillshare list"

# 4. Sync
record_command "sync" \
    "skillshare sync" \
    "skillshare sync"

# 5. Status
record_command "status" \
    "skillshare status" \
    "skillshare status"

# 6. Audit
record_command "audit" \
    "skillshare audit" \
    "skillshare audit"

# 7. Check for updates
record_command "check" \
    "skillshare check 2>&1 || true" \
    "skillshare check"

echo ""
echo "=== Done ==="
echo "Recorded $(ls "$CAST_DIR"/*.cast 2>/dev/null | wc -l) casts in $CAST_DIR/"
echo ""
echo "To copy to website:"
echo "  docker cp skillshare-playground:$CAST_DIR/. website/static/casts/"
