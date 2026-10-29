# CLI E2E Runbook: Agent skill variants

Verify logical counts, account routing, ambiguity, explicit selection and pruning.

**Origin**: PR #649 review — agent variants in tracked repositories.

## Scope

- Account targets preserve the selected agent's bytes in merge and copy modes.
- Init and status count logical skills separately from physical variants.
- Project ambiguity skips automatic matches; an explicit include resolves it.
- Repeat sync preserves selected entries; changing filters prunes managed entries.

## Environment

Run with a freshly built `skillshare` on PATH inside a fresh ssenv in the devcontainer.
Each step creates its own fixture under the isolated `$HOME`; no network is needed.

## Steps

### 1. Init, account routing and pruning

```bash
set -euo pipefail
fixture=$(mktemp -d "$HOME/variants-account.XXXXXX")
export SKILLSHARE_CONFIG="$fixture/config.yaml"
mkdir -p "$fixture/producer/_repo/skills" "$fixture/source"
for agent in claude codex; do
  for skill in fix review; do
    mkdir -p "$fixture/producer/_repo/skills/$agent/$skill"
    printf '# %s %s\n' "$agent" "$skill" > "$fixture/producer/_repo/skills/$agent/$skill/SKILL.md"
  done
done
git -C "$fixture/producer" init -q -b main
git -C "$fixture/producer" add .
git -C "$fixture/producer" -c user.name=Test -c user.email=test@example.invalid commit -qm fixture
skillshare init -g --remote "file://$fixture/producer" --no-skill --source "$fixture/source" > "$fixture/init.txt"
python3 - "$fixture/init.txt" <<'PY'
import sys
text = open(sys.argv[1]).read()
assert '2 skills' in text and '4 skills' not in text, text
PY
skillshare status -g --json > "$fixture/status.json"
python3 - "$fixture/status.json" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
assert (s['skill_count'], s['variant_count']) == (2, 4), s
PY
for mode in merge copy; do
  cat > "$SKILLSHARE_CONFIG" <<YAML
source: $fixture/source
mode: $mode
targets:
  codex-work:
    agent: codex
    config_dir: $fixture/account-$mode
YAML
  skillshare sync -g
  skillshare sync -g
  cmp "$fixture/source/_repo/skills/codex/fix/SKILL.md" "$fixture/account-$mode/skills/_repo__skills__codex__fix/SKILL.md"
  test ! -e "$fixture/account-$mode/skills/_repo__skills__claude__fix"
  python3 - "$SKILLSHARE_CONFIG" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
p.write_text(p.read_text() + "    exclude: ['*codex*']\n")
PY
  skillshare sync -g
  test ! -e "$fixture/account-$mode/skills/_repo__skills__codex__fix"
done
printf 'account variants PASS\n'
```

Expected:
- exit_code: 0
- account variants PASS

### 2. Project ambiguity and explicit selection

```bash
set -euo pipefail
fixture=$(mktemp -d "$HOME/variants-project.XXXXXX")
for mode in merge copy; do
  project="$fixture/$mode"
  mkdir -p "$project/.skillshare/skills/_repo/skills"
  for agent in claude claude-code codex; do
    mkdir -p "$project/.skillshare/skills/_repo/skills/$agent/fix"
    printf '# %s\n' "$agent" > "$project/.skillshare/skills/_repo/skills/$agent/fix/SKILL.md"
  done
  cat > "$project/.skillshare/config.yaml" <<YAML
targets:
  - name: claude
    mode: $mode
YAML
  cd "$project"
  skillshare sync -p
  test ! -e .claude/skills/_repo__skills__claude__fix
  test ! -e .claude/skills/_repo__skills__claude-code__fix
  cat > .skillshare/config.yaml <<YAML
targets:
  - name: claude
    mode: $mode
    include: ['_repo__skills__claude__fix']
YAML
  skillshare sync -p
  cmp .skillshare/skills/_repo/skills/claude/fix/SKILL.md .claude/skills/_repo__skills__claude__fix/SKILL.md
  test ! -e .claude/skills/_repo__skills__claude-code__fix
done
printf 'project variants PASS\n'
```

Expected:
- exit_code: 0
- project variants PASS

## Pass Criteria

Both steps exit successfully, all file/JSON assertions pass, and both PASS markers appear.
