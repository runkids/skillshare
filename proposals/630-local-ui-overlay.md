# Feature Proposal: Opt-in workflow for versioned local UI overlays

Tracking issue: [#630](https://github.com/runkids/skillshare/issues/630).

## Problem

A local dashboard customization can disappear when a new CLI version downloads
its UI bundle or when the UI cache is cleared. A local development workflow rebuilds
reviewed UI-only patches against the installed release and installs the result.
An optional macOS filesystem watcher can trigger that workflow again after updates.

This is an advanced local use case. It is not necessarily appropriate for every
installation, and an official UI bundle must remain the normal default.

## Proposed Solution

Discuss either a documented local-development recipe or a supported opt-in hook
for versioned UI overlays. This proposal contributes the workflow and constraints,
not personal build scripts, absolute paths, or an automatic patch service.

1. Require explicit trust in local patch and build inputs. Do not discover, fetch
   and execute arbitrary remote customizations automatically. Any local watcher
   must require a separate opt-in; it is not part of default startup.
2. Resolve an exact CLI/release version and known source revision. Record the
   release, patch/build inputs and their hashes in a receipt. Reapplying the same
   verified inputs should be idempotent; dependency/build changes must be considered
   when deciding whether an existing receipt still applies.
3. Use a task-owned staging checkout and build directory. Check compatibility and
   build completely before replacing UI assets. Preserve the official bundle and
   any active working customization on failure. Never overwrite a shared source
   checkout, another build, or another running version's files.
4. Publish the completed bundle with an atomic switch or equivalent rollback-safe
   operation, and serialize concurrent triggers. Explain when a UI server restart
   is needed so stale assets are not mistaken for a failed build.
5. Show that a local customization is active, record failures and provide an
   explicit restore-official path that also disables automatic reapplication.
   Do not modify credentials, installed skills, native agent settings or rules.
6. Keep platform-specific watchers as optional examples. A manual workflow should
   remain sufficient; macOS launchd is not a cross-platform prerequisite.

The local prototype uses a release-tag build, a patch checksum, an official-cache
backup and a launchd trigger. Its fixed paths, host-side build choice and cache
replacement are personal tooling details, not a general upstream implementation.
Any maintained recipe must follow the project's development environment rules and
avoid teaching destructive shared-checkout or partial-publication behavior.

Possible areas, if approved: development documentation or a constrained extension
hook near the UI distribution/cache boundary. No change to official download
verification or trust policy is proposed. Recipe-only work needs no new runtime
dependency; a runtime hook needs a separate design review.

## Alternatives Considered

- Keep local patches completely unsupported: lowest upstream maintenance burden.
- Rebuild manually after each update: adequate for occasional developer use.
- Add a general theming/extension system: more powerful but a much larger project.
- Silently patch every official download: unacceptable as a default because it
  hides executable local changes and complicates support and recovery.

## Scope

- [x] Small (1-3 files, < 200 lines), if limited to an approved development recipe.
- [ ] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

This PR adds one proposal only. A runtime extension hook is not included in that
estimate and would require separate approval, implementation and testing.

## Open Questions

1. Should this remain unsupported local tooling, or is a development recipe useful?
2. Where should active customization state and receipts live without being confused
   with official cache verification?
3. What restoration and restart behavior can be documented reliably across platforms?
