# Feature Proposal: Automate versioning and changelog generation with release-please

Issue: [#205](https://github.com/runkids/skillshare/issues/205)

## Problem

skillshare ships at high velocity — `v0.20.5` through `v0.20.9` landed in three days. At that cadence the manual release loop becomes the highest-friction part of the workflow:

- **Handwritten changelog entries accumulate fast.** `CHANGELOG.md` is already ~195 KB / 2700+ lines, and every entry is authored by hand.
- **Version bump decisions interrupt flow.** Someone has to reason about `patch` vs `minor` vs `major` for every release.
- **Tag pushes are easy to mis-type or forget.** A wrong tag name propagates through Homebrew, `install.sh`, and all downstream consumers.

The existing `release.yaml` / GoReleaser setup is solid — the only friction is the manual steps that precede it.

## Implemented design

The configuration and workflows implement a version-first, draft-first release process. This supersedes the original proposal's assumption that a `GITHUB_TOKEN` tag push would start the existing workflows unchanged. See [the release runbook](../wiki/release.md) for operating instructions.

### Version policy

The tracking manifest starts at the published `0.23.1`, with its exact commit as the bootstrap boundary. Existing changelog history is retained.

- `fix` and `perf`: patch increment.
- `feat`: minor increment, including before 1.0.
- Breaking change: minor increment before 1.0, with migration notes; major increment after 1.0.
- `1.0.0` or another explicitly chosen version: a `Release-As: X.Y.Z` commit footer or a reviewed `release-as` configuration override, removed after use.
- Documentation, tests and internal maintenance do not create ordinary release entries by themselves.

The manifest, built-in skill and both changelog copies must agree. Tag, binary version, archive names, Homebrew formula and Docker images use that same approved version.

### Workflow

1. **Release Please** runs on pushes to `main`, creating or updating a Release PR without tagging or publication. A post-processing helper normalizes the generated heading, updates the built-in skill metadata, and synchronizes only the newest website changelog entry while preserving frontmatter and history.
2. The maintainer reviews the version, notes, examples and migration instructions before merging. Both ordinary merge and squash-merge are supported. Conventional Commit messages are required for meaningful version calculation, but this change does not modify merge permissions or branch protection.
3. For a merged pending Release PR, the existing Test workflow checks its exact merge SHA. The pipeline checks that this is still the only pending candidate before proceeding.
4. Release Please creates the `vX.Y.Z` tag and a GitHub draft (`draft` and `force-tag-creation` enabled). Packaging is an explicit reusable-workflow call because `GITHUB_TOKEN` tag pushes do not trigger other workflows.
5. **Build Release Draft** uses GoReleaser to build all CLI platforms and UI assets. Homebrew upload is disabled during draft construction. Its generated formula is attached to the draft and included in checksums. Checksums, formula identity and the actual Linux CLI version are verified.
6. The maintainer runs **Publish Release** with the reviewed tag. The workflow downloads and verifies assets before publishing, updates the Homebrew tap, and explicitly runs Docker Publish for the same tag commit. A tag alone does not distribute a draft.

### Configuration and code

- `.github/release-please-config.json` and `.github/release-please-manifest.json`
- `.github/workflows/release-please.yml`, `release.yaml`, `publish-release.yml`, `docker-publish.yml`, and `test.yaml`
- `.goreleaser.yaml`: draft builds, existing-draft reuse, unpublished Homebrew formula
- `scripts/release/release.py` and its regression tests
- Release Dockerfiles: inject the approved binary version
- Release/changelog skill adapters and the release runbook

### Repository setup

The maintainer must enable **Allow GitHub Actions to create and approve pull requests** in the repository's Actions settings. The workflows declare their own write permissions. PR checks created or updated with `GITHUB_TOKEN` may require the native **Approve workflows to run** action; do not bypass it.

This implementation does not change repository permissions, merge rules, tag protections, secrets, or external publication state.

### Recovery

For a failed draft build, rerun the failed job or dispatch Build Release Draft with the existing draft tag. A manual rebuild verifies and tests the pinned tag commit. For distribution failures, rerun Publish Release with the same tag, or Docker Publish for an already published tag. Never retag or redistribute an older version over the current published version.

### Validation

Run the release helper regression tests, metadata checks, actionlint and the pinned GoReleaser configuration check inside the devcontainer, followed by the repository's required `make check`. Verify Release Please version behavior using the same library version bundled with the pinned action. Local validation does not establish live GitHub permissions or publication behavior.

The existing `brews` generator is deprecated in GoReleaser v2.18.2: configuration checking reports status 2, while the complete isolated build succeeds with validation enabled. Preserve the current Homebrew formula interface; a cask migration is outside this change.
