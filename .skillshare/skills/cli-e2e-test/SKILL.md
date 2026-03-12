---
name: skillshare-cli-e2e-test
description: >-
  Run isolated E2E tests in devcontainer from ai_docs/tests runbooks. Use this
  skill whenever the user asks to: run an E2E test, execute a test runbook,
  validate a feature end-to-end, create a new runbook, or test CLI behavior in
  isolation. If you need to run a multi-step CLI validation sequence (init →
  install → sync → verify), this is the skill — it handles ssenv isolation,
  flag verification, and structured reporting. Prefer this over ad-hoc docker
  exec sequences for any test that follows a runbook or needs reproducible
  isolation.
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

3. Confirm mdproof is installed:
   ```bash
   docker exec $CONTAINER /workspace/.devcontainer/ensure-mdproof.sh
   ```
   This auto-installs from GitHub release, or falls back to `/workspace/bin/mdproof` (local dev binary).

4. Check for lessons learned from previous runs:
   ```bash
   test -f /workspace/.mdproof/lessons-learned.md && cat /workspace/.mdproof/lessons-learned.md
   ```
   If the file exists, read it before writing or debugging runbooks — it contains known gotchas and assertion patterns.

### Phase 1: Detect Scope

1. Preview all available runbooks via the container:
   ```bash
   docker exec $CONTAINER mdproof --dry-run --report json /workspace/ai_docs/tests/
   ```
   This returns JSON with every runbook's steps, commands, and expected assertions — no manual markdown parsing needed. Use this to understand what each runbook covers.

2. Identify recent changes (unstaged + recent commits):
   ```bash
   git diff --name-only HEAD~3
   ```
3. Match changes to relevant runbooks (compare changed file paths against step commands in the JSON output).

### Phase 2: Select Tests

Prompt user (via AskUserQuestion):

- **Option A**: Run existing runbook (list all available + mark those related to recent changes)
- **Option B**: Auto-generate new test script based on recent changes
- **Option C**: If $ARGUMENTS specifies a runbook, skip to Phase 3

### Phase 3: Prepare & Execute

#### Running existing runbook:

1. Create isolated environment with **auto-initialization**:
   ```bash
   ENV_NAME="e2e-$(date +%Y%m%d-%H%M%S)"

   # Use --init to automatically run 'ss init -g' with all targets
   docker exec $CONTAINER ssenv create "$ENV_NAME" --init
   ```

2. Execute the entire runbook via mdproof inside the container:
   ```bash
   docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
     ssenv enter "$ENV_NAME" -- \
     mdproof --report json \
     /workspace/ai_docs/tests/<runbook_file>.md
   ```
   mdproof executes each step (`bash -c <command>`) in the ssenv-isolated HOME, then returns structured JSON:
   ```json
   {
     "version": "1",
     "runbook": "<runbook_file>.md",
     "duration_ms": 12345,
     "summary": { "total": 7, "passed": 5, "failed": 1, "skipped": 1 },
     "steps": [
       {
         "step": { "number": 1, "title": "...", "command": "...", "expected": ["..."] },
         "status": "passed",    // "passed" | "failed" | "skipped"
         "exit_code": 0,
         "stdout": "...",
         "stderr": "..."
       }
     ]
   }
   ```

3. Analyze the JSON output:
   - **All passed** → proceed to Phase 4
   - **Any failed** → filter for failures only (full JSON can be too large for terminal output):
     ```bash
     mdproof --report json runbook.md 2>&1 | jq '{
       summary: .summary,
       failed: [.steps[] | select(.status == "failed") | {
         step: .step.number, title: .step.title,
         exit_code: .exit_code,
         failed_assertions: [.assertions[]? | select(.matched == false) | .pattern],
         stderr: (.stderr // "" | .[0:200])
       }]
     }'
     ```
   - **Skipped steps** (executor=`manual`) → these need manual verification, run them individually:
     ```bash
     docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
       ssenv enter "$ENV_NAME" -- <command from step.command>
     ```

