#!/usr/bin/env bash
# Exercise the real CLI with two isolated native homes; copy only shared config.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
binary=${SKILLSHARE_BIN:-$repo/bin/skillshare}
export SSENV_ROOT=${SSENV_ROOT:-$repo/tmp/plugin-two-hosts-envs}
export PATH="$repo/.devcontainer/bin:$PATH"
run_id="plugin-hosts-$(date +%s)-$$"
a="$run_id-a"
b="$run_id-b"
work="$repo/tmp/$run_id"
mkdir -p "$work/source/.cursor-plugin" "$work/source/skills/demo"
printf '%s\n' '{"name":"demo","version":"1.0.0"}' > "$work/source/.cursor-plugin/plugin.json"
printf '%s\n' '---' 'name: demo' 'description: harmless two-machine fixture' '---' 'Hello.' > "$work/source/skills/demo/SKILL.md"
ssenv create "$a"
ssenv create "$b"
run() { ssenv enter "$1" -- env -u SKILLSHARE_CONFIG "$binary" "${@:2}"; }
config() { ssenv enter "$1" -- bash -c 'printf "%s/skillshare/config.yaml" "$XDG_CONFIG_HOME"'; }
run "$a" init -g --no-git --no-skill --all-targets
run "$b" init -g --no-git --no-skill --all-targets
cfg_a=$(config "$a")
cfg_b=$(config "$b")
run "$a" plugin add "$work/source" --target cursor --name demo --no-tui -g
cp "$cfg_a" "$cfg_b"
cp "$cfg_b" "$work/shared-before.yaml"
run "$b" plugin sync --dry-run --json -g | jq -e '.changes | length == 1 and .[0].action == "install"'
run "$b" plugin sync --no-tui -g
run "$b" plugin sync --dry-run --json -g | jq -e 'all(.changes[]; .action == "noop")'
cmp "$cfg_b" "$work/shared-before.yaml"
# The snapshots must be native paths belonging to different isolated homes.
run "$a" plugin list --json -g > "$work/a.json"
run "$b" plugin list --json -g > "$work/b.json"
jq -e --slurpfile other "$work/b.json" '.packages.demo.bindings.cursor.id != $other[0].packages.demo.bindings.cursor.id' "$work/a.json"
run "$a" plugin remove demo --no-tui -g
cp "$cfg_a" "$cfg_b"
cp "$cfg_b" "$work/shared-removed.yaml"
run "$b" plugin sync --dry-run --json -g | jq -e '.changes | length == 1 and .[0].action == "remove"'
run "$b" plugin sync --no-tui -g
run "$b" plugin sync --dry-run --json -g | jq -e '.changes | length == 0'
cmp "$cfg_b" "$work/shared-removed.yaml"
printf 'PASS: two-machine CLI lifecycle (%s, %s); artifacts: %s\n' "$a" "$b" "$work"
