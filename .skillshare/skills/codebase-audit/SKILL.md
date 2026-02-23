---
name: skillshare-codebase-audit
description: Cross-validate CLI flags, docs, tests, and targets for consistency across the codebase
targets: [claude, codex]
---

Read-only consistency audit across the skillshare codebase. $ARGUMENTS specifies focus area (e.g., "flags", "tests", "targets") or omit for full audit.

**Scope**: This skill only READS and REPORTS. It does not modify any files. Use `implement-feature` to fix issues or `update-docs` to fix documentation gaps.

## Audit Dimensions

Run all 4 dimensions in parallel where possible. For each, produce a summary table.

### 1. CLI Flag Audit

Compare every flag defined in `cmd/skillshare/*.go` against `website/docs/commands/*.md`.

```bash
# Find all flags in Go source
grep -rn 'flag\.\(String\|Bool\|Int\)' cmd/skillshare/
grep -rn 'Args\|Usage' cmd/skillshare/
```

Report:
- **UNDOCUMENTED**: Flag exists in code but not in docs
- **STALE**: Flag documented but not found in code
- **OK**: Flag matches between code and docs

### 2. Spec vs Code

For each spec in `specs/` marked as completed/done:
- Verify the described feature exists in source code
- Check that the spec's acceptance criteria are testable

Report:
- **IMPLEMENTED**: Spec complete, code exists
- **MISMATCH**: Spec says done but code missing or partial
- **PENDING**: Spec not yet marked complete (informational)

### 3. Test Coverage

For each command handler in `cmd/skillshare/<cmd>.go`:
- Check if `tests/integration/<cmd>_test.go` exists
- Check if key behaviors have test cases

```bash
# List all command handlers
ls cmd/skillshare/*.go | grep -v '_test.go\|main.go\|helpers.go\|mode.go'

# List all integration tests
ls tests/integration/*_test.go
```

Report:
- **COVERED**: Command has integration test file with test cases
- **PARTIAL**: Test file exists but missing key scenarios
- **MISSING**: No integration test for this command

### 4. Target Audit

Verify `internal/config/targets.yaml` entries:
- Each target has both `global_path` and `project_path`
- Aliases are consistent
- No duplicate entries

Report:
- **OK**: Target entry complete and valid
- **INCOMPLETE**: Missing required fields
- **DUPLICATE**: Name or alias collision

## Output Format

```
== Skillshare Codebase Audit ==

### CLI Flags (N issues)
| Command   | Flag        | Status       |
|-----------|-------------|--------------|
| install   | --force     | OK           |
| install   | --into      | UNDOCUMENTED |

### Specs (N issues)
| Spec File            | Status      |
|----------------------|-------------|
| copy-sync-mode.md    | IMPLEMENTED |
| some-feature.md      | MISMATCH    |

### Test Coverage (N issues)
| Command   | Status  | Notes              |
|-----------|---------|--------------------|
| sync      | COVERED |                    |
| audit     | PARTIAL | missing edge cases |
| target    | MISSING |                    |

### Targets (N issues)
| Target    | Status     | Notes         |
|-----------|------------|---------------|
| claude    | OK         |               |
| newagent  | INCOMPLETE | no project_path |

== Summary: X OK / Y issues found ==
```

## Rules

- **Read-only** — never modify files, only report
- **Evidence-based** — every finding must include file path and line number
- **No false positives** — verify with grep before flagging
- **Scope $ARGUMENTS** — if user specifies "flags", only run dimension 1
