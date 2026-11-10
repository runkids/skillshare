# Herdr + skillshare

skillshare beside your agents. Herdr's default window gains a skillshare button in the tab bar and a panel beside the work area that shows which of your skills, agents, MCP servers, plugins and hooks need an update or a sync, in the focused space's project and in your global source. From the panel you update, sync, install from a URL with an audit first, enable or disable a skill or agent, uninstall, ask the focused agent to audit something, sync the source repository both ways, and check skillshare on every Herdr machine.

![Herdr + skillshare](preview.png)

skillshare keeps one source of skills, agents, MCP servers, plugins and hooks and syncs it to Claude Code, Codex, Gemini CLI and the other AI coding tools you use. New to it? Start with the [main README](../../README.md); this folder's [README](README.md) covers installing the panel.

## What you see

Left to right, everything as in Herdr's `main` layout, plus two blocks:

- **Sidebar**, 268px: Herdr's own brand bar, Spaces, Agents and footer.
- **Work column**: the tab bar row, now `tabs` (weight 1) beside a 118px `skillshare-toggle` block. The toggle draws the 16px skillshare mark and "skillshare", on `selected` while the panel shows. Under it the shown tab and Herdr's Files/Changes side panel, then the status bar.
- **skillshare panel**, a 400px column (380–720px, the narrowest the tab row fits) right of the work area, top to bottom:
  - a 44px header: the mark, "skillshare", a pill with the folder (or "global" outside a project) and the machine, which shortens with an ellipsis when the row runs out of room, an "Update panel" button when a newer skillshare release is published and nothing is running, the CLI version in `muted`, and a close button;
  - Overview, Library and Machines tabs, the shown one underlined in `foreground`. At the row's right, a skillshare project gets an All · Project · Global switch that Overview and Library share; outside a project it says "Global". On Machines the same place says when the machines were last checked.
  - Overview:
    - a headline with its mark: "Reading skillshare", "Everything is in sync", "n targets need a sync" (or "n resources to sync" when only MCP servers, plugins or hooks wait), or "Syncing n targets"; a primary Sync button while something waits;
    - the map: a row of five counts (Skills · Agents · MCP · Plugins · Hooks). Each opens Library on that kind, and turns `blocked` when that kind has something waiting. Under it, one 32px line runs down to each target: a status dot, the agent's mark, the name, and a check or "n to sync". It draws at most six lines, waiting targets first; targets that do not fit become a count ("+n more", "n others"). Resting the pointer on a count for 250ms opens a hover card (base `HoverCard`, `top_left`, so it hangs under the line; `anchor` names the card's corner, so `bottom_*` opens it upward over the map) that lists those targets as the lines they would have been, inset so its dots continue the column above. Target names come from `status --json` and `diff --json`; a target's count is what `diff` reports for skills and agents, plus each MCP server, plugin or hook that waits on it. A waiting line is dashed in `blocked` and flows while a sync runs.
    - the agent's mark is Herdr's own program icon, found by the target's name or its `<agent>-` prefix; `omp` and `universal` use the two colour marks in `icons/`, and anything else the generic agent icon;
    - then only what needs you: "WAITING TO SYNC · n", "UPDATE AVAILABLE · n" with Update on each row, "HOOKS NEED A LOOK · n", and "MACHINES · n" once the Machines tab has read them.
  - Library:
    - Skills · Agents · MCP · Plugins · Hooks tabs with counts (named hook entries, including pending cleanup; notices do not count);
    - a filter field, a Sync button (primary while something waits) and a 30px install button;
    - the list, the only part that scrolls: rows whose sync state is unknown, then "NEED A LOOK · n" (hook notices), "NEEDS SYNC · n" in `blocked`, "UPDATE AVAILABLE · n" in `accent` with Update all when there are several, "IN SYNC · n", and "OFF · n" for disabled rows. For skills and agents "IN SYNC" has a List · By source switch, By source by default and kept in `shared`: rows sharing a repository (a tracked repo, or host/owner/repo of the source URL, with github.com left out) or the local source fold under a collapsed row with a chevron, a `git` or `folder` icon, the source and a count. A source with one row stays a plain row, and a filter opens every group.
    - Each row is 46px: a dot (filled `accent`, `blocked` or `ansi2`; hollow `ansi1` when never synced or `muted` when disabled), the name, one short state, and a chevron. A row with an update also has Update. Clicking a row opens its preview; rows do not open in place.
  - Machines: one card per Herdr machine. Its header has a status dot, the name, "this computer" and the version (a pill when a newer skillshare exists). Two lines follow in the map's grammar, Remote to Source and Source to its targets, each with its state at the right, and the targets that wait by name, three at most. The footer has Pull and sync, or Sync both ways when that machine has uncommitted or unpushed work and an upstream (a plain pull would refuse), "Update to …" when a newer skillshare exists, and the doctor's errors or warnings. While the first check runs, two card-shaped placeholders pulse.
  - a 34px status line while work runs (work, installs included, runs one at a time per machine; machines run side by side), with a spinning ring in `working`, or a notice: a done notice clears after 4 s and a warning or error after 8 s, and a stopped pull keeps a "Review conflicts" bar;
  - a 40px footer: the source state, such as "Source · n uncommitted · ↑n ↓n", "up to date" or "no remote" (`blocked` when it needs attention), and Sync both ways when the repository has an upstream.
