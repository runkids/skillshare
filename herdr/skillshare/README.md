# skillshare for Herdr

![The skillshare panel in Herdr: what needs an update or a sync, hooks for every agent, and every machine](panels.png)

[skillshare](../../README.md) keeps your skills, agents, MCP servers, plugins and hooks in one
source you control and syncs them to Claude Code, Codex, Gemini CLI and the other AI coding tools
you use. This panel puts it beside your agents in [Herdr](https://herdr.dev):

- **See what is out of date.** The focused space's project and your global source, grouped into
  Updates available, Needs sync and In sync.
- **Fix it without leaving Herdr.** Update, sync, uninstall, or install from a URL; every install
  is audited first, and a blocked one shows its findings before anything is written.
- **Ask your agent.** Hand an audit of any skill to the focused agent.
- **Every machine.** The Machines tab shows each Herdr machine's version, source state and drift,
  with Pull and sync.
- **The source repository.** Pull, push or sync both ways, and pick a version per file when both
  sides changed it.

New to skillshare? Start with the [main README](../../README.md) or the
[docs](https://skillshare.runkids.cc/docs/learn/with-herdr). If the panel saves you time, a star on
[runkids/skillshare](https://github.com/runkids/skillshare) helps other people find it.

![Herdr with the skillshare panel beside a Claude Code session](preview.png)

## Install

From [Herdr layouts](https://herdr.dev/layouts/runkids/skillshare--herdr--skillshare/), paste this prompt into an agent in Herdr:

```text
Read ~/.config/herdr-app/AGENTS.md, then add this layout: herdr.dev/layouts/runkids/skillshare--herdr--skillshare
```

By hand:

1. Copy `mods/skillshare/` to `~/.config/herdr-app/mods/skillshare/`.
2. Copy `layouts/herdr-skillshare.json` and `layouts/herdr-skillshare.png` to
   `~/.config/herdr-app/layouts/` and switch to it (`layout.switch {name: "herdr-skillshare"}`).
   `RECIPE.md` describes it in full and `preview.png` shows it. Or, in your own layout, add:
   - a toggle block to the tab bar row:
     `{ "id": "skillshare-toggle", "type": "block", "content": { "mod": "skillshare", "props": { "role": "toggle" } }, "size": { "px": 118 } }`
   - a column beside the work area:
     `{ "id": "skillshare-panel", "type": "column", "children": [{ "id": "skillshare", "type": "block", "content": { "mod": "skillshare" } }], "size": { "px": 400 }, "min": 320, "max": 720 }`
3. Run `herdr-app check ~/.config/herdr-app/mods/skillshare`.

The panel lives in its own layout node instead of the per-space `panel` stack. Herdr's tabs mod
restores that stack per space, so a panel there closes when you switch spaces. This node's
shown or hidden state is a layout click, which Herdr keeps across spaces and restarts.

## How it reads

- It runs on the shown space's machine (`exec` with `on`), in the focused terminal's folder.
- A folder with `.skillshare/config.yaml` or `skillshare/config.yaml` is a project; both
  scopes show and the All · Project · Global filter appears.
- Per scope, one detached script runs `list --all`, `diff`, `mcp list`, `plugin list` and `hooks list`
  with `--json`. `check` and `check agents` run every 10 minutes. Git state comes from `git status` in the
  source repository, because the CLI has no JSON for it.
- The CLI's install dry-run does not list a repository's skills, so the install dialog
  shallow-clones the repository to a temp folder to offer the pick list. It then runs
  `skillshare audit <path> --json` on that clone, because `install --json` names a blocked
  skill but not its findings. A block shows the findings before anything is written;
  "Install anyway" reruns the install with `--force`.
- The source dialog runs `skillshare pull` and `skillshare push -m`. When a pull or
  `push --pull` stops on files both sides changed, the panel reads both versions with
  `git show HEAD:<file>` and `git show @{u}:<file>`, puts the repository back, and asks
  which version to keep for each file.

Hook rows show their agents and planned sync changes. Top-level declared hooks offer confirmed
Enable, Disable and Remove actions; Sync applies them through `sync --all`.
Counts and groups include named hook entries and pending named cleanup, excluding diagnostic
notices. Notices follow the scope, kind and text filters and remain visible with zero entries.
Nested hook rows show their full root to distinguish projects with the same folder name.
“In sync” describes native file alignment; inactive hooks retain a warning dot and reason.
Warnings and preview failures appear above the resource groups. When preview fails, named
entries stay visible there as “sync status unknown”; they do not count as pending or in sync.
Disabled state remains visible alongside diagnostics, and pending summaries omit unchanged
targets while expanded details retain them. When only an unnamed native
include needs work, “Native hook files need sync” offers Sync for its scope.

Disabled rows are hidden by default; “In sync” offers Show disabled and Hide disabled when
there are any. The choice is kept across restarts, and a disabled hook with pending changes
stays listed. For skills and agents, “In sync” also switches between List and By source
(the default), which folds rows from the same repository, or local ones, into collapsible
groups. A source with one row stays a plain row, and a filter opens every group.
Hook conflicts block `sync --all` for every resource kind; resolve them with the CLI or
dashboard, then Sync again.

## Tests

`make test-herdr` (`node --test herdr/tests/*.test.mjs`) covers `model.js`, `jobs.js`,
`channel.js`, `operations.js` and `loader.js`, which have no Herdr imports. It runs in `make check` and CI. Check `main.js` with `herdr-app check`.
