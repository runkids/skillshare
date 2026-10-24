# CLI E2E Runbook: Project registrations

## Scope

Verify the global `project` CLI: defaults, selection, resource settings, previews,
JSON failures, saved partial sync and retry, and merge/copy/symlink removal.
Tests use local fixture skills and agents; no network or external repositories.
Native Windows link behavior is outside this Linux acceptance runbook.

## Environment

Run in the devcontainer with a freshly built `/workspace/bin/skillshare`, mdproof,
and jq. Create a fresh ssenv **without** `--init` and preserve it for inspection.
Each step defines its own fixture under the isolated HOME and replaces only that
environment's config. Variables never cross Markdown blocks. Expected failures
retain exit 1 and JSON stdout. Do not run against your normal HOME.

```text
# Container commands; replace the unique environment name for every run.
cd /workspace
go build -o bin/skillshare ./cmd/skillshare
/workspace/.devcontainer/bin/ssenv create project-command-UNIQUE
/workspace/.devcontainer/bin/ssenv enter project-command-UNIQUE -- mdproof --isolation shared --build true --setup true --teardown true --report json /workspace/ai_docs/tests/project_command_runbook.md
```

The narrow mdproof overrides bypass the shared config's initialization/teardown
because this runbook supplies complete fixtures. They preserve the fresh ssenv
and do not change `ai_docs/tests/mdproof.json`. No permission-bit failure tricks
are used: a dangling target-parent link blocks sync even when run as root.

## Steps

### 1. List defaults to the global project registry

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project --json
```

Expected:
- exit_code: 0
- jq: .projects == []

### 2. Add and sync preview leaves config and targets unchanged

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
cp "$XDG_CONFIG_HOME/skillshare/config.yaml" "$fixture/before.yaml"
ss project add "$root" --target claude --agents --sync --dry-run --json > "$fixture/result.json"
cmp "$fixture/before.yaml" "$XDG_CONFIG_HOME/skillshare/config.yaml"
test ! -e "$root/.claude"
cat "$fixture/result.json"
```

Expected:
- exit_code: 0
- jq: .action == "add" and .applied == false
- jq: .project.name == "app" and .project.agents != null
- jq: .sync.failed == [] and (.sync.results | length) > 0

### 3. Register defaults and show by explicit root

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$root" --target claude,codex --name work --json >/dev/null
ss project show "$root" --json
```

Expected:
- exit_code: 0
- jq: .project.name == "work" and .project.customName == "work"
- jq: .project.targets == ["claude", "codex"]
- jq: .project.skills != null and .project.agents == null
- jq: .project.missing == false and .project.hasOwnConfig == false

### 4. Edit replaces target and filter lists and enables agents

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$root" --target claude,codex --skills-include alpha --json >/dev/null
ss project edit app --target claude --skills-include beta --skills-exclude alpha --agents-mode copy --agents-include helper --skills-naming standard --json
```

Expected:
- exit_code: 0
- jq: .applied == true and .project.targets == ["claude"]
- jq: .project.skills.include == ["beta"] and .project.skills.exclude == ["alpha"]
- jq: .project.skills.targetNaming == "standard"
- jq: .project.agents.mode == "copy" and .project.agents.include == ["helper"]

### 5. Empty values reset overrides and unchanged edit reports a no-op

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$root" --target claude --name work --skills-mode copy --skills-naming prefixed --skills-include alpha --agents --json >/dev/null
ss project edit work --name= --skills-mode= --skills-naming= --skills-include= --no-agents --json > "$fixture/reset.json"
jq -e '.project.name == "app" and .project.skills.mode == "" and .project.skills.targetNaming == null and .project.skills.include == [] and .project.agents == null' "$fixture/reset.json" >/dev/null
ss project edit app --json
```

Expected:
- exit_code: 0
- jq: .applied == false and .unchanged == true

### 6. Invalid agent symlink filters are rejected before saving

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$root" --target claude --json >/dev/null
cp "$XDG_CONFIG_HOME/skillshare/config.yaml" "$fixture/before.yaml"
set +e
ss project edit app --agents-mode symlink --agents-include helper --json > "$fixture/result.json"
code=$?
set -e
cmp "$fixture/before.yaml" "$XDG_CONFIG_HOME/skillshare/config.yaml"
cat "$fixture/result.json"
exit "$code"
```

Expected:
- exit_code: 1
- jq: .error | contains("agent include/exclude")

### 7. Project config inside the folder does not change command scope

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
mkdir -p "$root/.skillshare"
printf '%s\n' 'targets: [claude]' > "$root/.skillshare/config.yaml"
cp "$root/.skillshare/config.yaml" "$fixture/project-before.yaml"
cd "$root"
ss project add . --target claude --json >/dev/null
cmp "$root/.skillshare/config.yaml" "$fixture/project-before.yaml"
ss project list --json
```

Expected:
- exit_code: 0
- jq: (.projects | length) == 1
- jq: .projects[0].name == "app" and .projects[0].hasOwnConfig == true

### 8. Explicit project mode is rejected as JSON

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project list -p --json
```

