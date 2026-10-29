#!/usr/bin/env bash
# Check the production entrypoint without building an image.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/home" "$tmp/xdg/skillshare" "$tmp/custom"
cat > "$tmp/bin/skillshare" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$INIT_LOG"
printf '%s\n' "${SKILLSHARE_CONFIG:-${XDG_CONFIG_HOME:-$HOME/.config}/skillshare/config.yaml}" >> "$INIT_LOG"
EOF
chmod +x "$tmp/bin/skillshare"

run_case() {
  local expected=$1 path=$2
  shift 2
  rm -f "$tmp/init.log"
  env -u SKILLSHARE_CONFIG -u XDG_CONFIG_HOME HOME="$tmp/home" PATH="$tmp/bin:$PATH" \
    INIT_LOG="$tmp/init.log" "$@" bash "$root/docker/production/entrypoint.sh" true
  if [[ "$expected" == init ]]; then
    [[ -f "$tmp/init.log" ]] && [[ $(sed -n '1p' "$tmp/init.log") == 'init -g --no-copy --no-git --skill' ]] && \
      [[ $(sed -n '2p' "$tmp/init.log") == "$path" ]] || { echo "Expected init at $path" >&2; exit 1; }
  else
    [[ ! -e "$tmp/init.log" ]] || { echo "Unexpected init for $path" >&2; exit 1; }
  fi
}

run_case init "$tmp/home/.config/skillshare/config.yaml"
touch "$tmp/custom/config.yaml"
run_case skip "$tmp/custom/config.yaml" SKILLSHARE_CONFIG="$tmp/custom/config.yaml"
run_case init "$tmp/custom/missing.yaml" SKILLSHARE_CONFIG="$tmp/custom/missing.yaml"
touch "$tmp/xdg/skillshare/config.yaml"
run_case skip "$tmp/xdg/skillshare/config.yaml" XDG_CONFIG_HOME="$tmp/xdg"
run_case init "$tmp/custom/missing.yaml" SKILLSHARE_CONFIG="$tmp/custom/missing.yaml" XDG_CONFIG_HOME="$tmp/xdg"
run_case init "$tmp/missing-xdg/skillshare/config.yaml" XDG_CONFIG_HOME="$tmp/missing-xdg"
mkdir -p "$tmp/home/.config/skillshare" "$tmp/other/child"
touch "$tmp/home/.config/skillshare/config.yaml"
run_case init "$tmp/missing-xdg/skillshare/config.yaml" XDG_CONFIG_HOME="$tmp/missing-xdg"
run_case skip "$tmp/custom/config.yaml" SKILLSHARE_CONFIG="$tmp/custom/config.yaml" XDG_CONFIG_HOME="$tmp/missing-xdg"
ln -s "$tmp/other/child" "$tmp/xdg/link"
run_case skip "$tmp/xdg/link/../skillshare/config.yaml" XDG_CONFIG_HOME="$tmp/xdg/link/.."
echo "Production entrypoint config checks passed"