- Loading draws the content's own shape: the map with empty counts and four lines, list rows, machine cards. Each pulses once every 1.2 s.
- Motion is Herdr's native kind: the status line's height slides (180 ms) so the list does not jump, a waiting line breathes and a syncing one flows, a dialog fades in and rises 8px (200 ms) and fades out (150 ms) before it closes, and buttons and rows show a pressed state.
- Dialogs open as Herdr overlays with `frame: "none"`, so each draws its own card: a 56px header (title, one line under it, an Esc keycap and a close button), the body, and a 54px footer with quiet actions at the left and one primary button at the right. They are: preview, install, hook change, enable or disable, uninstall, audit hand-off, source repository, sync both ways, pull conflicts, update skillshare, update the panel, command output.
  - Preview (760×560) has one tab per question, so no pane shares its width:
    - Document (Settings for an MCP server, Definition for a hook, About for a plugin): a skill's `SKILL.md` rendered as Markdown, with a file list at the left only when the skill has other files; an agent's file; an MCP server's transport, command, arguments and the names of its env, headers and token; a hook's description, agents and native files; a plugin's source and version.
    - Targets n (Agents n for hooks and plugins): the item on top and a line down to each target, as on Overview.
    - Details, for skills and agents: the frontmatter as rows, and where the files are.
    - A hook notice has no tabs: it shows what the notice says.
  - Install shows its four steps (Source, Choose, Audit, Install) above the body.

## How it behaves

