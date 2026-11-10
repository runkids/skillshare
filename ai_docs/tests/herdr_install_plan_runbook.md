# CLI E2E Runbook: Herdr Install Plan

Verifies that the panel offers the same skill paths the CLI installs, rather than target copies.

**Origin**: [#838](https://github.com/runkids/skillshare/issues/838) — false install success and mismatched discovery.

## Scope

- CLI-owned discovery excludes target copies and applies `.skillignore`.
- The panel uses canonical names and paths, including generic-directory frontmatter names.
- A selected plan leads to the same installed content; dry-run does not install.
- Old CLI output without a plan is rejected, and non-zero install exits are failures.

## Environment

Run inside the devcontainer with mdproof's per-runbook HOME isolation. Requires Git, Node and jq.
All fixtures are under the isolated HOME. No network access or running Herdr window is required.

## Steps

### 1. Preview, parse, audit and install the selected skill

```bash
set -euo pipefail
repo="$HOME/issue838-repo"
mkdir -p "$repo/.claude/skills/design" "$repo/cli/assets/design" "$repo/skills" "$repo/ignored"
printf '# Target copy\n' > "$repo/.claude/skills/design/SKILL.md"
printf '# Authored design\n' > "$repo/cli/assets/design/SKILL.md"
printf '%s\n' '---' 'name: Custom Tool' '---' '# Authored tool' > "$repo/skills/SKILL.md"
printf '# Ignored\n' > "$repo/ignored/SKILL.md"
printf 'ignored/\n' > "$repo/.skillignore"
git -C "$repo" init -q
git -C "$repo" -c user.email=test@example.com -c user.name=Test add .
git -C "$repo" -c user.email=test@example.com -c user.name=Test commit -qm fixture
/workspace/bin/skillshare install "file://$repo" --dry-run --json --kind skill -g > "$HOME/issue838-plan.json"
jq -e '.skills == [] and .failed == [] and .planned == [{"name":"design","path":"cli/assets/design"},{"name":"custom-tool","path":"skills"}]' "$HOME/issue838-plan.json" >/dev/null
test ! -f "$HOME/.config/skillshare/skills/design/SKILL.md"
node --input-type=module <<'JS'
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseDiscover, installResult } from '/workspace/herdr/skillshare/mods/skillshare/model.js';
const plan = readFileSync(`${process.env.HOME}/issue838-plan.json`, 'utf8');
const result = parseDiscover(`@dir /fixture\n${plan}`);
assert.equal(result.error, null);
assert.deepEqual(result.choices[0], { name: 'design', path: 'cli/assets/design' });
assert.match(parseDiscover('@dir /fixture\n{"skills":[]}').error, /upgrade/i);
assert.ok(installResult({code: 1, stdout: '{"skills":[],"failed":[]}'}).error);
JS
/workspace/bin/skillshare audit "$repo/cli/assets/design" --json -g >/dev/null
/workspace/bin/skillshare install "file://$repo" --skill design --json --kind skill -g > "$HOME/issue838-result.json"
jq -e '.skills == ["design"] and .failed == [] and (has("planned") | not)' "$HOME/issue838-result.json" >/dev/null
cmp "$repo/cli/assets/design/SKILL.md" "$HOME/.config/skillshare/skills/design/SKILL.md"
test ! -f "$HOME/.config/skillshare/skills/custom-tool/SKILL.md"
printf 'INSTALL_PLAN_OK\n'
```

Expected:
- exit_code: 0
- INSTALL_PLAN_OK

## Pass Criteria

- All steps pass.
- Only the selected authored skill is installed, with the same content the panel planned and audited.
- Legacy CLI output cannot produce a false install success or an unsafe fallback pick list.
