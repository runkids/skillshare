---
sidebar_position: 45
---

# cli

Find a Skillshare command by describing a task, without loading configuration or contacting a service.

## Usage

```bash
skillshare cli search <task words...> [--limit N] [--json]
skillshare cli search "scan security"
skillshare cli search restore backup --limit 3 --json
```

This searches the installed binary's public **top-level command names and English help summaries**. It uses lexical matching, not an AI model. It does not index every subcommand or flag. Follow a result with `skillshare <command> --help` to find its subcommands, flags, and examples.

This is separate from [`skillshare search`](/docs/reference/commands/search), which discovers remote skills. Command discovery works offline, even before `init`, and does not load themes, migrate configuration, or check for updates. It does not execute any matched command.

## Flags

| Flag | Description |
|------|-------------|
| `--limit N` | Maximum results, from 1 to 20 (default: 5); `--limit=N` also works |
| `--json`, `-j` | Emit only JSON |
| `--help`, `-h` | Show usage and examples |
| `--` | End options; subsequent arguments are task words |

An empty or whitespace-only task is a usage error. A nonempty task with no matches succeeds with an empty result list. Internal commands, protocol subcommands, discovery itself, help, completion, version, and UI runners are not search results.

## Output

Plain output lists matching commands and their summaries, followed by a help hint for the first result. With no matches, the hint points to `skillshare --help`.

JSON returns `query` and `results`. Each result contains `command`, `summary`, and `usage`. The `usage` value is the command's `--help` invocation, not a complete task invocation. Ranking scores are not part of the interface. The query preserves the joined task arguments.

```bash
skillshare cli search "unmatchedxyz" --json
```

```json
{
  "query": "unmatchedxyz",
  "results": []
}
```

Invalid arguments exit with status 1. With `--json` before the `--` delimiter, usage errors have this shape:

```json
{
  "error": {
    "code": "usage",
    "message": "search requires a non-empty task description",
    "hint": "Run 'skillshare cli search --help' for usage and examples."
  }
}
```

The stable `usage` code belongs to `cli` argument validation. It does not reclassify errors or change JSON formats from other commands.
