# Feature Proposal: Include extras in skillshare's git integration

Issue: [#171](https://github.com/runkids/skillshare/issues/171)

## Problem

Skillshare's git integration (`commit` / `push` / `pull`) is the cleanest way to version-control a skills setup — one tool, one workflow, one source of truth. Today, however, that integration operates on the **skills source directory** only. `commit` stages changes in the skills source and creates a commit; `push` and `pull` follow the same source.

`extras` (hooks, rules, commands, prompts) sits outside that story. By default `extras_source` resolves to a separate directory (e.g. `~/.config/skillshare/extras/<name>/`), which is not inside the skills source, so it never enters the git workflow. As a result, resources that skillshare already manages — for example a `pre-bash-guard.sh` hook that blocks `rm -rf $HOME` and secret-file access, or reusable commands/prompts/rules — cannot be committed and pushed to a remote alongside skills.

The capability is *almost* there. By moving `extras_source` to live inside the skills source and adding a `.skillignore` entry, you can nudge git into picking extras up (verified end-to-end with Claude Code + Codex hooks). But this is a manual workaround, not first-class behavior, and a new user would not discover it on their own.

Who is affected: anyone who manages both skills and extras with skillshare and wants the whole setup — not just skills — under a single git workflow.

## Proposed Solution

This proposal does **not** prescribe a specific implementation — the design is best left to the maintainer for consistency with the project's architecture. The intent is simply that `skillshare commit` / `push` / `pull` should treat extras as a normal part of what skillshare versions through git, rather than something the user has to wire up by hand.

At a high level, a solution would likely touch:

- **Git commands (`commit` / `push` / `pull`)** — extend the set of paths these commands stage/sync so that configured extras sources are included, not just the skills source.
- **Config (`.skillshare` / `config.yaml`)** — relate extras inclusion to the already-existing source configuration. Extras sources are resolved today via per-extra `source` > `extras_source` (legacy) > `sources.extras` > default. The git integration would need a coherent rule for which of these locations participate in commit/push/pull, especially when an extras source lives outside the skills source directory.
- **`.skillignore` / ignore semantics** — make sure extras inclusion composes predictably with existing ignore rules, so users can still exclude specific extras from git if they want.
- **Backward compatibility** — current behavior (git integration covering the skills source) should be preserved. Whether extras are included by default or behind an opt-in is an open question (see below); either way, existing repositories should not have their git history or commit contents change unexpectedly.
- **Docs** — `commit` / `push` / `pull` reference pages and the source-and-targets / extras documentation would need to describe the new behavior.

No new runtime dependencies are anticipated; this is primarily about which paths the existing git integration covers and how that is configured.

## Alternatives Considered

- **Keep the manual workaround** (move `extras_source` inside the skills source + add a `.skillignore` entry). This works today and is verified end-to-end, but it is undiscoverable, easy to get wrong, and forces users to colocate extras with skills even when they would rather keep them separate.
- **A separate `skillshare extras commit/push/pull` family of commands.** This keeps skills and extras git flows fully independent, but it fragments the "one tool, one workflow" story the issue is asking for — users would have to run two sets of git commands to version one setup.
- **Document the workaround only**, without changing behavior. Lower effort, but it leaves extras as a second-class citizen of the git integration and does not address the discoverability gap.

The proposed direction (making the existing git integration aware of extras) is preferred because it keeps a single workflow while letting the maintainer choose the exact mechanism.

## Scope

Estimate the scope of changes (best guess — actual scope is a maintainer decision):

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Expected areas:

- Git integration commands (`commit`, `push`, `pull`) path/source resolution
- Config parsing and source resolution for extras
- Ignore-rule handling (`.skillignore`) interaction
- Docs for the affected commands and the source/extras model

## Open Questions

- **Extras source outside the skills source directory.** When `extras_source` (or `sources.extras`) points outside the skills source — possibly outside any single git repository — how should commit/push/pull behave? Options include: include it via a shared/parent path, require it to live under a git repo, or skip external paths the way custom project sources skip gitignore management today.
- **Relationship with `.skillignore`.** Should extras inclusion respect `.skillignore` the same way skills do, and is one ignore file sufficient or do extras need their own ignore semantics?
- **Default behavior vs. opt-in.** Should extras be included in git integration by default, or behind an explicit config flag (e.g. an `include` setting) to preserve current behavior for existing users? What is the least surprising default?
- **Multiple extras sources.** If a user configures several extras sources (per-extra `source` overrides), should all of them be covered by a single `commit` / `push` / `pull`, or only those under a primary source?
- **`pull` / merge direction.** How should `pull` reconcile remote extras with local ones, particularly when local overrides (per-extra `source`) diverge from what is tracked in the remote?
