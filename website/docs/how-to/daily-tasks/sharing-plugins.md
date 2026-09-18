---
sidebar_position: 9
---

# Manage plugins across tools

A plugin can include skills, MCP connections, hooks, scripts, and other files
that work together. Skillshare keeps that package intact and lets you select
which tools receive it. You do not need to write YAML to get started.

## Add your first plugin

In the dashboard, open **Plugins → Add plugin**:

1. Paste a GitHub repository (`owner/repo`), HTTPS Git URL, or local directory.
2. Choose a plugin if the source contains several, then select compatible tools.
3. Review the changes and apply them.

Most users only need a repository and target checkboxes. **Advanced options** adds
a Git ref, to choose a release. Discovery shows each target's components and
compatibility separately. When OpenCode is listed as unsupported because its entry
could not be detected, **Set entry path** on that row takes the built file and
searches the source again, keeping what you already chose. Safe relative repository
symlinks are preserved.

The same guided flow is available in a terminal:

```bash
skillshare plugin add
```

Claude Code, Codex, Copilot, Antigravity CLI, Grok, Pi, or OpenCode CLI must be installed **where the Skillshare backend runs**.
Cursor and Antigravity desktop instead receive complete files in their local plugin directories.
A dashboard running inside a container cannot manage plugins installed only on
the host machine. Use the local CLI, or run Skillshare beside the native clients.

Install targets are **Claude Code, Codex, Cursor, Antigravity Desktop, Antigravity CLI,
GitHub Copilot CLI, Pi, and OpenCode**. Grok requires native installation and trust
before import. Kimi, Hermes, and Devin formats are discoverable; their automatic
installation is unavailable and the interface explains why.
Choose the format published for your target; Skillshare does not translate plugins
between tools. Project mode supports Claude, Antigravity Desktop, Pi, and OpenCode.
Antigravity Desktop (`antigravity` or `agy`) and CLI (`antigravity-cli`) use separate
stores. Choose the one you run.

## Already installed something?

Choose **Import installed** and select a native installation. Importing records
it without reinstalling, copying its authentication, or changing whether it is
enabled in its Agent. Import is available for Claude, Codex, Copilot, Antigravity CLI, Grok, Pi, and OpenCode;
use **Add plugin** for Cursor and Antigravity local packages.

```bash
skillshare plugin import review@team --from claude --no-tui
```

If a logical package uses different native distributions for different tools,
add/import each distribution using the same `--name` and the appropriate target.
Skillshare does not infer equivalence from display names.

## Choose where to sync

Each managed binding has a checkbox. The checkbox means **include this target in
sync**, not “enable inside the Agent.”

- Check it, then sync to install a missing plugin.
- Opening a plugin's row also lists, unticked, the other Agents its source has a package
  for. Ticking one opens the install preview. Agents the source cannot serve are counted
  at the end of the row, and that count opens the reasons.
- Uncheck it, then sync to remove that managed installation.
- The package definition remains, so you can select the target again later.
- A plugin disabled inside Claude or Codex stays disabled; manage native settings
  in that tool.

In the dashboard, the **Sync** box at the top right of the Plugins page lists what
the next sync would install or remove for each Agent. Its button opens a preview;
nothing changes in an Agent until you confirm it. The plugin list appears at once,
while the **Agents** column below the box fills in as each Agent's CLI answers.

```bash
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run
skillshare sync plugins --no-tui
```

A plugin's menu has **View files**: the reviewed local copy of its source, read only,
with rendered Markdown. An imported plugin has no local copy, so it has no such entry.

Plugins are separate from ordinary skills and MCP synchronization. Their bundled
components are not also copied into standalone Skillshare sources.

## Updates and recovery

Use **Check updates**, then review an update for a supported target. Claude can
update through its native CLI. Codex update is explicitly unavailable in this
adapter; updating a marketplace is not equivalent to updating an installed plugin.
Cursor and Antigravity replace managed local copies after checking for local edits.
Pi and OpenCode update the reviewed snapshot. Copilot can refresh a reviewed
source while preserving known enabled state. Antigravity CLI and Grok updates
stay in the native tool; see the command reference for imported-package limits.

If one target fails, the result keeps the successful outcomes. Resolve the native
client's authentication or configuration issue, then sync that target again:

```bash
skillshare sync plugins review --target claude --no-tui
```

Snapshots are owned by Skillshare. If their content has been edited externally,
Skillshare blocks replacement so you can preserve those edits first. Source
digests detect changes; they are not a guarantee that every native installation
or imported marketplace is reproducible across machines.

For automation, scope details and all flags, see [plugin](/docs/reference/commands/plugin).
