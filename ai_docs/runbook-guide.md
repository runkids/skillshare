# Runbook Runner Guide

`bin/runbook` is a CLI tool that parses, executes, and validates Markdown-based E2E test runbooks.

## Quick Start

Inside the devcontainer, the `runbook` command auto-builds from source:

```bash
# Run a single runbook
runbook ai_docs/tests/atomgit_install_runbook.md

# Run all runbooks in a directory
runbook ai_docs/tests/

# Dry-run (parse only, no execution)
runbook --dry-run --report json ai_docs/tests/atomgit_install_runbook.md

# JSON report
runbook --report json --no-tui ai_docs/tests/atomgit_install_runbook.md

# Run only steps 1 and 3
runbook --steps 1,3 ai_docs/tests/first_use_cli_e2e_runbook.md

# Run from step 4 onwards (skip steps 1-3)
runbook --from 4 ai_docs/tests/first_use_cli_e2e_runbook.md
```

Outside the devcontainer, build manually:

```bash
go build -o bin/runbook ./tools/runbook
bin/runbook --dry-run --report json ai_docs/tests/  # dry-run only on host
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Parse the runbook only; do not execute any commands |
| `--report json` | Output a JSON report |
| `--no-tui` | Disable interactive TUI; use plain text output |
| `--timeout 5m` | Per-step timeout (default: 2m, or from runbook.json) |
| `--steps 1,3,5` | Only run specific steps (comma-separated); others are skipped |
| `--from N` | Run from step N onwards; earlier steps are skipped |
| `--build "cmd"` | Command to run once before all runbooks |
| `--setup "cmd"` | Command to run before each runbook |
| `--teardown "cmd"` | Command to run after each runbook |

Input can be a single `.md` file or a directory (auto-discovers `*_runbook.md` / `*-runbook.md`).

`--steps` and `--from` are mutually exclusive. Filtered-out steps appear as `skipped` in the report.

## Lifecycle Hooks

### Config File (`runbook.json`)

Place a `runbook.json` in the runbook directory for persistent defaults:

```json
{
  "build": "cd /workspace && make build && go build -o /usr/local/bin/runbook ./tools/runbook/",
  "setup": "ss init -g --no-copy --all-targets --no-git --no-skill --force",
  "teardown": "ss uninstall --all --force 2>/dev/null || true",
  "timeout": "5m"
}
```

The runner auto-discovers `runbook.json` in the target directory (or parent directory of a single file).

### Priority

CLI flags override config file values:

```bash
# Uses runbook.json setup, but overrides timeout
runbook --timeout 10m ai_docs/tests/

# Overrides setup from runbook.json
runbook --setup "echo custom-init" ai_docs/tests/my_runbook.md
```

### Behavior

- **Build**: Runs once before all runbooks (not per-runbook). Uses `os/exec` directly. If build fails, the runner exits immediately — no runbooks are executed.
- **Setup**: Runs before each runbook as a synthetic step. Environment variables set in setup persist to all runbook steps (same session executor). If setup fails, all runbook steps are skipped.
- **Teardown**: Runs after each runbook. Teardown failure is logged but does not affect the runbook's pass/fail result.
- **Dry-run**: Hooks are ignored in `--dry-run` mode.
- Setup and teardown are not included in the step count or report steps.

Lifecycle: `build (once) → [per runbook: setup → steps → teardown]`

## Safety Mechanism

The runbook runner only executes inside containers (detected via `/.dockerenv`). Running outside a container is refused:

```
runbook: refusing to execute outside a container
  Use --dry-run to parse without executing, or set
  RUNBOOK_ALLOW_EXECUTE=1 to override this safety check.
```

- `--dry-run` works in any environment
- Set `RUNBOOK_ALLOW_EXECUTE=1` to override (for testing only)

---

## Writing Runbooks

### Basic Structure

```markdown
# Runbook Title

## Scope

Describe the test scope.

## Environment

- Requirements (e.g., devcontainer, network access)

## Steps

### Step 1: Step title

Optional description.

\`\`\`bash
ss install https://github.com/user/repo --all
\`\`\`

**Expected:**
- exit_code: 0
- Installed
- my-skill

### Step 2: Verify result

\`\`\`bash
ss list --no-tui
\`\`\`

**Expected:**
- exit_code: 0
- my-skill

## Pass Criteria

All steps pass.
```

