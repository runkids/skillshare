---
sidebar_position: 5
---

# enable / disable

Temporarily enable or disable skills without removing them.

```bash
skillshare disable draft-*          # Disable by pattern
skillshare enable draft-*           # Re-enable
skillshare disable "frontend/**"    # Disable every skill in a folder
skillshare disable my-skill -p      # Project mode
```

## When to Use

- Temporarily hide a skill from sync without uninstalling it
- Mute a draft or experimental skill across all targets
- Toggle skills on/off from the list TUI with the `t` key

## How It Works

`disable` adds a pattern to `.skillignore`; `enable` removes it. Disabled skills stay in the source directory but are excluded from `sync` and `collect`.

A skill name refers to every skill in a folder of that name, so `enable inner` affects both `group/inner` and `other/inner`. An exact path such as `group/inner`, or a flat name such as `group__inner`, refers to that skill alone. For each skill a name refers to, `enable` also removes the line for its path (the line the list TUI and dashboard write), so `enable inner` removes `group/inner` too; each `✓` row shows the line that was written or removed. `disable` writes the name as typed when that line matches the skill; a flat name, which no line can match, writes the skill's path instead.

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
| `<name\|pattern>` | One or more skill names or glob patterns (e.g., `draft-*`, `frontend/**`) |
| `--project, -p` | Use project `.skillignore` (`.skillshare/skills/.skillignore`) |
| `--global, -g` | Use global `.skillignore` (`~/.config/skillshare/.skillignore`) |
| `--dry-run, -n` | Preview without writing |
| `--help, -h` | Show help |

Mode is auto-detected when neither `-p` nor `-g` is specified (same as other commands).

## Examples

```bash
# Disable a single skill
$ skillshare disable my-draft
✓ my-draft  added to .skillignore

Next
  skillshare sync  apply the change

# Disable by glob pattern
$ skillshare disable "experimental-*"
✓ experimental-*  added to .skillignore

Next
  skillshare sync  apply the change

# Re-enable
$ skillshare enable my-draft
✓ my-draft  removed from .skillignore

Next
  skillshare sync  apply the change

# Preview without writing
$ skillshare disable my-skill --dry-run
  my-skill  would be added to ~/.config/skillshare/skills/.skillignore

Dry run — nothing was written

# The preview lists every line, including the path lines enable removes
$ skillshare enable inner --dry-run
  inner        would be removed from ~/.config/skillshare/skills/.skillignore
  group/inner  would be removed from ~/.config/skillshare/skills/.skillignore

Dry run — nothing was written

# Already disabled
$ skillshare disable my-draft
! my-draft  already disabled
```

## When Another Rule Decides

`disable` and `enable` write or remove one line, but other rules can still decide a skill's state: a glob such as `draft-*`, a folder rule such as `frontend/`, a `!` rule in `.skillignore.local`, or a tracked repo's own `.skillignore`. After writing, skillshare checks each skill or agent you named as `skillshare list` reports it. If one is still in its old state, a `✗` line names the file to edit and the command exits with status 1:

```bash
$ skillshare enable draft-one

✗ could not enable draft-one: still disabled: another rule in .skillignore or .skillignore.local matches it — remove that rule to enable it
```

When several are blocked, each gets its own `✗` row. Changes that did take effect are kept and shown with `✓`, and the operation log records the run as `partial`.

A glob argument such as `"draft-*"` names no single skill, so it is written or removed without this check.

## Disable a Whole Folder

`disable`/`enable` accept the same glob syntax as `.skillignore`, so there is no separate "group" flag — point a pattern at the folder and every skill inside is toggled at once.

```bash
# Disable every skill under frontend/ (any depth)
$ skillshare disable "frontend/**"
✓ frontend/**  added to .skillignore

Next
  skillshare sync  apply the change

# Re-enable the whole folder
$ skillshare enable "frontend/**"
✓ frontend/**  removed from .skillignore

Next
  skillshare sync  apply the change
```

:::tip Quote the pattern
Always wrap folder patterns in quotes (`"frontend/**"`) so your shell doesn't expand `*` before skillshare sees it.
:::

`frontend/**` writes a single line to `.skillignore` and keeps covering anything you add to the folder later. `enable` with the **same** pattern removes that line. To disable individual skills instead, list them by name (`skillshare disable a b c`). See [.skillignore pattern syntax](/docs/reference/filtering#skillignore) for the full glob reference (`*`, `**`, `?`, `[abc]`, `!negation`, anchored `/`, directory-only `pattern/`).

## TUI Toggle

You can also turn skills on and off in the `skillshare list` TUI. The change is written to `.skillignore` immediately. If another rule keeps the skill in its old state, the row stays as it was and the key line shows the same `✗` message as the CLI.

## Where is the .skillignore?

| Mode | Path |
|------|------|
| Global | `~/.config/skillshare/skills/.skillignore` |
| Project | `.skillshare/skills/.skillignore` |

The file is created automatically on first `disable`.

## Agent Support

Use `--kind agent` to enable or disable agents. This writes to `.agentignore` instead of `.skillignore`:

```bash
skillshare disable --kind agent draft-reviewer     # Disable an agent
skillshare enable --kind agent draft-reviewer      # Re-enable an agent
skillshare disable --kind agent "experimental-*"   # Disable by pattern
```

`.agentignore` lines match the file name, `.md` included. A name without `.md` that resolves to an agent with that file name is written with `.md` added, so `draft-reviewer` writes `draft-reviewer.md`. An agent's frontmatter `name` writes its path instead, so `disable --kind agent critic` for `rev.md` writes `rev.md`.

| Mode | `.agentignore` path |
|------|---------------------|
| Global | `~/.config/skillshare/agents/.agentignore` |
| Project | `.skillshare/agents/.agentignore` |

See [Agents](/docs/understand/agents) for background on agent management.

## See Also

- [list](./list.md) — View disabled skills and toggle with `t` key
- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — All filtering layers
- [.skillignore](/docs/reference/filtering#skillignore) — Pattern syntax
- [sync](./sync.md) — Apply changes after enable/disable
- [Agents](/docs/understand/agents) — Agent concepts
