# CLI E2E Runbook: Agents Commands

Validates all agent-related CLI commands: list, sync, status, diff,
collect, uninstall, trash, update, backup, and doctor.

**Origin**: v0.17.0 — agents support added as a new resource kind alongside skills.

## Scope

- Agent CRUD lifecycle (create source → sync → uninstall → trash → restore)
- Kind filter: `agents`, `all`, default (skills-only)
- JSON output for all commands that support it
- Diff and collect workflows
- Backup and restore round-trip
- Doctor agent checks
- Update with no tracked agents (local-only)

## Environment

Run inside devcontainer via mdproof (no ssenv wrapper needed).
All commands use `-g` to force global mode since `/workspace/.skillshare/` triggers project mode auto-detection.

## Steps

### 1. Setup: init global config and create agent source files

```bash
ss init -g --force --no-copy --all-targets --no-git --no-skill
AGENTS_DIR=~/.config/skillshare/agents
mkdir -p "$AGENTS_DIR"
cat > "$AGENTS_DIR/tutor.md" <<'EOF'
---
name: tutor
description: A tutoring agent
---
# Tutor Agent
Helps with learning.
EOF
cat > "$AGENTS_DIR/reviewer.md" <<'EOF'
---
name: reviewer
description: A code review agent
---
# Reviewer Agent
Reviews code for quality.
EOF
cat > "$AGENTS_DIR/debugger.md" <<'EOF'
---
name: debugger
description: A debugging agent
---
# Debugger Agent
Helps debug issues.
EOF
ls "$AGENTS_DIR"
```

Expected:
- exit_code: 0
- tutor.md
- reviewer.md
- debugger.md

### 2. List agents — shows source agents

```bash
ss list agents --no-tui -g
```

Expected:
- exit_code: 0
- tutor
- reviewer
- debugger

### 3. List agents — JSON includes kind field

```bash
ss list agents --json -g
```

Expected:
- exit_code: 0
- jq: length == 3
- jq: all(.[]; .kind == "agent")
- jq: [.[].name] | sort | . == ["debugger","reviewer","tutor"]

### 4. List default — skills only, no agents

```bash
ss list --json -g
```

Expected:
- exit_code: 0
- Not tutor
- Not reviewer

### 5. Sync agents — creates symlinks

```bash
ss sync agents -g
```

Expected:
- exit_code: 0
- regex: linked|synced

Verify:

```bash
CLAUDE_AGENTS=~/.claude/agents
test -L "$CLAUDE_AGENTS/tutor.md" && echo "tutor: symlinked" || echo "tutor: MISSING"
test -L "$CLAUDE_AGENTS/reviewer.md" && echo "reviewer: symlinked" || echo "reviewer: MISSING"
test -L "$CLAUDE_AGENTS/debugger.md" && echo "debugger: symlinked" || echo "debugger: MISSING"
```

Expected:
- exit_code: 0
- tutor: symlinked
- reviewer: symlinked
- debugger: symlinked
- Not MISSING

### 6. Sync agents — dry-run JSON shows no errors

```bash
ss sync agents --dry-run --json -g
```

Expected:
- exit_code: 0

### 7. Sync default — does NOT sync agents to unconfigured targets

```bash
CURSOR_AGENTS=~/.cursor/agents
rm -rf "$CURSOR_AGENTS" 2>/dev/null || true
ss sync -g
test -d "$CURSOR_AGENTS" && echo "cursor agents dir: EXISTS" || echo "cursor agents dir: not created"
```

Expected:
- exit_code: 0
- cursor agents dir: not created

### 8. Sync all — syncs both skills and agents

```bash
ss sync all -g
```

Expected:
- exit_code: 0

### 9. Status agents — text output

```bash
ss status agents -g
```

Expected:
- exit_code: 0
- regex: [Aa]gent
- regex: [Ss]ource

### 10. Status agents — JSON output

```bash
ss status agents --json -g
```

Expected:
- exit_code: 0
- jq: .agents.exists == true
- jq: .agents.count == 3

### 11. Status all — includes both skills and agents

```bash
ss status all --json -g
```

Expected:
- exit_code: 0
- jq: .agents != null
- jq: .agents.count == 3

### 12. Diff agents — no drift after sync

```bash
ss diff agents --no-tui -g
```

Expected:
- exit_code: 0

### 13. Diff agents — JSON output

```bash
ss diff agents --json -g
```

Expected:
- exit_code: 0

### 14. Collect agents — no local agents to collect

```bash
ss collect agents --force -g
```

Expected:
- exit_code: 0
- regex: [Nn]o local agents

### 15. Collect agents — collects a local agent file

```bash
CLAUDE_AGENTS=~/.claude/agents
mkdir -p "$CLAUDE_AGENTS"
rm -f "$CLAUDE_AGENTS/local-agent.md"
cat > "$CLAUDE_AGENTS/local-agent.md" <<'EOF'
---
name: local-agent
description: A locally created agent
---
# Local Agent
Created directly in target.
EOF
ss collect agents --force -g
```

Expected:
- exit_code: 0
- regex: [Cc]ollected

Verify:

