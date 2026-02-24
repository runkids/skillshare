---
name: skillshare-cli-e2e-test
description: Run isolated E2E tests in devcontainer from ai_docs/tests runbooks
argument-hint: "[runbook-name | new]"
targets: [claude, codex]
---

Run isolated E2E tests in devcontainer. $ARGUMENTS specifies runbook name or "new".

## Flow

### Phase 0: Environment Check

1. Confirm devcontainer is running and get container ID:
   ```bash
   CONTAINER=$(docker compose -f .devcontainer/docker-compose.yml ps -q skillshare-devcontainer)
   ```
   - If empty → prompt user: `docker compose -f .devcontainer/docker-compose.yml up -d`
   - Ensure `CONTAINER` is set for all subsequent `docker exec` calls.

2. Confirm Linux binary is available:
   ```bash
   docker exec $CONTAINER bash -c \
     '/workspace/.devcontainer/ensure-skillshare-linux-binary.sh && ss version'
   ```

### Phase 1: Detect Scope

1. Read all `*_runbook.md` files under `ai_docs/tests/` to list available runbooks
2. Identify recent changes (unstaged + recent commits):
   ```bash
   git diff --name-only HEAD~3
   ```
3. Match changes to relevant runbooks.

### Phase 2: Select Tests

Prompt user (via AskUserQuestion):

- **Option A**: Run existing runbook (list all available + mark those related to recent changes)
- **Option B**: Auto-generate new test script based on recent changes
- **Option C**: If $ARGUMENTS specifies a runbook, skip to Phase 3

### Phase 3: Prepare & Execute

#### Running existing runbook:

1. Read the selected runbook `.md` file
2. Create isolated environment with **auto-initialization**:
   ```bash
   ENV_NAME="e2e-$(date +%Y%m%d-%H%M%S)"

   # Use --init to automatically run 'ss init -g' with all targets
   docker exec $CONTAINER ssenv create "$ENV_NAME" --init
   ```

3. Execute each step from runbook:
   ```bash
   # Use SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 to prevent redirection to demo-project
   docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
     ssenv enter "$ENV_NAME" -- <command>
   ```
3. After each step, verify conditions in the Expected block
4. Mark each step PASS / FAIL

#### Generating new script:

1. Read `git diff HEAD~3` to find changed files in `cmd/skillshare/` or `internal/`
2. Read changed files to understand new/modified functionality
3. Generate new runbook to `ai_docs/tests/<slug>_runbook.md`, following existing conventions:
   - YAML-free, pure Markdown
   - Has Scope, Environment, Steps (each with bash + Expected), Pass Criteria
4. Then execute the new runbook (same flow as above)

### Phase 4: Cleanup & Report

1. Ask user before cleanup (via AskUserQuestion):
   - **Option A**: Delete ssenv environment now
   - **Option B**: Keep for manual debugging (print env name for later `ssenv delete`)

2. If user chose Option A:
   ```bash
   docker exec $CONTAINER ssenv delete "$ENV_NAME" --force
   ```

3. Output summary:
   ```
   ── E2E Test Report ──

   Runbook:  {runbook name}
   Env:      {ENV_NAME}
   Duration: {time}

   Step 1: {description}  PASS
   Step 2: {description}  PASS
   Step 3: {description}  FAIL ← {error detail}
   ...

   Result: {N}/{total} passed
   ```

4. If any FAIL → analyze cause, provide fix suggestions

## Rules

- **Always execute inside devcontainer** — use `docker exec`, never run CLI on host
- **Always use `ssenv` for HOME isolation** — don't pollute container default HOME
- **Verify every step** — never skip Expected checks
- **Don't abort on failure** — record FAIL, continue to next step, summarize at end
- **Ask before cleanup** — Phase 4 must prompt user before deleting ssenv environment
- **`ss` = `skillshare`** — same binary in runbooks
- **`~` = ssenv-isolated HOME** — `ssenv enter` auto-sets `HOME`
- **Use `--init`** — simplify setup by using `ssenv create <name> --init`

## ssenv Quick Reference

| Command | Purpose |
|---------|---------|
| `sshelp` | Show shortcuts and usage |
| `ssls` | List isolated environments |
| `ssnew <name>` | Create + enter isolated shell (interactive) |
| `ssuse <name>` | Enter existing isolated shell (interactive) |
| `ssback` | Leave isolated context |
| `ssenv enter <name> -- <cmd>` | Run single command in isolation (automation) |

- For interactive debugging: `ssnew <env>` then `exit` when done
- For deterministic automation: prefer `ssenv enter <env> -- <command>` one-liners

## Test Command Policy

When running Go tests inside devcontainer (not via runbook):

```bash
# ssenv changes HOME, so always cd to /workspace first for Go test commands
cd /workspace
go build -o bin/skillshare ./cmd/skillshare
SKILLSHARE_TEST_BINARY="$PWD/bin/skillshare" go test ./tests/integration -count=1
go test ./...
```

Always run in devcontainer unless there is a documented exception.
Note: `ssenv enter` changes HOME, which may affect Go module resolution — always `cd /workspace` before running `go test` or `go build`.

## Container Command Templates

```bash
# Single command
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- ss status

# Multi-line compound command (use bash -c)
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  ss init --no-copy --all-targets --no-git --no-skill
  ss status
'

# Check files (HOME is set to isolated path by ssenv)
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  cat ~/.config/skillshare/config.yaml
'

# With environment variables
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  TARGET=~/.claude/skills
  ls -la "$TARGET"
'

# Go tests (must cd /workspace because ssenv changes HOME)
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  cd /workspace
  go test ./internal/install -run TestParseSource -count=1
'
```
