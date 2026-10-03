package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
	gitops "skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	ssync "skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"

	"golang.org/x/term"
)

const skillshareSkillSource = "github.com/runkids/skillshare/skills/skillshare"
const remoteFetchTimeout = 15 * time.Second

// initOptions holds all parsed arguments for the init command
type initOptions struct {
	sourcePath   string
	remoteURL    string
	dryRun       bool
	copyFrom     string
	noCopy       bool
	targetsArg   string
	allTargets   bool
	noTargets    bool
	initGit      bool
	noGit        bool
	gitFlagSet   bool
	gitRootScope string
	initSkill    bool
	noSkill      bool
	skillFlagSet bool
	discover     bool
	selectArg    string
	mode         string
	subdir       string
}

// parseInitArgs parses command line arguments into initOptions
// errHelp is a sentinel error indicating --help was requested.
var errHelp = fmt.Errorf("help requested")

func printInitUsage() {
	printHelp("skillshare init [options]", "Initialize skillshare — detect AI CLI tools, create config, and set up skill syncing.",
		helpGroup{title: "Options", rows: []helpRow{
			{"-s, --source <path>", "Set source directory (default: ~/.config/skillshare/skills)"},
			{"--remote <url>", "Set git remote for cross-machine sync (implies --git)"},
			{"-p, --project", "Initialize project-level config in current directory"},
			{"-c, --copy-from <name>", "Copy existing skills from a detected CLI directory"},
			{"--no-copy", "Start with empty source (skip copy prompt)"},
			{"-t, --targets <list>", "Comma-separated target names to add"},
			{"--all-targets", "Add all detected targets"},
			{"--no-targets", "Skip target setup"},
			{"-m, --mode <mode>", "Set sync mode (merge, copy, symlink; default: merge).\nWith --discover, applies only to newly added targets"},
			{"--git", "Initialize git in source (default: prompt)"},
			{"--no-git", "Skip git initialization"},
			{"--git-root <scope>", "Git scope: skills (default), agents, extras, or root"},
			{"--skill", "Install built-in skillshare skill"},
			{"--no-skill", "Skip built-in skill installation"},
			{"-d, --discover", "Detect and add new AI CLI agents to existing config"},
			{"--select <list>", "Select specific agents to add (requires --discover)"},
			{"--subdir <name>", "Use a subdirectory as the source (e.g. skills/)"},
			{"-n, --dry-run", "Preview without making changes"},
		}},
		helpExamples(
			helpRow{"skillshare init", "Interactive setup"},
			helpRow{"skillshare init --remote git@github.com:u/skills", "With cross-machine sync"},
			helpRow{"skillshare init --no-copy --git --no-skill", "Non-interactive, minimal"},
			helpRow{"skillshare init --discover", "Add newly installed AI tools"},
			helpRow{"skillshare init -p", "Project-level init"},
		),
	)
}

func parseInitArgs(args []string) (*initOptions, error) {
	opts := &initOptions{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			return nil, errHelp
		case "--source", "-s":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--source requires a path argument")
			}
			opts.sourcePath = args[i+1]
			i++
		case "--remote":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--remote requires a URL argument")
			}
			opts.remoteURL = args[i+1]
			i++
		case "--dry-run", "-n":
			opts.dryRun = true
		case "--copy-from", "-c":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--copy-from requires a name or path argument")
			}
			opts.copyFrom = args[i+1]
			i++
		case "--no-copy":
			opts.noCopy = true
		case "--targets", "-t":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--targets requires a comma-separated list")
			}
			opts.targetsArg = args[i+1]
			i++
		case "--all-targets":
			opts.allTargets = true
		case "--no-targets":
			opts.noTargets = true
		case "--mode", "-m":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--mode requires a value (merge, copy, or symlink)")
			}
			opts.mode = args[i+1]
			i++
		case "--git":
			opts.initGit = true
			opts.gitFlagSet = true
		case "--no-git":
			opts.noGit = true
		case "--git-root":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--git-root requires a scope argument (skills|agents|extras|root)")
			}
			opts.gitRootScope = args[i+1]
			i++
		case "--skill":
			opts.initSkill = true
			opts.skillFlagSet = true
		case "--no-skill":
			opts.noSkill = true
		case "--subdir":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--subdir requires a directory name")
			}
			opts.subdir = args[i+1]
			i++
		case "--discover", "-d":
			opts.discover = true
		case "--select":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--select requires a comma-separated list")
			}
			opts.selectArg = args[i+1]
			i++
		}
	}

	return opts, nil
}

