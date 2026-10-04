# Changelog and Release

Use when writing a version changelog, release notes, version bump, tag, announcement, or full release.

## Authorization Boundary

A changelog-only request does not authorize commits, tags, pushes, publication, or version bumps. A full release request authorizes preparation and explicitly included local Git actions, but pushing or publishing still requires user confirmation. Never mix unrelated working-tree changes into staging.

## Changelog

1. Determine the range from the requested version or tags, then collect commits with `git log <previous>..<current> --no-merges`.
2. Read the newest two or three `CHANGELOG.md` entries and follow the current style.
3. Categorize user-visible features, bug fixes, performance improvements, and breaking changes. Exclude test-only, CI-only, pure refactoring, and internal implementation noise.
4. Verify every feature claim against source and never fabricate links.
5. Add the entry at the top of `CHANGELOG.md` and insert the same release entry after the frontmatter/intro separator in `website/src/pages/changelog.md`.

Version headings use `[X.Y.Z]` without a `v` prefix and include the date. Feature bullets use a bold name, an em dash, and a real usage example. Create only sections that contain content.

## Full Release

### Release Please automation

The release version is recorded in `.github/release-please-manifest.json`. The initial baseline is the published `v0.23.1` commit, also pinned as `bootstrap-sha` in `.github/release-please-config.json`; do not bump it merely to install automation. The development CLI defaults to `dev`, while GoReleaser and release Docker builds inject the approved version.

For `0.x`, fixes and performance changes increment patch; new features and breaking changes increment minor. Breaking changes require migration notes. Moving to `1.0.0` is an explicit maintainer decision. Use a `Release-As: X.Y.Z` commit footer or a reviewed `release-as` configuration override for a specific version; remove a configuration override after that release. Do not edit the manifest alone to request a bump.

For an explicit version such as `0.23.3`, put `Release-As: 0.23.3` after a blank line in a normal development commit that will reach `main` before merging the Release PR. A single footer on the final commit of a batch is sufficient for the entire pending release; intermediate versions may be skipped. Preserve the footer when merging or rebasing. For squash-merge, copy it into the final squash commit message on GitHub. If pending commits contain conflicting version requests, the newest override wins. Verify the resulting Release PR's proposed version before merging; the footer itself neither tags nor publishes a release. See the [Release Please version override documentation](https://github.com/googleapis/release-please#how-do-i-change-the-version-number).

1. A push to `main` runs **Release Please**. With no merged release pending, it creates or updates a Release PR and synchronizes its manifest version, built-in skill version and newest changelog entry in both files.
2. Review the proposed version and user-facing notes on the Release PR branch. `python3 scripts/release/release.py sync` normalizes the heading and syncs the files; `check` verifies their agreement. Run these helpers inside the devcontainer. Bot PR creation and updates using `GITHUB_TOKEN` may leave PR CI awaiting the maintainer's native **Approve workflows to run** prompt.
3. Merging the Release PR selects its exact merge SHA for the reusable Test workflow. Formatting, lint, Go unit/integration tests, release helper tests, Docker sandbox tests and applicable red-team jobs must pass before tagging. If another merged pending release appears during testing, the tag job stops. The ordinary push run of Test skips that merge commit, because this gate already tests it; the Release PR's own runs still execute as required checks.
4. Release Please creates the stable tag and an unpublished draft with `force-tag-creation`. The workflow explicitly calls **Build Release Draft**; it does not depend on a `GITHUB_TOKEN` tag push triggering another workflow.
5. GoReleaser builds all six CLI archives, UI assets and an unpublished Homebrew formula. The draft remains unpublished. The helper verifies filenames, all checksums, formula version/URLs and the actual native Linux CLI version. Contributors are added to the reviewed changelog-based release notes.
6. After reviewing the draft, explicitly authorize **Publish Release** with the exact tag. It verifies the downloaded assets again before publication, then updates Homebrew and explicitly calls Docker Publish and Website Pages. Release Docker images and the website use the same tag commit. Draft tags never publish Docker images, update the tap or deploy the website; pushes to `main` do not deploy it either.

For automation setup, the repository must allow GitHub Actions to create pull requests under **Settings > Actions > General > Workflow permissions**. Workflow YAML grants scoped write permissions; it does not change repository settings or native approval rules. Keep Conventional Commit messages on `main`; ordinary merge and squash-merge of a Release PR are both supported.

For failed draft packaging, rerun the failed packaging job or dispatch **Build Release Draft** for its existing tag. Manual draft rebuilds test the tag commit again. For a publication/distribution failure, rerun **Publish Release** for the same tag; it tolerates an already published release and unchanged tap formula. Docker Publish also has a published-tag recovery input, and Website Pages can be dispatched with a tag or branch to redeploy or ship a docs-only fix. Older versions cannot overwrite current distribution. Do not recreate a tag, force-push, or publish directly through the draft page as a substitute for the complete workflow.

### Local release preparation

Before starting:

- Confirm the requested version and branch.
- Use `git status` to separate task-owned changes from existing work.
- Run the complete `make check` inside the devcontainer and never skip a failure.

Then:

1. Bring the documentation up to date with the pending range `<latest published tag>..origin/main`, following the `documentation` topic: check each user-visible feature, fix and breaking change against the command pages, guides, troubleshooting, built-in skill and README in every locale, then build the whole website inside the devcontainer with `npm run build` in `website/`, as the Website Pages workflow does. For a minor or major release, also rewrite the `Latest` callout in `README.md` and every translated README, and add the range's contributors to the `README.md` Contributors section: authors of the issues and PRs it references, commit authors and `Co-authored-by` trailers, other than the maintainer and anyone already listed. Documentation fixes land on `main`, so finish them before editing the Release PR. A push to `main` whose commits change the generated notes, such as a new `feat` or `fix`, regenerates the Release PR and discards edits made on its branch; `docs` commits leave it as it is.
2. Generate and review the changelog.
3. Write the GitHub release body to `specs/RELEASE_NOTES_<version>.md`, following the newest one. After the draft is built, apply it with `gh release edit vX.Y.Z --notes-file specs/RELEASE_NOTES_<version>.md`; Publish Release keeps it, but rebuilding the draft regenerates the body from `CHANGELOG.md`.
4. Review the manifest's proposed version and synchronize the Release PR files; do not perform a separate manual version bump.
5. Commit review edits or merge the Release PR only when explicitly requested. Stage only task-owned files. The automation owns release tagging after verification.
6. Draft concise GitHub release notes and a social announcement.
7. Present tests, diffs, local-only artifacts, and commit/tag state; obtain confirmation before pushing or publishing.

Release notes are local maintainer artifacts by default. Never force-add an ignored file unless explicitly requested. Never infer authorization for credentials, registry login, or external publication.

## Verification and Report

Report actual test results, whether both changelog copies match, the verified version location, commit/tag state, and external actions not performed. If the working tree is dirty, list the unrelated changes that were preserved.

Release automation checks: run `python3 -m unittest discover -s scripts/release -p '*_test.py'` and `python3 scripts/release/release.py check` inside the devcontainer. Validate changed workflows with actionlint and validate `.goreleaser.yaml` with the pinned GoReleaser version. A local check cannot prove GitHub permissions, native CI approvals or external publication; report those separately.

GoReleaser v2.18.2 reports the existing `brews` generator as deprecated, so `goreleaser check` exits with status 2 despite a valid configuration. A full isolated `release --skip=publish` succeeds with validation enabled. Keep the current Homebrew formula interface for this migration; switching to the replacement cask requires a separate distribution decision.
