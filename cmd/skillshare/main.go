package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	versioncheck "skillshare/internal/version"
)

var version = "dev"

// commands maps command names to their handler functions
var commands = map[string]func([]string) error{
	"init":         cmdInit,
	"install":      cmdInstall,
	"uninstall":    cmdUninstall,
	"list":         cmdList,
	"sync":         cmdSync,
	"status":       cmdStatus,
	"diff":         cmdDiff,
	"backup":       cmdBackup,
	"restore":      cmdRestore,
	"collect":      cmdCollect,
	"pull":         cmdPull,
	"push":         cmdPush,
	"commit":       cmdCommit,
	"doctor":       cmdDoctor,
	"target":       cmdTarget,
	"upgrade":      cmdUpgrade,
	"update":       cmdUpdate,
	"check":        cmdCheck,
	"new":          cmdNew,
	"search":       cmdSearch,
	"trash":        cmdTrash,
	"analyze":      cmdAnalyze,
	"audit":        cmdAudit,
	"hub":          cmdHub,
	"log":          cmdLog,
	"ui":           cmdUI,
	"__ui-restart": cmdUIRestart,
	"tui":          cmdTUIToggle,
	"extras":       cmdExtras,
	"mcp":          cmdMCP,
	"plugin":       cmdPlugin,
	"enable":       cmdEnable,
	"disable":      cmdDisable,
	"completion":   cmdCompletion,
}

func main() {
	// Clean up any leftover .old files from Windows self-upgrade
	cleanupOldBinary()

	// Resolve theme early so OSC 11 probe overhead happens at startup,
	// not inside a TUI render loop.
	_ = theme.Get()

	// Migrate Windows legacy ~/.config/skillshare → %AppData%\skillshare
	results := config.MigrateWindowsLegacyDir()

	// Migrate legacy dirs (backups/trash/logs) to XDG data/state dirs
	results = append(results, config.MigrateXDGDirs()...)
	reportMigrationResults(results)

	// Set version for other packages to use
	versioncheck.Version = version

	// Inject target dotdirs for skill discovery (avoids circular import)
	install.TargetDotDirs = config.ProjectTargetDotDirs()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Handle special commands (no error return)
	switch cmd {
	case "version":
		ui.Logo(version)
		return
	case "-v", "--version":
		fmt.Printf("skillshare v%s\n", version)
		return
	case "help", "-h", "--help":
		printUsage()
		return
	}

	// Look up and execute command
	handler, ok := commands[cmd]
	if !ok {
		ui.Error("Unknown command: %s", cmd)
		printUsage()
		os.Exit(1)
	}

	if err := handler(args); err != nil {
		// jsonSilentError means JSON output was already written to stdout;
		// exit non-zero without adding plain-text noise.
		var silent *jsonSilentError
		if errors.As(err, &silent) {
			os.Exit(1)
		}
		fmt.Println()
		ui.Error("%v", err)
		os.Exit(1)
	}

	// Structured-output contract: machine-readable modes (--json, -j,
	// --format json/sarif/markdown) must produce only structured data on
	// stdout and nothing on stderr.  Skip the trailing newline and update
	// check entirely so machine consumers get a clean payload.
	if !isStructuredOutput(args) {
		fmt.Println()

		// Check for updates (non-blocking, silent on errors)
		// Skip for upgrade (just upgraded, old version in process) and doctor (has its own check)
		if cmd != "upgrade" && cmd != "doctor" {
			method := detectInstallMethod()
			if result := versioncheck.Check(version, method); result != nil && result.UpdateAvailable {
				ui.UpdateNotification(result.CurrentVersion, result.LatestVersion, result.InstallMethod.UpgradeCommand())
			}
		}
	}
}

// detectInstallMethod resolves the current executable path and determines
// how skillshare was installed (Homebrew vs direct download).
func detectInstallMethod() versioncheck.InstallMethod {
	execPath, err := os.Executable()
	if err != nil {
		return versioncheck.InstallDirect
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return versioncheck.InstallDirect
	}
	return versioncheck.DetectInstallMethod(execPath)
}

