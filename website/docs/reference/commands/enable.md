---
sidebar_position: 5
---

# enable / disable

Temporarily enable or disable skills without removing them.

```bash
skillshare disable draft-*          # Disable by pattern
skillshare enable draft-*           # Re-enable
skillshare disable my-skill -p      # Project mode
```

## When to Use

- Temporarily hide a skill from sync without uninstalling it
- Mute a draft or experimental skill across all targets
- Toggle skills on/off from the list TUI with the `E` key

## How It Works

`disable` adds a pattern to `.skillignore`; `enable` removes it. Disabled skills stay in the source directory but are excluded from `sync` and `collect`.

```mermaid
flowchart LR
    DIS["skillshare disable my-skill"]
    IGN[".skillignore += my-skill"]
    SYNC["sync skips my-skill"]
    DIS --> IGN --> SYNC
```

```mermaid
flowchart LR
    EN["skillshare enable my-skill"]
    IGN[".skillignore -= my-skill"]
    SYNC["sync includes my-skill"]
    EN --> IGN --> SYNC
```

:::tip
After enabling or disabling, run `skillshare sync` to apply the change to targets.
:::

## Options

| Flag | Description |
|------|-------------|
| `<name\|pattern>` | One or more skill names or patterns (e.g., `draft-*`) |
| `--project, -p` | Use project `.skillignore` (`.skillshare/skills/.skillignore`) |
| `--global, -g` | Use global `.skillignore` (`~/.config/skillshare/.skillignore`) |
| `--dry-run, -n` | Preview without writing |
| `--help, -h` | Show help |

Mode is auto-detected when neither `-p` nor `-g` is specified (same as other commands).

## Examples

```bash
# Disable a single skill
$ skillshare disable my-draft
Disabled: my-draft (added to .skillignore)
Run 'skillshare sync' to apply changes.

# Disable by glob pattern
$ skillshare disable "experimental-*"
Disabled: experimental-* (added to .skillignore)
Run 'skillshare sync' to apply changes.

# Re-enable
$ skillshare enable my-draft
Enabled: my-draft (removed from .skillignore)
Run 'skillshare sync' to apply changes.

# Preview without writing
$ skillshare disable my-skill --dry-run
Would add 'my-skill' to ~/.config/skillshare/skills/.skillignore

# Already disabled
$ skillshare disable my-draft
warning: my-draft is already disabled
```

## TUI Toggle

In the interactive `skillshare list` TUI, press **E** to toggle the selected skill's enabled/disabled state. The change is written to `.skillignore` immediately — no need to exit the TUI first.

Disabled skills show a red **disabled** badge in the detail panel.

## Where is the .skillignore?

| Mode | Path |
|------|------|
| Global | `~/.config/skillshare/skills/.skillignore` |
| Project | `.skillshare/skills/.skillignore` |

The file is created automatically on first `disable`.

## See Also

- [list](./list.md) — View disabled skills and toggle with `E` key
- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — All filtering layers
- [.skillignore](/docs/reference/filtering#skillignore) — Pattern syntax
- [sync](./sync.md) — Apply changes after enable/disable
