package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"skillshare/internal/cache"
	"skillshare/internal/config"
	"skillshare/internal/ui"
	versioncheck "skillshare/internal/version"
)

var version = "dev"

// discoveryCache is the shared discovery cache for all commands.
var discoveryCache *cache.DiscoveryCache

// commands maps command names to their handler functions
var commands = map[string]func([]string) error{
	"init":      cmdInit,
	"install":   cmdInstall,
	"uninstall": cmdUninstall,
	"list":      cmdList,
	"sync":      cmdSync,
	"status":    cmdStatus,
	"diff":      cmdDiff,
	"backup":    cmdBackup,
	"restore":   cmdRestore,
	"collect":   cmdCollect,
	"pull":      cmdPull,
	"push":      cmdPush,
	"doctor":    cmdDoctor,
	"target":    cmdTarget,
	"upgrade":   cmdUpgrade,
	"update":    cmdUpdate,
	"check":     cmdCheck,
	"new":       cmdNew,
	"search":    cmdSearch,
	"trash":     cmdTrash,
	"audit":     cmdAudit,
	"hub":       cmdHub,
	"log":       cmdLog,
	"ui":        cmdUI,
}

func main() {
	// Clean up any leftover .old files from Windows self-upgrade
	cleanupOldBinary()

	// Migrate Windows legacy ~/.config/skillshare → %AppData%\skillshare
	results := config.MigrateWindowsLegacyDir()

	// Migrate legacy dirs (backups/trash/logs) to XDG data/state dirs
	results = append(results, config.MigrateXDGDirs()...)
	reportMigrationResults(results)

	// Set version for other packages to use
	versioncheck.Version = version

	// Initialize shared discovery cache
	discoveryCache = cache.New(config.CacheDir())

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
		fmt.Println()
		ui.Error("%v", err)
		os.Exit(1)
	}

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
	g := "\033[90m" // gray
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
	cmd("install", "<source>", "Install a skill from local path or git repo")
	cmd("uninstall", "<name>...", "Remove skills from source directory")
	cmd("list", "", "List all installed skills")
	cmd("search", "[query]", "Search or browse GitHub for skills")
	cmd("sync", "", "Sync skills to all targets")
	cmd("status", "", "Show status of all targets")
	fmt.Println()

	// Skill Management
	fmt.Println("SKILL MANAGEMENT")
	cmd("new", "<name>", "Create a new skill with SKILL.md template")
	cmd("check", "", "Check for available updates")
	cmd("update", "<name>", "Update a skill or tracked repository")
	cmd("update", "--all", "Update all tracked repositories")
	cmd("upgrade", "", "Upgrade CLI and/or skillshare skill")
	fmt.Println()

	// Target Management
	fmt.Println("TARGET MANAGEMENT")
	cmd("target add", "<name> [path]", "Add a target (path optional in project mode)")
	cmd("target remove", "<name>", "Unlink target and restore skills")
	cmd("target list", "", "List all targets")
	cmd("diff", "", "Show differences between source and targets")
	fmt.Println()

	// Sync & Backup
	fmt.Println("SYNC & BACKUP")
	cmd("collect", "[target]", "Collect local skills from target(s) to source")
	cmd("backup", "", "Create backup of target(s)")
	cmd("restore", "<target>", "Restore target from latest backup")
	cmd("trash", "list", "List trashed skills")
	cmd("trash", "restore <name>", "Restore a skill from trash")
	fmt.Println()

	// Git Remote
	fmt.Println("GIT REMOTE")
	cmd("push", "", "Commit and push source to git remote")
	cmd("pull", "", "Pull from git remote and sync to targets")
	fmt.Println()

	// Utilities
	fmt.Println("UTILITIES")
	cmd("audit", "[name]", "Scan skills for security threats")
	cmd("hub", "<subcommand>", "Manage hubs (add, list, remove, default, index)")
	cmd("log", "", "View operation log")
	cmd("ui", "", "Launch web dashboard")
	cmd("doctor", "", "Check environment and diagnose issues")
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
	fmt.Println("  skillshare collect claude                           # Import local skills")
	fmt.Println("  skillshare install anthropics/skills/pdf -p         # Project install")
	fmt.Println("  skillshare target add cursor -p                     # Project target")
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