```bash
AGENTS_DIR=~/.config/skillshare/agents
test -f "$AGENTS_DIR/local-agent.md" && echo "local-agent: collected to source" || echo "local-agent: NOT IN SOURCE"
```

Expected:
- exit_code: 0
- local-agent: collected to source
- Not NOT IN SOURCE

### 16. Uninstall agents — force remove single agent

```bash
ss uninstall agents local-agent --force -g
```

Expected:
- exit_code: 0
- regex: [Rr]emov|local-agent

### 17. Verify agent was removed by JSON uninstall (step 16)

```bash
AGENTS_DIR=~/.config/skillshare/agents
test -f "$AGENTS_DIR/local-agent.md" && echo "FAIL: still exists" || echo "local-agent: removed"
```

Expected:
- exit_code: 0
- local-agent: removed
- Not FAIL

### 18. Trash agents — list shows uninstalled agent

```bash
ss trash agents list --no-tui -g
```

Expected:
- exit_code: 0
- local-agent

### 19. Trash agents — restore from trash

```bash
ss trash agents restore local-agent -g
```

Expected:
- exit_code: 0
- regex: [Rr]estor

Verify:

```bash
AGENTS_DIR=~/.config/skillshare/agents
test -f "$AGENTS_DIR/local-agent.md" && echo "local-agent: restored" || echo "FAIL: not restored"
```

Expected:
- exit_code: 0
- local-agent: restored
- Not FAIL

### 20. Uninstall agents --all

```bash
ss uninstall agents --all --force -g
```

Expected:
- exit_code: 0
- regex: [Rr]emov|[Uu]ninstall

Verify:

```bash
AGENTS_DIR=~/.config/skillshare/agents
COUNT=$(ls "$AGENTS_DIR"/*.md 2>/dev/null | wc -l | tr -d ' ')
echo "Remaining agents: $COUNT (expected: 0)"
```

Expected:
- exit_code: 0
- Remaining agents: 0 (expected: 0)

### 21. Uninstall agents — validation errors

```bash
ss uninstall agents -g 2>&1 || true
```

Expected:
- regex: name|--all|required|specify

### 22. Sync agents after uninstall — targets cleaned

```bash
ss sync agents -g
CLAUDE_AGENTS=~/.claude/agents
COUNT=$(ls "$CLAUDE_AGENTS"/*.md 2>/dev/null | wc -l | tr -d ' ')
echo "Remaining symlinks: $COUNT"
```

Expected:
- exit_code: 0
- regex: prun|[Nn]o agents

### 23. Update agents — no agents found

```bash
ss update agents --all -g
```

Expected:
- regex: [Nn]o agents|[Nn]o project agents

### 24. Re-create agents and test update — local only

```bash
AGENTS_DIR=~/.config/skillshare/agents
mkdir -p "$AGENTS_DIR"
cat > "$AGENTS_DIR/helper.md" <<'EOF'
---
name: helper
description: A helper agent
---
# Helper
EOF
ss update agents --all -g
```

Expected:
- regex: local|no tracked|up to date|[Nn]o agents

### 25. Update agents — --group not supported

```bash
ss update agents --group mygroup -g 2>&1 || true
```

Expected:
- regex: not supported|--group

### 26. Backup agents

```bash
ss sync agents -g
ss backup agents -g
```

Expected:
- exit_code: 0
- regex: [Bb]ackup|created|nothing

### 27. Backup agents — list shows backup entries

```bash
ss backup --list -g
```

Expected:
- exit_code: 0

### 28. Doctor — includes agent checks

```bash
ss doctor -g
```

Expected:
- exit_code: 0
- regex: [Aa]gent

### 29. List all — shows both skills and agents

```bash
ss list all --json -g
```

Expected:
- exit_code: 0
- jq: map(select(.kind == "agent")) | length > 0

### 30. Cleanup remaining agents

```bash
ss uninstall agents --all --force -g 2>/dev/null || true
ss sync agents -g 2>/dev/null || true
```

Expected:
- exit_code: 0

## Pass Criteria

- [ ] `list agents` shows only agents, not skills
- [ ] `list agents --json` includes `kind: "agent"` for all entries
- [ ] Default `list` (no kind) excludes agents
- [ ] `sync agents` creates symlinks in agent target directories
- [ ] `sync agents --dry-run` makes no changes
- [ ] Default `sync` does NOT sync agents
- [ ] `sync all` syncs both skills and agents
- [ ] `status agents` shows agent source and target info
- [ ] `status agents --json` returns structured agent data
- [ ] `diff agents` shows drift status
- [ ] `collect agents` collects local agent files to source
- [ ] `uninstall agents <name> --force` moves agent to trash
- [ ] `uninstall agents --all --force` removes all agents
- [ ] `uninstall agents` without name or --all → validation error
- [ ] `trash agents list` shows trashed agents
- [ ] `trash agents restore <name>` restores agent from trash
- [ ] `update agents --all` handles no-agents and local-only cases
- [ ] `update agents --group` → not supported error
- [ ] `backup agents` creates agent backup
- [ ] `doctor` includes agent diagnostic checks
- [ ] `list all --json` returns mixed skills + agents
