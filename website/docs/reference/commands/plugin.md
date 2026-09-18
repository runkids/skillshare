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

## Commands

| Command | Behavior |
|---|---|
| `list` | Configured bindings and native installation state; interactive manager in a terminal |
| `discover SOURCE` | Inspect a local directory, `owner/repo`, or HTTPS Git repository |
| `add [SOURCE]` | Choose and install a whole plugin with its target adapter |
| `import [NATIVE-ID]` | Adopt an existing installation without reinstalling or enabling it |
| `inspect NAME` | Inspect one managed package |
| `sync [NAME]` | Reconcile selected targets and retry incomplete native operations |
| `check [NAME]` | Compare source content with the recorded digest; never update |
| `update [NAME]` | Review source changes and use a supported native update operation |
| `enable / disable [NAME]` | Include/exclude a target in the next sync |
| `remove [NAME]` | Uninstall managed bindings and remove their definitions |

Bare commands prompt for missing inputs in a terminal. Noninteractive mutation
commands require explicit inputs. `sync` and `check` may operate on all packages.
`sync --all` does **not** include plugins; use `sync plugins` explicitly.

## Options

| Option | Meaning |
|---|---|
| `--target TARGET` | Repeatable selection: `claude`, `codex`, `cursor`, `antigravity` (`agy` alias), `antigravity-cli`, `copilot`, `grok`, `pi`, `opencode`; see the capability table below |
| `--plugin NAME` | Select one plugin from a source marketplace |
| `--name NAME` | Logical package name when adding or importing |
| `--from TARGET` | Import from Claude, Codex, Antigravity CLI, Copilot, Grok, Pi, or OpenCode |
| `--dry-run`, `-n` | Preview without changing Skillshare or Agent configuration |
| `--source-ref REF` | Git branch, tag, or commit for `discover`, `add`, and `update`; remote sources only |
| `--entry PATH` | Explicit built OpenCode JS/TS entry, relative to the package root (`discover` and `add`) |
| `--revision ID` | Refuse application if source, configuration, or native inventory changed since preview |
| `--json` | Machine-readable output; disables TUI |
| `--no-tui` | Disable interactive menus; also respects `tui: false` |
| `--global`, `-g` | Global Skillshare config and native user scope |
| `--project`, `-p` | Project config; Claude, Antigravity, Pi, or OpenCode (never global fallback) |

JSON output includes source paths and native identifiers. Do not put credentials
in source URLs. A failed mutation can still return successful per-target outcomes;
the CLI exits nonzero when any target fails. Inspect the result before retrying.

## Target support

| Target | Format | Global | Project | Update |
|---|---|:---:|:---:|---|
| Claude Code | `.claude-plugin/plugin.json` | Yes | Yes | Native update |
| Codex | `.codex-plugin/plugin.json` or Agent Plugins root manifest | Yes | No | Not supported |
| Cursor | `.cursor-plugin/plugin.json` or Agent Plugins root manifest | Yes | No | Replace reviewed local copy |
| Antigravity Desktop | Root `plugin.json` with an explicit name | Yes | Yes | Replace reviewed local copy |
| Pi | `package.json` with `pi` resources, or `pi-package` keyword and conventional resource folders | Yes | Yes, with native project trust | Refresh managed source snapshot |
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

### Pi and OpenCode

Pi uses `pi install` / `pi remove`; inventory reads documented package settings
without loading extension code. `PI_CODING_AGENT_DIR` is respected. Pi project
trust must be established in Pi; Skillshare does not pass `--approve` for you.

OpenCode registers the managed entry as a file URL in `opencode.json` or the
existing `opencode.jsonc`, preserving comments and unrelated entries. Version 1
uses `plugin`; version 2 uses `plugins`. `XDG_CONFIG_HOME` and an absolute global
`OPENCODE_CONFIG` are respected; ambiguous or unsupported overrides are rejected.
OpenCode must be on PATH so Skillshare can choose the version's schema.

A local OpenCode source must already include its built entry (`main`, a string
root export, or `index.js`) and required runtime dependencies. Skillshare does not
run build scripts or install dependencies into the source. Registration is not
proof the module loaded successfully; check OpenCode after reload.

Import accepts plain Pi package sources and plain OpenCode config entries.
Entries with resource filters/options are rejected to preserve those settings.
Imported Pi and OpenCode v1 packages are updated in their native tool. OpenCode
v2 global imports can use its native update command; project imports must be
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
  `plugin.json` packages. A Claude-only package is not silently converted.
- Sources may contain a marketplace with local plugin entries. External catalog
  catalogs are merged by plugin name/path. Conflicting paths are rejected; external
  entries are reported with instructions to add their repository directly or
  install natively and import. Command-based sources are not auto-approved.
- Complete source snapshots retain plugin scripts, assets, and safe relative
  symlinks (including `AGENTS.md → CLAUDE.md`). Absolute, escaping, dangling,
  cyclic, and `.git`-referencing links and special files are rejected; sources are limited to 20,000 files and 100 MiB.
- Native installation is not proof of runtime activation. Restart/reload the
  Agent and complete authentication or hook trust in that Agent.
- Codex native project installation and plugin updates are not provided by this
  adapter. Sync selection still works for global Codex installations.
- Imported plugins retain their original marketplace identity. `check` cannot
  infer release availability for an imported plugin without a source.
- Removal retains shared marketplace registrations and managed snapshots; it
  does not delete unrelated plugins or native caches directly.

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
