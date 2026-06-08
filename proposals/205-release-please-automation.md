# Feature Proposal: Automate versioning and changelog generation with release-please

Issue: [#205](https://github.com/runkids/skillshare/issues/205)

## Problem

skillshare ships at high velocity — `v0.20.5` through `v0.20.9` landed in three days. At that cadence the manual release loop becomes the highest-friction part of the workflow:

- **Handwritten changelog entries accumulate fast.** `CHANGELOG.md` is already ~195 KB / 2700+ lines, and every entry is authored by hand.
- **Version bump decisions interrupt flow.** Someone has to reason about `patch` vs `minor` vs `major` for every release.
- **Tag pushes are easy to mis-type or forget.** A wrong tag name propagates through Homebrew, `install.sh`, and all downstream consumers.

The existing `release.yaml` / GoReleaser setup is solid — the only friction is the manual steps that precede it.

## Proposed Solution

Adopt [release-please](https://github.com/googleapis/release-please) to automate the three manual steps above, leaving `release.yaml` and GoReleaser completely unchanged.

### How it works

1. Commits land on `main` using Conventional Commit messages (`feat:`, `fix:`, `perf:`, etc.).
2. release-please opens (or keeps updated) a **Release PR** — e.g. `chore(main): release v0.20.10` — containing the bumped version and generated `CHANGELOG.md` diff.
3. When ready to ship, **merge the PR**. release-please creates the `v*` tag.
4. The existing `release.yaml` fires on the tag — GoReleaser, Homebrew tap update, and contributor credits all run exactly as they do today.

Step 4 requires zero changes to `.goreleaser.yaml` or `release.yaml`.

---

### New files

#### `.github/workflows/pr-check.yml`

Enforces Conventional Commit format on every PR title so release-please has well-formed input. Without consistent commit messages the generated changelog is noisy.

```yaml
name: PR Title Check

on:
  pull_request:
    types: [opened, edited, synchronize, reopened]

jobs:
  check-title:
    name: Validate PR title
    runs-on: ubuntu-latest
    steps:
      - name: Check Conventional Commit format
        env:
          PR_TITLE: ${{ github.event.pull_request.title }}
        run: |
          # Accepts:  type(scope): description
          #           type: description
          # where type is one of the conventional-commit vocabulary
          PATTERN='^(feat|fix|chore|docs|test|ci|perf|revert|build|style|refactor)(\([^)]+\))?!?: .+'
          if echo "$PR_TITLE" | grep -Eq "$PATTERN"; then
            echo "OK: $PR_TITLE"
          else
            echo "FAIL: PR title does not follow Conventional Commits."
            echo ""
            echo "Required format:"
            echo "  type(scope): short description"
            echo "  type: short description"
            echo ""
            echo "Examples:"
            echo "  feat(cli): add zed target"
            echo "  fix: respect .skillignore in audit"
            echo "  chore(deps): bump golang.org/x/sys to v0.22"
            echo "  perf: cache skill index on first load"
            exit 1
          fi
```

#### `.github/workflows/release-please.yml`

Runs release-please on every push to `main`. Also exposes a manual escape hatch (`workflow_dispatch`) that pushes a tag directly — use this if release-please is broken or a hotfix needs to ship immediately without waiting for a Release PR merge.

```yaml
name: Release Please

on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      tag_name:
        description: 'Tag to push directly (e.g. v0.20.10). Bypasses Release PR — use only as a last resort.'
        required: true

permissions:
  contents: write
  pull-requests: write

jobs:
  release-please:
    name: Release Please
    runs-on: ubuntu-latest
    if: github.event_name == 'push'
    steps:
      - uses: googleapis/release-please-action@45996ed1f6d02564a971a2fa1b5860e934307cf7 # v4.4.0
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
          config-file: .github/release-please-config.json
          manifest-file: .github/release-please-manifest.json

  manual-tag:
    name: Push Manual Release Tag
    runs-on: ubuntu-latest
    if: github.event_name == 'workflow_dispatch'
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Validate tag format
        run: |
          TAG="${{ github.event.inputs.tag_name }}"
          if ! echo "$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
            echo "Invalid tag format: $TAG — must match vMAJOR.MINOR.PATCH"
            exit 1
          fi

      - name: Push tag
        run: |
          TAG="${{ github.event.inputs.tag_name }}"
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag "$TAG"
          git push origin "$TAG"
```

When a tag is pushed by either path, `release.yaml` fires as it always has.

#### `.github/release-please-config.json`

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "release-type": "go",
  "bump-minor-pre-major": true,
  "bump-patch-for-minor-pre-major": true,
  "changelog-sections": [
    { "type": "feat",    "section": "### New Features" },
    { "type": "fix",     "section": "### Bug Fixes" },
    { "type": "perf",    "section": "### Performance" },
    { "type": "revert",  "hidden": true},
    { "type": "docs",    "hidden": true },
    { "type": "chore",   "hidden": true },
    { "type": "test",    "hidden": true },
    { "type": "ci",      "hidden": true },
    { "type": "build",   "hidden": true },
    { "type": "style",   "hidden": true },
    { "type": "refactor","hidden": true }
  ],
  "packages": {
    ".": {}
  }
}
```

The section names map directly to the existing `CHANGELOG.md` headings (`### New Features`, `### Bug Fixes`, `### Performance`), so the generated output matches the current style without post-processing.

#### `.github/release-please-manifest.json`

Bootstrapped to the current release. release-please updates this automatically on each release.

```json
{
  ".": "0.20.9"
}
```

---

### Repository settings that must accompany this change

These are not automated by this proposal but are required for the workflow to function correctly.

#### Squash merges only

release-please reads the **merge commit message** to decide what version bump the Release PR itself represents. If regular merge commits or rebase-and-merge are allowed, the commit that release-please uses to detect its own PR may be malformed and it will open a duplicate Release PR or loop.

In **Settings → General → Pull Requests**:

- [x] Allow squash merging — set default message to "Pull request title and description"
- [ ] Allow merge commits — **disable**
- [ ] Allow rebase merging — **disable**

#### Branch protection on `main`

In **Settings → Branches → main**:

- [x] Require a pull request before merging
- [x] Require status checks to pass — add `check-title` (from `pr-check.yml`) and the existing `unit-test` job
- [x] Require branches to be up to date before merging
- [x] Do not allow bypassing the above settings (maintainer should merge the Release PR like any other PR)

#### Tag protection on `v*`

In **Settings → Tags → Protected tags**, add the pattern `v*`. This prevents a mistyped local `git push --tags` from publishing a broken release and ensures that all `v*` tags originate from CI (either the release-please job or the `manual-tag` fallback).

Exception: the `GITHUB_TOKEN` used by CI is exempt from tag protection rules by default, so `release-please.yml` and `manual-tag` both continue to work without additional configuration.

---

### Manual fallback / escape hatch

If release-please is broken, a PR is stuck, or a hotfix needs to ship immediately:

1. Go to **Actions → Release Please → Run workflow**.
2. Enter the tag to publish (e.g. `v0.20.10`).
3. The `manual-tag` job pushes the tag; `release.yaml` fires and GoReleaser runs as normal.

This means the existing fully-manual flow (local `git tag && git push --tags`) still works for the maintainer if needed — the tag protection only blocks non-CI pushes for non-maintainers.

---

### Migration path

1. Add `release-please-config.json` and seed `release-please-manifest.json` with `"0.20.9"` (the current latest).
2. Add `pr-check.yml` and `release-please.yml`.
3. Update branch protection and squash-merge settings.
4. Enable tag protection on `v*`.
5. From this point forward, use Conventional Commit PR titles. The first merge will open a Release PR.

No changes to `.goreleaser.yaml`, `release.yaml`, `docker-publish.yml`, or `test.yaml`.

---

### Conventional Commit vocabulary quick reference

| Prefix | Semver effect | Appears in changelog |
|---|---|---|
| `feat:` / `feat(scope):` | minor bump | Yes — New Features |
| `fix:` / `fix(scope):` | patch bump | Yes — Bug Fixes |
| `perf:` / `perf(scope):` | patch bump | Yes — Performance |
| `feat!:` or `BREAKING CHANGE:` footer | major bump | Yes — Breaking Changes |
| `revert:` | patch bump | Yes — Reverts |
| `docs:` `chore:` `test:` `ci:` `build:` `style:` `refactor:` | patch bump | Hidden |

The `.goreleaser.yaml` `changelog.filters.exclude` list already excludes `^docs:`, `^test:`, and `^ci:` — consistent with the hidden sections above, so GoReleaser-generated release notes and the release-please `CHANGELOG.md` will agree.

## Alternatives Considered

**Keep the manual flow.** Zero effort, but manual CHANGELOG entries and version decisions remain the highest-friction part of shipping at the current cadence.

**Use `semantic-release` instead.** More powerful but significantly more opinionated — it publishes on every push to `main` with no review gate. The release-please Release PR model gives a natural merge-when-ready gate that fits a single-maintainer project.

**Generate the changelog from GoReleaser only.** GoReleaser already generates release notes, but they live only in the GitHub release — not in `CHANGELOG.md`, and GoReleaser does not manage version bumps or tagging. release-please fills both gaps.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

New files: `.github/workflows/pr-check.yml`, `.github/workflows/release-please.yml`, `.github/release-please-config.json`, `.github/release-please-manifest.json` — all configuration, no Go source changes.

## Open Questions

- **CHANGELOG.md bootstrap.** The existing `CHANGELOG.md` is 2700+ lines of manually authored content. release-please will prepend generated entries above it. Should the file be kept as-is (clean separation), or should the old content be archived to a separate file on migration?
- **Release Gated:** Releases are fully gated, a release will not be published until the core maintainer merges the PR. Alternatively, if ever needed you have full control to create a release manually via the CI by supplying a tag i.e. `v0.22.0`

## Additional Comments
If you are interested to see this in a live project see the following projects that I have implemented this in:

1. [ssmctl](https://github.com/rhysmcneill/ssmctl/pull/108)
2. [skylos](https://github.com/duriantaco/skylos/pull/537)
3. [typo](https://github.com/yuluo-yx/typo/pull/170)

Furthermore, I have implemented this in many more personal, and professional repos (private, not public). This workflow will enhance your workflow, automate everything, you just need to push the button to release when you are happy - It will allow you to focus on things that matter - developing, and fixing bugs. :) 