### Step Heading Formats

The parser supports multiple heading formats:

```markdown
### Step 1: Title       <- standard
## Step 0: Setup        <- ## level works too
### 1. Title            <- number with period
### 1b. Sub-step        <- with suffix letter
```

### Code Blocks

- Supports ` ```bash `, ` ```sh `, or unmarked (defaults to bash)
- Multiple code blocks within a single step are merged with `---` separator
- Non-bash/sh language tags (e.g., `yaml`, `json`) are classified as `manual` and skipped during execution

### Expected Blocks

Write `**Expected:**` below the code block, followed by `- ` prefixed assertion lines.

---

## Assertion Syntax

### Five Assertion Types

#### 1. Substring (default)

```markdown
- Installed
- my-skill
```

Case-insensitive substring match against stdout + stderr combined.

#### 2. Negated Substring

```markdown
- Not error
- NOT warning
- not found
```

Supported negation prefixes (matched longest-first):

| Prefix | Example |
|--------|---------|
| `Should NOT ` | `Should NOT contain errors` |
| `should not ` | `should not fail` |
| `Must NOT ` | `Must NOT crash` |
| `must not ` | `must not hang` |
| `Does not ` | `Does not contain secrets` |
| `does not ` | `does not leak` |
| `NOT ` | `NOT Enumerating objects` |
| `Not ` | `Not error` |
| `not ` | `not failed` |
| `No ` | `No warnings` |
| `no ` | `no errors` |

#### 3. Exit Code

```markdown
- exit_code: 0       <- exact match
- exit_code: 1       <- expect failure
- exit_code: !0      <- any non-zero value
- exit_code: !1      <- any value except 1
```

Checks only the process exit code, not stdout/stderr.

#### 4. Regex

```markdown
- regex: \d+ skills
- regex: v\d+\.\d+\.\d+
- regex: (installed|updated)\s+successfully
```

Uses Go `regexp` syntax. Matches against stdout + stderr combined. Case-sensitive.

#### 5. jq Expression

```markdown
- jq: .count > 0
- jq: .skills | length >= 2
- jq: .status == "ok"
- jq: .installed == true
```

Runs `jq -e <expr>` against **stdout only** (stderr excluded). Requires `jq` to be installed.

### Match Targets

| Type | Matches Against | Case |
|------|----------------|------|
| substring | stdout + stderr | insensitive |
| negated | stdout + stderr | insensitive |
| exit_code | exit code value | N/A |
| regex | stdout + stderr | sensitive |
| jq | stdout only | N/A |

### Assertion vs Exit Code Behavior

- **With assertions**: Assertions determine pass/fail, **regardless of exit code**
  - Example: `exit_code: 1` + `error` -> passes even with exit code 1
- **Without assertions**: exit code 0 -> pass, non-zero -> fail

---

## JSON Output Format

```json
{
  "version": "1",
  "runbook": "atomgit_install_runbook.md",
  "duration_ms": 12345,
  "summary": {
    "total": 7,
    "passed": 5,
    "failed": 1,
    "skipped": 1
  },
  "steps": [
    {
      "step": {
        "number": 1,
        "title": "Install from AtomGit",
        "command": "ss install https://atomgit.com/...",
        "lang": "bash",
        "expected": ["exit_code: 0", "Installed"],
        "executor": "auto"
      },
      "status": "passed",
      "duration_ms": 3200,
      "stdout": "...",
      "stderr": "",
      "exit_code": 0,
      "assertions": [
        {
          "pattern": "exit_code: 0",
          "type": "exit_code",
          "matched": true
        },
        {
          "pattern": "Installed",
          "type": "substring",
          "matched": true
        }
      ]
    }
  ]
}
```

### AssertionResult Fields

| Field | Description |
|-------|-------------|
| `pattern` | Original assertion text |
| `type` | `substring`, `exit_code`, `regex`, or `jq` |
| `matched` | Whether the assertion passed |
| `negated` | Whether it's a negated assertion (`Not ...` or `exit_code: !N`) |
| `detail` | Extra info on failure (e.g., `got exit_code=1, expected 0`) |

---

## Session Executor (Cross-Step Variable Persistence)