func reportMigrationResults(results []config.MigrationResult) {
	for _, r := range results {
		switch r.Status {
		case config.MigrationMoved:
			ui.Info("Migrated legacy data: %s -> %s", r.From, r.To)
		case config.MigrationFailed:
			if r.From != "" && r.To != "" {
				ui.Warning("Legacy migration failed: %s -> %s (%v)", r.From, r.To, r.Err)
				continue
			}
			ui.Warning("Legacy migration failed: %v", r.Err)
		}
		// MigrationSkippedDestinationExists is silent — it means the new
		// location already has data (normal after first migration).
	}
}

func printUsage() {
	// Colors
	y := "\033[33m" // yellow - commands
	c := "\033[36m" // cyan - arguments
	g := ui.Dim     // dim
	r := "\033[0m"  // reset

	// ASCII art logo
	ui.Logo("")

	// Command width for alignment
	const w = 35

	// Helper: pad command to fixed width
	pad := func(s string, width int) string {
		// Count visible characters (exclude ANSI codes)
		visible := 0
		inEscape := false
		for _, ch := range s {
			if ch == '\033' {
				inEscape = true
			} else if inEscape && ch == 'm' {
				inEscape = false
			} else if !inEscape {
				visible++
			}
		}
		if visible < width {
			return s + fmt.Sprintf("%*s", width-visible, "")
		}
		return s
	}

	// Helper: format command line
	cmd := func(name, args, desc string) {
		var cmdPart string
		if args != "" {
			cmdPart = y + name + r + " " + c + args + r
		} else {
			cmdPart = y + name + r
		}
		fmt.Printf("  %s %s\n", pad(cmdPart, w), desc)
	}

	// Core Commands
	fmt.Println("CORE COMMANDS")
	cmd("init", "", "Initialize skillshare")
	cmd("install", "<source>", "Install skills/agents from local path or git repo")
	cmd("uninstall", "<name>...", "Remove skills/agents from source directory")
	cmd("list", "[agents] [pattern] [--all]", "List installed skills (or agents)")
	cmd("search", "[query]", "Search or browse GitHub for skills")
	cmd("sync", "[scope] [--all]", "Sync skills/agents/extras/MCP to targets")
	cmd("status", "", "Show status of all targets")
	fmt.Println()

	// Skill & Agent Management
	fmt.Println("SKILL & AGENT MANAGEMENT")
	cmd("new", "<name>", "Create a new skill with SKILL.md template")
	cmd("enable", "<name|pattern>", "Enable a disabled skill/agent")
	cmd("disable", "<name|pattern>", "Disable a skill/agent")
	cmd("check", "[agents] [--all]", "Check for available updates")
	cmd("update", "[agents] <name>", "Update a skill/agent or tracked repository")
	cmd("update", "--all", "Update all tracked repositories")
	cmd("upgrade", "", "Upgrade CLI and/or skillshare skill")
	fmt.Println()

	// Target Management
	fmt.Println("TARGET MANAGEMENT")
	cmd("target add", "<name> [path]", "Add a target (path optional in project mode)")
	cmd("target remove", "<name>", "Unlink target and restore skills")
	cmd("target list", "", "List all targets")
	cmd("diff", "[target]", "Show differences between source and targets")
	fmt.Println()

	// Sync & Backup
	fmt.Println("SYNC & BACKUP")
	cmd("collect", "[agents] [target]", "Collect local skills/agents from target(s)")
	cmd("backup", "", "Create backup of target(s)")
	cmd("restore", "<target>", "Restore target from latest backup")
	cmd("trash", "[agents] list", "List trashed skills/agents")
	cmd("trash", "[agents] restore <name>", "Restore a skill/agent from trash")
	fmt.Println()

	// Extras
	fmt.Println("EXTRAS")
	cmd("extras", "init <name>", "Create a new extra resource type")
	cmd("extras", "list", "List all configured extras and sync status")
	cmd("extras", "remove <name>", "Remove an extra resource type")
	cmd("extras", "collect <name>", "Collect local files into extras source")
	fmt.Println()

	// Plugins
	fmt.Println("PLUGIN MANAGEMENT")
	cmd("plugin list", "", "Browse plugins and native installation state (TUI)")
	cmd("plugin discover", "<source>", "Inspect plugins in a directory or Git repository")
	cmd("plugin add", "[source]", "Install a complete plugin (guided in a terminal)")
	cmd("plugin import", "[native-id]", "Adopt a native install with --from <target>")
	cmd("plugin inspect", "<name>", "Show a managed plugin and its target bindings")
	cmd("plugin sync", "[name]", "Apply sync selection using native clients")
	cmd("plugin check", "[name]", "Check source changes without applying")
	cmd("plugin update", "[name]", "Update supported targets from reviewed sources")
	cmd("plugin enable", "[name]", "Select targets for the next plugin sync")
	cmd("plugin disable", "[name]", "Deselect targets; next sync removes their installs")
	cmd("plugin remove", "[name]", "Uninstall managed plugins and remove definitions")
	cmd("sync plugins", "[name]", "Alias for plugin sync; excluded from sync --all")
	fmt.Println()

	// MCP
	fmt.Println("MCP")
	cmd("mcp add", "[name]", "Add a connection (guided, --url or -- command)")
	cmd("mcp edit", "[name]", "Change an endpoint, command or its clients")
	cmd("mcp import", "[name]", "Import from a client (--from) or file (--file)")
	cmd("mcp list", "", "Browse connections and per-client sync status")
	cmd("mcp remove", "[name]", "Remove a connection from the source")
	cmd("mcp restore", "[id]", "Restore client entries from a backup")
	fmt.Println()

	// Git Remote
	fmt.Println("GIT REMOTE")
	cmd("commit", "", "Create local git commit without pushing")
	cmd("push", "", "Commit and push source to git remote")
	cmd("pull", "", "Pull from git remote and sync to targets")
	fmt.Println()

	// Utilities
	fmt.Println("UTILITIES")
	cmd("audit", "[agents] [name]", "Scan skills/agents for security threats")
	cmd("hub", "<subcommand>", "Manage hubs (add, list, remove, default, index)")
	cmd("log", "", "View operation log")
	cmd("tui", "[on|off]", "Toggle interactive TUI mode")
	cmd("ui", "", "Launch web dashboard")
	cmd("doctor", "", "Check environment and diagnose issues")
	cmd("completion", "<shell>", "Generate shell completion scripts")
	cmd("version", "", "Show version")
	cmd("help", "", "Show this help")
	fmt.Println()

	// Global Options
	fmt.Println("GLOBAL OPTIONS")
	fmt.Printf("  %s%-33s%s %s\n", c, "--project, -p", r, "Use project-level config in current directory")
	fmt.Printf("  %s%-33s%s %s\n", c, "--global, -g", r, "Use global config (~/.config/skillshare)")
	fmt.Println()

	// Examples
	fmt.Println("EXAMPLES")
	fmt.Println(g + "  skillshare status                                   # Check current state")
	fmt.Println("  skillshare sync --dry-run                           # Preview before sync")
	fmt.Println("  skillshare sync agents                              # Sync agents only")
	fmt.Println("  skillshare sync --all                               # Sync skills + agents + extras + MCP")
	fmt.Println("  skillshare plugin                                  # Open the plugin manager")
	fmt.Println("  skillshare sync plugins --dry-run --json            # Preview plugin changes")
	fmt.Println("  skillshare list --all                               # List skills + agents")
	fmt.Println("  skillshare collect claude                           # Import local skills")
	fmt.Println("  skillshare install anthropics/skills/pdf -p         # Project install")
	fmt.Println("  skillshare install repo -a my-agent                 # Install specific agent")
	fmt.Println("  skillshare target add cursor -p                     # Project target")
	fmt.Println("  skillshare commit -m \"Update skill\"                # Commit locally")
	fmt.Println("  skillshare push -m \"Add new skill\"                  # Push to remote")
	fmt.Println("  skillshare pull                                     # Pull from remote")
	fmt.Println("  skillshare install github.com/team/skills --track   # Team repo")
	fmt.Println("  skillshare update --all                             # Update all repos" + r)
}

// cleanupOldBinary removes leftover .old files from Windows self-upgrade.
// On Windows, we rename the running exe to .old before replacing it.
// This cleanup runs on next startup to remove those files.
func cleanupOldBinary() {
	if runtime.GOOS != "windows" {
		return
	}
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return
	}
	oldPath := execPath + ".old"
	// Silently try to remove - may not exist, that's fine
	os.Remove(oldPath)
}
