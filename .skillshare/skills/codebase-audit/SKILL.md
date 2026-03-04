---
name: skillshare-codebase-audit
description: >-
  Cross-validate CLI flags, docs, tests, and targets for consistency across the
  codebase. Use this skill whenever the user asks to: audit the codebase, check
  for consistency issues, find undocumented flags, verify test coverage, validate
  targets.yaml, check handler split conventions, or verify oplog instrumentation.
  This is a read-only audit — it reports issues but never modifies files. Use
  after large refactors, before releases, or whenever you suspect docs/code/tests
  have drifted out of sync.
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

### 5. Handler Split Audit

For commands with >300 lines in `cmd/skillshare/<cmd>.go`, verify the handler split convention is followed:

```bash
# Find large command files
wc -l cmd/skillshare/*.go | sort -rn | head -20
```

Check that large commands are properly split:

| Suffix | Expected for large commands |
|--------|---------------------------|
| `_handlers.go` | Core logic extracted |
| `_render.go` | Output rendering separated |
| `_tui.go` | TUI components isolated |

Report:
- **SPLIT**: Large command properly follows handler split convention
- **MONOLITH**: >300 lines without split (should be refactored)
- **N/A**: Small command, no split needed

### 6. Oplog Coverage

Verify all mutating commands have oplog instrumentation:

```bash
# Find commands that modify state
grep -rn 'func handle\|func cmd' cmd/skillshare/*.go

# Check for oplog.Write calls
grep -rn 'oplog.Write' cmd/skillshare/
```

Mutating commands (install, uninstall, sync, update, init, collect, backup, restore, trash) should all write to oplog. Read-only commands (list, status, check, search, audit, log, version) should not.

Report:
- **INSTRUMENTED**: Mutating command has oplog.Write
- **MISSING**: Mutating command lacks oplog instrumentation
- **N/A**: Read-only command (no oplog expected)

### 7. Web API Consistency

Verify `internal/server/handler_*.go` routes match CLI commands:

```bash
# List all handler files
ls internal/server/handler_*.go | grep -v _test.go

# Check route registration in server.go
grep -n 'HandleFunc\|Handle(' internal/server/server.go
```

Report:
- **SYNCED**: CLI command has corresponding API handler
- **CLI-ONLY**: Command exists in CLI but not in Web API (may be intentional)
- **API-ONLY**: API handler without CLI counterpart (unusual)

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

### Handler Split (N issues)
| Command   | Lines | Status    | Notes              |
|-----------|-------|-----------|--------------------|
| install   | 450   | SPLIT     | 6 sub-files        |
| audit     | 320   | MONOLITH  | should split render |
| status    | 80    | N/A       |                    |

### Oplog (N issues)
| Command   | Mutating? | Status        |
|-----------|-----------|---------------|
| install   | Yes       | INSTRUMENTED  |
| trash     | Yes       | MISSING       |
| list      | No        | N/A           |

### Web API (N issues)
| Command   | CLI | API | Status   |
|-----------|-----|-----|----------|
| install   | Yes | Yes | SYNCED   |
| diff      | Yes | No  | CLI-ONLY |

== Summary: X OK / Y issues found ==
```

## Rules

- **Read-only** — never modify files, only report
- **Evidence-based** — every finding must include file path and line number
- **No false positives** — verify with grep before flagging
- **Scope $ARGUMENTS** — if user specifies "flags", only run dimension 1; "handlers" for dimension 5, "oplog" for dimension 6, "api" for dimension 7
