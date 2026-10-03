# Extras

Manage non-skill resources (rules, commands, prompts) that sync to arbitrary directories.

| Command | What it does | Project? | `--json`? |
|---------|-------------|:--------:|:---------:|
| `extras init <name>` | Create a new extra | ✓ (auto) | ✗ |
| `extras memory <command>` | Manage shared Markdown notes | ✓ (auto) | ✓ |
| `extras list` | List all extras + sync status | ✓ (auto) | ✓ |
| `extras remove <name>` | Remove an extra from config | ✓ (auto) | ✗ |
| `extras collect <name>` | Collect target files into source | ✓ (auto) | ✗ |
| `sync extras` | Sync all extras to targets | ✓ (auto) | ✓ |
| `diff` | Includes extras diff automatically | ✓ (auto) | ✓ |

**Source directories:**
- Global: `~/.config/skillshare/extras/<name>/`; `extras_source` and per-extra `source` can override it.
- Project: `.skillshare/extras/<name>/`; per-extra `source` is relative to the project root. Top-level `sources.extras` moves the default project extras parent.

## extras init

Create a new extra resource type. Without arguments, asks for each setting interactively.

```bash
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules
skillshare extras init commands --target ~/.claude/commands --mode copy
skillshare extras init prompts --target .claude/prompts -p
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md --source ~/dotfiles/prompts --target ~/.pi/agent
skillshare extras init                    # Interactive prompts
skillshare extras init rules --no-tui ... # Skip prompts
```

| Flag | Description |
|------|-------------|
| `--target <path>` | Target directory (repeatable, at least one required) |
| `--source <path>` | Custom source directory for this extra (relative to the project root in project mode) |
| `--file <filename>` | Single-file extra: sync only this file from the source directory |
| `--as <filename>` | File name at every target (default: `--file` name); requires `--file` |
| `--mode <mode>` | Sync mode: `merge` (default), `copy`, `symlink`; `import` only with `--file` |
| `--flatten` | Sync subdirectory files into the target root; not with `symlink` or `--file` |
| `--no-tui` | Skip interactive prompts |
| `-p` / `-g` | Force project / global mode |

`extras init` writes config only: it does not create the source file or sync. The interactive prompts ask Folder or Single file after the name.

## Existing extras

```bash
skillshare extras <name> --help
skillshare extras personal --add-target ~/.claude --as CLAUDE.md --mode import
```

`--as` sets the target filename for a single-file extra; it defaults to `file`.
Run `skillshare sync extras` after adding the target.

## extras list

Show all configured extras with sync status per target.

```bash
skillshare extras list
skillshare extras list --json
skillshare extras list -p
```

Statuses: `synced`, `drift`, `modified` (a single-file symlink or merge target replaced by a different real file), `not synced`, `no source`.

JSON output returns an array of:
```json
[{
  "name": "rules",
  "source_dir": "~/.config/skillshare/extras/rules",
  "file_count": 3,
  "source_exists": true,
  "targets": [{"path": "~/.claude/rules", "mode": "merge", "status": "synced"}]
}]
```

## extras remove

Remove an extra from config. Source files are preserved.

```bash
skillshare extras remove rules
skillshare extras remove rules --force    # Skip confirmation
skillshare extras remove prompts -p
```

| Flag | Description |
|------|-------------|
| `--force` / `-f` | Skip y/N confirmation prompt |
| `-p` / `-g` | Force project / global mode |

After removal, run `sync extras` to clean up orphaned links. A single-file extra (below) needs no cleanup: remove restores each target file before removing its config entry. If restoration fails, the entry is kept for retry.

## extras collect

Collect local (non-symlinked) files from a target back into the extras source directory. Merge-mode targets get symlinks in place of collected files; copy-mode targets keep their files. Files already in source are skipped unless `--force`.

```bash
skillshare extras collect rules
skillshare extras collect rules --from ~/.claude/rules --dry-run
skillshare extras collect rules --force   # overwrite source with target edits
skillshare extras collect prompts -p
```

| Flag | Description |
|------|-------------|
| `--from <path>` | Target to collect from (required if multiple targets) |
| `--force` / `-f` | Overwrite files that already exist in source |
| `--dry-run` | Preview without changes |
| `-p` / `-g` | Force project / global mode |