4. For failed steps, debug individually using manual docker exec (same as before):
   ```bash
   docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
     ssenv enter "$ENV_NAME" -- bash -c '<failed step command>'
   ```
   - **Prefer `--json` + `jq` for assertions** — see the JSON Reference below

#### Generating new runbook:

1. Read `git diff HEAD~3` to find changed files in `cmd/skillshare/` or `internal/`
2. Read changed files to understand new/modified functionality
3. **Validate all CLI flags before writing** — for every `ss <command> <flag>` in the runbook:
   - Grep `cmd/skillshare/<command>.go` for the exact flag string (e.g. `"--force"`)
   - Run `ss <command> --help` inside container if needed
   - Common mistakes to avoid:
     - `uninstall --yes` → **wrong**, use `--force` / `-f`
     - `init --target <name>` → **wrong**, `init` has no `--target` flag
     - `init -p` has a **completely separate flag set** from global `init` — only supports `--targets`, `--discover`, `--select`, `--mode`, `--dry-run`. Global-only flags like `--no-copy`, `--no-skill`, `--no-git`, `--all-targets`, `--force` do NOT exist in project mode
     - Audit custom rules: disable by **rule ID** (e.g. `prompt-injection-0`, `prompt-injection-1`), NOT pattern name (e.g. `prompt-injection`). Rule IDs are in `internal/audit/rules.yaml`
4. Generate new runbook to `ai_docs/tests/<slug>_runbook.md`, following existing conventions:
   - YAML-free, pure Markdown
   - Has Scope, Environment, Steps (each with bash + Expected), Pass Criteria
   - **Use `jq:` assertions in Expected blocks** for JSON commands — e.g. `- jq: .extras | length == 1`. This is a native mdproof assertion type, NOT a bash `jq` pipe
   - **Use `--json` + `jq -e` in bash** for inline verification within multi-command steps
   - **Config idempotency** — never bare `cat >> config.yaml`; always prepend `sed -i '/^section:/,$d'` to remove existing section first, or use CLI commands (`ss extras init`, `ss extras remove --force`) that handle duplicates
   - **Check `ai_docs/tests/runbook.json`** for project-level config (build, setup, teardown, step_setup, timeout) that affects all runbooks
   - **Check `.mdproof/lessons-learned.md`** for known assertion patterns and gotchas
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

