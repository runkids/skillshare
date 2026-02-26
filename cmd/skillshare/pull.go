package main

import (
	"errors"
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

type firstPullOutcome int

const (
	firstPullNoop firstPullOutcome = iota
	firstPullApplied
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
		oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
	}

	return err
}

// pullFromRemote pulls from git remote and syncs to all targets
func pullFromRemote(cfg *config.Config, dryRun, force bool) error {
	ui.Header("Pulling from remote")

	pullStart := time.Now()
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
		if _, err := firstPull(cfg.Source, authEnv, force, spinner); err != nil {
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

	spinner.Stop()
	ui.SuccessMsg("Pull complete (%.1fs)", time.Since(pullStart).Seconds())

	// Sync to all targets
	fmt.Println()
	return cmdSync([]string{})
}

// firstPull handles the initial pull when no upstream tracking exists.
// Fetches remote, then decides based on local/remote content:
//   - Remote has branches but no skills + local has skills:
//     merge unrelated histories (preserve local skills, import remote files)
//   - Both have skills + no --force: attempt merge (fail on conflict)
//   - Both have skills + --force: reset to remote (discard local)
//   - Local has no skills, or --force: reset to remote
func firstPull(sourcePath string, authEnv []string, force bool, spinner *ui.Spinner) (firstPullOutcome, error) {
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
		return firstPullNoop, fmt.Errorf("fetch failed: %w", err)
	}

	remoteBranch, err := gitops.GetRemoteDefaultBranch(sourcePath)
	if err != nil {
		if errors.Is(err, gitops.ErrNoRemoteBranches) {
			spinner.Warn("Remote has no branches yet")
			ui.Info("  Push your skills first: skillshare push")
			return firstPullNoop, nil
		}
		spinner.Fail("Failed to detect remote default branch")
		return firstPullNoop, fmt.Errorf("failed to detect remote default branch: %w", err)
	}

	// Check if remote actually has skill directories
	hasRemoteSkills, err := gitops.HasRemoteSkillDirs(sourcePath, remoteBranch)
	if err != nil {
		spinner.Fail("Failed to inspect remote skills")
		return firstPullNoop, fmt.Errorf("failed to inspect remote skills: %w", err)
	}

	// Check if local has skill directories
	hasLocalSkills, err := gitops.HasLocalSkillDirs(sourcePath)
	if err != nil {
		spinner.Fail("Failed to inspect local skills")
		return firstPullNoop, fmt.Errorf("failed to inspect local skills: %w", err)
	}

	if !hasRemoteSkills {
		// Remote has history/files but no skills.
		// If local has skills, merge histories so later push/pull won't hit
		// unrelated-history errors.
		if hasLocalSkills && !force {
			if err := mergeRemoteHistory(sourcePath, remoteBranch, spinner); err != nil {
				return firstPullNoop, err
			}
			setUpstream(sourcePath, remoteBranch)
			return firstPullApplied, nil
		}

		// Local has no skills (or --force): align to remote history directly.
		if err := resetToRemote(sourcePath, remoteBranch, spinner); err != nil {
			return firstPullNoop, err
		}
		setUpstream(sourcePath, remoteBranch)
		return firstPullApplied, nil
	}

	if hasLocalSkills && !force {
		// Both have skills — try merge; on conflict, guide the user.
		if err := mergeRemoteHistory(sourcePath, remoteBranch, spinner); err != nil {
			ui.Info("  Or force-pull: skillshare pull --force  (replaces local with remote)")
			return firstPullNoop, err
		}
		setUpstream(sourcePath, remoteBranch)
		return firstPullApplied, nil
	}

	// Safe to reset: either local has no skills, or --force was used
	if err := resetToRemote(sourcePath, remoteBranch, spinner); err != nil {
		return firstPullNoop, err
	}
	setUpstream(sourcePath, remoteBranch)
	return firstPullApplied, nil
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

func mergeRemoteHistory(sourcePath, remoteBranch string, spinner *ui.Spinner) error {
	spinner.Update("Merging remote history...")
	mergeCmd := exec.Command("git", "-c", "merge.ff=false", "merge", "--allow-unrelated-histories", "--no-edit", "origin/"+remoteBranch)
	mergeCmd.Dir = sourcePath
	if output, err := mergeCmd.CombinedOutput(); err != nil {
		spinner.Fail("Failed to merge remote history")
		fmt.Print(string(output))
		abortCmd := exec.Command("git", "merge", "--abort")
		abortCmd.Dir = sourcePath
		abortCmd.Run() // best-effort cleanup
		ui.Info("  Resolve manually: cd %s && git merge --allow-unrelated-histories origin/%s", sourcePath, remoteBranch)
		return fmt.Errorf("merge failed: %w", err)
	}
	return nil
}

func resetToRemote(sourcePath, remoteBranch string, spinner *ui.Spinner) error {
	spinner.Update("Pulling skills from remote...")
	resetCmd := exec.Command("git", "reset", "--hard", "origin/"+remoteBranch)
	resetCmd.Dir = sourcePath
	if output, err := resetCmd.CombinedOutput(); err != nil {
		spinner.Fail("Failed to pull from remote")
		fmt.Print(string(output))
		return fmt.Errorf("reset failed: %w", err)
	}
	return nil
}