// validateInitOptions validates mutual exclusions and adjusts defaults
func validateInitOptions(opts *initOptions, home string) error {
	if opts.copyFrom != "" && opts.noCopy {
		return fmt.Errorf("--copy-from and --no-copy are mutually exclusive")
	}

	exclusiveCount := 0
	if opts.targetsArg != "" {
		exclusiveCount++
	}
	if opts.allTargets {
		exclusiveCount++
	}
	if opts.noTargets {
		exclusiveCount++
	}
	if exclusiveCount > 1 {
		return fmt.Errorf("--targets, --all-targets, and --no-targets are mutually exclusive")
	}

	if opts.gitFlagSet && opts.noGit {
		return fmt.Errorf("--git and --no-git are mutually exclusive")
	}

	if opts.gitRootScope != "" {
		if !config.ValidGitRoot(opts.gitRootScope) {
			return fmt.Errorf("invalid --git-root %q (want: skills, agents, extras, or root)", opts.gitRootScope)
		}
		if opts.noGit {
			return fmt.Errorf("--git-root cannot be combined with --no-git")
		}
	}

	if opts.skillFlagSet && opts.noSkill {
		return fmt.Errorf("--skill and --no-skill are mutually exclusive")
	}

	if opts.selectArg != "" && !opts.discover {
		return fmt.Errorf("--select requires --discover flag")
	}
	opts.mode = normalizeSyncMode(opts.mode)
	if err := validateSyncMode(opts.mode); err != nil {
		return err
	}

	// --remote implies --git
	if opts.remoteURL != "" && !opts.noGit {
		opts.initGit = true
	}

	// Expand ~ in path
	if utils.HasTildePrefix(opts.sourcePath) {
		opts.sourcePath = filepath.Join(home, opts.sourcePath[1:])
	}

	return nil
}

func normalizeSyncMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func validateSyncMode(mode string) error {
	if mode == "" {
		return nil
	}
	switch mode {
	case "merge", "copy", "symlink":
		return nil
	default:
		return fmt.Errorf("invalid --mode value %q (expected: merge, copy, or symlink)", mode)
	}
}

func runningInInteractiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func modeOverrideForTarget(requestedMode, inheritedDefault string) string {
	requestedMode = normalizeSyncMode(requestedMode)
	defaultMode := normalizeSyncMode(inheritedDefault)
	if defaultMode == "" {
		defaultMode = "merge"
	}
	if requestedMode == "" {
		return ""
	}
	if requestedMode == "merge" && defaultMode == "merge" {
		return ""
	}
	return requestedMode
}

// handleExistingInit handles init when config already exists
func handleExistingInit(opts *initOptions) (bool, error) {
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		return false, nil // Not initialized, continue with fresh init
	}

	// Headless scope switch: `skillshare init --git-root <scope>` on an existing
	// setup initializes a repo at the new scope directory and persists git_root.
	// It does not relocate an existing repo (run `mv <old>/.git <new>/.git` for
	// that). With --remote also given, fall through so the remote is configured
	// on the just-switched scope's repo.
	if opts.gitRootScope != "" {
		cfg, err := config.Load()
		if err != nil {
			return true, err
		}
		if err := switchGitRootScope(cfg, opts.gitRootScope, opts.dryRun); err != nil {
			return true, err
		}
		if opts.remoteURL == "" {
			return true, nil
		}
	}

	// If --remote provided, just add the remote to existing setup
	if opts.remoteURL != "" {
		cfg, err := config.Load()
		if err != nil {
			return true, err
		}
		// Ensure git is initialized before setting up remote
		// (--remote implies --git unless --no-git is also passed)
		scope := cfg.GitRoot
		if scope == "" {
			scope = "skills"
		}
		gitRoot := cfg.EffectiveGitRoot()
		if opts.noGit {
			ui.Row(ui.MarkNone, "Git", "skipped"+ui.DimText(" · --no-git"), gitRowWidth)
		} else {
			doGitInitIfAbsent(gitRoot, scope, opts.dryRun)
			// Commit any uncommitted source files so push/pull work cleanly
			if !opts.dryRun {
				if err := commitSourceFiles(gitRoot); err != nil {
					ui.Warning("Failed to commit source files: %v", err)
				}
			}
		}
		setupGitRemote(gitRoot, opts.remoteURL, opts.dryRun, opts.noGit)
		if opts.dryRun {
			fmt.Println()
			ui.DryRun()
		}
		return true, nil
	}

	// If --discover provided, detect and add new agents
	if opts.discover {
		cfg, err := config.Load()
		if err != nil {
			return true, err
		}
		return true, reinitWithDiscover(cfg, opts.selectArg, opts.dryRun, opts.mode)
	}

	return true, fmt.Errorf("skillshare is already initialized (global config: %s). Run 'skillshare init --discover' to add new agents, or 'skillshare init -p' to initialize project-level skills", config.ConfigPath())
}

