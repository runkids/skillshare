---
sidebar_position: 8
---

# hooks

Manage standalone hook bundles for Claude and Codex.

```bash
skillshare hooks list
skillshare hooks import --from claude|codex
skillshare hooks sync [name...] --target claude|codex|all
```

## What hooks means in skillshare

Hook bundles are source-managed wrappers around hook scripts plus target-specific event wiring.

- Global source root: `~/.config/skillshare/hooks/`
- Project source root: `.skillshare/hooks/`
- Supported targets: `claude`, `codex`

A bundle is a directory containing:

- `hook.yaml` with `claude:` and/or `codex:` event sections
- optional `scripts/`

Commands in `hook.yaml` may use `{HOOK_ROOT}`. During sync that placeholder is rewritten to the rendered bundle root for the selected target.

Claude hook entries are matcher-aware. In `hook.yaml`, each Claude event entry can use the legacy shorthand:

```yaml
claude:
  events:
    PreToolUse:
      - command: "{HOOK_ROOT}/scripts/pre.sh"
```

Or the current matcher-group form flattened into bundle entries:

```yaml
claude:
  events:
    PreToolUse:
      - matcher: Bash
        command: "{HOOK_ROOT}/scripts/pre.sh"
        timeout: 8000
        status_message: Enriching with graph context...
        if: "Bash(git *)"
```

During Claude sync, skillshare renders the current `settings.json` matcher-group shape:

- Tool events default to matcher `*`
- Non-tool events default to matcher `""`
- Existing unmanaged handlers in the same matcher group are preserved

## Commands

### `skillshare hooks list`

List discovered hook bundles in the current source root.

```bash
skillshare hooks list
skillshare hooks list --json
```

Text output shows event counts per target:

```text
Hooks
audit  claude=2 codex=1
notify  claude=1 codex=0
```

JSON output:

```json
[
  {
    "name": "audit",
    "source_dir": "/home/user/.config/skillshare/hooks/audit",
    "targets": {
      "claude": 2,
      "codex": 1
    }
  }
]
```

### `skillshare hooks import`

Import hook definitions from local Claude or Codex config into standalone bundles.

```bash
skillshare hooks import --from claude --all
skillshare hooks import --from claude --owned-only
skillshare hooks import --from codex --all -p
```

Behavior:

- Claude import reads `.claude/settings.json`, and also merges legacy `.claude/hooks.json` when present.
- Codex import reads `.codex/hooks.json`.
- `--all` and `--owned-only` are mutually exclusive.
- `--owned-only` imports only commands already pointing at the Skillshare-managed hook roots.
- `--all` imports every discovered hook entry.
- When import sees a local script path, it copies that file into the bundle `scripts/` directory and rewrites the command to `{HOOK_ROOT}` while preserving wrapper tokens such as `node "..."`.
- When import cannot isolate a local script path, it keeps the command verbatim and records a warning instead of dropping the hook.

Warning-only import example:

```yaml
claude:
  events:
    SessionStart:
      - command: "echo hello from shell wrapper"
```

That command stays verbatim on import and the bundle records a warning instead of failing import.

Imported commands are rewritten to use `{HOOK_ROOT}` and copied into `.skillshare/hooks/<name>/scripts/` or the global hooks source.

### `skillshare hooks sync`

Render scripts into managed hook roots, then merge hook entries back into target config files.

```bash
skillshare hooks sync
skillshare hooks sync audit --target claude
skillshare hooks sync --target all --json
```

Managed roots and config files:

- Claude:
  - Root: `~/.claude/hooks/skillshare/<name>/` or `.claude/hooks/skillshare/<name>/`
  - Config: `~/.claude/settings.json` or `.claude/settings.json`
- Codex:
  - Root: `~/.codex/hooks/skillshare/<name>/` or `.codex/hooks/skillshare/<name>/`
  - Config: `~/.codex/hooks.json` or `.codex/hooks.json`
  - Feature flag: `~/.codex/config.toml` has `features.codex_hooks = true`

Sync is merge-based:

- Managed entries under the Skillshare hook root are updated in place.
- Unmanaged hook entries already present in the config are preserved.
- `--target all` may include successful rows plus warning-only no-op rows such as `no codex hooks defined`.
- A bundle can also render successfully and still emit warnings, for example when unsupported Codex events are skipped.

JSON output:

```json
{
  "hooks": [
    {
      "name": "audit",
      "target": "claude",
      "root": "/home/user/.claude/hooks/skillshare/audit",
      "merged": true
    }
  ]
}
```

## Supported Codex events

Codex sync accepts:

- `PreToolUse`
- `PostToolUse`
- `Notification`
- `SessionStart`
- `SessionEnd`

Other Codex events are not synced. They surface as warnings such as `unsupported codex event X not synced`.

Concrete `--target all` example for a Claude-only bundle:

```json
{
  "hooks": [
    {
      "name": "audit",
      "target": "claude",
      "root": "/home/user/.claude/hooks/skillshare/audit",
      "merged": true
    },
    {
      "name": "audit",
      "target": "codex",
      "warnings": ["no codex hooks defined"]
    }
  ]
}
```

## Hook bundle flow

The hook flow is distinct from plugin flow and from skill sync:

```text
source bundle
  -> copy scripts into managed hook root
  -> rewrite {HOOK_ROOT} placeholders
  -> merge managed entries back into target config
  -> preserve unmanaged entries already present
```

## Related JSON/report surfaces

Hook bundles also appear in:

- [`status --json`](./status.md)
- [`diff --json`](./diff.md)
- [`doctor --json`](./doctor.md)
- [`sync --all --json`](./sync.md)

Server endpoints:

- `GET /api/hooks`
- `GET /api/hooks/diff`
- `POST /api/hooks/import`
- `POST /api/hooks/sync`

## Options

| Flag | Applies to | Description |
|------|------------|-------------|
| `--json` | `list`, `sync` | Machine-readable output |
| `--from claude|codex` | `import` | Import source |
| `--all` | `import` | Import all hooks |
| `--owned-only` | `import` | Import only Skillshare-managed hooks |
| `--target claude|codex|all` | `sync` | Target selection |
| `--project, -p` | all | Use project mode |
| `--global, -g` | all | Use global mode |

## See also

- [plugins](./plugins.md)
- [doctor](./doctor.md)
- [Source & Targets](/docs/understand/source-and-targets)
