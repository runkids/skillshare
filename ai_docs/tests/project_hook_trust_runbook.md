# CLI E2E Runbook: Trust a project hook in one of two Codex accounts

Use a fresh devcontainer ssenv with the current Skillshare binary and Codex on PATH. Never run these steps against a real home. No Codex login or model call is needed.

## Steps

### Step 1: Verify account isolation and partial completion

```bash
set -eu
export CODEX_HOME="$HOME/.codex"
export SKILLSHARE_CONFIG="$HOME/global.yaml"
root="$HOME/repo"; mkdir -p "$root/.skillshare" "$HOME/.codex" "$HOME/.codex-2"
cd "$root"; git init -q
printf 'targets:\n  codex-2:\n    agent: codex\n    config_dir: %s/.codex-2\n    path: %s/.codex-2/skills\n' "$HOME" "$HOME" > "$SKILLSHARE_CONFIG"
printf 'targets: []\nhooks:\n  entries:\n    ready:\n      bindings:\n        codex: {events: {SessionStart: [{hooks: [{type: command, command: echo ready}]}]}}\n' > .skillshare/config.yaml
printf '[projects."%s"]\ntrust_level="trusted"\n' "$root" > "$HOME/.codex/config.toml"
# Only the default account already trusts the folder. Neither has approved the command.
skillshare hooks sync -p
if skillshare hooks sync -p --trust --json > review.json; then exit 1; fi
python3 -c 'import json; p=json.load(open("review.json")); assert p["codexTrust"][0]["command"]=="echo ready" and "codex-2" in p["codexTrustError"]'
if grep -q trusted_hash "$HOME/.codex/config.toml"; then exit 1; fi
if skillshare hooks sync -p --trust --yes --json > result.json; then exit 1; fi
python3 -c 'import json; p=json.load(open("result.json")); assert [(h["target"],h["action"]) for h in p["codexTrust"]]==[("codex","trusted")] and "codex-2" in p["codexTrustError"]'
grep -q trusted_hash "$HOME/.codex/config.toml"
if grep -qs trusted_hash "$HOME/.codex-2/config.toml"; then exit 1; fi
test ! -e .codex/config.toml
```

Expected:
- exit_code: 0

The nonzero status must distinguish partial completion from success. The checkout and skipped account gain no trust configuration.