## sync extras

Distribute extra files from source to all configured targets.

```bash
skillshare sync extras              # Sync all extras
skillshare sync extras --dry-run    # Preview
skillshare sync extras --force      # Overwrite conflicts
skillshare sync extras --json       # JSON output
skillshare sync --all               # Skills + agents + extras + MCP
```

Sync modes (per-target):
- `merge` (default): per-file symlinks, preserves local files in target
- `copy`: real-file copies
- `symlink`: entire directory symlinked

`--json` returns a non-zero exit status when extras sync has errors. For single-file extras, `--dry-run` also reports edits that would be backed up before replacement.

## diff (extras included)

`skillshare diff` automatically includes extras when configured — no extra flags needed.

```bash
skillshare diff                     # Skills + extras (if configured)
skillshare diff --json              # JSON output includes extras
```

## Config format

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: .claude/agents
      - path: .codex/agents
        flatten: true
        extension: codex-agents   # transform + rename via .skillshare/extensions/codex-agents/
  - name: commands
    targets:
      - path: ~/.claude/commands
        mode: symlink
```

A single-file extra syncs one file instead of the directory. `as` renames it per
target; `import` mode (single-file only) keeps an `@<source file>` line in a
managed block of the target file instead of replacing it:

```yaml
extras:
  - name: personal
    file: AGENTS.md          # extras/personal/AGENTS.md
    targets:
      - path: ~/.codex       # ~/.codex/AGENTS.md -> symlink
      - path: ~/.claude
        as: CLAUDE.md
        mode: import         # CLAUDE.md keeps its content, imports the file
```

The first sync records the target's attach-time state as its restore point
(a file, a symlink, or no file), then replaces it. `extras remove` and
`extras <name> --remove-target <path> --prune` put that state back; in `import`
mode they only drop the managed line when the file has other content. Later edits
that sync, overwrite, or restore replace are kept as drift backups in
`~/.local/state/skillshare/extras/backups/<id>/drift/` and are never restored.
When replacing a user junction for a single-file extra, the warning includes its original destination; restore recreates the junction.

`--remove-target` without `--prune` leaves the single-file target in place and unmanaged, and forgets its restore point. Later syncs do not clean it up; attaching it again records a new restore point.
`flatten` and `extension` are rejected on a single-file extra; `extras collect`
does not apply. `extras list` shows full file paths for its source and targets.
To sync several files from one folder, create one single-file extra per file with
the same `--source` (in project mode, relative to the project root, e.g.
`.skillshare/extras/prompts`); files no extra names are not synced.
The web dashboard lists single-file extras whose `file` is `AGENTS.md` under
Extras -> AGENTS.md as shared AGENTS.md files, and all others under Extras ->
Folders & files; its rename conversion (project mode) is blocked while the file
uses a shared AGENTS.md.

The `extension:` field names an extension directory under `.skillshare/extensions/` (project) or `~/.config/skillshare/extensions/` (global). It transforms each source file during sync and implies `copy` mode.

For project agents, prefer native target `agents:` config; it also accepts `extension:` (e.g. `agents: { extension: opencode-agents }`, implies `copy`). Use `extras: agents` only when you need extras-only behavior like `flatten`.

## Typical workflow

```bash
# 1. Create an extra
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 2. Add files to source
cp my-rule.md ~/.config/skillshare/extras/rules/

# 3. Sync to targets
skillshare sync extras

# 4. Verify
skillshare extras list
skillshare diff --no-tui

# 5. Or collect existing local files first
skillshare extras collect rules --from ~/.claude/rules
skillshare sync extras
```

## Extensions

An extension transforms each source file before writing it to a target. Set `extension: <name>` on any extras target; implies `copy` mode.

### Directory layout

```
.skillshare/extensions/<name>/
├── extension.yaml   ← required
├── convert.js       ← transformer (or convert.py, etc.)
└── helper.js        ← optional shared utilities
```

### extension.yaml

```yaml
run: ["node", "convert.js"]   # command array — run from extension directory
output_ext: toml              # renames output file (e.g. rule.md → rule.toml)
description: "MD → Codex TOML"
```

- `output_ext` is the only way to change the output file extension
- Omit to keep the source extension
- Global extensions: `~/.config/skillshare/extensions/<name>/`

### I/O contract

- Input: raw source file piped to `stdin`
- Output: transformed content on `stdout`; non-zero exit skips the file with a warning
- `SS_REL_PATH` env var: source file path relative to extras root

### Official extensions

Ready-to-copy reference implementations at `https://github.com/runkids/skillshare/tree/main/extensions`:

