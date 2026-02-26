package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"skillshare/internal/config"
	gitops "skillshare/internal/git"
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
		ui.Info("  Run: skillshare init --remote <url>")
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

// isAuthError returns true when git output indicates an authentication failure.
func isAuthError(output string) bool {
	return strings.Contains(output, "Authentication failed") ||
		strings.Contains(output, "Access denied") ||
		strings.Contains(output, "could not read Username") ||
		strings.Contains(output, "terminal prompts disabled")
}

// hintGitRemoteError prints helpful hints based on common git remote errors.
func hintGitRemoteError(output string) {
	switch {
	case isAuthError(output):
		ui.Info("  Authentication failed — options:")
		ui.Info("    1. SSH URL: git remote set-url origin git@<host>:<owner>/<repo>.git")
		ui.Info("    2. Token env var: GITHUB_TOKEN, GITLAB_TOKEN, BITBUCKET_TOKEN, AZURE_DEVOPS_TOKEN, or SKILLSHARE_GIT_TOKEN")
		if runtime.GOOS == "windows" {
			ui.Info("       PowerShell: $env:GITLAB_TOKEN = \"glpat-xxxx\"")
		} else {
			ui.Info("       export GITLAB_TOKEN=glpat-xxxx")
		}
		ui.Info("    3. Git credential helper: gh auth login")
		ui.Info("  Docs: https://skillshare.runkids.cc/docs/reference/environment-variables#git-authentication")
	case strings.Contains(output, "Could not read from remote"):
		ui.Info("  Check SSH keys: ssh -T git@github.com")
		ui.Info("  Or use HTTPS:   git remote set-url origin https://github.com/you/repo.git")
	case strings.Contains(output, "not found") || strings.Contains(output, "does not exist"):
		ui.Info("  Check remote URL: git remote get-url origin")
	case strings.Contains(output, "could not resolve host"):
		ui.Info("  Check network connection")
	}
}

// gitPush pushes to remote, auto-setting upstream on first push.
func gitPush(sourcePath string, spinner *ui.Spinner) error {
	spinner.Update("Pushing to remote...")

	authEnv := gitops.AuthEnvForRepo(sourcePath)
	args := []string{"push"}
	localBranch, err := gitops.GetCurrentBranch(sourcePath)
	if err != nil {
		localBranch = "main"
	}

	if !gitops.HasUpstream(sourcePath) {
		remoteBranch := detectRemoteDefaultBranchForPush(sourcePath, authEnv)
		if remoteBranch != "" && remoteBranch != localBranch {
			args = append(args, "-u", "origin", localBranch+":"+remoteBranch)
		} else {
			args = append(args, "-u", "origin", localBranch)
		}
	} else if remoteName, upstreamBranch := upstreamTrackingBranch(sourcePath); remoteName != "" && upstreamBranch != "" && upstreamBranch != localBranch {
		// push.default=simple refuses pushing when upstream branch name differs
		// from local branch name. Push explicitly to tracked upstream branch.
		args = []string{"push", remoteName, "HEAD:" + upstreamBranch}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = sourcePath
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if len(authEnv) > 0 {
		cmd.Env = append(cmd.Env, authEnv...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		spinner.Fail("Push failed")
		outStr := string(output)
		fmt.Print(outStr)
		hintGitRemoteError(outStr)
		if !strings.Contains(outStr, "Could not read from remote") && !isAuthError(outStr) {
			ui.Info("  Remote may have newer changes")
			ui.Info("  Run: skillshare pull")
			ui.Info("  Then: skillshare push")
		}
		return fmt.Errorf("push failed")
	}
	return nil
}

func upstreamTrackingBranch(sourcePath string) (remoteName, branch string) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = sourcePath
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	ref := strings.TrimSpace(string(out))
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func detectRemoteDefaultBranchForPush(sourcePath string, authEnv []string) string {
	fetchCmd := exec.Command("git", "fetch", "origin")
	fetchCmd.Dir = sourcePath
	if len(authEnv) > 0 {
		fetchCmd.Env = append(os.Environ(), authEnv...)
	}
	if err := fetchCmd.Run(); err != nil {
		return ""
	}

	branch, err := gitops.GetRemoteDefaultBranch(sourcePath)
	if err != nil {
		return ""
	}
	return branch
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
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}
