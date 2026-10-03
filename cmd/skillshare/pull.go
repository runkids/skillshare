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

	info, remoteEmpty, err := integrateRemote(source, force, cfg.GitRoot == "root", spinner)
	if err != nil {
		return err
	}
	spinner.Stop()
	if remoteEmpty {
		ui.Row(ui.MarkWarn, "Pull", "remote has no branches yet", width)
		ui.Note("Push your skills first: skillshare push")
	} else if info != nil {
		ui.Row(ui.MarkOK, "Pull", pullSummary(info)+ui.Took(time.Since(pullStart)), width)
		printCommitNotes(info.Commits)
	}

	return syncPulledScope(cfg)
}

// integrateRemote brings the remote's history into source. First pull (no
// upstream): fetch, then merge or reset onto the remote default branch and
// set upstream (see gitops.FirstPull). Subsequent pulls: normal git pull,
// which merges. remoteEmpty reports a remote with no branches yet.
// keepConfig (root scope) keeps this machine's config.yaml on later pulls even
// when the remote tracks one, and warns so the user can untrack it with push.
// FirstPull refuses that case instead (ErrRemoteTracksConfig).
func integrateRemote(source string, force, keepConfig bool, spinner *ui.Spinner) (info *gitops.UpdateInfo, remoteEmpty bool, err error) {
	if !gitops.HasUpstream(source) {
		spinner.Update("Fetching from remote...")
		info, err = gitops.FirstPull(source, force)
		if errors.Is(err, gitops.ErrNoRemoteBranches) {
			return nil, true, nil
		} else if errors.Is(err, gitops.ErrRemoteTracksConfig) {
			spinner.Fail("Remote tracks config.yaml")
			ui.Note("The remote repository tracks machine-specific config.yaml.")
			ui.Note("Untrack it on the remote first via 'skillshare push' from the machine that committed it, then pull.")
			return nil, false, err
		} else if err != nil {
			spinner.Fail("Pull failed")
			if errors.Is(err, gitops.ErrMergeFailed) {
				ui.Note(fmt.Sprintf("Resolve manually: cd %s && git merge --allow-unrelated-histories <remote branch>", source))
				ui.Note("Or force-pull: skillshare pull --force  (replaces local with remote)")
			} else if !isAuthError(err.Error()) {
				hintGitRemoteError(err.Error()) // auth guidance is already part of err
			}
			return nil, false, err
		}
		return info, false, nil
	}

	if keepConfig {
		// Not :=, which would shadow the named err the deferred restore reports into.
		restore, keepErr := gitops.KeepLocalConfig(source)
		if keepErr != nil {
			spinner.Fail("Pull failed")
			return nil, false, keepErr
		}
		defer func() {
			remoteTracks, restoreErr := restore()
			if restoreErr != nil {
				err = errors.Join(err, restoreErr)
			} else if remoteTracks {
				spinner.Warn("The remote tracks config.yaml; kept this machine's copy")
				ui.Note("Remove it from the remote: skillshare push")
			}
		}()
	}

	spinner.Update("Running git pull...")
	if info, err = gitops.PullWithEnv(source, gitops.AuthEnvForRepo(source)); err != nil {
		spinner.Fail("git pull failed")
		fmt.Println(err.Error())
		hintGitRemoteError(err.Error())
		return nil, false, fmt.Errorf("git pull failed: %w", err)
	}
	return info, false, nil
}

// syncPulledScope syncs what the git root scope holds (always global — pull
// operates on the global source). Sync opens with its own blank line, and
// extras are skipped when none are configured so sync does not print its
// setup guide. The config is read again because the pull may have changed it.
func syncPulledScope(cfg *config.Config) error {
	if pulled, err := config.Load(); err == nil {
		cfg.Extras = pulled.Extras
	}
	for _, args := range pulledScopeSyncArgs(cfg.GitRoot) {
		if args[0] == "extras" && len(cfg.Extras) == 0 {
			continue
		}
		if err := cmdSync(args); err != nil {
			return err
		}
	}
	return nil
}

// pulledScopeSyncArgs returns the `sync` invocations that cover a git root
// scope, in order. Retry hints print the same commands.
func pulledScopeSyncArgs(gitRoot string) [][]string {
	switch gitRoot {
	case "agents":
		return [][]string{{"agents", "--global"}}
	case "extras":
		return [][]string{{"extras", "--global"}}
	case "root":
		return [][]string{{"--global"}, {"agents", "--global"}, {"extras", "--global"}}
	}
	return [][]string{{"--global"}}
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
	printHelp("skillshare pull [options]", "Pull from git remote and sync to all targets.",
		helpGroup{title: "Options", rows: []helpRow{
			{"-n, --dry-run", "Preview changes without applying"},
			{"-f, --force", "Force-pull (reset local to remote on first pull)"},
		}},
		helpExamples(
			helpRow{"skillshare pull", "Pull and sync"},
			helpRow{"skillshare pull --dry-run", "Preview what would happen"},
			helpRow{"skillshare pull --force", "Discard local, use remote"},
		),
	)
}