3. Output summary (derived from the runbook JSON output):
   ```
   ── E2E Test Report ──

   Runbook:  {runbook name}
   Env:      {ENV_NAME}
   Duration: {duration_ms}ms

   Step 1: {title}  PASS
   Step 2: {title}  PASS
   Step 3: {title}  FAIL ← exit_code={N}, stderr: {error detail}
   ...

   Result: {passed}/{total} passed ({skipped} skipped)
   ```
   All values come directly from mdproof's JSON output — `summary.passed`, `summary.total`, `steps[].step.title`, `steps[].status`.

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
- [ ] **`--init` creates default extras** — `ssenv create --init` creates a `rules` extra by default. Runbooks that assume an empty extras list must add cleanup first: `ss extras remove rules --force -g 2>/dev/null || true` + `rm -rf ~/.claude/rules`
- [ ] **Correct confirmation flags** — `uninstall` uses `--force` (not `--yes`); `init` re-run needs no flag (just fails gracefully)
- [ ] **Skill data in registry.yaml** — assertions about installed skills check `registry.yaml`, NOT `config.yaml`; config.yaml should never contain `skills:`
- [ ] **File existence timing** — `registry.yaml` is only created after first install/reconcile, not on `ss init`
- [ ] **Project mode paths** — project commands use `.skillshare/` not `~/.config/skillshare/`
- [ ] **Project init flags** — `init -p` only supports `--targets`, `--discover`, `--select`, `--mode`, `--dry-run`; global-only flags (`--no-copy`, `--no-skill`, `--no-git`, `--all-targets`, `--force`) are not available
- [ ] **Audit rule IDs** — custom rules in `audit-rules.yaml` use rule IDs (e.g. `prompt-injection-0`), not pattern names (e.g. `prompt-injection`). Verify IDs against `internal/audit/rules.yaml`
- [ ] **Use `--json` for assertions** — if the command supports `--json`, use it with `jq` instead of grepping human-readable output. Text output changes between versions; JSON structure is stable
- [ ] **Expected = actual substrings, NOT descriptions** — the runbook assertion engine does case-insensitive substring matching. Write `- Installed` or `- cangjie-docs-navigator`, NOT `- Install completes without error` or `- Output contains at least one skill`. Negation: use `Not <substring>` prefix (e.g. `- Not cangjie-docs-navigator`)
- [ ] **Skill name ≠ repo name** — after `ss install <repo>`, the actual skill name may differ from the repo name (e.g. repo `cangjie-docs-mcp` → skill `cangjie-docs-navigator`). Always verify the installed skill name via `ss list` before writing uninstall/check steps
- [ ] **`/tmp/` cleanup** — ssenv only isolates `$HOME`; `/tmp/` is shared across runs. Any step using `/tmp/<path>` must start with `rm -rf /tmp/<path>` to avoid stale state from previous runs
- [ ] **`echo > symlink` writes through** — `echo "content" > path` where `path` is a symlink writes to the symlink's target, it does NOT replace the symlink with a real file. To create a local (non-managed) file at a symlinked path: either use a different filename, or `rm` the symlink first then `echo`
- [ ] **`cat >>` is not idempotent** — appending to config files (`cat >> config.yaml`) will duplicate sections on re-run. Prefer `ss extras init` (which validates duplicates) or full file replacement over `cat >>` when possible
- [ ] **Extras source path layout** — extras use `~/.config/skillshare/extras/<name>/` (not the legacy flat path `~/.config/skillshare/<name>/`). Symlink assertions must include `extras/` in the path regex (e.g. `regex: skillshare/extras/rules/tdd\.md`)
- [ ] **Prefer `jq:` over `python3 -c`** — for JSON output validation, use mdproof's native `jq:` assertion type (e.g. `- jq: .extras | length == 1`) instead of piping to `python3 -c`. It's one line vs 10, and mdproof handles failure reporting automatically
- [ ] **Config append idempotency** — when appending YAML sections with `cat >>`, always prepend `sed -i '/^section_key:/,$d'` to remove existing section. Or prefer CLI commands (`ss extras init`, `ss extras remove --force`) over manual config editing
- [ ] **Check lessons-learned** — read `.mdproof/lessons-learned.md` before writing new runbooks for known gotchas and proven assertion patterns

## Runbook Assertion Types

mdproof supports 6 assertion types under `Expected:` blocks. Use the most specific type for each check:

| Type | Syntax | When to use | Example |
|------|--------|-------------|---------|
| Substring | plain text | Simple output check | `- hello world` |
| Negated | `Not`/`Should NOT` prefix | Verify absence | `- Not FAIL` |
| Exit code | `exit_code: N` | Every step should have this | `- exit_code: 0` |
| Regex | `regex:` prefix | Pattern matching | `- regex: v\d+\.\d+` |
| jq | `jq:` prefix | **JSON output (preferred)** | `- jq: .extras \| length == 1` |
| Snapshot | `snapshot:` prefix | Stable output comparison | `- snapshot: api-response` |

**`jq:` best practices:**
```markdown
# Simple field check
- jq: .name == "rules"

# Array length
- jq: .extras | length == 3

# Sorted array comparison
- jq: [.extras[].name] | sort | . == ["a","b","c"]

# Null/missing field (omitempty)
- jq: .extras == null

# Nested access
- jq: .[0].targets[0].status == "synced"

# Boolean
- jq: .source_exists == true
```

## Rules

- **Always execute inside devcontainer** — use `docker exec`, never run CLI on host
- **Always use `ssenv` for HOME isolation** — don't pollute container default HOME
- **Always create fresh ssenv environments** — never reuse an environment from a previous run; stale config/state causes confusing cascade failures (e.g. duplicate YAML keys, "already exists" errors)
- **ssenv only isolates `$HOME`** — `/tmp/`, `/var/`, and other system paths are shared across all environments. Runbook steps using `/tmp/` must include `rm -rf` cleanup at the start
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

## `--json` Quick Reference

Most commands support `--json` for structured output, making assertions more reliable than text matching.

