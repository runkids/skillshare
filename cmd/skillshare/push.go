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
	"skillshare/internal/install"
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
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = sourcePath
	if err := cmd.Run(); err != nil {
		spinner.Fail("Source is not a git repository")
		ui.Note("Run: skillshare init --remote <url>")
		return fmt.Errorf("not a git repository")
	}

	cmd = exec.Command("git", "remote")
	cmd.Dir = sourcePath
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		spinner.Fail("No git remote configured")
		ui.Note(fmt.Sprintf("Run: cd %s && git remote add origin <url>", sourcePath))
		ui.Note("Or:  skillshare init --remote <url>")
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
	return install.IsAuthError(output)
}

// hintGitRemoteError prints helpful hints based on common git remote errors.
func hintGitRemoteError(output string) {
	switch {
	case isAuthError(output):
		ui.Note("Authentication failed — options:")
		ui.Note("  1. SSH URL: git remote set-url origin git@<host>:<owner>/<repo>.git")
		ui.Note("  2. Token env var: GITHUB_TOKEN, GITLAB_TOKEN, BITBUCKET_TOKEN, AZURE_DEVOPS_TOKEN, or SKILLSHARE_GIT_TOKEN")
		if runtime.GOOS == "windows" {
			ui.Note("     PowerShell: $env:GITLAB_TOKEN = \"glpat-xxxx\"")
		} else {
			ui.Note("     export GITLAB_TOKEN=glpat-xxxx")
		}
		ui.Note("  3. Git credential helper: gh auth login")
		ui.Note("Docs: https://skillshare.runkids.cc/docs/reference/environment-variables#git-authentication")
	case strings.Contains(output, "Could not read from remote"):
		ui.Note("Check SSH keys: ssh -T git@github.com")
		ui.Note("Or use HTTPS:   git remote set-url origin https://github.com/you/repo.git")
	case strings.Contains(output, "not found") || strings.Contains(output, "does not exist"):
		ui.Note("Check remote URL: git remote get-url origin")
	case strings.Contains(output, "could not resolve host"):
		ui.Note("Check network connection")
	}
}

// gitPush pushes to remote, auto-setting upstream on first push.
func gitPush(sourcePath string, spinner *ui.Spinner) error {
	spinner.Update("Pushing to remote...")

	authEnv := gitops.AuthEnvForRepo(sourcePath)
	cmd := exec.Command("git", gitops.PushArgs(sourcePath, authEnv)...)
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
			ui.Note("Remote may have newer changes")
			ui.Next("skillshare pull", "get them first", "skillshare push", "then push again")
		}
		return fmt.Errorf("push failed")
	}
	return nil
}

func cmdPush(args []string) (err error) {
	if wantsHelp(args) {
		printPushHelp()
		return nil
	}

	start := time.Now()
	opts := parsePushArgs(args)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config not found: run 'skillshare init' first")
	}

	if !opts.dryRun {
		defer func() {
			e := oplog.NewEntry("push", statusFromErr(err), time.Since(start))
			e.Args = map[string]any{"message": opts.message}
			if err != nil {
				e.Message = err.Error()
			}
			oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
		}()
	}

	spinner := ui.StartSpinner("Checking repository...")

	source, err := resolveGitRoot(cfg, spinner)
	if err != nil {
		return err
	}

	if err := checkGitRepo(source, spinner); err != nil {
		return err
	}

	if sweep := rootScopeSafetySweep(cfg, source, opts.dryRun); sweep.hasNotice() {
		spinner.Stop()
		sweep.printNotices(source)
		if len(sweep.nested) > 0 {
			return errNestedRepos // would push empty submodules — abort
		}
		spinner = ui.StartSpinner("Checking changes...")
	}

	changes, err := getGitChanges(source)
	if err != nil {
		spinner.Fail("Failed to check git status")
		return err
	}
	hasChanges := changes != ""

	width := ui.RowWidth("Commit", "Push")
	var files []string
	if hasChanges {
		files = strings.Split(changes, "\n")
	}

	if opts.dryRun {
		spinner.Stop()
		if hasChanges {
			ui.Row(ui.MarkNone, "Commit", "would commit "+plural(len(files), "file")+ui.DimText(" · "+opts.message), width)
			for _, line := range files {
				ui.Note(porcelainChange(line))
			}
		} else {
			ui.Row(ui.MarkNone, "Commit", "nothing to commit", width)
		}
		ui.Row(ui.MarkNone, "Push", "would push to "+pushDestination(source), width)
		fmt.Println()
		ui.DryRun()
		return nil
	}

	if hasChanges {
		if err := stageAndCommit(source, opts.message, spinner); err != nil {
			return err
		}
		spinner.Stop()
		ui.Row(ui.MarkOK, "Commit", plural(len(files), "file")+ui.DimText(" · "+opts.message), width)
	} else {
		spinner.Stop()
		ui.Row(ui.MarkNone, "Commit", "nothing to commit", width)
	}

	spinner = ui.StartSpinner("Pushing to remote...")
	if err := gitPush(source, spinner); err != nil {
		return err
	}

	spinner.Stop()
	ui.Row(ui.MarkOK, "Push", "to "+pushDestination(source)+ui.DimText(fmt.Sprintf(" · %.1fs", time.Since(start).Seconds())), width)
	ui.Next("skillshare pull", "get these changes on another machine")
	return nil
}

// porcelainChange turns a `git status --porcelain` line into the "~ path",
// "+ path", "- path" form update uses for changed files.
func porcelainChange(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return line
	}
	path := strings.Join(fields[1:], " ")
	switch {
	case fields[0] == "??" || strings.Contains(fields[0], "A"):
		return "+ " + path
	case strings.Contains(fields[0], "D"):
		return "- " + path
	}
	return "~ " + path
}

// pushDestination names the upstream branch, such as "origin/main", or
// "origin" before the first push sets one. It never prints the remote URL,
// which can carry credentials.
func pushDestination(sourcePath string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = sourcePath
	if out, err := cmd.Output(); err == nil {
		if upstream := strings.TrimSpace(string(out)); upstream != "" {
			return upstream
		}
	}
	return "origin"
}

func printPushHelp() {
	fmt.Println(`Usage: skillshare push [options]

Commit and push source skills to git remote.

Options:
  -m, --message <msg>   Commit message (default: "Update skills")
  --dry-run, -n         Preview changes without applying
  --help, -h            Show this help

Examples:
  skillshare push                      Push with default message
  skillshare push -m "Add new skill"   Push with custom message
  skillshare push --dry-run            Preview what would happen`)
}