// switchGitRootScope changes git_root on an already-initialized setup. It
// initializes a git repo at the new scope directory if absent (reusing one
// already there) and persists git_root to config. This is the headless
// counterpart to the web UI's POST /api/git/root; it never relocates an
// existing repo — switching scope means "version a different directory", not
// "move the old history" (run `mv <old>/.git <new>/.git` to carry it over).
func switchGitRootScope(cfg *config.Config, scope string, dryRun bool) error {
	dir := config.ScopeDir(cfg, scope)
	doGitInitIfAbsent(dir, scope, dryRun)
	if dryRun {
		ui.Row(ui.MarkNone, "Git root", "would set to "+scope, gitRowWidth)
		fmt.Println()
		ui.DryRun()
		return nil
	}
	// "skills" is the default — store it as empty to keep config.yaml clean.
	if scope == "skills" {
		cfg.GitRoot = ""
	} else {
		cfg.GitRoot = scope
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	ui.Row(ui.MarkOK, "Git root", scope+ui.DimText(" · versioning "+shortenPath(dir)), gitRowWidth)
	return nil
}

// performFreshInit sets skillshare up on a machine with no config. It
// gathers a plan first (flags, detection, then answers when interactive)
// and writes nothing until the plan is confirmed.
func performFreshInit(opts *initOptions, home string) (*initResult, error) {
	interactive := runningInInteractiveTTY()
	detected := detectCLIDirectories()
	printInitBanner(detected, interactive)

	p := newInitPlan(opts, detected, home)
	if interactive {
		if err := askInitPlan(p, opts, detected); err != nil {
			return nil, initCancelled(err)
		}
		if !p.dryRun {
			ok, err := confirmPlan(p, home)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, initCancelled(ui.ErrCancelled)
			}
		}
	} else {
		resolveHeadlessRemote(p, opts)
		printHeadlessPlan(p)
	}

	if p.dryRun {
		fmt.Println(strings.Join(summaryLines(p, "Dry run — nothing was written", ""), "\n"))
		ui.Note("Run without --dry-run to set it up")
		return nil, nil
	}

	spinner := ui.StartSpinner("Setting up…")
	res, err := applyInitPlan(p)
	spinner.Stop()
	if err != nil {
		return res, err
	}
	printInitDone(p, res)

	synced := false
	if len(p.targets) > 0 {
		doSync := true
		if interactive {
			doSync, err = ui.Confirm(fmt.Sprintf("Sync to %s now?", plural(len(p.targets), "tool")), true)
			if err != nil {
				doSync = false
			}
		}
		if doSync {
			_, kept, syncErr := firstSync(res.cfg)
			synced = syncErr == nil
			if syncErr != nil {
				ui.Warning("%v", syncErr)
			}
			if len(kept) > 0 {
				ui.Warning("Kept local copies that differ from the source: %s", strings.Join(kept, ", "))
				ui.Note("Replace them with links: skillshare sync --force")
			}
		}
	}
	printInitNext(p, synced, interactive)
	return res, nil
}

