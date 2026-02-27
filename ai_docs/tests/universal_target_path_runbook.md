# Universal Target Path Verification

Verify that the `universal` target path in `targets.yaml` matches the actual runtime path used by `npx skills` (vercel-labs/skills).

## Scope

- `internal/config/targets.yaml` — universal global_path
- `cmd/skillshare/init.go` — universal auto-injection in `detectCLIDirectories()` and `detectNewAgents()`
- Cross-validation with upstream `npx skills` CLI behavior

## Environment

- Devcontainer with network access (needs npm registry)
- `ssenv` isolated HOME
- Node.js / npm available in container
- A public GitHub repo with skills (e.g., `runkids/feature-radar`)

## Steps

### Step 1: Verify targets.yaml universal path

```bash
grep -A4 'global_name: universal' /workspace/internal/config/targets.yaml
```

**Expected:**
- `global_path` is `~/.agents/skills`
- `project_path` is `.agents/skills`

### Step 2: Install skills globally via npx skills

```bash
npx -y skills@latest add runkids/feature-radar -g --all
```

**Expected:**
- Command succeeds (exit 0)
- Output mentions installing to `~/.agents/skills/` (not `~/.config/agents/skills/`)

### Step 3: Verify skills installed to ~/.agents/skills

```bash
ls -la ~/.agents/skills/
```

**Expected:**
- Directory exists and contains installed skill directories
- Each skill has a `SKILL.md` file

### Step 4: Verify ~/.config/agents/skills was NOT created

```bash
ls ~/.config/agents/skills/ 2>&1
```

**Expected:**
- Directory does not exist or is empty
- Error message like "No such file or directory"

### Step 5: Verify npx skills list sees the skills

```bash
npx -y skills@latest list -g
```

**Expected:**
- Lists all installed skills
- Shows path as `~/.agents/skills/`
- Output includes "universal:" line listing covered agents (Amp, Cline, Codex, Cursor, etc.)

### Step 6: Verify symlinks to agent-specific directories

```bash
ls -la ~/.claude/skills/ 2>/dev/null
ls -la ~/.cursor/skills/ 2>/dev/null
```

**Expected:**
- Agent-specific directories contain symlinks pointing back to `../../.agents/skills/`
- Symlinks are valid (not broken)

### Step 7: Verify skillshare init auto-includes universal

Note: `ssenv create --init` already ran init. Remove existing config to test fresh init.

```bash
rm -rf ~/.config/skillshare
ss init --no-copy --all-targets --no-git --no-skill
cat ~/.config/skillshare/config.yaml
```

**Expected:**
- Config contains `universal:` target entry
- Universal target path resolves to `~/.agents/skills`

### Step 8: Verify skillshare sync creates universal target directory

```bash
mkdir -p ~/.config/skillshare/skills/test-skill
echo '---' > ~/.config/skillshare/skills/test-skill/SKILL.md
echo 'name: test-skill' >> ~/.config/skillshare/skills/test-skill/SKILL.md
echo '---' >> ~/.config/skillshare/skills/test-skill/SKILL.md
echo '# Test' >> ~/.config/skillshare/skills/test-skill/SKILL.md
ss sync
ls -la ~/.agents/skills/
```

**Expected:**
- `~/.agents/skills/` directory created by sync
- Contains symlink `test-skill` → source directory

### Step 9: Verify agent CLI can read skillshare-synced skill

`npx skills list -g` uses a lock file (`~/.agents/.skill-lock.json`), not directory scanning. So it will NOT show skills placed by skillshare. Instead, verify the agent directories directly.

```bash
cat ~/.claude/skills/test-skill/SKILL.md
cat ~/.agents/skills/test-skill/SKILL.md
```

**Expected:**
- Both paths resolve to the SKILL.md content ("# Test Skill")
- Confirms round-trip: skillshare sync → universal path + agent path → readable by agent CLI

## Pass Criteria

- All 9 steps pass
- Confirmed: `~/.agents/skills` is the canonical global path for universal target
- Confirmed: `~/.config/agents/skills` is NOT used by runtime `npx skills`
- Confirmed: skillshare sync to universal target is readable by agent CLIs via symlinks

## Notes

- vercel-labs/skills source code references `{configHome}/agents/skills` which suggests `~/.config/agents/skills`, but actual runtime behavior uses `~/.agents/skills`. This discrepancy was the root cause of Issue #54.
- The `npx skills add -g` command also creates symlinks from agent-specific directories (claude, cursor, etc.) back to `~/.agents/skills/`, which is the "universal" shared directory.
- `npx skills list -g` only shows skills tracked in `~/.agents/.skill-lock.json` (installed via `npx skills add`). Skills placed by external tools (like skillshare) won't appear in `npx skills list` but ARE visible to agent CLIs that read the directory.
