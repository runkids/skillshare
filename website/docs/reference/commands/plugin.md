---
sidebar_position: 4
---

# plugin

Manage complete native plugins across supported tools. Capability checks distinguish
installation support from format discovery. Start with
[Manage plugins across tools](/docs/how-to/daily-tasks/sharing-plugins).

```bash
skillshare plugin                         # Interactive manager
skillshare plugin add                     # Source → plugin → targets → review
skillshare plugin discover ./my-plugin --json
skillshare plugin add ./my-plugin --target claude --target codex --no-tui
skillshare plugin add ./my-plugin --no-tui   # Keep it in Skillshare; choose targets later
skillshare plugin import review@team --from claude --no-tui
skillshare plugin list --json
skillshare plugin inspect review --json
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run --json
skillshare sync plugins --no-tui
skillshare plugin enable review --target codex --no-tui
skillshare plugin check review --json
skillshare plugin update review --target claude --no-tui
skillshare plugin remove review --no-tui
```

`enable` and `disable` change **Skillshare's sync selection**, not the Agent's native
enabled state. Deselecting a target saves the choice. The next `sync plugins`
removes its managed installation while retaining the definition. Selecting it
again allows the next sync to reinstall it. Unmanaged plugins are unaffected.

`add` without `--target` (or with nothing selected in the interactive picker) keeps
the plugin in Skillshare without installing it anywhere. Add targets later with the
same source and `--name`, or from the plugin's row in the dashboard; that install
uses the source as it is then. Such a plugin stays managed when its last target is
removed. `remove NAME` without `--target` removes it from Skillshare.

## Commands

| Command | Behavior |
|---|---|
| `list` | Configured bindings and native installation state; interactive manager in a terminal |
| `discover SOURCE` | Inspect a local directory, `owner/repo`, or HTTPS Git repository |
| `add [SOURCE]` | Choose and install a whole plugin with its target adapter, or an `npm:` package through Pi |
| `import [NATIVE-ID]` | Adopt an existing installation without reinstalling or enabling it |
| `inspect NAME` | Inspect one managed package |
| `sync [NAME]` | Reconcile selected targets and retry incomplete native operations |
| `check [NAME]` | Compare source content with the recorded digest, or a Pi npm package's version with npm's latest; never update |
| `update [NAME]` | Review source changes and use a supported native update operation |
| `enable / disable [NAME]` | Include/exclude a target in the next sync |
| `remove [NAME]` | Uninstall managed bindings and remove their definitions |

Bare commands prompt for missing inputs in a terminal. Noninteractive mutation
commands require explicit inputs. `sync` and `check` may operate on all packages.
`sync --all` does **not** include plugins; use `sync plugins` explicitly.

## Options

