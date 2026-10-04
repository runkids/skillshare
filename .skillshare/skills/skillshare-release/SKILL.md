---
name: skillshare-release
description: >-
  Prepare and review skillshare releases using the Release Please PR, verify the
  proposed version and changelog, inspect draft assets, and publish through the
  manual Publish Release workflow when explicitly authorized. Use when the user
  says "release", "prepare release", "cut a release", or asks to publish a new
  version. For changelog-only tasks, use /changelog instead.
argument-hint: "[version]"
metadata:
  targets: [claude, universal]
---

Prepare or review a skillshare release. An optional version argument is an explicit requested version, not permission to commit, merge, tag, push, or publish.

Before acting, run `python3 scripts/ai-context.py release testing`. These topics are the source of truth for authorization, version policy, automation and verification.

## Workflow

### 1. Identify the release

Inspect Git status, the latest published release and the open Release PR. Preserve unrelated work. The manifest at `.github/release-please-manifest.json` is the release version source; the development CLI still defaults to `dev`.

Pushes to `main` create or update a Release PR. If one is missing, report the workflow state and required repository permissions. Dispatch the Release Please workflow only when the user authorizes that external action.

For `0.x`, `fix` and `perf` increment patch; `feat` and breaking changes increment minor. A breaking change needs migration notes. Moving to `1.0.0` requires an explicit version decision. For an override, use a `Release-As: X.Y.Z` commit footer or a reviewed `release-as` configuration value; remove a configuration override after it has been consumed. Do not edit the tracking manifest by itself to request a release.

#### Explicit version requests

When the user requests a version such as `0.23.3`, prepare a `Release-As: 0.23.3` footer on a normal development commit that will reach `main` before the Release PR is merged. For a batch with multiple commits, adding it once to the final commit is sufficient; the override chooses the version for the entire pending release, including the earlier changes. Intermediate versions do not need to be published.

```text
fix: correct sync behavior

Release-As: 0.23.3
```

With merge or rebase-merge, preserve the footer in the development commit. With squash-merge, copy it into the final squash commit message on GitHub; do not assume the individual commit messages will be preserved. Keep the Conventional Commit subject and a blank line before the footer. If multiple pending commits request different versions, the newest override wins; inspect the pending range for conflicting requests before proceeding.

After the commit reaches `main`, verify that Release Please creates or updates the Release PR to the requested version. Review its manifest and synchronized metadata before merging. Adding a footer alone does not create the release tag or publish anything. Follow the existing authorization boundary when committing, pushing or merging.

### 2. Sync documentation first

Before touching the Release PR, run `/skillshare-update-docs` over the pending range `<latest published tag>..origin/main`. Check every user-visible `feat`, `fix` and breaking change against the command pages, guides, troubleshooting, built-in skill and README, in English and every translated locale, and note the commits that need no documentation change. Then build the whole website inside the devcontainer the way the Website Pages workflow does:

```bash
cd website && npm run build
```

For a minor or major release (any `feat` or breaking change in the range), also update the README:

- Rewrite the `Latest` callout near the top of `README.md` for the new version, and the same callout in every translated README (`README-ja.md`, `README-ko.md`, `README-zh-CN.md`, `README-zh-TW.md`), each in its own language.
- Add the release's contributors to the `Contributors` section of `README.md` (the translations link to it). A contributor is the author of an issue or PR the range references, or a commit author or `Co-authored-by` in the range, other than the maintainer. Skip anyone already listed, and verify each account exists before adding its avatar link:

  ```bash
  git log <tag>..origin/main --format='%B' | grep -oE '#[0-9]+' | sort -u
  gh api repos/runkids/skillshare/issues/<n> --jq '.user.login'
  git log <tag>..origin/main --format='%an%n%(trailers:key=Co-authored-by,valueonly)' | sort -u
  ```

Documentation fixes go to `main`, with the usual authorization to commit and push. Finish them before the next step. A push to `main` whose commits change the generated notes, such as a new `feat` or `fix`, regenerates the Release PR and discards edits made on its branch; `docs` commits leave it as it is (the Release Please log says the PR "remained the same").

### 3. Review the Release PR

Use `/changelog` to turn generated notes into user-facing prose and examples. Work on the Release PR branch, then synchronize and check its files inside the devcontainer:

```bash
python3 scripts/release/release.py sync
python3 scripts/release/release.py check --tag vX.Y.Z
make check
python3 -m unittest discover -s scripts/release -p '*_test.py'
```

Check the manifest, built-in skill metadata and both changelog copies. Bot-created or bot-updated PRs may require the maintainer to approve GitHub's native workflow prompt before CI runs. Never bypass that prompt.

Commit and push review edits, or merge the PR, only when explicitly authorized. Do not create a release tag locally: the automation tests the exact merged commit before creating the tag and draft.

### 4. Inspect the draft

After the Release PR merges, the Release Please workflow tests its merge SHA, creates a draft and tag, and explicitly calls Build Release Draft. The workflow builds all six CLI archives and the UI archive, generates the Homebrew formula, verifies checksums and checks the packaged CLI version.

Review the draft notes, migration guidance, tag and artifacts. A failed packaging job leaves an unpublished draft. Rerun the failed packaging job or, with authorization, dispatch Build Release Draft for that existing tag; do not create a second tag or change the version to hide a build failure.

### 5. Release notes and announcements

The published GitHub release body is `specs/RELEASE_NOTES_<version>.md`, not the CHANGELOG entry. Follow the most recent `specs/RELEASE_NOTES_*.md` (TL;DR, then one section per area) and verify every claim against source. The ignored `specs/` files stay local unless the user explicitly asks to commit them. Do not force-add them by default.

After the draft is built, replace its body with the notes. The draft is unpublished, so this needs no publication authorization; Publish Release keeps the body:

```bash
gh release edit vX.Y.Z --notes-file specs/RELEASE_NOTES_X.Y.Z.md
```

Rebuilding the draft (Build Release Draft) regenerates the body from the CHANGELOG entry, so apply the notes again after any rebuild. If the draft listed external contributors, keep that section.

Draft announcements only when requested. Describe user-visible behavior, add examples and migration guidance, and avoid internal implementation details.

### 6. Publish only when authorized

Present the concrete draft and verification results. If publication is explicitly authorized, dispatch **Publish Release** with the exact `vX.Y.Z` tag. It verifies downloaded assets and the packaged CLI before publishing, then updates Homebrew and explicitly runs Docker Publish and Website Pages for the pinned tag commit. The website deploys only here, never on pushes to `main`.

Use this workflow rather than publishing directly from the GitHub draft page, so the complete distribution path runs. For a partial distribution failure, rerun the failed job with the same tag; never retag or publish an older version over the current one.

## Report

State the reviewed version, Release PR and draft, test results, artifact checks, and which external actions were actually performed. Distinguish local preparation, an unpublished draft, a published release and completed distribution.