// initCancelled reports a cancelled prompt; the message says nothing was
// written and the command exits non-zero without another error line.
func initCancelled(err error) error {
	if !errors.Is(err, ui.ErrCancelled) {
		return err
	}
	fmt.Println()
	ui.Cancelled("written")
	return &jsonSilentError{cause: err}
}

// hasGlobalOnlyInitFlags returns true if args contain flags only valid for global init
// (e.g. --remote, --source). Used to skip project-mode auto-detection.
func hasGlobalOnlyInitFlags(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--remote", "--source", "-s", "--copy-from", "-c", "--no-copy",
			"--git", "--no-git", "--skill", "--no-skill", "--subdir":
			return true
		}
	}
	return false
}

func cmdInit(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	applyModeLabel(mode)

	if mode == modeProject {
		return cmdInitProject(rest)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	if mode == modeAuto && !hasGlobalOnlyInitFlags(rest) {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil && projectConfigExists(cwd) {
			applyModeLabel(modeProject)
			return cmdInitProject(rest)
		}
	}

	opts, err := parseInitArgs(rest)
	if err == errHelp {
		printInitUsage()
		return nil
	}
	if err != nil {
		return err
	}

	if err := validateInitOptions(opts, home); err != nil {
		return err
	}

	handled, err := handleExistingInit(opts)
	if handled {
		logInitOp(config.ConfigPath(), 0, false, false, false, start, err)
		return err
	}

	res, cmdErr := performFreshInit(opts, home)
	if res != nil {
		logInitOp(config.ConfigPath(), len(res.cfg.Targets), true, res.gitInit, res.skillInstalled, start, cmdErr)
	}
	return cmdErr
}

func logInitOp(cfgPath string, targetsAdded int, sourceCreated bool, gitInit bool, skillInstalled bool, start time.Time, cmdErr error) {
	e := oplog.NewEntry("init", statusFromErr(cmdErr), time.Since(start))
	a := map[string]any{}
	if targetsAdded > 0 {
		a["targets_added"] = targetsAdded
	}
	if sourceCreated {
		a["source_created"] = true
	}
	if gitInit {
		a["git_init"] = true
	}
	if skillInstalled {
		a["skill_installed"] = true
	}
	e.Args = a
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

// doGitInitIfAbsent initializes a repo at gitRoot (with a scope-aware .gitignore)
// unless one already exists there. dryRun only prints intentions. The actual git
// work is shared with the web server via gitops.InitScopeRepo.
// gitRowWidth aligns the Git, Remote, Pull and Git root rows that init
// prints when it changes git on an existing setup.
var gitRowWidth = ui.RowWidth("Git root")

func doGitInitIfAbsent(gitRoot, scope string, dryRun bool) {
	if hasGitDir(gitRoot) {
		ui.Row(ui.MarkNone, "Git", "already set up in "+shortenPath(gitRoot), gitRowWidth)
		// Still ensure the scope-aware .gitignore (e.g. config.yaml at root scope).
		if !dryRun {
			if err := gitops.WriteScopeGitignore(gitRoot, scope); err != nil {
				ui.Warning("Failed to update .gitignore: %v", err)
			}
		}
		return
	}
	if dryRun {
		ui.Row(ui.MarkNone, "Git", "would initialize in "+shortenPath(gitRoot), gitRowWidth)
		return
	}

	identitySet, err := gitops.InitScopeRepo(gitRoot, scope)
	if err != nil {
		ui.Warning("Failed to initialize git: %v", err)
		return
	}
	ui.Row(ui.MarkOK, "Git", "initialized in "+shortenPath(gitRoot), gitRowWidth)
	if identitySet {
		printGitIdentityNote(gitRoot)
	}
}

// printGitIdentityNote explains the identity init wrote into the new repo
// at dir when git had none. It lives in that repo's config, which outranks
// --global, so the commands to replace it target the repo.
func printGitIdentityNote(dir string) {
	ui.Note("Git had no identity, so this repo commits as skillshare@local. Set yours:")
	ui.Note(fmt.Sprintf(`git -C %s config user.name "Your Name"`, shortenPath(dir)))
	ui.Note(fmt.Sprintf(`git -C %s config user.email "you@example.com"`, shortenPath(dir)))
}

// commitSourceFiles creates a single commit with all source files
// (.gitignore, copied skills, installed skills).
func commitSourceFiles(sourcePath string) error {
	gitDir := filepath.Join(sourcePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = sourcePath
	if out, err := addCmd.CombinedOutput(); err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return fmt.Errorf("git add failed")
		}
		return fmt.Errorf("git add failed: %s", trimmed)
	}

	// Determine commit message based on content
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}
	hasSkills := false
	for _, e := range entries {
		if e.Name() != ".git" && e.Name() != ".gitignore" {
			hasSkills = true
			break
		}
	}

	msg := "Initial commit"
	if hasSkills {
		msg = "Initial skills"
	}
	commitCmd := exec.Command("git", "commit", "-m", msg)
	commitCmd.Dir = sourcePath
	// A repo the user created may have no identity; commit with a fallback
	// for this one commit instead of writing one into their repo config.
	commitCmd.Env = append(append(os.Environ(), "LC_ALL=C"), gitops.FallbackIdentityEnv(sourcePath)...)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		trimmed := strings.TrimSpace(string(out))
		if strings.Contains(trimmed, "nothing to commit") || strings.Contains(trimmed, "no changes added to commit") {
			return nil
		}
		if trimmed == "" {
			return fmt.Errorf("git commit failed")
		}
		return fmt.Errorf("git commit failed: %s", trimmed)
	}
	return nil
}

