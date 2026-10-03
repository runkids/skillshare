---
sidebar_position: 1
---

# target

Manage sync targets (AI CLI skill directories).

```bash
skillshare target add <name> <path>    # Add a target
skillshare target remove <name>        # Remove a target
skillshare target list                 # List all targets
skillshare target <name>               # Show target info
skillshare target <name> --mode merge  # Change sync mode
skillshare target <name> --target-naming standard  # Change naming
skillshare target <name> --skills=false    # Stop syncing skills
```

## When to Use

- Add a new AI CLI target after installing a new tool
- Remove a target you no longer use
- Change sync mode (merge, copy, or symlink) for a target
- Change target naming (flat or standard) for a target
- Tune compatibility target-by-target instead of forcing one global mode
- Set up include/exclude filters for selective skill syncing
- Stop syncing skills to a tool that already reads another target's folder, while keeping its agents, MCP servers and instructions managed

## Subcommands

### target add

Add a new target for skill synchronization.

```bash
skillshare target add windsurf ~/.windsurf/skills
```

The command validates:
- Path exists or parent directory exists
- Path looks like a skills directory
- Target name is unique

Add `--no-skills` to add a target without syncing skills to it. Its agents, MCP servers and instructions are still managed, and the skills folder does not need to exist:

```bash
skillshare target add gemini ~/.gemini/skills --no-skills
# Added target: gemini -> ~/.gemini/skills (skills off)
```

