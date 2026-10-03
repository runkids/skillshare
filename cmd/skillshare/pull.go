package main

import (
	"errors"
	"fmt"
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

	remoteEmpty, err := integrateRemote(source, force, spinner)
	if err != nil {
		return err
	}
	if remoteEmpty {
		spinner.Warn("Remote has no branches yet")
		ui.Info("  Push your skills first: skillshare push")
	}

	spinner.Stop()
	ui.SuccessMsg("Pull complete (%.1fs)", time.Since(pullStart).Seconds())

	fmt.Println()
	return syncPulledScope(cfg)
}

// integrateRemote brings the remote's history into source. First pull (no
// upstream): fetch, then merge or reset onto the remote default branch and
// set upstream (see gitops.FirstPull). Subsequent pulls: normal git pull,
// which merges. remoteEmpty reports a remote with no branches yet.
func integrateRemote(source string, force bool, spinner *ui.Spinner) (remoteEmpty bool, err error) {
	if !gitops.HasUpstream(source) {
		spinner.Update("Fetching from remote...")
		if _, err := gitops.FirstPull(source, force); errors.Is(err, gitops.ErrNoRemoteBranches) {
			return true, nil
		} else if err != nil {
			spinner.Fail("Pull failed")
			if errors.Is(err, gitops.ErrMergeFailed) {
				ui.Info("  Resolve manually: cd %s && git merge --allow-unrelated-histories <remote branch>", source)
				ui.Info("  Or force-pull: skillshare pull --force  (replaces local with remote)")
			} else if !isAuthError(err.Error()) {
				hintGitRemoteError(err.Error()) // auth guidance is already part of err
			}
			return false, err
		}
		return false, nil
	}

	spinner.Update("Running git pull...")
	if _, err := gitops.PullWithEnv(source, gitops.AuthEnvForRepo(source)); err != nil {
		spinner.Fail("git pull failed")
		fmt.Println(err.Error())
		hintGitRemoteError(err.Error())
		return false, fmt.Errorf("git pull failed: %w", err)
	}
	return false, nil
}

// syncPulledScope syncs what the git root scope holds (always global — pull
// operates on the global source).
func syncPulledScope(cfg *config.Config) error {
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
