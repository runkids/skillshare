---
name: skillshare-changelog
description: Generate CHANGELOG.md entry from recent commits in conventional format
argument-hint: "[tag-version]"
targets: [claude, codex]
---

Generate a CHANGELOG.md entry for a release. $ARGUMENTS specifies the tag version (e.g., `v0.16.0`) or omit to auto-detect via `git describe --tags --abbrev=0`.

**Scope**: This skill only updates `CHANGELOG.md`. It does NOT write code (use `implement-feature`) or update docs (use `update-docs`).

## Workflow

### Step 1: Determine Version Range

```bash
# Auto-detect latest tag
LATEST_TAG=$(git describe --tags --abbrev=0)
# Find previous tag
PREV_TAG=$(git describe --tags --abbrev=0 "${LATEST_TAG}^")

echo "Generating changelog: $PREV_TAG → $LATEST_TAG"
```

### Step 2: Collect Commits

```bash
git log "${PREV_TAG}..${LATEST_TAG}" --oneline --no-merges
```

### Step 3: Categorize Changes

Group commits by conventional commit type:

| Prefix | Category |
|--------|----------|
| `feat` | New Features |
| `fix` | Bug Fixes |
| `refactor` | Refactoring |
| `docs` | Documentation |
| `perf` | Performance |
| `test` | Tests |
| `chore` | Maintenance |

### Step 4: Write User-Facing Entry

Write from the **user's perspective**. Only include changes users will notice or care about.

**Include**:
- New features with usage examples (CLI commands)
- Bug fixes that affected user-visible behavior
- Breaking changes (renames, removed flags, scope changes)
- Performance improvements users would notice

**Exclude**:
- Internal test changes (smoke tests, test refactoring)
- Implementation details (error propagation, internal structs)
- Dev toolchain changes (Makefile cleanup, CI tweaks)
- Pure documentation adjustments

**Wording guidelines**:
- Don't use "first-class", "recommended" for non-default options
- Be factual: "Added X" / "Fixed Y" / "Renamed A to B"
- Include CLI example when introducing a new feature

### Step 5: Update CHANGELOG.md

Read existing `CHANGELOG.md` and insert new entry at the top, after the header.

Format:
```markdown
## [vX.Y.Z] - YYYY-MM-DD

### New Features
- **Feature name**: Brief description (`skillshare command --flag`)

### Bug Fixes
- Fixed issue where X happened when Y

### Breaking Changes
- Renamed `old-name` to `new-name`
```

### Step 6: RELEASE_NOTES (Maintainer Only)

**IMPORTANT**: `specs/RELEASE_NOTES_<version>.md` is only generated when the user is the project maintainer (runkids). Contributors should skip this step.

Check if running as maintainer:
```bash
git config user.name  # Should match maintainer identity
```

If maintainer:
- Generate `specs/RELEASE_NOTES_${LATEST_TAG}.md` with detailed release notes
- Include migration guide if breaking changes exist

If not maintainer:
- Skip RELEASE_NOTES generation
- Only update CHANGELOG.md

## Rules

- **User perspective** — write for users, not developers
- **No fabricated links** — never invent URLs or references
- **Verify features exist** — grep source before claiming a feature was added
- **No internal noise** — exclude test-only, CI-only, or refactor-only changes
- **Conventional format** — follow existing CHANGELOG.md style exactly
- **RELEASE_NOTES = maintainer only** — contributors only update CHANGELOG.md
