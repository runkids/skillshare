package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"skillshare/internal/config"
	gitops "skillshare/internal/git"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdPull(args []string) error {
	if wantsHelp(args) {
		printPullHelp()
		return nil
	}

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

	source, err := resolveGitRoot(cfg, spinner)
	if err != nil {
		return err
	}

	// Check if source is a git repo with a configured remote
	if err := checkGitRepo(source, spinner); err != nil {
		return err
	}

	// Check for uncommitted changes
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = source
	output, err := cmd.Output()
	if err != nil {
		spinner.Fail("Failed to check git status")
		return fmt.Errorf("failed to check git status: %w", err)
	}

	if len(strings.TrimSpace(string(output))) > 0 {
		spinner.Fail("Local changes detected")
		ui.Info("  Run: skillshare push")
		ui.Info("  Or:  cd %s && git stash -u", source)
		return fmt.Errorf("local changes must be pushed or stashed before pulling")
	}

	if dryRun {
		spinner.Stop()
		ui.Warning("[dry-run] No changes will be made")
		fmt.Println()
		ui.Info("Would run: git pull")
		ui.Info("Would run: skillshare sync")
		return nil
	}

	// First pull (no upstream): fetch, then merge or reset onto the remote
	// default branch and set upstream (see gitops.FirstPull). Subsequent pulls:
	// normal git pull.
	authEnv := gitops.AuthEnvForRepo(source)
	if !gitops.HasUpstream(source) {
		spinner.Update("Fetching from remote...")
		if _, err := gitops.FirstPull(source, force); errors.Is(err, gitops.ErrNoRemoteBranches) {
			spinner.Warn("Remote has no branches yet")
			ui.Info("  Push your skills first: skillshare push")
		} else if err != nil {
			spinner.Fail("Pull failed")
			if errors.Is(err, gitops.ErrMergeFailed) {
				ui.Info("  Resolve manually: cd %s && git merge --allow-unrelated-histories <remote branch>", source)
				ui.Info("  Or force-pull: skillshare pull --force  (replaces local with remote)")
			} else if !isAuthError(err.Error()) {
				hintGitRemoteError(err.Error()) // auth guidance is already part of err
			}
			return err
		}
	} else {
		spinner.Update("Running git pull...")
		cmd = exec.Command("git", "pull")
		cmd.Dir = source
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

	// Sync what the pulled scope holds (always global — pull operates on the
	// global source).
	fmt.Println()
	switch cfg.GitRoot {
	case "agents":
		return cmdSync([]string{"agents", "--global"})
	case "extras":
		return cmdSync([]string{"extras", "--global"})
	case "root":
		if err := cmdSync([]string{"--global"}); err != nil {
			return err
		}
		fmt.Println()
		return cmdSync([]string{"agents", "--global"})
	}
	return cmdSync([]string{"--global"})
}

func printPullHelp() {
	fmt.Println(`Usage: skillshare pull [options]

Pull from git remote and sync to all targets.

Options:
  --dry-run, -n     Preview changes without applying
  --force, -f       Force-pull (reset local to remote on first pull)
  --help, -h        Show this help

Examples:
  skillshare pull                Pull and sync
  skillshare pull --dry-run      Preview what would happen
  skillshare pull --force        Discard local, use remote`)
}