Expected:
- exit_code: 1
- jq: .error == "projects belong in the global config"

### 9. Ambiguous name and relative root are rejected

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$root" --target claude --name named --json >/dev/null
mkdir -p "$fixture/named"
ss project add "$fixture/named" --target claude --name other --json >/dev/null
cd "$fixture"
ss project show named --json
```

Expected:
- exit_code: 1
- jq: .error | contains("ambiguous")

### 10. Saved add reports partial sync and leaves the other project untouched

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$fixture/other" --target claude --json >/dev/null
ln -s "$fixture/absent-parent" "$root/.agents"
set +e
ss project add "$root" --target claude,codex --sync --json > "$fixture/result.json"
code=$?
set -e
ss project show app --json >/dev/null
test -L "$root/.claude/skills/alpha"
test ! -e "$fixture/other/.claude"
cat "$fixture/result.json"
exit "$code"
```

Expected:
- exit_code: 1
- jq: .action == "add" and .applied == true
- jq: (.sync.failed | length) > 0 and (.sync.results | length) > 0

### 11. Standalone sync retries after the target conflict is removed

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ln -s "$fixture/absent-parent" "$root/.agents"
set +e
ss project add "$root" --target claude,codex --sync --json >/dev/null
code=$?
set -e
test "$code" -eq 1
unlink "$root/.agents"
ss project sync app --json > "$fixture/result.json"
test -L "$root/.agents/skills/alpha"
test -L "$root/.claude/skills/beta"
cat "$fixture/result.json"
```

Expected:
- exit_code: 0
- jq: .action == "sync" and .applied == null
- jq: .sync.failed == [] and (.sync.results | length) > 0

### 12. Merge removal preview is read-only and removal copies back skills, keeping agents

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$root" --target claude --agents --sync --json >/dev/null
cp "$XDG_CONFIG_HOME/skillshare/config.yaml" "$fixture/before.yaml"
ss project remove app --dry-run --json > "$fixture/preview.json"
cmp "$fixture/before.yaml" "$XDG_CONFIG_HOME/skillshare/config.yaml"
test -L "$root/.claude/skills/alpha"
jq -e '.applied == false and (.detach.copiedBack | length) > 0' "$fixture/preview.json" >/dev/null
ss project remove app --yes --json > "$fixture/result.json"
test -f "$root/.claude/skills/alpha/SKILL.md"
test ! -L "$root/.claude/skills/alpha"
test -f "$root/.claude/agents/helper.md"
ss project list --json | jq -e '.projects == []' >/dev/null
cat "$fixture/result.json"
```

Expected:
- exit_code: 0
- jq: .action == "remove" and .registration == "removed" and .applied == true
- jq: (.detach.copiedBack | length) > 0

### 13. Copy removal keeps files and removes the manifest

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$root" --target claude --skills-mode copy --sync --json >/dev/null
test -f "$root/.claude/skills/.skillshare-manifest.json"
ss project remove app --yes --json > "$fixture/result.json"
test -f "$root/.claude/skills/alpha/SKILL.md"
test ! -e "$root/.claude/skills/.skillshare-manifest.json"
cat "$fixture/result.json"
```

Expected:
- exit_code: 0
- jq: .registration == "removed" and (.detach.retained | length) > 0

### 14. Whole-folder symlink removal leaves source skills intact

```bash
set -euo pipefail
ss() { /workspace/bin/skillshare "$@"; }
fixture=$(mktemp -d "$HOME/project-command-XXXXXX")
root="$fixture/app"
mkdir -p "$root" "$fixture/other" "$fixture/skills/alpha" "$fixture/skills/beta" "$fixture/agents" "$XDG_CONFIG_HOME/skillshare"
printf '%s\n' '---' 'name: alpha' 'description: Alpha fixture' '---' '# Alpha' > "$fixture/skills/alpha/SKILL.md"
printf '%s\n' '---' 'name: beta' 'description: Beta fixture' '---' '# Beta' > "$fixture/skills/beta/SKILL.md"
printf '%s\n' '---' 'name: helper' 'description: Helper agent fixture' '---' '# Helper' > "$fixture/agents/helper.md"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<CONFIG
sources:
  skills: $fixture/skills
  agents: $fixture/agents
mode: merge
targets: {}
CONFIG
ss project add "$root" --target claude --skills-mode symlink --sync --json >/dev/null
test -L "$root/.claude/skills"
ss project remove app --yes --json > "$fixture/result.json"
test ! -L "$root/.claude/skills"
test -f "$fixture/skills/alpha/SKILL.md"
cat "$fixture/result.json"
```

Expected:
- exit_code: 0
- jq: .registration == "removed" and (.detach.removed | length) > 0

## Pass Criteria

All 14 mdproof steps pass with zero failures or skips. JSON steps satisfy their
jq assertions, expected rejection/partial-failure steps exit 1, and filesystem
checks prove preview safety, sync scope, retry, detach behavior, and source survival.
