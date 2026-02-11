package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdPull(args []string) error {
	start := time.Now()
	dryRun := false

	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	err = pullFromRemote(cfg, dryRun)

	if !dryRun {
		e := oplog.NewEntry("pull", statusFromErr(err), time.Since(start))
		if err != nil {
			e.Message = err.Error()
		}
		oplog.Write(config.ConfigPath(), oplog.OpsFile, e) //nolint:errcheck
	}

	return err
}

// pullFromRemote pulls from git remote and syncs to all targets
func pullFromRemote(cfg *config.Config, dryRun bool) error {
	ui.Header("Pulling from remote")

	spinner := ui.StartSpinner("Checking repository...")

	// Check if source is a git repo
	gitDir := filepath.Join(cfg.Source, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		spinner.Fail("Source is not a git repository")
		ui.Info("  Run: cd %s && git init", cfg.Source)
		return nil
	}

	// Check if remote exists
	cmd := exec.Command("git", "remote")
	cmd.Dir = cfg.Source
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		spinner.Fail("No git remote configured")
		ui.Info("  Run: cd %s && git remote add origin <url>", cfg.Source)
		return nil
	}

	// Check for uncommitted changes
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = cfg.Source
	output, err = cmd.Output()
	if err != nil {
		spinner.Fail("Failed to check git status")
		return fmt.Errorf("failed to check git status: %w", err)
	}

	if len(strings.TrimSpace(string(output))) > 0 {
		spinner.Fail("Local changes detected")
		ui.Info("  Run: skillshare push")
		ui.Info("  Or:  cd %s && git stash", cfg.Source)
		return nil
	}

	if dryRun {
		spinner.Stop()
		ui.Warning("[dry-run] No changes will be made")
		fmt.Println()
		ui.Info("Would run: git pull")
		ui.Info("Would run: skillshare sync")
		return nil
	}

	// Git pull
	spinner.Update("Running git pull...")
	cmd = exec.Command("git", "pull")
	cmd.Dir = cfg.Source
	pullOutput, err := cmd.CombinedOutput()
	if err != nil {
		spinner.Fail("git pull failed")
		outStr := string(pullOutput)
		fmt.Print(outStr)
		hintGitRemoteError(outStr)
		return fmt.Errorf("git pull failed: %w", err)
	}

	spinner.Success("Pull complete")

	// Sync to all targets
	fmt.Println()
	return cmdSync([]string{})
}
