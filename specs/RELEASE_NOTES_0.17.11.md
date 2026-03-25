# skillshare v0.17.11 Release Notes

Release date: 2026-03-25

## TL;DR

1. **Extras flatten** — new `flatten: true` option for extras targets syncs all files from subdirectories directly into the target root, solving the problem where AI tools like Claude Code only discover top-level files
2. **Full CLI support** — `--flatten` for `extras init`, `--flatten`/`--no-flatten` for `extras mode`, flatten indicator in `extras list`
3. **TUI + Web UI** — flatten toggle in the init wizard and Extras page, with config editor validation

---

## Extras Flatten

AI CLI tools like Claude Code's `/agents` only discover files at the top level of their config directory — they don't recurse into subdirectories. If you organize your extras source with subdirectories (e.g., `agents/curriculum/`, `agents/software/`), synced files end up in those subdirectories and are invisible to the tool.

The new `flatten` option fixes this:

```yaml
extras:
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true
```

With `flatten: true`, `source/curriculum/tactician.md` syncs to `target/tactician.md` instead of `target/curriculum/tactician.md`.

**Collision handling**: When files in different subdirectories share the same basename (e.g., `team-a/agent.md` and `team-b/agent.md`), the first file wins (sorted alphabetically by path) and subsequent collisions are skipped with a warning.

**Constraints**: Flatten only works with `merge` and `copy` modes — it cannot be used with `symlink` mode (which links the entire directory).

## CLI Flags

Create a new extra with flatten:

```bash
skillshare extras init agents --target ~/.claude/agents --flatten
```

Toggle flatten on an existing target:

```bash
skillshare extras agents --flatten
skillshare extras agents --no-flatten
```

The `extras list` output now shows a `, flatten` indicator next to the mode for flatten-enabled targets.

## TUI

The `extras init` interactive wizard now includes a "Flatten files into target root? (y/N)" prompt after mode selection. It is automatically skipped when `symlink` mode is selected.

The extras list TUI adds a new `F` keybinding to toggle flatten on/off for a target. For single-target extras it toggles directly; for multi-target extras it shows a target picker first.

## Web UI

The Extras page now shows a flatten checkbox per target — both in the "Add Extra" modal and on existing targets. The checkbox is disabled when mode is `symlink`. The YAML config editor warns when `flatten: true` is combined with `mode: symlink`.

## Config Editor — Target Name Docs

Clicking a target name in the config editor now shows the correct "target name" field documentation. Previously, clicking `name: claude` under `targets` incorrectly showed the sync mode documentation. Short-form entries like `- agents` are also recognized and show the same target name docs.
