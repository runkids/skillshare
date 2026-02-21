package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	gitops "skillshare/internal/git"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdPull(args []string) error {
	start := time.Now()
	dryRun := false
	force := false

	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	err = pullFromRemote(cfg, dryRun, force)

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
func pullFromRemote(cfg *config.Config, dryRun, force bool) error {
	ui.Header("Pulling from remote")

	spinner := ui.StartSpinner("Checking repository...")

	// Check if source is a git repo
	gitDir := filepath.Join(cfg.Source, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		spinner.Fail("Source is not a git repository")
		ui.Info("  Run: skillshare init --remote <url>")
		return nil
	}

	// Check if remote exists
	cmd := exec.Command("git", "remote")
	cmd.Dir = cfg.Source
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		spinner.Fail("No git remote configured")
		ui.Info("  Run: skillshare init --remote <url>")
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

	// First pull (no upstream): fetch + reset to remote branch, then set
	// upstream. This mirrors tryPullAfterRemoteSetup() in init.go and avoids
	// merge conflicts between the local init commit and remote history.
	// Subsequent pulls: normal git pull.
	authEnv := gitops.AuthEnvForRepo(cfg.Source)
	if !gitops.HasUpstream(cfg.Source) {
		if err := firstPull(cfg.Source, authEnv, force, spinner); err != nil {
			return err
		}
	} else {
		spinner.Update("Running git pull...")
		cmd = exec.Command("git", "pull")
		cmd.Dir = cfg.Source
		if len(authEnv) > 0 {
			cmd.Env = append(os.Environ(), authEnv...)
		}
		pullOutput, err := cmd.CombinedOutput()
		if err != nil {
			spinner.Fail("git pull failed")
			outStr := string(pullOutput)
			fmt.Print(outStr)
			hintGitRemoteError(outStr)
			return fmt.Errorf("git pull failed: %w", err)
		}
	}

	spinner.Success("Pull complete")

	// Sync to all targets
	fmt.Println()
	return cmdSync([]string{})
}

// firstPull handles the initial pull when no upstream tracking exists.
// Fetches remote, then decides based on local/remote content:
//   - Remote has no skills → just set upstream (nothing useful to pull)
//   - Local has no skills  → reset to remote (safe, nothing to lose)
//   - Both have skills     → refuse and let user choose push or pull --force
func firstPull(sourcePath string, authEnv []string, force bool, spinner *ui.Spinner) error {
	spinner.Update("Fetching from remote...")

	fetchCmd := exec.Command("git", "fetch", "origin")
	fetchCmd.Dir = sourcePath
	if len(authEnv) > 0 {
		fetchCmd.Env = append(os.Environ(), authEnv...)
	}
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		spinner.Fail("Fetch failed")
		outStr := string(output)
		fmt.Print(outStr)
		hintGitRemoteError(outStr)
		return fmt.Errorf("fetch failed: %w", err)
	}

	// Detect remote default branch (main or master)
	remoteBranch := ""
	for _, b := range []string{"main", "master"} {
		checkCmd := exec.Command("git", "rev-parse", "--verify", "origin/"+b)
		checkCmd.Dir = sourcePath
		if err := checkCmd.Run(); err == nil {
			remoteBranch = b
			break
		}
	}
	if remoteBranch == "" {
		spinner.Warn("Remote has no branches yet")
		ui.Info("  Push your skills first: skillshare push")
		return nil
	}

	// Check if remote actually has skill directories
	hasRemoteSkills := false
	lsCmd := exec.Command("git", "ls-tree", "-d", "--name-only", "origin/"+remoteBranch)
	lsCmd.Dir = sourcePath
	if lsOut, err := lsCmd.Output(); err == nil && strings.TrimSpace(string(lsOut)) != "" {
		hasRemoteSkills = true
	}

	// Check if local has skill directories
	hasLocalSkills := false
	entries, _ := os.ReadDir(sourcePath)
	for _, e := range entries {
		if e.IsDir() && e.Name() != ".git" {
			hasLocalSkills = true
			break
		}
	}

	if !hasRemoteSkills {
		// Remote has no skills — just set upstream, user can push later
		setUpstream(sourcePath, remoteBranch)
		return nil
	}

	if hasLocalSkills && !force {
		// Both have skills — refuse to overwrite
		spinner.Fail("Remote has skills, but local skills also exist")
		ui.Info("  Push local:  skillshare push")
		ui.Info("  Pull remote: skillshare pull --force  (replaces local with remote)")
		return nil
	}

	// Safe to reset: either local has no skills, or --force was used
	spinner.Update("Pulling skills from remote...")
	resetCmd := exec.Command("git", "reset", "--hard", "origin/"+remoteBranch)
	resetCmd.Dir = sourcePath
	if output, err := resetCmd.CombinedOutput(); err != nil {
		spinner.Fail("Failed to pull from remote")
		fmt.Print(string(output))
		return fmt.Errorf("reset failed: %w", err)
	}

	setUpstream(sourcePath, remoteBranch)
	return nil
}

func setUpstream(sourcePath, remoteBranch string) {
	localBranch, _ := gitops.GetCurrentBranch(sourcePath)
	if localBranch == "" {
		localBranch = "main"
	}
	trackCmd := exec.Command("git", "branch", "--set-upstream-to=origin/"+remoteBranch, localBranch)
	trackCmd.Dir = sourcePath
	trackCmd.Run() // best-effort
}
