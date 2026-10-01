# Native hooks

Manage hooks with `skillshare hooks`; use `-g` or `-p` explicitly. Declarations
live in `hooks.entries` in the selected config. Entries contain `description`,
optional `enabled` (default true), and `bindings` keyed by receiving Agent.
Command bindings keep native `events`; Pi, Amp and OpenCode bindings keep native
`code`. Optional command scripts use `files`. Do not translate native event
names, matchers, handler types or timeout units across Agents.

```bash
skillshare hooks list --json
skillshare hooks add check --file ./check.yaml --dry-run --json
skillshare hooks add check --file ./check.yaml
skillshare hooks edit check --file ./updated.yaml --sync
skillshare hooks import --from claude --json
skillshare hooks sync --dry-run --json
skillshare hooks sync
skillshare sync hooks --dry-run --json
skillshare hooks disable check --sync
skillshare hooks enable check --sync
skillshare hooks remove check --sync
skillshare hooks remove check --keep-files
skillshare hooks restore BACKUP_ID --dry-run --json
skillshare hooks restore BACKUP_ID
```

The file for add/edit is an Entry, without the `hooks.entries` wrapper. Import
without a name lists candidates; provide a name to save one. Saving an import
takes over the registrations it read, so the next sync adopts them without
`--replace`; unknown event names only warn. Import `--file`
reads supplied native configuration or code with `--from` selecting its dialect.
No import, preview or sync executes hook commands.

Mutations save source only unless `--sync` is supplied. An empty bindings map
keeps source without publishing; disabling keeps source and queues removal of
owned unchanged outputs. `--replace` explicitly replaces an existing source
or the selected entry's conflicting native output. Never add it to bypass a
conflict automatically. A stale preview requires refresh (`--revision` can
require a specific one). Unrelated native settings and unowned hooks survive.
`remove --keep-files` stops managing an entry: its native entries stay as they
are, sync no longer touches them, and import offers them again (not with `--sync`).

Agent IDs: `claude`, `codex`, `gemini`, `copilot`, `cursor`, `droid`, `qwen`,
`antigravity`, `pi`, `amp`, `opencode`, `git`; `factory` aliases `droid`, and
`antigravity-cli`/`agy` alias `antigravity`. Code must match the native
Agent's installed API version. Check actual destination paths in the inventory
and preview; global config-directory overrides do not apply to project paths.

Global `hooks.projects` can manage absolute project roots with their own entries
from one config. Roots with their own `.skillshare/config.yaml` belong in project
scope. Dashboard destinations show Hooks beside the other managed resources;
Settings > Backups groups hook backups by Agent and native file.

Backups restore native output, not source definitions. Edited owned outputs
conflict on sync/removal/restore. Native trust and loading remain controlled by
the Agent: a successful sync is configuration-generation evidence only.

## Accounts

A global binding key can name a target declared with `agent` and `config_dir`,
for example `codex-2: {events: ...}` for a Codex account. Claude, Codex and Pi
accounts keep their Agent's native format and write into that account home.
Accounts are global only; project configs and `hooks.projects` bind the Agent.
Missing account homes are skipped with warnings and never created.
The `git` key always means Git config hooks; an account named `git` gets no
hooks, with a warning once an entry uses that binding.

When a native directory environment override points at a declared account home,
the plain Agent uses its built-in default instead. Other overrides still apply.
Outputs outside the target's current home, missing homes and removed targets
keep their ownership and files with a warning; explicitly preview and run
`hooks sync NAME --replace -g` to release parked ownership without editing files.
Import uses the account key: `hooks import --from codex-2 -g --json`.
Each Codex home must trust its own changed hooks through `/hooks`.

## Git config hooks

Git 2.54+ reads `bindings.git.commands`, keyed by unique friendly names (never
Git event names). Each value has `events: [pre-commit]`, a single-line `command`
and optional `parallel` (Git 2.55+). Inline `files` provide executable UTF-8/LF
helpers; `{files}` expands to their quoted absolute directory on each machine.
Use a helper for compound shell logic: Git appends event arguments to commands.
Do not pass those arguments blindly to tools that reject them.
Use distinct friendly names across global and project commands: Git merges both scopes.

Global output is `$XDG_CONFIG_HOME/git/skillshare/hooks.gitconfig` (default
`~/.config/git/…`). Include-target precedence is absolute `GIT_CONFIG_GLOBAL`,
existing `~/.gitconfig`, existing `$XDG_CONFIG_HOME/git/config`, new
`~/.gitconfig`. Project output is `<git-common-dir>/skillshare/hooks.gitconfig`,
included from the common `config`; linked worktrees share it. Declare one
repository top-level root per common directory.

Preview before sync. Regular writable configs receive an owned include;
symlinks/unwritable targets receive exact manual lines with `inactive` status.
Old Git is also inactive, with no dispatcher. Plans preserve file actions and
carry `inactiveReason` separately; synced inactive output is not a pending write. Missing Git or roots skip outputs
and preserve ownership. Manual and conditional includes stay user-owned,
including nested includes beneath inactive parent conditions. Unreadable or
excessively nested declarations block sync. Conditional presence does not prove
activation. Check `hooks list -g --json`
(`git`, `projectGit`) and read-only `git hook list --show-scope EVENT`.

The generated include file is owned whole. `--replace` regenerates it after
backup; never silently discard external edits. A colliding friendly name can
be replaced only in the writable regular scope target, not system/included
config. `enabled=false` overrides warn. Config operations use native locks and
narrow backups, preserving unrelated later settings on restore.
Git bindings refuse `--keep-files`. Git import, scripts bindings, hookdir
scanning/recognizers and doctor checks are not implemented in this phase.
Inspect old hookdir copies manually before migration; Git can execute both.
Management never executes hooks. Native Windows execution is unverified.
