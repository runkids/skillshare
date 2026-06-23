---
name: skillshare-release
description: >-
  End-to-end release workflow for skillshare. Runs tests, generates changelog
  (via /changelog), optionally writes local RELEASE_NOTES, updates version
  numbers, commits, and drafts announcements. Use when the user says "release",
  "prepare release", "cut a release", "release v0.19", or any request to
  publish a new version. For changelog-only tasks, use /changelog instead.
argument-hint: "<version>"
metadata:
  targets: [claude, universal]
---

End-to-end release workflow for skillshare. $ARGUMENTS specifies the version (e.g., `v0.19.0`).

## Prerequisites

- All feature work merged to current branch
- Working directory clean (`git status` shows no uncommitted changes)

## Workflow

### Phase 1: Validate

Run full test suite and code quality checks. Fix any failures before proceeding.

```bash
make check   # fmt-check + lint + test (builds binary first)
```

If tests fail: fix them, don't skip. Do not ask the user — fix and re-run.

### Phase 2: Changelog

Invoke `/changelog $VERSION` to generate the changelog entry.

This handles:
- Collecting commits since last tag
- Categorizing by conventional commit type
- Writing user-facing CHANGELOG.md entry
- Syncing website changelog (`website/src/pages/changelog.md`)

Review the output before proceeding.

### Phase 3: Release Notes Draft (Maintainer Only, Local by Default)

Check if running as maintainer:
```bash
git config user.name  # Should match "Willie" or maintainer identity
```

**If maintainer**:

Read the most recent `specs/RELEASE_NOTES_*.md` as a style reference, then generate `specs/RELEASE_NOTES_<version>.md` (no `v` prefix, e.g., `RELEASE_NOTES_0.19.0.md`).

Release notes are a local maintainer artifact by default. The `specs/` directory is gitignored; do **not** force-add or commit `specs/RELEASE_NOTES_<version>.md` unless the user explicitly asks for release notes to be committed.

Structure:
- Title: `# skillshare vX.Y.Z Release Notes`
- TL;DR section with numbered highlights
- One `##` section per feature/fix — describe **what changed** in plain language, with a CLI example or code block if relevant
- Include migration guide if breaking changes exist

**Wording rules** (same user-facing standard as CHANGELOG):
- Describe **what changed** from the user's perspective, not how the code changed
- **Never mention**: function names, variable names, struct fields, file paths, Go syntax, internal APIs
- ✅ Good: "Sync now auto-creates missing target directories and shows what it did"
- ❌ Bad: "upgraded `Server.mu` from `sync.Mutex` to `sync.RWMutex` and applied a snapshot pattern across 30 handlers"
- Keep it concise — a short paragraph per feature is enough

**If not maintainer**: Skip this phase.

### Phase 4: Version Bump

Update the version in `skills/skillshare/SKILL.md` frontmatter:

```yaml
metadata:
  version: vX.Y.Z
```

This ensures `skillshare upgrade --skill` detects the new version correctly.

### Phase 5: Commit & Tag

```bash
git add CHANGELOG.md website/src/pages/changelog.md skills/skillshare/SKILL.md
# Only if the user explicitly asked to commit release notes:
# git add -f specs/RELEASE_NOTES_<version>.md
git commit -m "chore: release vX.Y.Z"
git tag vX.Y.Z
```

Do NOT push yet — wait for user confirmation.

### Phase 6: Draft Announcements

Prepare two drafts for user review:

1. **GitHub Release Notes** — concise, user-facing summary suitable for the GitHub release page. Shorter than RELEASE_NOTES, highlight top 3-5 changes with one-liners.

2. **Social media post** — 2-3 sentences max, casual tone, mention the version and 1-2 headline features. No hashtag spam.

**Tone**: short, direct. Don't oversell. The user will edit before posting.

### Phase 7: Present & Confirm

Show the user:
- [ ] Test results (pass/fail)
- [ ] CHANGELOG.md diff
- [ ] RELEASE_NOTES file (if generated, and whether it was left local or committed)
- [ ] Version bump diff
- [ ] GitHub release draft
- [ ] Social media draft

Wait for user approval before pushing:
```bash
git push origin HEAD --tags
```

## Rules

- **Fix, don't skip** — if tests fail, fix them before continuing
- **User perspective** — all written output is for users, not developers
- **Short drafts** — announcements default to concise; user will ask for more detail if needed
- **No fabricated links** — never invent URLs or references
- **Verify before claiming** — grep source before stating a feature exists
- **Ask before push** — never push or publish without explicit user confirmation
- **Release notes stay local by default** — never force-add or commit `specs/RELEASE_NOTES_*.md` unless the user explicitly asks
- **Commit message** — always `chore: release vX.Y.Z`
- **No competitive references** — never mention competitor repos in commit messages or notes
