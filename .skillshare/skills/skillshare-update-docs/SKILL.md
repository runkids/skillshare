---
name: skillshare-update-docs
description: >-
  Update website docs to match recent code changes, cross-validating every flag
  against source. Use this skill whenever the user asks to: update documentation,
  sync docs with code, document a new flag or command, fix stale docs, or update
  the README. This skill covers all website/docs/ categories (commands, reference,
  understand, how-to, troubleshooting, getting-started) plus the built-in skill
  description and README. If you just implemented a feature and need to update
  docs, this is the skill to use. Never manually edit website docs without
  cross-validating flags against Go source first.
argument-hint: "[command-name | commit-range]"
metadata: 
  targets: [claude, universal]
---

Sync website documentation with recent code changes. $ARGUMENTS specifies scope: a command name (e.g., `install`), commit range, or omit to auto-detect from `git diff HEAD~1`.

**Scope**: This skill updates `website/docs/`, the built-in skill (`skills/skillshare/`), and `README.md`. It does NOT write Go code (use `implement-feature`) or CHANGELOG (use `changelog`).

Before acting, run `python3 scripts/ai-context.py documentation`. That topic is the source of truth for documentation ownership, code cross-validation and verification; this skill retains the update workflow.

## Workflow

### Step 1: Detect Changes

```bash
# Auto-detect recently changed code
git diff HEAD~1 --stat -- cmd/skillshare/ internal/

# Also check for structural changes that affect concept/reference docs
git diff HEAD~1 --stat -- internal/config/targets.yaml internal/audit/rules.yaml
```

Map changed files to affected documentation using this guide:

**Command docs** (`website/docs/reference/commands/`):
- `cmd/skillshare/<cmd>.go` → `website/docs/reference/commands/<cmd>.md`
- Flag changes, new subcommands, output format changes

**Concept docs** (`website/docs/understand/`):
- `internal/audit/` → `understand/audit-engine.md`
- `internal/sync/` → `understand/sync-modes.md`, `understand/source-and-targets.md`
- `internal/install/tracked.go` → `understand/tracked-repositories.md`
- `internal/config/` → `understand/declarative-manifest.md`
- `.skillshare/` project config changes → `understand/project-skills.md`
- `skills/skillshare/SKILL.md` format → `understand/skill-format.md`

**Reference docs** (`website/docs/reference/`):
- `internal/config/targets.yaml` → `reference/targets/`
- `internal/audit/rules.yaml` → `reference/commands/audit-rules.md`
- `reference/appendix/` for CLI quick-reference tables

**How-to guides** (`website/docs/how-to/`):
- New workflow patterns → `how-to/daily-tasks/`, `how-to/advanced/`, `how-to/recipes/`
- Sharing/org features → `how-to/sharing/`

**Troubleshooting** (`website/docs/troubleshooting/`):
- New error messages → `troubleshooting/common-errors.md`
- FAQ additions → `troubleshooting/faq.md`

**Getting started** (`website/docs/getting-started/`):
- Breaking changes to init/install flow → `getting-started/first-sync.md`
- Quick reference updates → `getting-started/quick-reference.md`

**Learn** (`website/docs/learn/`):
- New target integrations → `learn/with-<tool>.md`

### Step 2: Cross-Validate Flags

For each affected command:

1. Read the Go source to extract actual flags and behavior. Commands parse arguments by hand, so flags appear as string literals (`"--force"`) in the parse function of `cmd/skillshare/<cmd>.go`; `ss <cmd> --help` inside the devcontainer lists them too.

2. Read the corresponding doc page:
   ```
   website/docs/reference/commands/<cmd>.md
   ```

3. Compare and fix:
   - **New flags** in code → add to docs with usage example
   - **Removed flags** from code → remove from docs
   - **Changed behavior** → update description
   - **Every `--flag` in docs** must appear as a string literal in the command's source

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
- Recent Updates callout
- Why skillshare bullet points (5 selling points)
- Highlights section (core feature examples)

### Step 6: Build Verification

```bash
docker exec "$CONTAINER" bash -c 'cd /workspace/website && npm run build'   # as the Website Pages workflow builds
```

Confirm no broken links or build errors.

### Step 7: Report

List all changes made with rationale:
```
== Documentation Updates ==

Modified:
  website/docs/reference/commands/install.md
    - Added --into flag documentation
    - Updated install examples

  skills/skillshare/SKILL.md
    - Added --into to feature list (desc: 987/1024 chars)

Build: PASS (no broken links)
```

## Rules

Apply the `documentation` topic. Keep this adapter scoped to documentation; use the implementation or release workflows for other changes.
