---
name: skillshare-changelog
description: >-
  Generate CHANGELOG.md entry from recent commits in conventional format. Also
  syncs the website changelog page. Use this skill whenever the user asks to:
  write release notes, generate a changelog, prepare a version release, document
  what changed between tags, or create a new CHANGELOG entry. If you see
  requests like "write the changelog for v0.17", "what changed since last
  release", or "prepare release notes", this is the skill to use. Do NOT
  manually edit CHANGELOG.md without this skill — it ensures proper formatting,
  user-perspective writing, and website changelog sync.
argument-hint: "[tag-version]"
targets: [claude, codex]
---

Generate a CHANGELOG.md entry for a release. $ARGUMENTS specifies the tag version (e.g., `v0.16.0`) or omit to auto-detect via `git describe --tags --abbrev=0`.

**Scope**: This skill updates `CHANGELOG.md` and syncs the website changelog (`website/src/pages/changelog.md`). It does NOT write code (use `implement-feature`) or update docs (use `update-docs`).

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

### Step 4: Read Existing Entries for Style Reference

Before writing, read the most recent 2-3 entries in `CHANGELOG.md` to match the established tone and structure. The style evolves over time — always match the latest entries, not a hardcoded template.

### Step 5: Write User-Facing Entry

Write from the **user's perspective**. Only include changes users will notice or care about.

**Include**:
- New features with usage examples (CLI commands, code blocks)
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
- Use em-dash (`—`) to separate feature name from description
- Group related features under `####` sub-headings when there are 2+ distinct areas

### Step 6: Update CHANGELOG.md

Read existing `CHANGELOG.md` and insert new entry at the top, after the header. Match the style of the most recent entries exactly.

Structural conventions (based on actual entries):
```markdown
## [X.Y.Z] - YYYY-MM-DD

### New Features

#### Feature Area Name

- **Feature name** — description with `inline code` for commands and flags
  ```bash
  skillshare command --flag    # usage example
  ```
  Additional context as sub-bullets or continuation text

#### Another Feature Area

- **Feature name** — description

### Bug Fixes

- Fixed specific user-visible behavior — with context on what changed
- Fixed another issue

### Performance

- **Improvement name** — description of what got faster

### Breaking Changes

- Renamed `old-name` to `new-name`
```

Key style points:
- Version numbers use `[X.Y.Z]` without `v` prefix in the heading
- Feature bullets use `**bold name** — em-dash description` format
- Code blocks use `bash` language tag for CLI examples
- Bug fixes describe the symptom, not the implementation
- Only include sections that have content (skip empty Performance, Breaking Changes, etc.)

### Step 7: Sync Website Changelog

The website has its own changelog page at `website/src/pages/changelog.md`. After updating `CHANGELOG.md`, sync the new entry to the website version.

**Differences between the two files**:
- Website file has MDX frontmatter (`title`, `description`) and an intro paragraph — preserve these, don't overwrite
- Website file has a `---` separator after the intro, before the first version entry
- The release entries themselves are identical in content

**How to sync**: Read the website changelog, then insert the same new entry after the `---` separator (line after intro paragraph), before the first existing version entry. Do NOT replace the entire file — only insert the new entry block.

### Step 8: RELEASE_NOTES (Maintainer Only)

`specs/RELEASE_NOTES_<version>.md` is only generated when the user is the project maintainer (runkids). Contributors skip this step.

Check if running as maintainer:
```bash
git config user.name  # Should match maintainer identity
```

If maintainer:
- Read `specs/RELEASE_NOTES_0.16.12.md` (or the most recent existing one) as a style reference
- Generate `specs/RELEASE_NOTES_<version>.md` (no `v` prefix, e.g. `RELEASE_NOTES_0.16.13.md`) matching that style:
  - Title: `# skillshare vX.Y.Z Release Notes`
  - TL;DR section with numbered highlights
  - Feature sections with "The problem" / "Solution" / "Design decisions" / "Usage patterns" structure
  - Bug Fixes section with bold labels
- Include migration guide if breaking changes exist

**RELEASE_NOTES wording rules** (same user-facing standard as CHANGELOG):
- Bug Fixes describe **what the user experiences**, not how the code changed
- **Never mention**: function names, variable names, struct fields, file paths, Go syntax, internal APIs
- ✅ Good: "fixed a potential crash when multiple browser tabs hit the dashboard"
- ❌ Bad: "upgraded `Server.mu` from `sync.Mutex` to `sync.RWMutex` and applied a snapshot pattern across 30 handlers"
- Feature sections ("The problem" / "Solution") may reference CLI flags and commands but not internal implementation
- "Design decisions" can be slightly more technical but should still focus on *what* was chosen and *why*, not *how* the code works

If not maintainer:
- Skip RELEASE_NOTES generation
- Only update CHANGELOG.md + website changelog

## Rules

- **User perspective** — write for users, not developers
- **No fabricated links** — never invent URLs or references
- **Verify features exist** — grep source before claiming a feature was added
- **No internal noise** — exclude test-only, CI-only, or refactor-only changes
- **Conventional format** — follow existing CHANGELOG.md style exactly
- **Always sync both** — `CHANGELOG.md` and `website/src/pages/changelog.md` must have identical release entries
- **RELEASE_NOTES = maintainer only** — contributors only update CHANGELOG.md + website changelog
