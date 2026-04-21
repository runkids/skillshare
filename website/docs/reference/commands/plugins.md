---
sidebar_position: 7
---

# plugins

Manage native plugin bundles for Claude and Codex.

```bash
skillshare plugins list
skillshare plugins import <plugin-ref-or-path> --from claude|codex
skillshare plugins sync [name...] --target claude|codex|all
skillshare plugins install <plugin-ref-or-path> --from claude|codex
```

## What plugins means in skillshare

Plugin bundles live in the Skillshare source tree and are separate from skills, agents, and extras:

- Global source root: `~/.config/skillshare/plugins/`
- Project source root: `.skillshare/plugins/`
- Supported targets: `claude`, `codex`

Each bundle can contain:

- `.claude-plugin/plugin.json`
- `.codex-plugin/plugin.json`
- Shared files such as `skills/`, `assets/`, or `vendor/`
- Optional `skillshare.plugin.yaml` metadata used to generate a missing target manifest

## Commands

### `skillshare plugins list`

List discovered plugin bundles in the current source root.

```bash
skillshare plugins list
skillshare plugins list --json
skillshare plugins list -p
```

Text output shows one bundle per line with target availability:

```text
Plugins
demo  claude=true codex=false
audit  claude=true codex=true
```

JSON output is an array of bundle objects:

```json
[
  {
    "name": "demo",
    "source_dir": "/home/user/.config/skillshare/plugins/demo",
    "has_claude": true,
    "has_codex": false
  }
]
```

### `skillshare plugins import`

Import a plugin bundle from a local directory or from local Claude/Codex plugin state.

```bash
skillshare plugins import demo --from claude
skillshare plugins import audit-tool@skillshare --from codex
skillshare plugins import ./fixtures/my-plugin
```

Rules:

- If the argument is a local directory, `--from` is optional.
- If the argument is a reference, `--from claude|codex` is required.
- Import copies the plugin into the Skillshare source root; it does not activate it on targets.
- Claude import resolves local installs from `~/.claude/plugins/installed_plugins.json` and the recorded `installPath` entries. If a short name matches multiple installed refs, import fails with `use full ref`.
- Codex import resolves local installs from hashed plugin caches under `~/.codex/plugins/cache/<provider>/<name>/<hash>/` plus configured refs in `~/.codex/config.toml`. If a short name matches multiple refs, import fails with `use full ref`.

Ambiguous short-name example:

```bash
skillshare plugins import demo --from claude
# claude plugin "demo" is ambiguous; use full ref

skillshare plugins import demo@alpha --from claude
```

### `skillshare plugins sync`

Render plugin bundles into target-specific marketplace roots and optionally install/enable them.

```bash
skillshare plugins sync
skillshare plugins sync demo --target claude
skillshare plugins sync --target all --json
skillshare plugins sync --target codex --no-install
```

Behavior:

- Claude render root:
  - Global: `~/.config/skillshare/rendered/claude-marketplace/plugins/<name>/`
  - Project: `.skillshare/rendered/claude-marketplace/plugins/<name>/`
- Codex render root:
  - Global: `~/.agents/plugins/<name>/`
  - Project: `.agents/plugins/<name>/`
- Codex install cache:
  - Global only: `~/.codex/plugins/cache/skillshare/<name>/local/`

With installation enabled, sync also:

- Updates marketplace indexes for Claude/Codex
- Runs the Claude plugin install/update/enable flow. The install vs update decision uses Claude installed-plugin metadata, not bare directory heuristics.
- Enables the Codex plugin in `~/.codex/config.toml`

Project nuance:

- Project-scoped plugin source is supported.
- Codex plugin activation still updates the global `~/.codex/config.toml`.

JSON output:

```json
{
  "plugins": [
    {
      "name": "demo",
      "target": "codex",
      "rendered": "/home/user/.agents/plugins/demo",
      "installed": true,
      "generated": false
    }
  ]
}
```

### `skillshare plugins install`

Convenience wrapper for `import` plus `sync`.

```bash
skillshare plugins install demo --from claude
skillshare plugins install ./fixtures/my-plugin --from codex --target codex
```

This command:

1. Imports the plugin bundle into the Skillshare source root.
2. Syncs plugins to the selected target or `all`.
3. Installs/enables the plugin unless you use `plugins sync --no-install` directly instead.

## How plugin bundles translate across targets

The plugin flow is distinct from skill sync:

```text
source bundle
  -> stage bundle for one target
  -> copy shared files
  -> keep existing native manifest or generate one from shared metadata
  -> render into marketplace root
  -> optionally install/enable in the target runtime
```

If only one native manifest exists, skillshare can generate the missing target manifest from:

- the existing native manifest, plus
- `skillshare.plugin.yaml` shared metadata, when present

That generated-target capability is also reflected in JSON surfaces such as `diff --json`, `/api/plugins/diff`, and doctor checks, so those views only report targets a bundle can actually sync to.

Warnings may be emitted when translation skips unsupported shared directories such as `commands`, `agents`, or `hooks`.

Concrete generated-target example:

```text
bundle source:
  .claude-plugin/plugin.json
  skillshare.plugin.yaml

skillshare plugins sync demo --target all
```

That single bundle can sync to both Claude and Codex because Skillshare generates the missing `.codex-plugin/plugin.json` from shared metadata.

## Related JSON/report surfaces

Plugin bundles also appear in:

- [`status --json`](./status.md)
- [`diff --json`](./diff.md)
- [`doctor --json`](./doctor.md)
- [`sync --all --json`](./sync.md)

Server endpoints:

- `GET /api/plugins`
- `GET /api/plugins/diff`
- `POST /api/plugins/import`
- `POST /api/plugins/sync`

## Options

| Flag | Applies to | Description |
|------|------------|-------------|
| `--json` | `list`, `sync` | Machine-readable output |
| `--from claude|codex` | `import`, `install` | Import source |
| `--target claude|codex|all` | `sync`, `install` | Target selection |
| `--no-install` | `sync` | Render only; skip target activation |
| `--project, -p` | all | Use project mode |
| `--global, -g` | all | Use global mode |

## See also

- [hooks](./hooks.md)
- [sync](./sync.md)
- [Source & Targets](/docs/understand/source-and-targets)