func setupGitRemote(sourcePath, remoteURL string, dryRun, noGit bool) bool {
	// Check if git is initialized
	gitDir := filepath.Join(sourcePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if remoteURL == "" {
			return false
		}
		if dryRun && !noGit {
			// The dry run reported the git init that would come first.
			ui.Row(ui.MarkNone, "Remote", "would add origin → "+remoteURL, gitRowWidth)
			return false
		}
		ui.Row(ui.MarkWarn, "Remote", "not added: "+shortenPath(sourcePath)+" has no git repo", gitRowWidth)
		if noGit {
			ui.Note("Run: skillshare init --remote " + remoteURL + " (without --no-git)")
		}
		return false
	}

	// Check if remote already exists
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = sourcePath
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		existingRemote := strings.TrimSpace(string(output))
		if existingRemote == remoteURL {
			ui.Row(ui.MarkNone, "Remote", "already origin → "+existingRemote, gitRowWidth)
		} else {
			ui.Row(ui.MarkWarn, "Remote", "origin is already "+existingRemote, gitRowWidth)
			ui.Note(fmt.Sprintf("To change it: git -C %s remote set-url origin %s", shortenPath(sourcePath), remoteURL))
		}
		return false
	}

	if dryRun {
		ui.Row(ui.MarkNone, "Remote", "would add origin → "+remoteURL, gitRowWidth)
		return false
	}
	return addRemote(sourcePath, remoteURL)
}

func addRemote(sourcePath, remoteURL string) bool {
	// SetOrAddRemote validates the URL (rejecting flag-smuggling values that
	// begin with "-") and passes "--" before positional args. setupGitRemote
	// has already confirmed origin is absent, so this takes the add path.
	if err := gitops.SetOrAddRemote(sourcePath, remoteURL); err != nil {
		ui.Warning("Failed to add remote: %v", err)
		return false
	}

	ui.Row(ui.MarkOK, "Remote", "origin → "+remoteURL, gitRowWidth)

	// Try to fetch and auto-pull if remote has existing skills
	hadSkills := tryPullAfterRemoteSetup(sourcePath, remoteURL)
	if !hadSkills {
		ui.Next("skillshare push", "share your skills")
	}
	return hadSkills
}

// remoteFetchEnv returns env vars for non-interactive remote checks.
func remoteFetchEnv(remoteURL string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"LC_ALL=C",
	)
	// Keep user-provided SSH command if present (custom key/proxy).
	if v, ok := os.LookupEnv("GIT_SSH_COMMAND"); !ok || strings.TrimSpace(v) == "" {
		env = append(env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes -o ConnectTimeout=8")
	}
	if authEnv := install.AuthEnvForURL(remoteURL); len(authEnv) > 0 {
		env = append(env, authEnv...)
	}
	return env
}

