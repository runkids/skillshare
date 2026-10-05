# Plugins

Plugins are complete native packages, not standalone skills. Keep their components
together. Install targets: `claude`, `codex`, `cursor`, `antigravity` (`agy` alias),
`antigravity-cli`, `copilot`, `pi`, and `opencode`. Grok supports native import/removal,
with trust/install handled in Grok. Kimi, Hermes, and Devin are discovery-only.
Read `targetDefinitions` from JSON output instead of assuming every format supports
all operations. `targetInfo` reports per-target components, version, and problems.

## Inspect before changing

```bash
skillshare plugin list --json -g
skillshare plugin discover ./plugin-directory --json
skillshare plugin add ./plugin-directory --plugin demo --target claude --dry-run --json -g
skillshare plugin add npm:@scope/package --target pi --dry-run --json -g
```

`add` accepts a local folder, owner/repo, or HTTPS Git URL. For Pi it also accepts
`npm:<package>[@version]` (packages listed on pi.dev): Pi downloads it and runs its install
scripts, so nothing is reviewed first. Given a pi.dev page (`https://pi.dev/packages/<package>`)
or a `pi install npm:<package>` command, pass `npm:<package>`; the CLI does not convert them.
It needs a Pi `--target` (not an account running
another executable), takes no `--source-ref`, `--entry` or `--plugin`, and `discover` rejects
it. Pi keeps one entry per package name; another version replaces it and keeps its extension
filters, and `update` skips a package pinned to an exact version. `check` compares a
package added without a version with npm's latest; a range, tag or private registry is left
to Pi. A Pi package another
Skillshare package already manages is refused. In a project with a `.pi` folder, Pi changes
packages only after the user trusts the project in Pi. For a multi-plugin
marketplace, select a named candidate with `--plugin`. Use `--name` to bind
different native distributions under one logical package, never infer equivalence
from display names. External catalog sources are not auto-converted. Multiple local catalogs are merged.
Safe internal relative symlinks are preserved; escaping, absolute, broken, cyclic,
and `.git` links are rejected. A malformed manifest does not hide other formats.

`add` without `--target`, or an interactive picker confirmed with nothing selected,
keeps the plugin in Skillshare and installs it nowhere; `list` shows it as `no targets`.
Add targets later with the same source and `--name`, which installs the source as it is
then. Such a package stays managed when its last target is removed.

Use `--source-ref TAG_OR_COMMIT` with discover/add/update for a remote Git source.
Bindings retain `source_ref` and the reviewed `commit`; `--revision` is a separate
preview token. Use `--entry dist/plugin.js` with discover/add for an explicit
OpenCode entry. Never build or execute package code just to discover it.

## Apply and adopt

```bash
skillshare plugin add ./plugin-directory --plugin demo --target claude --no-tui -g
skillshare plugin import demo@team --from claude --no-tui -g
skillshare plugin inspect demo --json -g
```

Add installs; import records an existing native installation without reinstalling
or changing native enabled state. Pass `--revision ID` when applying a specific
preview. A stale preview is rejected. Review native trust/authentication problems
in the native client; never add native auto-accept flags to bypass them.

## Sync selection

```bash
skillshare plugin disable demo --target codex --no-tui -g
skillshare sync plugins demo --dry-run --json -g
skillshare sync plugins demo --no-tui -g
skillshare plugin enable demo --target codex --no-tui -g
```

Enable/disable only select targets for synchronization. Deselecting a managed
binding removes its installation on the next sync, but keeps the definition.
This is not native enable/disable. `sync --all` does not include plugins.

## Update and remove

```bash
skillshare plugin check demo --json -g
skillshare plugin update demo --target claude --dry-run --json -g
skillshare plugin update demo --target claude --no-tui -g
skillshare plugin remove demo --dry-run --json -g
```

`remove NAME --target` uninstalls that binding. `remove NAME` without `--target`, once
no Agent holds it, drops the package from Skillshare entirely.

Claude supports native updates; Codex re-adds the reviewed snapshot unless the plugin is disabled in Codex. Cursor/Antigravity replace managed
local copies; Pi/OpenCode refresh reviewed source snapshots. Imported Codex plugins update by upgrading their marketplace; imported Pi packages through `pi update`, global only. Imported OpenCode v1 packages must be updated natively. OpenCode v2 global
imports may use native update; project imports may not. Copilot source updates
require known native enabled state; Antigravity CLI and Grok update natively. Project mode supports Claude, Antigravity, Pi,
OMP and OpenCode, never falling back to global scope. An update skips a target it
cannot reach, says why, and still updates the other targets.

