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
		ui.Note("Run: skillshare push")
		ui.Note(fmt.Sprintf("Or:  cd %s && git stash -u", source))
		return fmt.Errorf("local changes must be pushed or stashed before pulling")
	}

	width := ui.RowWidth("Pull", "Sync")
	if dryRun {
		spinner.Stop()
		ui.Row(ui.MarkNone, "Pull", "would run git pull", width)
		ui.Row(ui.MarkNone, "Sync", "would run skillshare sync", width)
		fmt.Println()
		ui.DryRun()
		return nil
	}

	// First pull (no upstream): fetch, then merge or reset onto the remote
	// default branch and set upstream (see gitops.FirstPull). Subsequent pulls:
	// normal git pull.
	authEnv := gitops.AuthEnvForRepo(source)
	var info *gitops.UpdateInfo
	if !gitops.HasUpstream(source) {
		spinner.Update("Fetching from remote...")
		info, err = gitops.FirstPull(source, force)
		if errors.Is(err, gitops.ErrNoRemoteBranches) {
			spinner.Stop()
			ui.Row(ui.MarkWarn, "Pull", "remote has no branches yet", width)
			ui.Note("Push your skills first: skillshare push")
		} else if err != nil {
			spinner.Fail("Pull failed")
			if errors.Is(err, gitops.ErrMergeFailed) {
				ui.Note(fmt.Sprintf("Resolve manually: cd %s && git merge --allow-unrelated-histories <remote branch>", source))
				ui.Note("Or force-pull: skillshare pull --force  (replaces local with remote)")
			} else if !isAuthError(err.Error()) {
				hintGitRemoteError(err.Error()) // auth guidance is already part of err
			}
			return err
		}
	} else {
		spinner.Update("Running git pull...")
		if info, err = gitops.PullWithEnv(source, authEnv); err != nil {
			spinner.Fail("git pull failed")
			fmt.Println(err.Error())
			hintGitRemoteError(err.Error())
			return fmt.Errorf("git pull failed: %w", err)
		}
	}

	if info != nil {
		spinner.Stop()
		ui.Row(ui.MarkOK, "Pull", pullSummary(info)+ui.DimText(fmt.Sprintf(" · %.1fs", time.Since(pullStart).Seconds())), width)
		printCommitNotes(info.Commits)
	}

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

// pullSummary says what a pull brought in, like update does for a tracked
// repository.
func pullSummary(info *gitops.UpdateInfo) string {
	switch {
	case info.UpToDate:
		return "already up to date"
	case len(info.Commits) == 0:
		return "checked out the remote branch"
	}
	return fmt.Sprintf("%s, %s changed (+%d −%d)",
		plural(len(info.Commits), "commit"), plural(info.Stats.FilesChanged, "file"),
		info.Stats.Insertions, info.Stats.Deletions)
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