In non-TUI mode, all `auto` steps run sequentially within a single bash process. Each step executes in a subshell `()` for isolation, but environment variables are persisted via an env file:

```
Step 1: export FOO=bar  ->  saved to /tmp/env
Step 2: echo $FOO       ->  sources /tmp/env first -> prints "bar"
```

Steps in a runbook can naturally reference variables set by earlier steps without any extra handling.

### Step Failure Does Not Abort

A failed step (non-zero exit code) does not stop subsequent steps. All steps run to completion, and the final report aggregates all results.

### Manual Steps

Steps without a bash code block, or with non-bash language tags, are classified as `manual`, automatically skipped, and marked as `skipped`.

---

## Full Example Runbook

```markdown
# My Feature E2E Test

## Scope

Verify the full install -> list -> sync -> uninstall lifecycle.

## Environment

- Devcontainer with ssenv isolation
- Network access required

## Steps

### Step 1: Install skill

\`\`\`bash
ss install https://github.com/user/repo --all
\`\`\`

**Expected:**
- exit_code: 0
- Installed

### Step 2: Verify in list

\`\`\`bash
ss list --json | jq -e '.[] | select(.name == "my-skill")'
\`\`\`

**Expected:**
- exit_code: 0
- jq: .name == "my-skill"

### Step 3: Check sync

\`\`\`bash
ss sync
\`\`\`

**Expected:**
- exit_code: 0
- regex: \d+ linked

### Step 4: Uninstall

\`\`\`bash
ss uninstall my-skill --force
\`\`\`

**Expected:**
- exit_code: 0
- Moved to trash

### Step 5: Verify cleanup

\`\`\`bash
ss list --no-tui
\`\`\`

**Expected:**
- exit_code: 0
- Not my-skill

## Pass Criteria

All 5 steps pass.
```

---

## Common Patterns

### Selective Execution (Debugging)

When iterating on a specific step, skip the rest to save time:

```bash
# Re-run only step 5 after fixing the command
runbook --steps 5 ai_docs/tests/my_runbook.md

# Run the last 3 steps (skip setup steps)
runbook --from 4 ai_docs/tests/my_runbook.md

# Run steps 2 and 5, get JSON for analysis
runbook --steps 2,5 --report json --no-tui ai_docs/tests/my_runbook.md
```

Skipped steps still appear in the report with `status: "skipped"`, so the step numbering stays consistent.

### Verify File Exists

```bash
test -f ~/.config/skillshare/registry.yaml && echo "EXISTS" || echo "MISSING"
```

```markdown
**Expected:**
- exit_code: 0
- EXISTS
- Not MISSING
```

### JSON Field Validation

```bash
ss status --json
```

```markdown
**Expected:**
- exit_code: 0
- jq: .skill_count >= 1
- jq: .targets | length > 0
```

### Expect a Failing Step

```bash
ss install nonexistent-repo
```

```markdown
**Expected:**
- exit_code: !0
- Not Installed
- error
```

### Multi-Line Script

```bash
SKILL_DIR=~/.config/skillshare/skills/my-skill
test -d "$SKILL_DIR" && echo "DIR_OK" || echo "DIR_MISSING"
cat "$SKILL_DIR/.skillshare-meta.json" | jq -e '.tree_hash'
echo "META_OK"
```

```markdown
**Expected:**
- exit_code: 0
- DIR_OK
- META_OK
```

---

## Usage in CI / E2E Skill

### Build and Execute Inside Devcontainer

```bash
# Build Linux binary
docker exec $CONTAINER bash -c 'cd /workspace && go build -o bin/runbook ./tools/runbook'

# Execute in ssenv-isolated environment
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- \
  /workspace/bin/runbook --report json --no-tui \
  /workspace/ai_docs/tests/my_runbook.md
```

### Dry-Run Preview

```bash
bin/runbook --dry-run --report json ai_docs/tests/ | jq -s '
  [.[] | select(.runbook) | {
    runbook: .runbook,
    steps: .summary.total
  }]'
```

### Analyze Failed Steps

```bash
bin/runbook --report json --no-tui my_runbook.md | jq '
  .steps[] | select(.status == "failed") | {
    step: .step.number,
    title: .step.title,
    exit_code: .exit_code,
    failed_assertions: [.assertions[] | select(.matched == false)]
  }'
```
