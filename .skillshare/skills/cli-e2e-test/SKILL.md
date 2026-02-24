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
4. After each step, verify conditions in the Expected block
5. Mark each step PASS / FAIL

#### Generating new runbook:

1. Read `git diff HEAD~3` to find changed files in `cmd/skillshare/` or `internal/`
2. Read changed files to understand new/modified functionality
3. **Validate all CLI flags before writing** — for every `ss <command> <flag>` in the runbook:
   - Grep `cmd/skillshare/<command>.go` for the exact flag string (e.g. `"--force"`)
   - Run `ss <command> --help` inside container if needed
   - Common mistakes to avoid:
     - `uninstall --yes` → **wrong**, use `--force` / `-f`
     - `init --target <name>` → **wrong**, `init` has no `--target` flag
     - `init -p --no-skill --force` → `--force` is not an init flag; use `--no-copy`
4. Generate new runbook to `ai_docs/tests/<slug>_runbook.md`, following existing conventions:
   - YAML-free, pure Markdown
   - Has Scope, Environment, Steps (each with bash + Expected), Pass Criteria
5. **Run the runbook quality checklist** (see below) before executing
6. Then execute the new runbook (same flow as above)

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

4. If any FAIL → distinguish between runbook bug vs real bug:
   - **Runbook bug**: wrong flag, wrong file path, stale assertion → fix runbook, re-run step
   - **Real bug**: CLI misbehavior → analyze cause, provide fix suggestions

5. **Retrospective** — ask user (via AskUserQuestion):
   > Did you encounter any friction during this test run that the skill or runbook could handle better?
   - **Option A**: Yes, improve e2e skill — review test friction (wrong flags, stale assertions, missing checklist items, unclear instructions), then update SKILL.md and/or runbooks
   - **Option B**: Yes, but only fix the runbook — fix the specific runbook without changing the skill itself
   - **Option C**: No, skip

   Improvement targets:
   - **SKILL.md**: add new checklist items, common-mistake examples, or rule clarifications learned from this run
   - **Runbooks**: fix stale assertions (e.g. config.yaml → registry.yaml), wrong flags, outdated paths
   - **Both**: when a systemic issue (e.g. a refactor changed file locations) affects both the skill's guidance and existing runbooks

## Runbook Quality Checklist

Before executing a newly generated runbook, verify:

- [ ] **All CLI flags exist** — every `ss <cmd> --flag` was grep-verified against source
- [ ] **`--init` interaction** — if runbook has `ss init`, account for `ssenv create --init` already initializing (add `--force` to re-init, or skip init step)
- [ ] **Correct confirmation flags** — `uninstall` uses `--force` (not `--yes`); `init` re-run needs no flag (just fails gracefully)
- [ ] **Skill data in registry.yaml** — assertions about installed skills check `registry.yaml`, NOT `config.yaml`; config.yaml should never contain `skills:`
- [ ] **File existence timing** — `registry.yaml` is only created after first install/reconcile, not on `ss init`
- [ ] **Project mode paths** — project commands use `.skillshare/` not `~/.config/skillshare/`

## Rules

- **Always execute inside devcontainer** — use `docker exec`, never run CLI on host
- **Always use `ssenv` for HOME isolation** — don't pollute container default HOME
- **Verify every step** — never skip Expected checks
- **Don't abort on failure** — record FAIL, continue to next step, summarize at end
- **Ask before cleanup** — Phase 4 must prompt user before deleting ssenv environment
- **`ss` = `skillshare`** — same binary in runbooks
- **`~` = ssenv-isolated HOME** — `ssenv enter` auto-sets `HOME`
- **Use `--init`** — simplify setup by using `ssenv create <name> --init`
- **`--init` already runs init** — the env is pre-initialized; runbook steps calling `ss init` again will fail unless the step explicitly resets state first

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