## Additional formats and scopes

- Cursor: `.cursor-plugin/plugin.json` or Agent Plugins root manifest; copy to
  `~/.cursor/plugins/local/`. Local imports must be allowed; marketplace copies can
  take precedence. No CLI required. Reload and verify in Cursor.
- Antigravity: root `plugin.json` with explicit `name`; copy to
  `~/.gemini/config/plugins/`, or project `.agents/plugins/` (existing `_agents/plugins/`
  is supported). This targets desktop/workspace discovery, not the separate
  standalone agy CLI plugin store. Use `antigravity-cli` for that store; `agy`
  remains the desktop alias. No Gemini CLI adapter is provided.
- Pi: `package.json` with a `pi` resource manifest or `pi-package` conventions. Native install/remove; read-only
  settings inventory honors `PI_CODING_AGENT_DIR`. Project trust must be completed
  in Pi; do not bypass it with automatic approval flags.
  Managed plugin cards use the same package icon and `Plugins · <name>` heading
  as OMP, without repeating the plugin origin in tags. Version, scope and native
  project override labels remain; the navigation icon opens Plugins. Unmanaged
  Pi packages retain their native identity and Pi icon.
  Which of a package's extensions load is chosen per Pi target in the dashboard's
  Extensions tab (exact `+`/`-` rules, previewed, refused if the file changed or Pi's lock
  is held or lost; a switch removes the file's own rule instead when that alone gives the
  requested state; "Remove rule", shown otherwise, leaves the file to the remaining rules; a single-file source,
  and a string entry of a package with convention skills/prompts/themes folders its manifest leaves
  out (conversion to an object can't be shown to keep them unchanged), are read-only; so is an entry
  with an empty source or an unpaired UTF-16 surrogate escape / invalid UTF-8 in its source or rules); there is no CLI
  command for it. It is editable only on Pi 0.99.2 or later (a fork account
  is read-only and never run). On a project page it saves only `.pi/settings.json`, as
  `pi config` does: a global package gets a project entry `{source, autoload: false,
  extensions}` (local source relative to `.pi`). With its last rule removed, the entry
  is retained if deletion could expose earlier filters. Only explicit JSON `false`
  is a delta, not `null`; the global
  settings and `trust.json` are never written and Skillshare never trusts the project. A
  global source with credentials or a query, or a project/global entry it can't read
  exactly, keeps the affected packages read-only.
- OMP (`omp`): marketplace-backed local/Git plugins use reviewed snapshots and native
  fresh-cache installation with post-operation inventory verification. The reviewed
  runtime destination must also be absent; existing directories and links are never
  replaced. Preview binds that path and install rechecks it. Native 18.6.1
  uninstall/upgrade can delete shared cache still used by an invisible project.
  Remove and deselection-plus-sync use a scoped adapter: registration, runtime
  selection and verified links are removed, not disabled; cache, marketplace
  registrations and plugin settings remain. Reinstall chooses a fresh cache
  identity. A missing cache manifest requires a unique runtime-lock key and link
  to the exact retained cache, never a guessed marketplace name. Unverifiable active
  ownership or metadata blocks removal, Windows requires private
  native ACLs, and interrupted removal can be retried. Do not run native plugin
  mutations concurrently; their writer does not share the adapter lock.
  Updates remain blocked, and imported code is not upgraded without a reviewed source. Native `.omp-plugin/marketplace.json` catalogs are supported.
  User/project scopes require the matching native root. Import `name@marketplace`,
  not bare npm/Git/link packages; manage those in OMP. OMP accounts are not plugin
  targets: `config_dir` does not redirect its plugin store. Preview never calls native
  `--dry-run`. OMP has no project trust prompt; plugin extensions/tools may execute
  in-process on its next startup. Keep every plugin's resources together.
  The target's Extensions tab groups plugins by inspected package root and other
  files by source/scope/directory. Plugin headers use the native package name and
  known manifest version; shared folders are plugin-relative, with full paths in
  tooltips. Details uses Pi's labeled two-column layout for source, native name,
  selection identifiers and notes. File lists remain visible without collapse controls.
  It can preview/apply `disabledExtensions`
  changes for verified standalone modules, using native locks and stale-revision protection.
  It requires a statically identifiable OMP 18.6.1 package; unknown versions/wrappers,
  ambiguous same-name groups, explicit-file bypasses, ambient hooks, linked paths,
  uncertain selection and resources owned by Hooks/Plugins remain read-only.
  Selection-only backup records live under `StateDir/omp-extensions/backups`; they
  do not copy unrelated native settings or credentials. No extension code is executed
  during selection management, and selected never means loaded or running.
- OpenCode: SDK dependency, `.opencode/plugins/` convention, or explicit `--entry`,
  with an existing JS/TS entry.
  Preserve the whole tree and register its file URL in the native JSON/JSONC config.
  Version 1 uses `plugin`, version 2 uses `plugins`. Runtime dependencies must already
  be available. The CLI version selects the schema; do not guess from docs alone.
- Another account of an Agent (a target with `agent:` and `config_dir:` in the global
  config) is a plugin target under its own name: `--target claude-work`, `--from
  claude-work`. Supported for `claude`, `codex` and `pi`. Skillshare runs that Agent's CLI
  against the account's config directory, through `CLAUDE_CONFIG_DIR`, `CODEX_HOME` or
  `PI_CODING_AGENT_DIR`, and the account keeps its own bindings. Global scope only. With
  `cli:` it runs that compatible CLI instead (such as `omo` for Pi); a Pi account also sets
  `SENPI_CODING_AGENT_DIR` and `OMO_CODING_AGENT_DIR`. A missing CLI fails, with no fallback.
- Codex without `cli:`: `codex` on PATH, then Homebrew (`/opt/homebrew/bin`, `/usr/local/bin`) or the
  Windows installer (`%LOCALAPPDATA%\Programs\OpenAI\Codex\bin`)
  and the CLI shipped in the Codex desktop app (macOS `ChatGPT.app`, or `Codex.app` from an older install, Windows
  `%LOCALAPPDATA%\OpenAI\Codex\bin\<version>`). The machine-local env var
  `SKILLSHARE_CODEX_CLI` overrides the search; prefer it over a path in a shared config.
  The missing-CLI error lists every place searched.
- Local directory plugins cannot import unowned folders or marketplace installs.
  Pi 0.99.2 or later can import supported filtered entries without changing
  native settings. Preview shows retained keys; raw entries stay in private state
  and shared config stores a digest. Uninstall captures current options; reinstall
  restores the object before native install. Missing/changed/cross-target records,
  uncertain sources or precedence, and non-normalized local references are refused.
  Windows registrations use private ACLs, not POSIX mode bits. Unsafe existing
  directory/file ACLs block import/restoration without changing those ACLs; preserve
  the records and have their owner repair access protection before retrying.
  Filtered OpenCode imports remain blocked.
  Supply `--name` when a native package source is not a valid logical name.

```bash
skillshare plugin add ./agy-plugin --target agy --dry-run --json -p
skillshare plugin add ./pi-package --target pi --no-tui -g
skillshare plugin add ./opencode-package --target opencode --no-tui -g
```

On a partial failure, inspect each target outcome and retry with `sync plugins`.
Do not delete native caches or rewrite native installed-plugin registries. Removal
also removes the per-plugin marketplace Skillshare registered for Claude/Codex
(`skillshare-<plugin>-<hash>`, or `skillshare-<hash>` for older installs), retrying on the
next sync if cleanup fails, and retains imported plugins' marketplaces and all snapshots. An
imported plugin whose native marketplace is gone is skipped: restore it natively or re-add
from source. A Claude plugin named like a skill folder that has a plugin manifest
(`<name>@skills-dir`) wins, and Claude skips that folder; the add preview says so. Installed does not mean loaded, logged
in, or hook-trusted. Interactive humans can use the bare `skillshare plugin` manager;
automation must provide explicit arguments and use `--json` or `--no-tui`.

## Native capability boundaries

- Copilot installs a reviewed snapshot. An imported binding without a source cannot
  be reinstalled automatically after removal; install natively, then sync again.
- Antigravity CLI accepts native and Claude manifests using its own CLI. Its native
  list does not expose enablement; never present unknown as disabled. Update in agy.
- Grok install/update requires native trust; never add `--trust` automatically.
- Kimi, Hermes, and Devin discovery does not authorize native installation,
  capability consent, or cloud changes. Explain the adapter limitation.
- Registration is not proof of resource loading. Verify inside the target Agent.
- Failed snapshot updates restore the previous managed snapshot; this is not a
  claim that every native cache side effect can be rolled back.
