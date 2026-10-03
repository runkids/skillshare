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
		ui.Note("Run: skillshare init")
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
	if opts.pull {
		return fmt.Errorf("--pull is only supported by push: skillshare push --pull")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config not found: run 'skillshare init' first")
	}

	spinner := ui.StartSpinner("Checking repository...")
	source, err := resolveGitRoot(cfg, spinner)
	if err != nil {
		return err
	}

	if err := checkGitWorktree(source, spinner); err != nil {
		return err
	}

	sweep := rootScopeSafetySweep(cfg, source, opts.dryRun)
	if sweep.hasNotice() {
		spinner.Stop()
		sweep.printNotices(source)
		if len(sweep.nested) > 0 {
			return errNestedRepos // would commit empty submodules — abort
		}
		spinner = ui.StartSpinner("Checking changes...")
	}

	changes, err := getGitChanges(source)
	if err != nil {
		spinner.Fail("Failed to check git status")
		return err
	}
	changes = sweep.previewChanges(changes)
	if changes == "" {
		spinner.Stop()
		ui.Done(ui.MarkNone, "Nothing to commit", 0)
		return nil
	}

	if opts.dryRun {
		spinner.Stop()
		files := strings.Split(changes, "\n")
		ui.Row(ui.MarkNone, "Commit", "would commit "+plural(len(files), "file")+ui.DimText(" · "+opts.message), ui.RowWidth("Commit"))
		for _, line := range files {
			ui.Note(porcelainChange(line))
		}
		fmt.Println()
		ui.DryRun()
		return nil
	}

	if err := stageAndCommit(source, opts.message, spinner); err != nil {
		return err
	}

	spinner.Stop()
	files := strings.Split(changes, "\n")
	ui.Row(ui.MarkOK, "Commit", plural(len(files), "file")+ui.DimText(" · "+opts.message)+ui.Took(time.Since(start)), ui.RowWidth("Commit"))
	ui.Next("skillshare push", "share it with your other machines")

	e := oplog.NewEntry("commit", "ok", time.Since(start))
	e.Args = map[string]any{"message": opts.message}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func printCommitHelp() {
	printHelp("skillshare commit [options]", "Create a local git commit for source skills without pushing.",
		helpGroup{title: "Options", rows: []helpRow{
			{"-m, --message <msg>", "Commit message (default: \"Update skills\")"},
			{"-n, --dry-run", "Preview changes without applying"},
		}},
		helpExamples(
			helpRow{"skillshare commit", "Commit with default message"},
			helpRow{"skillshare commit -m \"Update skill\"", "Commit with custom message"},
			helpRow{"skillshare commit --dry-run", "Preview what would happen"},
		),
	)
}
