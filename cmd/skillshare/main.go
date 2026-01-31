package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"skillshare/internal/ui"
	versioncheck "skillshare/internal/version"
)

var version = "dev"

func main() {
	// Clean up any leftover .old files from Windows self-upgrade
	cleanupOldBinary()

	// Set version for other packages to use
	versioncheck.Version = version

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "install":
		err = cmdInstall(args)
	case "uninstall":
		err = cmdUninstall(args)
	case "list":
		err = cmdList(args)
	case "sync":
		err = cmdSync(args)
	case "status":
		err = cmdStatus(args)
	case "diff":
		err = cmdDiff(args)
	case "backup":
		err = cmdBackup(args)
	case "restore":
		err = cmdRestore(args)
	case "pull":
		err = cmdPull(args)
	case "push":
		err = cmdPush(args)
	case "doctor":
		err = cmdDoctor(args)
	case "target":
		err = cmdTarget(args)
	case "upgrade":
		err = cmdUpgrade(args)
	case "update":
		err = cmdUpdate(args)
	case "new":
		err = cmdNew(args)
	case "search":
		err = cmdSearch(args)
	case "version", "-v", "--version":
		ui.Logo(version)
	case "help", "-h", "--help":
		printUsage()
	default:
		ui.Error("Unknown command: %s", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	// Check for updates (non-blocking, silent on errors)
	// Skip for upgrade command since we just upgraded (current process still has old version)
	if cmd != "upgrade" {
		if result := versioncheck.Check(version); result != nil && result.UpdateAvailable {
			ui.UpdateNotification(result.CurrentVersion, result.LatestVersion)
		}
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
	cmd("uninstall", "<name>", "Remove a skill from source directory")
	cmd("list", "", "List all installed skills")
	cmd("search", "<query>", "Search GitHub for skills")
	cmd("sync", "", "Sync skills to all targets")
	cmd("status", "", "Show status of all targets")
	fmt.Println()

	// Skill Management
	fmt.Println("SKILL MANAGEMENT")
	cmd("new", "<name>", "Create a new skill with SKILL.md template")
	cmd("update", "<name>", "Update a skill or tracked repository")
	cmd("update", "--all", "Update all tracked repositories")
	cmd("upgrade", "", "Upgrade CLI and/or skillshare skill")
	fmt.Println()

	// Target Management
	fmt.Println("TARGET MANAGEMENT")
	cmd("target add", "<name> <path>", "Add a target")
	cmd("target remove", "<name>", "Unlink target and restore skills")
	cmd("target list", "", "List all targets")
	cmd("diff", "", "Show differences between source and targets")
	fmt.Println()

	// Sync & Backup
	fmt.Println("SYNC & BACKUP")
	cmd("pull", "[target]", "Pull local skills from target(s) to source")
	cmd("push", "", "Commit and push skills to git remote")
	cmd("backup", "", "Create backup of target(s)")
	cmd("restore", "<target>", "Restore target from latest backup")
	fmt.Println()

	// Utilities
	fmt.Println("UTILITIES")
	cmd("doctor", "", "Check environment and diagnose issues")
	cmd("version", "", "Show version")
	cmd("help", "", "Show this help")
	fmt.Println()

	// Examples
	fmt.Println("EXAMPLES")
	fmt.Println(g + "  skillshare init --source ~/.skills")
	fmt.Println("  skillshare new my-skill")
	fmt.Println("  skillshare install github.com/user/skill-repo")
	fmt.Println("  skillshare target add claude ~/.claude/skills")
	fmt.Println("  skillshare sync" + r)
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
