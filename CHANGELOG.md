# Changelog

## [0.24.3] - 2026-10-04

### New Features

- **`doctor` points out links at the top of the skills source** — a symlink or Windows junction directly inside the skills source is not followed by discovery, so the skills behind it never reach your Agents. `doctor` now lists each one as a **Source link** info row, in global and project mode, without following it; `doctor --json` reports them as `undeclared_source_links` checks with status `info`. A write into the skills source whose path crosses such a link is refused instead of written through it. Refs: #274.
  ```bash
  skillshare doctor
  ```

### Bug Fixes

- **Plugin commands find the Codex CLI that the desktop app ships** — Codex installed only through the Codex desktop app, Homebrew or the Windows installer was not on `PATH` for Skillshare, so plugin commands for the `codex` target stopped with a missing-CLI error. Skillshare now also looks in those places (macOS `ChatGPT.app`, Windows `%LOCALAPPDATA%\OpenAI\Codex\bin\<version>`, `/opt/homebrew/bin`, `/usr/local/bin`), and the error lists every place it searched. On a machine where Codex lives elsewhere, set `SKILLSHARE_CODEX_CLI`; it is read from the environment, so a `config.yaml` shared between machines needs no machine-specific path. Refs: #338.
  ```bash
  export SKILLSHARE_CODEX_CLI=/path/to/codex
  skillshare sync plugins
  ```
- **The shared-memory `active` guidance says where a lesson goes** — a durable fact or decision gets its own note linked from `INDEX.md`; a lesson from a failure goes under a new heading in `LEARNED.md`, as the starter file and the sharing-memory guide describe. The `--update-mode` help now also says that `passive` saves notes when you ask, not only points them out.

### Performance

- **Agents are told to save to the shared notes instead of their own memory** — the guidance block connected to each Agent now says that the notes folder is shared by every coding tool, to read `INDEX.md` at the start of each task, and, in `active` mode, to save a fact worth keeping here rather than in the tool's own memory, with a note format and the `INDEX.md` line to add. `passive` still saves only on request, but points out facts worth keeping so you can ask. Existing guidance blocks show as **Outdated** in the dashboard's Memory page; reconnect to update them.
  ```bash
  skillshare extras memory instructions --update-mode active -g
  ```

## [0.24.2] - 2026-10-04

### Performance

- **Upgrades and the first dashboard start download much faster on throttled networks** — GitHub's release file server limits each connection on some networks, so `skillshare upgrade` could spend minutes on the 9 MB archive and the first `skillshare ui` of a new version waited again for the dashboard files. Downloads now split the file into 8 parts fetched at the same time, each on its own connection; in one measurement the darwin_arm64 archive went from 171 s to 25 s. Checksums are verified as before, and a server without partial download support gets a single download as before.
  ```bash
  skillshare upgrade
  ```

## [0.24.1] - 2026-10-04

### New Features

- **Agents can keep the shared memory notes up to date** — each connected agent now has an update mode. `passive`, the default, reads the notes and updates them only when you ask. `active` also lets the agent save facts that will matter in later sessions, such as a stated preference or a decision with its reason; it skips one-off details, asks before saving when unsure, updates an existing note instead of adding a duplicate, and tells you what it saved. Choose the mode per agent in the dashboard's **Connect to agents**, which now shows each file's change as one diff, or print the guidance from the CLI:
  ```bash
  skillshare extras memory instructions --update-mode active -g
  ```
  Agents that read the same file share one mode. The dashboard flags an agent that reads guidance in both modes from different files.
- **Config files saved by the CLI are laid out like the dashboard's Beautify** — when a command saves `config.yaml`, globally or in a project, its sections follow the dashboard's order with a blank line between them, instead of running together with `mcp`, `plugins` and `hooks` above the sources. A file that uses YAML aliases keeps its order where sorting would break them.

### Bug Fixes

- **Pi turns a global MCP server off in a project with its own override** — a `disabled` entry now writes `"NAME": {"enabled": false}` to `.pi/mcp.json`, the override Pi 1.0.1 and later read, instead of a copy of the global server's command or url. Turning the server back on with Pi's `/mcp` used to start it without its args, env or headers; now the global server keeps all of them. It also works in project mode, where `pi` in a `disabled` entry used to be an error. The next sync rewrites switches written by earlier releases, and an identical override made with Pi's `/mcp` is no longer a conflict. Older Pi reports the override as invalid. Refs: #378.
- **Audit no longer blocks config keys named like role labels** — `prompt-injection-1` reported lines such as `root: ./src`, `admin: false` or `ignore: { tags: ["design"] }` as CRITICAL prompt overrides and blocked the install. Lowercase config keys with a plain value now pass, while directives such as `SYSTEM: jailbreak`, `system: Override ...` or an empty `system:` opening an indented block are still flagged. Refs: #374.
- **The dashboard counts hooks** — the global and project dashboards listed every resource type except hooks, so a source with hooks looked empty. Hooks now appear between MCP and Plugins, as in the sidebar.

## [0.24.0] - 2026-10-03

### Breaking Changes

- **`init` without a terminal now sets up what pressing Enter would** — in CI, scripts and AI agents, `init` used to do less than the interactive defaults. It now enables git, installs the built-in skill and selects every detected tool, and prints each decision with the flag that changes it. To keep the previous minimal setup, pass the flags explicitly:
  ```bash
  skillshare init --no-git --no-skill --no-targets --no-copy
  ```
- **Output that is not a terminal is plain text** — piped or redirected output no longer contains ANSI colors. Parse `--json` where a command offers it rather than the human-readable text, which changed throughout this release.

### New Features

#### Init

- **Init asks first, then writes** — `init` collects every answer, shows a summary with Yes / Change settings / Cancel, and writes nothing until you confirm. Esc cancels cleanly, and `--dry-run` no longer creates tool folders. The built-in skill is installed by default, and the first sync runs at the end so the tools work right away.
- **Connect an existing skillshare repo** — on a second machine, choose "Connect my existing skillshare repo". Init checks the repo before writing, detects its layout, prefers the repo's version of a skill with the same name, and keeps local-only skills for the next push. Init no longer writes a git identity into a repo that already exists.

#### Command output

- **One output style for every command** — `sync`, `status`, `install`, `update`, `check`, `push`, `pull`, `doctor`, `audit`, `mcp`, `plugin`, `extras`, `target` and the other commands report results as aligned rows and end with one closing line or the next step to take, without boxes, logos or trees.
- **Help that fits on one screen** — `skillshare help` lists one line per command, and `--help` after a subcommand shows that subcommand's help instead of running it.
  ```bash
  skillshare extras memory --help
  ```
- **`doctor` warns about an outdated built-in skill** — when a newer built-in skill is published, `doctor` says so and counts it in the summary.
- **`trash empty --force`** — empties the trash without asking.

#### Terminal UI

- **One frame and keymap for every full-screen TUI** — `list`, `trash`, `log`, `audit`, `analyze`, `diff`, `restore`, `search`, `target`, `extras` and the MCP manager share one title line, one key line with the common keys and the position, and lowercase letters for actions. Less common keys are under `?`, and confirmations ask on the key line so the list stays visible. In `list`, `tab` switches between skills and agents; the All tab and the `s` status cycle are gone, and `status:` in the filter or `--status` covers enabled and disabled.
- **Small choices are asked inline** — the install skill picker, a large repo's folder picker, the restore source, the `new` wizard, `extras init` and MCP and plugin values no longer take over the screen. The question appears in place and collapses into a `✓` line that stays in the scrollback. Long lists can be narrowed with `/`, and long descriptions are cut to one row.
- **A file viewer in the TUIs** — browse a skill's files from `list`, `extras`, `audit` (opened at the finding), `analyze` and `trash` (before restoring). Control characters and hidden Unicode are shown as symbols, and very large files are skipped.

#### Git sync

- **`push --pull` syncs both ways in one command** — commits local changes, merges the remote the same way `pull` does, pushes, and syncs the scope's targets. A conflict stops before anything is pushed and keeps the local commit. It never rebases or force-pushes.
  ```bash
  skillshare push --pull
  ```
  The dashboard's Git Sync page has the same action as **Sync both ways**, which lists what it will do before changing anything.

#### Memory

- **Shared Markdown memory across agents** — keep notes in `extras/memory`, globally or per project, and connect the agents that should read them. The dashboard has a two-pane Memory browser with search, moves and renames, version history and recovery from backups. From the CLI:
  ```bash
  skillshare extras memory init
  skillshare extras memory list --search deploy
  skillshare extras memory instructions   # guidance to add to your agent instructions
  ```
  This is a shared file overlay; each agent's own automatic memory stays separate.

#### Pi

- **Turn individual Pi extensions on and off** — a Pi target's **Extensions** tab in the dashboard lists every package in Pi's settings, including ones installed with Pi itself, and switches each extension for the global, account or project target. Each change shows the exact edit to Pi's settings before it is written, changes only that package's extension list, and is refused if Pi would apply it differently. Turning a file back to what the package loads by default removes its rule instead of adding the opposite one. Editing needs Pi 0.99.2 or later; older versions are read-only and say which version is needed.
- **Add npm packages from pi.dev** — `plugin add npm:<package>` installs a Pi package through `pi install` and records it like an import. The preview says that Pi runs the package's install scripts, and only targets that run Pi take npm sources. The dashboard also accepts a pasted `pi install` command or a pi.dev address.
  ```bash
  skillshare plugin add npm:@scope/package --target pi -g
  ```

#### Install and check

- **`check` detects changes at local install sources** — skills installed from a local path, such as one shipped inside an app, used to show only "local source". `check` now compares the files at that path with the ones installed and reports an available update, or an error naming the path when the source is gone. Project installs with a relative path are compared against the project root.
- **Paste an `npx skills add` command in the install dialog** — the dashboard reduces `npx skills@latest add owner/repo --skill=name` or `owner/repo@skill` to its source and preselects the named skills.

### Bug Fixes

#### Git sync

- **Pull no longer discards local commits that hold only files** — a first pull treated an agents or extras repo whose content was root-level files as empty and reset it onto the remote without `--force`.
- **This machine's `config.yaml` is protected at the root git scope** — a first pull is refused when the remote tracks `config.yaml`, a later pull that brings in a tracked copy keeps the local file and warns, and a push is refused when unpushed commits add or change `config.yaml`. The refusal says how to repair the history.
- **Pull syncs extras at the root git scope** — in the CLI and the dashboard, merged extras changes now reach their targets.
- **The pull conflict dialog shows what differs** — both versions get taller panes, lines only one side has are tinted, each side scrolls to its first difference, and the header counts the differing lines.

#### Install and upgrade

- **`upgrade` verifies the download before replacing the binary** — the release archive is checked against the release's `checksums.txt`; a mismatch or an unreachable checksums file stops the upgrade and leaves the running binary untouched.
- **Local installs update from any directory** — global installs record the absolute path of a local source, so `update` and `check` no longer fail with "source path does not exist" when run elsewhere. A skill installed as its `SKILL.md` alone is updated in that shape instead of copying the whole folder.

#### Windows

- **No more flashing console windows** — started without a console, such as from a scheduled task or `pythonw`, a single `pull` opened about ten terminal windows. Skillshare now runs its child processes in a hidden console.
- **Stopping skillshare stops its child processes** — killing it mid-pull no longer leaves git, ssh or hooks running. The `ui start` server, browser and editor still keep running.
- **`install file:///C:/...` works** — the URL is no longer rewritten with backslashes before it is handed to git.
- Interrupted or oversized downloads no longer leave temp files in `%TEMP%`, `audit` no longer reports `content-missing` for absolute paths in skill metadata, and Pi's locks left by a crashed process are recovered.

#### Plugins and MCP

- **Symlinked config folders are accepted** — a linked `/var` on macOS, a home on a linked volume, a dotfiles-linked `~/.pi`, or a `PI_CODING_AGENT_DIR`, `CLAUDE_CONFIG_DIR` or `CODEX_HOME` behind a link no longer fails with "refusing to modify symlinked native path". A linked file or folder inside the config folder is still refused.
- **Pi project MCP overrides are explained** — a `.pi/mcp.json` entry that only enables or disables a global server, as `/mcp` in Pi 1.0.1 writes, is reported as a project override with nothing to import, and a sync conflict with it says to replace it or change it with `/mcp` in Pi.

## [0.23.5] - 2026-10-02

### Bug Fixes

#### Sync

- **An `include` pattern that matches no skill is reported** — target filters match the source path name, such as `frontend__dev`, even when `target_naming: standard` shows the folder as `dev`. A pattern written with the name the target shows selected nothing, and sync removed what a working pattern had linked without saying why. Sync now warns about each `include` pattern that matches no skill, names the target, and suggests the source path name when one fits. The warning appears in `skillshare sync` and in the dashboard's sync results. `--json` output is unchanged.
  ```text
  ! claude: include pattern "dev" matches no skill in the source (filters use the source path name; did you mean "frontend__dev"?)
  ```

## [0.23.4] - 2026-10-01

### Bug Fixes

#### Plugins

- **Claude marketplace cleanup no longer fails after settings lost the declaration** — when a Skillshare marketplace was only recorded in Claude's `known_marketplaces.json`, for example after a dotfile manager overwrote `~/.claude/settings.json`, every sync failed with "could not finish removing its marketplace" and the plugin stayed pending. The marketplace is now removed from every settings scope. A name also declared at another path is reported as a conflict and left in place.
  ```bash
  skillshare sync plugins -g
  ```
- **Skill and plugin name clashes are explained before install** — Claude reads a skill folder that has a plugin manifest as `<name>@skills-dir` and loads only one plugin per name, so adding a Claude plugin from the same repository as a synced skill made `/plugin` report the skill as not loaded. The install preview now says Claude will load the plugin and skip that folder until one of them is renamed or removed.

## [0.23.3] - 2026-10-01

### Bug Fixes

#### Plugins

- **Plugin marketplaces are named after their plugin** — the local marketplace Skillshare registers for each Claude Code or Codex plugin was named `skillshare-<hash>`, which said nothing in `/plugin` or `codex plugin marketplace list`. New installs are named `skillshare-<plugin>-<hash>`, such as `skillshare-humanizer-322a2e01808560f4`. Existing installs keep their names.
- **Excluding or removing a plugin removes its marketplace** — the marketplace stayed registered after its plugin was excluded or removed, and for good when the plugin had already been uninstalled in the Agent or its first install had failed. Skillshare now removes the marketplace it registered in those cases too, and a failed cleanup runs again on the next sync. A marketplace with the same name at another path, and imported plugins' marketplaces, are left alone. An update registers a missing Skillshare marketplace again.
  ```bash
  skillshare plugin disable humanizer --target claude -g
  skillshare sync plugins -g
  ```
- **Imported plugins whose marketplace is gone are skipped with the reason** — when an imported plugin's native marketplace was no longer registered, for example after synced settings dropped it, sync failed with "claude command failed". Sync and update now skip that Agent and say how to recover, and the plugin's other Agents still sync. Skillshare does not switch an imported plugin to its recorded source by itself.
- **Plugin errors are translated and shown once** — the Plugins page showed failed operations in English, and repeated the same message once per failed Agent.

## [0.23.2] - 2026-10-02

### New Features

#### Hooks

- **Git config hooks** — a hook entry can bind `git` to register named Git config hooks (Git 2.54+) globally or in a project. Skillshare writes the commands to its own include file, adds the `include.path` line, and writes the helper scripts listed under `files`; `{files}` expands to their directory on each machine. Preview, sync, backups and restore work as they do for Agent hooks, and managing hooks never runs them. Linked worktrees share their project's hooks.
  ```yaml
  bindings:
    git:
      commands:
        project.check:
          events: [pre-commit]
          command: "{files}/check.sh"
      files:
        check.sh: |
          #!/bin/sh
          exec make check
  ```
  ```bash
  skillshare hooks add git-check --file git-check.yaml -g --dry-run
  skillshare hooks sync -g
  ```
  With Git older than 2.54 the files are still written, and the plan says why the hooks cannot run. `parallel` needs Git 2.55+. When the include target is a symlink or not writable, the plan prints the lines to add by hand. The `git` key always means Git, so an account target named `git` receives no hooks; the plan warns once an entry uses the `git` binding.

#### Doctor

- **Check MCP servers, hooks, plugins and extras** — `skillshare doctor` and the dashboard's Doctor page now check these too, in global and project mode:
  - MCP: environment variables, commands, client rules and sync state, without DNS lookups or starting servers
  - Hooks: what `hooks sync` would still change or refuse. Git hooks that cannot run, for example because Git is missing, are reported with the reason instead of as unsynced
  - Plugins: what `plugin sync` would change, without fetching sources, when a plugin is configured
  - Extras: config errors, broken links in targets, and drift as `diff` reports it
  ```bash
  skillshare doctor --json
  ```

#### MCP

- **Pi 1.0 OAuth metadata URL** — `piOptions.oauth.authServerMetadataUrl`, which Pi 1.0 uses instead of discovering a server's authorization server, is checked when you save: it must use https, or http on localhost. Pi 1.0 keeps OAuth sign-ins per server name and URL, so after renaming a server or changing its `url`, sign in again in Pi.
  ```yaml
  mcp:
    servers:
      example:
        url: https://mcp.example.com/mcp
        piOptions:
          oauth:
            authServerMetadataUrl: https://example.okta.com/.well-known/openid-configuration
  ```

#### Backups

- **Backup limits and Delete all** — `backup.max_count` and `backup.max_size_mb` in the global config set how many target folder snapshots to keep and their total size (defaults 10 and 500 MB, `0` = no limit). `sync`, the dashboard's sync and `backup --cleanup` apply them; backups older than 30 days are still removed. The dashboard's **Target folders** tab shows the limits, edits them, and adds **Delete all** behind a confirmation. Project snapshots keep the defaults, and file, MCP and hooks backups are not affected.
  ```yaml
  backup:
    max_count: 20
    max_size_mb: 1000
  ```

#### Dashboard

- **Share several plugins as one command** — **Share** on the Plugins page, in the header or a plugin's menu, lists the plugins added from an HTTPS source and copies one line that adds the ticked ones in order. Each add uses `--no-tui -g`, so the plugins land in the recipient's global config without a picker per plugin; they choose Agents on the Plugins page afterwards. An option keeps the per-plugin Agent picker. Plugins added from a local directory are left out, since the path only exists on your machine.

## [0.23.1] - 2026-10-01

### New Features

#### Plugins

- **Update Codex plugins** — `plugin update` now updates a Codex binding. Codex has no update command, so Skillshare refreshes the reviewed snapshot, adds the plugin again, which replaces the installed copy, and checks the installed version. A plugin disabled in Codex is skipped with the reason instead of being turned back on.
  ```bash
  skillshare plugin update review --target codex --no-tui
  ```
- **Run a compatible CLI for an account** — an account target can set `cli` so its plugin commands run a compatible executable instead of the Agent's own, such as `omo` for a Pi account. It takes a name on `PATH` or an absolute path. If the CLI is missing, the command fails; Skillshare does not fall back to the Agent's CLI.
  ```bash
  skillshare target add omo --agent pi --config-dir ~/.omo/agent --cli omo
  ```

#### Hooks

- **Hooks for account targets** — a global hook binding can name an account target declared with `agent` and `config_dir`, such as a second Codex home. Claude, Codex and Pi accounts use their Agent's native binding format, and sync, import and backups keep the account name.
  ```yaml
  targets:
    codex-2:
      agent: codex
      config_dir: ~/.codex-2
  hooks:
    entries:
      check:
        bindings:
          codex-2:
            events:
              Stop:
                - hooks:
                    - type: command
                      command: "echo checked"
  ```

#### MCP

- **Turn off a global server for Pi in one project** — a `disabled` entry for a project under `mcp.projects` now reaches Pi. Skillshare writes the global server's `command`, or its `url` without the query, with `enabled: false` to that project's `.pi/mcp.json`; args, env and headers stay out of the file. This replaces the 0.23.0 change that removed `pi` from the targets of `disabled` entries. If a 0.23.0 sync already removed `pi` from such an entry, add it back to `targets`. A project's own config still cannot turn off a global server for Pi.
  ```yaml
  mcp:
    projects:
      ~/work/project01:
        servers:
          context7:            # off in this project only
            disabled: true
  ```
- **Turn a server off in Pi from the dashboard** — **Turned on in Pi** in a server's Pi settings writes `enabled: false` when you clear it, so Pi keeps the server without connecting to it, and removes it when you check it again. It sits above **Tool exposure** and stays in step with the **Other Pi settings** JSON.

#### Dashboard

- **Discard Git Sync changes** — **Discard changes** restores tracked files in the source repository to the last commit and deletes untracked files and folders, after you confirm. Ignored files, nested repositories and the root `config.yaml` are kept, and a dry run shows what would change.
- **Resolve Git pull conflicts across computers** — when this computer and the remote both changed the same files, Git Sync no longer only stops with the merge undone. **Review conflicts** shows the local and remote version of each file side by side; keep one whole-file version per file, then **Apply choices and pull**. Both commit histories are kept, other files merge normally, and `.metadata.json` conflicts still resolve automatically. If either side gets new commits before you apply, the dialog asks you to choose again. Binary files and files over 16 KiB can be chosen but have no preview.
- **Pull and merge, Commit and pull** — when both computers have new commits, **Pull** becomes **Pull and merge** and the page explains that both histories are kept before you push. With uncommitted changes while the remote is ahead, **Commit and push** becomes **Commit and pull**.

### Bug Fixes

#### Sync

- **Sync no longer deletes skill links you made yourself** — in merge mode, sync removed any link in a target that pointed into the skills source but was filtered out, including links you created by hand or with another tool, and the removed links could not be restored. A live link is now removed only when Skillshare created it, as recorded in `.skillshare-manifest.json`, or with `--force`; other links are kept and counted as local. Links created before 0.15.0 that are now filtered out are kept as well; remove them by hand or with `--force`.
- **Agent and extra links to other places are kept** — merge-mode sync and `extras --remove-target --prune` removed every agent `.md` link or extra link that was not expected, including links to files outside the source. Now only broken links and links into the source are removed. A link whose file cannot be read, for example because of permissions, is no longer treated as broken.
- **Copy-mode agents keep your own files** — in copy and extension modes, sync deleted every agent file in the target that did not match a source agent, including files you put there. Copies are now tracked in a `.skillshare-manifest.json` in that agents folder, and sync removes only copies it wrote that you have not edited. Copies that were already orphaned before upgrading are kept; delete them by hand.
- **Failed agent prunes are reported** — when sync could not delete an orphaned agent link or copy, it still counted it as pruned. It now shows a warning, and the other orphans are still removed.

#### Plugins

- **One Agent no longer blocks a plugin update** — a plugin installed on several Agents could not update anywhere when one of them could not take the update, such as a plugin disabled in Codex or Copilot, or an imported package. Those Agents are now skipped with the reason, the others update, and the skipped update stays pending. Imported Codex plugins now update through `codex plugin marketplace upgrade`, and imported Pi packages in global mode through `pi update`. A plugin whose Agents hold different versions shows each version.
- **Claude account targets list their plugins** — a Claude account target could fail with "native plugin list contains an unsupported plugin identifier", for example when the dashboard ran from the home folder, because Claude also lists plugins of other scopes. Those entries are now ignored.
- **Plugin results are translated** — the dashboard's Plugins page showed last-action statuses and installation messages in English in every language.

#### Hooks

- **Editing a hook from a target tab keeps account bindings** — saving a hook from a target page dropped its bindings for account targets such as `codex-2`, so the next sync removed that account's hook.
- **Hooks report config errors** — when the global config failed to load, `skillshare hooks` ran without account targets and reported `unsupported Agent "codex-2"` instead of the real error.
- **Missing account folders are explained** — the dashboard's hook config view showed nothing for an account whose `config_dir` is missing. It now shows a warning for that target.

#### MCP

- **Account shells keep the default home** — when `CLAUDE_CONFIG_DIR`, `CODEX_HOME` or `PI_CODING_AGENT_DIR` pointed at a declared account's `config_dir`, sync, import and the dashboard sent the plain Agent target to that account's home and could prune entries in its default home. The plain target now keeps its default home, and sync warns about the shadowed variable.
- **Project-mode MCP status is complete again** — in project mode, `mcp check`, the dashboard and the target list dropped pending syncs and conflicts, and `mcp remove --keep-files` did not stop managing a project's Claude off switch, so the next sync removed it anyway.
- **Pi 0.99.2 settings are kept** — Pi 0.99.2's `auth: {provider: NAME}`, which sends a provider's `/login` token to an HTTP server, was refused in `piOptions` as a `pi-mcp-adapter` setting, and loading or importing a config dropped it, and the dashboard's **Other Pi settings** refused it too. It is now kept and checked: it needs an https `url`, or http on localhost, and global mode, because Pi reads it only from its global file. Only the adapter's string `auth` is still removed. `oauth.clientName` is checked as text, and `description` passes through.
  ```bash
  skillshare mcp add docs --url https://example.com/mcp --target pi --pi-options '{"auth":{"provider":"github"}}' --no-tui -g
  ```
- **Server names Pi reads as one are refused** — Pi 0.99.2 reads names that differ only in `-` and `_`, such as `my-docs` and `my_docs`, as one server and skips the second with a config error. Skillshare now refuses the second before writing it.
- **Pi's error says how to turn off a server** — a `disabled` entry for Pi that Skillshare cannot write now fails with a message pointing at a complete server with `piOptions: {"enabled": false}`, instead of only listing the clients that support a switch. The MCP docs explain the same.

#### Dashboard

- **Beautify keeps config.yaml in order** — **Beautify** and save in **Settings → Files** now keep config sections in a consistent order with a blank line between them, and keep the `# yaml-language-server: $schema=…` line at the top of the file, moving one that was saved in the wrong place.

### Website

