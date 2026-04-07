# CLI E2E Runbook: Install Mixed Repo (Skills + Agents)

Validates that `skillshare install` from a repo containing both skills and agents
installs skills to the skills source and agents to the agents source, then sync
distributes agents only to targets with an agents path configured.

**Origin**: Bug fix — agents from mixed repos were incorrectly ignored or installed to skills dir.

## Scope

- Mixed repo install: skills go to `~/.config/skillshare/skills/`, agents go to `~/.config/skillshare/agents/`
- Pure agent repo install: agents go to agents dir
- Sync agents: targets with agents path receive agents, targets without are skipped with warning
- Project mode: agents go to `.skillshare/agents/`, not global agents dir

## Environment

Run inside devcontainer via mdproof (no ssenv wrapper needed).
All global commands use `-g` to force global mode.

## Steps

### 1. Create mixed git repo with skills and agents

```bash
rm -rf /tmp/mixed-repo
mkdir -p /tmp/mixed-repo/skills/demo-skill /tmp/mixed-repo/agents
cat > /tmp/mixed-repo/skills/demo-skill/SKILL.md <<'EOF'
---
name: demo-skill
---
# Demo Skill
A demo skill for testing.
EOF
cat > /tmp/mixed-repo/agents/demo-agent.md <<'EOF'
---
name: demo-agent
description: A demo agent
---
# Demo Agent
A demo agent for testing.
EOF
cd /tmp/mixed-repo && git init && git config user.email "test@test.com" && git config user.name "test" && git add -A && git commit -m "init" 2>&1
ls skills/demo-skill/SKILL.md agents/demo-agent.md
```

Expected:
- exit_code: 0
- SKILL.md
- demo-agent.md

### 2. Install mixed repo — both skills and agents found

```bash
ss install -g file:///tmp/mixed-repo --yes --force
```

Expected:
- exit_code: 0
- regex: 1 skill\(s\), 1 agent\(s\)
- Installed: demo-skill
- Installed agent: demo-agent

### 3. Verify skill in skills source, agent in agents source

```bash
SKILLS_DIR=~/.config/skillshare/skills
AGENTS_DIR=~/.config/skillshare/agents
test -f "$SKILLS_DIR/demo-skill/SKILL.md" && echo "skill: in skills dir" || echo "skill: MISSING"
test -f "$AGENTS_DIR/demo-agent.md" && echo "agent: in agents dir" || echo "agent: MISSING"
test -f "$SKILLS_DIR/demo-agent.md" && echo "agent: WRONG in skills dir" || echo "agent: not in skills dir (correct)"
```

Expected:
- exit_code: 0
- skill: in skills dir
- agent: in agents dir
- agent: not in skills dir (correct)
- Not MISSING
- Not WRONG

### 4. Sync all — agents go to targets with agents path

```bash
ss sync all -g
```

Expected:
- exit_code: 0

### 5. Verify agents synced to claude (has agents path) but not to targets without

```bash
CLAUDE_AGENTS=~/.claude/agents
test -L "$CLAUDE_AGENTS/demo-agent.md" && echo "claude: agent synced" || echo "claude: agent MISSING"
```

Expected:
- exit_code: 0
- claude: agent synced
- Not MISSING

### 6. Sync agents — warning lists targets without agents path

```bash
ss sync agents -g 2>&1
```

Expected:
- exit_code: 0
- regex: skipped for agents

### 7. Create pure agent repo and install

```bash
rm -rf /tmp/agent-only-repo
mkdir -p /tmp/agent-only-repo/agents
cat > /tmp/agent-only-repo/agents/helper.md <<'EOF'
---
name: helper
description: A helper agent
---
# Helper Agent
EOF
cd /tmp/agent-only-repo && git init && git config user.email "test@test.com" && git config user.name "test" && git add -A && git commit -m "init" 2>&1
ss install -g file:///tmp/agent-only-repo --yes --force
```

Expected:
- exit_code: 0
- regex: 1 agent\(s\)
- helper

### 8. Verify pure agent repo installed to agents dir

```bash
AGENTS_DIR=~/.config/skillshare/agents
test -f "$AGENTS_DIR/helper.md" && echo "helper: in agents dir" || echo "helper: MISSING"
SKILLS_DIR=~/.config/skillshare/skills
test -d "$SKILLS_DIR/helper" && echo "helper: WRONG in skills dir" || echo "helper: not in skills dir (correct)"
```

Expected:
- exit_code: 0
- helper: in agents dir
- helper: not in skills dir (correct)
- Not MISSING
- Not WRONG

### 9. Project mode — install mixed repo to project agents dir

```bash
rm -rf /tmp/test-project
mkdir -p /tmp/test-project
cd /tmp/test-project
ss init -p --targets claude 2>&1
ss install -p file:///tmp/mixed-repo --yes --force 2>&1
```

Expected:
- exit_code: 0
- Installed: demo-skill
- Installed agent: demo-agent

### 10. Verify project mode paths

```bash
cd /tmp/test-project
test -f .skillshare/skills/demo-skill/SKILL.md && echo "project skill: correct" || echo "project skill: MISSING"
test -f .skillshare/agents/demo-agent.md && echo "project agent: correct" || echo "project agent: MISSING"
GLOBAL_AGENTS=~/.config/skillshare/agents
test -f "$GLOBAL_AGENTS/demo-agent.md" && echo "global agent: EXISTS (wrong for project install)" || echo "global agent: not there (correct)"
```

Expected:
- exit_code: 0
- project skill: correct
- project agent: correct
- Not MISSING

### 11. Cleanup

```bash
rm -rf /tmp/mixed-repo /tmp/agent-only-repo /tmp/test-project
ss uninstall demo-skill --force -g 2>/dev/null || true
ss uninstall agents --all --force -g 2>/dev/null || true
```

Expected:
- exit_code: 0

## Pass Criteria

- [ ] Mixed repo install shows "N skill(s), N agent(s)" in Found message
- [ ] Skills installed to skills source dir
- [ ] Agents installed to agents source dir (not skills dir)
- [ ] Pure agent repo installs agents correctly
- [ ] Sync distributes agents to targets with agents path
- [ ] Sync shows warning listing targets without agents path
- [ ] Project mode install puts agents in `.skillshare/agents/`, not global