- The panel follows the space the window shows. It runs the CLI on that space's machine, in the focused terminal's folder or the space's folder. A folder with `.skillshare/config.yaml` or `skillshare/config.yaml` is a project. It refreshes every 20 s while shown and runs `skillshare check` every 10 minutes. It does no work while hidden.
- The toggle and the close button run `layout.toggle` / `layout.hide {id: "skillshare-panel"}`. The node's hidden state is a layout click, so the panel stays open or closed across space switches and restarts.
- Clicking a row opens its preview. Its footer holds what you can do:
  - skills and agents that have an update offer Update, and skills also See changes;
  - skills and agents that need a sync offer Diff;
  - skills and agents offer Disable, or Enable when disabled (`skillshare disable` / `enable` with the entry's own path, `--kind agent` for an agent; the next sync applies it), Uninstall, and "Ask <agent> to audit…" when Herdr knows an agent: the one focused, or the last one focused;
  - top-level declared hooks offer Enable, Disable and Remove;
  - MCP and plugin previews have no actions; Sync handles them.
- A preview reads on the item's machine: the skill's file names (40 at most) and the first 60 kB of the file shown, an agent's `.md`, or the MCP source file `mcp list --json` names. Env, header and bearer-token values are replaced before the dialog gets them, so it shows names only.
- Hook conflicts block `sync --all` for every resource kind; resolve them in the CLI or dashboard before retrying.
- Sync, on Overview, syncs every scope that has something waiting in the scope the switch shows, and also runs `sync plugins` when a plugin waits. Sync in Library does the same, and with nothing waiting it syncs the shown scope again.
- The install button opens a URL dialog with recent sources:
  - a repository with several skills shows a pick list, marking the ones that would replace an installed skill;
  - a progress view then runs Download (a shallow clone), Audit (`skillshare audit <path> --json` on the clone) and Install;
  - a blocked audit stops before anything is written and shows its findings, with "Ask <agent> to review…", Cancel and Install anyway (`--force`).
- The source dialog lists uncommitted files and offers:
  - Sync both ways…;
  - Pull, which stays disabled while there are uncommitted changes or nothing to pull;
  - Commit and push with a commit message field when files are uncommitted, or Push n commits when only commits are waiting.
- When a pull stops on files both machines changed, the conflict dialog shows each file's two versions with the differing lines marked. You keep one version per file and then apply. `.metadata.json` merges per entry the way skillshare's own pull does, so only real files ask.
- The panel updates itself. `version.js` holds the skillshare release it shipped with. Once an hour while shown it asks, on this computer, where `github.com/runkids/skillshare/releases/latest` lands, and offers "Update panel" when that release is newer. The update clones only `herdr/skillshare/mods/skillshare` at that tag, checks that `mods/skillshare` in Herdr's home holds this panel's version, copies the current files once to the mod folder's `.bak-update-<version>`, and copies the new files in with `main.js` after its modules and `version.js` last. Herdr reloads the panel, and the new build says "Updated the panel to v…". The layout is not touched.
- The list answers a click before skillshare does. `optimistic` (model) lays every request still running, or ended and not yet read back, over the rows: a disable, an enable, an uninstall and a hook change show their outcome at once, and a finished install lists its skills; an update and a sync can be stopped, so their rows only say "updating…" or "syncing…". A request that fails is dropped and the rows go back; otherwise the read that starts after the request ended replaces them (an update's rows wait for the checks). Git sync, upgrading skillshare and updating the panel are not guessed.
- Hook changes (Enable, Disable and Remove), enabling or disabling a skill or agent, Uninstall, the audit hand-off, conflict choices, updating skillshare itself and updating the panel each ask first. Sync applies hook changes through `sync --all`.

## Built from

- `layouts/herdr-skillshare.json`: a `layout.clone` of `main` (Herdr 0.1.12, `cloned_from` recorded) with two changes:
  - the tab bar block becomes `{ "id": "tabs-row", "type": "row", "size": { "px": 42 }, "resizable": false, "children": [ the tabs block with "size": { "weight": 1 }, { "id": "skillshare-toggle", "type": "block", "content": { "mod": "skillshare", "props": { "role": "toggle" } }, "size": { "px": 118 } } ] }`;
  - `work-body` gains `{ "id": "skillshare-panel", "type": "column", "size": { "px": 400 }, "min": 380, "max": 720, "children": [{ "id": "skillshare", "type": "block", "content": { "mod": "skillshare" } }] }`.
  - Root `"preview": "layouts/herdr-skillshare.png"` is the picture Settings shows.
- `mods/skillshare/`:
  - `main.js` holds the panel and its roles (`props.role`): the panel by default, `toggle`, and the dialogs.
  - `model.js` holds the parsing and the shell scripts, with no Herdr imports.
  - `jobs.js` runs skillshare: short reads as one `exec`, longer work detached and polled.
  - `channel.js` passes requests from dialogs to the panel through `shared`, with one lock per machine and the notice line.
  - `operations.js` holds the commands each request runs and the conflict handling of a pull.
  - `loader.js` reads the panel's data, the Machines cards and what a preview shows.
  - `version.js` is the panel's version; Release Please rewrites it in every release.
  - `mark.svg` is the 16px mark, and `icons/` holds the stroke icons and the `omp` and `universal` marks.
- Herdr functions:
  - `exec` with `on`;
  - `readTextFile`;
  - `spaces`, `focused`, `machines`, `agents`, `knownAgents`, `layout`;
  - `shared` / `setShared`, `settings`, `theme`, `props`;
  - `openOverlay` / `closeOverlay`, `nextCommand`, `run`, `hc`.
- Commands: `layout.toggle`, `layout.hide`, `agent.focus`.
- Programs:
  - `skillshare` (found in `~/.local/bin`, `~/go/bin`, `/opt/homebrew/bin` or `/usr/local/bin`);
  - `git`, which clones the install URL you type from the network, and the skillshare repository for a panel update;
  - `curl`, which asks GitHub for the latest release;
  - `find`, `sed`, `sort` and `head` to list a previewed skill's files and read the first 60 kB of one;
  - `sh`, `mktemp`, `find` and `rm` for temp folders (`rm -rf` only on folders it made with `mktemp -d`, named `skillshare-panel.*`, and on an unfinished `.bak-update-<version>.tmp` in its own folder);
  - `cp`, `mkdir`, `mv` and `grep` for a panel update;
  - `setsid` where it exists, `sleep`, `cat`, `kill -0`, `ps` and `grep` to run long commands detached and check they still run;
  - `hc send` for the audit hand-off.
- Files read: the temp folders it made, through `readTextFile`; and, for a preview, the skill, agent or MCP source file you opened.
- Files written: on a panel update, its own folder in Herdr's home (`mods/skillshare/`, with `.bak-update-<version>` inside).
- Its look is Herdr's: theme tokens only, no theme or fonts of its own.

## Watch out

- Herdr stops an `exec` after 30 s. Long commands (the list snapshot, check, doctor, install, update, sync, pull, push, upgrade, the panel update) run detached into a temp folder on that machine, and the panel polls for their exit code. Herdr ends an exec's whole process group when it returns, and hangs up programs that still carry the exec's `HC_SCOPE`, so each job drops `HC_SCOPE` and gets its own group (`setsid`, or sh job control where `setsid` is missing, as on macOS). The panel checks every 5 s that the job still runs, and reports "skillshare stopped before it finished" when it was killed instead of waiting the full 15 minutes. A panel closed mid-job never collects the result, so the first job on each machine removes `skillshare-panel.*` folders unchanged for 15 minutes whose job no longer runs.
- The CLI has no JSON for git state. Git state comes from `git status --porcelain=v1 --branch`.
- The CLI's install dry-run does not list a repository's skills, and `install --json` names a blocked skill without its findings. That is why the install dialog clones and audits on its own.
- `install --json` overwrites a skill of the same name. The dialog asks before replacing one.
- Keep the panel out of the `panel` stack. Herdr's tabs mod restores that stack per space, so a panel there closes on every space switch.
- `cx.notify()` throws during `render`, and `cx.spawn` runs its body up to its first `await` at once, so work started from `render` waits for the render to end first (`later`). Otherwise the error is swallowed and the button does nothing.
- QuickJS rejects `??` mixed with `||` or `&&` without parentheses. `herdr-app check` catches it; `node --check` does not.
- If a `skillshare` mod is already installed, rename this one and the `mod` names in the layout.
- Saving a mod's file reloads it and ends the running build's tasks, so the build that starts a panel update never sees it finish. It leaves `{ to, at }` in `shared`, and the build whose `version.js` matches reports the result and gives the lock back. That is why `version.js` is copied last, why the button hides while any work runs, and why the update holds the lock of every machine until the reloaded panel gives it back: work started on another machine meanwhile would lose its remaining steps.
- The dialog's role is `panelupdate`: `panel` is the role a block without one gets, so a dialog with it would draw a second panel.
- `enable` and `disable` exit 0 once the entry's own line in `.skillignore` or `.agentignore` is written or was not there. A glob, a folder pattern or a negation in the `.local` file can still decide the other way, so the panel reads `list --all --json` afterwards and reports an entry that did not change.
- The dialog role for enabling or disabling a skill or agent is `able`, not `toggle`: `toggle` is the tab-bar button's role.
- A layout clone is never updated: rebuild from this recipe on a new Herdr version.

## Done when

- `herdr-app check ~/.config/herdr-app/mods/skillshare` passes.
- `layout.switch {name: "herdr-skillshare"}` shows the layout with no `herdr-skillshare.error`.
- The tab bar shows Files, Changes and skillshare. The skillshare button shows the panel, and a second click hides it. Switching spaces keeps it as it was.
- In a skillshare project the tab row shows the All · Project · Global switch. Outside one it says "Global" and shows the global source.
- Clicking a row opens its preview, and Esc fades it out.
- With `version.js` set below the latest release, the header shows "Update panel" within a minute of the panel showing; confirming replaces the files, keeps `.bak-update-<version>`, and the reloaded panel says "Updated the panel to v…".
