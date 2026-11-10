---
sidebar_position: 8
---

# Using skillshare in Herdr

> A skillshare panel beside your agents: see what needs an update or a sync, and act on it without leaving [Herdr](https://herdr.dev).

![skillshare panel in Herdr](/img/herdr-panel-demo.png)

## Prerequisites

- Herdr 0.1.12 or later
- skillshare installed on each machine Herdr connects to, in `~/.local/bin`, `~/go/bin`, `/opt/homebrew/bin` or `/usr/local/bin`

## Setup

### From Herdr layouts

The layout is listed at [Herdr layouts](https://herdr.dev/layouts/runkids/skillshare--herdr--skillshare/). Paste this prompt into an agent in Herdr:

```text
Read ~/.config/herdr-app/AGENTS.md, then add this layout: herdr.dev/layouts/runkids/skillshare--herdr--skillshare
```

The agent reads every file before it adds the layout, and your own layout stays as it is. Switch layouts with `ctrl-shift-l`.

### By hand

The panel ships in the skillshare repository under [`herdr/skillshare`](https://github.com/runkids/skillshare/tree/main/herdr/skillshare).

1. Copy `mods/skillshare/` to `~/.config/herdr-app/mods/skillshare/`.
2. Copy `layouts/herdr-skillshare.json` and `layouts/herdr-skillshare.png` to `~/.config/herdr-app/layouts/`, then switch to the `herdr-skillshare` layout in Herdr's settings. To keep your own layout instead, add the two blocks listed in the folder's `README.md`.
3. Run `herdr-app check ~/.config/herdr-app/mods/skillshare`.

A **skillshare** button appears in the tab bar. It shows and hides the panel, and the panel stays open or closed as you switch spaces.

## Updating the Panel

While it is shown, the panel asks GitHub once an hour for the latest skillshare release. When a newer one is published, its header shows **Update panel**. It asks first, replaces the files in `~/.config/herdr-app/mods/skillshare/` with the ones that release ships, and Herdr reloads the panel. The files it replaced are kept in that folder's `.bak-update-<version>`; changes you made to them are not carried over. Your layout is not touched.

A panel installed before this button existed has no update check. Copy `mods/skillshare/` from the latest release over `~/.config/herdr-app/mods/skillshare/` once by hand; later updates come from the button.

## What the Panel Does

The panel follows the space Herdr shows. It runs skillshare on that space's machine, in the focused terminal's folder.

- **Overview** answers first: *Everything is in sync*, or how many targets need one, with a single Sync button. Under it, a count of each kind in the source and a line to every target show which ones are waiting. The rest of the tab lists only what needs you: what waits for a sync, what has an update, and hook notices. In a project with `.skillshare/config.yaml`, an All · Project · Global switch beside the tabs picks the scope.
- **Library** lists skills, agents, MCP servers, plugins and hooks, grouped into *Needs sync*, *Update available*, *In sync* and *Off* for disabled entries. For skills and agents, *In sync* groups entries from the same repository into collapsible rows; switch to List for a flat list.
- **Rows** open a preview. A skill shows its `SKILL.md` and files, an MCP server its settings with secret values hidden, and the preview's Targets tab shows the state per target. Skills and agents offer Update, Diff, Uninstall, and *Ask &lt;agent&gt; to audit…*, which sends the item to the focused Herdr agent after you confirm. Top-level declared hooks offer Enable, Disable and Remove, each after confirmation; Sync applies the saved changes.
- Hook counts include named entries and pending cleanup; diagnostic notices get rows of their own. Hook conflicts block syncing all resource kinds until resolved in the CLI or dashboard. If the plan cannot be read, entries show unknown sync status outside the groups.
- **Install** takes a repository URL. A repository with several skills shows a pick list. The panel clones it, runs [`skillshare audit`](/docs/reference/commands/audit) on the clone, and installs only when the audit passes. A blocked audit shows the findings, with *Install anyway* (`--force`) and *Ask &lt;agent&gt; to review…*.
- **Source** shows your source repository's state. Its dialog offers Pull, Commit and push, and Sync both ways. When a pull stops on files both machines changed, you pick a version per file from a side-by-side preview. `.metadata.json` merges per entry, the way [`skillshare pull`](/docs/reference/commands/pull) does.
- **Machines** shows each Herdr machine's skillshare version, its remote, source and targets, and doctor warnings, with Pull and sync and, when a newer release exists, Update. A machine with uncommitted or unpushed changes offers Sync both ways instead, because a plain pull would refuse.

The panel refreshes every 20 seconds while it is shown, runs `skillshare check` every 10 minutes, and does nothing while hidden.

## Related

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push) and [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
