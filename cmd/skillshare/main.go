package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"skillshare/internal/childproc"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
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
	"hooks":        cmdHooks,
	"plugin":       cmdPlugin,
	"enable":       cmdEnable,
	"disable":      cmdDisable,
	"completion":   cmdCompletion,
}

func main() {
	// Windows: children stop when this process is terminated, and without a
	// console it reruns inside a hidden one so they do not each open a window.
	relaunchInHiddenConsole(childproc.BindToProcess())

	// Clean up any leftover .old files from Windows self-upgrade
	cleanupOldBinary()

	// Resolve theme early so OSC 11 probe overhead happens at startup,
	// not inside a TUI render loop.
	if t := theme.Get(); t.NoColor || t.Plain {
		ui.DisableColors()
	}

	// Migrate Windows legacy ~/.config/skillshare → %AppData%\skillshare
	results := config.MigrateWindowsLegacyDir()

	// Migrate legacy dirs (backups/trash/logs) to XDG data/state dirs
	results = append(results, config.MigrateXDGDirs()...)
	reportMigrationResults(results)

	// Set version for other packages to use
	versioncheck.Version = version

	// Inject target dotdirs for skill discovery (avoids circular import)
	targetDotDirs := config.ProjectTargetDotDirs()
	install.TargetDotDirs = targetDotDirs
	sourcewalk.TargetDotDirs = targetDotDirs

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Handle special commands (no error return)
	switch cmd {
	case "version":
		fmt.Printf("skillshare %s\n", ui.VersionLabel(version))
		return
	case "-v", "--version":
		fmt.Printf("skillshare %s\n", ui.VersionLabel(version))
		return
	case "help", "-h", "--help":
		printUsage()
		return
	}

	// Look up and execute command
	handler, ok := commands[cmd]
	if !ok {
		// A lone option such as --no-tui names no command: show the help.
		if strings.HasPrefix(cmd, "-") {
			printUsage()
		} else {
			printUnknownCommand(cmd)
		}
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
			ui.Success("Moved legacy data %s → %s", shortenPath(r.From), shortenPath(r.To))
		case config.MigrationFailed:
			if r.From != "" && r.To != "" {
				ui.Warning("Moving legacy data %s → %s failed: %v", shortenPath(r.From), shortenPath(r.To), r.Err)
				continue
			}
			ui.Warning("Legacy migration failed: %v", r.Err)
		}
		// MigrationSkippedDestinationExists is silent — it means the new
		// location already has data (normal after first migration).
	}
}

func printUsage() {
	fmt.Println(theme.Primary().Bold(true).Render("skillshare") + " " + ui.DimText(ui.VersionLabel(version)+"  Your AI coding setup, everywhere."))
	fmt.Println()
	printHelp("skillshare <command> [options]", "",
		helpGroup{title: "Start", rows: []helpRow{
			{"init", "Set up the source and pick Targets"},
			{"status", "Show the source and every Target"},
			{"sync", "Link skills, agents, extras and MCP into Targets"},
			{"ui", "Open the web dashboard"},
			{"doctor", "Find and explain setup problems"},
		}},
		helpGroup{title: "Skills and agents", rows: []helpRow{
			{"install", "Install from a repo or path"},
			{"uninstall", "Remove from the source"},
			{"list", "List what is installed"},
			{"search", "Find skills on GitHub or a hub"},
			{"new", "Create a skill from a template"},
			{"enable", "Turn a skill or agent back on"},
			{"disable", "Turn a skill or agent off"},
			{"check", "See which ones have updates"},
			{"update", "Pull updates"},
			{"collect", "Bring local skills from a Target into the source"},
			{"audit", "Scan for security problems"},
			{"analyze", "Show how much context each Target's skills use"},
		}},
		helpGroup{title: "More to sync", rows: []helpRow{
			{"extras", "Rules, commands and other shared files"},
			{"mcp", "MCP servers"},
			{"hooks", "Agent hooks"},
			{"plugin", "Agent plugins"},
		}},
		helpGroup{title: "Targets", rows: []helpRow{
			{"target", "Add, remove and list Targets"},
			{"diff", "Compare the source with a Target"},
		}},
		helpGroup{title: "History", rows: []helpRow{
			{"backup", "Back up Targets"},
			{"restore", "Roll a Target back"},
			{"trash", "Restore something you uninstalled"},
			{"log", "See what ran"},
		}},
		helpGroup{title: "Git", rows: []helpRow{{"commit · push · pull", ""}}},
		helpGroup{title: "CLI", rows: []helpRow{{"upgrade · hub · tui · completion · version", ""}}},
		helpGroup{title: "Common options", rows: []helpRow{
			{"-p, --project", "Use this project's config"},
			{"-g, --global", "Use the global config"},
			{"-n, --dry-run", "Preview, write nothing"},
			{"--json", "Machine-readable output"},
			{"--no-tui", "Plain prompts, no full-screen UI"},
			{"-h, --help", "Help for any command"},
		}},
	)
	fmt.Println()
	fmt.Println(ui.DimText("Run 'skillshare <command> --help' for its subcommands and options."))
	fmt.Println(ui.DimText("Docs  https://skillshare.runkids.cc"))
}

// printUnknownCommand names the mistyped command, the closest real one,
// and where to find the rest.
func printUnknownCommand(cmd string) {
	ui.Error("Unknown command: %s", cmd)
	if s := suggestCommand(cmd, slices.Collect(maps.Keys(commands))); s != "" {
		ui.Note(fmt.Sprintf("Did you mean %q?", s))
	}
	ui.Note("Run 'skillshare help' to see all commands")
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
