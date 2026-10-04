---
sidebar_position: 7
---

# unfollow

Stop following a first-level link in the skills source. The link's target is never touched.

```bash
skillshare unfollow _team-skills               # Remove the declaration and the link
skillshare unfollow _team-skills --keep-link   # Remove the declaration only
skillshare unfollow _team-skills --local       # Remove it from .skillfollow.local only
skillshare unfollow _team-skills -p            # Project mode
```

## What It Does

1. Removes `<name>` from every declaration file that contains it (`.skillfollow` and `.skillfollow.local`, which form a union) and lists each file it edited. Comments and other entries are kept.
2. Removes the link `<source>/<name>` itself. A real directory (`not-link`) is never removed.
3. Removes the link's ignore line from the managed block of the source's `.gitignore`, only when the link was removed. Otherwise the line is kept and mentioned.

With `--local`, only `.skillfollow.local` is edited. If `.skillfollow` still declares the name, `unfollow` says the entry remains followed and leaves the link in place.

If a write fails, `unfollow` reports failure and names any file it had already edited. It never reports a partial unfollow as success.

After an unfollow, run `skillshare sync`: the entry's skills are no longer discovered, so sync prunes their managed links. See [Cleanup safety](../skillfollow.md#cleanup) for how a link that points directly at the external path is handled.

## Options

| Flag | Description |
|------|-------------|
| `<name>` | Declared first-level entry |
| `--local` | Remove the name from `.skillfollow.local` only |
| `--keep-link` | Keep the link and its ignore line |
| `--project, -p` | Use the project skills source (`.skillshare/skills/`) |
| `--global, -g` | Use the global skills source |
| `--json` | Output as JSON |
| `--help, -h` | Show help |

## Examples

```bash
$ skillshare unfollow _team-skills
✓ _team-skills  removed from .skillfollow
✓ _team-skills  link removed; its target was not touched
✓ .gitignore    removed /_team-skills

Next
  skillshare sync  prune the entry's managed links

# Keep the link
$ skillshare unfollow _team-skills --keep-link
✓ _team-skills  removed from .skillfollow
  _team-skills  link kept: --keep-link
  .gitignore    kept /_team-skills

# The committed file still declares it
$ skillshare unfollow _team-skills --local
✓ _team-skills  removed from .skillfollow.local
! _team-skills  still declared in .skillfollow; it remains followed

# A real directory stays
$ skillshare unfollow realdir
✓ realdir  removed from .skillfollow
  realdir  link kept: not a link; a real directory is never removed
```

## JSON Output

```bash
skillshare unfollow _team-skills --json
```

```json
{
  "name": "_team-skills",
  "source": "/home/me/.config/skillshare/skills",
  "files_edited": [".skillfollow"],
  "still_declared_in": [],
  "not_declared": false,
  "link": "/home/me/.config/skillshare/skills/_team-skills",
  "link_removed": true,
  "ignore_line": "/_team-skills",
  "ignore_line_removed": true,
  "ignore_line_kept": false
}
```

`link_kept` gives the reason when the link stays. A failure prints `{"error": "..."}` and exits with status 1.

## See Also

- [follow](./follow.md) — Declare an entry
- [.skillfollow](../skillfollow.md) — File format, states, and safety rules
- [sync](./sync.md) — Prune the entry's managed links