// tryPullAfterRemoteSetup attempts to fetch from remote and pull if it has content.
// Returns true if remote had content (pulled or warned), false if remote is empty/unreachable.
func tryPullAfterRemoteSetup(sourcePath, remoteURL string) bool {
	spinner := ui.StartSpinner("Checking remote for existing skills...")

	// Try to fetch (inject HTTPS token auth when available)
	fetchCtx, cancel := context.WithTimeout(context.Background(), remoteFetchTimeout)
	defer cancel()
	fetchCmd := exec.CommandContext(fetchCtx, "git", "fetch", "origin")
	fetchCmd.Dir = sourcePath
	fetchCmd.Env = remoteFetchEnv(remoteURL)
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		spinner.Stop()
		if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) {
			ui.Row(ui.MarkWarn, "Pull", "remote check timed out"+ui.DimText(" · push and pull retry it"), gitRowWidth)
			return false
		}
		ui.Row(ui.MarkWarn, "Pull", "could not reach the remote"+ui.DimText(" · push and pull retry it"), gitRowWidth)
		outStr := strings.TrimSpace(string(output))
		if strings.Contains(outStr, "not found") || strings.Contains(outStr, "does not exist") {
			ui.Note("Check the URL: git -C " + shortenPath(sourcePath) + " remote get-url origin")
		} else if strings.Contains(outStr, "Could not read from remote") && !strings.HasPrefix(remoteURL, "file://") {
			ui.Note("Check SSH keys: ssh -T git@github.com")
		}
		return false
	}

	remoteBranch, err := gitops.GetRemoteDefaultBranch(sourcePath)
	if err != nil {
		spinner.Stop()
		if errors.Is(err, gitops.ErrNoRemoteBranches) {
			ui.Row(ui.MarkNone, "Pull", "remote is empty", gitRowWidth)
			return false
		}
		ui.Row(ui.MarkWarn, "Pull", "could not find the remote's default branch"+ui.DimText(" · push and pull retry it"), gitRowWidth)
		return false
	}

	hasRemoteSkills, err := gitops.HasRemoteSkillDirs(sourcePath, remoteBranch)
	if err != nil {
		spinner.Stop()
		ui.Row(ui.MarkWarn, "Pull", "could not read the remote's skills"+ui.DimText(" · push and pull retry it"), gitRowWidth)
		return false
	}
	if !hasRemoteSkills {
		spinner.Stop()
		ui.Row(ui.MarkNone, "Pull", "remote has no skills", gitRowWidth)
		return false
	}

	hasLocalSkills, err := gitops.HasLocalContent(sourcePath)
	if err != nil {
		spinner.Stop()
		ui.Row(ui.MarkWarn, "Pull", "could not read the local skills"+ui.DimText(" · pull retries it"), gitRowWidth)
		return true
	}

	if hasLocalSkills {
		spinner.Stop()
		ui.Row(ui.MarkWarn, "Pull", "skipped: both the remote and this machine have skills", gitRowWidth)
		ui.Next(
			"skillshare pull", "merge both",
			"skillshare push", "keep this machine's skills",
			"skillshare pull --force", "replace them with the remote's",
		)
		return true
	}

	if gitops.RemoteTracksConfig(sourcePath, "origin/"+remoteBranch) && gitops.HasLocalRootConfig(sourcePath) {
		spinner.Warn("Remote tracks machine-specific config.yaml")
		ui.Info("  Untrack it on the remote first via 'skillshare push' from the machine that committed it, then pull.")
		return true
	}

	// Local is empty — reset to remote branch for clean linear history.
	// This is safe because we verified hasLocalSkills is false above.
	spinner.Update("Pulling skills from remote...")

	resetCmd := exec.Command("git", "reset", "--hard", "origin/"+remoteBranch)
	resetCmd.Dir = sourcePath
	if output, err := resetCmd.CombinedOutput(); err != nil {
		spinner.Stop()
		ui.Row(ui.MarkFail, "Pull", "failed", gitRowWidth)
		fmt.Println(strings.TrimSpace(string(output)))
		ui.Note(fmt.Sprintf("Try it by hand: git -C %s reset --hard origin/%s", shortenPath(sourcePath), remoteBranch))
		return true
	}

	// Set up tracking so push/pull work without specifying remote
	localBranch := "main"
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = sourcePath
	if out, err := branchCmd.Output(); err == nil {
		if b := strings.TrimSpace(string(out)); b != "" {
			localBranch = b
		}
	}
	trackCmd := exec.Command("git", "branch", "--set-upstream-to=origin/"+remoteBranch, localBranch)
	trackCmd.Dir = sourcePath
	trackCmd.Run() // best-effort

	// Count pulled skills (deep: count SKILL.md files, not top-level dirs)
	discovered, _ := ssync.DiscoverSourceSkills(sourcePath)
	skillCount := len(discovered)

	spinner.Stop()
	ui.Row(ui.MarkOK, "Pull", plural(skillCount, "skill")+" from origin/"+remoteBranch, gitRowWidth)
	ui.Next("skillshare sync", "link them into your targets")
	return true
}

