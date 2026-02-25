#!/usr/bin/env bash
# Cross-compile skillshare for Windows ARM64 and copy to UTM shared folder.
# Usage: ./scripts/build-windows.sh [shared_folder]
set -euo pipefail

if [ -n "${1:-}" ]; then
	SHARED="$1"
else
	# Scan common shared folder locations
	candidates=()
	for dir in "$HOME/Downloads" "$HOME/Public" "$HOME/Desktop" "$HOME/Shared"; do
		[ -d "$dir" ] && candidates+=("$dir")
	done

	# Also check UTM shared mount points (VirtioFS typically shows up here)
	for dir in /Volumes/Shared* /Volumes/UTM*; do
		[ -d "$dir" ] 2>/dev/null && candidates+=("$dir")
	done

	if [ ${#candidates[@]} -eq 0 ]; then
		echo "✗ No shared folder found. Usage: $0 <path>" >&2
		exit 1
	elif [ ${#candidates[@]} -eq 1 ]; then
		SHARED="${candidates[0]}"
	else
		echo "Select shared folder:"
		for i in "${!candidates[@]}"; do
			printf "  %d) %s\n" $((i + 1)) "${candidates[$i]}"
		done
		printf "  q) Cancel\n"
		printf "> "
		read -r choice
		case "$choice" in
			q|Q) echo "Cancelled."; exit 0 ;;
			*) SHARED="${candidates[$((choice - 1))]}" ;;
		esac
	fi
fi

if [ ! -d "$SHARED" ]; then
	echo "✗ Directory not found: $SHARED" >&2
	exit 1
fi

echo "Building Windows ARM64..."
GOOS=windows GOARCH=arm64 go build -o bin/skillshare-arm64.exe ./cmd/skillshare/

cp bin/skillshare-arm64.exe "$SHARED/skillshare.exe"
echo "✓ $(file bin/skillshare-arm64.exe | cut -d: -f2 | xargs)"
echo "→ $SHARED/skillshare.exe ($(du -h "$SHARED/skillshare.exe" | cut -f1 | xargs))"
