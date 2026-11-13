# Feature Proposal: Install an extra from a remote source

## Problem

Teams can publish skills and agents in a Git repository and install them in
consumer projects. Rules, commands, docs and scripts can already be synced as
extras, but their source must be a local directory. When that directory is
missing, extras sync skips it. Every consumer has to acquire the files itself.

This proposal addresses #434 alone. It keeps acquisition separate from the
existing extras sync and keeps #239 as the broader roadmap discussion.
Neither issue currently has the `approved` label. Any implementation offered
with this proposal is an unapproved reference implementation.

## Proposed Solution

**Proposed command** (`--kind extra` is new; the other flags below exist):

```sh
skillshare install example-org/ai-hub/targets/cursor/rules --kind extra --name cursor-rules -p
```

Fetch exactly that directory into `<effective extras root>/cursor-rules`.
A whole repository is also a valid source. Record the origin in a new
`remote:` field and an optional `branch:` on the extra's configuration entry.
`source:` keeps its current meaning as a local directory override; an extra
cannot set both `source:` and `remote:`. Remote values are never treated as
filesystem paths. A new entry has `targets: []`, which existing extras
validation permits.

Then use the existing commands to choose delivery and sync:

```sh
skillshare extras cursor-rules --add-target .cursor/rules -p
skillshare sync extras -p
```

Install does not sync or execute the fetched files. It does not change the
existing `hub:` search-index mapping, add a server, add a transport, or add
bidirectional sync. There is no new Go dependency.

The proposed acquisition contract is:

- Use the existing sparse-checkout and clone chain, with the forge content
  APIs disabled for extras. The checkout's full 40-character commit identifies
  the files being audited. Match the selected path exactly, without skill
  discovery or fuzzy `SKILL.md` matching. Existing submodule refusal remains.
- Store origin, commit, tree hash and file hashes in the extras root's
  `.metadata.json`, with kind `extra`. In projects also store a separate
  `extras` map in `skills.lock.json`. All skill lock writers preserve it.
  A skill and an extra can have the same name without sharing a pin.
- A matching pin selects the commit fetched by the explicit install command.
  An unavailable pinned commit is an error. The existing `--update` flag on
  the proposed command follows the branch head and replaces the pin.
  Project copies are gitignored; commit the declaration and lock instead.
- Reject HTTP(S) URL userinfo, query strings and fragments, and SSH URL
  passwords, before output or persistence can expose the value. Plain SSH
  usernames remain valid. Private sources use Git credential helpers or the
  existing token environment variables. Apply this rule to configuration
  loads and writes too. Ordinary skill sources keep their current behavior.
- Check the selected directory and all its checkout ancestors before copying.
  Refuse absolute, escaping, dangling and directory symlinks, and special
  files. Materialize links to regular files inside the selected directory.
  Refuse a symlink at the managed destination. The user-configured extras
  root may itself be a symlink.
- Audit the staged tree before publication. Findings at or above the chosen
  threshold block installation unless the user supplies `--force`; record
  that override. A scan error blocks even with `--force`. Only the existing
  `--skip-audit` flag bypasses scanning, and the result reports the skip.
  Validate `-T` before fetching. Scanning has depth 6 and a 1 MB per-file
  limit; passing a scan does not prove content is safe.
- Preserve Git's executable bit on POSIX, including executable files without
  a shebang and non-executable files with one. Windows has no such bit.
  Permission-only local changes do not count as content edits.
- Stage, audit, lock the extras root, reread state, copy to a staging
  directory inside the extras root, on the destination filesystem, write the
  project ignore entry, swap, then write metadata, lock and configuration.
  Roll back in-process failures, including accepted findings and previous file
  bytes. Detect leftover staging directories after process death and repair on
  the next explicit install. This is not a crash-proof transaction.
- Reinstall preserves existing targets. Edited or unrecorded directories need
  `--force`; a same-named local extra always blocks installation. A remote
  extra's rename or local-source edit is refused with remove-and-reinstall
  instructions in the CLI and dashboard.
- CLI, TUI and API removal share one operation. Restore single-file targets
  as today, then remove declaration, pin, metadata and accepted findings.
  Delete an unchanged fetched directory and its ignore entry; preserve edited
  files and their ignore entry and report the path. Do not use trash. Local
  extras retain their existing behavior.
- Support the existing `--json` and `--dry-run` flags. Structured output stays
  parseable and a dry run changes nothing. Include the origin and commit in
  extras inspection and preserve both fields through dashboard edits.

For a teammate restoring a project, the **proposed explicit command** above
honors the committed pin. Bare `skillshare install -p` does not restore extras
in this proposal. All project contributors must upgrade before relying on
extra pins: older binaries can discard the new lock map when rewriting it.

## Alternatives Considered

- Keep acquiring files manually. This works today but leaves onboarding and
  provenance outside the manifest.
- Put a URL in `source:`. That field already controls a local directory; a
  separate `remote:` avoids changing its meaning or writing into arbitrary
  local directories.
- Use `--as` for the installed name, as the #434 example does. Use `--name`
  instead: install already has it, while extras commands use `--as` for a
  single-file target's filename.
- Use the content API fallback for extras. It cannot bind the downloaded
  content to a requested full commit or preserve Git file modes. Keep it for
  skills and use Git for extras.
- Add an `extras.lock.json`. A separate map in the existing project lock
  keeps revision state together without colliding with skill names.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

This is one acquisition behavior, including the safety, lifecycle, config,
API, lock, completion, documentation and translation work needed to make it
usable. It is not split to meet a size guideline. Its final diff and actual
CI results must be stated on any reference implementation PR.

Not in this proposal: bare manifest restore and dedicated extras update/check
commands (slice 2), fetching single files (3), hub catalog selection and
suggested targets (4), skill submodules (5), MCP import (6), routed context
modules and the team hub guide (7, with router R1/R2), search discovery (8),
or a dashboard install/update flow (optional 9). The interruption diagnostic
required for safe acquisition is included; remote update discovery is later.
Also excluded: bindings/profile merging, plaintext secret templates,
`resources:` or scalar `hub:`, a server, a new transport and bidirectional
sync. The standalone companion protocol is separate work.

## Open Questions

These are the proposed choices for maintainer feedback, not claims of approval:

1. Is `remote:` the right field name while `source:` remains a local path?
2. Is reusing `--name` preferable to adding the `--as` in #434's example?
3. Is Git-only acquisition for extras acceptable so files, modes and the full
   pinned commit come from the same checkout?
4. Should extras fail closed when scanning cannot run, with `--skip-audit` as
   the sole bypass for that failure?
5. Is an `extras` map in `skills.lock.json` preferable to a second lock file?
6. Should URL credentials be refused for extras only in this change? Should
   skill sources adopt the same rule in a separate change?
