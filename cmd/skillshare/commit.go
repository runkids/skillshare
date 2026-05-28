package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

// checkGitWorktree verifies source is a git worktree without requiring a remote.
func checkGitWorktree(sourcePath string, spinner *ui.Spinner) error {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = sourcePath
	if err := cmd.Run(); err != nil {
		spinner.Fail("Source is not a git repository")
		ui.Info("  Run: skillshare init")
		return fmt.Errorf("not a git repository")
	}
	return nil
}

func cmdCommit(args []string) error {
	if wantsHelp(args) {
		printCommitHelp()
		return nil
	}

	start := time.Now()
	opts := parsePushArgs(args)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config not found: run 'skillshare init' first")
	}

	ui.Header("Committing local changes")

	spinner := ui.StartSpinner("Checking repository...")
	source, ok := resolveGitRoot(cfg, spinner)
	if !ok {
		return nil // Mismatch guidance already displayed
	}

	if err := checkGitWorktree(source, spinner); err != nil {
		return nil // Error already displayed
	}

	changes, err := getGitChanges(source)
	if err != nil {
		spinner.Fail("Failed to check git status")
		return err
	}
	if changes == "" {
		spinner.Stop()
		ui.Info("No changes to commit")
		return nil
	}

	if opts.dryRun {
		spinner.Stop()
		ui.Warning("[dry-run] No changes will be made")
		fmt.Println()
		lines := strings.Split(changes, "\n")
		ui.Info("Would stage %d file(s):", len(lines))
		for _, line := range lines {
			ui.Info("  %s", line)
		}
		ui.Info("Would commit with message: %s", opts.message)
		return nil
	}

	if err := stageAndCommit(source, opts.message, spinner); err != nil {
		return err
	}

	spinner.Stop()
	ui.SuccessMsg("Commit complete (%.1fs)", time.Since(start).Seconds())

	e := oplog.NewEntry("commit", "ok", time.Since(start))
	e.Args = map[string]any{"message": opts.message}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func printCommitHelp() {
	fmt.Println(`Usage: skillshare commit [options]

Create a local git commit for source skills without pushing.

Options:
  -m, --message <msg>   Commit message (default: "Update skills")
  --dry-run, -n         Preview changes without applying
  --help, -h            Show this help

Examples:
  skillshare commit                         Commit with default message
  skillshare commit -m "Update skill"       Commit with custom message
  skillshare commit --dry-run               Preview what would happen`)
}