| Command | `--json` | Notes |
|---------|----------|-------|
| `ss status` | `--json` | Skills, targets, sync status |
| `ss list` | `--json` / `-j` | All skills with metadata |
| `ss target list` | `--json` | Configured targets |
| `ss install <src>` | `--json` | Implies `--force --all` (skip prompts) |
| `ss uninstall <name>` | `--json` | Implies `--force` (skip prompts) |
| `ss collect <path>` | `--json` | Implies `--force` (skip prompts) |
| `ss check` | `--json` | Update availability per repo |
| `ss update` | `--json` | Update results per skill |
| `ss diff` | `--json` | Per-file diff details |
| `ss sync` | `--json` | Sync stats per target |
| `ss audit` | `--format json` | Also accepts `--json` (deprecated alias) |
| `ss log` | `--json` | Raw JSONL (one object per line) |

**Key behaviors:**
- `--json` that implies `--force` / `--all` skips interactive prompts — safe for automation
- Output goes to **stdout only** (progress/spinners suppressed)
- `audit` prefers `--format json`; `--json` still works but is the deprecated form
- `log --json` outputs JSONL (newline-delimited), not a JSON array

### Assertion Patterns with `jq`

```bash
# Count installed skills
ss list --json | jq 'length'

# Check a specific skill exists
ss list --json | jq -e '.[] | select(.name == "my-skill")'

# Verify target is configured
ss target list --json | jq -e '.[] | select(.name == "claude")'

# Assert no critical audit findings
ss audit --format json | jq -e '.summary.critical == 0'

# Check update availability
ss check --json | jq -e '.tracked_repos | length > 0'

# Verify sync succeeded (zero errors)
ss sync --json | jq -e '.errors == 0'

# Install and verify result
ss install https://github.com/user/repo --json | jq -e '.skills | length > 0'
```

When a `jq -e` expression fails (exit code 1 = false, 5 = no output), the step FAILs — no ambiguous text matching needed.

## Container Command Templates

```bash
# Single command
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- ss status

# JSON assertion (preferred for verification)
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  ss list --json | jq -e ".[] | select(.name == \"my-skill\")"
'

# Multi-line compound command (use bash -c) — global mode flags
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  ss init --no-copy --all-targets --no-git --no-skill
  ss status
'

# Project mode init (different flag set!)
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- bash -c '
  cd /tmp/test-project && ss init -p --targets claude
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

## Relationship with `/mdproof` Skill

This skill (`/cli-e2e-test`) and the `/mdproof` skill are **complementary**, not competing:

| Concern | `/cli-e2e-test` | `/mdproof` |
|---------|-----------------|------------|
| **Scope** | Skillshare project-specific E2E | General-purpose runbook authoring |
| **Infrastructure** | Devcontainer, ssenv, binary build | None — format and assertions only |
| **Config** | `ai_docs/tests/runbook.json` (build, setup, teardown) | Assertion types, snapshot, coverage |
| **Lessons** | Checklist items, CLI flag gotchas | `.mdproof/lessons-learned.md` |
| **When** | Running or debugging a test | Writing or improving a runbook |

### How they work together

1. **Writing a new runbook** → invoke `/mdproof` first for format guidance (assertion types, `jq:` patterns, snapshot usage), then `/cli-e2e-test` to execute it in isolation
2. **Improving existing runbooks** → invoke `/mdproof` for assertion quality review (python3 → jq:, idempotency), then `/cli-e2e-test` to verify changes pass
3. **Debugging failures** → `/cli-e2e-test` Phase 3 step 4 handles manual docker exec; `/mdproof` lessons-learned captures recurring patterns
4. **After a test run** → `/mdproof` Self-Learning section guides recording discoveries to `.mdproof/lessons-learned.md`

### Rule of thumb

- Need to **run** tests or **debug** in devcontainer? → `/cli-e2e-test`
- Need to **write** assertions or **improve** runbook quality? → `/mdproof`
- User says "run extras E2E" → `/cli-e2e-test`
- User says "improve runbook assertions" → `/mdproof` then `/cli-e2e-test` to verify
