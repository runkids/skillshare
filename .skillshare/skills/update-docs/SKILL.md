---
name: skillshare-update-docs
description: Update website docs to match recent code changes, cross-validating every flag against source
argument-hint: "[command-name | commit-range]"
targets: [claude, codex]
---

Sync website documentation with recent code changes. $ARGUMENTS specifies scope: a command name (e.g., `install`), commit range, or omit to auto-detect from `git diff HEAD~1`.

**Scope**: This skill only updates `website/docs/`. It does NOT write Go code (use `implement-feature`) or CHANGELOG (use `changelog`).

## Workflow

### Step 1: Detect Changes

```bash
# Auto-detect recently changed commands
git diff HEAD~1 --stat -- cmd/skillshare/ internal/
```

Map changed files to affected documentation:
- `cmd/skillshare/install.go` → `website/docs/commands/install.md`
- `internal/audit/` → `website/docs/commands/audit.md`
- `internal/config/targets.yaml` → `website/docs/reference/supported-targets.md`

### Step 2: Cross-Validate Flags

For each affected command:

1. Read the Go source to extract actual flags and behavior:
   ```bash
   grep -n 'flag\.\|Usage\|Args' cmd/skillshare/<cmd>.go
   ```

2. Read the corresponding doc page:
   ```
   website/docs/commands/<cmd>.md
   ```

3. Compare and fix:
   - **New flags** in code → add to docs with usage example
   - **Removed flags** from code → remove from docs
   - **Changed behavior** → update description
   - **Every `--flag` in docs** must have a matching `grep` hit in source

### Step 3: Update Documentation

Apply changes following existing doc conventions:
- Match heading structure of neighboring doc pages
- Include CLI examples with expected output
- Keep flag tables consistent in format

### Step 4: Check Built-in Skill

If changes affect user-visible CLI behavior:

1. Read `skills/skillshare/SKILL.md`
2. Check if the built-in skill description needs updating
3. Verify description stays under 1024 characters (CodeX limit)

### Step 5: Check README

Review `README.md` for sections that may need updates:
- Recent Updates section
- Feature list
- Usage examples

### Step 6: Build Verification

```bash
cd website && npm run build
```

Confirm no broken links or build errors.

### Step 7: Report

List all changes made with rationale:
```
== Documentation Updates ==

Modified:
  website/docs/commands/install.md
    - Added --into flag documentation
    - Updated install examples

  skills/skillshare/SKILL.md
    - Added --into to feature list (desc: 987/1024 chars)

Build: PASS (no broken links)
```

## Rules

- **Source of truth is code** — docs must match what the code actually does
- **Every flag claim must be verified** — grep source before writing docs
- **No speculative docs** — never document planned but unimplemented features
- **No code changes** — this skill only touches `website/docs/`, `skills/skillshare/SKILL.md`, and `README.md`
- **Preserve style** — match existing doc page structure and tone
- **Built-in skill desc limit** — must stay under 1024 characters
