# Feature Proposal: Review Codex hook trust for one project

## Problem

`skillshare hooks sync -p` writes managed project hooks, but a user who also wants to trust a new command must find and review it separately in each Codex account. Issue [#951](https://github.com/runkids/skillshare/issues/951) asks whether Skillshare can narrow that review to the hooks it owns in the current checkout.

## Proposed Solution

Offer `hooks sync -p --trust` and a project Dashboard action backed by the same preview and confirmation. Keep plain sync and the global flow unchanged. Show the absolute checkout path, each affected Codex home, and the exact new or changed commands. Explain that confirmation records trust in the user's Codex configuration, not in the repository. Require explicit confirmation; for the CLI, follow the existing `--yes` and non-terminal rules.

Resolve account homes from user-managed configuration, not from repository input. Ask each home's Codex app-server to list hooks for the selected checkout. Match its source, key and hash against the current project's owned native entries; exclude unmanaged and plugin hooks. Never grant workspace trust. If Codex does not expose an expected entry or ownership cannot be established, do not treat an empty result as success. Direct the user to Codex's `/hooks` for that home. Do not infer that a missing hook necessarily means the folder is untrusted.

Bind the preview to the checkout, home, command and hash. Revalidate before writing, so a changed command or a different home cannot inherit confirmation. Keep disabled hooks disabled. No new dependency is proposed.

## Alternatives Considered

- Keep project trust exclusively in `/hooks`: preserves the native workflow but leaves multi-account users to locate each managed command manually.
- Approve every hook in the project's native file: rejected because other tools and users can own entries in the same file.

## Scope

- [x] Large (10+ files expected across the shared hooks service, CLI, Dashboard API and UI, tests, and documentation)

## Per-account outcomes

Skip a home if its expected hooks cannot be verified. Name it and explain why before asking for confirmation. Independently verified homes can proceed. Report partial completion, not success, when some homes were written and others skipped.
