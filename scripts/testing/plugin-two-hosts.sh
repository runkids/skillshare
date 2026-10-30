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
cleanup() {
  local status=$? name
  for name in "$a" "$b"; do
    if [[ -d "$SSENV_ROOT/$name" ]]; then
      if ! ssenv delete "$name" --force; then
        printf 'Failed to clean up ssenv environment: %s\n' "$name" >&2
        if (( status == 0 )); then status=1; fi
      fi
    fi
  done
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
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
# Cursor IDs are package names on both hosts; compare actual native roots.
root_a=$(ssenv enter "$a" -- bash -c 'realpath "$HOME/.cursor/plugins/local/demo"')
root_b=$(ssenv enter "$b" -- bash -c 'realpath "$HOME/.cursor/plugins/local/demo"')
test "$root_a" != "$root_b"
test -f "$root_a/.cursor-plugin/plugin.json"
test -f "$root_b/.cursor-plugin/plugin.json"
cmp "$root_a/skills/demo/SKILL.md" "$root_b/skills/demo/SKILL.md"
run "$a" plugin remove demo --no-tui -g
test ! -e "$root_a"
test -d "$root_b"
cp "$cfg_a" "$cfg_b"
cp "$cfg_b" "$work/shared-removed.yaml"
run "$b" plugin sync --dry-run --json -g | jq -e '.changes | length == 1 and .[0].action == "remove"'
run "$b" plugin sync --no-tui -g
test ! -e "$root_b"
run "$b" plugin sync --dry-run --json -g | jq -e '.changes | length == 0'
cmp "$cfg_b" "$work/shared-removed.yaml"
# OpenCode's file adapter is real; only its version probe is stubbed.
mkdir -p "$work/bin" "$work/code-source" "$work/code-next" "$work/code-native"
cat > "$work/bin/opencode" <<'MOCK'
#!/usr/bin/env bash
if [[ "$*" == --version ]]; then echo 1.18.32; else exit 1; fi
MOCK
chmod +x "$work/bin/opencode"
export PATH="$work/bin:$PATH"
for dir in code-source code-next code-native; do
  printf '%s\n' '{"name":"demo","main":"index.js","dependencies":{"@opencode-ai/plugin":"1"}}' > "$work/$dir/package.json"
  printf '%s\n' 'export default async () => ({})' > "$work/$dir/index.js"
done
run "$a" plugin add "$work/code-source" --target opencode --name code --no-tui -g
cp "$cfg_a" "$cfg_b"
code_config_b=$(ssenv enter "$b" -- bash -c 'printf "%s/opencode/opencode.json" "$XDG_CONFIG_HOME"')
mkdir -p "$(dirname "$code_config_b")"
jq -n --arg id "file://$work/code-native/index.js" '{plugin: [$id]}' > "$code_config_b"
run "$b" plugin import "file://$work/code-native/index.js" --from opencode --name code --no-tui -g
# Change A's declared source, then copy only the completed shared declaration.
run "$a" plugin remove code --no-tui -g
run "$a" plugin add "$work/code-next" --target opencode --name code --no-tui -g
cp "$cfg_a" "$cfg_b"
cp "$cfg_b" "$work/shared-source.yaml"
run "$b" plugin list --json -g | jq -e '.packages.code.bindings.opencode.importRetained == true'
run "$b" plugin unimport code --target opencode --dry-run --json -g | jq -e '.changes | length == 1 and .[0].action == "remove"'
jq -e '.plugin | length == 1' "$code_config_b"
run "$b" plugin unimport code --target opencode --no-tui -g
cmp "$cfg_b" "$work/shared-source.yaml"
jq -e '.plugin | length == 0' "$code_config_b"
run "$b" plugin sync code --no-tui -g
run "$b" plugin list --json -g | jq -e '.packages.code.bindings.opencode | .source != null and .imported != true and .importRetained != true'
cmp "$cfg_b" "$work/shared-source.yaml"
cp "$cfg_b" "$cfg_a"
run "$a" plugin sync code --dry-run --json -g | jq -e '.changes | length == 1 and .[0].action == "noop"'
run "$a" plugin sync code --no-tui -g
run "$a" plugin list --json -g | jq -e '.packages.code.bindings.opencode | .source != null and .imported != true'
printf 'PASS: two-machine CLI lifecycle (%s, %s); artifacts: %s\n' "$a" "$b" "$work"
