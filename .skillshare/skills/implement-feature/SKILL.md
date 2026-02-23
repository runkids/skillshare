---
name: skillshare-implement-feature
description: Implement a feature from a spec file or description using TDD workflow
argument-hint: "[spec-file-path | feature description]"
targets: [claude, codex]
---

Implement a feature following TDD workflow. $ARGUMENTS is a spec file path (e.g., `specs/my-feature.md`) or a plain-text feature description.

**Scope**: This skill writes Go code and tests. It does NOT update website docs (use `update-docs` after) or CHANGELOG (use `changelog` after).

## Workflow

### Step 1: Understand Requirements

If $ARGUMENTS is a file path:
1. Read the spec file
2. Extract acceptance criteria and edge cases
3. Identify affected packages

If $ARGUMENTS is a description:
1. Search existing code for related functionality
2. Identify the right package to extend
3. Confirm scope with user before proceeding

### Step 2: Identify Affected Files

List all files that will be created or modified:

```bash
# Typical pattern for a new command
cmd/skillshare/<command>.go          # Command handler
cmd/skillshare/<command>_project.go  # Project-mode handler (if dual-mode)
internal/<package>/<feature>.go      # Core logic
tests/integration/<command>_test.go  # Integration test
```

Display the file list and continue. If scope is unclear, ask the user.

### Step 3: Write Failing Tests First (RED)

Write integration tests using `testutil.Sandbox`:

```go
func TestFeature_BasicCase(t *testing.T) {
    sb := testutil.NewSandbox(t)
    defer sb.Cleanup()

    // Setup
    sb.CreateSkill("test-skill", map[string]string{
        "SKILL.md": "---\nname: test-skill\n---\n# Content",
    })

    // Act
    result := sb.RunCLI("command", "args...")

    // Assert
    result.AssertSuccess()
    result.AssertOutputContains("expected output")
}
```

Verify tests fail:
```bash
make test-int
# or run specific test:
go test ./tests/integration -run TestFeature_BasicCase
```

### Step 4: Implement (GREEN)

Write minimal code to make tests pass:

1. Follow existing patterns in `cmd/skillshare/` and `internal/`
2. Use `internal/ui` for terminal output (colors, spinners, boxes)
3. Add oplog instrumentation for mutating commands:
   ```go
   start := time.Now()
   // ... do work ...
   e := oplog.NewEntry("command-name", statusFromErr(err), time.Since(start))
   oplog.Write(configPath, oplog.OpsFile, e)
   ```
4. Register command in `main.go` commands map if new command

Verify tests pass:
```bash
make test-int
```

### Step 5: Refactor and Verify

1. Clean up code while keeping tests green
2. Run full quality check:
   ```bash
   make check  # fmt-check + lint + test
   ```
3. Fix any formatting or lint issues

### Step 6: E2E Runbook (Major Features Only)

If the feature meets **any** of these criteria, generate an E2E runbook:
- New command or subcommand
- Changes to install/uninstall/sync flow
- Security-related (audit, hash verification, rollback)
- Multi-step user workflow (init → install → sync → verify)
- Edge cases that integration tests alone can't cover (Docker, network, file permissions)

Generate `ai_docs/tests/<slug>_runbook.md` following the existing convention:

```markdown
# CLI E2E Runbook: <Title>

<One-line summary of what this validates.>

**Origin**: <version> — <why this runbook exists>

## Scope

- <bullet list of behaviors being validated>

## Environment

Run inside devcontainer with `ssenv` isolation.

## Steps

### 1. Setup: <description>

\```bash
<commands>
\```

**Expected**: <what should happen>

### 2. <Action>: <description>
...

## Pass Criteria

- All steps marked PASS
- <additional criteria>
```

Key conventions:
- YAML-free, pure Markdown
- Each step has `bash` block + `Expected` block
- `ss` = `skillshare`, `~` = ssenv-isolated HOME
- Runbook can be executed by the `cli-e2e-test` skill

If the feature does not meet the criteria above, skip this step.

### Step 7: Stage and Report

1. List all created/modified files
2. Confirm each acceptance criterion is met with test evidence
3. Remind user to run `update-docs` if the feature affects CLI flags or user-visible behavior

## Rules

- **Test-first** — always write failing test before implementation
- **Minimal code** — only write what's needed to pass tests
- **Follow patterns** — match existing code style in each package
- **3-strike rule** — if a test fails 3 times after fixes, stop and report what's blocking
- **No docs** — this skill writes code only; use `update-docs` for documentation
- **No changelog** — use `changelog` skill for release notes
- **Spec ambiguity** — ask the user rather than guessing
