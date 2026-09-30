# CLI E2E Runbook: Hooks Fire in Real Agents

## Scope

Prove that the native files `skillshare hooks` writes are loaded and run by the
real Agents, not only that they match the shape the unit tests expect. Every hook
writes a marker file and nothing else. Each Agent runs once per scope with a
throwaway HOME and fake credentials: it starts a session, runs its hooks, then
fails at the model call. No login, real Agent configuration or trust store is used.

Covered by firing here: Claude Code, Codex, Gemini CLI, Qwen Code, Copilot CLI,
Factory Droid, OpenCode, Amp (plugin load only) and Pi, in project and global scope.
Antigravity and Cursor check credentials before any hook runs; see
[Agents that need a login](#agents-that-need-a-login).

## Environment

Run inside the devcontainer with a fresh ssenv HOME and network access (the first
step downloads Agent CLIs into `/tmp/hooks-agents`). Fixtures live in
`/workspace/tmp/hooks-e2e-<target>` (gitignored, each its own Git root) and
`/tmp/hooks-e2e-*`. Helper: `scripts/hooks/real-agents.sh`.

```sh
CONTAINER=$(docker compose -f .devcontainer/docker-compose.yml ps -q skillshare-devcontainer)
docker exec "$CONTAINER" ssenv create hooks-real-agents --init
docker exec "$CONTAINER" /workspace/.devcontainer/ensure-mdproof.sh
docker exec "$CONTAINER" ssenv enter hooks-real-agents -- \
  mdproof --report json /workspace/ai_docs/tests/hooks_real_agents_runbook.md
```

Trust gates are satisfied only inside the throwaway HOME or with per-run flags:
Codex gets a `trust_level = "trusted"` entry in its temporary `CODEX_HOME` plus
`--dangerously-bypass-hook-trust`, Copilot a `trustedFolders` entry in its temporary
`COPILOT_HOME`, Gemini `GEMINI_CLI_TRUST_WORKSPACE=true`, and Pi `--approve`.
Without these, project hooks are skipped by design.

## Agents that need a login

These run manually on a machine where the Agent is already logged in. Generate
the fixture in the devcontainer with the host path as the marker directory, run
the Agent from that directory on the host, then check for `.hook-fired-*`:

```sh
HOST_TMP="$PWD/tmp"   # on the host, from the repository root
docker exec "$CONTAINER" ssenv enter hooks-real-agents -- \
  bash /workspace/scripts/hooks/real-agents.sh project antigravity "$HOST_TMP/hooks-e2e-antigravity"
cd tmp/hooks-e2e-antigravity && agy -p "reply ok"; ls -A | grep hook-fired
```

- **Antigravity (`agy`)**: project hooks in `.agents/hooks.json` load only after
  the folder is trusted in an interactive session, which is stored persistently;
  global hooks live in the real `~/.gemini/config/hooks.json`. With a throwaway
  HOME, `agy -p` asks for Google OAuth before any hook runs.
- **Cursor (`cursor-agent`)**: it validates `CURSOR_API_KEY` before the session
  starts, and project hooks need a trusted workspace.
- **Claude Code and Pi** can also be confirmed on the host: `claude -p "reply ok"`
  (print mode skips the workspace trust dialog) and `pi -p --approve --no-session
  "reply ok"` (`--approve` trusts project files for that run only).
- **Codex on the host**: project hooks need a persisted `trust_level` in the real
  `~/.codex/config.toml`; `-c` overrides do not mark a project trusted.

## Steps

### Step 1: Build and install Agent CLIs

```bash
cd /workspace && make build >/dev/null
bash /workspace/scripts/hooks/real-agents.sh clean
bash /workspace/scripts/hooks/real-agents.sh install
```

Expected:
- exit_code: 0
- regex: claude \d+\.
- regex: codex codex-cli \d+\.
- regex: copilot GitHub Copilot CLI \d+\.
- regex: pi \d+\.
- regex: droid \d+\.

### Step 2: Claude Code runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project claude >/dev/null
P=$(bash "$S" fire claude project)
bash "$S" global claude >/dev/null
G=$(bash "$S" fire claude global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("SessionStart")) != null and (.global | index("SessionStart")) != null
- jq: (.project | index("UserPromptSubmit")) != null and (.global | index("UserPromptSubmit")) != null

### Step 3: Codex runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project codex >/dev/null
P=$(bash "$S" fire codex project)
bash "$S" global codex >/dev/null
G=$(bash "$S" fire codex global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("SessionStart")) != null and (.global | index("SessionStart")) != null
- jq: (.project | index("UserPromptSubmit")) != null and (.global | index("UserPromptSubmit")) != null

### Step 4: Gemini CLI runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project gemini >/dev/null
P=$(bash "$S" fire gemini project)
bash "$S" global gemini >/dev/null
G=$(bash "$S" fire gemini global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("SessionStart")) != null and (.global | index("SessionStart")) != null
- jq: (.project | index("BeforeAgent")) != null and (.global | index("BeforeAgent")) != null

### Step 5: Qwen Code runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project qwen >/dev/null
P=$(bash "$S" fire qwen project)
bash "$S" global qwen >/dev/null
G=$(bash "$S" fire qwen global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("SessionStart")) != null and (.global | index("SessionStart")) != null
- jq: (.project | index("UserPromptSubmit")) != null and (.global | index("UserPromptSubmit")) != null

### Step 6: GitHub Copilot CLI runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project copilot >/dev/null
P=$(bash "$S" fire copilot project)
bash "$S" global copilot >/dev/null
G=$(bash "$S" fire copilot global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("sessionStart")) != null and (.global | index("sessionStart")) != null
- jq: (.project | index("userPromptSubmitted")) != null and (.global | index("userPromptSubmitted")) != null

### Step 7: Factory Droid runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project droid >/dev/null
P=$(bash "$S" fire droid project)
bash "$S" global droid >/dev/null
G=$(bash "$S" fire droid global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("SessionStart")) != null and (.global | index("SessionStart")) != null

### Step 8: OpenCode runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project opencode >/dev/null
P=$(bash "$S" fire opencode project)
bash "$S" global opencode >/dev/null
G=$(bash "$S" fire opencode global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("load")) != null and (.global | index("load")) != null
- jq: (.project | index("session.created")) != null and (.global | index("session.created")) != null

### Step 9: Amp runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project amp >/dev/null
P=$(bash "$S" fire amp project)
bash "$S" global amp >/dev/null
G=$(bash "$S" fire amp global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("load")) != null and (.global | index("load")) != null

### Step 10: Pi runs project and global hooks

```bash
set -e
S=/workspace/scripts/hooks/real-agents.sh
bash "$S" project pi >/dev/null
P=$(bash "$S" fire pi project)
bash "$S" global pi >/dev/null
G=$(bash "$S" fire pi global)
printf '%s\n%s\n' "$P" "$G" | jq -sc '{project: .[0].markers, global: .[1].markers}'
```

Expected:
- exit_code: 0
- jq: (.project | index("load")) != null and (.global | index("load")) != null
- jq: (.project | index("session_start")) != null and (.global | index("session_start")) != null

### Step 11: Clean up

```bash
bash /workspace/scripts/hooks/real-agents.sh clean
find /workspace/tmp /tmp -maxdepth 1 -name 'hooks-e2e-*' | wc -l
```

Expected:
- exit_code: 0
- jq: . == 0

## Pass Criteria

Steps 1 to 11 pass: every listed marker exists for both scopes. Amp's
`session.start` needs a login, so only plugin loading is proven there. Remove
the ssenv afterwards with `ssenv delete hooks-real-agents --force`.
