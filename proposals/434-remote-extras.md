# Feature Proposal: Install an extra from a remote source

Issue: [#434](https://github.com/runkids/skillshare/issues/434)

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
- A matching pin selects the commit fetched by the explicit install command:
  metadata supplies it globally, and the lock supplies it in projects.
  An unavailable pinned commit is an error. The existing `--update` flag on
  the proposed command follows the branch head and replaces the pin.
  In projects, ignore the fetched directory, extras metadata and staging
  directory; commit the declaration and lock instead. If the configured root
  is outside the project, proceed with a warning that no project ignore rule
  can be written for it.
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
  threshold block installation unless the user explicitly supplies `--force`;
  record that override. A scan error blocks even with `--force`. Only the
  existing `--skip-audit` flag bypasses scanning, and the result reports the skip.
  Validate `-T` before fetching. Scanning has depth 6, a 1 MB per-file limit
  and an extension filter; for example, `.mdc` and `.cursorrules` are skipped
  by the text scans.
  Passing a scan does not prove content is safe.
- Preserve Git's executable bit on POSIX, including executable files without
  a shebang and non-executable files with one. Windows has no such bit.
  Permission-only local changes do not count as content edits.
- Stage, audit, lock the extras root and reread state. Before the first
  publication write, snapshot the prior files and accepted findings and retain
  a durable per-name marker or journal. Copy to a staging directory inside the
  extras root, on the destination filesystem, write the project ignore entry,
  swap, then write metadata, lock and configuration. Keep the marker until all
  writes and cleanup finish, so a death at every boundary of first install or
  replacement is detectable. Roll back in-process failures and repair an
  interrupted publication on the next explicit install before conflict checks.
  Publication coordinates with removal and supported remote-extra target,
  mode, collect and dashboard edits. Merge shared records from fresh state;
  rollback restores only this operation's changes when still applicable,
  preserving observed concurrent writes. This does not isolate unrelated
  local-extra, skill or manual writers and is not a crash-proof transaction.
- Reinstall preserves existing targets. Edited or unrecorded directories need
  explicit `--force`; `--json` alone never implies it for extras, including
  audit overrides. A same-named local extra always blocks installation. A
  remote extra's rename or local-source edit is refused with
  remove-and-reinstall instructions in the CLI and dashboard. Existing `extras collect` edits to
  the fetched source count as local modifications. `extras source` changes
  the global root setting without moving files; when relocating the root,
  moving its metadata with the fetched directories retains provenance.
- CLI, TUI and API removal share one operation. Use existing cleanup helpers
  to detach owned merge/file links and directory symlink targets, and restore
  single-file targets as today. Preserve copy or transformed outputs without
  proof of ownership and warn. On cleanup failure stop before deleting the
  source or configuration. Then remove declaration, pin, metadata and accepted
  findings. Delete an unchanged fetched directory and its ignore entry;
  preserve edited files and their ignore entry and report the path. Do not use
  trash. Local extras retain their existing behavior.
- Support the existing `--json` and `--dry-run` flags. Structured output stays
  parseable and a dry run changes nothing. Include the origin and commit in
  extras inspection and preserve both fields through dashboard edits.

For a teammate restoring a project, the **proposed explicit command** above
honors the committed pin. Bare `skillshare install -p` does not restore extras
in this proposal. When a remote extra's source is missing, sync directs the
user to `skillshare install <remote> --kind extra --name <name> -p` in a
project, or the corresponding `-g` command in global mode, instead of asking
them to create the directory, in plain and JSON output and the API. All project
contributors must upgrade before relying on extra pins: older binaries can
discard the new lock map when rewriting it.

## Alternatives Considered

- Clone a repository or add it as a submodule and point `source:` at its
  directory. This works today but leaves acquisition and provenance outside
  the extras manifest. Alternatively, `git_root: extras` with `pull` shares
  the extras source as a Git scope, rather than pinning each remote extra in
  the project manifest.
- Put a URL in `source:`. That field already controls a local directory; a
  separate `remote:` avoids changing its meaning or writing into arbitrary
  local directories.
- Use `ResourceEntry` for extras. Delivery targets, file and mode already live
  in `ExtraConfig`; keeping the remote there preserves `source:` as a local
  override and keeps acquisition with its delivery configuration.
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

Not in this proposal: bare manifest restore and remote update discovery
(slice 2), fetching a single file as the acquisition source (3), hub catalog
selection and suggested targets (4), skill submodules (5), MCP import (6),
routed context modules and the team hub guide (7, with router R1/R2), search discovery (8),
or a dashboard install/update flow (optional 9). The interruption diagnostic
required for safe acquisition is included; remote update discovery is later.
Also excluded: bindings/profile merging, plaintext secret templates,
`resources:` or scalar `hub:`, a server, a new transport and bidirectional
sync. The standalone companion protocol is separate work.
An acquired directory can still use existing single-file delivery configuration.

Implementation validation is planned to cover global and project install,
sync, reinstall and removal end to end, alongside state and security
regressions. These are planned checks, not test results.

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