| Option | Meaning |
|---|---|
| `--target TARGET` | Repeatable selection: `claude`, `codex`, `cursor`, `antigravity` (`agy` alias), `antigravity-cli`, `copilot`, `grok`, `pi`, `omp`, `opencode`, or the name of [another account of an Agent](#accounts); see the capability table below |
| `--plugin NAME` | Select one plugin from a source marketplace |
| `--name NAME` | Logical package name when adding or importing |
| `--from TARGET` | Import from Claude, Codex, Antigravity CLI, Copilot, Grok, Pi, OMP marketplaces, OpenCode, or [another account of an Agent](#accounts) |
| `--dry-run`, `-n` | Preview without changing Skillshare or Agent configuration |
| `--source-ref REF` | Git branch, tag, or commit for `discover`, `add`, and `update`; remote sources only |
| `--entry PATH` | Explicit built OpenCode JS/TS entry, relative to the package root (`discover` and `add`) |
| `--revision ID` | Refuse application if source, configuration, or native inventory changed since preview |
| `--json` | Machine-readable output; disables TUI |
| `--no-tui` | Disable interactive menus; also respects `tui: false` |
| `--global`, `-g` | Global Skillshare config and native user scope |
| `--project`, `-p` | Project config; Claude, Antigravity, Pi, OMP, or OpenCode (never global fallback) |

JSON output includes source paths and native identifiers. Do not put credentials
in source URLs. A failed mutation can still return successful per-target outcomes;
the CLI exits nonzero when any target fails. Inspect the result before retrying.

## Target support

| Target | Format | Global | Project | Update |
|---|---|:---:|:---:|---|
| Claude Code | `.claude-plugin/plugin.json` | Yes | Yes | Native update |
| Codex | `.codex-plugin/plugin.json` or Agent Plugins root manifest | Yes | No | Refresh reviewed source and add it again, only if enabled in Codex |
| Cursor | `.cursor-plugin/plugin.json` or Agent Plugins root manifest | Yes | No | Replace reviewed local copy |
| Antigravity Desktop | Root `plugin.json` with an explicit name | Yes | Yes | Replace reviewed local copy |
| Pi | `package.json` with `pi` resources, or `pi-package` keyword and conventional resource folders | Yes | Yes, with native project trust | Refresh managed source snapshot |
| Oh My Pi | Marketplace-backed local/Git plugins with native-compatible metadata | Fresh-cache install/reinstall, import and scoped removal | At OMP's project anchor | Updates blocked; removal retains shared cache |
| OpenCode | `package.json` with an SDK dependency, `.opencode/plugins/` entry, or explicit `--entry` | Yes | Yes | Refresh managed source snapshot |

| Antigravity CLI | Native root manifest or Claude manifest accepted by `agy` | Yes | No | Update natively to preserve enablement |
| GitHub Copilot CLI | `.plugin/plugin.json`, `.github/plugin/plugin.json`, Claude manifest, or Agent Plugins root manifest | Yes | No | Refresh reviewed source, only if native enabled state is known and enabled |
| Grok Build | `.grok-plugin/plugin.json` or Claude manifest | Import/remove only; native trust required for install | No | Update natively |
| Kimi Code | `kimi.plugin.json` or `.kimi-plugin/plugin.json` | Discovery only | No | Not automated |
| Hermes | `.hermes-plugin/plugin.yaml` | Discovery only | No | Not automated |
| Devin | `.devin-plugin/plugin.json` | Discovery only | No | Not automated |

Kimi's noninteractive lifecycle, Hermes's profile inventory/consent, and Devin's
local inventory/trust/cloud distinction are not yet verified by these adapters.
Their formats are shown during discovery, but installation is disabled with a
reason. A source declaring a target does not imply that Skillshare can manage it.
`list --json` and `discover --json` include `targetDefinitions` with allowed
operations; discovery also exposes `targetInfo` for each format's version,
components, entry, and validation problem. A broken manifest is isolated to its
target; a malformed catalog is reported as a warning without hiding valid formats.

### Another account of an Agent {#accounts}

A target declared as [another account of an Agent](/docs/reference/targets/configuration#agent-config-dir) is a plugin target too, for `claude`, `codex` and `pi`. Skillshare runs that Agent's own CLI against the account's config directory, through `CLAUDE_CONFIG_DIR`, `CODEX_HOME` or `PI_CODING_AGENT_DIR`:

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work
```

```bash
skillshare plugin add owner/repo --target claude-work
skillshare plugin import demo@market --from claude-work
```

An account with [`cli`](/docs/reference/targets/configuration#agent-config-dir) runs that compatible CLI instead, such as `omo` for a Pi account, against the same config directory. A Pi account also sets `SENPI_CODING_AGENT_DIR` and `OMO_CODING_AGENT_DIR`, which Pi forks read before `PI_CODING_AGENT_DIR`. If the CLI cannot be found, the operation fails; Skillshare does not fall back to the Agent's own.

The account takes the operations of the Agent it belongs to and keeps its own bindings under its own name, so a plugin can be installed in one account and not in the other. Accounts exist in global scope only: a project's plugins belong to the project, not to one account. `--target` and `--from` accept the account name, and the terminal picker and the dashboard's Plugins page list it next to the Agents.

### Cursor and Antigravity

These adapters copy the whole plugin into documented local discovery directories.
They do not require a CLI executable or modify marketplace registries:

- Cursor: `~/.cursor/plugins/local/<name>`; local imports must be allowed. Reload
  Cursor and check Customize. An installed marketplace plugin with the same name
  takes precedence over a local copy.
- Antigravity desktop: `~/.gemini/config/plugins/<name>` globally; `.agents/plugins/<name>`
  in a workspace (or an existing `_agents/plugins/` directory). If both workspace
  directories exist, consolidate them first.
- The standalone **agy CLI** has a separate plugin store. The `antigravity` target
  manages the desktop/workspace discovery paths, not that CLI store. `agy` is only
  a Skillshare target alias. Use `--target antigravity-cli` for the standalone CLI;
  `--target agy` retains its existing desktop meaning.

Use `plugin add` for these local packages. Importing pre-existing local folders or
marketplace installs is not supported. Skillshare refuses to overwrite unowned
folders, symlinks, or locally edited managed content. An explicit Antigravity
manifest name keeps the identity stable across Git checkouts and snapshots.

### Oh My Pi (OMP) {#omp}

OMP uses its own marketplace lifecycle, not Pi's package commands. Skillshare
manages reviewed local/Git plugin sources and imports native marketplace entries
identified as `name@marketplace`:

```bash
skillshare plugin add ./my-omp-plugin --target omp --dry-run --json -g
skillshare plugin add ./my-omp-plugin --target omp --no-tui -g
skillshare plugin import demo@my-market --from omp --dry-run --json -g
```

Installation requires OMP **18.6.1**, a reviewed local/Git source and a verified,
previously unused cache destination and an absent scope-local runtime destination.
The runtime package name comes from the reviewed source, not just the marketplace
name. Existing module directories or links block installation rather than being
replaced; the destination is bound to the preview and checked again before install.
The adapter keeps the plugin tree together,
uses native marketplace installation, and verifies the resulting registration. Catalogs can use
`.omp-plugin/marketplace.json` or the legacy `.claude-plugin/marketplace.json`.
Preview does not invoke OMP's native `--dry-run`, which can write files. Review
the source before applying: OMP can execute plugin
extensions and tools in-process at the next startup, without a project trust
prompt. Installation is not proof that those resources loaded successfully.

**Shared-cache limitation in OMP 18.6.1:** its native uninstall and upgrade can
delete or replace plugin cache files still referenced by a different project.
Native inventory lists only the user registry and the current project's registry,
so it cannot establish the safety of those operations across other projects.
Skillshare does not invoke those destructive commands. **Remove** and
disable-plus-sync instead use a scoped adapter that deletes the installation's
registry entry, runtime selection and verified `node_modules` link. The plugin
is uninstalled from that scope, not merely disabled. Shared cache, marketplaces
and plugin settings are retained; no cache garbage collection is attempted.
Reinstall automatically chooses a fresh marketplace/cache identity when the old
cache still exists, without replacing files another project may use. Unreadable,
linked or non-directory cache destinations remain blocked.

Removal requires the verified 18.6.1 metadata contract, matching installed
version, unambiguous JSON and proven runtime-link ownership. A real module folder,
foreign link, npm dependency collision or ambiguous runtime owner is refused.
If the cache manifest is missing, the adapter requires a unique runtime-lock key
whose scope-local link points to that installation's cache; it never guesses the
marketplace name and reports success with the actual runtime link still installed.
Unverifiable active installations remain blocked. If native uninstall already
removed the cache, registration and runtime link/selection, Forget drops only the
stale Skillshare binding; another plugin's `node_modules` directory does not block it.
Native file changes invalidate the preview. Partially completed removal can be
retried from the retained cache and binding identity without saving full native
settings backups. Windows writes additionally require verifiable private ACLs;
Skillshare does not change permissions to make them pass. No extension code runs.

The adapter serializes Skillshare removals and checks native file revisions, but
OMP's plugin commands do not participate in its lock; do not run native plugin
mutations concurrently. This is not an atomic multi-file native transaction.
**Updates remain blocked** until a non-destructive upgrade contract is available;
imported code is never upgraded without a reviewed source.

Project operations require the selected root to match OMP's native project
anchor; they never silently install globally or in an ancestor project. Native
profile/config-root overrides (`PI_CONFIG_DIR`, `PI_CODING_AGENT_DIR`,
`OMP_PROFILE` or `PI_PROFILE`) block plugin management. A relocated native cache
invalidates an existing preview. An arbitrary account `config_dir` does not redirect OMP's
plugin store, so OMP account plugin targets are not supported.

Native npm/Git/link packages are visible in inventory but are not imported as
marketplace plugins. Manage those packages in OMP; `plugin add npm:...` remains a
Pi-only workflow. Do not separately copy a plugin's extensions, hooks, skills or
MCP entries into Skillshare's other resources.

#### Choosing standalone OMP extensions

Open an OMP target's **Extensions** tab to review file-based selection. Cards group
files by source and scope, with plugins grouped by their inspected package root
and other files by directory. File lists remain visible. Plugin cards use a package
icon and `Plugins · <name>` heading from the native package identity, not the
extension folder name. Known manifest versions and scope stay visible, with an
**Open Plugins** navigation icon rather than repeated origin tags. Shared folders
are plugin-relative, and full file paths remain in tooltips. **Details** uses the
same labeled two-column layout as Pi for the full source, native name, selection
identifiers and notes. Missing identity metadata falls back to the inspected
folder; no package identity or version is guessed. Read-only
reasons and links to Hooks or Plugins remain visible with their entries.
Supported standalone modules have switches; **Preview** shows the change and **Apply**
writes only `disabledExtensions` in the selected native YAML settings file.
Global, project and explicitly configured account directories remain separate.
The editor preserves unrelated YAML and comments, checks the reviewed revision
again under OMP's native lock, and refuses stale or busy writes.

Selection editing requires a verifiable OMP **18.6.1** package behind the selected
launcher. Unidentified versions, standalone binaries and wrapper launchers stay
read-only. This check reads package metadata without running OMP. Same-name
module groups, explicit files that bypass the disabled-name filter, ambient
`hooks/pre|post` factories, uncertain selection and linked paths are read-only.
Hooks-owned and plugin-owned entries stay with their respective managers.

The backup records the before/after selection and settings hashes under
`omp-extensions/backups` in Skillshare's state directory, not unrelated YAML
values or credentials. It is a selection recovery record, not a whole-settings
restore. A successful apply describes configuration on disk, not a running
extension; restart/reload remains an OMP action.

### Pi and OpenCode

Pi keeps the first global registration and the last project registration of a
package. If an earlier global or later project source has an unresolved identity,
Skillshare cannot prove which entry owns a package and keeps potentially shadowed
entries Unknown/read-only, including inherited project deltas. It does not guess
identity by stripping URL queries. Proven entries outside that ambiguity remain editable.

Pi uses `pi install` / `pi remove`; inventory reads documented package settings
without loading extension code. `PI_CODING_AGENT_DIR` is respected. Pi project
trust must be established in Pi; Skillshare does not pass `--approve` for you.

#### npm packages from pi.dev

`plugin add npm:<package>` installs a package published to npm, such as one listed on
[pi.dev](https://pi.dev/packages), through Pi itself:

```bash
skillshare plugin add npm:@scope/package --target pi --dry-run --json -g
skillshare plugin add npm:@scope/package@1.2.0 --target pi --no-tui -g
```

Pi downloads the package and runs its install scripts, so Skillshare can't review its
content first; check the package on pi.dev or npm before adding it. `discover` does not
accept npm sources, and an npm source takes no `--source-ref`, `--entry` or `--plugin`.
Only Pi targets take npm sources, including a Pi account that runs `pi`. For an account
that runs another executable, install the package with that executable, then import it.
With `--project`, Pi installs the package into the project's settings; once the project
has a `.pi` folder, Pi changes its packages only after you trust the project in Pi.

Pi keeps one entry per package name. If Pi already has the same source, `add` imports
it; another version of the package is installed, and Pi replaces that entry's source.
`update` runs `pi update`, except for a package pinned to an exact version, which Pi
keeps: add the package again with the new version instead. For a package added without a
version, `check` compares the version in the installed package's `package.json` with the
version npm's `latest` tag names on the public registry, and `update` leaves a package that is
already at that version alone. A package added with a version range or tag, one that the
environment or an `.npmrc` sends to another registry, or one whose version is not a plain
`X.Y.Z`, is reported as something to check in Pi. The dashboard shows the installed version of every Pi package, on
the Plugins page and in a Pi target's Extensions tab. When you turned off some of the
package's extensions, Pi keeps those rules on the new version, and Skillshare records them
again so a later reinstall restores them. A Pi package that another Skillshare package
already manages is refused; update or remove that one instead.

In the dashboard's add dialog, you can paste the `pi install npm:<package>` command or the
package's pi.dev address; either becomes its `npm:` source.

#### Choosing a package's extensions

In the dashboard, the target page of `pi` and of a Pi account has an **Extensions**
tab. It lists each package entry of that target's `settings.json` with the
extensions its filters select. Packages managed by Skillshare use a package icon
and `Plugins · <name>` heading, matching OMP plugin cards. They retain the installed
version, scope and relevant project override shape; the navigation icon opens
Plugins. Unmanaged Pi packages keep their native identity and Pi icon. This
presentation does not change ownership or extension-selection rules. Extension
rows remain visible; **Details** independently shows the source and filter rules.
A switch writes one exact `+path` or `-path` rule
into that entry's `extensions` list. When removing the file's own exact rule already
gives the state the switch asks for, the switch removes that rule instead, so turning
a file back to what the remaining rules select leaves no rule behind. **Remove rule**,
shown when the switch would not remove the rule itself, deletes the exact rule for that
file, written as a relative or an absolute path, and the file then follows the
remaining rules; the preview shows the result. Apply shows a preview first and
edits only those lists: the entry's other keys, its `skills`, `prompts` and
`themes` filters, glob and `!` rules, and the rest of the file stay exactly as
written. A string entry becomes `{"source": ...}` so it can hold a rule. For a
string entry, Pi takes a package's skills, prompts and themes only from its `pi`
manifest, while an object entry also loads them from the package's `skills`,
`prompts` and `themes` folders when the manifest leaves them out. A string entry
of a package with such a folder is read-only, because Skillshare can't show that
converting it leaves those resources as they are. A source that is one file is
read-only too, because Pi loads it as it is and ignores filters. A user-scoped npm registration without a managed cache is Unknown/read-only, not
necessarily uninstalled: Pi may use a legacy global npm/pnpm root that Skillshare
does not probe. If the file changed after the preview, or Pi holds its settings lock, nothing is written.
An empty lock directory older than Pi's 10-second stale threshold can be reclaimed
only if its inode and mtime are unchanged. Fresh, renewed, replaced, nonempty,
file, and symlink locks are refused. Reclamation follows Pi's stale-lock protocol;
age does not prove an owner has died, and the final check/removal is not an atomic CAS.
While writing, Skillshare holds that lock the way Pi does and writes nothing if it
loses it. Before each apply, Skillshare saves a persistent record of the changed
extension lists and the before/after file hashes. Successful records are kept
without automatic pruning; if Apply fails, only the new record for that attempt
is removed. This is not a copy of `settings.json` and cannot restore the whole file. The tab lists every package in the settings, including ones installed with Pi itself, such as `npm:` packages from [pi.dev](https://pi.dev/packages). Skillshare installs and removes packages only through `plugin`: `plugin add` takes a local directory, a Git source or an [npm package](#npm-packages-from-pidev), and `plugin import --from pi` adopts a package installed with Pi.

Skillshare reads packages without running them, so the tab shows what the
settings select (switches and selection notes), not whether Pi loaded them; reload Pi
after applying. A file the settings name but the package lacks is marked as
missing. A choice Skillshare can't work out shows **Can't tell** with the reason
and where to change it, never a guessed on or off. Editing needs the target's own
Pi to be 0.99.2 or later (the oldest version checked against Pi itself) and
strict JSON settings; an older version is read-only and the tab says which version it found. A Pi account that runs a
different executable is read-only, and Skillshare does not run it. An entry whose
list is `[]` (nothing loads) is read-only, as is any extension decided by a
pattern Skillshare cannot evaluate, such as `?` against an emoji. An entry with an
empty source, or whose source or rules contain an unpaired UTF-16 surrogate escape
or invalid UTF-8, is left as written and read-only, since Skillshare can't read it
exactly as Pi does. Pi uses only the first global entry of a package, so when
Skillshare can't read that entry, the package's later entries are read-only too.

A project that syncs to Pi has the same tab on its project page. It shows each
package as the project's settings select it on top of the global ones, marked as
inherited from `pi (global)` or as a project override. A switch saves a rule to
the project's `.pi/settings.json` only, the way `pi config` does: a global package
gets a project entry `{"source": ..., "autoload": false, "extensions": [...]}`
that changes only the files it names and leaves the global entry as it is. A local
source is written relative to `.pi`, an npm or git source as the global settings
have it. When the last project rule of such an entry is removed, the entry is
removed only if that cannot reveal an earlier registration's filters; otherwise
the empty winning override is kept. Only explicit JSON `false` means a delta;
`autoload: null` is not `false`. A project entry with `autoload: false` and no global entry loads only the
files it names with `+`. The file, and its `.pi` folder, are created only when you
apply. The global settings and Pi's `trust.json` are never written, and
Skillshare never trusts a project for you: Pi uses the project's settings only if
it trusts the project. A global source that carries credentials or a query is
never copied into a project, so that package is read-only there, and so is every
package when the project's settings have an entry Skillshare can't read. Apply
holds Pi's lock on the project file and checks both settings files and the
package again right before it writes. Extensions in Pi's own `extensions`
folders, including files linked there by [extras](./extras.md), are listed
read-only with where to change them; a project lists its own folder, which Pi
reads only if it trusts the project, and the global one.

OpenCode registers the managed entry as a file URL in `opencode.json` or the
existing `opencode.jsonc`, preserving comments and unrelated entries. Version 1
uses `plugin`; version 2 uses `plugins`. `XDG_CONFIG_HOME` and an absolute global
`OPENCODE_CONFIG` are respected; ambiguous or unsupported overrides are rejected.
OpenCode must be on PATH so Skillshare can choose the version's schema.

A local OpenCode source must already include its built entry (`main`, a string
root export, or `index.js`) and required runtime dependencies. Skillshare does not
run build scripts or install dependencies into the source. Registration is not
proof the module loaded successfully; check OpenCode after reload.

Import accepts plain Pi package sources and, on Pi 0.99.2 or later,
filtered object entries with supported sources and option shapes. Preview lists
retained field names, never opaque values. Import changes neither native settings
nor installed files. The original entry is kept in private Skillshare state;
shared config stores only its digest. Sync/update retain the live entry. When
uninstalling, Skillshare captures its latest rules and options; reinstall restores
that object before native installation, avoiding a default-enabled window.
Multiple restores in one Apply recognize only that Apply's own exact writes;
unrelated settings changes still stop later restores.
Keep private state with these bindings: a missing, modified, or cross-target record
blocks restoration. On Windows, new registration directories use protected owner/SYSTEM
ACLs. Existing directories and records must grant access only to the current user and
privileged SYSTEM/Administrators principals; unsafe or unverifiable ACLs refuse import
or restoration. Skillshare does not rewrite existing ACLs: retain the records and repair
their access protection as the owner before retrying. Unresolved sources, ambiguous precedence, unsupported encoding,
and local references Pi would normalize remain read-only. Plain OpenCode entries
can be imported; filtered OpenCode entries are still rejected.
Imported Pi packages are updated with `pi update SOURCE` in global mode, which
keeps their settings entry; a project's are updated in Pi, because `pi update`
also reaches global packages. Imported OpenCode v1 packages are updated in their
native tool. OpenCode v2 global imports can use its native update command; project imports must be
updated natively because the v2 update command is global.

```bash
skillshare plugin add ./cursor-plugin --target cursor --no-tui
skillshare plugin add ./agy-plugin --target agy --no-tui -p
skillshare plugin add ./pi-package --target pi --no-tui
skillshare plugin add ./opencode-package --target opencode --no-tui
skillshare plugin import npm:my-pi-package --from pi --name my-package --no-tui
```

## Refs and explicit entries

Advanced options in **Add plugin** accept an optional Git ref and OpenCode entry.
Leave them empty for the normal guided flow. The terminal wizard accepts the same
flags; automation can use:

```bash
skillshare plugin discover obra/superpowers --source-ref v6.3.0 --json
skillshare plugin add owner/repo --source-ref v1.0.0 --target copilot --no-tui
skillshare plugin add ./package --entry dist/plugin.js --target opencode --no-tui
skillshare plugin update review --source-ref v1.1.0 --target claude --dry-run --json
```

Bindings record `source_ref` and the resolved `commit`. Installation uses the
reviewed commit; `check` and `update` resolve the configured ref again, so a branch
can advance while a commit remains pinned. `--revision` is a preview token, not
a Git ref. `--entry` is relative to each candidate's package root and must already
exist; it does not trigger a build or package-manager install.

Copilot and Antigravity CLI installs use reviewed local snapshots. Imports have
no reviewed source for reinstall: after removal, install in the native client and
sync again. Grok also requires native trust before install/reinstall. Skillshare
never supplies native trust approval flags.

## Compatibility and boundaries

- Claude requires its native `.claude-plugin/plugin.json` package.
- Codex accepts `.codex-plugin/plugin.json` and recognized portable root
  `plugin.json` packages. When a package has both, Codex installs from the portable
  root `plugin.json`, so Skillshare expects that manifest's version, or `1.0.0`
  when it has none.
  A Claude-only package is not silently converted.
- Sources may contain a marketplace with local plugin entries. External catalog
  catalogs are merged by plugin name/path. Conflicting paths are rejected; external
  entries are reported with instructions to add their repository directly or
  install natively and import. Command-based sources are not auto-approved.
- Complete source snapshots retain plugin scripts, assets, and safe relative
  symlinks (including `AGENTS.md → CLAUDE.md`). Absolute, escaping, dangling,
  cyclic, and `.git`-referencing links and special files are rejected; sources are limited to 20,000 files and 100 MiB.
- Native installation is not proof of runtime activation. Restart/reload the
  Agent and complete authentication or hook trust in that Agent.
- Codex runs `codex` from `PATH`. When it is not there, Skillshare tries Homebrew's
  `/opt/homebrew/bin` and `/usr/local/bin`, then the CLI inside the Codex desktop app
  (`ChatGPT.app` or `Codex.app` on macOS, `%LOCALAPPDATA%\OpenAI\Codex\bin` on Windows). Set
  [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli)
  on a machine where Codex is elsewhere. An account with its own `cli` is not searched.
- Codex native project installation is not provided by this adapter. Sync
  selection still works for global Codex installations.
- Codex has no update command, so an update adds the plugin again from the
  refreshed snapshot. Adding always enables it, so a plugin disabled in Codex is
  skipped. An imported Codex plugin is updated with
  `codex plugin marketplace upgrade NAME`, which reinstalls every plugin Codex
  installed from that marketplace, as Codex also does when it starts.
- An update skips a target it cannot reach and says why; the plugin's other
  Agents still update, and a skipped update stays pending for a later sync.
- Imported plugins retain their original marketplace identity. `check` cannot
  infer release availability for an imported plugin without a source, except an npm
  package in Pi, which it checks against npm. If an
  imported Claude or Codex plugin's native marketplace is gone, sync and update
  skip that target and say so. This also happens on another machine where that
  marketplace was never added. Restore the marketplace in the Agent, or remove
  the target and add the plugin again from its source; Skillshare never moves an
  imported plugin to another source on its own.
- Skillshare registers one marketplace per managed Claude/Codex plugin, named
  `skillshare-<plugin>-<hash>` (older installs keep `skillshare-<hash>`).
  Removing or excluding the plugin also removes that marketplace, even when the
  plugin is already gone; a failed cleanup is retried on the next sync. An update
  registers it again if it went missing. Same-name registrations at another path
  and imported plugins' marketplaces are left alone; snapshots and native caches
  are retained.
- These registrations point at this machine's Skillshare state directory, in
  user and project settings alike. Sharing Agent settings through Git or a
  dotfile manager carries paths that do not exist elsewhere; add the plugin from
  its source on each machine instead.
- Claude also reads a skill folder in its skills directory that has a plugin
  manifest as a plugin named `<name>@skills-dir`, and loads only one plugin per
  name. Adding a Claude plugin with the same name says so in the preview: Claude
  loads the plugin and skips the skill folder until one of them is renamed or
  removed.

The native lifecycle is exercised with Claude Code `2.1.276`, Codex CLI
`0.154.0`, Pi `0.85.1`, and Copilot CLI `1.0.86`. Antigravity CLI
`1.2.6` was checked with isolated native install/list/remove operations. OpenCode `1.18.31` is used to verify version-aware
registration; the v2 schema is covered by fixture tests. Cursor and Antigravity
filesystem lifecycles are tested in isolated directories, without claiming GUI
runtime activation. Installed command capabilities and inventory schemas are checked at
runtime; unsupported operations are blocked with an explanation.

## Official format references

- [Cursor local plugins](https://prod.cursor.com/docs/plugins)
- [Antigravity desktop plugins](https://www.antigravity.google/docs/plugins)
- [Antigravity standalone CLI plugins](https://www.antigravity.google/docs/cli/plugins)
- [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)
- [OpenCode v1 plugins](https://opencode.ai/docs/plugins/)
- [OpenCode v2 plugins](https://opencode.ai/v2/docs/plugins)
