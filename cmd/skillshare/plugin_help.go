package main

import "fmt"

func printPluginHelp() {
	fmt.Println(`Usage: skillshare plugin [command] [options]

Commands:
  list                       Browse managed plugins (default; TUI in a terminal)
  add [source]               Choose a plugin from a directory or HTTPS Git repository
  discover <source>          Inspect source candidates without installing
  import [native-id]        Adopt an existing native install with --from
  inspect <name>             Show bindings and native installation state
  sync [name]                Install missing bindings or retry pending operations
  check [name]               Check source content for changes (read-only)
  update [name]              Review and update supported targets
  enable / disable [name]    Select / deselect managed targets for the next sync
  remove [name]              Uninstall managed bindings; retain shared marketplaces

Options:
  --target <target>         Receiving client (repeat to select several)
  --plugin <name>            Select one plugin from a marketplace
  --name <name>              Logical Skillshare package name
  --from <target>           Import from this native client in the selected scope
  --dry-run, -n              Preview without writing config or Agent state
  --source-ref <ref>        Git branch, tag or commit (discover/add/update)
  --entry <path>            Explicit OpenCode JS/TS entry (discover/add)
  --revision <id>            Require the matching preview before applying
  --json                    Machine-readable output, no interactive prompts
  --no-tui                  Plain output, no interactive prompts
  --global, -g              Global configuration (native user scope)
  --project, -p             Project configuration (supported targets only)

Targets: claude, codex, cursor, antigravity (desktop; alias: agy),
         antigravity-cli, copilot, grok, pi, opencode
Discovery only: kimi, hermes, devin (native automation not yet verified)
Grok requires native trust: install there first, then import.
Project targets: claude, antigravity, pi, opencode
Cursor/Antigravity sync whole local folders; OpenCode syncs registrations.
Pi project operations require native project trust.

Sync alias: skillshare sync plugins [name] [--dry-run] [--json]
Plugins are not included in sync --all. Complete plugin trees stay together.
Codex project installation and native updates are not supported here.
Enable/disable saves sync selection only; run sync to install/remove the target.
Authentication, hook trust and command-source approval remain in the native client.
Use check before update. Import does not reinstall or change enabled state.`)
}