const fallbackSkillContent = `---
name: skillshare
description: Manage and sync skills across AI CLI tools
---

# Skillshare CLI

Run ` + "`skillshare update`" + ` to download the full skill with AI integration guide.

## Quick Commands

- ` + "`skillshare status`" + ` - Show sync state
- ` + "`skillshare sync`" + ` - Sync to all targets
- ` + "`skillshare pull <target>`" + ` - Pull from target
- ` + "`skillshare update`" + ` - Update this skill
`

// agentInfo holds information about a detected agent for discover mode
type agentInfo struct {
	name        string
	path        string
	description string
}

// detectNewAgents finds agents not already in the config
func detectNewAgents(existingCfg *config.Config) []agentInfo {
	defaultTargets := config.DefaultTargets()
	var newAgents []agentInfo
	universalAlias := false

	for name, target := range defaultTargets {
		if isSharedUniversalSkillsAlias(name, target, defaultTargets) {
			// Detected through its install dir instead; counts for universal.
			if dir := config.DetectDir(name); dir != "" {
				if _, err := os.Stat(dir); err == nil {
					universalAlias = true
				}
			}
			continue
		}

		if _, exists := existingCfg.Targets[name]; exists {
			continue
		}

		sc := target.SkillsConfig()
		parent := filepath.Dir(sc.Path)
		if _, err := os.Stat(parent); err != nil {
			continue
		}

		status := getAgentStatus(sc.Path)
		newAgents = append(newAgents, agentInfo{
			name:        name,
			path:        sc.Path,
			description: status,
		})
	}

	sort.Slice(newAgents, func(i, j int) bool { return newAgents[i].name < newAgents[j].name })

	// Auto-include universal if any new agent is found but universal isn't
	// already configured and not yet in the candidate list.
	if len(newAgents) > 0 || universalAlias {
		if _, configured := existingCfg.Targets["universal"]; !configured {
			if !sliceHasName(newAgents, "universal", func(a agentInfo) string { return a.name }) {
				if target, ok := defaultTargets["universal"]; ok {
					newAgents = append(newAgents, agentInfo{
						name:        "universal",
						path:        target.SkillsConfig().Path,
						description: sharedSkillsDirectoryDescription,
					})
				}
			}
		}
	}

	return newAgents
}

// getAgentStatus returns the status description for an agent path
func getAgentStatus(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "(not initialized)"
	}

	entries, _ := os.ReadDir(path)
	skillCount := 0
	for _, e := range entries {
		if e.IsDir() && !utils.IsHidden(e.Name()) {
			skillCount++
		}
	}

	if skillCount > 0 {
		return fmt.Sprintf("(%d skills)", skillCount)
	}
	return "(empty)"
}

// promptAgentSelection asks which new tools to add; all start checked.
func promptAgentSelection(newAgents []agentInfo) ([]string, error) {
	options := make([]ui.Option, len(newAgents))
	values := make([]string, len(newAgents))
	for i, agent := range newAgents {
		options[i] = ui.Option{Label: fmt.Sprintf("%-14s %s", agent.name, theme.Dim().Render(utils.FoldHomePath(agent.path)+" "+agent.description)), Value: agent.name}
		values[i] = agent.name
	}
	names, err := ui.MultiSelect("Add which tools?", options, values)
	if err != nil {
		return nil, err
	}
	ui.Answered("Targets", describeTools(names))
	return names, nil
}