| Extension | Converts | Output |
|-----------|----------|--------|
| `codex-agents` | Claude agent MD (frontmatter + body) | Codex TOML (`name`, `description`, `developer_instructions`) |
| `gemini-commands` | Markdown command docs | Gemini CLI TOML commands |

The `codex-agents` extension requires `name` and `description` frontmatter — files missing either field are skipped with an error. Non-agent files (e.g. prompts, changelogs) should be excluded from the extras source using the applicable extras filters; a filename prefix alone is not an ignore rule.

### Caveats

- Native agents targets (`agents: { path: ... }`) do **not** support `extension:` — extras only
- If a Node.js extension fails because inherited `NODE_OPTIONS` references an unavailable
  preload module, clear that variable for the extension process: `run: ["env", "-u", "NODE_OPTIONS", "node", "convert.js"]`

## Shared memory notes

`skillshare extras memory init -g` registers a source-only `memory` folder and
creates missing `INDEX.md` and `LEARNED.md` templates, preserving existing files. Use `-p` for project
notes. Native automatic memory is separate.

```bash
skillshare extras memory list --search decisions --json -g
skillshare extras memory show decisions.md --json -g
skillshare extras memory write decisions.md --from ./note.md -g
skillshare extras memory instructions -g
```

A new write omits `--version`; updates require the hash from `show --json`.
Stale versions are rejected and changed notes are backed up. `--from -` reads
stdin. Delete with `extras memory delete <note.md> --version <hash>` using the
version from `show --json`; deletion backs up the saved file and rejects stale
versions. Recover with `backup files show <absolute-path>` and
`backup files restore <absolute-path> <id>`. Update index links yourself.
Writes accept relative `.md` paths only, UTF-8 up to 1 MiB; hidden files and nested
links are excluded. Paths such as `wiki/architecture.md` create missing folders;
listing and search include notes in subfolders. CLI users maintain `INDEX.md` links themselves.
`LEARNED.md` provides date,
context, conclusion, and evidence fields for durable lessons; updates require the
user's request. Templates do not enable automatic learning.

The dashboard's Extras → Memory tab edits the same source. Use **Connect to
agents**, select tools, **Review changes**, then **Apply changes**. It appends or
updates a scope/hash-marked block in the existing instruction file or shared
source without changing other content, assignments, or connection modes.
Existing files are backed up and a stale review must be repeated. Intact outdated
blocks can be updated after review; modified or malformed blocks are preserved
for manual repair. Unsynced or unreadable instructions are skipped.
**Copy guidance** is the manual fallback; **Open AGENTS.md** edits instructions.
CLI `instructions` only prints the same block. Project sources inside the repo
are relative to the project root, regardless of the instruction file's location;
external overrides and global sources use absolute paths.

A configured tool has the current guidance; this does not prove a read. Use a
fresh session and **Copy verification prompt** to request `INDEX.md` and a relevant
note, its full path, and a temporary verification value you added. Inspect the
actual file read event manually. There is no guaranteed read telemetry.

**New note** offers **Link from INDEX.md**, default checked with a readable index.
It appends a link at EOF with a version check and backup; a failed link leaves
the note created. **Add to INDEX** links an unindexed note. Broken links warn but
are not removed automatically. CLI writes do not add links.

A stale editor save keeps the draft and displays the latest saved content for
comparison. Confirm **Save my draft** to replace it using the refreshed version
and a backup. **History** and the post-delete restore link open **Backup Files**
filtered to the absolute note path. Unsupported notes remain listed without
blocking valid notes.

**Move or rename** accepts a new relative Markdown path, creates missing folders,
and preserves content and permissions. It requires the last-read version, rejects
existing destinations, and backs up the source before removal. Markdown links and
path-keyed backup history are not rewritten; keep the root `INDEX.md` in place
because reading guidance references it. CLI commands and flags are unchanged.
Automatic learning, native automatic memory, and Obsidian integration are not
implemented.