See [Skills on or off](#skills-off).

#### Another account of an Agent {#another-account}

If you run a second account of an Agent from its own config directory, such as Claude Code with `CLAUDE_CONFIG_DIR=~/.claude-work`, Codex with `CODEX_HOME` or Pi with `PI_CODING_AGENT_DIR`, add that directory as a target. Skillshare works out the skills and agents paths from it:

```bash
skillshare target add claude-work --agent claude --config-dir ~/.claude-work
# Added target: claude-work -> ~/.claude-work/skills
```

Add as many accounts as you have, each under its own name. The name also works as an [MCP target](./mcp.md#accounts), so one sync reaches the skills, the agents and the MCP servers of every account.

If the account runs a compatible CLI, such as omo for Pi, `--cli` makes its [plugin commands](./plugin.md#accounts) use that CLI:

```bash
skillshare target add omo --agent pi --config-dir ~/.omo/agent --cli omo
```

`--agent` accepts `claude` (`CLAUDE_CONFIG_DIR`), `codex` (`CODEX_HOME`) and `pi` (`PI_CODING_AGENT_DIR`). A Codex or Pi account syncs its skills to `<config_dir>/skills`; only Claude also has an agents directory. The directory must be absolute or start with `~`, must not be the Agent's default one, and cannot be shared by two targets.

Removing such a target never fails because of MCP: if `mcp.targets` or a server's `targets` still names it, `skillshare target remove` removes the target and warns you to take the name out there too.

### target remove

Remove a target and restore its skills to regular directories.

```bash
skillshare target remove cursor           # Remove single target
skillshare target remove --all            # Remove all targets
skillshare target remove cursor --dry-run # Preview
```

**What happens:**
1. Creates backup of target
2. Detects sync mode:
   - **Symlink mode:** Removes the directory symlink, copies source contents back as a real directory
   - **Merge mode:** Removes only symlinks pointing to source (by path prefix), copies each skill back as real files. Local (non-symlink) skills are preserved.
   - **Copy mode:** Removes `.skillshare-manifest.json`. Managed copies and local skills are preserved as regular directories.
3. Removes target from config

If another target writes to the same skills folder, such as `codex` and `universal` in `~/.agents/skills`, step 2 is skipped: the skills stay linked for that target, and only the removed one leaves the config.

A target with [skills off](#skills-off) has nothing synced, so step 2 is skipped for it too and its folder is left as is.

### target list

List all configured targets.

```bash
skillshare target list                 # Interactive TUI (default on TTY)
skillshare target list --no-tui        # Plain text output
skillshare target list --json          # JSON output for CI/scripts
```

#### Interactive TUI

On a TTY, `target list` opens an interactive view: targets on the left, and the selected target's paths, mode and filters on the right. From there you can change a target's sync mode, naming, and include/exclude filters, or remove it (backed up and unlinked, same as `target remove`). The keys are listed at the bottom of the screen.

Changes are saved to config immediately. Run `skillshare sync` to apply them.

Use `--no-tui` to skip the TUI and print plain text instead:

```
claude
  Skills    ~/.claude/skills  merge · flat · merged · 43 shared
  Agents    ~/.claude/agents  merge · 2/2 linked

cursor
  Skills    ~/.cursor/skills  merge · flat · merged · 43 shared, 1 local
  Agents    ~/.cursor/agents  merge · 2/2 linked

codex
  Skills    ~/.openai-codex/skills  symlink · flat · linked

3 targets
```

#### JSON Output

```bash
skillshare target list --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "targetNaming": "flat",
      "include": [],
      "exclude": [],
      "skillsEnabled": true
    },
    {
      "name": "cursor",
      "path": "~/.cursor/skills",
      "mode": "merge",
      "targetNaming": "standard",
      "include": [],
      "exclude": [],
      "skillsEnabled": true
    }
  ]
}
```

### target info / settings

Show target details or change settings.

```bash
# Show info
skillshare target claude

# Change mode
skillshare target claude --mode symlink
skillshare target claude --mode merge

# Change target naming
skillshare target claude --target-naming standard
skillshare target claude --target-naming flat

skillshare sync  # Apply changes
```

## Sync Modes

| Mode | Behavior |
|------|----------|
| `merge` | Each skill symlinked individually. Preserves local skills. **Default.** |
| `copy` | Each skill copied as real files. For AI CLIs that can't follow symlinks. |
| `symlink` | Entire directory is one symlink. Exact copies everywhere. |

`target --mode` is the main compatibility control surface. Keep your global default simple, then override only where needed.

## Target Naming

| Naming | Behavior |
|--------|----------|
| `flat` | Nested skills flattened with `__` separators (e.g. `frontend__dev`). **Default.** |
| `standard` | Uses SKILL.md `name` field directly (e.g. `dev`). Follows the [Agent Skills spec](https://agentskills.io/specification). |

`target --target-naming` controls how skill directories are named in targets. In `standard` mode, skills with invalid or colliding names are warned and skipped. Ignored in symlink mode.

```bash
# Set target to copy mode (for Cursor, Copilot CLI, etc.)
skillshare target cursor --mode copy
skillshare sync  # Apply the change
```

### Mixed strategy example

```bash
# Keep default merge behavior for most targets
skillshare target claude --mode merge

# Compatibility-first for one target
skillshare target cursor --mode copy

# Exact mirror for another target
skillshare target codex --mode symlink

skillshare sync
```

## Target Filters (include/exclude) {#target-filters-includeexclude}

Manage per-target include/exclude filters for both skills and agents from the CLI:

```bash
# Skills
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare target claude --remove-exclude "_legacy*"

# Agents
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
skillshare target claude --remove-agent-exclude "draft-*"
```

After changing filters, run `skillshare sync` to apply.

Filters work in **merge and copy modes**. Patterns use Go `filepath.Match` syntax (`*`, `?`, `[...]`). In symlink mode, filters are ignored.

Agent filters are only available for targets that have an agents path, either from a built-in target definition or an explicit `agents.path` override in config.

See [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters) for pattern cheat sheet and scenarios.

:::tip
Target filters are one of three filtering layers. See [Filtering Reference](/docs/reference/filtering) for how they interact with `.skillignore` and SKILL.md `targets`.
:::

## Skills On or Off {#skills-off}

Some tools read skills from another target's folder as well as their own. Pi, for example, reads `~/.pi/agent/skills` and also `~/.agents/skills`, the folder of the `universal` target. Syncing skills into both makes Pi find each skill twice: Pi keeps the first and warns about the other, and some tools list both. Turn skills off for that target, and skillshare keeps managing its agents, MCP servers and instructions but leaves its skills folder alone:

```bash
skillshare target pi --skills=false --dry-run   # Preview
skillshare target pi --skills=false
```

```
✓ Removed   2 links  alpha, beta
  Kept      1 local skill  my-notes

✓ Skills off for pi
  Agents, MCP servers and instructions are still managed
```

Turning skills off saves `skills.enabled: false` in the config, then cleans the folder:

- **Merge mode:** removes the links into the source. Your own skills stay.
- **Symlink mode:** removes the folder's link to the source, never what it points to.
- **Copy mode:** keeps the copies, since they are real folders you may have edited, and lists them apart. The tool still loads them, so if it reads the same skills from another folder, delete the copies yourself:

  ```
  ! Kept      2 copied skills  alpha, beta

  ✓ Skills off for pi
    The tool still loads these copies; delete them if it reads the same skills elsewhere
    Agents, MCP servers and instructions are still managed
  ```

- **Shared folder:** if an enabled target writes to the same folder, nothing is removed.

From then on, `sync`, `diff`, `status` and `doctor` skip the target's skills; `status` and `sync` show it as `skills off`. Turn skills back on with `--skills=true`; the next `skillshare sync` syncs them again.

`--skills` cannot be combined with include/exclude flags in one command; run them separately. It works the same in project mode (`-p`).

In the web dashboard, use **Stop syncing skills** on the target's Skills tab. Before anything is removed, it lists what goes and what stays, and warns when other tools read the same folder.

## Options

### target add

| Flag | Description |
|------|-------------|
| `--agent <agent>` | Add [another account](#another-account) of this Agent instead of a path. Goes with `--config-dir` |
| `--config-dir <dir>` | The config directory that account uses |
| `--cli <executable>` | Runs that account's plugin commands with this compatible CLI instead of the Agent's. A name on `PATH` or an absolute path |
| `--no-skills` | Add the target with [skills off](#skills-off) |

### target remove

| Flag | Description |
|------|-------------|
| `--all, -a` | Remove all targets |
| `--dry-run, -n` | Preview without making changes |

### target list

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--no-tui` | Disable interactive TUI, use plain text output |

### target info / settings

| Flag | Description |
|------|-------------|
| `--mode, -m <mode>` | Set sync mode (merge, copy, or symlink) |
| `--agent-mode <mode>` | Set agents sync mode (merge, copy, or symlink) |
| `--target-naming <naming>` | Set target naming (flat or standard) |
| `--skills <true\|false>` | Turn skills sync [on or off](#skills-off); also `--skills=false` |
| `--dry-run, -n` | With `--skills=false`, preview what would be removed |
| `--add-include <pattern>` | Add an include filter pattern |
| `--add-exclude <pattern>` | Add an exclude filter pattern |
| `--remove-include <pattern>` | Remove an include filter pattern |
| `--remove-exclude <pattern>` | Remove an exclude filter pattern |
| `--add-agent-include <pattern>` | Add an agent include filter pattern |
| `--add-agent-exclude <pattern>` | Add an agent exclude filter pattern |
| `--remove-agent-include <pattern>` | Remove an agent include filter pattern |
| `--remove-agent-exclude <pattern>` | Remove an agent exclude filter pattern |

## Supported AI CLIs

skillshare auto-detects these during `init`:

| CLI | Default Path |
|-----|-------------|
| Claude Code | `~/.claude/skills` |
| Cursor | `~/.cursor/skills` |
| OpenCode | `~/.opencode/skills` |
| Windsurf | `~/.windsurf/skills` |
| Codex | `~/.openai-codex/skills` |
| Antigravity (app) | `~/.gemini/config/skills` |
| Antigravity CLI | `~/.gemini/antigravity-cli/skills` |
| Gemini CLI | `~/.gemini/skills` |
| Amp | `~/.amp/skills` |
| ... and 45+ more | See [supported targets](/docs/reference/targets/supported-targets) |

## Examples

```bash
# Add custom target
skillshare target add my-tool ~/my-tool/skills

# Check target status
skillshare target claude

# Switch to copy mode (for AI CLIs that can't read symlinks)
skillshare target cursor --mode copy
skillshare sync

# Switch to symlink mode
skillshare target claude --mode symlink
skillshare sync

# Configure agent sync mode
skillshare target claude --agent-mode copy
skillshare sync

# Add/remove skill filters
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare sync

# Add/remove agent filters
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare sync

# Remove target (restores skills)
skillshare target remove cursor
```

## Project Mode

Manage targets for the current project:

```bash
skillshare target add windsurf -p                                # Add known target
skillshare target add custom ./tools/ai/skills -p                # Add custom path
skillshare target remove cursor -p                                # Remove target
skillshare target list -p                                         # List project targets
skillshare target claude -p                                  # Show target info
skillshare target claude --add-include "team-*" -p          # Add filter
skillshare target claude --add-agent-include "team-*" -p    # Add agent filter
```

### How It Differs

| | Global | Project (`-p`) |
|---|---|---|
| Config | `~/.config/skillshare/config.yaml` | `.skillshare/config.yaml` |
| Paths | Absolute (e.g., `~/.claude/skills`) | Relative or absolute (e.g., `.claude/skills`) |
| Sync mode | Merge, copy, or symlink | Merge, copy, or symlink (default merge) |
| Mode change | `--mode` flag | `--mode` flag |

### Project Target List Example

```
claude
  Skills    .claude/skills  merge · flat · merged · 3 shared

cursor
  Skills    .cursor/skills  merge · flat · merged · 3 shared

custom-tool
  Skills    ./tools/ai/skills  merge · flat · merged · 3 shared

3 targets
```

Targets in project mode support:
- **Known target names** (e.g., `claude`, `cursor`) — resolved to project-local paths
- **Custom paths** — relative to project root or absolute with `~` expansion

## See Also

- [sync](/docs/reference/commands/sync) — Sync skills to targets
- [status](/docs/reference/commands/status) — Show target status
- [Targets](/docs/reference/targets) — Target management guide
- [Project Skills](/docs/understand/project-skills) — Project mode concepts