// saveAddedAgents adds agents to config and saves
func saveAddedAgents(cfg *config.Config, names []string, asked, dryRun bool, mode string) error {
	defaultTargets := config.DefaultTargets()

	for _, name := range names {
		if target, ok := defaultTargets[name]; ok {
			target.EnsureSkills().Mode = modeOverrideForTarget(mode, cfg.Mode)
			cfg.Targets[name] = target
		}
	}

	if !dryRun {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	printDiscoverResult(config.ConfigPath(), names, asked, dryRun, "skillshare sync")
	return nil
}

// printDiscoverResult reports the tools --discover added in init's style:
// answered rows, then the command to run next. asked is true when the tools
// prompt already printed its own Targets row.
func printDiscoverResult(configPath string, names []string, asked, dryRun bool, syncCmd string) {
	if !asked {
		ui.Answered("Targets", describeTools(names))
	}
	if dryRun {
		fmt.Println(theme.Dim().Render("Dry run — nothing was written"))
		return
	}
	ui.Answered("Config", utils.FoldHomePath(configPath))
	fmt.Println()
	fmt.Println(theme.Primary().Bold(true).Render("Next"))
	fmt.Printf("  %s  %s\n", theme.Accent().Render(syncCmd), theme.Dim().Render("sync your skills to "+describeTools(names)))
}

func discoverFound(n int) {
	fmt.Println("Found " + plural(n, "new AI tool"))
}

// reinitWithDiscover detects new agents and allows user to add them to existing config
func reinitWithDiscover(existingCfg *config.Config, selectArg string, dryRun bool, mode string) error {
	newAgents := detectNewAgents(existingCfg)
	if len(newAgents) == 0 {
		fmt.Println("No new AI tools found")
		return nil
	}

	discoverFound(len(newAgents))

	// Non-interactive mode with --select
	if selectArg != "" {
		return addSelectedAgentsByName(existingCfg, newAgents, selectArg, dryRun, mode)
	}

	// No one to ask: add every new tool, as Enter would choose.
	selectedNames := make([]string, 0, len(newAgents))
	for _, agent := range newAgents {
		selectedNames = append(selectedNames, agent.name)
	}
	asked := runningInInteractiveTTY()
	if asked {
		var err error
		if selectedNames, err = promptAgentSelection(newAgents); err != nil {
			return initCancelled(err)
		}
	}

	if len(selectedNames) == 0 {
		fmt.Println("No AI tools added")
		return nil
	}

	return saveAddedAgents(existingCfg, selectedNames, asked, dryRun, mode)
}

// addSelectedAgentsByName adds agents specified by --select flag (non-interactive)
func addSelectedAgentsByName(existingCfg *config.Config, newAgents []agentInfo, selectArg string, dryRun bool, mode string) error {
	defaultTargets := config.DefaultTargets()

	// Build a map of available new agents for quick lookup
	availableAgents := make(map[string]bool)
	for _, agent := range newAgents {
		availableAgents[agent.name] = true
	}

	// Parse comma-separated selection
	names := strings.Split(selectArg, ",")
	var addedNames []string

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Check if it's in the available new agents
		if !availableAgents[name] {
			// Check if it's already in config
			if _, exists := existingCfg.Targets[name]; exists {
				fmt.Printf("  %s is already set up (skipped)\n", name)
			} else if _, ok := defaultTargets[name]; !ok {
				ui.Warning("Unknown AI tool: %s (skipped)", name)
			} else {
				ui.Warning("%s was not found on this machine (skipped)", name)
			}
			continue
		}

		// Add to config
		if target, ok := defaultTargets[name]; ok {
			target.EnsureSkills().Mode = modeOverrideForTarget(mode, existingCfg.Mode)
			existingCfg.Targets[name] = target
			addedNames = append(addedNames, name)
		}
	}

	if len(addedNames) == 0 {
		fmt.Println("No AI tools added")
		return nil
	}

	if !dryRun {
		if err := existingCfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	printDiscoverResult(config.ConfigPath(), addedNames, false, dryRun, "skillshare sync")
	return nil
}
