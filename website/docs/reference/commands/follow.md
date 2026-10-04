---
sidebar_position: 6
---

# follow

Declare a first-level link in the skills source so discovery follows it. This is the one-step way to set up [`.skillfollow`](../skillfollow.md) instead of editing the files by hand.

```bash
skillshare follow _team-skills --to ~/work/team-skills   # Create the link and declare it
skillshare follow _team-skills                           # Declare a link that already exists
skillshare follow _team-skills --local                   # Declare it for this machine only
skillshare follow _team-skills -p                        # Project mode
```

## When to Use

- Keep a working repository where you normally edit it, and have skillshare discover its skills
- Turn a link that `doctor` reports as `undeclared-link` into a followed entry
- Add a machine-local link that teammates do not share (`--local`)

## What It Does

1. With `--to <dir>`, creates the link `<source>/<name>` pointing at `<dir>`: a symlink on macOS/Linux, a junction on Windows. `<dir>` must be an existing directory outside the skills source. If `<source>/<name>` already exists, it must already be a link to the same directory.
2. Without `--to`, `<source>/<name>` must already exist, as a link or as a real directory. A real directory is accepted and reported as `not-link`; it is discovered anyway.
3. Adds `<name>` to `.skillfollow`, or to `.skillfollow.local` with `--local`. Comments, blank lines, and the order of existing lines are kept. A name that is already declared is left alone.
4. When the source is inside a Git work tree, adds the anchored ignore line (for example `/_team-skills`) to the source's `.gitignore`, the same line `doctor` asks for. With `--local` it also adds `/.skillfollow.local`. A line that Git already ignores is not added again.
5. Prints the entry's resulting [state](../skillfollow.md#states) and reason, so a declaration that cannot be followed is visible right away.

`follow` does not sync. Run `skillshare sync` afterwards.

A `_`-prefixed name with `.git` inside is followed as a tracked repository; any other name is followed as a group. `<name>` must be a direct child name: no `/` or `\`, no glob or negation characters, no absolute paths or drive names.

If the link is already tracked by Git, `follow` does not untrack it. It prints the `git -C <source> rm --cached` command to run yourself, which works from any directory. `commit` and `doctor` print the same command without `-C`, to run from the source.

## Options

| Flag | Description |
|------|-------------|
| `<name>` | First-level entry in the skills source |
| `--to <dir>` | Create the link to `<dir>` first (a junction on Windows) |
| `--local` | Write `.skillfollow.local` instead of `.skillfollow`, and ignore it in Git |
| `--project, -p` | Use the project skills source (`.skillshare/skills/`) |
| `--global, -g` | Use the global skills source |
| `--json` | Output as JSON |
| `--help, -h` | Show help |

Mode is auto-detected when neither `-p` nor `-g` is specified (same as other commands).

## Examples

```bash
# Create the link and declare it in a Git source
$ skillshare follow _team-skills --to ~/work/team-skills
✓ _team-skills  linked to /home/me/work/team-skills
✓ _team-skills  added to .skillfollow
✓ .gitignore    added /_team-skills
✓ _team-skills  followed — following directory

Next
  skillshare sync  apply the change

# Already declared
$ skillshare follow _team-skills
! _team-skills  already in .skillfollow
✓ _team-skills  followed — following directory

# A declaration that cannot be followed is reported, not hidden
$ skillshare follow out --to ~/.claude
✓ out         linked to /home/me/.claude
✓ out         added to .skillfollow
✓ .gitignore  added /out
! out         target-overlap — target overlaps active skills target /home/me/.claude/skills

Next
  skillshare sync  apply the change

# The link is already tracked by Git
$ skillshare follow _dev
✓ _dev        added to .skillfollow
✓ .gitignore  added /_dev
! _dev        indexed in Git; run git -C '/home/me/.config/skillshare/skills' rm --cached -- '_dev'
✓ _dev        followed — following directory

Next
  skillshare sync  apply the change
```

A rejected entry stays declared and pauses target cleanup until you fix it or run [`unfollow`](./unfollow.md). See [States and recovery](../skillfollow.md#states).

## JSON Output

```bash
skillshare follow _team-skills --json
```

```json
{
  "name": "_team-skills",
  "source": "/home/me/.config/skillshare/skills",
  "file": ".skillfollow",
  "added": true,
  "link": "/home/me/.config/skillshare/skills/_team-skills",
  "link_created": false,
  "ignore_file": "/home/me/.config/skillshare/skills/.gitignore",
  "ignore_lines_added": ["/_team-skills"],
  "state": "followed",
  "reason": "following directory",
  "resolved_target": "/home/me/work/team-skills"
}
```

`link_target` appears when `--to` was given, and `untrack_command` when the link is indexed. A failure prints `{"error": "..."}` and exits with status 1. An argument error, such as an unknown flag or a missing name, prints plain text even with `--json`.

## See Also

- [unfollow](./unfollow.md) — Stop following an entry
- [.skillfollow](../skillfollow.md) — File format, states, and safety rules
- [doctor](./doctor.md) — Reports declared states and missing ignore lines
- [sync](./sync.md) — Apply the change to targets