- **Desktop app guide** — a new [Desktop App](https://skillshare.runkids.cc/docs/getting-started/desktop-app) page covers installing Skillshare App on macOS, Windows and Linux, in all five documentation languages. The homepage, navigation and README now show the desktop app before the CLI installation. On Apple Silicon Macs:
  ```bash
  brew tap runkids/tap
  brew install --cask skillshare-app
  ```

## [0.23.0] - 2026-10-01

### New Features

#### Hooks

- **Manage native hooks** — `skillshare hooks` keeps named hooks in `hooks.entries` and writes them into each Agent's own hook configuration: Claude Code, Codex, Gemini CLI, Copilot CLI, Cursor, Factory Droid, Qwen Code, Antigravity (and its CLI, `agy`), Pi, Amp and OpenCode, in global and project scope. Each binding keeps that Agent's own event names and format; Skillshare does not translate between Agents. Sync previews every file it changes, keeps your other settings and hooks, reports a hook you edited by hand as a conflict, and backs files up before writing. It never runs a hook or changes an Agent's trust.
  ```bash
  skillshare hooks add check --file ./check.yaml
  skillshare hooks sync --dry-run
  skillshare hooks sync
  skillshare sync --all              # now includes hooks
  ```
- **Import the hooks you already have** — `hooks import --from AGENT` lists that Agent's existing hooks. Saving one takes over those registrations in place, so the next sync does not add a duplicate.
  ```bash
  skillshare hooks import --from claude
  ```
- **Disable, stop managing and restore** — `hooks disable` keeps the definition and removes it from the Agents on the next sync. `hooks remove --keep-files` stops managing a hook and leaves its entries in the Agent files as yours; `hooks import` offers them again. `hooks restore` brings back a backup of an Agent file without touching unrelated later edits.
  ```bash
  skillshare hooks disable check --sync
  skillshare hooks remove check --keep-files
  skillshare hooks restore BACKUP_ID --dry-run
  ```
- **Project hooks from the global config** — `hooks.projects` holds hooks for other project folders, so one sync from the global config reaches each of them.
- **Removal cleans up after itself** — removing a hook deletes a hook file Skillshare created once nothing else is left in it, together with the folders it created that are now empty. Files and folders that existed before, and anything you added, stay.

#### MCP

- **Tool policy per server** — `tools.allow` and `tools.deny` say which of a server's tools reach the model. Write them once; sync translates them into Pi's `toolExposure`, Codex's `enabled_tools` and `disabled_tools`, and Copilot's `tools`. Parts an Agent cannot hold, such as the whole policy for OpenCode and Kilo Code, are named in the sync plan and by `mcp check` instead of being dropped.
  ```bash
  skillshare mcp add github --target pi --target codex --tools-allow 'get_*,search_code' --tools-deny get_secret -- github-mcp
  skillshare mcp edit github --tools-allow ''    # clear the allow list
  ```

#### Dashboard

- **Hooks page** — add and edit hooks by picking each target's documented events, copy commands from another target's tab, or edit the native JSON with completion and checks. Every sync shows a diff of each file it changes. Import lists each target's hooks that Skillshare does not manage yet. Hooks also appear on target and project pages, on the Sync page and in **Settings → Backups**.
- **Clear remove choices** — removing a hook offers **Remove and sync**, **Remove from source only** and **Stop managing**; hovering or focusing a button explains what it does to the target files. In the hook and MCP remove dialogs, **Remove and sync** names the other pending hooks or servers that go out with it.
- **Tools in the MCP server dialog** — load a server's tool list with the dialog's current settings, even before saving, and tick the tools the model gets. The dialog says which selected Agents follow the policy fully, partly or not at all.
- **Plan notices before an MCP sync** — the MCP sync dialog and the Sync page list the plan's notices, such as Pi's built-in MCP needing Pi 0.99.0, before you sync.
- **Open config.yaml at the right place** — the **config.yaml** button in the Hooks and MCP page headers opens **Settings → Files** at the `hooks:` or `mcp:` section. The field panel there explains every `hooks` key.

### Bug Fixes

- **Script installs no longer need administrator access** — `install.sh` now installs to `~/.local/bin` by default; `INSTALL_DIR` still overrides the location. When that folder is not on your PATH, or an older `skillshare` such as `/usr/local/bin/skillshare` comes first, the installer says so and prints the PATH line to add. Remove the old copy (`sudo rm /usr/local/bin/skillshare`) so the new one runs.
- **Stop managing an MCP server works in the dashboard** — **Stop managing** in the MCP remove dialog always failed with a "changed since preview" error. It now succeeds, and its message says the entries stay in the Agent files.
- **Beautify unfolds one-line YAML** — in **Settings → Files**, **Beautify** left a section squeezed onto one line, such as `servers: {docs: {url: …}}`, unchanged and said there was nothing to tidy. It now unfolds nested one-line sections; short lists such as `targets: [claude, codex]` stay on one line.

### Breaking Changes

#### Pi MCP

- **Pi uses only its built-in MCP** — Skillshare now writes Pi's servers only to Pi's own `mcp.json` (`~/.pi/agent/mcp.json`, or `.pi/mcp.json` in a project). It no longer writes for `pi-mcp-adapter` or `pi-mcp-extension`, and the `piExtension` choice is gone. The first sync after upgrading moves your servers: entries Skillshare wrote to `mcp-adapter.json` are removed and written to `mcp.json`, and `pi-mcp-extension` entries are rewritten in place. Entries you added to `mcp-adapter.json` yourself are left alone, and `mcp import --from pi` still reads them.
- **Needs Pi 0.99.0 or later** — Pi added its built-in MCP in 0.99.0. On older Pi the moved servers stop loading until you update Pi. Skillshare does not check Pi's version; the sync that moves servers prints a warning.
- **Remove the old extension from Pi** — if `pi-mcp-adapter` or `pi-mcp-extension` is still installed in Pi, uninstall it. Pi's docs say an installed extension that registers `/mcp` replaces the built-in MCP, and `pi-mcp-adapter` 3.0.0 and later no longer read `mcp.json`.
- **Adapter-only server options are dropped** — Pi's built-in MCP does not read these `piOptions` fields, so the next sync removes them from your config: `approveTools`, `auth`, `bearerToken`, `bearerTokenEnv`, `bearerTokenStore`, `caFile`, `debug`, `exposeResources`, `idleTimeout`, `inheritEnv`, `lifecycle`, `protocolVersion`, `requestHeadersCommand`, `requestTimeoutMs`, `searchKeywords`, `socket`, `tasks`, `toolPrefix`, `trace`.
- **No per-project off switch for Pi** — Pi cannot turn off one global server in one project. The next sync removes `pi` from the targets of `disabled` entries, so that server is on again in that project. To keep a server off for Pi, set `piOptions: {enabled: false}` on a complete entry.
- **`directTools` becomes Pi's exposure** — `directTools: true` becomes `piOptions.exposure: direct`, `"search"` becomes `deferred`, and a list of tool names becomes `piOptions.toolExposure` with those tools `direct`. `mcp.directTools` and a project's default are copied into each Pi server that sets none. `piOptions.includeTools` / `excludeTools` become `tools.allow` / `tools.deny`.
- **Removed flags** — `--pi-extension`, `--direct-tools` and `--pi-options-prune` now fail with a message saying what to use. Use `--pi-options '{"exposure":"direct"}'` or `--pi-options '{"toolExposure":{"TOOL":"direct"}}'` instead of `--direct-tools`. Sync always removes unchanged Pi fields Skillshare wrote earlier, so `--pi-options-prune` is no longer needed.
- **Your config is updated on the first sync** — the old config still loads, and `sync mcp --dry-run` names each retired setting. The first `sync mcp`, `sync --all` or dashboard sync saves `config.yaml` (or the `sources.mcp` file) without them, keeping the previous version in the file history.

## [0.22.2] - 2026-09-30

### New Features

#### MCP

- **Check servers before an Agent starts them** — `skillshare mcp check` tells you whether each server will work as synced: a `fromEnv` variable that is unset, a `command` not found on `PATH`, a remote host that does not resolve, a server an Agent's rule refuses, and an Agent entry that conflicts with the source or is not synced yet. It starts nothing, writes nothing and never prints variable values. It exits 1 when it finds an error, so it fits in scripts. In the global config it also checks the servers under `mcp.projects` and names their project.
  ```bash
  skillshare mcp check
  skillshare mcp check docs github --json
  skillshare mcp check --no-dns    # skip the DNS lookup of remote hosts
  ```
- **Probe servers live** — `mcp check --live` also starts each local server, or sends one request to each remote server, reports its name, version, protocol version and number of tools, and stops it again. A server that needs sign-in is a warning; Skillshare never signs in. Each probe gets 10 seconds, or `--timeout`, and configured values are removed from every message.
  ```bash
  skillshare mcp check --live --timeout 30s
  ```
- **Pi built-in MCP** — Pi 0.99.0 includes MCP, and `piExtension: builtin` syncs servers into `~/.pi/agent/mcp.json` (`.pi/mcp.json` in a project) with no extension to install. `piOptions` takes Pi's per-server fields such as `exposure` and `toolExposure`, and import keeps them. A new server that goes to Pi without a mode now uses `builtin`; before, `mcp add --target pi` without `--pi-extension` failed. Existing servers keep their mode.
  ```bash
  skillshare mcp add docs --url https://example.com/mcp --target pi --pi-options '{"exposure":"deferred"}' --no-tui
  ```
- **Remove cleared Pi settings** — clearing a Pi setting stops managing it and leaves its value in Pi. `--pi-options-prune`, or **Remove cleared settings from Pi** in the dashboard, removes the fields Skillshare wrote that nobody has changed since.
  ```bash
  skillshare mcp edit docs --pi-options '{}' --pi-options-prune --no-tui
  ```

#### Dashboard

- **Check MCP servers** — the MCP page and each project's MCP tab have a **Check** button in the Sync box once there are servers to check. It runs the same check as `mcp check` without `--live`, with a summary above the list and each problem under its server.
- **See every file Sync writes for a server** — **View what each Agent gets** lists the Skillshare source and every target Agent's file, with its path and format (JSON, JSONC, TOML, YAML), and shows the one you pick with syntax highlighting. It counts the files Sync writes and names the scope: global, or the project.
- **Pi settings in the server dialog** — a Pi server chooses its Pi MCP mode, built-in by default, its tool exposure, other Pi settings as JSON, and **Remove cleared settings from Pi**. When a pasted snippet holds one server, the paste tab shows the same Pi settings.
- **Tidier MCP page** — the header keeps only **Import from a target** and **Add server**; **Check** and **Backups and restore** moved into the Sync box. Each server row shows its command next to its name and the Agents it goes to as chips below, and the Pi chip shows the Pi mode. Project MCP tabs use the same rows, and the MCP dialogs are wider.

### Bug Fixes

- **Shell completion matches the CLI** — completion offered commands that did not work: `backup restore` backed up a target named `restore`, `extras mode` only printed help, `install --source` failed as an unknown option, and `hub index --audit-skills` was rejected. Those are gone. bash, zsh, fish, PowerShell and Nushell now complete `mcp check` with `--live`, `--timeout` and `--no-dns`, subcommands such as `backup files` and `audit rules`, and flags no shell offered before.
- **SSH failures say why** — installing or loading a hub over SSH that failed showed only "Could not read from remote repository.". The message now keeps SSH's reason, such as `Permission denied (publickey)`, `Host key verification failed` or an unresolved host, and a denied key suggests `ssh -T user@host`.
- **Dashboard target links open the target** — target rows on the dashboard, the Playful pin notes and the "needs attention" entries opened the target list. They now open that target's page.
- **Chinese dashboard says 目標 and 目标** — buttons and messages such as "從 target 匯入" mixed the English word into Traditional and Simplified Chinese. They now say 目標 and 目标.
- **Hubs page opens on the default hub** — with no default hub saved, the Hubs page opened with nothing selected and Skillshare Hub had no default star, although `search --hub` already falls back to it. The page now stars Skillshare Hub in that case and opens on the default hub when you have no hub of your own.
- **Find skills no longer installs when it finds nothing** — in the Install dialog, **Find skills** on a source with no skills or agents went on to install the whole source as one skill without a `SKILL.md`, and so did **Install** on such a search result. The dialog now says nothing was found and installs nothing.
- **MCP restore preview keeps its size** — switching backups resized the restore dialog while the preview loaded. It now stays the same size.

## [0.22.1] - 2026-09-30

### Bug Fixes

- **Dashboard file editor no longer crashes** — opening a file tab on a target page, such as `APPEND_SYSTEM.md` on `pi`, showed "Something went wrong — Unrecognized extension value in extension set". The dashboard bundled two copies of its code editor library; it now bundles one.

## [0.22.0] - 2026-09-30

### New Features

#### Dashboard

- **Richer markdown preview** — skill and file previews now render HTML such as `<details>`, GitHub alerts, `:::` containers, emoji shortcodes, `==mark==`, `++ins++`, `^sup^` and `~sub~`, and footnote links resolve. HTML goes through a sanitizer, since previews show skills from anyone. Tables no longer squeeze short columns until words break, file previews keep single line breaks as the tool reads them, a leading `---` block counts as frontmatter only when it is YAML, and outside links open in a new tab.
- **Failed sync targets listed first** — when a dashboard sync loses a target, the Sync page now lists each failed target with its part (Skills, Agents, Extras or Config) and error above the other warnings, marks it in the change list, and shows a warning toast instead of **Sync complete**. Each failure gets a plain-language explanation for common causes (a symlink pointing elsewhere, permission denied, a read-only disk, a file where a folder belongs, a missing path, invalid target settings). A skills symlink that points elsewhere offers **Turn on Force**. The **Last sync** card names the failed targets, and the project sync dialog shows the same list instead of a success note.

### Bug Fixes

#### Sync

- **Dashboard sync keeps going after a target fails** — the first skills target that failed stopped a dashboard sync with an error, leaving every later target unsynced and agents, extras and MCP not run. Like `skillshare sync`, the failed target is now reported as `<target>: sync failed: <err>` and the rest still sync; the operation log records it as `partial`.
- **Invalid target settings fail only that target** — one target with invalid settings, such as a path that is a file instead of a folder, or a project target without a path, stopped the whole `sync`, `sync -p` and dashboard sync before anything ran. That target is now skipped with its error and the others still sync.
- **Project dashboard works with a target that has no path** — `skillshare ui -p` would not start, and a running project dashboard failed every request, when a custom target had no `skills.path`. It now starts with a warning, and its sync lists that target as failed.
- **`sync -p agents` checks settings and is logged** — it synced agents into targets with invalid settings and left no entry in `skillshare log`. It now skips those targets, exits non-zero, and logs the run like `sync agents`.
- **Dashboard sync reports every failed target when all fail** — when every skills target failed, the dashboard showed only the first error, and agents, extras and MCP were not synced. It now lists every failed target the same way as a partial failure.
- **Agent failures counted in the operation log** — a target whose agents failed to sync was left out of the sync entry's `targets_failed`, so a partly failed sync could be logged as `ok`. The entry now counts it and lists `failed_targets`.
- **Prune failures reported** — when removing orphaned skills or agents from a target failed, `sync` and the dashboard said nothing and stale links stayed behind. They now show `<target>: agents prune failed: <err>` (or `prune failed` for skills) along with the prune's own warnings.
- **Dashboard respects symlink conflicts** — in symlink mode, a target folder linked somewhere else was replaced by a dashboard sync without the conflict message `skillshare sync` shows. Without **Force** it now fails with `conflict - symlink points to X (use --force to override)`.
- **`sync --all` exits non-zero when an extras target fails** — it printed a warning and exited 0, and with `--json` ignored extras errors entirely. It now fails in text and JSON, global and project, and the dashboard's operation log records extras failures as `partial` instead of `ok`.
- **An extras failure no longer stops a dashboard sync** — the first failed extras target ended the run: the skills result and warnings already on screen were dropped, MCP was never written, and the error named neither the extra nor the target. The sync now finishes and lists every failed extras target.
- **Missing extras source is skipped, not created** — global `sync` and the dashboard created an empty source folder for an extra whose source did not exist, which hid a misconfigured path. Every sync now skips that extra and says `Source directory does not exist: <path>`.
- **Dashboard skips extras targets the agents sync writes** — like the CLI, a dashboard sync no longer writes the `agents` extra into a target folder that the agents sync already manages, so the two stop fighting over it.
- **Context-cost warnings name the right target** — the dashboard's sync warnings showed `{target}` instead of a name, and when several targets tied, listed offenders from a different target than the one named. They now name the target, and ties go to the alphabetically first one with its own offenders.

#### Tracked repositories

- **Unreadable git status no longer treated as clean** — when skillshare could not read a tracked repo's git status (for example, a corrupt index), `update` pulled over it, `uninstall` moved it to trash after a warning, and `status` and `list` showed it as up to date. `update` and `uninstall` now fail that repo with `failed to check git status: <err>` unless `--force`, while other items in the batch still run; `status` and `list` show it as unknown with a warning, and `status --json` reports `"status": "unknown"`. The error now includes what git printed, not just `exit status 128`.
  ```bash
  skillshare update --all --force   # pull even when git status cannot be read
  ```
- **Dashboard checks private repos with your token** — the dashboard's update check fetched without the HTTPS token, so private tracked repos and skills without a pinned branch reported errors that `skillshare check` did not. It now uses the same check as the CLI.

#### Project mode

- **Project uninstall reports the real problem** — every name that could not be resolved was reported as "not found", even when it matched several nested skills or pointed at a file. Project mode now gives the same specific errors as global.
- **Project uninstall logs like global** — the operation log recorded `--help`, dry runs and declined prompts as uninstalls, always with zero succeeded. It now records one entry when the uninstall ran, with the real count, and uninstalls from the project `list` view are logged too.
- **`logs/`, `trash/` and `backups/` stay out of your project's `.gitignore`** — in projects created before 0.17.3, project uninstall added them to the project's own `.gitignore` instead of `.skillshare/.gitignore`.

#### CLI

- **`search` shows mixed-case skill names** — skills named like `MySkill`, which `install` accepts, never appeared in `search` results. Search now uses the same name rule as install.

#### Dashboard

- **Revert on ignore-file tabs** — on the `.skillignore` and `.agentignore` tabs of the Config page, **Revert** reset `config.yaml` and left the ignore file's edits in place. It now resets the file the tab has open.
- **Install dialog marks the right skills as installed** — the dialog matched installed skills by name only, so a skill with the same name from another repo showed as **Installed**. It now matches by source.
- **Path overlap warning translated** — the Sync page showed the overlap warning in English in every language. It is now translated and has a button that opens Health Check.
- **Target-only resources grouped by target** — the Sync page's list of resources that exist only in targets was one comma-joined line. Each target now has its own group with its logo and count.
- **Failed target settings no longer reported as saved** — batch target updates reported success even when saving the settings failed, so an override could disappear on the next reload. They now show the error.
- **Sync page in Chinese, Japanese and Korean** — the Playful theme's handwritten note on the Sync page repeated the subtitle and, with no CJK glyphs in its font, showed as stray large text. It has been removed.
- **Selected tool in Add target** — the picked tool now uses the same highlighted row as the collect and install dialogs, instead of an inset ring.

### Breaking Changes

- **`update` and `uninstall` stop on an unreadable git status** — a tracked repo whose git status cannot be read now fails instead of being treated as clean. Scripts that run `update --all` unattended should expect a non-zero exit for such a repo, or pass `--force` to pull anyway.
- **`sync --all` exits non-zero when an extras target fails** — it used to warn and exit 0. `sync --json` also reports the failure.
- **`sync -p agents` exits non-zero for a target with invalid settings** — it used to sync agents into that target and exit 0.
- **Missing extras sources are no longer created** — sync used to create an empty source folder for an extra whose source did not exist. Create the folder yourself or fix the path in the config.
- **`POST /api/sync` returns 200 when every target fails** — the dashboard API used to answer 500 with the first error. It now returns the usual result with every failed target in `failed`; check that list instead of the status code.

## [0.21.17] - 2026-09-29

### New Features

#### Dashboard

- **Edit Pi's APPEND_SYSTEM.md on its target page** — the `pi` and `omp` target pages have an **APPEND_SYSTEM.md** tab next to the instruction file, so you can edit the text Pi appends to its system prompt without leaving the dashboard. **+** adds a tab for any other file a tool reads, including files in subfolders; the dashboard saves them under the target's `files`. Three tabs show at a time and the rest move to a menu; removing a tab doesn't delete the file. Refs: #301.
  ```yaml
  targets:
    pi:
      files:
        - SYSTEM.md
        - prompts/review.md
  ```
  Paths stay inside the tool's folder (`~/.pi/agent` for pi, `.pi` in a project). **Share with Extras** on a tab opens **Add extra** filled in with the file's folder and name, so one file can be shared across tools.

#### Extras

- **Single-file extras named after their file** — in **Add extra**, a single file's **Name** follows its file name without the extension (`APPEND_SYSTEM.md` gives `APPEND_SYSTEM`) until you type one, and the hint suggests using the file name. Refs: #300.

### Bug Fixes

- **Ticking an Agent after importing from it takes its entry over** — importing an MCP server from an Agent without ticking that Agent, then ticking it later, left the Agent's entry unmanaged: no sync was offered, nothing was recorded in `state.json`, and unticking the Agent removed nothing. The preview now lists the entry as **Take over**; sync records it as managed without changing the file, and unticking the Agent afterwards removes it. A server you turned off in the Agent itself is still never claimed. Refs: #303.
- **MCP tab for `antigravity-cli`** — the `antigravity-cli` target page now has an MCP tab, showing the `~/.gemini/config/mcp_config.json` file it shares with Antigravity.
- **Dialogs keep focus on their first field** — dialogs that focus a field when they open, such as **Add extra**, moved focus to the close button instead.
- **Pi's logo in its brand colors** — Pi now shows its coral, blue and yellow logo instead of a black mark.
- **Sync page lists you can scan** — the expanded **targets in sync** list was one comma-separated line mixing global targets with `<project>@<tool>` ones. It now shows logo chips, global targets apart from each project's. The **ignored** list is grouped by `.skillignore` and `.agentignore` and by folder, so a prefix such as `security/` is written once.

## [0.21.16] - 2026-09-29

### New Features

#### Sync

- **Targets that undo each other's sync** — when two targets sync skills into the same folder with different include or exclude filters, each sync adds what one wants and removes what the other filters out, so `sync` keeps showing the same changes. `sync`, `doctor` and the dashboard's Sync page now name both targets and the folder and suggest keeping one (`universal` when it is one of them); the Sync page has a button that stops syncing skills for the other.
  ```bash
  skillshare target codex --skills=false   # let universal alone write ~/.agents/skills
  ```
  Targets with the same filters in one folder are not reported.

#### Dashboard

- **Plugin version and logo without an Agent** — a plugin added without picking an Agent now shows its version, and the logo from its Codex manifest (`interface.logo`, up to 1MB) in the Plugins list and the add preview, instead of the default icon.

### Bug Fixes

- **No false "also reads" note for a shared folder** — a target sharing its skills folder with another, such as `codex` and `universal` both on `~/.agents/skills`, was told it also reads the other's skills and sees each one twice. It holds one copy, so the note no longer appears.
- **Duplicate skills described accurately** — the dashboard and docs said each skill "shows up twice" when a tool reads two skills folders. Pi keeps the first and warns, and Gemini and others pick one, so they now say the tool finds each skill twice.
- **Antigravity no longer counted as reading `~/.agents/skills`** — Antigravity documents only `~/.gemini/config/skills` as the desktop app's global skills folder, so switching its skills off no longer claims it still sees `universal`'s skills.

## [0.21.15] - 2026-09-29

### New Features

#### Targets

- **Switch skills off for a target** — a target can now stop syncing skills while skillshare keeps managing its agents, MCP servers and instructions. Use it for a tool that also reads another target's folder, such as Pi reading `~/.agents/skills` of `universal`, so each skill no longer shows up twice. Turning skills off saves `skills.enabled: false` and removes only the links into your source: your own skills stay, copies from copy mode are kept and listed apart, and a folder another enabled target writes to is left alone. `sync`, `diff`, `status` and `doctor` then skip the target's skills, and `analyze` counts what the folder still holds.
  ```bash
  skillshare target pi --skills=false --dry-run   # preview what is removed
  skillshare target pi --skills=false
  skillshare target add gemini ~/.gemini/skills --no-skills
  ```
  Turn skills back on with `--skills=true`; the next `skillshare sync` syncs them again. Works in project mode with `-p`.
- **10 more targets** — `autohand-code`, `fx`, `jazz`, `kimchi`, `kimi-code`, `ona`, `posit-assistant`, `qoder-cn`, `reasonix` and `zcode`, with the folders each tool also reads recorded so the dashboard can tell you when two targets overlap.
- **More vendor logos** — more targets show their vendor's colored logo instead of a letter.

#### Dashboard

- **Stop syncing skills from the target page** — **Stop syncing skills** on a target's Skills tab lists what will be removed and what stays before anything changes, and warns when other tools read the same folder and would lose those skills. The dashboard's target list shows such targets as **Skills off**.
- **See who else reads a skills folder** — a target's Skills tab names the targets that stopped syncing skills and read its folder instead, and the other folders a tool also reads, so you can spot skills that load twice.
- **Adding a target shows what it writes** — the add dialog pins `universal` as the shared folder when it is not configured yet, and for the picked tool lists every place skillshare will write: the skills folder (with a switch to add it with skills off), the agents folder, the MCP config and the instruction file.
- **Where a target's AGENTS.md comes from, in one card** — the AGENTS.md tab now shows one card with the shared file the target follows or imports, the other targets using it, and the actions for it. **Change** switches the target to another shared AGENTS.md in place; **Edit** opens the shared file. The tab opens in **Preview** when the file has content, **Show all** expands it to its full length, and **Save** appears once there is something to save.

#### Extras

- **Several single-file extras in one source folder** — in the **Add extra** dialog, a single file's **Source file** is one path: the source folder, then the file name. The folder defaults to the extra's name; click a folder another single-file extra uses, or type a new one, and each extra syncs its own file from it. Project extras now accept `--source` too, relative to the project root, so a project can share a folder the same way. Removing an extra keeps files the others still use. Refs: #300.
  ```bash
  skillshare extras init review -p --source .skillshare/extras/prompts --file review.md --target .pi
  skillshare extras init append -p --source .skillshare/extras/prompts --file append.md \
    --target .pi --as APPEND_SYSTEM.md
  ```

### Bug Fixes

- **Windows paths of single-file extras use backslashes throughout** — the Extras page showed paths such as `~\.pi\agent/APPEND_SYSTEM.md`, with a slash before the file name. Refs: #300.
- **Kilo Code's AGENTS.md location** — the dashboard suggested `~/.kilocode/AGENTS.md`, a folder Kilo never reads. The `kilocode` target now uses `~/.config/kilo/AGENTS.md` globally and `AGENTS.md` in a project, as Kilo documents; **Change location** on its AGENTS.md tab sets another path such as `~/.kilo/AGENTS.md`. Refs: #302.

### Breaking Changes

- **The `zencoder` target moved to `.agents/skills`** — `~/.agents/skills` globally and `.agents/skills` in a project, because Zencoder now documents that location and reads `.zencoder/skills` only for backward compatibility. A global config stores full paths, so an existing one keeps its path. A project config stores target names only, so it follows the new default on its next `sync -p`, which also removes the links skillshare left in the old folder; folders you made by hand there are kept.
- **`replit` is project-only** — Replit documents only the project `.agents/skills` folder, so the target no longer has a global default path. An existing global `replit` target keeps the path in your config.

## [0.21.14] - 2026-09-29

### New Features

#### Install

- **Pin a skill to a tag or commit from its web URL** — the branch, tag or commit SHA after `tree/` or `blob/` in a GitHub URL (`-/tree/` on GitLab, `src/` on Bitbucket) is now the install ref, so a pasted URL installs the version it names. `skillshare update` keeps the pin. A ref the remote no longer has, such as `tree/master/` after a rename to `main`, fails the install instead of falling back to the default branch. Branch names containing `/` are matched against the remote's branches and tags, `tree/HEAD/` links use the default branch, and `--branch` still overrides the URL. Refs: #293.
  ```bash
  skillshare install github.com/team/skills/tree/v1.2.0/skills/foo
  ```
  Skills installed earlier from a `tree/<ref>/` URL got the default branch, and `update` and `skillshare install` from config keep them there.
- **Pin hub entries** — a hub index entry whose `source` names a ref installs that revision for everyone, from `skillshare search --hub` and the dashboard's **Hubs** page. Move the pin by editing the ref in the index. Pinned skills from one repo and ref still share one clone. Refs: #293.
  ```json
  { "name": "reviewer", "source": "github.com/owner/repo/tree/v1.2.0/skills/reviewer" }
  ```

#### Extras

- **Sync any single file** — `skillshare extras init` takes `--file` to sync one file from the source folder instead of the whole folder, and `--as` to give it a different name at the targets. Use `--add-target <path> --as <name>` for a different name per target. `init` only writes the config: it does not create the source file or sync. Refs: #300.
  ```bash
  skillshare extras init pi-prompt --source ~/dotfiles/prompts --file system.md \
    --target ~/.pi/agent --as APPEND_SYSTEM.md
  ```
  In global mode, several single-file extras can share one `--source` folder, each syncing only its own file. `extras list` shows the full source and target file paths, and the `extras init` wizard asks whether to sync a folder or a single file.
- **Single files on the dashboard** — the first Extras tab is now **Folders & files** and lists every extra except shared AGENTS.md files. **Add extra** can create a single file, with a file name per target and `merge`, `copy` or `import` mode.

#### Dashboard

- **One Hubs page for browsing and building** — browsing a hub and building your own are now the same page: your hub is shown the way others will see it and edited in place. Entries can be added from any repository URL, with a branch or tag picked from the remote, and skills added from installed ones keep the version they were installed from.
- **Backups grouped by day** — **Settings → Backup** groups folder backups by day with one compact row per backup. Opening a row lists each folder with its file count and size, and its own **Restore** button. The page says what it is doing while backups load and while **Back up now** runs.

#### Docs site

- **llms.txt for AI tools** — the docs site now serves [`llms.txt`](https://skillshare.runkids.cc/llms.txt), an index of every page, and [`llms-full.txt`](https://skillshare.runkids.cc/llms-full.txt), the full English docs in one file, so you can point an AI assistant at the docs. Both are linked from the site footer.

### Bug Fixes

#### Dashboard

- **Previewing your own hub shows it** — previewing a draft from the hub builder opened an empty "pick a hub" page instead of the draft.
- **Counts of one read correctly** — labels such as "1 skills", "1 of 1 targets" and "1 backups" now use the singular in every language.
- **The dashboard no longer looks stuck after an upgrade** — restarting after `skillshare upgrade` deleted the UI files the upgrade had just downloaded, so the server fetched them from GitHub again. On a slow connection that outlasted the reconnect wait.
- **The project-mode note no longer says Backup is hidden** — **Settings → Backup** appears in project mode, scoped to the project.

### Performance

- **The Backup page opens faster** — each snapshot folder is read once for its size and file count, instead of twice.

## [0.21.13] - 2026-09-28

### New Features

#### Dashboard

- **Choose how each tool gets a shared AGENTS.md** — on **Extras → AGENTS.md**, every connected tool now has a mode dropdown: `import` adds one `@import` line and keeps your own lines, `symlink` links the file, and `copy` writes a copy. Saving the shared file in the dashboard updates the copies right away. Switching back to `import` brings back your own content, including edits you made while in `import` mode. Refs: #299.
  ```bash
  skillshare ui
  ```
- **See what a restore puts back before it happens** — turning a tool off, or **Restore all**, first shows the file the tool will have afterwards, with a diff against what it has now.
- **Move a tool's instruction file** — **Change location** on a target's AGENTS.md tab sets a custom path and file name for any supported tool, not only custom targets. A folder is refused, and the option is off while a shared file is connected.
- **Put a shared AGENTS.md in any folder** — under **Other locations**, **Add location** writes the shared file into a folder that is not in the targets list, under its own name or another one such as `instructions.md`, as `symlink`, `copy` or `import`. Each location has its own mode, and **Remove** shows the restore preview first. In a project, shared files move from the **Folders** tab to the **AGENTS.md** tab, with the same locations relative to the project root. Refs: #299.
- **Preview everywhere, and Cmd/Ctrl+S on the target page** — every box that shows an AGENTS.md has **Preview** and **Source** (or **Edit**) tabs, long lines wrap, and the shared file's card opens in **Preview**. The target page editor saves with Cmd/Ctrl+S and asks before you leave with unsaved edits.
- **Update progress** — the **Updates** tab on **Skills** and **Agents** shows a progress bar, marks the row being updated, and moves blocked or failed updates into their own section with a one-line reason.

#### Backups

- **Manage every backup from Settings → Backup** — the tab now has three sections. **Target folders** lists snapshots with a filter by target or agents, and can delete one. **Files** lists each AGENTS.md, CLAUDE.md or shared-file location skillshare backed up before rewriting it, with each version's reason (converted, `@AGENTS.md` added, edited, collected, overwritten…), a diff preview, and restore. **MCP** lists MCP config backups by agent and what each changed. The tab now appears in project mode too, scoped to the project. Refs: #299.
  ```bash
  skillshare backup files                      # files with backups
  skillshare backup files show ~/.claude/CLAUDE.md
  skillshare backup files restore ~/.claude/CLAUDE.md <id>
  skillshare backup --delete 2026-09-28_10-52-00
  ```
  `skillshare backup files` is now a subcommand; to back up a target named `files`, use `skillshare backup -t files`. In a project, `backup --list -p` and `backup --cleanup -p` work on the project's agent snapshots.

#### Extras

- **Attach a single-file extra under another name** — `--add-target` takes `--as` to pick the file name in the target folder, such as `instructions.md` in a notes folder. `skillshare extras <name> --help` now prints that extra's options.
  ```bash
  skillshare extras personal --add-target ~/work/notes --as instructions.md --mode symlink
  ```

### Bug Fixes

#### Windows

- **AGENTS.md no longer shows up as a folder** — single files were linked with directory junctions, which tools read as a folder. Files are now linked with file symlinks, and when Windows can't create them (Developer Mode off and not an administrator), skillshare writes copies and keeps them up to date instead. The dashboard marks `symlink` as unavailable and explains why. Refs: #299.
- **Junctions are recognized as links again** — since Go 1.23, a junction is no longer reported as a symlink, so skills synced as junctions could be reported as local folders by `status`, `doctor`, `sync` and the dashboard.
- **The AGENTS.md tab shows the file name on Windows** — a target's instructions tab was labeled with the whole Windows path instead of its file name, such as `CLAUDE.md`.

#### Extras

- **One shared file per linked or copied tool** — a tool whose file is a link to, or a copy of, one shared file could also be connected to another shared file. The import line was then written through the link into the other shared file, and every tool reading it picked it up. The dashboard, **Connect all**, `--add-target`, `--mode` and `sync extras` now refuse this and name the file that holds the tool.
- **Your files are kept in more cases** — an instruction file that is a link into your dotfiles is left as it was after restore; a missing end marker no longer duplicates the managed block; replacing a link you pointed elsewhere is reported and recorded; and the managed block follows the file's CRLF line endings.
- **Project imports use relative paths** — in project mode the `@import` line written into a committed `CLAUDE.md` pointed at an absolute path in your home folder. It is now relative to the file, so it works for everyone and after the project moves.
- **Clearer status for single-file extras** — `extras list -p` no longer reports correctly synced relative links as drift; an edited copy shows `modified` and can be collected from the dashboard; an invalid `mode` in `config.yaml` is no longer shown as healthy; and `sync extras --dry-run` says when an edit would be backed up and replaced.
- **`sync extras --json` exits with an error when a target fails** — it used to exit 0 while the plain output exited 1.

#### Dashboard

- **Agents backups can be restored from the dashboard** — restoring a `<target>-agents` snapshot failed with "target not found".
- **Messages about shared AGENTS.md files are translated** — warnings and errors from mode changes, connecting, moving and restoring used to appear in English in every language.
- **Empty rules folders are left out of the read order** — a target's AGENTS.md tab listed `~/.claude/rules/` even when it held no rule files.

#### CLI

- **`NO_COLOR` is honored everywhere** — `status`, `doctor`, `extras list` and about twenty other commands still printed colors with `NO_COLOR` set.

## [0.21.12] - 2026-09-28

### New Features

#### Dashboard

- **Edit the instruction files of tools that get skills through universal** — Codex, Gemini CLI, Pi and other tools read skills from `~/.agents/skills`, but none of them reads `~/.agents/AGENTS.md`, so with only `universal` as a target their own files, such as `~/.codex/AGENTS.md`, had no page. Universal's **AGENTS.md** tab now has a dropdown with each of these tools that is installed and isn't a target of its own; pick one to see and edit its file. They also appear in the shared AGENTS.md list under **Extras**, so a shared file can be connected to them. Cline and the Warp Agent CLI read `~/.agents/AGENTS.md` itself, and are now shown as sharing universal's file.
  ```bash
  skillshare ui
  ```

### Bug Fixes

#### Targets

- **Removing a target no longer takes skills away from another target in the same folder** — `codex` and `universal` both write to `~/.agents/skills`. Removing one of them turned the other's synced skills into local copies that skillshare no longer managed. The folder is now left alone while another target still uses it, and `target remove` says so, including with `--dry-run`.
  ```bash
  skillshare target remove codex --dry-run
  ```

#### Upgrade

- **A skill left owned by root now says how to fix it** — an upgrade run with sudo before v0.21.10 wrote the built-in skill as root, and every later upgrade failed with only `permission denied`. The error now includes the `chown` command that gives the skills folder back to you.

## [0.21.11] - 2026-09-27

### New Features

#### Dashboard

- **More tools' instruction files are filled in** — the instruction file tab on a target page now knows the file 22 more targets read, instead of asking you for its path. For example, `universal` reads `~/.agents/AGENTS.md`, `pi` reads `~/.pi/agent/AGENTS.md`, `copilot` reads `~/.copilot/copilot-instructions.md` and `qwen` reads `~/.qwen/QWEN.md`. A file you already have at that path shows up with no setup. The full list is in [Share one AGENTS.md across your tools](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-instructions/).
  ```bash
  skillshare ui
  ```

### Bug Fixes

#### Dashboard

- **Reloading a page other than the first one no longer hangs** — opening or refreshing an address such as `/targets/claude` asked for the page's scripts in the wrong folder, so it stayed on its loading placeholder and the browser console showed `Failed to load module script` errors.

#### Plugins

- **Opening a plugin no longer downloads its source again** — expanding a plugin to see which other Agents can take it cloned its whole repository every time, which could take tens of seconds. Skillshare now reads the copy it reviewed when the plugin was added, and downloads the source only if that copy was changed.
- **Clearer messages while a plugin source is read** — when a source can't be reached over the network, or the repository or branch doesn't exist, the dashboard now says which, instead of a general git failure. It also says when you are offline, and when reading a source or asking an Agent's CLI is taking a while. For example, `codex plugin list` can wait on Codex's remote marketplace; Skillshare waits up to 90 seconds for each Agent.

## [0.21.10] - 2026-09-27

### New Features

#### Dashboard

- **Share one AGENTS.md across your tools** — the Extras page has a new **AGENTS.md** tab. Create a shared `AGENTS.md` and choose which targets read it. Claude has no user-level `AGENTS.md`, so it gets an `@` import in `~/.claude/CLAUDE.md` and keeps its own content; Codex, Gemini and other targets get a link in place of their file, which is backed up first. Several shared files can sit side by side, such as one for personal and one for work, and targets that follow `@` imports can use more than one. **Restore** asks first, then puts back exactly what the target had before it was attached.
  ```bash
  skillshare ui
  ```
- **Edit each tool's instruction file** — every target page has a tab named after the file that tool reads, such as **CLAUDE.md**, **GEMINI.md** or **AGENTS.md**. It shows the read order, an editor, and warnings such as Windsurf reading only the first 6,000 characters. **Convert…** moves the content of `CLAUDE.md` into `AGENTS.md` by import, rename or copy, and backs up the file first.
- **`AGENTS.md` in projects** — in project mode the tab shows whether each target reads `./AGENTS.md`, and adds a small fix for tools that only read their own file, such as `@AGENTS.md` in `CLAUDE.md`.
- **Tools skillshare doesn't know** — a custom target can say which instruction file it reads, in the **Custom target** dialog when you add it or later from the same tab. The setting is saved as `instructions` on the target:
  ```yaml
  targets:
    myagent:
      path: ~/.myagent/skills
      instructions:
        path: ~/.myagent/AGENTS.md
        import: true        # the tool follows @path lines
  ```
- **See which projects a skill reaches** — on the Skills page, the **Targets** column shows your global tools as icons, followed by a folder badge with the number of projects the skill is synced into. Before, each project repeated its tools' icons, so `claude` plus two projects showed three Claude icons. Hover to see the global tools and each project's tools, including projects the skill does not reach. The **Target** filter now groups global tools and projects, with one entry per project, and in the tree view the **Targets** row shows where the selected skill actually goes. Refs: #297.

#### Extras

- **Single-file extras** — an extra with `file` syncs one file instead of a whole folder, `as` renames it per target, and the new `import` mode writes an `@` line into the target's own file instead of replacing it. `extras list` shows `modified` when a linked target was replaced by a different file, and `extras remove` puts back what each target had before the first sync.
  ```yaml
  extras:
    - name: personal
      file: AGENTS.md
      targets:
        - path: ~/.codex
        - path: ~/.claude
          as: CLAUDE.md
          mode: import
  ```

### Bug Fixes

#### Git sync

- **A pull that fails midway no longer leaves the remote's files behind** — when git could not finish a pull, for example because a file in the source folder was owned by root, the files it had already written showed up as local changes. The dashboard then blocked the next pull and suggested committing them, which would have pushed stale content back. A failed pull now restores those files and leaves your own edits alone, and a permission failure shows the `chown` command that gives the source folder back to you.

#### Upgrade

- **Upgrading with sudo no longer leaves root-owned files in your home** — when the binary lived in a root-owned folder, `upgrade` ran entirely under sudo, so the built-in skill, dashboard assets and logs were written as root and a later `git pull` of the skills source failed with `Permission denied`. Only the binary replacement now runs with sudo.
  ```bash
  skillshare upgrade
  ```

#### MCP

- **Pi servers load again with `pi-mcp-adapter` 3.0** — the adapter stopped reading Pi's `mcp.json` and now reads `mcp-adapter.json` in the same folder, so servers synced for it were ignored. Skillshare now writes them to `~/.pi/agent/mcp-adapter.json`, `.pi/mcp-adapter.json` in a project, or `mcp-adapter.json` in a Pi account's folder. The next sync removes the entries Skillshare had written to `mcp.json` and leaves your own there. If you already renamed the file as Pi's warning suggests, Skillshare keeps managing the entries you moved. `pi-mcp-extension` still uses `mcp.json`. Refs: #298.
  ```bash
  skillshare sync mcp
  ```

## [0.21.9] - 2026-09-26

### New Features

#### Dashboard

- **Turn a whole folder or tracked repo on or off** — the **tree** view on the Skills and Agents pages now shows the folders on the left and what you selected on the right. Selecting a folder or tracked repo gives one switch that enables or disables everything inside it, and each skill is listed with its own switch. Cmd/Ctrl-click adds to the selection and Shift-click selects a range, so the same switch works on any set of skills. A disabled skill is marked with a power-off icon in place of its usual one. The divider between the two sides can be dragged. Refs: #295.
- **Targets for skills in tracked repos** — setting targets on a tracked repo, one of its subfolders or a single skill in it used to be refused. It now works, and the setting is kept outside the cloned repo, so the repo stays clean and `skillshare update` keeps it. Refs: #295.
- **Filter and group by folder** — the list and cards views have a **Folder** filter, and grouping has a **Folder** option, so skills installed into a folder can be seen one folder at a time or side by side. A tracked repo counts as one folder, and skills at the top of the source are grouped under **Root**.
  ```bash
  skillshare install ~/my-skill --into frontend
  ```
- **A shorter toolbar on the Skills and Agents pages** — the Source, Status, Target and Folder filters are now chips that show only their name until set; a set filter shows its value and a button to clear it. Grouping and sorting share one menu, and expand all / collapse all in the tree view is one button.

### Bug Fixes

#### Sync

- **Skills under a dot folder are no longer reported missing** — a source skill in a folder such as `.system/` syncs to a target entry starting with a dot, which target scans skipped. `diff`, `doctor`, `status` and the dashboard reported these synced skills as missing, and `sync` did not prune them after the source skill was deleted. Refs: #294.

#### Dashboard

- **One Agent's plugin error no longer breaks the Plugins page** — when an Agent's plugin list could not be read, for example OpenCode with both `opencode.json` and `opencode.jsonc`, the whole Plugins page failed to load. That Agent now shows its error and the other Agents work as usual. Refs: #296.
- **The dashboard server no longer risks crashing under overlapping requests** — changing a skill while another request was being handled could crash the server.

## [0.21.8] - 2026-09-25

### New Features

#### Git sync

- **See what the remote has before you push** — opening the **Git Sync** page fetches from the remote, and **Pull** shows how many commits are waiting, such as **Pull 2 commits**, so a machine that is behind finds out before a push is refused.
- **`.metadata.json` conflicts resolve themselves** — every install or update rewrites `.metadata.json`, so pulling after two machines had both installed or updated skills almost always conflicted there. `pull` now merges that file skill by skill; when both machines changed the same skill, the one installed later wins. A conflict in any other file stops the pull, undoes the merge and names the files, so the repository is never left half-merged.
  ```bash
  skillshare pull
  ```

### Bug Fixes

#### Git sync

- **Pulling after both machines committed** — once this machine and the remote each had commits the other lacked, `pull` failed with `Need to specify how to reconcile divergent branches`, in the terminal and on the **Git Sync** page, and the only way out was git in a terminal. `pull` now merges the two histories. `skillshare update` does the same for a tracked repository with local commits.
- **A refused push offers Pull** — when the remote had newer commits, **Push** on the **Git Sync** page showed git's raw `rejected` message. It now says to pull first and puts a **Pull** button next to the error.

## [0.21.7] - 2026-09-24

### New Features

#### Extras

- **Pull target edits back with `extras collect --force`** — `collect` skipped every file that already existed in source, so an edit made directly in a copy-mode target could not be brought back. With `--force`, the target's version overwrites the source file. Files whose content already matches are still skipped, and with `flatten`, a second target file with the same name is reported instead of overwriting the first.
  ```bash
  skillshare extras collect rules --force
  ```
  Refs: #291.

### Bug Fixes

#### Extras

- **Collecting from a copy-mode target keeps its files** — `extras collect`, in the terminal and in the extras list, replaced each collected file in the target with a symlink, even when the target uses `mode: copy`. Copy-mode targets now keep their files. Refs: #291.
- **`--from` finds the target however its path is written** — `extras collect --from` only matched a target written exactly as in `config.yaml`, so `/Users/me/.claude/rules` or a trailing slash did not match `~/.claude/rules`, and the target's `mode`, `flatten` and extension settings were ignored. Paths are now compared after expanding `~`. Refs: #291.

## [0.21.6] - 2026-09-23

### New Features

#### Targets

- **MCP servers on each target's page** — a target whose Agent has an MCP config file now has an **MCP** tab with one row per server: its endpoint, whether this Agent gets it, and whether a change is waiting to be synced. Clicking a row adds the server to that Agent or takes it out, saved right away. As on the MCP page, this only changes the source; **Sync all targets** then writes the MCP files of every target at once. Adding and importing servers, resolving conflicts and restoring backups stay on the MCP page. The Targets list also counts the MCP servers each Agent gets, next to its skills.
- **A project's MCP on its targets in project mode** — with `skillshare ui -p`, the tab lists the project's servers, which go to its own files such as `.mcp.json`. For Claude Code, OpenCode, Kilo Code and Pi, it also lists the switches that turn a global server off in this project.

#### Sync

- **MCP and project sync dialogs look like the Skills one** — the dialogs behind **Sync MCP** and **Sync project** show one row per target instead of one line per item, as the Skills sync dialog does. An MCP row names the project of the Agent file and the servers that change, with counts such as **1 to add** or **1 to turn off**. **Sync project** still says which MCP entry is in conflict and why. The Sync page keeps the item-by-item list.

### Bug Fixes

#### MCP connections

- **Quick clicks on Agent toggles no longer undo each other** — ticking two Agents in quick succession on the MCP page or a project's MCP tab could fail with an error and untick the first one again. Each toggle now waits until the previous one is saved.
- **Turning a server off in a project for another Agent is saved** — on the MCP page of `skillshare ui -p`, ticking an Agent on a server that is off in this project also listed the project's other Agents, including ones that cannot turn a server off, such as Cursor, so the save was refused. Only Agents that can turn a server off in a project are listed now.

## [0.21.5] - 2026-09-22

### New Features

#### Targets

- **A second account of Claude Code, Codex or Pi as its own target** — when you run an Agent with a second config folder, such as a work account started with `CLAUDE_CONFIG_DIR=~/.claude-work`, add that folder as a target and Skillshare works out where its skills and agents go. Codex (`CODEX_HOME`) and Pi (`PI_CODING_AGENT_DIR`) work the same way. In the dashboard, choose **Add target** → **Another account**; in the terminal:
  ```bash
  skillshare target add claude-work --agent claude --config-dir ~/.claude-work
  ```
  In `config.yaml` the target only records the folder:
  ```yaml
  targets:
    claude-work:
      agent: claude
      config_dir: ~/.claude-work
  ```
  Refs: #289.
- **Accounts receive MCP servers and plugins too** — an account's name works wherever an Agent's does in `mcp.targets`, a server's `targets`, `--target` and `--from`. Servers are written into the account's own file (`.claude.json`, `config.toml` or `mcp.json` in its folder), and plugins are installed with the Agent's CLI pointed at that folder, so one account can have a server or plugin the other does not. `skillshare mcp import --from claude-work` reads that account's servers. A Pi account needs `piExtension: pi-mcp-adapter`, since `pi-mcp-extension` always reads `~/.pi/agent/mcp.json`. Refs: #289.
- **Removing an account says where MCP still names it** — `skillshare target remove` and the dashboard remove the target, then warn when `mcp.targets` or a server's `targets` still lists it, since the next MCP sync would fail. Refs: #289.

#### Sync

- **Sync one part from its own page** — **Skills**, **Agents** and **MCP** each have a sync button that previews and writes only that part, and a project page has **Sync project**, which writes that project's skills, agents and MCP without touching global targets or other projects. The sync prompts after an update, uninstall or collect open the same dialog. Refs: #289.

#### MCP connections

- **Stop managing a server without touching Agent files** — removing a server always cleared its entries from the Agents on the next sync. **Stop managing** drops it from Skillshare and leaves the entries where they are, so the Agent keeps using it. It is the third choice in the dashboard's remove dialog and in the remove wizard:
  ```bash
  skillshare mcp remove context7 --keep-files
  ```
  Refs: #290.
- **Servers added to an Agent directly are pointed out** — the MCP page and each project's MCP tab say which Agent files hold servers Skillshare does not manage, and **Import** opens on that file. Refs: #290.
- **Pi adapter settings show on the server row** — **Direct tools** are listed on a line under the endpoint instead of only on hover. Other adapter settings only say that they are set. Refs: #289.

### Bug Fixes

#### MCP connections

- **Importing a project's conflict uses that project's files** — **Import from** an Agent on a project's conflict read the Agent's global file and wrote to the global source, so the project's own entry could never be taken over. Refs: #290.
- **Pi adapter settings survive unticking Pi** — unticking every Agent dropped **Direct tools** and `piOptions`, so ticking Pi again reset the adapter. Refs: #289.
- **The MCP page no longer crashes on a cold load.**

#### Dashboard

- **Skill and agent pages list their projects** — the Skills and Agents lists counted project targets, but the detail page only showed global ones. It now lists each project the skill or agent syncs to, with a link.
- **Skills and Agents sync previews match the Sync page** — they showed changes on targets that were already in sync.

#### CLI

- **Global setup errors say they are about the global config** — `skillshare ui -g` with a missing source folder said "source directory not found" and suggested `skillshare init`, which then said Skillshare was already initialized. The error now says it is the global source, names the config, and tells you to create the folder or point `sources.skills` at an existing one.

## [0.21.4] - 2026-09-22

### New Features

#### Plugins

- **Add a plugin without choosing Agents yet** — leave every Agent unticked in **Add plugin** to keep the plugin in Skillshare without installing it anywhere. Its row shows **No Agents yet**; tick an Agent there when you want it, and the install preview uses the source as it is then. In the terminal, leave out `--target`, or confirm the picker with nothing selected:
  ```bash
  skillshare plugin add ./my-plugin --no-tui    # keep it in Skillshare; choose targets later
  ```
  A plugin added this way stays managed when its last Agent is removed. `skillshare plugin remove NAME` without `--target` removes it from Skillshare.
- **Versions are easier to see** — each plugin's version is now a tag next to its name. After **Check updates**, a plugin whose source has a new version shows `1.0.0 → 1.1.0` in the list and in the preview.

#### MCP connections

- **Keep a server without sending it to any Agent** — untick every Agent in the server dialog to keep a server in Skillshare while no Agent receives it, for example to take it out of use for a while without losing its settings. The next sync removes the entries it had, and ticking an Agent brings it back. Project servers work the same way. In the terminal, use `--target none`; in `config.yaml`, an empty list:
  ```bash
  skillshare mcp edit context7 --target none    # stays in Skillshare, written to no Agent
  ```
  Refs: #289.
- **Set other `pi-mcp-adapter` fields from Skillshare** — the adapter has per-server fields that Skillshare has no setting for, such as `excludeTools` and `approveTools`. Put them under `piOptions` and they are written into the server's entry in Pi's file as given. The server dialog has a JSON box for them under **Direct tools**, which checks that the text is a JSON object before saving. In the terminal:
  ```bash
  skillshare mcp edit github --pi-options '{"excludeTools":["*emulator*"]}'
  ```
  A field removed from `piOptions` stays in Pi's file until you delete it there. Refs: #289.

#### Upgrade

- **`skillshare upgrade` shows download progress** — on a slow connection the release download can take minutes, and the spinner alone made the upgrade look stuck. In a terminal it now shows how much has been downloaded, for the CLI and for the dashboard assets:
  ```
  Downloading v0.21.4...  3.2 MB / 9.1 MB
  ```

### Bug Fixes

#### Plugins

- **Pi and OpenCode are no longer blocked by a scoped npm name** — a `package.json` named like `@owner/pkg` made both Agents show the plugin as blocked, although they install it by path.
- **Pi packages whose `package.json` has no name can be installed** — they were blocked for Pi even when another manifest named the plugin.

#### Config

- **Installing a plugin no longer reindents `config.yaml`** — saving a plugin rewrote the whole file with 4-space indentation. Every YAML file Skillshare writes, including skill and agent frontmatter and `audit-rules.yaml`, now uses 2 spaces.

#### MCP connections

- **An entry left behind by a deleted configuration can be taken over** — when the Skillshare configuration that owned an Agent's MCP entry was moved or deleted, the conflict still said the entry was managed by another configuration and named a file that is no longer there, leaving no way forward in the dashboard. Such a conflict now reports the entry as left over, and **Import from** the Agent or **Replace with source** takes it over, in the terminal and from the conflict in the dashboard. A configuration that only cannot be read, such as one on a drive that is not mounted, still counts as present. Refs: #288.
- **OpenCode project settings kept in `.opencode/` are used** — OpenCode also reads `opencode.json` and `opencode.jsonc` from a project's `.opencode/` folder. A file kept there was ignored, and sync created a second one at the project root. It is now the file Skillshare writes to; a new file is still created at the root. Refs: #289.
- **Turning a global server off in a project follows the project's Agents** — the dashboard saved the switch with every Agent the global server reaches, including Agents the project does not use, and the list went stale when the project's targets changed. The switch now stores no Agent list and applies to the Agents the project uses that have a per-project switch. The row shows those Agents as logos and says which of the project's Agents still load the server. An entry saved by an earlier version shows **Match the project** to update it. Refs: #289.
- **The Sync page says when a change only turns a server off** — a project's switch for Claude Code is kept in `~/.claude.json`, and it read like the global server being added or removed there. Such a change now says it turns the server off or back on, and names the project. A conflict on a switch offers only **Replace with source**, since importing reads the Agent's global file. Refs: #289.

#### Dashboard

- **Agent logos show on Windows machines that report the wrong type for SVG files** — a few larger logos, Antigravity among them, appeared as broken images on some Windows machines while every other logo was fine. Refs: #289.
- **Going back to the global targets asks first** — switching a project's MCP targets back to **Same as global** saved at once and dropped the project's own list.
- **Status labels and Kilo Code messages follow the dashboard language** — **enabled** on the Skills page, **In sync** on the Extras page, the Kilo Code project warning and the error for an unreadable pasted snippet stayed in English. Refs: #289.

## [0.21.3] - 2026-09-21

### New Features

#### Dashboard

- **Analyze only the skills Skillshare manages** — when a target also holds skills you added by hand, an **Only skillshare-managed** switch on the Analyze tab hides them, and the always-loaded and on-demand totals follow the switch. The dashboard remembers your choice. Local skills now show their folder name instead of an internal `synced/…` path; the full path appears on hover and in the skill's details.

### Bug Fixes

#### Plugins

- **One bad entry no longer blocks a whole marketplace** — a marketplace failed to load when one entry used an external source, pointed at a missing folder, or had an invalid or duplicate name. Such an entry is now marked in its own row with the reason, in your language, and the rest of the marketplace can be installed. When one catalog points a plugin at an external source and another at a folder inside the repository, the folder is used.
- **Claude Code plugins without `plugin.json` can be installed** — Claude Code does not require the manifest. Such a plugin is now read from its marketplace entry and its default folders, and a `SKILL.md` at the plugin root counts as its only skill. Definitions in the entry, such as `strict: false`, `skills` and `lspServers`, and its version, are carried into the install.
- **Entries that share one folder install their own skills** — when several entries in a marketplace pointed at the same folder and each listed its own `skills`, every one of them installed the same content.
- **Plugins whose marketplace name differs from their manifest install in Claude Code** — Claude Code installs such a plugin under the marketplace name, so it is no longer blocked there. Codex refuses the mismatch, so other Agents still show it as blocked.
- **Each Agent installs from its own folder** — when a marketplace ships a separate copy of a plugin for each Agent, every Agent now gets the copy made for it instead of the first one found.
- **Broken links no longer block a plugin source** — a single broken symlink anywhere in a repository, or one that pointed outside it, made every plugin in it fail. Such links are now left out of the install and listed in a warning. They are still never followed.
- **Add plugin no longer preselects a plugin that cannot be installed** — when the only plugin in a source was blocked, the dialog chose it for you.
- **Pi's version is shown** — Pi prints its version to stderr, so the plugin list showed it without one.

#### MCP connections

- **View what each Agent gets shows a spinner while it loads** — an empty dialog gave no sign whether the preview was still coming.

## [0.21.2] - 2026-09-21

### New Features

#### Projects

- **Manage many project folders from the global config** — a `projects:` key declares a folder once, with the tools used there. `sync` then treats each one as a `<project>@<tool>` target with its own mode and filters, so you no longer list every project skills folder as a custom target or run project mode in each repo.

  ```yaml
  # ~/.config/skillshare/config.yaml
  projects:
    ~/work/project01:
      targets: [claude, codex]
      skills:
        mode: copy
        include:
          - myskill-*
      agents: {}
  ```

  ```bash
  skillshare sync --dry-run   # preview
  skillshare sync
  ```

  Tools that share a project skills folder are written once. A project whose folder is missing is skipped with a warning instead of failing the sync, and `collect` and `target` leave project targets alone. Refs: #286.
- **MCP servers for many projects in one place** — `mcp.projects` lists project roots, each with its own `targets`, `servers` and `directTools`. One `sync mcp` plans the global files and every root together, and removing a root cleans up what was written there.

  ```yaml
  # ~/.config/skillshare/config.yaml
  mcp:
    projects:
      ~/work/project01:
        targets: [opencode, pi]
        servers:
          context7:                  # off in this project only
            disabled: true
  ```

  `mcp.projects` is refused in a project config. A `disabled` entry works for Claude Code, OpenCode, Kilo Code and Pi with `pi-mcp-adapter`. For Claude Code the switch is written to `~/.claude.json`, where it keeps its per-project off list, and the server definitions in that file are left as they are. Codex is refused, because a switch for a server its global config lacks stops Codex loading its config at all. Refs: #286.
- **Projects page in the dashboard** — in global mode, a Projects page lists each project with Skills, Agents and MCP tabs. Adding a project takes a folder and its targets, and an ordinary target that already points inside a project can be converted. On a project's MCP tab, every global server has a switch that turns it off for that project.

#### Install

- **Project lockfile** — project mode now writes `.skillshare/skills.lock.json` next to `config.yaml`. The config records which skills the project wants; the lockfile records the exact commit each one resolved to. Commit both, and `skillshare install -p` gives every teammate the same commit even after upstream moves on.

  ```bash
  skillshare install github.com/team/skills --all -p   # writes the lockfile
  git add .skillshare/ && git commit -m "Add team skills"

  skillshare install -p          # a teammate gets the locked commits
  skillshare update --all -p     # moves the pins forward; commit the lockfile again
  ```

  Tracked repos are pinned too and stay on their branch, so they can still be updated. `update -p` and the dashboard move a pin only for skills whose commit changed, a tracked repo with uncommitted changes is left alone, and `uninstall -p` drops the pin.
- **Pin an install to a tag or a commit** — `--branch` now accepts a tag or a commit SHA as well as a branch name. `check` resolves tags and skips the network for full SHAs, which can never move.

  ```bash
  skillshare install github.com/team/skills --branch v1.2.0 --all
  skillshare install github.com/team/skills --branch 8f14e45fceea167a5a36dedd4bea2543ce848564 --all
  ```

  `--track` is refused with a tag or a SHA, in the CLI and the dashboard, because a tracked repo needs a branch to pull. Refs: #281.
- **Gitea and CNB sources** — skills install from Gitea (gitea.com and self-hosted) and CNB (cnb.cool), including subdirectories and web URLs. Private repos use `GITEA_TOKEN` and `CNB_TOKEN`, and self-hosted instances are listed under `gitea_hosts` and `cnb_hosts` in the config, or in `SKILLSHARE_GITEA_HOSTS` and `SKILLSHARE_CNB_HOSTS`. Subdirectory installs download through each platform's contents API and fall back to a git clone. Refs: #168.

#### MCP connections

- **`directTools` for Pi** — `pi-mcp-adapter` can register a server's tools as individual Pi tools. Set it per server, as a default under `mcp`, or per project. It accepts `true`, `false`, `search`, or tool names separated by commas.

  ```bash
  skillshare mcp add docs --url https://example.com/mcp --target pi --pi-extension pi-mcp-adapter --direct-tools search --no-tui
  ```

- **Edit MCP defaults in the dashboard** — a Defaults section on the Servers tab edits `mcp.targets` and `mcp.directTools`, which could only be changed by hand before.

#### Analyze

- **`analyze` measures what each target actually loads** — a symlink target exposes the whole source folder, so skills disabled in `.skillignore` are now counted for it and flagged `disabled`. For merge and copy targets, skills you dropped into the target folder by hand are counted and flagged `local`. Token estimates charge one token per wide character, so Chinese, Japanese and Korean text is no longer undercounted by roughly four times. The CLI, the TUI, the dashboard and the sync summary now report the same numbers.

#### Targets

- **`antigravity-cli` is its own target** — the Antigravity CLI reads `~/.gemini/antigravity-cli/skills` and never the app's folder, so as an alias of `antigravity` it received skills in a folder it does not read. A path-less `antigravity-cli:` entry in the config is now valid.
- **Convert agents for a tool that reads a different format** — an `extension:` on a target's `agents` block converts each agent while it syncs, reusing the extension mechanism of extras. The new built-in `opencode-agents` turns Claude-style agents into OpenCode agents. It keeps only the fields OpenCode documents and adds `mode: subagent` when missing. It refuses agents that limit their tools (`tools`, `disallowedTools`, `permissionMode`), because dropping those would give the OpenCode agent every tool. `codex-agents` works here too and writes `.toml` files. Converted targets are never collected back into your source. The dashboard sets the extension from the target's Agents tab, and the targets list tags a target that uses one. Refs: #267, #241.

  ```yaml
  targets:
    opencode:
      agents:
        extension: opencode-agents
  ```

#### Dashboard

- **Beautify button in the config editor** — reformats `config.yaml` in the editor, keeping comments, so the result is visible and revertable before you save.
- **Expand the config editor** — an Expand button opens the same editor and assistant panel in a near-fullscreen dialog, for files too long to read in the inline row. The dialog ignores backdrop clicks and Escape, so an edit in progress is never dismissed by accident.
- **The install dialog opens on the URL tab** — installing from a known repo URL is the common path. A `?install=search` link still opens the search tab.
- **Copy the config path from the sidebar** — a copy button appears on hover next to the truncated path.
- **Analyze follows the target in the URL** — target and project pages link straight into the analysis of that target, and project targets are listed after your own with their tool icon.

#### Documentation

- **The docs site is available in four more languages** — Traditional Chinese, Simplified Chinese, Japanese and Korean, at full parity with the English docs. The README is available in the same languages.

### Bug Fixes

- **A skill installed into a group reaches the project config** — `install <git source> --into <group> -p`, and installing a config entry that has `group:`, wrote the skill's metadata into the group folder. The skill was then missing from `config.yaml`, so teammates running `install -p` did not get it. Config installs were affected in global mode too.
- **`doctor` suggests removing the right target for a discovery overlap** — it pointed at the target that owns the shared folder, such as `universal`, and removing that hides skills from every tool reading the folder. It now suggests removing the scanning target, such as `codex`. Refs: #135.
- **Overlap warnings match each tool's documentation** — the list of extra folders each tool scans covered nine targets and had drifted. Kimi no longer gets a false warning for `~/.agents/skills`, and OpenCode, Goose, Copilot, Crush, Droid, Pi, Cline, Command Code, Deep Agents, Kilo Code, OpenClaw, OpenHands and Kode now get a warning when paired with a target whose folder they also read. Refs: #135.
- **The dashboard's Update now no longer hangs waiting for a password** — when the binary lives in a root-owned folder, the upgrade waited up to ten minutes for a `sudo` prompt nobody could see. It now fails at once and names the terminal command to run. Cached credentials and `NOPASSWD` setups still upgrade. The release download also has a timeout.
- **A rejected target save in the dashboard changes nothing** — when one field was invalid, such as a malformed include pattern, fields sent in the same save were still applied in memory and written by the next successful save.
- **The Gitea token stays on the API host** — a download URL on another origin is refused and the install falls back to a git clone, so a hostile server cannot collect `GITEA_TOKEN`.

### Breaking Changes

- **The `codex`, `goose` and `openhands` targets moved to `.agents/skills`** — `~/.agents/skills` globally and `.agents/skills` in a project, because each tool now documents that location and reads its old folder only for backward compatibility. With `universal` also configured, the old default showed every skill twice. A global config stores full paths, so an existing one keeps its path and its `doctor` overlap warning. A project config stores target names only, so it follows the new default on its next `sync -p`, which also removes the links Skillshare left in the old folder; folders you made by hand there are kept. A fresh `init` sets up only `universal` for Codex. Refs: #135.

## [0.21.1] - 2026-09-20

### New Features

#### MCP connections

- **Turn off a global MCP server in one project** — a server in an Agent's global config loads in every project. In project mode, add an entry with the same name and mark it `disabled` to stop it loading in that project only. The Agent keeps the command or URL from its global entry.

  ```bash
  cd my-project
  skillshare mcp add company-docs --disabled --target claude --target opencode
  skillshare sync mcp
  ```

  ```yaml
  # .skillshare/config.yaml
  mcp:
    servers:
      company-docs:
        disabled: true
        targets: [claude, opencode]
  ```

  Works with Claude Code, OpenCode, Kilo Code, and Pi with `pi-mcp-adapter`. OpenCode, Kilo Code and Pi get a lone switch in their project file. Claude Code takes a whole entry from one scope, so Skillshare adds the name to that project's off list in `~/.claude.json`, the one the `/mcp` panel edits; a name you turned off yourself is never claimed or removed. Other clients are refused, because a lone switch would replace the server instead of turning it off. In the dashboard, open it from the project folder, choose **Add server** and pick **Off in this project**. Refs: #286.
- **Kilo Code is an MCP client** — `kilocode` joins the supported clients, in global and project scope. It uses OpenCode's format; Skillshare writes to whichever `kilo.jsonc` or `kilo.json` already exists and creates `kilo.jsonc` when there is none. Kilo ignores a project config that holds an environment reference, so `fromEnv` and `bearerToken` are refused for Kilo in project mode before anything is written. Refs: #287.
- **Claude Code local scope servers are reported** — a server added with `claude mcp add` and no `--scope` wins, whole, over one of the same name in `.mcp.json` or the user scope. In project mode the plan and the dashboard now point out such a server next to the entry it hides, with the command that removes it. The sync is not blocked.
- **Limits of each client are caught before the write** — Claude Code skips the reserved names `workspace`, `claude-in-chrome` and `computer-use`, and reads its own credentials such as `ANTHROPIC_API_KEY` as empty in a remote server's `url` and `headers`; both are now refused for Claude with a message that says what to do. The dashboard's config preview refuses the same things saving would.

#### Dashboard

- **Manual only skills are visible** — a skill with `disable-model-invocation: true` carries a **manual only** tag in the skills list, on its tile and on its detail page, matching the badge the list TUI shows after `M`. Refs: #283.
- **Add field explains each frontmatter field** — the skill editor's **Add field** menu shows what each field does under its name, instead of bare keys such as `context` and `shell`.
- **Agent icons in the MCP import picker** — the Agent dropdown in **Import from a target** shows each Agent's logo. Factory, LM Studio, Kilo Code and Claude Desktop now have their own icons.

### Bug Fixes

- **The MCP page detects Agents that have no MCP file yet** — detection looked only for the MCP file, so a fresh Claude Code install showed as not detected. An Agent's own settings folder now counts. In project mode the page listed every Agent as detected; it now lists those with a project MCP file or a global install.
- **Cline's MCP settings are found again** — Cline moved its settings to `~/.cline/data/settings/`, shared by the VS Code extension, the CLI and the SDK. Skillshare now writes there and honors `CLINE_MCP_SETTINGS_PATH`, `CLINE_DATA_DIR` and `CLINE_DIR`. The old VS Code extension path is still used while it is the only one that exists.
- **An MCP entry owned by a deleted config can be taken over** — an entry managed by a Skillshare config that was later moved or deleted stayed blocked forever as "managed by another Skillshare config". An explicit import or replace now takes it over, and the conflict names the owning file.
- **`--disabled` is refused where it does nothing** — only `mcp add` uses the flag, but the other `mcp` commands and `sync mcp` accepted it and ignored it.
- **A relative `XDG_CONFIG_HOME` is ignored for MCP paths** — as the XDG specification requires, instead of producing a path relative to the current directory.
- **Skill detail and new skill pages** — the file viewer no longer leaves a gap above it when it sticks while scrolling, and the info column beside a long `SKILL.md` stays in view.

### Breaking Changes

- **The `kilocode` skills target moved to `.kilo/skills`** — from `.kilocode/skills`, in both global and project scope, because that is the only location Kilo Code's documentation lists now. Run `skillshare sync` once to write skills to the new location.

## [0.21.0] - 2026-09-19

### New Features

#### MCP connections

- **Define an MCP server once, sync it into each Agent's own config** — a server lives in `config.yaml` (or a separate `mcp.yaml`) as a portable definition, and `sync mcp` writes it in the format each client expects: JSON, JSONC, TOML or YAML, under that client's own key. Nothing is started or proxied; Skillshare only manages the settings.

  ```bash
  skillshare mcp add                                   # guided: URL, command, or pasted JSON
  skillshare mcp add docs --url https://example.com/mcp --target claude --target cursor
  skillshare mcp import docs --from claude --sync      # adopt what an Agent already has
  skillshare sync mcp --dry-run                        # preview every file that would change
  skillshare sync mcp
  ```

  Supported clients: Claude Code, Codex, Cursor, VS Code, OpenCode, Grok CLI, Antigravity, Amp, Claude Desktop, Cline, Copilot CLI, Factory, Gemini CLI, Goose, Junie, Kiro, LM Studio, Warp, Windsurf and Pi. Global and project scope are both supported where the client has them. `skillshare mcp` without arguments opens a TUI for browsing, editing and syncing.

- **Secrets stay out of the files** — headers and environment values are written as `{fromEnv: VARIABLE}` references and rendered in each client's own reference syntax. On import, literal tokens and values carrying a URL password, such as database DSNs, are turned into references, and credential-like arguments are flagged.
- **Only entries Skillshare wrote are ever changed** — ownership is tracked per entry. An entry you wrote by hand is left alone even when it matches the source, and stays that way until you `import` it. A managed entry edited in the Agent is reported as a conflict instead of being overwritten, Agent-only fields such as timeouts survive a sync, and previews stay valid while an Agent rewrites unrelated settings in the same file.
- **Backups and restore** — every write backs up the Agent file first, keeping the newest 20 per file.

  ```bash
  skillshare mcp restore BACKUP_ID --dry-run
  skillshare mcp restore BACKUP_ID
  ```

- **Pasted snippets are recognised by shape** — `mcp import --file` and the add wizard pick the client format from the top-level key, so VS Code `servers` and OpenCode `mcp` snippets copied from a provider's docs import without naming a client. TOML still needs `--from`, because Codex and Grok share the format.
- **Pi through a chosen extension** — Pi has no built-in MCP support, so a server targeting Pi names the third-party package that reads the file, `pi-mcp-adapter` or `pi-mcp-extension`. Install one in Pi yourself; Skillshare only writes its configuration.

  ```bash
  skillshare mcp add docs --url https://example.com/mcp --target pi --pi-extension pi-mcp-adapter --no-tui
  ```

- **`sync --all` includes MCP** — it syncs skills, agents, extras and MCP settings together, and `--json` reports the MCP plan alongside the rest.

#### Plugins

- **Install a complete plugin and choose which tools receive it** — a plugin bundles skills, hooks, MCP settings and scripts that only work together. `plugin` keeps that package intact and installs it through each Agent's own CLI, so native formats and unrelated installations are untouched.

  ```bash
  skillshare plugin add                                          # guided: source, plugin, targets, review
  skillshare plugin add owner/repo --target claude --target codex --no-tui
  skillshare plugin import review@team --from claude             # adopt an existing native installation
  skillshare plugin disable review --target codex                # save the selection only
  skillshare sync plugins --dry-run                              # then install or remove on sync
  skillshare plugin check review
  skillshare plugin update review --target claude
  ```

  Install targets: Claude Code, Codex, Cursor, Antigravity Desktop, Antigravity CLI, GitHub Copilot CLI, Pi and OpenCode. Grok Build supports import and removal, with install and trust handled in Grok. Sources can be a local folder, `owner/repo` or an HTTPS Git URL, pinned with `--source-ref`. Plugins are synced with `sync plugins` and are not part of `sync --all`.

#### Dashboard

- **Redesigned in two styles** — the dashboard now comes in **Clean** and **Playful**, each in light and dark, switched from the theme menu. Skills and Agents are separate pages that hold their own install, search, updates and trash, and General, Backup, Log, Health, Extensions and Files are collected into one Settings page. Old routes redirect.
- **MCP page** — one row per server with its Agents as toggles. Add a server from a URL, a command, a pasted snippet or a file, or import what an installed Agent already has. Conflicts explain their cause and offer Import or Replace, backups are grouped by day with a preview before restoring, and **View what each Agent gets** shows exactly what each Agent would receive, including edits you have not saved yet. MCP settings are only served when the dashboard is opened through `localhost` or an IP address.
- **Plugins page** — one row per plugin with its Agents as toggles, the same as MCP. Expanding a row also lists the other Agents the source supports, so ticking one previews an install there. **View files** opens a read-only browser of the local copy Skillshare reviewed.
- **Hubs page** — browse a hub and install from it, or assemble your own index from installed skills, validate it and export it, all at `/hubs`. A draft can be browsed straight away, before it is hosted anywhere.
- **Filter skills and agents by target**, and a **Remote** source filter for skills from GitLab, Gitea, self-hosted and SSH sources, which previously matched no filter. Refs: #278.
- **Config editors explain themselves** — the Files tab and the audit rules editor keep a panel beside the YAML showing what the field under the cursor does and the unsaved changes. Audit rules also get a **Test** tab that runs a rule's regex against pasted lines.
- Dashboard source counts link to their pages, and plugins are counted alongside the other kinds.

#### Audit

- **Findings accepted with `--force` are remembered** — a skill that legitimately quotes attack strings, such as a security scanner or red-team notes, no longer has to be forced on every `update --all`. The accepted findings are recorded for that skill, and later updates treat the same rule matching the same text as acknowledged. Any new finding, or the same rule matching different text, still blocks. Refs: #279.

#### List TUI

- **Press `M` to make a skill manual only** — toggles `disable-model-invocation` in the selected skill's `SKILL.md`, so the skill stays installed and can be invoked by name but is no longer loaded by the model on its own. The detail panel shows a **manual only** badge. For a tracked or installed skill the TUI asks first, because the edit counts as a local change; pressing `M` again restores the file exactly. Refs: #283.

### Bug Fixes

- **`sync --dry-run agents` and `sync -g extras` no longer sync skills** — the resource kind was only recognised as the first argument, so putting a flag before it silently synced skills instead. The kind now matches in any position.
- **A mistake in the `mcp` section no longer breaks every command** — an invalid MCP setting is reported by MCP commands and the config editor, while `sync`, `install`, `list` and the dashboard keep working.
- **Web UI: saving the config keeps flow-style YAML** — a hand-written `targets: [claude, codex]` is no longer expanded into a block list on every save. Indentation is still normalised and comments are preserved.

## [0.20.29] - 2026-09-11

### Bug Fixes

- **`install -p` no longer removes the skills it just installed from `config.yaml`** — a project install driven by `config.yaml` reconciled against the metadata it had read before installing, so a skill that arrived as a plain copy — any source pointing at a subdirectory, which has no `.git` to fall back on — looked absent and its entry was pruned from the declarative list. The install itself reported success and the skill and its metadata landed correctly, but the entry was gone from `config.yaml`, which broke the `install -p && sync` flow for anyone setting the project up from a clean checkout. Metadata is now re-read from disk before reconciling, which also covers project-mode installs made through `search`. Refs: #280.

## [0.20.28] - 2026-09-09

### New Features

- **Visible `skillshare/` project directory** — repositories that treat skills as reviewable content, rather than tool state, can now use a visible `skillshare/` directory instead of the hidden `.skillshare/`.

  ```bash
  skillshare init -p --visible    # create skillshare/ instead of .skillshare/
  ```

  Everything lives inside whichever directory is in use: `config.yaml`, `skills/`, `agents/`, `extras/`, and the operational `trash/`, `backups/` and `logs/`. Detection checks `.skillshare/config.yaml` first and `skillshare/config.yaml` second, so existing projects are unaffected and `.skillshare/` wins when both exist. `init -p` without the flag still creates `.skillshare/`. To move an existing project, run `mv .skillshare skillshare` followed by `skillshare sync -p` to repair target symlinks, and update any `sources` paths in `config.yaml` that name `.skillshare/` explicitly. Refs: #256.

### Bug Fixes

- **`.skillignore` matches again when the file uses CRLF line endings** — rules were compiled with a trailing carriage return that could never equal a path segment, so a `.skillignore` saved on Windows silently ignored nothing while `status` still reported its patterns as loaded. Editing patterns was affected the same way: existing entries were not found, adding one duplicated it, and removing one failed. A regression since v0.17.4. Refs: #275.
- **A directory symlink inside a skill no longer discards every file hash** — hashing stopped at the first entry that could not be read as a file and threw away the hashes computed so far, which left installed skills without the metadata used to detect local edits. Directory symlinks are skipped; file symlinks are still hashed, and real failures such as broken links still surface. Refs: #272.

## [0.20.27] - 2026-09-02

### New Features

- **Agents can declare which targets they belong to** — a `targets` list in an agent's frontmatter now restricts that agent to the listed tools, the same way `metadata.targets` works for skills. Agent files are copied verbatim and Claude Code, OpenCode, Cursor and Copilot read different frontmatter fields, so this lets you keep a per-tool variant of the same agent side by side. Agents without the field still sync everywhere; target aliases such as `claude-code` match `claude`. Applies to `sync agents`, the dashboard, `doctor` and the target summary. Refs: #267.

  ```yaml
  # ~/.config/skillshare/agents/reviewer.md
  ---
  targets: [claude]
  tools: Read, Write, Bash(git log)
  ---

  # ~/.config/skillshare/agents/reviewer-opencode.md
  ---
  targets: [opencode]
  mode: subagent
  permission: { read: allow, write: allow, bash: ask }
  ---
  ```

### Bug Fixes

- **`update --all` no longer deletes a skill when the audit blocks its update** — when several skills from one repository were updated together, the existing skill directory was removed before the new content was copied in, and an audit block then cleaned up the new content as well, leaving nothing on disk and a stale metadata entry that later updates could not find. Grouped updates now stage the new content in a temporary directory, run the audit there, and only swap it into place once the audit passes, matching what `update <name>` already did. If a skill was already lost this way, `skillshare update <name> --force` reinstalls it. Refs: #271.
- **Batch updates and the web UI follow the branch a skill was installed from** — `update --all` cloned the remote default branch for skills installed with `-b <branch>`, so they were reported stale or downgraded, and `--prune` moved them to trash. The dashboard's check and update did the same, comparing against the remote HEAD instead of the installed branch. All of these now group by repository and branch and fetch from the installed branch. Refs: #268.
- **Windows drive-letter paths install as local skills** — `skillshare install D:\skills\my-skill` failed with `unrecognized source format` because only paths starting with `/`, `~`, `./` or `../` were treated as local. Drive-letter paths (`D:older`, `C:/Users/...`) and backslash-relative paths (`.\skill`) are now recognised in the CLI and the dashboard install page. Refs: #269.
- **Dashboard labels non-GitHub git sources as Remote** — skills installed from Gitea, GitLab, self-hosted or SSH sources showed a **Local** badge because only GitHub metadata types were recognised. Any non-local git source now shows **Remote**, matching `skillshare list`. Tracked repos and GitHub sources keep their existing badges. Refs: #270.

## [0.20.26] - 2026-08-27

### Bug Fixes

- **`update --force` can override the security audit again** — when an update was blocked by an audit finding, the error told you to pass `--force`, but the flag never reached the audit gate. Update stages new content in a temporary directory and had to leave its internal overwrite flag off to keep the gate active, which discarded your `--force` along with it, so a flagged update could not be applied short of `--skip-audit` — which turns scanning off entirely. `--force` now reaches the gate on every update path: regular skills, tracked repos, agents, and the web UI's Force Retry button.

  ```bash
  skillshare update my-skill --force       # apply despite audit findings
  skillshare update my-skill --skip-audit  # skip scanning entirely
  ```

  Audit *scan failures* stay fail-closed regardless of `--force`: accepting findings you have seen is not the same as proceeding when the scanner could not run.

- **`install --json` no longer bypasses the security audit** — `--json` set the internal overwrite flag so installs could run non-interactively, and the audit gate read that same flag, so JSON-mode installs silently accepted content that would have been blocked interactively.
- **Grouped batch updates are covered by the audit gate** — updating several skills from a single repository inherited the same overwrite flag and skipped the block threshold.
- **Web UI: Force Retry only appears where it can help** — a failure such as `failed to remove existing skill: ... permission denied` offered a Force Retry button that retried with force and failed identically, with no hint of what would actually fix it. The button now shows for audit blocks and for pulls the server would retry with force, and is hidden elsewhere.
- **Web UI: update errors no longer quote CLI flags** — messages ending in `Use --force to override or --skip-audit to bypass scanning` were rendered verbatim in the dashboard, where there is no command line to type them into.

### Breaking Changes

- **A `--force` at install time no longer exempts later updates from the audit gate** — `--force` is a per-command decision. A skill installed with `--force` is scanned again on its next `update`, and needs `--force` (or `--skip-audit`) again to apply findings at or above the block threshold.

## [0.20.25] - 2026-08-10

### Bug Fixes

- **Batch updates no longer mark skills under target dot-directories as stale** — skills installed from subdirectories such as `.claude/skills/...` or `.codex/skills/...` were skipped by discovery during batch updates and reported as deleted upstream, which could incorrectly suggest `--prune`. Batch update now resolves the requested subdirectory directly before declaring it missing. Refs: #261.
- **Explicit target dot-directory installs discover their requested content** — commands such as `skillshare install user/repo/.claude` no longer return zero results just because `.claude` is normally excluded from repository-wide discovery. Explicitly requested roots are scanned for both skills and agents, while repository-root scans continue to skip synced target copies.
- **Project commands preserve existing projects when `config.yaml` is missing** — when `.skillshare/` already contains skills or agents but `.skillshare/config.yaml` has disappeared, project-mode commands now stop with recovery guidance instead of silently re-initializing an empty config, dropping target configuration, and leaving stale links. Fresh projects and shared repositories that intentionally gitignore `config.yaml` still initialize automatically.
- **Windows can uninstall nested skills again** — uninstalling a skill stored under a folder no longer passes Windows backslashes into the trash-name validator and fails with `trash name must not contain backslash`. Nested skill names are normalized to slash-separated paths in global and project mode without weakening traversal checks. Refs: #264.

## [0.20.24] - 2026-08-03

### Bug Fixes

- **Gemini CLI and Antigravity are separate targets again** — `gemini` had been folded into `antigravity` as an alias, but the two runtimes read different global skill directories, so one of them was always pointed at the wrong path. Antigravity now resolves to `~/.gemini/config/skills` (global) and `.agents/skills` (project); `gemini` is a target of its own resolving to `~/.gemini/skills` and `.gemini/skills`, with `~/.agents/skills` and `.agents/skills` still scanned as fallbacks. Aliases: `gemini-cli` for Gemini CLI, `antigravity-cli` for Antigravity. Refs: #255.

  ```bash
  skillshare target add gemini        # Gemini CLI
  skillshare target add antigravity   # Antigravity
  skillshare sync
  ```

  Antigravity's skill scanner also skips symlinked skill directories entirely — silently on macOS and Linux, and as `Incorrect function` on Windows. The troubleshooting docs now cover this along with its two workarounds: switch the target to `copy` mode, or point Antigravity at your skillshare source directory via its Skill Custom Paths setting.

- **`doctor`'s symlink compatibility hint is deterministic** — the hint chose its example target by iterating a map, so the same config produced a different suggestion on each run, and it could name a target whose runtime handles symlinks fine — telling users to switch something that was not broken. The example is now drawn only from the targets known to skip symlinked skill directories, in a fixed order. The sync docs are also corrected: the hint is printed by `doctor`, not `sync`.

### Breaking Changes

- **`gemini` no longer resolves to the Antigravity target** — configs that used `gemini` (or `gemini-cli`) to reach Antigravity now sync to Gemini CLI's own directory instead. Use `antigravity` for Antigravity. Existing `antigravity` targets change global path from `~/.gemini/skills` to `~/.gemini/config/skills`; run `skillshare sync` after upgrading.

## [0.20.23] - 2026-07-30

### New Features

- **`list --status` filters enabled or disabled entries outside the TUI** — the enabled/disabled filter previously existed only inside the list TUI (`s` key, `status:` search tag), so scripts had to know that `disabled` is omitted for enabled entries and filter the JSON themselves. `--status` combines with the pattern and `--type` using AND semantics, and works in project mode and for agents.

  ```bash
  skillshare list --status disabled
  skillshare list --status enabled --json
  ```

  `--status all` is the default and produces output identical to omitting the flag. Refs: #244.

### Bug Fixes

- **Backups no longer copy your source, and old snapshots are pruned automatically** — pre-sync backups followed merge-mode skill symlinks and copied the resolved content, so every snapshot duplicated the source directory, including files kept out of Git but still present on disk such as model weights, `.venv`, and browser profiles. Retention only ran when `backup --cleanup` was invoked by hand, so nothing removed the accumulating snapshots; backup directories could reach hundreds of gigabytes until `sync` failed with `no space left on device`. Snapshots now capture only local target content, and the existing retention policy (10 snapshots, 30 days, 500 MB) runs after every automatic backup. Refs: #252.
- **`backup --cleanup` keeps the newest snapshot when it exceeds the size cap** — a single snapshot larger than the 500 MB cap previously removed every backup including the most recent one, leaving no restore point at all.
- **Backup previews and cleanup now agree** — `backup --cleanup --dry-run` no longer reports that it will delete an oversized newest snapshot when the actual cleanup keeps it. Retention also counts only snapshots that remain after cleanup, avoiding unnecessary deletion of older snapshots that still fit.
- **Failed backups are discarded instead of appearing as restore points** — if a file cannot be copied, the partial snapshot is removed and cannot consume the newest retention slot. Automatic cleanup failures are now shown as warnings instead of being silently ignored.
- **Targets holding only symlinks no longer produce empty snapshots** — an empty restore point consumed a retention slot and evicted older snapshots that did have content.
- **`doctor` handles linked target directories correctly** — in project symlink mode the valid link `../.skillshare/skills` was reported as pointing to the wrong location, because relative links were resolved against the current working directory instead of the link's parent. Symlink-mode targets no longer report every source skill as a duplicate target-local copy, while copy-mode targets reached through a symlink still report real duplicate copies. Refs: #251.

### Breaking Changes

- **Backups capture local target content only** — merge-mode skill symlinks are skipped, so restoring a target recovers its local skills and then needs `skillshare sync` to recreate symlinks for synced skills. Copy-mode targets are unaffected because they hold real files. The documented backup location is also corrected to `~/.local/share/skillshare/backups/` (the XDG data directory), which had been listed under `~/.config` in some places.

## [0.20.22] - 2026-07-21

### New Features

- **Grok CLI target** — added xAI's Grok CLI as a built-in target, syncing skills to `~/.grok/skills` (global) and `.grok/skills` (project), with legacy fallback to `~/.agents/skills`. Aliases: `xai`, `grok-cli`.

  ```bash
  skillshare target add grok
  skillshare sync
  ```

### Bug Fixes

- **`collect --json` no longer forces overwrites** — JSON mode previously implied `--force`, silently overwriting existing skills and agents in the source. It now still skips confirmation prompts but keeps the overwrite guard, so existing resources are preserved unless `--force` is passed.
- **`uninstall --json` no longer bypasses the uncommitted-changes guard** — JSON mode previously implied `--force`, removing tracked skills that had uncommitted changes without warning. Dirty repositories now return a structured error unless `--force` is passed.
- **Batch uninstall explains all-dirty failures** — uninstalling multiple tracked skills where every repository has uncommitted changes now reports why nothing was removed instead of failing without explanation.
- **Editing a skill's source no longer rewrites the source directory's Git remote** — in the dashboard, changing a nested skill's source URL could overwrite the `origin` of the skills source directory itself when that directory is a Git repository (for example, backed by Git Sync). Source edits now only affect a tracked skill's own repository.
- **Freshly installed skills show their correct source instead of "Local"** — after installing a skill from the dashboard, the resource now immediately shows its GitHub source and type, rather than appearing as a local skill with an empty source until the server was restarted.

## [0.20.21] - 2026-06-29

### Bug Fixes

- **Copy-mode sync respects file ignore patterns** — `sync` in `copy` mode now skips configured `ignore:` artifacts, plus default `.DS_Store`, `.git/`, and `__pycache__/` files, across CLI sync, diff, and dashboard sync/diff paths. This prevents ignored cache and build artifacts from being copied into targets or changing copy-mode checksums.
- **Global sync uses the default skills source when source is omitted** — global configs that only define targets now work with `skillshare sync --global`, using the default `~/.config/skillshare/skills` source instead of failing with `source path is empty`. Refs: #238.

## [0.20.20] - 2026-06-19

### Bug Fixes

- **Self-managed GitLab URLs with deep project paths install correctly** — generic HTTPS sources such as `https://domain.com/dir1/dir2/dir3/dir4` now retry deeper repository boundaries when the initial `dir1/dir2` clone is not a Git repository, so nested GitLab projects can install without adding `.git` or configuring `gitlab_hosts`. Authentication, SSL, branch, and network errors still fail directly instead of retrying unrelated paths.

## [0.20.19] - 2026-06-17

### Bug Fixes

- **`init` shows shared skills directories as a single universal target** — when a detected CLI uses the shared `~/.agents/skills` directory, `skillshare init` now presents the shared directory guidance instead of listing each matching CLI target separately.
- **Trash restore handles nested entries and current-directory restores again** — nested trashed skills returned by `skillshare trash list` can be restored with their slash-separated names, and restoring to `.` no longer fails the path safety check while sibling-prefix escapes are still rejected.

## [0.20.18] - 2026-06-16

### Bug Fixes

- **Dashboard project-root installs are rejected before copying into themselves** — the web dashboard now rejects project-mode installs where the local source resolves to the project root, matching `skillshare install ./ -p` and returning the same "install a specific skill subdirectory" guidance instead of recursively copying `.skillshare/skills` into itself.
- **Trash operations reject traversal-style names** — moving skills or agents to trash, restoring from trash, and automatic trash cleanup now validate trash-relative names and destination paths before filesystem writes or removals. Traversal segments, absolute paths, backslashes, NUL-containing names, and empty names are rejected while safe nested names like `org/team-skill` and `demo/my-agent` continue to work.

## [0.20.17] - 2026-06-15

### Bug Fixes

- **`update --all` no longer crashes on unreadable metadata** — when `.metadata.json` is corrupt or unreadable, update now reports a metadata warning and continues scanning local skills instead of panicking with a stack trace.
- **Project root installs are rejected before copying into themselves** — `skillshare install ./ -p` now fails with a clear message telling users to install a skill subdirectory, preventing `.skillshare/skills` from recursively copying into itself on Windows.

## [0.20.16] - 2026-06-15

### Bug Fixes

- **Repository subdir installs reject traversal paths** — source parsing now rejects repository subdirectories with absolute paths, `.`/`..` segments, backslashes, NUL/control characters, or encoded traversal before install and download flows use them. Inputs such as `github.com/owner/repo/../../etc/passwd` and unsafe blob `SKILL.md` paths now fail instead of resolving outside the repository boundary. Refs: #224.
- **Metadata files stay readable after atomic saves** — install and update operations now write `.metadata.json` with `0644` permissions, so Git and other tooling can read metadata after Skillshare replaces the file.

## [0.20.15] - 2026-06-14

### Bug Fixes

- **Git branch refreshes now fail visibly when fetch fails** — the dashboard no longer serves stale remote branches or continues a checkout after `git fetch` fails. Branch listing and checkout requests now report the fetch failure so users can fix connectivity, authentication, or remote problems before switching branches.
- **Source URL edits keep metadata and remotes in sync** — updating a tracked skill or agent source now updates the Git remote before saving metadata. If the remote update fails, the API returns an error and leaves the existing source metadata unchanged instead of reporting success with an old on-disk remote.
- **Target removal preserves config when cleanup fails** — removing a target from the dashboard now stops if Skillshare cannot inspect the target, remove the target symlink, remove the target manifest, or clean managed symlinks. The target remains configured so users can fix the filesystem issue and retry instead of losing the target entry.
- **Version checks handle release tag formats correctly** — update checks now accept versions with a leading `v` prefix while still rejecting malformed version segments. Local metadata builds only advertise a release version when built from a clean exact tag; non-release builds stay in `dev` mode so update checks do not compare against commit-describe strings.
- **JSON-mode automation stays clean during cleanup warnings** — temporary Git clone cleanup failures are still logged for human-readable flows, but cleanup warnings no longer leak into `--json` stderr output.
- **Skill linting reports rule load failures instead of panicking** — malformed embedded lint rules now return explicit errors through analysis/discovery paths, and repeated lint runs keep the load error instead of losing it after the first attempt.
- **Audit finding severity dots are vertically centered** — severity indicators in the dashboard Audit findings list now align with their badges and messages.

## [0.20.14] - 2026-06-13

### Bug Fixes

- **Push failures redact token-auth URLs without losing diagnostics** — failed Git push flows now sanitize credential-bearing error output before it reaches CLI/API/UI callers, while still preserving useful Git and pre-push hook diagnostics. Refs: #214.

## [0.20.13] - 2026-06-11

### New Features

#### Web Dashboard

- **Rehydrate missing tracked repos from the dashboard** — the Updates page now shows a warning banner listing tracked repos declared in `.metadata.json` whose clone directories are missing on disk, with a one-click **Rehydrate** button that re-clones them from metadata. The Dashboard's **Update All** also warns about missing repos and points to rehydrate, instead of reporting that there is nothing to update. Refs: #212.

### Bug Fixes

- **`update --all` reports missing tracked repos in batch and project mode** — reporting a missing tracked repo previously only worked when it was the single update target; when `update --all` covered multiple items (the common case) the batch path skipped missing repos silently, and project mode (`-p`) never detected them at all. Both now surface each missing repo with a warning and a one-shot rehydrate hint:
  ```bash
  skillshare update --all          # ! _team-skills  clone directory absent
  skillshare install               # rehydrate from metadata
  ```
  `update --all --json` now carries an aggregated `missing_tracked_repos` summary (names + hint), and the per-item error is the concise `clone directory absent`. Refs: #212.

## [0.20.12] - 2026-06-11

### New Features

- **Droid syncs custom droids as agents** — the `droid` target now distributes custom droids (`.md` files with YAML frontmatter) alongside skills, mapping them to `~/.factory/droids` (global) and `.factory/droids` (project) through the existing agents sync. The target also accepts `factory` as an alias:
  ```bash
  skillshare target add factory   # same as: skillshare target add droid
  skillshare sync agents
  ```
  Refs: #213.

### Bug Fixes

- **Project-mode agent symlinks are now relative** — `skillshare sync agents` created absolute symlinks in project mode, which broke when the repository was moved or checked out on another machine. Agent symlinks now use relative paths, matching how project skill symlinks already work.
- **Factory alias syncs Droid agents correctly** — adding the Droid target by its `factory` alias now also resolves the built-in agents path, so `skillshare sync agents` writes custom droids to `~/.factory/droids` or `.factory/droids` instead of skipping the target as agentless.
- **Web UI sync respects agent filters** — syncing from the dashboard now honors target-level `agents.include` and `agents.exclude` filters, matching the CLI. Agents that become excluded are pruned from target directories on the next sync. Refs: #211.

## [0.20.11] - 2026-06-10

### Bug Fixes

- **Grouped tracked repositories rehydrate at the correct path** — when `.metadata.json` contains tracked repos installed with `--track --into <group>`, `skillshare install` now restores the missing clone at the original grouped path instead of applying the group twice and failing on paths like `anthropics/anthropics/_skills`. Refs: #212.

## [0.20.10] - 2026-06-10

### Bug Fixes

- **Missing tracked repositories are no longer silently ignored** — when `.metadata.json` declares a tracked repo but the local `_repo/` clone is missing (common after a fresh clone on another machine), `status`, `check`, `update --all`, and `doctor` now report it as missing instead of showing no tracked repos. The message points to the existing recovery path:
  ```bash
  skillshare install
  skillshare sync
  ```
  `update --all --json` now counts the missing repo as skipped and includes an item explaining the recovery step. Refs: #212.

## [0.20.9] - 2026-06-05

### New Features

- **Batch enable/disable in the web dashboard** — the Resources page now has a selection mode. Click **Select**, tick multiple skills or agents across the grid, folder, or table view (folders offer a select-all checkbox), then enable or disable them all at once from the bottom action bar. Enabling applies immediately; disabling asks for confirmation first. Works for both skills (`.skillignore`) and agents (`.agentignore`). Refs: #203.

### Bug Fixes

- Fixed `enabled: false` being ignored for tier and cross-skill audit rules — disabling one of these rules in `audit-rules.yaml` marked it disabled in the rule listing, but the scan still fired it at full severity. Both the per-skill and single-file scan paths now honor the disabled rule. Refs: #204.

## [0.20.8] - 2026-06-05

### Bug Fixes

- **SSH GitHub Enterprise hub entries inherit the hub SSH login** — when an SSH hub returns same-host GitHub or GitHub Enterprise domain-prefixed sources, search results now install them over SSH using the hub username and host. For example, a hub loaded from `acme@acme.ghe.com:Org/skills.git//hubs/team.json` can return `acme.ghe.com/Org/skills/skills/reviewer`, and Skillshare installs it as `acme@acme.ghe.com:Org/skills.git//skills/reviewer`. Explicit HTTPS/SSH sources, cross-host entries, local paths, and in-memory indexes keep their existing behavior. Refs: #196.
- **SSH skill previews can read private hub results** — previews for SSH GitHub/GHE sources now fall back to a shallow clone when no token is available or the Contents API rejects the request, so the dashboard can show full `SKILL.md` content for SSH-only private hub results instead of only index metadata.

### Performance

- **Faster `.skillignore` globstar matching** — repeated `**` patterns no longer trigger exponential backtracking, so commands that scan ignored skills stay responsive with complex ignore rules.

## [0.20.7] - 2026-06-03

### Bug Fixes

- Fixed single-skill uninstall for disabled skills in the dashboard — a skill hidden by `.skillignore` still appeared on the Resources page, but uninstalling it from the item menu or detail page could return "skill not found". Single-resource uninstall now resolves disabled skills the same way the list and batch uninstall flows do.

### Performance

- **Trash page virtualization** — the dashboard Trash page now renders long trash lists incrementally, so large skill or agent trash folders stay responsive instead of rendering every trashed item at once.

## [0.20.6] - 2026-06-03

### New Features

- **Clearer hub errors in the web dashboard** — when a hub fails to load, the Search page now names the failing hub and explains the likely cause (malformed URL, missing index file, authentication required, or invalid JSON) instead of showing a bare `HTTP 400`.

### Bug Fixes

- Fixed the skill preview showing only index metadata (name, description, tags) for skills from non-github.com hubs — the web dashboard always fetched `SKILL.md` via `api.github.com`, so GitHub Enterprise, GitLab, and other sources never rendered their full content. The preview now reads from the source's own host (the GHE Contents API, or a shallow clone for other platforms), and degrades with a clear notice when a source genuinely can't be fetched.
- Fixed the hub selector dropdown being clipped behind the search box on the dashboard's Search page.
- Fixed `skillshare audit` ignoring `.skillignore` inside tracked hub repos — skills excluded via a tracked repo's `.skillignore` were still scanned and reported. Audit now skips those skills, matching how sync and the rest of the CLI treat them.

## [0.20.5] - 2026-06-03

### New Features

- **Zed editor target** — `zed` is now a supported sync target. Add it, then sync; skills go to `~/.agents/skills` (global) and `.agents/skills` (project):
  ```bash
  skillshare target add zed
  skillshare sync
  ```

### Bug Fixes

- Fixed disabled skills failing to uninstall from the web dashboard — a skill disabled via `.skillignore` still appeared in the dashboard list but couldn't be removed, reporting "skill not found". Uninstall now resolves disabled skills the same way the list does. The CLI was unaffected.

## [0.20.4] - 2026-06-02

### New Features

#### SSH hub sources

- **Fetch a hub index over SSH** — `skillshare search --hub` and `skillshare hub add` now accept SSH URLs, so a shared hub index can live in a private or GitHub Enterprise repo that teammates reach over SSH without cloning it first. Skillshare shallow-clones the repo with your SSH keys and reads the index from it:
  ```bash
  skillshare search react --hub git@ghe.corp.com:team/skills.git
  skillshare hub add git@ghe.corp.com:team/skills.git//hubs/team.json --label ghe
  ```
  The index path inside the repo comes from the `//path` suffix and defaults to `skillshare-hub.json` at the repo root. Both scp-style (`git@host:org/repo.git`) and scheme-style (`ssh://git@host/org/repo.git`) URLs work. In the web dashboard, SSH hub sources must be saved first — the server only clones saved hubs.

#### Extras per-target management

- **Add or remove a single target on an existing extra** — `extras <name> --add-target` and `--remove-target` manage one target without recreating the whole extra. Both update config only; run `skillshare sync extras` afterward to apply:
  ```bash
  skillshare extras rules --add-target ~/.cursor/rules --mode copy
  skillshare extras rules --remove-target ~/.cursor/rules
  ```
  Removing a target leaves already-synced files in place by default. Add `--prune` to also delete the skillshare-managed files under that target — in merge mode only symlinks are removed, so your own files are preserved. Removing the last remaining target is rejected (use `extras remove <name>` for the whole extra). The web dashboard's Extras page gains matching per-target add/remove controls.

### Bug Fixes

- Fixed extension downloads in the web dashboard's Config page clearing each other's progress — starting a second download no longer wipes the first one's loading spinner; each download now tracks its own state.

### Breaking Changes

- **Removed the `extras mode` subcommand** — change a target's sync mode or flatten setting with the `extras <name>` shorthand instead (same behavior, one less command):
  ```bash
  skillshare extras rules --mode copy --target ~/.claude/rules
  skillshare extras agents --flatten
  ```

## [0.20.3] - 2026-06-01

### New Features

- **Copilot CLI agents** — agents now sync to Copilot CLI alongside skills. Copilot uses the same `.agent.md` format Skillshare already manages, so `skillshare sync agents` symlinks your agents into `~/.copilot/agents` (global) and `.github/agents` (project) with no conversion:
  ```bash
  skillshare sync agents    # now includes copilot
  ```

### Bug Fixes

- **Installing a specific skill no longer drags in every agent** — when installing from a repo that contains both skills and agents, `-s`/`--skill` now installs only the named skills (no agents), and `-s` with `-a`/`--agent` installs only the named agents. An unknown `-a` name fails the whole command up front, so automation never sees a half-completed install. `--all`/`--yes` still install everything.

## [0.20.2] - 2026-05-31

### New Features

#### Doctor remediation suggestions

- **`skillshare doctor` now suggests how to fix what it flags** — when doctor detects targets writing to a shared path, or one target's runtime discovering another target's skills, it prints a remediation suggestion next to the warning instead of only reporting the overlap. Suggestions are also included in `doctor --json` under a new `suggestions` field.
- **Suggestions point at a ready-to-run target removal command** — overlap suggestions now include the exact command to preview removing a duplicate target:
  ```bash
  skillshare target remove <name> --global --dry-run
  ```
- **Health Check page surfaces suggestions** — the dashboard Health Check page renders each check's remediation suggestions inside its expandable detail block, localized across all supported languages.

### Bug Fixes

- **GitHub Enterprise Cloud data residency hosts are now recognized** — repositories on `*.ghe.com` tenants are detected as GitHub over both HTTPS and SSH, and the GitHub API base is resolved as `https://api.<tenant>.ghe.com`, so installing and updating from data residency accounts works.
- **SSH remote URLs with a custom username now work** — clone URLs such as `acme@acme.ghe.com:org/repo.git` (any username, not only `git`) are parsed and normalized correctly when installing and updating.

## [0.20.1] - 2026-05-31

### Bug Fixes

- **Codex agent transforms reject incomplete agents** — the bundled `codex-agents` extension now fails clearly when the resolved `name`, `description`, or Markdown body is blank. Missing `name` still falls back to the source filename, and the extension docs link to Codex's custom agent schema for the required fields.
- **Dashboard Extras sync shows extension errors** — when an extras transform fails, the Extras page toast now shows the first file-level error instead of only an error count, so users can fix the specific source file without opening logs.

## [0.20.0] - 2026-05-30

### New Features

#### Git scope control (`git_root`)

- **`git_root` scope** — choose which directory `skillshare commit`, `push`, and `pull` version. The default stays your skills source, but you can point git at `agents`, `extras`, or `root` (skills + agents + extras together in a single repo). Set it during init, or switch later on an existing setup:
  ```bash
  skillshare init --git-root root      # version skills, agents, and extras in one repo
  skillshare init --git-root agents    # switch scope headlessly later
  ```
  A `root`-scope repo automatically keeps `config.yaml` out of version control (it holds machine-specific paths), and nested git repositories are detected and blocked before they would upload as empty submodules. If `git_root` points to a scope whose directory has no repo, `commit`/`push`/`pull` print a "Git root mismatch" error with the exact commands to fix it.
- **Switch scope from the dashboard** — the Git Sync page can change the `git_root` scope, set the git remote during the switch, and offers a one-click action when the scoped directory isn't a repository yet.

#### Extras extension transforms

- **`extension` field on extras targets** — convert Markdown into a tool's native format during sync, for tools that don't read Markdown. Reference extensions ship for Gemini CLI (TOML commands) and Codex CLI (TOML agents):
  ```yaml
  extras:
    - name: commands
      targets:
        - path: .gemini/commands
          extension: gemini-commands    # transforms .md → .toml during sync
  ```
  Transforms run source → target only (`extras collect` skips them), use `copy` semantics, and never overwrite a local file or directory without `--force`. The Codex agents extension maps `name`, `description`, and `model` from frontmatter.
- **Manage extensions from the dashboard** — the Config page lists installed extensions with descriptions and guards against removing one that is still in use; the Extras page and Add Extra modal include a per-target extension picker.

#### List filtering

- **Filter skills by enabled/disabled status** — press `s` in the `list` TUI to cycle All → Enabled → Disabled, or use the `s:enabled` / `s:disabled` tag to combine status with other filters. The dashboard Resources page gains the same status filter.

### Bug Fixes

- **Hardened git remote handling** — remote URLs beginning with `-` (which git could misinterpret as a flag) are now rejected when setting or adding a remote, including via `skillshare init --remote`.
- **Dashboard pull keeps skill paths correct with scoped git roots** — pulling from a `root`, `agents`, or `extras` git scope no longer causes the follow-up sync to flatten paths such as `skills/foo` into the wrong skill name.
- **Transformed extras no longer show false drift** — dashboard diff/status checks now compare transformed filenames (for example `.md` → `.toml`) consistently, and missing extension definitions surface as warnings instead of silently falling back.
- **Safer reference transforms** — bundled TOML transforms now escape control characters correctly and stop hung transform commands instead of blocking sync indefinitely.

## [0.19.24] - 2026-05-27

### New Features

#### Local skill checkpoints

- **`skillshare commit`** — create a local git commit for source skills without pushing to the remote. This is useful when iterating locally and wanting a restore point before you are ready to share changes across machines. It stages all source changes and commits them with the provided message, and it works even when no git remote is configured.
  ```bash
  skillshare commit -m "Update writing skill"
  skillshare commit --dry-run
  ```
- **Dashboard local commit action** — the Git Sync page now has a **Commit locally** button alongside **Push** and **Pull**. It uses the same commit message and dry-run preview area, but only creates the local commit and never pushes.

### Bug Fixes

- **Dashboard update results stay current after updating** — after updating skills from the dashboard, successfully updated or already-current items are now marked **Up to date** and keep their latest check status instead of falling back to **Unchecked**.

## [0.19.23] - 2026-05-26

### Bug Fixes

- **Nested GitHub-installed skills stop reappearing as updateable after update** — `skillshare update` and the dashboard Update page now refresh the stored metadata for skills installed under a subdirectory, so items such as `tools/agent-browser` no longer keep showing **Update available** immediately after a successful update
- **Dashboard Update checks are remembered between visits** — the Update page now keeps the last completed check status in browser storage and shows the previous check time, so returning to the page no longer resets every item to **Unchecked**

## [0.19.22] - 2026-05-26

### Bug Fixes

- **Web UI install now handles mixed-track repos** — installing a repository that contains both skills and agents with **Track** enabled (for example `github/awesome-copilot`) used to fail with a `tracked install is ambiguous; pass --kind skill or --kind agent` error toast and no way forward. The dashboard now opens a kind picker showing skill and agent counts; choose **Skills** or **Agents** and the install proceeds with the chosen `--kind`. Refs: #167

### Performance

- **One fewer clone when recovering from a mixed-track install** — the install API now reports skill/agent counts in the ambiguity error itself, so the dashboard no longer re-clones the repository via `/api/discover` before showing the kind picker

## [0.19.21] - 2026-05-24

### Bug Fixes

- **Update page checks now finish for nested GitHub-installed skills** — the dashboard now matches check results by relative path as well as display name, so items such as `tools/agent-browser` no longer stay stuck on `Checking` after **Check All** completes
- **Update Selected now targets nested GitHub-installed skills correctly** — selecting an item installed under a subdirectory now sends its relative path (for example `tools/agent-browser`) to the update endpoint instead of the flattened display name
- **Remote update checks no longer wait indefinitely on Git prompts** — lightweight `git ls-remote` probes now disable interactive credential prompts and time out after 15 seconds, surfacing an error in the UI/CLI instead of leaving the update check spinner running forever

## [0.19.20] - 2026-05-24

### New Features

#### Web UI process management

- **`skillshare ui start` / `skillshare ui stop`** — run the web dashboard as a managed background process instead of holding a foreground shell. Re-running `start` reuses the existing healthy process; `stop` shuts it down using the remembered host/port. The legacy foreground `skillshare ui` and the `--no-open &` shell-backgrounding workaround keep working unchanged.
  ```bash
  skillshare ui start                 # start in background, return to shell
  skillshare ui start --clear-cache   # clear cached UI assets, then start
  skillshare ui stop                  # stop the background server
  ```
- **`--app` flag** — `skillshare ui start --app` opens the dashboard in a Chromium app-mode window (Chrome, Edge, Brave) so it gets its own Dock/taskbar entry and chrome-less frame on macOS, Windows, and Linux.

#### In-place upgrade from the dashboard

- **Update without leaving the browser** — the Update dialog and the Doctor page's Version card now show an **Update now** button. The dashboard runs `skillshare upgrade` on the host, restarts the local UI server, then auto-reloads the page once the new server reports healthy. Two new endpoints back this flow:
  ```
  POST /api/upgrade   # run skillshare upgrade in place
  POST /api/restart   # restart the UI server (optional { "clearCache": true })
  ```
  If the dashboard cannot reconnect on its own, it surfaces a message asking you to run `skillshare ui start` to bring the background server back up.
- **Doctor Version card redesign** — the version section now leads with a state-coloured icon (blue when an update is available, green when up to date), shows the version delta as `current → latest`, and hides the **Update now** button when there is nothing to upgrade.

#### Installable as a Progressive Web App

- **PWA support** — the dashboard now ships a `site.webmanifest`, app icons (192px / 512px), and a minimal service worker. Browsers that support it (Chrome, Edge, Safari, Brave) let you install Skillshare as a standalone desktop app from the address bar; offline cache covers the static shell so the dashboard opens even before the local server is reachable.

## [0.19.19] - 2026-05-23

### Bug Fixes

- **Tracked root-level `SKILL.md` repos now get integrity hashes** — `skillshare install <repo> --track` for single-skill repos with `SKILL.md` at the repo root now writes `file_hashes` into `.metadata.json`, so `skillshare doctor` can verify them instead of warning that integrity checks are unavailable. Running `skillshare update _repo` also backfills hashes for already-installed tracked root-skill repos that were created by v0.19.18. Refs: #165

## [0.19.18] - 2026-05-22

### Bug Fixes

- **Tracked repos with a root-level `SKILL.md` are fully supported** — installing a single-skill repo with `--track` (where `SKILL.md` sits at the repo root, not nested) used to report `Found 0 skill(s)`, skip metadata persistence, and omit the repo from `skillshare status` and `skillshare list`. Now the install reports the correct count, writes the tracked entry to `.metadata.json`, and the repo appears in status output with `skill_count: 1`. Cross-machine recovery via metadata works again for this layout. The misleading `Run skillshare sync to distribute skills` next-step hint is also suppressed when the install produced zero skills or agents. Refs: #163
- **GitLab SSH URLs with nested subgroups now produce a clean tracked-repo name** — `skillshare install git@gitlab.example.com:org/subgroup/my-skills.git --track` previously stored the repo under a directory name containing the subgroup path; the resolved name is now just `my-skills`, matching the existing HTTPS subgroup behavior

## [0.19.17] - 2026-05-22

### New Features

- **`doctor` warns on overlapping skill paths** — `skillshare doctor` now flags two classes of duplicate-skill problems before they reach the runtime picker. Refs: #135
  ```
  ! Shared path ~/.agents/skills ← universal, warp
  ! codex will see content from: universal
      ~/.agents/skills ← universal
  ```
  The first warning fires when two enabled targets resolve to the same primary path. The second fires when an enabled target's runtime also scans a directory another enabled target writes to (e.g. Codex Desktop reads `~/.agents/skills` in addition to `~/.codex/skills`). Both checks are pure metadata — no filesystem probing, no runtime calls. `skillshare sync` also prints a one-line hint when overlap is detected, pointing back to `doctor` for the full breakdown:
  ```
  ! Skill path overlap across 2 target(s) — run `skillshare doctor` for details
  ```

### Bug Fixes

- **`cline` target path reverted to `~/.cline/skills`** — the cline target was briefly pointing at the shared `~/.agents/skills` root, which doesn't match the official cline documentation. Restored to the brand-specific `~/.cline/skills` (global) and `.cline/skills` (project). If you synced cline between v0.19.14 and v0.19.16, re-run `skillshare sync` to move skills to the correct location

## [0.19.16] - 2026-05-21

### New Features

- **Global config `sources:` map** — global mode now accepts the same `sources:` map shipped for project mode in v0.19.15. Override skills, agents, or extras source directories from a single place:
  ```yaml
  # ~/.config/skillshare/config.yaml
  sources:
    skills: ~/work/skills
    agents: ~/work/agents
    extras: ~/work/extras
  ```
  Each key is optional. Existing top-level `source:` / `agents_source:` / `extras_source:` fields continue to work unchanged; when both formats are set, `sources.<key>` wins. Fresh `skillshare init -g` writes the new `sources:` shape; existing configs are never auto-rewritten

### Behavior Changes

- **`extras init` no longer silently backfills `extras_source:`** — running `skillshare extras init <name>` on a global config without an explicit extras source used to write the derived `extras_source:` path back into your `config.yaml`. The runtime now resolves the extras parent on the fly, so the legacy field stays empty unless you set it yourself. Same change applies to the equivalent server endpoint (`POST /api/extras`)
- **`extras source <path>` writes to whichever field the user already configured** — if `sources.extras` is already set, the command updates that key; otherwise it falls back to the legacy `extras_source` field. This prevents the new value from being silently shadowed by `sources.extras`

## [0.19.15] - 2026-05-20

### New Features

- **Custom project source directories** — project mode can now read skills, agents, and extras from any directory in your repo. Useful for co-locating skill content with existing project documentation. Refs: #153, #162
  ```yaml
  # .skillshare/config.yaml
  sources:
    skills: ./docs/skills
    agents: ./docs/agents
    extras: ./docs/extras
  targets:
    - claude
  ```
  Each key is optional — omit to fall back to `.skillshare/<type>/`. Paths are resolved from the project root; absolute paths and `~` work too. `skillshare init -p` still seeds the default `.skillshare/` directories. Trash, backups, and operation logs always stay under `.skillshare/` regardless of `sources` settings

### Behavior Changes

- **Project commands fail closed on malformed `config.yaml`** — `uninstall`, `new`, `enable`/`disable`, and `check` now return `failed to load project config` instead of silently falling back to `.skillshare/skills`. With custom `sources`, the old fallback could have operated on the wrong directory. Fix any YAML errors (e.g. `targets: {}` → `targets: []`) and the command will succeed
- **Sync rejects source/target path overlap** — `skillshare sync -p` errors when `sources.skills` or `sources.agents` aliases or nests with a target path. Without this check, `sync --force` could delete the configured source directory. Common safe layout: `sources.skills: ./docs/skills` with a `claude` target (no overlap)

## [0.19.14] - 2026-05-20

### Refactoring

- **Unified Gemini and Antigravity targets** — the standalone `gemini` target has been merged into `antigravity`. The shared skill path `~/.gemini/skills` is now served by the `antigravity` target. Old names `gemini`, `gemini-cli`, and `antigravity-cli` continue to work as aliases

### Bug Fixes

- **Audit: Swift files are now scannable** — `.swift` files are included in security audit scans alongside `.go`, `.py`, `.ts`, and other source files
- **Audit: reduced false positives** — tightened publisher-claim extraction to avoid flagging ordinary phrases like "from CSV data" as organization claims. Bracket placeholders like `[Count] ([Percentage]%)` no longer trigger dangling-link warnings. Common documentation domains (MDN, Apple Developer, etc.) are excluded from external-link findings
- **Audit: uppercase URI schemes no longer flag as dangling links** — custom URI schemes like `VSCode://` are now correctly recognized as external links

## [0.19.13] - 2026-05-17

### Bug Fixes

- **Tracked skills now retain custom name and branch from config** — previously, `skillshare update` on a tracked repo re-derived the directory name from the clone URL (e.g. `_owner-repo`), ignoring any custom `name:` set in `config.yaml`. Now the resolution priority is: `--name` flag > config `name:` > URL-derived name. Similarly, a non-default `branch:` in config is now respected during clone and update. Refs: #158

## [0.19.12] - 2026-05-14

### Bug Fixes

- **Project `skills:` in config.yaml no longer stripped** — previously, project-mode commands silently removed the `skills:` section from `.skillshare/config.yaml` during an internal migration, leaving no committable record of remote skill dependencies. Now `skills:` stays in `config.yaml` as the declarative source of truth. Teammates can clone the repo and run `skillshare install -p` to install all listed skills. Refs: #157
  ```yaml
  # .skillshare/config.yaml — committed to git
  targets:
    - claude
    - cursor
  skills:
    - name: pdf
      source: anthropic/skills/pdf
    - name: review
      source: github.com/team/skills/code-review
      group: frontend
  ```
  `skillshare install <source> -p` automatically adds the skill to `config.yaml`. `skillshare uninstall` removes it. Runtime metadata (hashes, timestamps) stays in `.metadata.json` (gitignored)

## [0.19.11] - 2026-05-14

### New Features

- **`preserve_tilde_on_save`** — opt-in config flag that folds `$HOME` prefixes back to `~` when saving `config.yaml`. Keeps the on-disk config portable across machines when shared via dotfiles (stow, chezmoi, yadm, bare git repo). Refs: #155
  ```yaml
  preserve_tilde_on_save: true
  ```
  Non-home absolute paths (e.g. `/opt/shared/skills`) are passed through unchanged. The in-memory config is unaffected — `Load()` still expands `~` as usual

### Bug Fixes

- Fixed `skillshare init -p` not gitignoring `.skillshare/backups/` — backup artifacts from project-mode agent sync could be accidentally committed

## [0.19.10] - 2026-05-12

### Bug Fixes

- Fixed `skillshare update --all` creating duplicate `.metadata.json` entries with `../../...` relative-path keys when the skills source directory is a symlink or custom location. Existing metadata keys such as `browser/agent-browser` now stay stable during update. Refs: #152

## [0.19.9] - 2026-05-11

### New Features

- **Context cost summary after sync** — `skillshare sync` now displays a one-line token cost summary showing always-loaded and on-demand context usage per target. Targets with identical token counts are grouped on a single line. Refs: #150
  ```
  ✔ Synced 47 skill(s) to 4 target(s) in 312ms
    Context: ~12.4K always-loaded · ~58.2K on-demand (claude, cursor, codex, opencode)
  ```
- **Configurable budget warnings** — set token budget thresholds in `config.yaml`. When `sync` or `analyze` detects a target exceeding the budget, a warning shows the top 3 offenders by token count
  ```yaml
  context_budget:
    warn_always_loaded_tokens: 10000   # default; 0 = disabled
    warn_on_demand_tokens: 100000      # default; 0 = disabled
  ```
  Defaults to 10K always-loaded / 100K on-demand. Set to `0` to disable
- **`--quiet` / `-q` flag for sync** — suppresses the token summary and budget warnings. JSON output (`--json`) always includes `context_cost` regardless of `--quiet`
- **Context cost in Web UI** — the Sync page now displays token cost groups and budget violation warnings after each sync

### Bug Fixes

- Fixed `skillshare install` failing with `http://` protocol URLs (contributed by @eekryuos)

## [0.19.8] - 2026-05-08

### New Features

- **Shell completion** — new `skillshare completion` command generates tab-completion scripts for bash, zsh, fish, PowerShell, and Nushell. Supports `--install` to auto-write the script to the correct platform path. Refs: #148
  ```bash
  skillshare completion bash --install    # one-line setup
  skillshare completion zsh --install
  skillshare completion fish --install
  skillshare completion powershell --install
  skillshare completion nushell --install
  ```
  Covers all commands, subcommands, and per-command flags. Bash, zsh, and PowerShell scripts auto-detect aliases (e.g. `alias ss=skillshare`) and register completions for them

## [0.19.7] - 2026-05-05

### New Features

- **Azure DevOps Server (on-premises) support** — added `azure_hosts` config parameter for self-hosted Azure DevOps Server instances on custom domains. Previously, only `dev.azure.com` and `*.visualstudio.com` were recognized; custom-domain URLs would fail with an incorrect `.git` suffix in the clone URL. Refs: #147
  ```yaml
  azure_hosts:
    - azuredevops.mycompany.com
  ```
  ```bash
  skillshare install https://azuredevops.mycompany.com/Org/Project/_git/Repo
  ```
  Works in both global and project configs. For CI/CD, use the `SKILLSHARE_AZURE_HOSTS` environment variable (comma-separated, merged with config file values)

## [0.19.6] - 2026-05-05

### New Features

- **8 new agent targets** — added AiderDesk, CodeArts Agent (Huawei), Code Studio (Syncfusion), Devin for Terminal (Cognition), Dexto, ForgeCode, Rovo Dev (Atlassian), and Tabnine CLI. Total built-in targets: 64+
- **Cursor project path updated to `.agents/skills`** — Cursor officially supports `.agents/skills` as a project-level skill path alongside `.cursor/skills`. The project path now uses the ecosystem-standard `.agents/skills` convention, matching Cursor's official documentation. The global path (`~/.cursor/skills`) and agents paths are unchanged
- **Automatic target inference from directory structure** — skills organized under a target's project path are now automatically scoped to that target during sync, without needing `targets:` in SKILL.md frontmatter. For example, placing a skill at `.cursor/skills/my-skill/` in the source directory automatically restricts it to the `cursor` target
  ```
  source/
    .cursor/skills/browse/     → syncs only to cursor
    .factory/skills/bench/     → syncs only to droid
    openclaw/skills/investigate/ → syncs only to openclaw
    shared/my-tool/            → syncs to all targets (no inference)
  ```
  Inference matches against the project paths defined in `targets.yaml`. When multiple targets share the same path (e.g. `universal`, `codex`, `amp` all use `.agents/skills`), the skill is correctly scoped to all of them

### Bug Fixes

- Fixed host path inference only returning the canonical target name when multiple targets share the same project path — `.agents/skills/` skills now correctly infer all sharing targets instead of just `universal`
- Fixed `skillshare update` failing for skills originally installed at the repo root of an orchestrator (multi-skill) repository — the update now correctly detects the repo layout and re-applies subdirectory extraction

## [0.19.5] - 2026-04-23

### New Features

#### Search Experience

- **Skill preview modal** — clicking a search result card or table row now opens a preview modal that renders the remote `SKILL.md` so you can read the full content before installing. The **Install** button has moved inside the modal
  ```bash
  skillshare ui              # open Search, click any result
  ```
  Backed by a new `GET /api/preview?source=owner/repo/path` endpoint that fetches `SKILL.md` from the GitHub Contents API with a Git Tree fallback for hub sources, plus a 5-minute in-memory cache. If the fetch fails but hub metadata exists, the modal falls back to showing the description and tags instead of an error. Refs: #141

- **Higher-quality search results** — the scoring formula that ranked search hits was dominated by name matches (45%) while stars — the strongest quality signal — only contributed 5%, so zero-star personal experiments often outranked well-known skill repos. Weights are now rebalanced (name 30%, description 20%, stars 25%, source 25%), with a better star-count curve in the 5–50 range where most useful skills live
- **Low-quality result filtering** — search now filters out 0-star non-preferred repos, results with very short descriptions, and known spam orgs, and boosts preferred hubs like `anthropics/skills` and `vercel-labs/skills`. Results are also deduplicated when the same skill appears from multiple sources
- **Repo-scoped search queries** — you can now search inside a specific repo (or subdirectory) using `owner/repo` or `owner/repo/subdir` patterns
- **Frontmatter validation** — search results without valid `SKILL.md` frontmatter are filtered out, so non-skill `.md` files no longer appear in results

#### Skill Detail Editor

- **Editable source URL on skill detail page** — the skill editor now exposes a **Source URL** field so you can correct or reassign the upstream URL without touching disk
  ```bash
  skillshare ui              # open any skill's detail page → edit Source URL
  ```
  For tracked repos, updating one skill's source URL updates every sibling skill from the same repo and rewrites the git remote origin to match. Saving source-URL-only changes skips the diff review dialog
- Backed by a new `PATCH /api/resources/{name}/source` endpoint

### Bug Fixes

- Fixed a skill lookup collision when a standalone skill and a tracked-repo skill shared the same base name — detail-page requests now prefer an exact `FlatName` match before falling back to the base name, so the correct skill is always returned
- Fixed spurious frontmatter diffs when only the body or source URL changed — the YAML serializer now preserves the original formatting (quote style, line wrapping) unless a field's value actually changed
- Fixed `IntersectionObserver` on the Search page not reconnecting after "load more", so infinite scrolling now continues to fire as you reach the bottom of the list
- Fixed background page scrolling when a modal was open — `DialogShell` now locks body scroll for all modal dialogs

### Improvements

- **Keyboard-accessible search results** — search result cards and table rows now have `role="button"` with `tabIndex=0` and respond to Enter/Space, so the preview modal can be opened without a mouse
- **Bounded preview cache** — the preview cache is now capped at 200 entries with TTL purge plus a full reset on overflow to prevent unbounded memory growth on long-running UI sessions
- **Shared input components in the frontmatter editor** — the editor now uses the same `Input` / `Textarea` primitives as the rest of the UI (with a new `size="sm"` variant) for visual consistency, and the editor header now uses the shared `KindBadge` / `SourceBadge` components

## [0.19.4] - 2026-04-22

### Bug Fixes

- **UI install no longer deletes local source files** — installing a skill from a local path via the Web UI would permanently delete the original source directory after discovery. The cleanup logic now only removes temporary directories created by git clones, leaving user directories untouched. CLI installs were unaffected. Refs: #139

### Improvements

- **`$ARGUMENTS` tokens highlighted on detail page** — skill detail pages in the Web UI now render `$ARGUMENTS` placeholders with the same warning-badge style used in the editor, making them easier to spot when reviewing a skill's content

## [0.19.3] - 2026-04-18

### New Features

#### Web-Based Skill Editor

- **Edit skills directly in the Web UI** — the resource detail page now has an **Edit** action that opens a full in-browser editor, replacing the previous read-only view. Changes are saved back to the source `SKILL.md` through a new `POST /api/resources/:name/content` endpoint
  ```bash
  skillshare ui              # then open any skill and click Edit
  ```

- **Two-pane Markdown editor** — side-by-side textarea and live preview with synced scrolling. A mode toggle in the status bar switches between Edit / Split / Preview (`⌘P` cycles), and `⌘S` saves while `Esc` cancels. An outline drawer lets you jump to any heading, and the status bar shows token / word / line / file counts with a 5K-character budget warning

- **Structured frontmatter editor** — all 13 official SKILL.md fields are grouped into Identity / Invocation / Execution / Metadata sections with field-appropriate controls: switch toggles for booleans, segmented control for enums, and chip inputs for list-valued fields. A shared 1,536-character budget is enforced across `description` + `when_to_use`. A raw **YAML mode** is also available, and round-trips cleanly with the Fields view. Legacy root-level `targets:` is automatically migrated to `metadata.targets` on load

- **Diff review before save** — saving opens a side-by-side diff modal so you can confirm every change before writing to disk. The YAML serializer emits plain scalars when safe, so the diff no longer shows spurious quote wrapping for values containing `:`, `*`, `#`, or `"`

- **Open in local editor** — a new `POST /api/resources/:name/open` endpoint opens the skill file in your preferred local editor (via `$EDITOR`), useful when you prefer `vim` / VS Code over the browser editor

- **Targets visible in detail sidebar** — the resource detail sidebar now shows a **Targets** row when `metadata.targets` is set, so you can see at a glance which targets a skill is scoped to

#### Web UI Localization

- **11 languages in the Web UI** — every page is now fully translated. Pick your language from the language switcher in the top navigation bar; the preference is saved to your browser and auto-detected on first visit from `navigator.languages`
  - Supported: English, 中文, 日本語, 한국어, Español, Français, Deutsch, فارسی, Português (BR), Bahasa Indonesia
  - Persian (فارسی) automatically switches the layout to right-to-left

## [0.19.2] - 2026-04-14

### New Features

- **Local agent count in Targets** — the Targets page and `skillshare target --json` now show how many local (non-source) agents exist per target alongside the linked/expected counts. The Collect button appears for targets with local agents and routes directly to the Collect page with the correct scope pre-selected
  ```bash
  skillshare target --json   # agentLocalCount field in each target
  ```

- **Richer `target list --json` output** — each target entry now includes `targetNaming` (effective naming strategy, e.g. `flat`), `sync` (skill sync summary), and `agentSync` (agent sync summary), making JSON output more useful for scripted workflows and external tooling
  ```bash
  skillshare target list --json   # targetNaming, sync, agentSync per target
  ```

### Bug Fixes

- Fixed batch uninstall for agents — selecting multiple agents from the same tracked repo on the Uninstall page sent only the repo name instead of individual agent names, causing all uninstalls to fail with "agent not found". The confirmation dialog now correctly lists each agent and the API receives individual names
- Fixed empty state messages on the Resources and Uninstall pages — switching to the Agents tab with no agents installed now shows "No agents installed" instead of the generic "No skills installed" text
- Fixed Health Check theme display — the Theme check on the Web UI always showed a warning with technical internals (`fallback-dark-no-tty`, `no_color: false`) that users couldn't understand. Now the CLI and UI both show human-readable messages like "auto-detected from terminal" or "set via SKILLSHARE_THEME", and the UI no longer shows a false warning caused by the server subprocess lacking a terminal

## [0.19.1] - 2026-04-13

### Bug Fixes

- **Orchestrator repos no longer copy the entire repository** — installing a multi-skill repo with a root `SKILL.md` (e.g. `skillshare install user/project`) previously copied the entire repo root into the root skill directory, including source code, assets, CI configs, and build scripts. Now the root skill installs only its `SKILL.md`, and each child skill installs as an independent flat directory. Refs: #124
  ```
  # Before: skills/MyProject/ contained the entire repo
  # After:
  skills/MyProject/SKILL.md       ← root skill (SKILL.md only)
  skills/child-a/...              ← independent child
  skills/child-b/...              ← independent child
  ```

- **Structured output no longer corrupted by update notices** — `--json`, `-j`, and `--format json/sarif/markdown` modes could emit a trailing human-readable update notification into stdout, producing invalid JSON for downstream consumers. The update check is now skipped entirely in structured-output modes, and the notification itself writes to stderr as a safety net. Refs: #129

## [0.19.0] - 2026-04-11

### New Features

#### Agent Management

Agents are now a first-class resource type alongside skills. You can install, sync, audit, and manage agent files (`.md`) across agent-capable targets (Claude, Cursor, OpenCode, Augment) with the same workflow as skills.

- **Agents source directory** — agents live in `~/.config/skillshare/agents/` (or `.skillshare/agents/` in project mode). `skillshare init` creates the directory automatically, and `agents_source` is a new config field that can be customized
  ```bash
  skillshare init              # creates skills/ and agents/
  skillshare init -p           # same for project mode
  ```

- **Positional kind filter** — most commands accept an `agents` keyword to scope the operation to agents only. Without it, commands operate on skills (existing behavior is unchanged)
  ```bash
  skillshare sync agents             # sync agents only
  skillshare sync --all              # sync skills + agents + extras
  skillshare list agents             # list installed agents
  skillshare check agents            # detect drift on agent repos
  skillshare update agents --all     # update every agent
  skillshare audit agents            # scan agents for security issues
  skillshare uninstall agents foo    # uninstall a single agent
  skillshare enable foo --kind agent
  skillshare disable foo --kind agent
  ```

- **Install agents from repos** — `install` auto-detects agents in three layouts:
  - `agents/` convention subdirectory
  - mixed-kind repos with both `SKILL.md` and `agents/`
  - pure-agent repos (root `.md` files, no `SKILL.md`)
  ```bash
  skillshare install github.com/team/agents            # auto-detect
  skillshare install github.com/team/repo --kind agent # force agent mode
  skillshare install github.com/team/repo --agent cr   # specific agents
  ```
  Conventional files (`README.md`, `LICENSE.md`, `CHANGELOG.md`) are automatically excluded

- **Tracked agent repos** — agents can be installed with `--track` for git-pull updates, including nested discovery. `check`, `update`, `doctor`, and `uninstall` all recognise tracked agent repos, and the `--group` / `-G` flag filters by repo group

- **Agent sync modes** — merge (default, per-file symlink), symlink (whole directory), and copy are all supported. `skillshare sync agents` skips targets that don't declare an `agents:` path and prints a warning

- **`.agentignore` support** — agents can be excluded via `.agentignore` and `.agentignore.local` using the same gitignore-style patterns as `.skillignore`. The Web UI Config page now has a dedicated `.agentignore` tab

- **Agent audit** — `skillshare audit` scans agent files individually against the full audit rule set, with Skills/Agents tab switching in both the TUI and Web UI. Audit results carry a `kind` field so tooling can filter by resource type

- **Agent backup and restore** — sync automatically backs up agents before applying changes, in both global and project mode. The backup TUI and trash TUI tag agents with an `[A]` badge and route restores to the correct source directory

- **Project-mode agent support** — every agent command works in project mode with `-p`. Agents are reconciled alongside skills into `.skillshare/`

- **JSON output for agents** — `install --json` and `update --json` now emit agent-aware payloads and apply the same audit block-threshold gate as skills. Useful for scripted agent workflows
  ```bash
  skillshare update agents --json --audit-threshold high
  ```

- **Kind badges** — TUI and Web UI surface `[S]` / `[A]` badges throughout (list, diff, audit, trash, backup, detail, update, targets pages) so you can tell at a glance what kind of resource you're looking at

#### Unified Web UI Resources

- **`/resources` route** — the old `/skills` page is now `/resources`, with Skills and Agents tabs. Tab state persists to localStorage, and the underline tab style follows the active theme (playful mode gets wobble borders)

- **Targets page redesign** — equal Skills and Agents sections, with a modal picker for adding targets. Filter Studio links include a `?kind=` param so you jump directly to the right context

- **Update page redesign** — a new three-phase flow (selecting → updating → done) with skills/agents tabs, group-based sorting, and status cards. EventSource streaming is properly cleaned up on page change

- **Filter Studio agent support** — agent filters can be edited via `PATCH /api/targets/:name` (`agent_include`, `agent_exclude`, `agent_mode`) and via the CLI through new flags on `skillshare target <name>`:
  ```bash
  skillshare target claude --add-agent-include "team-*"
  skillshare target claude --remove-agent-include "team-*"
  skillshare target claude --agent-mode copy       # merge | symlink | copy
  ```
  The UI Filter Studio is a single-context view driven by `?kind=skill|agent`

- **Audit cache** — audit results are now cached with React Query and invalidated on mutation. The audit card icon colour follows the max severity, and the count no longer mixes agent totals with finding counts

- **Collect page scope switcher** — a new segmented control lets you collect skills or agents from targets

#### Theme System

- **`internal/theme` package** — unified light/dark terminal palette with WCAG-AA-compliant light colours and softened dark primary. Resolution order: `NO_COLOR` > `SKILLSHARE_THEME` > OSC 11 terminal probe > dark fallback. All TUIs, list output, audit output, and plain CLI output now route through the theme
  ```bash
  SKILLSHARE_THEME=light skillshare list
  SKILLSHARE_THEME=dark skillshare audit
  ```
  `skillshare doctor` includes a theme check to help debug unreadable colours

#### Install & TUI Polish

- **Explicit `SKILL.md` URLs resolve to one skill** — pasting a direct `blob/.../SKILL.md` URL now installs only that skill, bypassing the orchestrator pack prompt. Previously, the URL would trigger the full multi-select picker even though the intent was clear
  ```bash
  skillshare install https://github.com/team/repo/blob/main/frontend/tdd/SKILL.md
  ```
  Refs: #124

- **Radio checklist follows the cursor** — the single-select TUI (used for orchestrator selection, branch selection, and similar flows) now auto-selects the focused row. No more confusing empty-selection state — pressing Enter always confirms the item your cursor is on

- **Diff TUI** — single-line items with group headers instead of the old verbose-per-item layout. Agent diffs are shown with an `[A]` badge

- **List TUI** — entries are now grouped by tracked repo root and local top directory, with a new `k:kind` filter tag for quick agent/skill filtering inside the fuzzy filter

#### Centralized Metadata Store

- **`.metadata.json` replaces sidecar files and `registry.yaml`** — installation metadata is now stored in a single atomic file per source (`~/.config/skillshare/skills/.metadata.json`). This fixes long-standing issues with grouped skill collisions (e.g. two skills both named `dev` in different folders) where the old basename-keyed registry would mix them up
  - **Automatic migration** — the first load after upgrade reads any existing `registry.yaml` and per-skill `.skillshare-meta.json` sidecars, merges them into `.metadata.json`, and cleans up the old files. Idempotent — safe to run repeatedly
  - **Full-path keys** — lookups use the full source-relative path, so nested skills never collide
  - No user action required; existing installs continue to work

### Bug Fixes

- **Sync extras no longer flood when `agents` target overlaps** — targets that declare an extras entry called `agents` are now skipped automatically when agent sync is active, preventing duplicate file writes
- **Nested agent discovery** — `check agents` now uses the recursive discovery engine, so agents in sub-folders (e.g. `demo/code-reviewer.md`) are detected correctly
- **Doctor drift count excludes disabled agents** — agents disabled via `.agentignore` no longer count toward the drift total reported by `skillshare doctor`
- **Audit card mixes counts** — the Web UI audit card no longer mixes agent counts with finding counts, and excludes `_cross-skill` from the card total (shown separately)
- **Audit scans disabled agents too** — the audit scan walks every agent file regardless of `.agentignore` state, so hidden agents still get checked
- **List TUI tab bar clipping** — the tab bar no longer gets cut off in the split-detail layout on narrow terminals
- **Sync extras indent** — removed the stray space between the checkmark and the path in `sync` extras output; summary headers are now consistent across skills, agents, and extras
- **UI skill detail agent mode** — the detail page hides the Files section for agents (single-file resources), remembers the selected tab via localStorage, and shows the correct folder-view labels
- **Sync page layout** — the stats row and ignored-skills grouping are now easier to scan
- **Button warning variant** — the shared `Button` component now supports a `warning` variant that was already referenced by several pages
- **Check progress bar pop-in** — removed the loading progress bar that caused layout shift on the Skills page
- **Tracked repo check status** — the propagated check status is now applied to every item within the repo, not just the root
- **Target name colouring in doctor** — only the status word is coloured in `doctor` target output, not the full line

### Breaking Changes

- **`audit --all` flag removed** — use the positional kind filter instead:
  ```bash
  skillshare audit                # skills (default, unchanged)
  skillshare audit agents         # agents only
  ```
  The old `--all` flag is gone because audit now runs per kind and the Web UI has dedicated tabs

## [0.18.9] - 2026-04-07

### New Features

- **Relative symlinks in project mode** — `skillshare sync -p` now creates relative symlinks (e.g., `../../.skillshare/skills/my-skill`) instead of absolute paths. This makes the project directory portable — rename it, move it, or clone it on another machine and all skill symlinks continue to work. Global mode continues to use absolute paths. Existing absolute symlinks are automatically upgraded to relative on the next sync

### Bug Fixes

- **Status version detection** — `skillshare status` no longer reports `! Skill: not found or missing version` when the version is stored under `metadata.version` in the SKILL.md frontmatter. Previously, the `status` command used its own local parser that only checked for a top-level `version:` key, while `doctor` (fixed in v0.18.7) and `upgrade` already used the correct shared parser

## [0.18.8] - 2026-04-06

### Bug Fixes

- **Sync no longer deletes registry entries for installed skills** — running `skillshare sync` (or project-mode `sync -p`) would silently remove `registry.yaml` entries for skills whose source files were not present on disk. This meant that installing a skill and then syncing could erase the installation record entirely. Sync now leaves the registry untouched — only `install` and `uninstall` manage registry entries

## [0.18.7] - 2026-04-04

### New Features

#### Folder-Level Target Display & Bulk Editing

- **Folder target aggregation** — the Skills page grouped view now shows aggregated target info on each folder row. If all skills share the same target, it shows that target; mixed targets show the union with a warning badge; `All` is shown when no targets are set
  - Compact display: folders with 4+ targets show `N targets` with full list in tooltip

- **Bulk set target** — right-click any folder to set or remove the target for all skills in that subtree at once
  ```
  Right-click folder → Available in... → claude
  ```
  Writes `metadata.targets` to every SKILL.md in the folder. Selecting `All` removes the field. Disabled skills are skipped

- **Single skill target editing** — right-click any skill in grouped or grid view, or use the inline `Available in` dropdown in table view, to change which targets receive that skill

#### Right-Click Context Menu

- **Context menu on all views** — right-click skills in grouped, grid, or table view for quick actions:
  - **Available in...** — submenu to set target (hover-expand with 180ms intent delay)
  - **View Detail** — navigate to skill detail page
  - **Enable / Disable** — toggle skill visibility
  - **Uninstall** — with confirmation dialog

- **Folder context menu** — right-click folders in grouped view for `Folder available in...` (batch target) — only target actions, no uninstall

- **Submenu pattern** — top-level items with sub-options expand on hover. Future actions (e.g. Move, Rename) can be added as flat items alongside

#### Table View Improvements

- **Inline target selector** — the table now has an `Available in` column with an inline dropdown for one-click target switching — no context menu needed
- **Actions column** — `⋯` button opens a flat menu with View Detail, Enable/Disable, and Uninstall
- **Simplified layout** — reduced from 7 columns to 5 by merging Path and Source into the Name cell. Path shows as a subtitle when different from the name; source shows as a clickable globe icon linking to the repo
- **Persistent page size** — the selected page size (10/25/50) is remembered across sessions

#### UX Polish

- **Right-click tip banner** — a one-time dismissible tip appears on first visit, explaining that right-click is available for actions. Styled with playful theme support (wobble borders, paper-warm background, slight tilt)
- **Optimistic updates** — all mutations (set target, enable/disable, uninstall) update the UI instantly with automatic rollback on error
- **Active item highlight** — when a context menu is open, the targeted skill or folder gets a hover-matching highlight

### Bug Fixes

- **Config page dashed border clipping** — in playful theme, the Structure panel's dashed borders were cut off at the right edge because the panel wrapper kept `overflow-hidden` after expanding. Now uses `overflow-visible` when expanded

- **Tracked repos in project-mode dashboard** — the Web UI dashboard now shows tracked repositories when running in project mode (`skillshare ui -p`), with Update and Uninstall actions

- **Nested tracked repo update and uninstall** — repos installed with a nested path (e.g. `org/_team-skills`) can now be updated and uninstalled from both the CLI and Web UI. Previously, the server failed to resolve nested repo paths for these operations

- **Project-mode uninstall cleans correct `.gitignore`** — uninstalling a tracked repo in project mode now removes entries from `.skillshare/.gitignore` instead of the global source `.gitignore`. Previously, stale ignore rules were left behind

- **Registry prune no longer affects sibling repos** — uninstalling a nested tracked repo (e.g. `org/_team-skills`) no longer accidentally removes registry entries belonging to a sibling with the same basename (e.g. `dept/_team-skills`)

- **Nested trash lifecycle** — trash, restore, cleanup, and listing now work correctly for nested tracked repo names. Parent directories are created on restore and cleaned up after expiry

- **Dashboard tracked repo row polish** — status indicators (`clean` / `modified`) moved next to the repo name as compact badges. Action buttons use a smaller `xs` size to reduce visual weight

- **Bulk target folder matching** — setting targets on a folder with a trailing slash (e.g. `frontend/`) no longer silently skips all skills. The server now normalizes folder paths before matching

- **Doctor version detection** — `skillshare doctor` no longer reports `! Skill: missing version` when the version is stored under `metadata.version` in the SKILL.md frontmatter. Previously, the inline parser only checked for a top-level `version:` key

- **Target display on Skills page** — the Skills page now correctly shows saved targets for each skill. Previously, the API did not parse SKILL.md frontmatter, so targets always appeared as `All` even after being set

- **Target editing respects tracked repos** — setting targets via the context menu or batch folder action now skips tracked-repo skills. Previously, writing to SKILL.md inside a tracked repo would make the repo dirty and block future updates. Audit hash integrity is also preserved after target edits

- **Uninstall Repo from context menu** — right-clicking a tracked-repo skill now shows `Uninstall Repo` instead of the individual `Uninstall` action (which would always fail with an error)

- **Enable/disable with glob patterns** — enabling a skill that was disabled by a glob or directory pattern in `.skillignore` (or `.skillignore.local`) now correctly returns an error with guidance, instead of silently showing a success toast while the skill stays disabled

- **Disabled tracked skills stay in Tracked view** — disabling a tracked-repo skill via `.skillignore` no longer removes it from the Tracked tab. The discovery engine now correctly preserves the `isInRepo` flag for all disabled skills

## [0.18.6] - 2026-04-01

### Bug Fixes

- **UI batch uninstall now removes nested skill registry entries** — previously, uninstalling grouped skills (e.g. `frontend/vue/vue-best-practices`) from the Web UI left stale entries in `registry.yaml` because the flat name (`__`) didn't match the stored path name (`/`). Uninstall now tracks the exact resolved path for accurate cleanup

- **Sync prunes stale registry entries** — `skillshare sync` and the Web UI Sync page now automatically remove `registry.yaml` entries for skills that no longer exist in the source directory. Covers manual deletions, not just `uninstall`. Skills hidden by `.skillignore` are preserved

- **Uninstall page search works as substring match** — typing `matt` in the filter box now matches `mattpocock/tdd` (substring search). Previously, only glob patterns like `*matt*` worked. Glob syntax (`*`, `?`) still works when present

- **Uninstall page shows path format** — the confirmation dialog and result summary now display `frontend/vue/vue-best-practices` instead of `frontend__vue__vue-best-practices`

- **Updates page removes redundant status line** — the "0 repo(s) and 20 skill(s) already up to date" line no longer appears when everything is already current (the empty state already says this)

## [0.18.5] - 2026-04-01

### New Features

- **`--help` for all commands** — every command now supports `--help` / `-h` to show usage info, flags, and examples. Previously, commands like `push`, `pull`, `sync`, `status`, `collect`, `doctor`, and `ui` would execute instead of showing help when `--help` was passed
  ```bash
  skillshare push --help     # shows usage instead of pushing
  skillshare sync -h         # shows flags and examples
  skillshare ui --help       # shows port/host options
  ```

## [0.18.4] - 2026-03-31

### New Features

#### Branch Support (`--branch` / `-b`)

- **Install from a specific branch** — new `--branch` / `-b` flag lets you clone from any branch instead of the remote default:
  ```bash
  skillshare install github.com/team/skills --branch develop --all
  skillshare install github.com/team/skills --track --branch frontend
  ```
  - Works with both tracked repos (`--track`) and regular skill installs
  - Branch is persisted in metadata — `update` and `check` automatically use the correct branch
  - Same repo on different branches: use `--name` to avoid collisions:
    ```bash
    skillshare install github.com/team/skills --track --branch frontend --name team-frontend
    skillshare install github.com/team/skills --track --branch backend --name team-backend
    ```
  - Supported in project mode (`-p`) and config-driven rebuild (`skillshare install` with no args)
  - `registry.yaml` stores the branch for cross-device reproducibility

- **Branch in Web UI** — the Install form shows a Branch input field (inline with Source) when a git source is detected. Skills page shows a branch badge on cards, and the Skill Detail page includes branch in the metadata section

- **Branch in CLI list** — `skillshare list` detail panel shows the tracked branch when non-default

- **Branch-aware check** — `skillshare check` compares against the correct remote branch ref, not just HEAD. JSON output includes a `branch` field for tracked repos

#### Sync Accuracy

- **Accurate skill counts on Targets page** — the expected skill count now reflects what sync actually resolves (after name collision and validation filtering), instead of the raw source count. Previously, targets using `standard` naming could show `39032/64075 shared` when all resolved skills were actually in sync

- **Skipped skill visibility** — when skills are excluded by naming validation or collisions, the Targets page and Sync page now show a summary (e.g. "12345 skill(s) skipped, 456 name collision(s)") instead of silently dropping them. Suggests switching to `flat` naming to include all skills

#### Target Naming Mode

- **`target_naming` config option** — choose how synced skill directories are named in targets. Set globally or per-target:
  ```yaml
  target_naming: standard    # use SKILL.md name as directory name
  targets:
    claude:
      skills:
        target_naming: flat  # override: keep flattened prefix (default)
  ```
  - `flat` (default) — nested skills like `frontend/dev` become `frontend__dev` in targets
  - `standard` — uses the SKILL.md `name` field directly (e.g. `dev`), following the [Agent Skills specification](https://agentskills.io/specification)
  - Standard mode validates that SKILL.md names match their directory name, warns and skips invalid skills
  - Name collisions (e.g. two skills both named `dev`) are detected and both are skipped with a warning
  - Switching from `flat` to `standard` automatically migrates existing managed entries (symlinks and copies are renamed in place)
  - If a local skill already occupies the bare name, the legacy flat entry is preserved with a warning
  - `target_naming` is ignored in `symlink` mode (the entire source directory is linked)

- **Collision output redesigned** — name conflict warnings are now deduplicated across targets and displayed as a compact summary instead of repeating each collision per target

#### Folder Tree View (Web UI)

- **Skills folder tree** — the second layout on `/skills` is now a true folder tree with multi-level expand/collapse, matching your actual directory structure from `--into`:
  - Click any folder to expand/collapse its children
  - **Expand All / Collapse All** buttons in the toolbar
  - **Sticky folder header** — scrolling through a long folder keeps the folder name pinned at the top; click it to jump back
  - **Search-aware** — filtering or searching auto-expands all matching folders; clearing restores your previous collapse state
  - **Hover tooltip** on skill rows shows path, source, and install date (follows cursor, 1.5s delay)
  - Collapse state persists across page reloads via localStorage
  - Virtualized rendering handles 10,000+ skills with no performance impact

- **Skill detail button layout** — the Enable/Update/Uninstall buttons no longer wrap awkwardly on narrow screens

#### Agent Target Paths

- **Agent-specific paths** — targets that support agents (Claude, Cursor, OpenCode, Augment) now declare separate `agents:` paths in their configuration. This is groundwork for upcoming agent sync support

#### GitHub Actions

- **`setup-skillshare` action** — install skillshare in CI with a single step:
  ```yaml
  - uses: runkids/setup-skillshare@v1
  ```

### Bug Fixes

- **CLI sync output no longer floods terminal** — targets with thousands of naming validation warnings (common with `standard` naming and large skill sets) now print a compact summary instead of one line per skipped skill
- **Target dropdown lag removed** — changing sync mode or target naming in the Web UI Targets page now updates instantly via optimistic cache update, instead of waiting 2 seconds for the API round-trip

### Performance

- **Cached branch lookups** — `skillshare list` caches `git` branch queries per tracked repo, so listing 500 skills from the same repo runs 1 git command instead of 500

## [0.18.3] - 2026-03-29

### New Features

#### Enable / Disable Skills

- **`skillshare enable` / `skillshare disable`** — temporarily hide skills from sync without uninstalling them. Adds or removes patterns in `.skillignore`:
  ```bash
  skillshare disable draft-*        # hide from sync
  skillshare enable draft-*         # restore
  skillshare disable my-skill -p    # project mode
  skillshare disable my-skill -n    # dry-run preview
  ```

- **TUI toggle** — press `E` in `skillshare list` to toggle the selected skill's enabled/disabled state. The change is written immediately without leaving the TUI. Disabled skills show a red **disabled** badge in the detail panel and a `[disabled]` suffix in compact view

- **Web UI toggle** — the Skill Detail page now shows an Enable/Disable button. The REST API exposes `POST /api/skills/{name}/enable` and `POST /api/skills/{name}/disable` endpoints

#### Target Config — Skills Sub-Key

- **Per-resource-type configuration** — target configs now support a `skills` sub-key with its own `path`, `mode`, `include`, and `exclude` settings. Existing flat-field configs are auto-migrated on first load — no manual editing needed:
  ```yaml
  targets:
    claude:
      skills:
        path: ~/.claude/skills
        mode: merge
  ```

#### Upgrade Improvements

- **Auto-sudo for protected paths** — `skillshare upgrade` now auto-detects when the binary is in a write-protected directory (e.g., `/usr/local/bin`) and transparently re-executes with `sudo` (#105)

### Bug Fixes

- Fixed `diff` showing skills with unsupported `targets` values (e.g., `targets: ["*"]`) as "source only" pending items — these skills are now correctly filtered out, matching the behavior of `sync`
- Fixed `collect` ignoring inherited sync mode when a target's mode was set at the global level — the command now resolves the effective mode before deciding whether to scan for local skills
- Fixed `collect` using a stale manifest after switching a target from `copy` to `merge` mode
- Fixed `pull` info message referencing `git stash` instead of `git stash -u`
- Fixed config migration writing directly to the config file — now uses atomic write-to-temp + rename to prevent corruption on disk errors

## [0.18.2] - 2026-03-28

### New Features

#### Update Page Improvements

- **Sticky progress bar** — the Update page now shows a real-time progress bar during batch updates with percentage, completed/total count, and ETA. The bar stays pinned to the top while scrolling through the item list

- **Auto-scroll to active item** — during batch updates, the page automatically scrolls to the item currently being updated so you can follow the progress without manual scrolling

- **Purge stale skills** — when a skill fails to update because its subdirectory no longer exists in the repository, a **Purge** button appears instead of Force Retry. Clicking it removes the stale skill from your source directory

#### Analyze & Install UX

- **Install picker improvements** — the skill picker modal in Install and Analyze pages now auto-focuses the search field, shows clearer skill descriptions, and handles keyboard navigation better

- **Tooltip enhancements** — tooltips across the dashboard now follow the cursor and stay within viewport bounds

### Bug Fixes

- Fixed analyze page crash when a target has no skills (null targets array from API)
- Fixed redundant path line showing in the skill detail dialog on the Analyze page
- Fixed install source field not clearing after a successful installation

## [0.18.1] - 2026-03-27

### New Features

#### Analyze — Filter & Token Budget

- **`--filter` flag** — filter skills by name or group path with case-insensitive substring matching. Works across all output modes:
  ```bash
  skillshare analyze claude --json --filter frontend   # JSON with filtered_summary
  skillshare analyze --filter marketing                 # TUI with pre-populated filter
  ```

- **Dynamic token subtotals** — the TUI stats line and Web UI now show always-loaded, on-demand, and total token sums for the current filtered set. In the TUI, the format changes to `5/50 skills` when a filter is active

- **Web UI filtered summary bar** — when searching or filtering on the Analyze page, a summary bar appears above the table with token counts per category. The bar slides in/out with a smooth animation

- **JSON `filtered_summary`** — when `--filter` is used with `--json`, the output includes `filter`, `matched_count`, `total_count`, and a `filtered_summary` object with `always_loaded`, `on_demand`, and `total` token counts

#### Registry Location

- **Registry moved to source directory** — `registry.yaml` now lives at `~/.config/skillshare/skills/registry.yaml` (inside the source directory) instead of `~/.config/skillshare/registry.yaml`. This means `git sync` automatically includes the registry, so tracked skill metadata is preserved across machines. Migration is automatic on first run (#103)

### Bug Fixes

- **`init --remote` skips skill prompt** — when `init --remote` clones a repo that already contains skills, the interactive "choose skills to install" prompt is now skipped since the remote already defines the skill set (#102)

### Improvements

- **Analyze Web UI polish** — improved empty state with icon and helper text, fixed table height to prevent layout jumps when filtering, lint filter badge now shows readable rule names (e.g., "No Trigger Phrase" instead of `no-trigger-phrase`)

## [0.18.0] - 2026-03-26

### New Features

#### Analyze Command — Context Window & Skill Quality

- **`skillshare analyze`** — new command that calculates context window token usage for each target's skills. Shows two layers of cost: "always loaded" (name + description, loaded every request) and "on-demand" (skill body, loaded when triggered). Token estimates use `chars / 4`:
  ```bash
  skillshare analyze               # interactive TUI
  skillshare analyze claude        # single target (auto-verbose)
  skillshare analyze --verbose     # top 10 largest descriptions
  skillshare analyze --json        # machine-readable output
  skillshare analyze -p            # project mode
  ```

- **Skill quality lint** — `analyze` runs 7 built-in lint rules against every skill, checking SKILL.md structure and description quality:
  - **Errors**: missing `name`, missing `description`, empty body
  - **Warnings**: description too short (<50 chars), too long (>1024 chars), near limit (900–1024), missing trigger phrases (e.g., "Use when…")

  Lint issues appear in the TUI (✗ for errors, ⚠ for warnings) and in `--json` output as `lint_issues` per skill

- **Interactive TUI** — full-screen bubbletea TUI with left skill list and right detail panel. Features include:
  - Color-coded dots (red/yellow/green by percentile) indicating relative token cost
  - **Tab/Shift+Tab** to switch between targets; identical targets are merged into groups
  - **`/`** to filter skills, **`s`** to cycle sort (tokens↓ → tokens↑ → name A→Z → Z→A)
  - Quality section in detail panel showing all lint findings with icons

- **`--no-tui` flag** — disable the interactive TUI and print plain text summary

#### Web UI — Analyze Page

- **Analyze dashboard** — new page in the web dashboard showing per-target token usage with a chart, skill table with token breakdown, and lint issue indicators
- **Skill detail token breakdown** — the Skill Detail page now shows always-loaded and on-demand token counts
- **`GET /api/analyze` endpoint** — REST API returning per-target context analysis with lint issues, skill paths, tracked status, and descriptions

#### Web UI — Update Page Improvements

- **Sticky search filter** — the search input on the Update page now sticks to the top when scrolling, making it easy to filter skills in long lists
- **SplitButton actions** — Update page action buttons replaced with a SplitButton component for cleaner interaction

### Improvements

- **Unified dialog styling** — all modal dialogs (Confirm, File Viewer, Hub Manager, Keyboard Shortcuts, Skill Picker, Sync Preview, Update) now share consistent styling via a shared `DialogShell` component

## [0.17.11] - 2026-03-25

### New Features

#### Extras — Flatten Option

- **`flatten` for extras targets** — when `flatten: true` is set on an extras target, all files from subdirectories are synced directly into the target root instead of preserving the directory structure. This is useful for AI tools (e.g., Claude Code's `/agents`) that only discover files at the top level:
  ```yaml
  extras:
    - name: agents
      targets:
        - path: ~/.claude/agents
          flatten: true    # source/curriculum/tactician.md → target/tactician.md
  ```

- **`--flatten` flag for `extras init`** — enable flatten when creating a new extra:
  ```bash
  skillshare extras init agents --target ~/.claude/agents --flatten
  ```

- **`--flatten` / `--no-flatten` for `extras mode`** — toggle flatten on existing targets:
  ```bash
  skillshare extras agents --flatten
  skillshare extras agents --no-flatten
  ```

- **Flatten in TUI wizard** — the `extras init` interactive wizard now includes a "Flatten files into target root? (y/N)" step after mode selection (skipped for symlink mode)

- **Filename collision handling** — when flatten causes files from different subdirectories to share the same name (e.g., `team-a/agent.md` and `team-b/agent.md`), the first file wins (sorted alphabetically) and subsequent collisions are skipped with a warning

- **`F` flatten toggle in TUI** — press `F` in the extras list TUI to toggle flatten on/off for a target. Single-target extras toggle directly; multi-target extras show a target picker first

#### Web UI — Flatten Support

- **Flatten checkbox** — the Extras page shows a flatten checkbox per target, both when creating extras and on existing targets. Disabled when mode is symlink
- **Config editor validation** — the YAML config editor warns when `flatten: true` is combined with `mode: symlink`
- **Target name field docs** — clicking a target name in the config editor (both `name: claude` and short-form `- agents`) now shows the correct "target name" documentation instead of unrelated field docs

## [0.17.10] - 2026-03-24

### New Features

- **Update notification in Web UI** — a dialog appears on first visit when a newer CLI or skill version is available. Shows current and latest versions with a copyable `skillshare upgrade` command. Dismissed once per browser session

### Bug Fixes

- **Skill cards equal height** — skill cards on the Skills page now stretch to equal height within each row
- **Tour step target fix** — the skill-filters tour step now correctly highlights its target element

## [0.17.9] - 2026-03-20

### New Features

- **Force toggle on Extras page** — a new Force button in the Extras page header lets you overwrite existing files when the sync mode has changed. Hover for a tooltip explaining what it does. Previously, skipped files could only be force-synced via the CLI (`skillshare sync extras --force`)

### Bug Fixes

- **Config page assistant panel now scrollable** — the right-side Structure/Diff panel now has a fixed 500px content area matching the editor height, enabling vertical scrolling when the YAML structure is long
- **Removed false "Unknown target" warnings** — the config editor no longer flags custom target names as unknown. Target names are user-defined and freely configurable — any name is valid
- **Audit rules assistant panel scrollable** — same fixed-height scrolling fix applied to the Audit Rules page's assistant panel
- **Extras sync API response key** — fixed the JSON response key from `results` to `extras`, which prevented the Extras page from displaying sync results

### Improvements

- **Richer `targets` field docs** — the `targets` field documentation example now shows all sub-fields (`path`, `mode`, `include`, `exclude`) with multiple targets
- **Filter Studio virtual scrolling** — the skill preview list now uses virtual scrolling for smooth performance with large skill collections
- **Force auto-resets after sync** — the Force option on both the Sync and Extras pages automatically disables after a successful sync, preventing accidental overwrites on subsequent runs
- **Smarter skip toast** — when extras files are skipped, the toast suggests "enable Force to override" instead of a CLI command. If Force is already enabled, the hint is omitted

## [0.17.8] - 2026-03-19

### New Features

#### Extras — Configurable Source Paths

- **Custom extras source directory** — extras source paths are now configurable instead of hardcoded. Add `extras_source` to `config.yaml` to set a global default, or use per-extra `source` for individual overrides:
  ```yaml
  extras_source: ~/my-extras               # all extras default to here
  extras:
    - name: rules
      source: ~/company-shared/rules       # this one overrides extras_source
      targets:
        - path: ~/.claude/rules
    - name: commands                        # uses extras_source (~/my-extras/commands/)
      targets:
        - path: ~/.cursor/commands
  ```
  Resolution priority: per-extra `source` > `extras_source` > default (`~/.config/skillshare/extras/<name>/`)

- **`--source` flag for `extras init`** — specify a custom source directory when creating an extra:
  ```bash
  skillshare extras init rules --target ~/.claude/rules --source ~/company-shared/rules
  ```

- **Source input in TUI wizard** — the `extras init` interactive wizard now includes a source directory step between name and target input. Leave empty to use the default

- **`extras source` command** — show or set the global `extras_source` directory from the CLI instead of editing `config.yaml` manually:
  ```bash
  skillshare extras source                          # show current value
  skillshare extras source ~/company-shared/extras  # set new value
  ```

- **`--force` flag for `extras init`** — overwrite an existing extra without needing to `extras remove` first:
  ```bash
  skillshare extras init rules --target ~/.cursor/rules --force
  ```

- **`--source` rejected in project mode** — `extras init --source` now returns a clear error in project mode instead of silently ignoring the flag. Project mode always uses `.skillshare/extras/<name>/`

- **`source_type` in JSON output** — `extras list --json` and `GET /api/extras` now include a `source_type` field (`per-extra`, `extras_source`, or `default`) indicating which level resolved the source path

#### Web UI — Extras Source

- **Source type badge** — the Extras page shows a `(per-extra)` or `(extras_source)` badge next to non-default source paths
- **Source input in Add Extra modal** — optional "Source path" field when creating extras from the dashboard
- **API accepts `source` field** — `POST /api/extras` now accepts an optional `source` field in the request body

#### Web UI — Config Editor Assistant Panel

- **Context-aware assistant panel** — the Config page now has a right-side panel that shows relevant information as you edit `config.yaml`:
  - **Field docs** — move your cursor to any field and see its description, type, allowed values, and an example snippet. Covers all 28+ config fields (source, mode, targets, extras, audit, hub, log, tui, gitlab_hosts, and sub-fields)
  - **Structure tree** — visual outline of your YAML structure with line numbers. Click any node to jump to that line in the editor
  - **Real-time validation** — inline error markers for YAML syntax errors. Schema validation warns about unknown target names (with typo suggestions), invalid sync modes, and invalid audit settings
  - **Diff preview** — see what changed since last save, with colored add/remove lines. Includes a "Revert All" button to reset to the last saved version
  - The panel auto-switches between views by priority (errors → field docs → structure), or lock to Structure/Diff via the bottom bar
  - Collapse the panel with the toggle button or `Cmd+B`; save with `Cmd+S`
- **Empty config guide** — when the editor is empty, the panel shows all available top-level fields as a quick reference
- **`.skillignore` panel** — the `.skillignore` tab shows a simplified panel with change count and the list of currently ignored skills (from all sources, including tracked repos)

#### Web UI — Audit Rules Assistant Panel

- **YAML editor assistant panel** — the Audit Rules page's YAML editor now has the same assistant panel as the Config page:
  - **Field docs** — cursor-aware documentation for all audit rule fields (id, severity, pattern, message, regex, exclude, enabled) with allowed values and examples
  - **Regex tester** — move your cursor to a `regex:` field and the panel auto-switches to an inline regex tester. Paste test lines and see match results instantly with highlighted matches. Supports `(?i)` flag conversion from Go to JavaScript
  - **Real-time validation** — warns about invalid severity values, uncompilable regex patterns (with Go-specific syntax detection), and YAML syntax errors
  - **Structure tree + Diff preview** — same as Config editor
  - Three bottom-bar locks: Structure / Diff / **Test**
- **Save button in header** — the Save button is now in the page header (top-right) for better visibility

### Bug Fixes

- **`.skillignore` save no longer shows sync preview banner** — the "Preview Sync" prompt after save is now only shown for `config.yaml` changes, not `.skillignore`
- **Tracked repo ignores visible without root `.skillignore`** — previously, if the root `.skillignore` file didn't exist, the API returned no ignore stats at all, hiding tracked repos' own `.skillignore` entries. Now always reports all ignored skills regardless of whether a root file exists
- **Dirty state guard on tab switch** — switching between `config.yaml` and `.skillignore` tabs with unsaved changes now shows a confirmation dialog instead of silently discarding edits

## [0.17.7] - 2026-03-19

### New Features

#### Web UI — Uninstall Skills Page

- **Batch uninstall from the dashboard** — a new "Uninstall Skills" page lets you remove multiple skills at once with filtering and multi-select. Filter by group directory, glob pattern (`*react*`, `frontend/*`), or type (Tracked / GitHub / Local), then check the skills you want to remove and confirm:
  - Group dropdown narrows to a specific directory
  - Glob pattern input with real-time matching (supports `*` and `?`)
  - Type filter tabs with counts (matching the Skills page style)
  - Select All / Deselect All for the current filtered view
  - Tracked repos auto-escalate — selecting any skill inside a tracked repo selects the entire repo, with a force option for uncommitted changes
  - Results show per-item success/failure with a "Run sync" reminder
- **Batch uninstall API** — `POST /api/uninstall/batch` accepts multiple skill names in a single request with skip-and-continue semantics. Includes registry cleanup, config reconciliation, and `.gitignore` batch removal — matching the CLI's behavior

#### TUI — Target Remove Action

- **Remove targets with `R` key** — the target list TUI (`skillshare list --targets`) now supports pressing `R` to remove a target from your config

### Bug Fixes

- **Sidebar no longer shifts on long skill lists** — fixed a layout issue where scrolling to the bottom of the Skills page caused the sidebar to shift horizontally

## [0.17.6] - 2026-03-19

### Bug Fixes

- **Sync auto-creates missing target directories with notification** — v0.17.5 introduced a strict check that blocked sync when a target directory didn't exist (e.g., `~/.claude/skills` on a fresh Claude Code install). This prevented first-time users from syncing without manually creating directories ([#87](https://github.com/runkids/skillshare/issues/87)). Sync now auto-creates missing directories and shows what it did:
  ```
  ✓ claude: merged (99 linked, 0 local, 0 updated, 0 pruned)
  ℹ   Created target directory: ~/.claude/skills
  ```
  Dry-run mode previews which directories would be created without actually creating them
- **`skillshare init` creates target directories** — when `init` detects an installed CLI tool (e.g., `~/.claude/` exists) but the skills subdirectory is missing, it now creates it automatically instead of leaving it as "not initialized"

### Web UI

- **Sync Preview shows directory creation** — the Config → Preview Sync modal and the Sync page now display a "directory created" or "directory will be created" badge per target when a target directory is auto-created
- **Sync Preview stays open after sync** — the Config → Preview Sync → Sync Now flow now shows sync results in the modal with a "Sync Complete" banner instead of immediately closing. This gives you time to review what changed before dismissing

## [0.17.5] - 2026-03-18

### New Features

#### Config Save Validation

- **Semantic validation on config save** — `PUT /api/config` now validates config semantics before writing, not just YAML syntax. Invalid configs return HTTP 400 with a descriptive error instead of saving silently and failing at sync time:
  - Source path must exist and be a directory
  - Sync mode must be `merge`, `symlink`, or `copy` (global and per-target)
  - Target paths must exist and be directories
- **CLI validation before sync** — `skillshare sync` validates config before starting. Invalid source path or sync mode exits immediately with a clear error instead of a cryptic filesystem error mid-sync
- **Config save warnings** — when saving config with non-fatal issues, the API returns `{ success: true, warnings: [...] }`. The Config page shows a warning toast with details

#### Sync Safety — No Auto-Create

- **Sync no longer auto-creates target directories** — previously, `sync` would silently `mkdir -p` any missing target path. This masked typos (e.g., `~/.cusor/skills` instead of `~/.cursor/skills`). Now sync fails fast with a clear error:
  ```
  Error: target directory does not exist: /home/user/.cusor/skills
  ```
  This applies to all sync modes (merge, copy, symlink conversion) and also to `--dry-run`
- **Dry-run path validation** — `sync --dry-run` now detects missing target paths and reports errors, matching the behavior of a real sync. Previously dry-run skipped existence checks

#### Web UI — Sync Warnings

- **Sync pre-check warnings** — the sync API response now includes a `warnings` field surfacing issues like empty source directories or missing target paths. Warnings appear as a yellow banner on the Sync page and in the Sync Preview modal
- **Dashboard full paths** — the Source Directory card on the Dashboard now shows the full absolute path instead of abbreviating with `~/`

#### Web UI — Config Save → Sync Preview

- **Preview Sync from Config page** — after saving `config.yaml` or `.skillignore`, a banner appears above the editor offering to preview what sync will do. Click "Preview Sync" to open a modal showing a dry-run per target — which skills will be linked, updated, or pruned — before committing to the real sync:
  ```
  Save config → Banner: "Config updated — preview what sync will do?"
    → Modal shows dry-run results per target (compact badge view)
    → "Sync Now" to confirm, or Cancel to walk away
  ```
  The banner auto-dismisses when you start editing again. Handles edge cases: no targets configured, everything already in sync, API errors with retry, and a refresh button to re-run the dry-run

#### Web UI — Base Path for Reverse Proxy

- **`--base-path` flag** — serve the Web UI under a sub-path behind a reverse proxy (e.g., Nginx, Caddy):
  ```bash
  skillshare ui --base-path /skillshare    # UI at http://host/skillshare/
  ```
  Also configurable via `SKILLSHARE_UI_BASE_PATH` environment variable. All API routes, static assets, and client-side navigation automatically adjust to the base path

#### Skill Design Patterns — `skillshare new` Wizard

- **Design pattern templates** — `skillshare new` now offers five structural design patterns for your skills, each with a tailored SKILL.md template and recommended directory structure:

  | Pattern | Description |
  |---------|-------------|
  | `tool-wrapper` | Teach agent how to use a library/API |
  | `generator` | Produce structured output from a template |
  | `reviewer` | Score/audit against a checklist |
  | `inversion` | Agent interviews user before acting |
  | `pipeline` | Multi-step workflow with checkpoints |

- **Interactive wizard** — running `skillshare new my-skill` without flags launches a TUI wizard that guides you through pattern, category, and directory scaffolding. Esc goes back to the previous step:
  ```bash
  skillshare new my-skill              # Interactive wizard
  skillshare new my-skill -P reviewer  # Skip wizard, use reviewer pattern directly
  skillshare new my-skill -P none      # Plain template (previous behavior)
  ```
- **Category tagging** — optionally tag your skill with a use-case category (library, verification, data, automation, scaffold, quality, cicd, runbook, infra) stored as a `category:` frontmatter field
- **Scaffold directories** — when using a pattern, the wizard offers to create recommended subdirectories (`references/`, `assets/`, `scripts/`) with `.gitkeep` placeholders. Auto-created when using `-P`

#### Web UI — New Skill Wizard

- **Create skills from the dashboard** — the Skills page now has a **"+ New Skill"** button that opens a step-by-step wizard at `/skills/new`:
  ```
  Name → Pattern → Category → Scaffold → Confirm
  ```
  The wizard dynamically skips steps based on your choices — selecting "none" pattern goes straight to confirm. Pattern and category selection use card grids with descriptions. Scaffold directories are toggle cards (all on by default). On success, navigates to the new skill's detail page
- **`GET /api/skills/templates`** — new endpoint returning available patterns and categories. Used by the wizard, also available for custom integrations
- **`POST /api/skills`** — new endpoint to create a skill with name, pattern, category, and scaffold directories. Validates name format, checks for duplicates (409), and validates scaffold dirs against the pattern's allowed list

#### `.skillignore.local` — Local Override

- **`.skillignore.local`** — a local-only override file that works alongside `.skillignore`. Place it in the same directory (source root or tracked repo root) to override patterns without modifying the shared `.skillignore`:
  ```bash
  # _team-repo/.skillignore blocks private-*
  # _team-repo/.skillignore.local un-ignores your own:
  echo '!private-mine' > _team-repo/.skillignore.local
  skillshare sync   # private-mine is now discovered
  ```
  Patterns from `.skillignore.local` are appended after `.skillignore`, so `!negation` rules naturally override the base file. Works at both the source root and repo level
- **CLI indicators** — `sync`, `status`, and `doctor` show `.local active` when a `.skillignore.local` is in effect. JSON output includes `.local` file paths in the `files` array

#### `metadata.targets` — Ecosystem-Aligned Frontmatter

- **`metadata.targets`** — the `targets` field in SKILL.md can now be placed under a `metadata:` block, aligning with the emerging agent skill ecosystem convention used across 30+ AI CLI tools:
  ```yaml
  ---
  name: claude-prompts
  description: Prompt patterns for Claude Code
  metadata:
    targets: [claude]
  ---
  ```
  The top-level `targets:` format continues to work. If both are present, `metadata.targets` takes priority. This is a backward-compatible change — existing skills require no modification

#### New Target: Hermes Agent

- **Hermes Agent** — [Nous Research](https://hermes-agent.nousresearch.com/)'s CLI is now a built-in target (56+ total). Global: `~/.hermes/skills`, Project: `.hermes/skills`

## [0.17.4] - 2026-03-17

### New Features

#### Doctor JSON Output

- **`doctor --json`** — structured JSON output for CI pipelines and automation. Returns per-check results with status, message, and details:
  ```bash
  skillshare doctor --json
  skillshare doctor --json | jq '.summary'          # Quick summary
  skillshare doctor --json | jq -e '.summary.errors == 0'  # CI gate
  ```
  Exit code 1 when errors are found, 0 for warnings-only or all-pass

#### Web UI — Health Check Page

- **Health Check page** — new dashboard page at `/doctor` showing environment diagnostics with summary cards (pass/warnings/errors), filter toggles, expandable check details, and version info. Access from the sidebar under "System → Health Check"
- **`GET /api/doctor`** — new API endpoint returning the same structured JSON as `doctor --json`

#### Root-Level .skillignore

- **Root-level `.skillignore`** — place a `.skillignore` file in the source root (e.g. `~/.config/skillshare/skills/.skillignore`) to hide skills and directories from all commands. Previously `.skillignore` only worked inside tracked repos (`_repo/.skillignore`); now it works at both levels:
  ```bash
  # ~/.config/skillshare/skills/.skillignore
  draft-*          # Hide all draft skills
  _archived/       # Hide entire directory
  ```
- **SkipDir optimization** — directories matching `.skillignore` patterns are now skipped entirely during discovery (not entered), improving performance for large source trees

#### Full Gitignore Syntax for .skillignore

- **Gitignore-compatible pattern matching** — `.skillignore` now supports the full gitignore syntax instead of just exact names, prefixes, and trailing `*`. New supported features:

  | Pattern | Example | Behavior |
  |---------|---------|----------|
  | `**` | `**/test` | Match at any directory depth |
  | `?` | `?.md` | Match a single character |
  | `[abc]` | `[Tt]est` | Character class |
  | `!pattern` | `!important` | Negation — un-ignore a previously ignored skill |
  | `/pattern` | `/root-only` | Anchored to the .skillignore location |
  | `pattern/` | `build/` | Match directories only |
  | `\#`, `\!` | `\#file` | Escaped literal characters |

  ```bash
  # .skillignore — now supports gitignore syntax
  **/temp              # Ignore "temp" at any depth
  test-*               # Ignore all test- prefixed skills
  !test-important      # But keep test-important
  vendor/              # Ignore vendor directories only
  [Dd]raft*            # Character class matching
  ```
- **Parent directory inheritance** — if a directory is ignored, all its contents are automatically ignored too. `vendor` in `.skillignore` will exclude `vendor/lib/deep/skill` without needing `vendor/**`
- **Safe directory skipping with negation** — when negation patterns (`!`) are present, skillshare avoids skipping parent directories prematurely, ensuring negated skills inside ignored directories are still discovered

#### .skillignore Visibility

- **`status` shows .skillignore info** — when a `.skillignore` file exists, `status` now displays an extra line below the source path showing active pattern count and ignored skill count:
  ```
  Source: ~/.config/skillshare/skills (12 skills)
    .skillignore: 5 patterns, 3 skills ignored
  ```
- **`status --json` includes skillignore field** — the `source` object in JSON output now includes a `skillignore` field with `active`, `files`, `patterns`, `ignored_count`, and `ignored_skills`
- **`doctor --json` skillignore check** — a new `skillignore` check appears in the doctor output. Shows `pass` with pattern/ignored counts when `.skillignore` exists, or `info` status when absent
- **`info` status in doctor** — new fourth status alongside `pass`/`warning`/`error` for informational checks that are neither passing nor failing. Does not count toward errors or warnings

#### Web UI — .skillignore Editor

- **Config page tabs** — the Config page now has two tabs: `config.yaml` and `.skillignore`, switchable via the same pill toggle used on the Skills and Doctor pages. Each tab has independent save state
- **`.skillignore` editor** — full CodeMirror text editor for `.skillignore` with live stats showing how many skills are currently ignored. Below the editor, an "Ignored Skills" summary shows which skills are excluded
- **`GET/PUT /api/skillignore`** — new API endpoints for reading and writing the `.skillignore` file with ignore statistics

#### Web UI — Doctor Page Unification

- **SegmentedControl filter** — the Doctor page filter toggles (All/Error/Warning/Pass) now use the same `SegmentedControl` component as the Skills page, replacing the previous hand-styled buttons for visual consistency

#### Sync — .skillignore Ignored Skills

- **`sync` shows ignored skills** — after sync completes, the CLI now lists skills excluded by `.skillignore` with a source hint showing whether ignores come from the root-level file, repo-level files, or both:
  ```
  7 skill(s) ignored by .skillignore:
    • _team/vendor/lib
    • _team/feature-radar
    • draft-wip
    (from root .skillignore + 1 repo-level file)
  ```
- **`sync --json` includes `ignored_count` and `ignored_skills`** — JSON output now includes the full list of ignored skills for automation and scripting
- **`doctor` shows skillignore status** — the Checking Environment section now displays `.skillignore` pattern count and ignored skill count (was previously JSON-only)
- **Web UI Sync page** — a collapsible "Ignored by .skillignore" card appears on the Sync page showing which skills were excluded and whether the ignores come from root or repo-level files. An "ignored" badge also appears in the pending changes summary

#### Doctor Output Readability

- **Visual spacing in `doctor` output** — config directory paths, source/environment checks, and skill validation checks are now separated by blank lines for easier scanning
- **Duplicate skill names truncated** — when targets have overlapping skills isolated by filters, `sync` now shows only the first 5 names instead of dumping all (e.g., 14000+) on a single line

### Bug Fixes

- **`.skillignore` respected in all discovery paths** — `.skillignore` patterns were not applied during source discovery, causing `doctor` to report false "unverifiable (no metadata)" warnings for intentionally excluded directories (e.g., `.venv/` inside tracked repos). Discovery now consistently honors `.skillignore` across all commands ([#83](https://github.com/runkids/skillshare/issues/83))
- **`.skillignore` directory-only patterns during install** — patterns with trailing slash (e.g., `demo/`) now correctly match directories during `skillshare install` discovery, not just during sync
- **Quieter integrity checks** — `doctor` no longer warns about locally-created skills missing metadata (this is expected). Only installed skills with incomplete metadata are flagged, with skill names listed for easy identification
- **Doctor check labels** — the web UI Health Check page shows human-readable labels ("Source Directory", "Sync Status") instead of raw identifiers (`source`, `sync_drift`)
- **Doctor global mode in web UI** — `skillshare ui -g` now correctly passes `-g` to the doctor subprocess, preventing it from auto-detecting project mode when the server's working directory contains `.skillshare/config.yaml`

## [0.17.3] - 2026-03-16

### New Features

#### Centralized Skills Repo

- **`--config local` for project init** — `skillshare init -p --config local` gitignores `config.yaml` so each developer manages their own targets independently. Skills are shared via git, config stays local:
  ```bash
  # Creator: set up shared skills repo
  skillshare init -p --config local --targets claude
  skillshare install <skill> -p && git push

  # Teammate: clone and configure own targets
  git clone <repo> && cd <repo>
  skillshare init -p
  skillshare target add myproject ~/DEV/myproject/.claude/skills -p
  ```
- **Smart shared repo detection** — when a teammate clones a shared skills repo and runs `skillshare init -p`, skillshare auto-detects the shared repo pattern (config.yaml in .gitignore) and creates an empty config with guided next steps. No `--config local` flag needed for cloners

#### Init Source Path Prompt

- **Interactive source path customization** — `skillshare init` now asks whether you want to customize the source directory path instead of silently using the default (`~/.config/skillshare/skills/`). Use `--source` to skip the prompt in scripts:
  ```bash
  skillshare init                          # Prompts for source path
  skillshare init --source ~/my-skills     # Skips prompt
  ```

#### Target List Interactive TUI

- **Interactive target browser** — `skillshare target list` now launches a full-screen TUI with a split panel layout (target list on the left, detail panel on the right). Includes fuzzy filtering via `/` and keyboard navigation:
  ```bash
  skillshare target list           # Interactive TUI (default on TTY)
  skillshare target list --no-tui  # Plain text output
  ```
- **Mode picker** — press `M` on any target to change its sync mode (merge, copy, symlink) without leaving the TUI. Changes are saved to config immediately
- **Include/Exclude editor** — press `I` or `E` to open an inline pattern editor for the selected target. Add patterns with `a`, delete with `d` — changes persist to config on each action

#### Web UI — Filter Studio

- **Filter Studio page** — new dedicated page for managing target include/exclude filters at `/targets/{name}/filters`. Two-column layout: edit glob patterns on the left, see a live preview of which skills will sync on the right. Click any skill in the preview to toggle it between include/exclude:
  ```
  Dashboard → Targets → Customize filters → (Filter Studio opens)
  ```
- **Always-visible filter summary** — every target card on the Targets page now permanently displays a skill count line (`All 18 skills` or `12/18 skills`) with filter tag previews (max 3 tags, `+N more` for overflow). Replaces the hidden ghost "Filters" button that nobody noticed
- **Skill Detail — Target Distribution** — the skill detail sidebar now shows a "Target Distribution" card listing which targets this skill syncs to, with status indicators (synced, excluded, not included, SKILL.md targets mismatch). Links to Filter Studio for editing
- **Live preview with search** — Filter Studio's preview panel includes a search box to quickly find skills in the list, and updates in real-time (500ms debounce) as you add or remove patterns
- **`GET /api/sync-matrix`** — new API endpoint returning the authoritative skill × target sync matrix with status and reason for each entry. Supports `?target=` filter. `POST /api/sync-matrix/preview` accepts draft patterns for what-if preview without saving
- **Auto-commit on blur** — `FilterTagInput` now automatically adds the typed pattern when the input loses focus, preventing the common mistake of typing a pattern but forgetting to press Enter

#### Web UI — Skill Detail Styling

- **Post-it sidebar cards** — Metadata, Files, Security, Target Distribution, and Target Sync cards in the skill detail sidebar now use semantic pastel backgrounds in playful theme (yellow, green, blue, cyan) with thumbtack pin decorations. Clean theme uses white backgrounds
- **Hand-drawn manifest block** — the SKILL.md manifest area uses a sketchy dashed border with tape decoration in playful theme
- **Unified input borders** — all text inputs, textareas, selects, and tag inputs now use consistent `border-2 border-muted` styling with `focus:border-pencil` across the dashboard

### Bug Fixes

- **Web UI network error guidance** — the web dashboard now shows a clear "restart `skillshare ui`" message when the API server is unreachable, instead of a generic "Failed to fetch" error
- **`init --help` completeness** — `skillshare init --help` now shows the `--subdir` flag and lists flags in the same order as the documentation
- **Project trash gitignore** — `skillshare init -p` now automatically adds `trash/` to `.skillshare/.gitignore`, preventing soft-deleted skills from being accidentally committed. Existing projects are patched on the next `uninstall` run
- **Web UI target filter persistence** — target include/exclude filters set via the web dashboard are now correctly persisted; previously, in-memory state could drift from disk after saving, causing subsequent page loads to show stale filter values
- **Web UI extras list empty state** — the extras list page now renders correctly when no extras are configured, fixing a missing tour target in the empty state
- **Partial init repair auto-select** — `skillshare init -p` now automatically selects all detected targets when repairing a partial initialization (`.skillshare/` exists but `config.yaml` is missing), instead of prompting you to pick from a checklist
- **Target list TUI help bar** — scroll hints (`Ctrl+d/u`) and help bar key ordering now follow the same convention as other TUIs (navigate → filter → scroll → actions → quit)
- **Web UI server stability** — fixed a potential crash when concurrent API requests (e.g., multiple browser tabs) hit the dashboard while target config was being modified

## [0.17.2] - 2026-03-14

### New Features

#### Web UI Git Sync Enhancements

- **Repository Info Card** — the Git Sync page now displays a card showing the current repository URL, branch, and latest commit at the top of the page
- **Branch switcher** — switch between git branches directly from the Git Sync page without leaving the web dashboard

#### Web UI Backup & Sidebar

- **Backup page restore UX** — improved restore flow with clearer action buttons and confirmation dialogs
- **Collapsible sidebar tools** — sidebar tool sections can now be collapsed/expanded to reduce visual clutter

#### CLI TUI Improvements

- **Trash TUI split panel** — the trash list now uses the same left-right split panel layout as other TUIs, with a detail panel showing item info alongside the list

### Bug Fixes

- **Backup command spinner** — `skillshare backup` now shows a progress spinner during backup operations instead of appearing frozen
- **Git Sync footer layout** — Push/Pull action buttons are now pinned to the bottom of the page and no longer shift when content changes

### Performance

- **Restore TUI async size calculation** — backup version sizes are now computed asynchronously in the background instead of blocking the TUI. Large backups with many versions no longer freeze the interface when browsing or selecting versions. Detail panel I/O is capped at 20 skills to prevent lag on large backups

## [0.17.1] - 2026-03-13

### New Features

#### Web UI Theme System

- **Multi-theme support** — the web dashboard now offers two visual styles and three color modes, switchable via the **Theme** button in the sidebar:
  - **Styles**: `Clean` (professional, minimal) and `Playful` (hand-drawn borders, organic shapes)
  - **Modes**: `Light`, `Dark`, and `System` (follows OS preference)
  - Preferences persist in localStorage across sessions. Default: Playful + Light

#### Init Source Subdirectory

- **Subdirectory prompt during init** — `skillshare init` now prompts whether to store skills in a subdirectory instead of the repository root. Useful when embedding skills inside a dotfiles or monorepo:
  ```bash
  skillshare init --remote git@github.com:you/dotfiles.git --subdir skills
  ```
  This sets the source path to `~/.config/skillshare/skills/skills/`, keeping the repo root free for README, CI config, and other non-skill files

### Bug Fixes

- **Collect skips `.git/` directories** — `skillshare collect` now excludes `.git/` when copying skills from target to source. Previously, collecting a git-cloned repo (e.g., `obra/superpowers` in `~/.cursor/skills/`) could produce only empty directories because `filepath.Walk` would abort on `.git/` pack files
- **Actionable git error messages** — `skillshare install` and `skillshare update` now show context-specific guidance instead of raw exit codes when git operations fail:
  - Authentication failures suggest token env vars (`GITHUB_TOKEN`, `GITLAB_TOKEN`, etc.), SSH URLs, or `gh auth login`
  - SSL certificate errors suggest custom CA bundle, SSH, or `GIT_SSL_NO_VERIFY`
  - Token rejections distinguish between missing auth and expired/invalid tokens
  - Divergent branch conflicts show the `fatal:` line instead of just `exit status 128`

## [0.17.0] - 2026-03-11

### Breaking Changes

- **Extras directory structure** — extras source files are now stored under `extras/<name>/` instead of directly under the config root. Existing directories are **auto-migrated** on first `sync extras` run. No manual action required

### New Features

#### First-Class Extras Command Group

Extras (non-skill resources like rules, prompts, commands) are now a first-class feature with their own command group:

- **`extras init`** — create a new extra with interactive TUI wizard or CLI flags:
  ```bash
  skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules
  skillshare extras init prompts --target .claude/prompts --mode copy -p
  ```
- **`extras list`** — view all configured extras with sync status (`synced`, `drift`, `not synced`, `no source`). Interactive TUI with split-pane detail view, or `--json` / `--no-tui` output
- **`extras mode`** — change sync mode of an extra's target from CLI, TUI (`M` key), or Web UI:
  ```bash
  skillshare extras rules --mode copy                   # single target auto-resolved
  skillshare extras mode rules --target ~/.claude/rules --mode copy
  ```
- **`extras remove`** — remove an extra from config (source files and synced targets are preserved)
- **`extras collect`** — reverse-sync local files from a target back into the extras source directory:
  ```bash
  skillshare extras collect rules --from ~/.claude/rules --dry-run
  ```
- **Project mode** — all extras commands support `--project`/`-p` for `.skillshare/` scoped extras

#### Extras Integration with Existing Commands

- **`status`** — shows extras file count and target count per extra
- **`doctor`** — checks that extras source directories exist and target parent directories are reachable
- **`diff`** — automatically includes per-file diff for extras targets when configured
- **`sync extras --json`** — structured JSON output for programmatic consumption
- **`sync --all -p`** — project-mode `--all` now includes extras sync

#### Web UI Redesign

The web dashboard (`skillshare ui`) received a complete visual overhaul — replacing the hand-drawn aesthetic with a clean, minimal design:

- **Redesigned design system** — new DM Sans typography, clean border-radius, streamlined color palette with proper dark mode support
- **Table view with pagination** — skills and search results now offer a table view alongside the existing card/grouped views, with client-side pagination for large collections
- **Sticky search and filters** — SkillsPage toolbar stays pinned at the top while scrolling, with grouped view sticky headers
- **Keyboard modifier shortcuts** — press `?` to see available shortcuts, with an on-screen HUD overlay showing active modifiers
- **Sync progress animation** — visual feedback during sync operations
- **Onboarding tour** — step-by-step spotlight tour for first-time users, highlighting key features
- **Shared UI components** — new DialogShell, IconButton, Pagination, and SegmentedControl components for consistent interactions across pages

#### Web UI Extras Page

- New **Extras page** in the web dashboard with list, sync, remove, add-extra modal, and inline mode dropdown per target
- **Dashboard card** showing extras count, total files, and total targets
- REST API: `GET /api/extras`, `GET /api/extras/diff`, `POST /api/extras`, `POST /api/extras/sync`, `PATCH /api/extras/{name}/mode`, `DELETE /api/extras/{name}`

#### Custom GitLab Domain Support

- **JihuLab auto-detection** — hosts containing `jihulab` in the name (e.g., `jihulab.com`) are now automatically detected alongside `gitlab`, so nested subgroup URLs work without any config
- **`gitlab_hosts` config** — declare self-managed GitLab hostnames so skillshare treats URLs with nested subgroup paths correctly. Hosts containing `gitlab` or `jihulab` in the name are detected automatically; this config is for other custom domains like `git.company.com`:
  ```yaml
  # ~/.config/skillshare/config.yaml (or .skillshare/config.yaml)
  gitlab_hosts:
    - git.company.com
    - code.internal.io
  ```
  ```bash
  # With config above, full path is treated as repo (not owner/repo + subdir)
  skillshare install git.company.com/team/frontend/ui
  ```
  Without config, append `.git` as a workaround: `git.company.com/team/frontend/ui.git`

- **`SKILLSHARE_GITLAB_HOSTS` env var** — comma-separated list of GitLab hostnames for CI/CD pipelines that don't have a config file:
  ```bash
  SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
  ```
  When both the env var and config file are set, their values are merged (deduplicated). Invalid entries in the env var are silently skipped

### Bug Fixes

- **GitLab subgroup URL parsing** — `skillshare install` now correctly handles GitLab nested subgroup URLs with arbitrary depth. Previously, URLs like `gitlab.com/group/subgroup/project` were misinterpreted as repo `group/subgroup` with subdir `project`. Now the entire path is treated as the repo path:
  ```bash
  # These all work now (previously failed)
  skillshare install gitlab.com/group/subgroup/project
  skillshare install onprem.gitlab.internal/org/sub1/sub2/project
  skillshare install https://gitlab.com/group/subgroup/project.git
  ```
  To specify a subdir within a multi-segment repo, use `.git` as the explicit boundary:
  ```bash
  # Clone group/subgroup/project, install from skills/my-skill subdir
  skillshare install gitlab.com/group/subgroup/project.git/skills/my-skill
  ```
  Non-GitLab hosts (GHE, Gitea, etc.) retain the original `owner/repo` + subdir behavior. GitLab web URLs with `/-/tree/` and Bitbucket `/src/` markers continue to work as before. `--track` mode generates correct names for subgroup paths (e.g., `group-subgroup-project`)
- **HTTPS fallback on non-GitLab hosts** — fixed platform-aware HTTPS URL parsing that could misroute GitHub Enterprise and Gitea URLs with subdirectory paths
- **Skill discovery in projects** — `skillshare install` now skips known AI tool config directories (`.claude/`, `.cursor/`, etc.) when scanning a project directory for skills, preventing circular discovery and false duplicates
- **Sync collision message** — `skillshare sync` now shows both duplicate skill names in collision warning messages for easier troubleshooting
- **Extras mode switch without `--force`** — changing an extra's sync mode (e.g., from `merge` to `copy`) and re-syncing now automatically replaces old symlinks. Previously, leftover symlinks from the old mode were treated as conflicts requiring `--force`

## [0.16.14] - 2026-03-09

### New Features

#### Terminal Rendering Improvements

- **SGR dim for consistent gray text** — all dim/gray text across CLI and TUI now uses the SGR dim attribute (`\x1b[0;2m`) instead of bright-black (`\033[90m`) or fixed 256-color grays. This adapts to any terminal theme — dark, light, or custom — instead of rendering too dark or invisible on certain configurations
- **Progress bar counter visibility** — the file counter (e.g. `0/63947`) now appears at a fixed position right after the percentage, preventing it from being pushed off-screen by long titles on narrow terminals:
  ```
  ■■■■■■■■■■■■･････ 69%  0/63947  Updating files
  ```
- **Progress bar accent color** — progress bar now uses cyan (the project accent color) instead of orange, matching spinners, titles, and other interactive elements

### Bug Fixes

- Fixed progress bar getting stuck at 99% on large scans (e.g. 63k+ skills) — parallel scan workers could race past the final frame, leaving the bar one tick short of 100%
- Fixed skill path segments (e.g. `security/` in `security/sarif-parsing`) rendering as fixed 256-color gray in TUI list and audit views — now uses theme-adaptive dim

## [0.16.13] - 2026-03-06

### New Features

#### TUI Grouped Layout

- **Grouped skill list** — `skillshare list` TUI now groups skills by tracked repo with visual separators. Each group shows the repo name and skill count. Standalone (local) skills appear in their own section. When only one group exists, separators are omitted for a cleaner view
  ```
  ── runkids-my-skills (42) ──────────────
    ✓ security/skill-improver
    ! security/audit-demo-debug-exfil
  ── standalone (27) ─────────────────────
    ! react-best-practices
  ```
- **Grouped audit results** — `skillshare audit` TUI uses the same grouped layout. Panel height dynamically adjusts based on footer content, maximizing visible rows
- **Structured filter tags** — filter skills precisely with `key:value` tags in the `/` filter input:
  ```
  t:tracked g:security audit
  → type=tracked AND group contains "security" AND free text "audit"
  ```
  Available tags: `t:`/`type:` (tracked/remote/local/github), `g:`/`group:` (substring), `r:`/`repo:` (substring). Multiple tags use AND logic. Tracked skills now show a repo-name badge so they remain identifiable even in filtered results without group headers

#### New Targets

- **3 new AI agent targets** — Warp, Purecode AI (`purecode`), and Witsy, bringing supported tools to 55+

### Bug Fixes

- Fixed long skill names wrapping to multiple lines in list and audit TUIs — names now truncate with `…` when exceeding column width
- Fixed items at the bottom of the audit TUI list being hidden behind the footer
- Fixed detail panel showing duplicate information (installed date, repo name repeated across sections)
- Reduced color noise in audit CLI and TUI output — non-zero counts use semantic severity colors, zero counts are dimmed
- Fixed devcontainer wrapper not suppressing redirect banner for `-j` short flag

## [0.16.12] - 2026-03-06

### New Features

#### Structured JSON Output

- **`--json` flag on 8 more commands** — structured JSON output for agent and CI/CD consumption, bringing total coverage to 12 commands:
  - Mutating: `sync`, `install`, `update`, `uninstall`, `collect`
  - Read-only: `target list`, `status`, `diff`
  ```bash
  skillshare status --json                          # overview as JSON
  skillshare list --json | jq '.[].name'            # extract skill names
  skillshare sync --json | jq '.details'            # per-target sync details
  skillshare install github.com/user/repo --json    # non-interactive install
  ```
  - For mutating commands, `--json` implies `--force` (skips interactive prompts)
  - Fully silent: no spinners, no stderr progress — only pure JSON on stdout
  - Previously supported: `audit --format json`, `log --json`, `check --json`, `list --json`
- **`status --project --json`** — project-mode status now supports `--json` output

### Bug Fixes

- Fixed `--json` mode leaking spinner and progress text to stderr, breaking `2>&1 | jq .` pipelines
- Fixed non-zero exit codes being swallowed in `--json` error paths
- Fixed `status --json` showing hardcoded analyzer list instead of actual active analyzers
- Fixed argument validation being skipped in `status --project` mode

### Performance

- **Parallelized git dirty checks** — `status --json` now runs git status checks concurrently across tracked repos

## [0.16.11] - 2026-03-05

### New Features

#### Supply-Chain Trust Verification

- **Metadata analyzer** — new audit analyzer that cross-references SKILL.md metadata against the actual git source URL to detect social-engineering attacks:
  - `publisher-mismatch` (HIGH): skill claims an organization (e.g., "by Anthropic") but repo owner differs
  - `authority-language` (MEDIUM): skill uses authority words ("official", "verified") from an unrecognized source
  ```bash
  skillshare audit                         # metadata analyzer runs by default
  skillshare audit --analyzer metadata     # run metadata analyzer only
  ```

#### Hardcoded Secret Detection

- **10 new audit rules** (`hardcoded-secret-0` through `hardcoded-secret-9`) detect inline API keys, tokens, and passwords embedded in skill files:
  - Google API keys, AWS access keys, GitHub PATs (classic + fine-grained), Slack tokens, OpenAI keys, Anthropic keys, Stripe keys, PEM private key blocks, and generic `api_key`/`password` assignments
  - Severity: HIGH — blocks installation at default threshold
  ```bash
  skillshare audit                         # hardcoded secrets detected automatically
  skillshare audit rules --pattern hardcoded-secret  # list all secret rules
  ```

#### Skill Integrity Verification

- **`doctor` integrity check** — verifies installed skills haven't been tampered with by comparing current file hashes against stored `.skillshare-meta.json` hashes:
  ```
  ✓ Skill integrity: 5/6 verified
  ⚠ _team-repo__api-helper: 1 modified
  ⚠ Skill integrity: 1 skill(s) unverifiable (no metadata)
  ```

#### Web UI Streaming & Virtualization

- **Real-time SSE streaming** — all long-running web dashboard operations (audit, update, check, diff) now stream results via Server-Sent Events with per-item progress bars instead of waiting for the full batch
- **Per-skill audit** — audit individual skills directly from the skill detail page
- **Virtualized scrolling** — audit results and diff item lists now use virtual scrolling for smooth performance with large datasets (replaces "Show more" pagination)

### Improvements

- **SSL error guidance** — `skillshare install` now detects SSL certificate errors and shows actionable options (custom CA bundle, SSH, or skip verification)
- **Cleaner TUI layout** — removed detail panel box borders in list/log views for a cleaner, less cluttered appearance

## [0.16.10] - 2026-03-04

### New Features

#### Sync Extras

- **`sync extras` subcommand** — sync non-skill resources (rules, commands, memory files, etc.) from your config directory to arbitrary target paths:
  ```bash
  skillshare sync extras              # sync all configured extras
  skillshare sync extras --dry-run    # preview without changes
  skillshare sync extras --force      # overwrite existing files
  ```
  Each extra supports per-target sync modes (`symlink`, `copy`, or `merge`). Configure in `config.yaml`:
  ```yaml
  extras:
    - name: rules
      targets:
        - path: ~/.claude/rules
        - path: ~/.cursor/rules
          mode: copy
  ```
- **`sync --all` flag** — run skill sync and extras sync together in one command:
  ```bash
  skillshare sync --all
  ```

#### TUI Preferences

- **`tui` subcommand** — persistently enable or disable interactive TUI mode:
  ```bash
  skillshare tui          # show current setting
  skillshare tui off      # disable TUI globally
  skillshare tui on       # re-enable TUI
  ```
  When disabled, all commands fall back to plain text output. Setting is stored in `config.yaml`.

### Bug Fixes

- Fixed TUI detail panel bottom content being clipped in list view

### Documentation

- Added sync extras documentation to website, built-in skill, and README
- Split monolith audit page into focused sub-pages for easier navigation

## [0.16.9] - 2026-03-03

### New Features

#### Audit Rules Management

- **`audit rules` subcommand** — browse, search, disable, enable, and override severity for individual rules or entire patterns:
  ```bash
  skillshare audit rules                          # interactive TUI browser
  skillshare audit rules --format json             # machine-readable listing
  skillshare audit rules disable credential-access-ssh-private-key
  skillshare audit rules disable --pattern prompt-injection
  skillshare audit rules severity my-rule HIGH
  skillshare audit rules reset                     # restore built-in defaults
  skillshare audit rules init                      # create starter audit-rules.yaml
  ```
- **Audit Rules TUI** — two-level interactive browser with accordion pattern groups, severity tabs (ALL/CRIT/HIGH/MED/LOW/INFO/OFF), text filter, and inline disable/enable/severity-override actions
- **Pattern-level rule overrides** — `audit-rules.yaml` now supports pattern-level entries (e.g., `prompt-injection: disabled: true`) that apply to all rules under a pattern

#### Security Policy & Deduplication

- **`--profile` flag** — preset security profiles that set block threshold and deduplication mode in one flag:
  ```bash
  skillshare audit --profile strict      # blocks on HIGH+, global dedupe
  skillshare audit --profile permissive  # blocks on CRITICAL only, legacy dedupe
  ```
  Profiles: `default` (CRITICAL threshold, global dedupe), `strict` (HIGH threshold, global dedupe), `permissive` (CRITICAL threshold, legacy dedupe)
- **`--dedupe` flag** — control finding deduplication: `global` (default) deduplicates across all skills using SHA-256 fingerprints; `legacy` keeps per-skill behavior
- **Policy display** — active policy (profile, threshold, dedupe mode) shown in audit header, summary box, and TUI footer

#### Analyzer Pipeline

- **`--analyzer` flag** — run only specific analyzers (repeatable): `static`, `dataflow`, `tier`, `integrity`, `structure`, `cross-skill`:
  ```bash
  skillshare audit --analyzer static --analyzer dataflow
  ```
- **Finding enrichment** — JSON, SARIF, and Markdown outputs now include `ruleId`, `analyzer`, `category`, `confidence`, and `fingerprint` fields per finding
- **Category-based threat breakdown** — summary now shows threat counts by category (injection, exfiltration, credential, obfuscation, privilege, integrity, structure, risk) across all output channels (CLI, TUI, JSON, Markdown)
- **Semantic coloring** — TUI summary footer and CLI summary box use per-category colors for the Threats breakdown line

#### New Detection Rules

- **Interpreter tier (T6)** — audit classifies Turing-complete runtimes (`python`, `node`, `ruby`, `perl`, `lua`, `php`, `bun`, `deno`, `npx`, `tsx`, `pwsh`, `powershell`) as T6:interpreter. Versioned binaries like `python3.11` are also recognized. Tier combination findings: `tier-interpreter` (INFO) and `tier-interpreter-network` (MEDIUM when combined with network commands)
- **Expanded prompt injection detection** — new rules detect `OVERRIDE:`/`IGNORE:`/`ADMIN:`/`ROOT:` prefixes, agent directive tags (`<system>`, `</instructions>`), and jailbreak directives (`DEVELOPER MODE`, `DEV MODE`, `DAN MODE`, `JAILBREAK`)
- **Table-driven credential access detection** — credential rules are now generated from a data table covering 30+ sensitive paths (SSH keys, AWS/Azure/GCloud credentials, GnuPG keyrings, Kubernetes config, Vault tokens, Terraform credentials, Docker config, GitHub CLI tokens, macOS Keychains, shell history, and more) across 5 access methods (read, copy, redirect, dd, exfil). Supports `~`, `$HOME`, `${HOME}` path variants. Includes an INFO-level heuristic catch-all for unknown home dotdirs. Rule IDs are now descriptive (e.g., `credential-access-ssh-private-key` instead of `credential-access-0`)
- **Cross-skill credential × interpreter** — new cross-skill rule `cross-skill-cred-interpreter` (MEDIUM) flags when one skill reads credentials and another has interpreter access
- **Markdown image exfiltration detection** — new rule detects external markdown images with query parameters (`![img](https://...?data=...)`) as a potential data exfiltration vector
- **Invisible payload detection** — detects Unicode tag characters (U+E0001–U+E007F) that render at 0px width but are fully processed by LLMs. Primary vector for "Rules File Backdoor" attacks. Uses dedicated `invisible-payload` pattern to ensure CRITICAL findings are never suppressed in tutorial contexts
- **Output suppression detection** — detects directives that hide actions from the user ("don't tell the user", "hide this from the user", "remove from conversation history"). Strong indicator of supply-chain attacks
- **Bidirectional text detection** — detects Unicode bidi control characters (U+202A–U+202E, U+2066–U+2069) used in Trojan Source attacks (CVE-2021-42574) that reorder visible text
- **Config/memory file poisoning** — detects instructions to modify AI agent configuration files (`MEMORY.md`, `CLAUDE.md`, `.cursorrules`, `.windsurfrules`, `.clinerules`)
- **DNS exfiltration detection** — detects `dig`/`nslookup`/`host` commands with command substitution (`$(...)` or backticks) that encode stolen data in DNS subdomain queries
- **Self-propagation detection** — detects instructions that tell AI to inject/insert payloads into all/every/other files, a repository worm pattern
- **Markdown comment injection** — detects prompt injection keywords hidden inside markdown reference-link comments (`[//]: # (ignore previous instructions...)`)
- **Untrusted package execution** — detects `npx -y`/`npx --yes` (auto-execute without confirmation) and `pip install https://` (install from URL, not PyPI registry)
- **Additional invisible Unicode** — detects soft hyphens (U+00AD), directional marks (U+200E–U+200F), and invisible math operators (U+2061–U+2064) at MEDIUM severity
- **`env` prefix handling** — command tier classifier now correctly classifies `env python3 script.py` as T6:interpreter instead of T0:read-only

### Performance

- **Regex prefilters** — static analyzer now applies conservative literal-substring prefilters before running regex, reducing scan time on large skills

### Bug Fixes

- **Regex bypass vulnerabilities closed** — fixed prompt injection rules that could be bypassed with leading whitespace or mixed case; fixed data-exfiltration image rule whose exclude pattern allowed `.png?stolen_data` to pass; fixed `dd if=/etc/shadow` being mislabeled as `destructive-commands` instead of `credential-access`
- **SSH public key false positive** — `~/.ssh/id_rsa.pub` and other `.pub` files no longer trigger CRITICAL credential-access findings (only private keys are flagged)
- **Catch-all regex bypass** — fixed heuristic catch-all rule that could be silenced when a known credential path appeared on the same line as an unknown dotdir
- **Structured output ANSI leak** — `audit --format json/sarif/markdown` no longer leaks pterm cursor hide/show ANSI codes into stdout
- **Severity-only merge no longer wipes rules** — editing only severity in `audit-rules.yaml` no longer drops the rule's regex patterns
- **Profile threshold fallback** — profile presets now correctly set block threshold when config has no explicit `block_threshold`
- **TreeSpinner ghost cursor** — fixed missing `WithWriter` that caused cursor hide/show codes to leak on structured output
- **TUI summary overflow** — category threat breakdown now renders on a separate line to prevent horizontal overflow on narrow terminals

## [0.16.8] - 2026-03-02

### New Features

- **`audit --format`** — new `--format` flag supports `text` (default), `json`, `sarif`, and `markdown` output formats. `--json` is now deprecated:
  ```bash
  skillshare audit --format sarif     # SARIF 2.1.0 for GitHub Code Scanning
  skillshare audit --format markdown  # Markdown report for GitHub Issues/PRs
  skillshare audit --format json      # Machine-readable JSON
  ```
- **Analyzability score** — each audited skill now receives an analyzability percentage (how much of the skill's content can be statically analyzed). Shown per-skill in audit output and as an average in the summary
- **Command safety tiering (T0–T5)** — audit classifies shell commands by behavioral tier: T0 read-only, T1 mutating, T2 destructive, T3 network, T4 privilege, T5 stealth. Tier labels appear alongside pattern-based findings for richer context
- **Dataflow taint tracking** — audit detects cross-line exfiltration patterns: credential reads or environment variable access on one line followed by network sends (`curl`, `wget`, etc.) on a subsequent line
- **Cross-skill interaction detection** — when auditing multiple skills, audit now checks for dangerous capability combinations across skills (e.g., one skill reads credentials while another has network access). Results are also exposed in the REST API (`GET /api/audit`)
- **Audit TUI filter** — the `/` filter in the audit TUI now searches across risk level, status (blocked/warning/clean), max severity, finding pattern names, and file names — not just skill names
- **Pre-commit hook** — `.pre-commit-hooks.yaml` for the [pre-commit](https://pre-commit.com/) framework. Runs `skillshare audit -p` on every commit to catch security issues before they land:
  ```yaml
  repos:
    - repo: https://github.com/runkids/skillshare
      rev: v0.16.8
      hooks:
        - id: skillshare-audit
  ```
- **AstrBot target** — new target for AstrBot AI assistant (`~/.astrbot/data/skills`)
- **Cline target updated** — Cline now uses the universal `.agents/skills` project path

### Performance

- **Cross-skill analysis O(N) rewrite** — cross-skill interaction detection rewritten from O(N²) pair-wise comparison to O(N) capability-bucket approach, significantly faster for large skill collections

### Bug Fixes

- **TUI gray text contrast** — improved gray text readability on dark terminals by increasing ANSI color contrast
- **Spinner on structured output** — `audit` now shows progress spinner on stderr when using `--format json/sarif/markdown`, so structured stdout remains clean for piping
- **SARIF line-0 region** — SARIF output no longer emits an invalid `region` object for findings at line 0

## [0.16.7] - 2026-03-02

### Bug Fixes

- **Preserve external symlinks during sync** — sync (merge/copy mode) no longer deletes target directory symlinks created by dotfiles managers (e.g., stow, chezmoi, yadm). Previously, switching from symlink mode to merge/copy mode would unconditionally remove the target symlink, breaking external link chains. Now skillshare checks whether the symlink points to the source directory before removing it — external symlinks are left intact and skills are synced into the resolved directory
- **Symlinked source directory support across all commands** — all commands that walk the source directory (`sync`, `update`, `uninstall`, `list`, `diff`, `install`, `status`, `collect`) now resolve symlinks before scanning. Skills managed through symlinked `~/.config/skillshare/skills/` (common with dotfiles managers) are discovered correctly everywhere. Chained symlinks (link → link → real dir) are also handled
- **Group operation containment guard** — `uninstall --group` and `update --group` now reject group directories that are symlinks pointing outside the source tree, preventing accidental operations on external directories
- **`status` recognizes external target symlinks** — `CheckStatusMerge` no longer reports external symlinks as "conflict"; it follows the symlink and counts linked/local skills in the resolved directory
- **`collect` scans through external target symlinks** — `FindLocalSkills` now follows non-source symlinks instead of skipping them, so local skills in dotfiles-managed target directories can be collected
- **`upgrade` prompt cleanup** — upgrade prompts ("Install built-in skill?" and "Upgrade to vX?") no longer leave residual lines that break the tree-drawing layout

## [0.16.6] - 2026-03-02

### New Features

- **`diff` interactive TUI** — new bubbletea-based split-panel interface for `skillshare diff`: left panel lists targets with status icons (✓/!/✗), right panel shows categorized file-level diffs for the selected target. Supports fuzzy filter (`/`), detail scrolling (`Ctrl+d/u`), and narrow terminal fallback. Add `--no-tui` for plain text output
- **`diff --patch`** — show unified text diffs for each changed file:
  ```
  skillshare diff --patch
  ```
- **`diff --stat`** — show per-file change summary with added/removed line counts:
  ```
  skillshare diff --stat
  ```
- **`diff` file-level detail** — diff entries now include per-file data (added/removed/modified/renamed), source paths, modification times, and git-style status symbols (`+`/`−`/`≠`/`→`)
- **`diff` statistics summary** — every diff run prints a summary line with total counts by category (e.g., `3 added, 1 modified, 2 removed`)
- **Glob pattern matching** — `install`, `update`, and `uninstall` now accept glob patterns (`*`, `?`, `[...]`) in skill name arguments; matching is case-insensitive:
  ```bash
  skillshare install repo -s "core-*"
  skillshare update "team-*"
  skillshare uninstall "old-??"
  ```
- **`trash` interactive TUI** — bubbletea-based TUI with multi-select, fuzzy filter, and inline restore/delete/empty operations; includes SKILL.md preview in the detail panel
- **`restore` interactive TUI** — two-phase TUI: target picker → version list with left-right split panel, showing skill diffs and descriptions in the detail panel. Add `--help` flag and delete-backup action from TUI
- **`backup` version listing** — `backup` now lists available backup versions per target and correctly follows top-level symlinks in merge-mode targets
- **Homebrew-aware version check** — Homebrew users no longer see false "update available" notifications; `doctor` and post-command checks now query `brew info` instead of the GitHub Release API when installed via Homebrew
- **Devcontainer skill** — new built-in skill that teaches AI assistants when and how to run CLI commands, tests, and debugging inside the devcontainer
- **Red destructive confirmations** — all destructive action confirmations (delete, empty, uninstall) now render in red across trash, restore, and list TUIs

### Fixed

- **`backup`/`restore` mode flags** — `-g` and `-p` flags now work correctly; previously `-g` was misinterpreted as a target name
- **`diff` hides internal metadata** — `.skillshare-meta.json` is no longer shown in file-level diff output
- **`diff --stat` implies `--no-tui`** — `--stat` now correctly skips the TUI and prints to stdout

## [0.16.5] - 2026-02-28

### New Features

- **Web UI: Dark theme** — toggle between light and dark mode via the sun/moon button; persists to localStorage and respects `prefers-color-scheme`
- **Web UI: Update page** — dedicated page for batch-updating tracked skills with select-all, per-item progress tracking, and result summary
- **Web UI: Security overview card** — dashboard now shows a risk-level badge and severity breakdown; highlights critical findings with an accent card
- **Web UI: Sync mode selector** — change a target's sync mode (merge/symlink) directly from the Targets page dropdown
- **Web UI: Install skill picker** — skill descriptions from SKILL.md frontmatter are now shown inline in the picker modal; search also matches descriptions
- **`upgrade` version transition** — `skillshare upgrade` now shows clear before/after versions:
  ```
  Upgraded  v0.16.3 → v0.16.5
  ```
  Works for Homebrew, direct download, and skill installs

### Fixed

- **Custom targets flagged as unknown** — `check` and `doctor` no longer warn about user-defined targets in global or project config (fixes [#57](https://github.com/runkids/skillshare/issues/57))
- **Web UI: Modal scroll-away** — clicking checkboxes in the skill picker no longer causes content to scroll out of view (replaced `overflow-hidden` with `overflow-clip`)
- **Web UI: Subdir URL discovery** — install form now correctly discovers skills from git subdirectory URLs
- **Web UI: Accessibility** — added `aria-labels`, `htmlFor`, focus trap for modals, and `ErrorBoundary` for graceful error recovery

### New Targets

- **omp** — [oh-my-pi](https://github.com/can1357/oh-my-pi) (`~/.omp/agent/skills`, `.omp/skills`; alias: `oh-my-pi`)
- **lingma** — [Lingma](https://help.aliyun.com/zh/lingma/user-guide/skills) (`~/.lingma/skills`, `.lingma/skills`)

## [0.16.4] - 2026-02-28

### New Features

- **Cross-path duplicate detection** — `install` now detects when a repo is already installed at a different location and blocks the operation with a clear hint:
  ```bash
  skillshare install runkids/feature-radar --into feature-radar
  # later...
  skillshare install runkids/feature-radar
  # ✗ this repo is already installed at skills/feature-radar/scan (and 2 more)
  #   Use 'skillshare update' to refresh, or reinstall with --force to allow duplicates
  ```
- **Same-repo skip** — reinstalling a skill from the same repo now shows a friendly `⊘ skipped` indicator instead of an error; skipped skills are grouped by directory with repo label in the summary
- **Web UI install dedup** — the Web UI install endpoints enforce the same cross-path duplicate check as the CLI, returning HTTP 409 when duplicates are found
- **5 new audit rules** — the security scanner now detects 36 patterns (up from 31):
  - `fetch-with-pipe` (HIGH) — detects `curl | bash`, `wget | sh`, and pipes to `python`, `node`, `ruby`, `perl`, `zsh`, `fish`
  - `ip-address-url` (MEDIUM) — URLs with raw IP addresses that bypass DNS-based security; private/loopback ranges excluded
  - `data-uri` (MEDIUM) — `data:` URIs in markdown links that may embed executable content
- **Unified batch summary** — `install`, `uninstall`, and `update` now share a consistent single-line summary format with color-coded counts and elapsed time

### Performance

- **Batch gitignore operations** — `.gitignore` updates during `install` reconciliation and `uninstall` are now batched into a single file read/write instead of one per skill; eliminates hang when `.gitignore` grows large (100K+ lines)
- **`update --all` grouped skip** — skills from the same repo are now skipped when installed metadata already matches remote state (commit or tree-hash match), avoiding redundant reinstall/copy; on large repos this eliminates the majority of work
- **`update --all` batch speed** — removed a fixed 50ms per-skill delay in grouped batch iteration that dominated runtime on large skill sets (~90 min at 108K skills → seconds)
- **`update --all` progress visibility** — batch progress bar now advances per-skill instead of per-repo, so it no longer appears stuck at 0% during large grouped updates; a scanning spinner and phase headers (`[1/3] Pulling N tracked repos...`) show which stage is running
- **`status` and `doctor` at scale** — both commands now run a single skill discovery pass instead of repeating it per-section (status: 7× → 1×, doctor: 5× → 1×); target status checks are cached so drift detection reuses the first result; `doctor` overlaps its GitHub version check with local I/O; a spinner is shown during discovery so the CLI doesn't appear frozen
- **`collect` scan speed** — directory size calculation is no longer run eagerly during skill discovery; deferred to the Web UI handler where it is actually needed

### Fixed

- **`universal` target path** — corrected global path from `~/.config/agents/skills` to `~/.agents/skills` (the shared agent directory used by multiple AI CLIs)
- **`init` auto-includes `universal`** — `init` and `init --discover` now automatically include the `universal` target whenever any AI CLI is detected; labeled as "shared agent directory" so users understand what it is
- **`universal` coexistence docs** — added FAQ section explaining how skillshare and `npx skills` coexist on the same `~/.agents/skills` path, including sync mode differences and name collision caveats
- **`--force` hint accuracy** — the force hint now uses the actual repo URL (not per-skill subpath) and includes `--into` when applicable
- **`update` root-level skills** — root-level skill repos (SKILL.md at repo root) no longer appear as stale/deleted during batch update; fixed `Subdir` normalization mismatch between metadata (`""`) and discovery (`"."`)
- **`pull` project mode leak** — `pull` now forces `--global` for the post-pull sync, preventing unintended project-mode auto-detection when run inside a project directory
- **`list` TUI action safety** — `audit`, `update`, and `uninstall` actions in the skill list TUI now show a confirmation overlay before executing; actions pass explicit `--global`/`--project` mode flags to prevent mode mismatch

### Improvements

- **`update` batch summary** — batch update summary now uses the same single-line stats format as `sync` with color-coded counts
- **Command output spacing** — commands now consistently print a trailing blank line after output for better terminal readability

## [0.16.3] - 2026-02-27

### Improvements

- **`diff` output redesign** — actions are now labeled by what they do (`add`, `remove`, `update`, `restore`) with a grouped summary showing counts per action; overall summary line at the end
- **Install progress output** — config and search installs now show tree-style steps with a summary line (installed/skipped/failed counts + elapsed time) and real-time git clone progress
- **Web UI log stats bar** — Log page now shows a stats bar with success rate and per-command breakdown
- **Hub batch install progress** — multi-skill installs from `search --hub` now show real-time git clone progress (`cloning 45%`, `resolving 67%`) instead of a static "installing..." label; only the active install is shown to keep the display compact
- **Hub risk badge colors** — risk labels in hub search results are now color-coded by severity (green for clean, yellow for low, red for critical) in both the list and detail panel
- **Hub batch failure output** — failure details are classified by type (security / ambiguous / not found) with distinct icons; long audit findings and ambiguous path lists are truncated to 3 lines with a "(+N more)" summary

### Performance

- **Batch install reconcile** — config reconciliation now runs once after all installs complete instead of after each skill, eliminating O(n²) directory walks that caused batch installs of large collections to appear stuck
- **Repo-grouped cloning** — skills from the same git repo are now cloned once and installed from the shared clone, reducing network requests for multi-skill repos

### Fixed

- **Race condition in `sync`** — targets sharing the same filesystem path no longer produce duplicate or missing symlinks
- **Race condition in `sync` group key** — canonicalized group key prevents non-deterministic sync results
- **Web UI stats on "All" tab** — dashboard now computes stats from both ops and audit logs, not just ops
- **Web UI last operation timestamp** — timestamps are compared as dates instead of strings, fixing incorrect "most recent" ordering
- **`log --stats --cmd audit`** — now correctly reads from `audit.log` instead of `operations.log`
- **`log max_entries: 0`** — setting max_entries to 0 now correctly means unlimited instead of deleting all entries
- **Oplog data loss** — rewriteEntries now checks for write errors before truncating the original file
- **TUI content clipping** — detail panels in `list` and `log` TUIs now hard-wrap content and account for padding, preventing text from being clipped at panel edges
- **TUI footer spacing** — list and log TUI footers have proper breathing room between action hints
- **Copy mode symlink handling** — `sync` in copy mode now dereferences directory symlinks instead of copying broken link files; prevents missing content in targets like Windsurf that use file copying
- **`uninstall --all` stale summary** — spinner and confirm prompt now show correct noun type after skipping dirty tracked repos; added skip count message ("1 tracked repo skipped, 2 remaining"); fixed unnatural pluralization ("2 group(s)" → "2 groups")
- **Empty `list` / `log` TUI** — `list` and `log` no longer open a blank interactive screen when there are no skills or log entries; they print a plain-text hint instead
- **`install` quiet mode** — tracked config dry-run messages are now suppressed in quiet mode

### New Targets

- **Verdent** — added [Verdent](https://www.verdent.ai/) AI coding agent (`verdent`)

## [0.16.2] - 2026-02-26

### New Features

- **`diff` command** — new command to preview what `sync` would change without modifying anything; parallel target scanning, grouped output for targets with identical diffs, and an overall progress bar:
  ```bash
  skillshare diff              # all targets
  skillshare diff claude       # single target
  skillshare diff -p           # project mode
  ```
- **Interactive TUI for `audit`** — `skillshare audit` launches a bubbletea TUI with severity-colored results, fuzzy filter, and detail panel; progress bar during scanning; confirmation prompt for large scans (1,000+ skills) (`skillshare audit --no-tui` for plain text)
- **Tree sidebar in `list` TUI** — detail panel now shows the skill's directory tree (up to 3 levels) with glamour-rendered markdown preview; SKILL.md pinned at top for quick reading
- **Log TUI: delete entries** — press `space` to select entries, `d` to delete with confirmation; supports multi-select (`a` to select all)
- **Log `--stats` flag** — aggregated summary with per-command breakdown, success rate, and partial/blocked status tracking:
  ```bash
  skillshare log --stats
  ```
- **Azure DevOps URL support** — install from Azure DevOps repos using `ado:` shorthand, full HTTPS (`dev.azure.com`), legacy HTTPS (`visualstudio.com`), or SSH v3 (`ssh.dev.azure.com`) URLs:
  ```bash
  skillshare install ado:myorg/myproject/myrepo
  skillshare install https://dev.azure.com/org/proj/_git/repo
  skillshare install git@ssh.dev.azure.com:v3/org/proj/repo
  ```
- **`AZURE_DEVOPS_TOKEN` env var** — automatic HTTPS token injection for Azure DevOps private repos, same pattern as `GITHUB_TOKEN` / `GITLAB_TOKEN` / `BITBUCKET_TOKEN`:
  ```bash
  export AZURE_DEVOPS_TOKEN=your_pat
  skillshare install https://dev.azure.com/org/proj/_git/repo --track
  ```
- **`update --prune`** — remove stale skills whose upstream source no longer exists (`skillshare update --prune`)
- **Stale detection in `check`** — `skillshare check` now reports skills deleted upstream as "stale (deleted upstream)" instead of silently skipping them
- **Windows ARM64 cross-compile** — `make build-windows` / `mise run build:windows` produces Windows ARM64 binaries

### Performance (large skill collections)

- **Parallel target sync** — both global and project-mode `sync` now run target syncs concurrently (up to 8 workers) with a live per-target progress display
- **mtime fast-path for copy mode** — repeat syncs skip SHA-256 checksums when source directory mtime is unchanged, making no-op syncs near-instant
- **Cached skill discovery** — skills are discovered once and shared across all parallel target workers instead of rediscovering per target

### Improvements

- **Batch progress for hub installs** — multi-skill installs from `search` now show per-skill status (queued/installing/done/error) with a live progress display
- **Log retention** — operation log auto-trims old entries with configurable limits and hysteresis to avoid frequent rewrites
- **Partial completion tracking** — `sync`, `install`, `update`, and `uninstall` now log `"partial"` status when some targets succeed and others fail, instead of a blanket `"error"`
- **Unified TUI color palette** — all bubbletea TUIs share a consistent color palette via shared `tc` struct

### Website

- **Documentation restructure** — website now follows Diátaxis IA (getting-started / how-to / learn / understand / reference / troubleshooting)
- **Blog launch** — 5 launch posts covering tutorials, recipes, and migration guides

### Fixed

- **`upgrade` spinner nesting** — brew output and GitHub release download steps now render cleanly inside tree spinners instead of breaking the layout

## [0.16.1] - 2026-02-25

### Improvements

- **Async TUI loading for `list`** — skill list now loads inside the TUI with a spinner instead of blocking before rendering; metadata reads use a parallel worker pool (64 workers) for faster startup
- **Unified filter bar across all TUIs** — `list`, `log`, and `search` now share the same filter UX: press `/` to enter filter mode, `Esc` to clear, `Enter` to lock; search TUI suppresses action keys while typing to avoid accidental checkbox toggles
- **Colorized audit output** — severity counts (CRITICAL/HIGH/MEDIUM/LOW/INFO), risk labels, and finding details are now color-coded by severity level
- **Improved install output** — single-skill and tracked-repo installs show inline tree steps (description, license, location) instead of a separate SkillBox; description truncation increased to 100 characters with visible ellipsis (`…`)
- **Parallel uninstall discovery** — `uninstall --all` uses parallel git dirty checks (8 workers) for faster execution

### Fixed

- **Frozen terminal during `check` and `update`** — header and spinners now appear immediately before filesystem scans, so users see feedback instead of a blank screen
- **Spinner flicker during `install` clone** — eliminated visual glitch when transitioning between clone and post-clone phases
- **Large operation log files crash `log` TUI** — JSONL parser now uses streaming `json.Decoder` instead of reading entire lines into memory, handling arbitrarily large log entries

## [0.16.0] - 2026-02-25

### Performance

- **Per-skill tree hash comparison for `check`** — `skillshare check` now uses blobless git fetches (~150-200 KB) and compares per-skill directory tree hashes instead of whole-commit hashes; detects updates to individual skills within monorepos without downloading full history ([#46](https://github.com/runkids/skillshare/issues/46))
- **Parallel checking with bounded concurrency** — `check` and `check --all` run up to 8 concurrent workers; deduplicates `ls-remote` calls for repos hosting multiple skills; progress bar now shows skill count instead of URL count ([#46](https://github.com/runkids/skillshare/issues/46))
- **Sparse checkout for subdir installs** — `install owner/repo/subdir` uses `git sparse-checkout` (git 2.25+) to clone only the needed subdirectory with `--filter=blob:none`; falls back to full clone on older git versions (fixes [#46](https://github.com/runkids/skillshare/issues/46))
- **Batch update progress** — `update --all` now shows a progress bar with the current skill name during batch operations

### New Features

- **Interactive TUI for `list`** — `skillshare list` launches a bubbletea TUI with fuzzy search, filter, sort, and a detail panel showing description, license, and metadata; inline actions: audit, update, and uninstall directly from the list (`skillshare list --no-tui` for plain text)
- **Interactive TUI for `log`** — `skillshare log` launches a bubbletea TUI with fuzzy filter and detail panel for browsing operation history (`skillshare log --no-tui` for plain text)
- **Interactive TUI for `search`** — `skillshare search` results now use a bubbletea multi-select checkbox interface instead of survey prompts
- **Interactive TUI for `init`** — target selection in `skillshare init` now uses a bubbletea checklist with descriptions instead of survey multi-select
- **Skill registry separation** — installed skill metadata moved from `config.yaml` to `registry.yaml`; `config.yaml` remains focused on user settings (targets, audit thresholds, custom targets); silent auto-migration on first v0.16.0 run — no user action required
- **Project-mode skills for this repo** — `.skillshare/skills/` ships 5 built-in project skills for contributors: `cli-e2e-test`, `codebase-audit`, `implement-feature`, `update-docs`, `changelog`; install with `skillshare sync -p` in the repo
- **Restore validation preview** — Web UI restore modal now shows a pre-restore validation with conflict warnings, backup size, and symlink detection before committing (`POST /api/restore/validate`)
- **Expanded detail panel in `list` TUI** — detail view now includes word-wrapped description and license field

### Changed

- **CLI visual language overhaul** — all single-item operations (install, update, check) now use a consistent hierarchical layout with structured labels (`Source:`, `Items:`, `Skill:`) and adaptive spinners; audit findings section only appears when findings exist
- **`check` single-skill output** — single skill/repo checks now use the same hierarchical tree layout as `update` with spinner and step results instead of a progress bar
- **`check` summarizes clean results** — up-to-date and local-only skills are now shown as summary counts (e.g., "3 up to date, 2 local") instead of listing each one individually
- **Symlink compat hint moved to `doctor`** — per-target mode hints removed from `sync` output; `doctor` now shows a universal symlink compatibility notice when relevant targets are configured
- **Web UI migrated to TanStack Query** — all API calls use `@tanstack/react-query` with automatic caching, deduplication, and background refetching; Skills page uses virtual scrolling for large collections
- **Deprecated `openclaude` target removed** — replaced by `openclaw`; existing configs using `openclaude` should update to `openclaw`

### Fixed

- **Infinite loop in directory picker for large repos** — bubbletea directory picker now handles repos with many subdirectories without hanging
- **Leading slash in subdir path breaks tree hash lookup** — `check` now normalizes `//skills/foo` to `skills/foo` for consistent path matching
- **`update --all` in project mode skipped nested skills** — recursive skill discovery now enabled for project-mode `update --all`
- **Batch update path duplication** — `update --all` now uses caller-provided destination paths to prevent doubled path segments
- **`file://` URL subdir extraction** — `install file:///path/to/repo//subdir` now correctly extracts subdirectories via the `//` separator
- **Git clone progress missing in batch update** — progress output now wired through to batch update operations
- **Backup restore with symlinks** — `ValidateRestore` now uses `os.Lstat` to correctly detect symlink targets instead of following them

## [0.15.5] - 2026-02-23

### Added
- **`init --mode` flag** — `skillshare init --mode copy` (or `-m copy`) sets the default sync mode for all targets at init time; in interactive mode (TTY), a prompt offers merge / copy / symlink selection; `init --discover --mode copy` applies the mode only to newly added targets, leaving existing targets unchanged (closes [#42](https://github.com/runkids/skillshare/issues/42))
- **Per-target sync mode hint** — after `sync` and `doctor`, a contextual hint suggests `copy` mode for targets known to have symlink compatibility issues (Cursor, Antigravity, Copilot, OpenCode); suppressed when only symlink-compatible targets are configured
- **`uninstall --all`** — remove all skills from source in one command; requires confirmation unless `--force` is set; works in both global and project mode

### Changed
- **Improved CLI output** — compact grouped audit findings (`× N` dedup), structured section labels, lighter update headers

### Fixed
- **Orphan real directories not pruned after uninstall** — `sync` in merge mode now writes `.skillshare-manifest.json` to track managed skills; after `uninstall`, orphan directories (non-symlinks) that appear in the manifest are safely removed instead of kept with "unknown directory" warnings; user-created directories not in the manifest are still preserved (fixes [#45](https://github.com/runkids/skillshare/issues/45))
- **Exclude filter not removing managed real directories** — changing `exclude` patterns now correctly prunes previously-managed real directories (not just symlinks) from targets; manifest entries are cleaned up to prevent stale ownership
- **MultiSelect filter text cleared after selection** — filter text is now preserved after selecting an item in interactive prompts (e.g., `install` skill picker)

## [0.15.4] - 2026-02-23

### Added
- **Post-update security audit gate** — `skillshare update` now runs a security audit after pulling tracked repositories; findings at or above the active threshold trigger rollback/block; interactive mode prompts for confirmation, non-interactive mode (CI) fails closed; use `--skip-audit` to bypass
- **Post-install audit gate for `--track`** — `skillshare install --track` and tracked repo updates now run the same threshold-based security gate; fresh installs are removed on block, updates are rolled back via `git reset`; use `--skip-audit` to bypass
- **Threshold override flags on `update`** — `skillshare update` now supports `--audit-threshold`, `--threshold`, `-T` (including shorthand aliases like `-T h`) for per-command blocking policy
- **`--diff` flag for `update`** — `skillshare update team-skills --diff` shows a file-level change summary after update; for tracked repos, includes line counts via `git diff`; for regular skills, uses file hash comparison to show added/modified/deleted files
- **Content hash pinning** — `install` and `update` now record SHA-256 hashes of all skill files in `.skillshare-meta.json`; subsequent `audit` runs detect tampering (`content-tampered`), missing files (`content-missing`), and unexpected files (`content-unexpected`)
- **`source-repository-link` audit rule** (HIGH) — detects markdown links labeled "source repo" or "source repository" pointing to external URLs, which may be used for supply-chain redirect attacks
- **Structural markdown link parsing for audit** — audit rules now use a full markdown parser instead of regex, correctly handling inline links with titles, reference-style links, autolinks, and HTML anchors while skipping code fences, inline code spans, and image links; reduces false positives in `external-link` and `source-repository-link` rules (extends link-audit foundation from [#39](https://github.com/runkids/skillshare/pull/39))
- **Severity-based risk floor** — audit risk label is now the higher of the score-based label and a floor derived from the most severe finding (e.g., a single HIGH finding always gets at least a `high` risk label)
- **Severity-based color ramp** — audit output now uses consistent color coding: CRITICAL → red, HIGH → orange, MEDIUM → yellow, LOW/INFO → gray; applies to batch summary, severity counts, and single-skill risk labels
- **Audit risk score in `update` output** — CLI and Web UI now display the risk label and score (e.g., "Security: LOW (12/100)") after updating regular skills; Web UI toast notifications include the same information for all update types

### Fixed
- **Uninstall group directory config cleanup** — uninstalling a group directory (e.g., `frontend/`) now properly removes member skill entries (e.g., `frontend/react`, `frontend/vue`) from `config.yaml` via prefix matching
- **Batch `update --all` error propagation** — repos blocked by the security audit gate now count as "Blocked" in the batch summary and cause non-zero exit code
- **`--skip-audit` passthrough** — the flag is now consistently honored for both tracked repos and regular skills during `update` and `install`
- **Server rollback error reporting** — Web UI update endpoint now implements post-pull threshold gate with automatic rollback on findings at/above threshold
- **Audit rollback error accuracy** — rollback failures now report whether the reset succeeded ("rolled back") or failed ("malicious content may remain") instead of silently ignoring errors
- **Audit error propagation** — file hash computation now propagates walk/hash errors instead of silently skipping, ensuring complete integrity baselines

## [0.15.3] - 2026-02-22

### Added
- **Multi-name and `--group` for `audit`** — `skillshare audit a b c` scans multiple skills at once; `--group`/`-G` flag scans all skills in a group directory (repeatable); names and groups can be mixed freely (e.g. `skillshare audit my-skill -G frontend`)
- **`external-link` audit rule** (closes #38) — new `external-link-0` rule (LOW severity) detects external URLs in markdown links (`[text](https://...)`) that may indicate prompt injection vectors or unnecessary token consumption; localhost and loopback links are excluded; completes #38 together with dangling-link detection from v0.15.1 (supersedes #39)
- **Auth tokens for hub search** — `search --hub` now automatically uses `GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`, or `SKILLSHARE_GIT_TOKEN` when fetching private hub indexes; no extra configuration needed

### Changed
- **`pull` merges by default** — when both local and remote have skills on first pull, `pull` now attempts a git merge instead of failing; if the merge has conflicts, it stops with guidance; `--force` still replaces local with remote
- **Parallel audit scanning** — `skillshare audit` (all-skills scan) now runs up to 8 concurrent workers for faster results in both CLI and Web UI

### Fixed
- **`audit` resolves nested skill names** — `skillshare audit nested__skill` now correctly finds skills by flat name or basename with short-name fallback
- **CodeX SKILL.md description over 1024 chars** (fixes #40) — built-in skill description trimmed to stay within CodeX's 1024-character limit

## [0.15.2] - 2026-02-22

### Added
- **`--audit` flag for `hub index`** — `skillshare hub index --audit` enriches the index with per-skill risk scores (0–100) and risk labels so teammates can assess skill safety before installing; `search` displays risk badges in hub results; schema stays v1 with optional fields (omitted when `--audit` is not used)

### Changed
- **`hub index --audit` parallel scanning** — audit scans now run concurrently (up to 8 workers) for faster index generation on large skill collections

### Fixed
- **`init --remote` timing** — initial commit is now deferred to after skill installation, preventing "Local changes detected" errors on first `pull`; re-running `init --remote` on existing config handles edge cases with proper timeout and error recovery
- **Auth error messages for `push`/`pull`** — authentication failures now show actionable hints (SSH URL, token env vars, credential helper) instead of misleading "pull first" advice; includes platform-specific syntax (PowerShell on Windows, `export` on Unix) and links to docs with required token scopes per platform (GitLab, Bitbucket)
- **Git output parsing on non-English systems** — `push`, `pull`, and `init` now set `LC_ALL=C` to force English git output, preventing locale-dependent string matching failures (e.g. "nothing to commit" not detected on Chinese/Japanese systems)
- **Skill version double prefix** — versions like `v0.15.0` in SKILL.md frontmatter no longer display as `vv0.15.0`

## [0.15.1] - 2026-02-21

### Added
- **Dangling link detection in audit** — `skillshare audit` now checks `.md` files for broken local relative links (missing files or directories); produces `LOW` severity findings with pattern `dangling-link`; disable via `audit-rules.yaml` with `- id: dangling-link` / `enabled: false`

### Fixed
- **`push`/`pull` first-sync and remote flow** — overhauled `init --remote`, `push`, and `pull` to handle edge cases: re-running `init --remote` on an existing config, pushing/pulling when remote has no commits yet, and conflicting remote URLs
- **Partial project init recovery** — if `.skillshare/` exists but `config.yaml` is missing, commands now repair config instead of failing

## [0.15.0] - 2026-02-21

### Added
- **Copy sync mode** — `skillshare target <name> --mode copy` syncs skills as real files instead of symlinks, for AI CLIs that can't follow symlinks (e.g. Cursor, Copilot CLI); uses SHA256 checksums for incremental updates; `sync --force` re-copies all; existing targets can switch between merge/copy/symlink at any time (#31, #2)
- **Private repo support via HTTPS tokens** — `install` and `update` now auto-detect `GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`, or `SKILLSHARE_GIT_TOKEN` for HTTPS clone/pull; no manual git config needed; tokens are never written to disk
- **Better auth error messages** — auth failures now tell you whether the issue is "no token found" (with setup suggestions) or "token rejected" (check permissions/expiry); token values are redacted in output

### Fixed
- **`diff` now detects content changes in copy mode** — previously only checked symlink presence; now compares file checksums
- **`doctor` no longer flags copy-managed skills as duplicates**
- **`target remove` in project mode cleans up copy manifest**
- **Copy mode no longer fails on stray files** in target directories or missing target paths
- **`update` and `check` now honor HTTPS token auth** — private repo pull/remote checks now auto-detect `GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`, and `SKILLSHARE_GIT_TOKEN` (same as install)
- **Devcontainer project mode no longer pollutes workspace root** — `ss` keeps caller working directory and redirects `-p` from `/workspace` to demo project
- **Project mode auto-repairs partial initialization** — if `.skillshare/` exists but `config.yaml` is missing, commands repair config instead of failing with "project already initialized"

### Changed
- **`agents` target renamed to `universal`** — existing configs using `agents` continue to work (backward-compatible alias); Kimi and Replit paths updated to match upstream docs
- **`GITHUB_TOKEN` now used for HTTPS clone** — previously only used for GitHub API (search, upgrade); now also used when cloning private repos over HTTPS

## [0.14.2] - 2026-02-20

### Added
- **Multi-name and `--group` for `update`** — `skillshare update a b c` updates multiple skills at once; `--group`/`-G` flag expands a group directory to all updatable skills within it (repeatable); positional names that match a group directory are auto-detected and expanded; names and groups can be mixed freely
- **Multi-name and `--group` for `check`** — `skillshare check a b c` checks only specified skills; `--group`/`-G` flag works identically to `update`; no args = check all (existing behavior preserved); filtered mode includes a loading spinner for network operations
- **Security guide** — new `docs/guides/security.md` covering audit rules, `.skillignore`, and safe install practices; cross-referenced from audit command docs and best practices guide

### Changed
- **Docs diagrams migrated to Mermaid SVG** — replaced ASCII box-drawing diagrams across 10+ command docs with Mermaid `handDrawn` look for better rendering and maintainability
- **Hub docs repositioned** — hub documentation reframed as organization-first with private source examples
- **Docker/devcontainer unified** — consolidated version definitions, init scripts, and added `sandbox-logs` target; devcontainer now includes Node.js 24, auto-start dev servers, and a `dev-servers` manager script

## [0.14.1] - 2026-02-19

### Added
- **Config YAML Schema** — JSON Schema files for both global `config.yaml` and project `.skillshare/config.yaml`; enables IDE autocompletion, validation, and hover documentation via YAML Language Server; `Save()` automatically prepends `# yaml-language-server: $schema=...` directive; new configs from `skillshare init` include the directive out of the box; existing configs get it on next save (any mutating command)

## [0.14.0] - 2026-02-18

### Added
- **Global skill manifest** — `config.yaml` now supports a `skills:` section in global mode (previously project-only); `skillshare install` (no args) installs all listed skills; auto-reconcile keeps the manifest in sync after install/uninstall
- **`.skillignore` file** — repo-level file to hide skills from discovery during install; supports exact match and trailing wildcard patterns; group matching via path-based comparison (e.g. `feature-radar` excludes all skills under that directory)
- **`--exclude` flag for install** — skip specific skills during multi-skill install; filters before the interactive prompt so excluded skills never appear
- **License display in install** — shows SKILL.md `license` frontmatter in selection prompts and single-skill confirmation screen
- **Multi-skill and group uninstall** — `skillshare uninstall` accepts multiple skill names and a repeatable `--group`/`-G` flag for batch removal; groups use prefix matching; problematic skills are skipped with warnings; group directories auto-detected with sub-skill listing in confirmation prompt
- **`group` field in skill manifest** — explicit `group` field separates placement from identity (previously encoded as `name: frontend/pdf`); automatic migration of legacy slash-in-name entries; both global and project reconcilers updated
- **6 new audit security rules** — detection for `eval`/`exec`/`Function` dynamic code, Python shell execution, `process.env` leaking, prompt injection in HTML comments, hex/unicode escape obfuscation; each rule includes false-positive guards
- **Firebender target** — coding agent for JetBrains IDEs; paths: `~/.firebender/skills` (global), `.firebender/skills` (project); target count now 49+
- **Declarative manifest docs** — new concept page and URL formats reference page

### Fixed
- **Agent target paths synced with upstream** — antigravity: `global_skills` → `skills`; augment: `rules` → `skills`; goose project: `.agents/skills` → `.goose/skills`
- **Docusaurus relative doc links** — added `.md` extension to prevent 404s when navigating via navbar

### Changed
- **Website docs restructured** — scenario-driven "What do you want to do?" navigation on all 9 section index pages; standardized "When to Use" and "See Also" sections across all 24 command docs; role-based paths in intro; "What Just Happened?" explainer in getting-started
- **Install integration tests split by concern** — tests reorganized into `install_basic`, `install_discovery`, `install_filtering`, `install_selection`, and `install_helpers` for maintainability

## [0.13.0] - 2026-02-16

### Added
- **Skill-level `targets` field** — SKILL.md frontmatter now accepts a `targets` list to restrict which targets a skill syncs to; `check` validates unknown target names
- **Target filter CLI** — `target <name> --add-include/--add-exclude/--remove-include/--remove-exclude` for inline filter editing; Web UI inline filter editor on Targets page
- **XDG Base Directory support** — respect `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`; backups/trash stored in data dir, logs in state dir; automatic migration from legacy layout on first run
- **Windows legacy path migration** — existing Windows installs at `~\.config\skillshare\` are auto-migrated to `%AppData%\skillshare\` with config source path rewrite
- **Fuzzy subdirectory resolution** — `install owner/repo/skill-name` now fuzzy-matches nested skill directories by basename when exact path doesn't exist, with ambiguity error for multiple matches
- **`list` grouped display** — skills are grouped by directory with tree-style formatting; `--verbose`/`-v` flag for detailed output
- **Runtime UI download** — `skillshare ui` downloads frontend assets from GitHub Releases on first launch and caches at `~/.cache/skillshare/ui/<version>/`; `--clear-cache` to reset; `upgrade` pre-downloads UI assets

### Changed
- **Unified project target names** — project targets now use the same short names as global (e.g. `claude` instead of `claude-code`); old names preserved as aliases for backward compatibility
- **Binary no longer embeds UI** — removed `go:embed` and build tags; UI served exclusively from disk cache, reducing binary size
- **Docker images simplified** — production and CI Dockerfiles no longer include Node build stages

### Fixed
- **Windows `DataDir()`/`StateDir()` paths** — now correctly fall back to `%AppData%` instead of Unix-style `~/.local/` paths
- **Migration result reporting** — structured `MigrationResult` with status tracking; migration outcomes printed at startup
- **Orphan external symlinks after data migration** — `sync` now auto-removes broken external symlinks (e.g. leftover from XDG/Windows path migration); `--force` removes all external symlinks; path comparison uses case-insensitive matching on Windows

### Breaking Changes
- **Windows paths relocated** — config/data moves from `%USERPROFILE%\.config\skillshare\` to `%AppData%\skillshare\` (auto-migrated)
- **XDG data/state split (macOS/Linux)** — backups and trash move from `~/.config/skillshare/` to `~/.local/share/skillshare/`; logs move to `~/.local/state/skillshare/` (auto-migrated)
- **Project target names changed** — `claude-code` → `claude`, `gemini-cli` → `gemini`, etc. (old names still work via aliases)

## [0.12.6] - 2026-02-13

### Added
- **Per-target include/exclude filters (merge mode)** — `include` / `exclude` glob patterns are now supported in both global and project target configs
- **Comprehensive filter test coverage** — added unit + integration tests for include-only, exclude-only, include+exclude precedence, invalid patterns, and prune behavior
- **Project mode support for `doctor`** — `doctor` now supports auto-detect project mode plus explicit `--project` / `--global`

### Changed
- **Filter-aware diagnostics** — `sync`, `diff`, `status`, `doctor`, API drift checks, and Web UI target counts now compute expected skills using include/exclude filters
- **Web UI config freshness** — UI API now auto-reloads config on requests, so browser refresh reflects latest `config.yaml` without restarting `skillshare ui`
- **Documentation expanded** — added practical include/exclude strategy guidance, examples, and project-mode `doctor` usage notes

### Fixed
- **Exclude pruning behavior in merge mode** — when a previously synced source-linked entry becomes excluded, `sync` now unlinks/removes it; existing local non-symlink target folders are preserved
- **Project `doctor` backup/trash reporting** — now uses project-aware semantics (`backups not used in project mode`, trash checked from `.skillshare/trash`)

## [0.12.5] - 2026-02-13

### Fixed
- **`target remove` merge mode symlink cleanup** — CLI now correctly detects and removes all skillshare-managed symlinks using path prefix matching instead of exact name matching; fixes nested/orphaned symlinks being left behind
- **`target remove` in Web UI** — server API now handles merge mode targets (previously only cleaned up symlink mode)

## [0.12.4] - 2026-02-13

### Added
- **Graceful shutdown** — HTTP server handles SIGTERM/SIGINT with 10s drain period, safe for container orchestrators
- **Server timeouts** — ReadHeaderTimeout (5s), ReadTimeout (15s), WriteTimeout (30s), IdleTimeout (60s) prevent slow-client resource exhaustion
- **Enhanced health endpoint** — `/api/health` now returns `version` and `uptime_seconds`
- **Production Docker image** (`docker/production/Dockerfile`) — multi-stage build, `tini` PID 1, non-root user (UID 10001), auto-init entrypoint, healthcheck
- **CI Docker image** (`docker/ci/Dockerfile`) — minimal image for `skillshare audit` in pipelines
- **Docker dev profile** — `make dev-docker-up` runs Go API server in Docker for frontend development without local Go
- **Multi-arch Docker build** — `make docker-build-multiarch` produces linux/amd64 + linux/arm64 images
- **Docker publish workflow** (`.github/workflows/docker-publish.yml`) — auto-builds and pushes production + CI images to GHCR on tag push
- **`make sandbox-status`** — show playground container status

### Changed
- **Compose security hardening** — playground: `read_only`, `cap_drop: ALL`, `tmpfs` with exec; all profiles: `no-new-privileges`, resource limits (2 CPU / 2G)
- **Test scripts DRY** — `test_docker.sh` accepts `--online` flag; `test_docker_online.sh` is now a thin wrapper
- **Compose version check** — `_sandbox_common.sh` verifies Docker Compose v2.20+ with platform-specific install hints
- **`.dockerignore` expanded** — excludes `.github/`, `website/`, editor temp files
- **Git command timeout** — increased from 60s to 180s for constrained Docker/CI networks
- **Online test timeout** — increased from 120s to 300s

### Fixed
- **Sandbox `chmod` failure** — playground volume init now uses `--cap-add ALL` to work with `cap_drop: ALL`
- **Dev profile crash on first run** — auto-runs `skillshare init` before starting UI server
- **Sandbox Dockerfile missing `curl`** — added for playground healthcheck

## [0.12.2] - 2026-02-13

### Fixed
- **Hub search returns all results** — hub/index search no longer capped at 20; `limit=0` means no limit (GitHub search default unchanged)
- **Search filter ghost cards** — replaced IIFE rendering with `useMemo` to fix stale DOM when filtering results

### Added
- **Scroll-to-load in Web UI** — search results render 20 at a time with IntersectionObserver-based incremental loading

## [0.12.1] - 2026-02-13

### Added
- **Hub persistence** — saved hubs stored in `config.yaml` (both global and project), shared between CLI and Web UI
  - `hub add <url>` — save a hub source (`--label` to name it; first add auto-sets as default)
  - `hub list` — list saved hubs (`*` marks default)
  - `hub remove <label>` — remove a saved hub
  - `hub default [label]` — show or set the default hub (`--reset` to clear)
  - All subcommands support `--project` / `--global` mode
- **Hub label resolution in search** — `search --hub <label>` resolves saved hub labels instead of requiring full URLs
  - `search --hub team` looks up the "team" hub from config
  - `search --hub` (bare) uses the config default, falling back to community hub
- **Hub saved API** — REST endpoints for hub CRUD (`GET/PUT/POST/DELETE /api/hub/saved`)
- **Web UI hub persistence** — hub list and default hub now persisted on server instead of browser localStorage
- **Search fuzzy filter** — hub search results filtered by fuzzy match on name + substring match on description and tags
- **Tag badges in search** — `#tag` badges displayed in both CLI interactive selector and Web UI hub search results
- **Web UI tag filter** — inline filter input on hub search cards matching name, description, and tags

### Changed
- `search --hub` (bare flag) now defaults to community skillshare-hub instead of requiring a URL
- Web UI SearchPage migrated from localStorage to server API for hub state

### Fixed
- `audit <path>` no longer fails with "config not found" in CI environments without a skillshare config

## [0.12.0] - 2026-02-13

### Added
- **Hub index generation** — `skillshare hub index` builds a `skillshare-hub.json` from installed skills for private or team catalogs
  - `--full` includes extended metadata (flatName, type, version, repoUrl, installedAt)
  - `--output` / `-o` to customize output path; `--source` / `-s` to override scan directory
  - Supports both global and project mode (`-p` / `-g`)
- **Private index search** — `skillshare search --hub <url>` searches a hub index (local file or HTTP URL) instead of GitHub
  - Browse all entries with no query, or fuzzy-match by name/description/tags/source
  - Interactive install prompt with `source` and optional `skill` field support
- **Hub index schema** — `schemaVersion: 1` with `tags` and `skill` fields for classification and multi-skill repo support
- **Web UI hub search** — search private indexes from the dashboard with a hub URL dropdown
  - Hub manager modal for adding, removing, and selecting saved hub URLs (persisted in localStorage)
- **Web UI hub index API** — `GET /api/hub/index` endpoint for generating indexes from the dashboard
- Hub index guide and command reference in documentation

### Fixed
- `hub index` help text referenced incorrect `--index-url` flag (now `--hub`)
- Frontend `SearchResult` TypeScript interface missing `tags` field

## [0.11.6] - 2026-02-11

### Added
- **Auto-pull on `init --remote`** — when remote has existing skills, init automatically fetches and syncs them; no manual `git clone` or `git pull` needed
- **Auto-commit on `git init`** — `init` creates an initial commit (with `.gitignore`) so `push`/`pull`/`stash` work immediately
- **Git identity fallback** — if `user.name`/`user.email` aren't configured, sets repo-local defaults (`skillshare@local`) with a hint to set your own
- **Git remote error hints** — `push`, `pull`, and `init --remote` now show actionable hints for SSH, URL, and network errors
- **Docker sandbox `--bare` mode** — `make sandbox-bare` starts the playground without auto-init for manual testing
- **Docker sandbox `--volumes` reset** — `make sandbox-reset` removes the playground home volume for a full reset

### Changed
- **`init --remote` auto-detection** — global-only flags (`--remote`, `--source`, etc.) now skip project-mode auto-detection, so `init --remote` works from any directory
- **Target multi-select labels** — shortened to `name (status)` for readability; paths shown during detection phase instead

### Fixed
- `init --remote` on second machine no longer fails with "Local changes detected" or merge conflicts
- `init --remote` produces clean linear git history (no merge commits from unrelated histories)
- Pro tip message only shown when built-in skill is actually installed

## [0.11.5] - 2026-02-11

### Added
- **`--into` flag for install** — organize skills into subdirectories (`skillshare install repo --into frontend` places skills under `skills/frontend/`)
- **Nested skill support in check/update/uninstall** — recursive directory walk detects skills in organizational folders; `update` and `uninstall` resolve short names (e.g., `update vue` finds `frontend/vue/vue-best-practices`)
- **Configurable audit block threshold** — `audit.block_threshold` in config sets which severity blocks install (default `CRITICAL`); `audit --threshold <level>` overrides per-command
- **Audit path scanning** — `skillshare audit <path>` scans arbitrary files or directories, not only installed skills
- **Audit JSON output** — `skillshare audit --json` for machine-readable results with risk scores
- **`--skip-audit` flag for install** — bypass security scanning for a single install command
- **Risk scoring** — weighted risk score and label (clean/low/medium/high/critical) per scanned skill
- **LOW and INFO severity levels** — lighter-weight findings that contribute to risk score without blocking
- **IBM Bob target** — added to supported AI CLIs (global: `~/.bob/skills`, project: `.bob/skills`)
- **JS/TS syntax highlighting in file viewer** — Web UI highlights `.js`, `.ts`, `.jsx`, `.tsx` files with CodeMirror
- **Project init agent grouping** — agents sharing the same project skills path (Amp, Codex, Copilot, Gemini, Goose, etc.) are collapsed into a single selectable group entry

### Changed
- **Goose project path** updated from `.goose/skills` to `.agents/skills` (universal agent directory convention)
- **Audit summary includes all severity levels** — LOW/INFO counts, risk score, and threshold shown in summary box and log entries

### Fixed
- Web UI nested skill update now uses full relative path instead of basename only
- YAML block scalar frontmatter (`>-`, `|`, `|-`) parsed correctly in skill detail view
- CodeMirror used for all non-markdown files in file viewer (previously plain `<pre>`)

## [0.11.4] - 2026-02-11

### Added
- **Customizable audit rules** — `audit-rules.yaml` externalizes security rules for user overrides
  - Three-layer merge: built-in → global (`~/.config/skillshare/audit-rules.yaml`) → project (`.skillshare/audit-rules.yaml`)
  - Add custom rules, override severity, or disable built-in rules per-project
  - `skillshare audit --init-rules` to scaffold a starter rules file
- **Web UI Audit Rules page** — create, edit, toggle, and delete rules from the dashboard
- **Log filtering** — filter operation/audit logs by status, command, or keyword; custom dropdown component
- **Docker playground audit demo** — pre-loaded demo skills and custom rules for hands-on audit exploration

### Changed
- **Built-in skill is now opt-in** — `init` and `upgrade` no longer install the built-in skill by default; use `--skill` to include it
- **HIGH findings reclassified as warnings** — only CRITICAL findings block `install`; HIGH/MEDIUM are shown as warnings
- Integration tests split into offline (`!online`) and online (`online`) build tags for faster local runs

## [0.11.0] - 2026-02-10

### Added
- **Security Audit** — `skillshare audit [name]` scans skills for prompt injection, data exfiltration, credential access, destructive commands, obfuscation, and suspicious URLs
  - CRITICAL findings block `skillshare install` by default; use `--force` to override
  - HIGH/MEDIUM findings shown as warnings with file, line, and snippet detail
  - Per-skill progress display with tree-formatted findings and summary box
  - Project mode support (`skillshare audit -p`)
- **Web UI Audit page** — scan all skills from the dashboard, view findings with severity badges
  - Install flow shows `ConfirmDialog` on CRITICAL block with "Force Install" option
  - Warning dialog displays HIGH/MEDIUM findings after successful install
- **Audit API** — `GET /api/audit` and `GET /api/audit/{name}` endpoints
- **Operation log (persistent audit trail)** — JSONL-based operations/audit logging across CLI + API + Web UI
  - CLI: `skillshare log` (`--audit`, `--tail`, `--clear`, `-p/-g`)
  - API: log list/clear endpoints for operations and audit streams
  - Web UI: Log page with tabs, filters, status/duration formatting, and clear/refresh actions
- **Sync drift detection** — `status` and `doctor` warn when targets have fewer linked skills than source
  - Web UI shows drift badges on Dashboard and Targets pages
- **Trash (soft-delete) workflow** — uninstall now moves skills to trash with 7-day retention
  - New CLI commands: `skillshare trash list`, `skillshare trash restore <name>`, `skillshare trash delete <name>`, `skillshare trash empty`
  - Web UI Trash page for list/restore/delete/empty actions
  - Trash API handlers with global/project mode support
- **Update preview command** — `skillshare check` shows available updates for tracked repos and installed skills without modifying files
- **Search ranking upgrade** — relevance scoring now combines name/description/stars with repo-scoped query support (`owner/repo[/subdir]`)
- **Docs site local search** — Docusaurus local search integrated for command/doc lookup
- **SSH subpath support** — `install git@host:repo.git//subdir` with `//` separator
- **Docs comparison guide** — new declarative vs imperative workflow comparison page

### Changed
- **Install discovery + selection UX**
  - Hidden directory scan now skips only `.git` (supports repos using folders like `.curated/` and `.system/`)
  - `install --skill` falls back to fuzzy matching when exact name lookup fails
  - UI SkillPicker adds filter input and filtered Select All behavior for large result sets
  - Batch install feedback improved: summary toast always shown; blocked-skill retry targets only blocked items
  - CLI mixed-result installs now use warning output and condensed success summaries
- **Search performance + metadata enrichment** — star/description enrichment is parallelized, and description frontmatter is used in scoring
- **Skill template refresh** — `new` command template updated to a WHAT+WHEN trigger format with step-based instructions
- **Search command UX** — running `search` with no keyword now prompts for input instead of auto-browsing
- **Sandbox hardening** — playground shell defaults to home and mounts source read-only to reduce accidental host edits
- **Project mode clarity** — `(project)` labels added across key command outputs; uninstall prompt now explicitly says "from the project?"
- **Project tracked-repo workflow reliability**
  - `ProjectSkill` now supports `tracked: true` for portable project manifests
  - Reconcile logic now detects tracked repos via `.git` + remote origin even when metadata files are absent
  - Tracked repo naming uses `owner-repo` style (for example, `_openai-skills`) to avoid basename collisions
  - Project `list` now uses recursive skill discovery for parity with global mode and Web UI
- **Privacy-first messaging + UI polish** — homepage/README messaging updated, dashboard quick actions aligned, and website hero/logo refreshed with a new hand-drawn style
- `ConfirmDialog` component supports `wide` prop and hidden cancel button
- Sidebar category renamed from "Utilities" to "Security & Utilities"
- README updated with audit section, new screenshots, unified image sizes
- Documentation links and navigation updated across README/website

### Fixed
- Web UI uninstall handlers now use trash move semantics instead of permanent deletion
- Windows self-upgrade now shows a clear locked-binary hint when rename fails (for example, when `skillshare ui` is still running)
- `mise.toml` `ui:build` path handling fixed so `cd ui` does not leak into subsequent build steps
- Sync log details now include target count, fixing blank details in some entries
- Project tracked repos are no longer skipped during reconcile when metadata is missing

## [0.10.0] - 2026-02-08

### Added
- **Web Dashboard** — `skillshare ui` launches a full-featured React SPA embedded in the binary
  - Dashboard overview with skill/target counts, sync mode, and version check
  - Skills browser with search, filter, SKILL.md viewer, and uninstall
  - Targets page with status badges, add/remove targets
  - Sync controls with dry-run/force toggles and diff preview
  - Collect page to scan and pick skills from targets back to source
  - GitHub skill search with one-click install and batch install
  - Config editor with YAML validation
  - Backup/restore management with cleanup
  - Git sync page with push/pull, dirty-file detection, and force-pull
  - Install page supporting path, git URL, and GitHub shorthand inputs
  - Update tracked repos from the UI with commit/diff details
- **REST API** at `/api/*` — Go `net/http` backend (30+ endpoints) powering the dashboard
- **Single-binary distribution** — React frontend embedded via `go:embed`, no Node.js required at runtime
- **Dev mode** — `go build -tags dev` serves placeholder SPA; use Vite on `:5173` with `/api` proxy for hot reload
- **`internal/git/info.go`** — git operations library (pull with change info, force-pull, dirty detection, stage/commit/push)
- **`internal/version/skill.go`** — local and remote skill version checking
- **Bitbucket/GitLab URL support** — `install` now strips branch prefixes from Bitbucket (`src/{branch}/`) and GitLab (`-/tree/{branch}/`) web URLs
- **`internal/utils/frontmatter.go`** — `ParseFrontmatterField()` utility for reading SKILL.md metadata
- Integration tests for `skillshare ui` server startup
- Docker sandbox support for web UI (`--host 0.0.0.0`, port 19420 mapping)
- CI: frontend build step in release and test workflows
- Website documentation for `ui` command

### Changed
- Makefile updated with `ui-build`, `build-ui`, `ui-dev` targets
- `.goreleaser.yaml` updated to include frontend build in release pipeline
- Docker sandbox Dockerfile uses multi-stage build with Node.js for frontend assets

## [0.9.0] - 2026-02-05

### Added
- **Project-level skills** — scope skills to a single repository, shared via git
  - `skillshare init -p` to initialize project mode
  - `.skillshare/` directory with `config.yaml`, `skills/`, and `.gitignore`
  - All core commands support `-p` flag: `sync`, `install`, `uninstall`, `update`, `list`, `status`, `target`, `collect`
- **Auto-detection** — commands automatically switch to project mode when `.skillshare/config.yaml` exists
- **Per-target sync mode for project mode** — each target can use `merge` or `symlink` independently
- **`--discover` flag** — detect and add new AI CLI targets to existing project config
- **Tracked repos in project mode** — `install --track -p` clones repos into `.skillshare/skills/`
- Integration tests for all project mode commands

### Changed
- Terminology: "Team Sharing" → "Organization-Wide Skills", "Team Edition" → "Organization Skills"
- Documentation restructured with dual-level architecture (Organization + Project)
- Unified project sync output format with global sync

## [0.8.0] - 2026-01-31

### Breaking Changes

**Command Rename: `pull <target>` → `collect <target>`**

For clearer command symmetry, `pull` is now exclusively for git operations:

| Before | After | Description |
|--------|-------|-------------|
| `pull claude` | `collect claude` | Collect skills from target to source |
| `pull --all` | `collect --all` | Collect from all targets |
| `pull --remote` | `pull` | Pull from git remote |

### New Command Symmetry

| Operation | Commands | Direction |
|-----------|----------|-----------|
| Local sync | `sync` / `collect` | Source ↔ Targets |
| Remote sync | `push` / `pull` | Source ↔ Git Remote |

```
Remote (git)
   ↑ push    ↓ pull
Source
   ↓ sync    ↑ collect
Targets
```

### Migration

```bash
# Before
skillshare pull claude
skillshare pull --remote

# After
skillshare collect claude
skillshare pull
```

## [0.7.0] - 2026-01-31

### Added
- Full Windows support (NTFS junctions, zip downloads, self-upgrade)
- `search` command to discover skills from GitHub
- Interactive skill selector for search results

### Changed
- Windows uses NTFS junctions instead of symlinks (no admin required)

## [0.6.0] - 2026-01-20

### Added
- Team Edition with tracked repositories
- `--track` flag for `install` command
- `update` command for tracked repos
- Nested skill support with `__` separator

## [0.5.0] - 2026-01-16

### Added
- `new` command to create skills with template
- `doctor` command for diagnostics
- `upgrade` command for self-upgrade

### Changed
- Improved sync output with detailed statistics

## [0.4.0] - 2026-01-16

### Added
- `diff` command to show differences
- `backup` and `restore` commands
- Automatic backup before sync

### Changed
- Default sync mode changed to `merge`

## [0.3.0] - 2026-01-15

### Added
- `push` and `pull --remote` for cross-machine sync
- Git integration in `init` command

## [0.2.0] - 2026-01-14

### Added
- `install` and `uninstall` commands
- Support for git repo installation
- `target add` and `target remove` commands

## [0.1.0] - 2026-01-14

### Added
- Initial release
- `init`, `sync`, `status`, `list` commands
- Symlink and merge sync modes
- Multi-target support
