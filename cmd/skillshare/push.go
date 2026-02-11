package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

// pushOptions holds parsed push command options
type pushOptions struct {
	dryRun  bool
	message string
}

// parsePushArgs parses push command arguments
func parsePushArgs(args []string) *pushOptions {
	opts := &pushOptions{message: "Update skills"}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dry-run", "-n":
			opts.dryRun = true
		case "-m", "--message":
			if i+1 < len(args) {
				i++
				opts.message = args[i]
			}
		default:
			if strings.HasPrefix(arg, "-m=") {
				opts.message = strings.TrimPrefix(arg, "-m=")
			} else if strings.HasPrefix(arg, "--message=") {
				opts.message = strings.TrimPrefix(arg, "--message=")
			}
		}
	}

	return opts
}

// checkGitRepo verifies source is a git repo with remote
func checkGitRepo(sourcePath string, spinner *ui.Spinner) error {
	gitDir := sourcePath + "/.git"
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		spinner.Fail("Source is not a git repository")
		ui.Info("  Run: cd %s && git init", sourcePath)
		return fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "remote")
	cmd.Dir = sourcePath
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		spinner.Fail("No git remote configured")
		ui.Info("  Run: cd %s && git remote add origin <url>", sourcePath)
		ui.Info("  Or:  skillshare init --remote <url>")
		return fmt.Errorf("no remote configured")
	}

	return nil
}

// getGitChanges returns uncommitted changes
func getGitChanges(sourcePath string) (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = sourcePath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check git status: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// stageAndCommit stages all changes and commits
func stageAndCommit(sourcePath, message string, spinner *ui.Spinner) error {
	spinner.Update("Staging changes...")
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = sourcePath
	if err := cmd.Run(); err != nil {
		spinner.Fail("Failed to stage changes")
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	spinner.Update("Committing...")
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = sourcePath
	if err := cmd.Run(); err != nil {
		spinner.Fail("Failed to commit")
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// hintGitRemoteError prints helpful hints based on common git remote errors.
func hintGitRemoteError(output string) {
	switch {
	case strings.Contains(output, "Could not read from remote"):
		ui.Info("  Check SSH keys: ssh -T git@github.com")
		ui.Info("  Or use HTTPS:   git remote set-url origin https://github.com/you/repo.git")
	case strings.Contains(output, "not found") || strings.Contains(output, "does not exist"):
		ui.Info("  Check remote URL: git remote get-url origin")
	case strings.Contains(output, "could not resolve host"):
		ui.Info("  Check network connection")
	}
}

// gitPush pushes to remote
func gitPush(sourcePath string, spinner *ui.Spinner) error {
	spinner.Update("Pushing to remote...")
	cmd := exec.Command("git", "push")
	cmd.Dir = sourcePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		spinner.Fail("Push failed")
		outStr := string(output)
		fmt.Print(outStr)
		hintGitRemoteError(outStr)
		if !strings.Contains(outStr, "Could not read from remote") {
			ui.Info("  Remote may have newer changes")
			ui.Info("  Run: skillshare pull")
			ui.Info("  Then: skillshare push")
		}
		return fmt.Errorf("push failed")
	}
	return nil
}

func cmdPush(args []string) error {
	start := time.Now()
	opts := parsePushArgs(args)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config not found: run 'skillshare init' first")
	}

	ui.Header("Pushing to remote")

	spinner := ui.StartSpinner("Checking repository...")

	if err := checkGitRepo(cfg.Source, spinner); err != nil {
		return nil // Error already displayed
	}

	changes, err := getGitChanges(cfg.Source)
	if err != nil {
		spinner.Fail("Failed to check git status")
		return err
	}
	hasChanges := changes != ""

	if opts.dryRun {
		spinner.Stop()
		ui.Warning("[dry-run] No changes will be made")
		fmt.Println()
		if hasChanges {
			lines := strings.Split(changes, "\n")
			ui.Info("Would stage %d file(s):", len(lines))
			for _, line := range lines {
				ui.Info("  %s", line)
			}
			ui.Info("Would commit with message: %s", opts.message)
		} else {
			ui.Info("No changes to commit")
		}
		ui.Info("Would push to remote")
		return nil
	}

	if hasChanges {
		if err := stageAndCommit(cfg.Source, opts.message, spinner); err != nil {
			return err
		}
	}

	if err := gitPush(cfg.Source, spinner); err != nil {
		return nil // Error already displayed
	}

	spinner.Success("Push complete")

	e := oplog.NewEntry("push", "ok", time.Since(start))
	e.Args = map[string]any{"message": opts.message}
	oplog.Write(config.ConfigPath(), oplog.OpsFile, e) //nolint:errcheck

	return nil
}
