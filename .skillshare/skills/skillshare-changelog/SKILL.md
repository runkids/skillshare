---
name: skillshare-changelog
description: >-
  Generate CHANGELOG.md entry from recent commits in conventional format. Also
  syncs the website changelog page. Use this skill whenever the user asks to:
  generate a changelog, document what changed between tags, or create a new
  CHANGELOG entry. If you see requests like "write the changelog for v0.17",
  "what changed since last release", this is the skill to use. Do NOT manually
  edit CHANGELOG.md without this skill — it ensures proper formatting,
  user-perspective writing, and website changelog sync. For full release
  workflows (Release PR review, tests, draft assets, publication, announcements),
  use /release instead.
argument-hint: "[tag-version]"
metadata:
  targets: [claude, universal]
---

Review or generate a user-facing changelog entry. Prefer the open Release Please PR and its manifest version for an upcoming release. For historical work, $ARGUMENTS specifies the requested tag or commit range.

**Scope**: This skill updates `CHANGELOG.md` and syncs the website changelog (`website/src/pages/changelog.md`). It does NOT generate RELEASE_NOTES, choose a new version, or handle the full release workflow — use `/release` for that.

Before acting, run `python3 scripts/ai-context.py release`. That topic is the source of truth for authorization, changelog content and synchronization rules; this skill retains the entry-generation workflow.

## Workflow

### Step 1: Determine Version Range

For an upcoming release, work on the Release PR branch and read `.github/release-please-manifest.json`. Verify the latest published tag and use its range to the Release PR head. Keep the proposed version; this skill does not authorize a version change. Tags may exist for unpublished drafts, so do not assume the highest local tag is the latest published release.

For historical work, use the user-requested tags or range.

### Step 2: Collect Commits

```bash
git log <previous-published-tag>..<release-pr-head-or-requested-tag> --oneline --no-merges
```

Review the generated Release Please entry before writing. Preserve useful manual examples and migration notes. Do not add a second entry for the same version.

### Step 3: Categorize Changes

Group commits by conventional commit type:

| Prefix | Category |
|--------|----------|
| `feat` | New Features |
| `fix` | Bug Fixes |
| `perf` | Performance |
| breaking change | Breaking Changes and migration notes |
| `revert` | Reverts, when user-visible |

Exclude test-only, CI-only, pure refactoring, documentation-only and internal maintenance entries from published notes.

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

Edit the existing generated entry on the Release PR branch. If the requested historical entry does not exist, insert it at the appropriate position. Match the style of the most recent entries.

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

For an upcoming Release PR, run inside the devcontainer:

```bash
python3 scripts/release/release.py sync
python3 scripts/release/release.py check
```

The helper normalizes the generated version heading, replaces or prepends only the newest website entry, and synchronizes the built-in skill metadata to the already proposed manifest version. It preserves website frontmatter, intro and historical entries. It does not choose or bump a version.

For historical corrections, edit the matching entry in both files directly; the helper handles only the current manifest release. Review the diff and run `python3 scripts/ai-context.py check`.

## Rules

Apply the `release` topic. This adapter is changelog-only and does not broaden authorization to commit, tag, push or publish.
