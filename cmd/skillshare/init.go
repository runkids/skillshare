package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"
	"skillshare/internal/config"
	gitops "skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

const skillshareSkillSource = "github.com/runkids/skillshare/skills/skillshare"
const skillshareSkillURL = "https://raw.githubusercontent.com/runkids/skillshare/main/skills/skillshare/SKILL.md"
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
	initSkill    bool
	noSkill      bool
	skillFlagSet bool
	discover     bool
	selectArg    string
	mode         string
}

// parseInitArgs parses command line arguments into initOptions
// errHelp is a sentinel error indicating --help was requested.
var errHelp = fmt.Errorf("help requested")

func printInitUsage() {
	fmt.Println("Usage: skillshare init [flags]")
	fmt.Println()
	fmt.Println("Initialize skillshare — detect AI CLI tools, create config, and set up skill syncing.")
	fmt.Println()
	fmt.Println("FLAGS")
	fmt.Println("  --source, -s <path>       Set source directory (default: ~/.config/skillshare/skills)")
	fmt.Println("  --remote <url>            Set git remote for cross-machine sync (implies --git)")
	fmt.Println("  --copy-from, -c <name>    Copy existing skills from a detected CLI directory")
	fmt.Println("  --no-copy                 Start with empty source (skip copy prompt)")
	fmt.Println("  --targets, -t <list>      Comma-separated target names to add")
	fmt.Println("  --all-targets             Add all detected targets")
	fmt.Println("  --no-targets              Skip target setup")
	fmt.Println("  --mode, -m <mode>         Set sync mode (merge, copy, symlink; default: merge).")
	fmt.Println("                            With --discover, applies only to newly added targets")
	fmt.Println("  --git                     Initialize git in source (default: prompt)")
	fmt.Println("  --no-git                  Skip git initialization")
	fmt.Println("  --skill                   Install built-in skillshare skill")
	fmt.Println("  --no-skill                Skip built-in skill installation")
	fmt.Println("  --discover, -d            Detect and add new AI CLI agents to existing config")
	fmt.Println("  --select <list>           Select specific agents to add (requires --discover)")
	fmt.Println("  --dry-run, -n             Preview without making changes")
	fmt.Println("  -p, --project             Initialize project-level config in current directory")
	fmt.Println()
	fmt.Println("EXAMPLES")
	fmt.Println("  skillshare init                                    # Interactive setup")
	fmt.Println("  skillshare init --remote git@github.com:u/skills   # With cross-machine sync")
	fmt.Println("  skillshare init --no-copy --git --no-skill         # Non-interactive, minimal")
	fmt.Println("  skillshare init --discover                         # Add newly installed AI tools")
	fmt.Println("  skillshare init -p                                 # Project-level init")
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
		case "--skill":
			opts.initSkill = true
			opts.skillFlagSet = true
		case "--no-skill":
			opts.noSkill = true
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

func hasGitRepo(sourcePath string) bool {
	_, err := os.Stat(filepath.Join(sourcePath, ".git"))
	return err == nil
}

func hasBuiltinSkill(sourcePath string) bool {
	_, err := os.Stat(filepath.Join(sourcePath, "skillshare", "SKILL.md"))
	return err == nil
}

func shouldPromptInitMode(opts *initOptions, sourcePath string, withSkills, detected []detectedDir) bool {
	if opts.mode != "" || !runningInInteractiveTTY() {
		return false
	}

	willPromptCopy := !opts.noCopy && opts.copyFrom == "" && len(withSkills) > 0
	willPromptTargets := !opts.noTargets && !opts.allTargets && opts.targetsArg == "" && len(detected) > 0
	willPromptGit := !opts.noGit && !opts.initGit && !hasGitRepo(sourcePath)
	willPromptSkill := !opts.noSkill && !opts.initSkill && !hasBuiltinSkill(sourcePath)

	return willPromptCopy || willPromptTargets || willPromptGit || willPromptSkill
}

func promptSyncModeSelection() string {
	ui.Header("Sync mode preference")

	type modeOption struct {
		name string
		desc string
	}
	modes := []modeOption{
		{"merge", "per-skill symlinks, preserves local skills"},
		{"copy", "real files, recommended if unsure whether your AI CLI supports symlinks"},
		{"symlink", "entire directory linked"},
	}

	for i, m := range modes {
		fmt.Printf("  %d) %-8s — %s\n", i+1, m.name, m.desc)
	}
	fmt.Println()
	fmt.Print("  Enter choice [1]: ")

	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		ui.Success("Sync mode: merge")
		return "merge"
	}
	input = strings.TrimSpace(input)

	// Default or invalid → merge
	idx := 0
	if input != "" {
		if n := input[0] - '1'; n < byte(len(modes)) {
			idx = int(n)
		}
	}

	selected := normalizeSyncMode(modes[idx].name)
	ui.Success("Sync mode: %s", selected)
	fmt.Println()
	if selected != "copy" {
		ui.Info("If a tool doesn't support symlinks, switch to copy mode:")
		fmt.Printf("  %sskillshare config --mode copy && skillshare sync%s\n", ui.Yellow, ui.Reset)
	}
	return selected
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

	// If --remote provided, just add the remote to existing setup
	if opts.remoteURL != "" {
		cfg, err := config.Load()
		if err != nil {
			return true, err
		}
		// Ensure git is initialized before setting up remote
		// (--remote implies --git, so opts.initGit is already true)
		initGitIfNeeded(cfg.Source, opts.dryRun, opts.initGit, opts.noGit)
		// Commit any uncommitted source files so push/pull work cleanly
		if !opts.dryRun && !opts.noGit {
			if err := commitSourceFiles(cfg.Source); err != nil {
				ui.Warning("Failed to commit source files: %v", err)
			}
		}
		setupGitRemote(cfg.Source, opts.remoteURL, opts.dryRun)
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

	return true, fmt.Errorf("already initialized. Run 'skillshare init --discover' to add new agents, or 'skillshare init -p' to initialize project-level skills")
}

// performFreshInit performs a fresh initialization
func performFreshInit(opts *initOptions, home string) error {
	ui.Logo(version)

	// Detect existing CLI skills directories
	detected := detectCLIDirectories(home)

	// Default source path (same location as config)
	sourcePath := opts.sourcePath
	if sourcePath == "" {
		sourcePath = filepath.Join(config.BaseDir(), "skills")
	}

	// Find directories with skills to potentially copy from
	var withSkills []detectedDir
	for _, d := range detected {
		if d.hasSkills {
			withSkills = append(withSkills, d)
		}
	}

	// Determine copy source (non-interactive or prompt)
	copyFromPath, copyFromName := promptCopyFrom(withSkills, opts.copyFrom, opts.noCopy, home)

	if opts.dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	// Create source directory if needed
	if err := createSourceDir(sourcePath, opts.dryRun); err != nil {
		return err
	}

	// Copy skills from selected directory
	if copyFromPath != "" {
		copySkillsToSource(copyFromPath, sourcePath, opts.dryRun)
	}

	// Build targets list
	targets := buildTargetsList(detected, copyFromPath, copyFromName, opts.targetsArg, opts.allTargets, opts.noTargets)
	mode := opts.mode
	if shouldPromptInitMode(opts, sourcePath, withSkills, detected) {
		mode = promptSyncModeSelection()
	}
	if mode == "" {
		mode = "merge"
	}

	// Create config
	cfg := &config.Config{
		Source:  sourcePath,
		Mode:    mode,
		Targets: targets,
		Ignore: []string{
			"**/.DS_Store",
			"**/.git/**",
		},
		Audit: config.AuditConfig{
			BlockThreshold: "CRITICAL",
		},
	}

	if opts.dryRun {
		summarizeInitConfig(cfg)
	} else if err := cfg.Save(); err != nil {
		return err
	}

	// Initialize git in source directory for safety
	initGitIfNeeded(sourcePath, opts.dryRun, opts.initGit, opts.noGit)

	// Set up git remote for cross-machine sync
	setupGitRemote(sourcePath, opts.remoteURL, opts.dryRun)

	// Install built-in skillshare skill (opt-in)
	installSkillIfNeeded(sourcePath, opts.dryRun, opts.initSkill, opts.noSkill)

	// Single initial commit with all source files (.gitignore + skills)
	if !opts.dryRun && !opts.noGit {
		if err := commitSourceFiles(sourcePath); err != nil {
			ui.Warning("Failed to create initial commit: %v", err)
		}
	}

	// Print completion message
	skillInstalled := false
	if _, err := os.Stat(filepath.Join(sourcePath, "skillshare", "SKILL.md")); err == nil {
		skillInstalled = true
	}
	printInitSuccess(sourcePath, opts.dryRun, skillInstalled)

	return nil
}

// createSourceDir creates the source directory
func createSourceDir(sourcePath string, dryRun bool) error {
	if dryRun {
		if _, err := os.Stat(sourcePath); err == nil {
			ui.Info("Source directory exists: %s", sourcePath)
		} else {
			ui.Info("Would create source directory: %s", sourcePath)
		}
		return nil
	}

	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}
	return nil
}

// printInitSuccess prints the success message after initialization
func printInitSuccess(sourcePath string, dryRun bool, skillInstalled bool) {
	if dryRun {
		ui.Header("Dry run complete")
		ui.Info("Would write config: %s", config.ConfigPath())
		ui.Info("Run 'skillshare init' to apply these changes")
		return
	}

	ui.Header("Initialized successfully")
	ui.Success("Source: %s", sourcePath)
	ui.Success("Config: %s", config.ConfigPath())
	fmt.Println()
	ui.Info("Next steps:")
	fmt.Printf("  %sskillshare sync%s              %s# Sync to all targets%s\n", ui.Yellow, ui.Reset, ui.Gray, ui.Reset)
	if skillInstalled {
		fmt.Println()
		ui.Info("Pro tip: Let AI manage your skills!")
		fmt.Println("  \"Pull my new skill from Claude and sync to all targets\"")
		fmt.Println("  \"Show me skillshare status\"")
	}
}

// hasGlobalOnlyInitFlags returns true if args contain flags only valid for global init
// (e.g. --remote, --source). Used to skip project-mode auto-detection.
func hasGlobalOnlyInitFlags(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--remote", "--source", "-s", "--copy-from", "-c", "--no-copy",
			"--git", "--no-git", "--skill", "--no-skill":
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

	cmdErr := performFreshInit(opts, home)
	// Config is created by performFreshInit, so cfgPath is valid now
	cfgPath := config.ConfigPath()
	logInitOp(cfgPath, 0, true, opts.initGit, hasBuiltinSkill(opts.sourcePath), start, cmdErr)
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

type detectedDir struct {
	name       string
	path       string
	skillCount int
	hasSkills  bool
	exists     bool // true if skills dir exists, false if only parent exists
}

// sliceHasName returns true if any element's name matches.
func sliceHasName[T any](items []T, name string, getName func(T) string) bool {
	for _, item := range items {
		if getName(item) == name {
			return true
		}
	}
	return false
}

func detectCLIDirectories(home string) []detectedDir {
	ui.Header("Detecting CLI skills directories")
	defaultTargets := config.DefaultTargets()
	var detected []detectedDir

	for name, target := range defaultTargets {
		if info, err := os.Stat(target.Path); err == nil && info.IsDir() {
			// Skills directory exists - count skills
			entries, _ := os.ReadDir(target.Path)
			skillCount := 0
			for _, e := range entries {
				if e.IsDir() && !utils.IsHidden(e.Name()) {
					skillCount++
				}
			}
			detected = append(detected, detectedDir{
				name:       name,
				path:       target.Path,
				skillCount: skillCount,
				hasSkills:  skillCount > 0,
				exists:     true,
			})
			if skillCount > 0 {
				ui.Success("Found: %-12s %s (%d skills)", name, target.Path, skillCount)
			} else {
				ui.Info("Found: %-12s %s (empty)", name, target.Path)
			}
		} else {
			// Skills directory doesn't exist - check if parent exists (CLI installed)
			parent := filepath.Dir(target.Path)
			if _, err := os.Stat(parent); err == nil {
				detected = append(detected, detectedDir{
					name:       name,
					path:       target.Path,
					skillCount: 0,
					hasSkills:  false,
					exists:     false,
				})
				ui.Info("Found: %-12s %s (not initialized)", name, target.Path)
			}
		}
	}

	// Auto-include universal target when any CLI is detected.
	// The universal path (~/.agents/skills/) is the cross-tool shared directory
	// used by vercel-labs/skills (npx skills list). It won't exist on disk until
	// we create it, so normal directory detection won't find it.
	if len(detected) > 0 && !sliceHasName(detected, "universal", func(d detectedDir) string { return d.name }) {
		if target, ok := defaultTargets["universal"]; ok {
			detected = append(detected, detectedDir{
				name: "universal", path: target.Path,
			})
			ui.Info("Found: %-12s %s (shared agent directory)", "universal", target.Path)
		}
	}

	return detected
}

func promptCopyFrom(withSkills []detectedDir, copyFromArg string, noCopy bool, home string) (copyFrom, copyFromName string) {
	// Non-interactive: --no-copy
	if noCopy {
		ui.Info("Starting with empty source (--no-copy)")
		return "", ""
	}

	// Non-interactive: --copy-from
	if copyFromArg != "" {
		// First, try to match by name (e.g., "claude", "cursor")
		for _, d := range withSkills {
			if strings.EqualFold(d.name, copyFromArg) {
				ui.Success("Will copy skills from %s (matched by name)", d.name)
				return d.path, d.name
			}
		}

		// Treat as path
		path := copyFromArg
		if utils.HasTildePrefix(path) {
			path = filepath.Join(home, path[1:])
		}

		// Verify path exists
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			ui.Success("Will copy skills from %s", path)
			return path, ""
		}

		ui.Warning("Copy source not found: %s", copyFromArg)
		return "", ""
	}

	// Interactive mode
	if len(withSkills) == 0 {
		return "", ""
	}

	ui.Header("Initialize from existing skills?")
	fmt.Println("  Copy skills from an existing directory to the shared source?")
	fmt.Println()

	for i, d := range withSkills {
		fmt.Printf("  [%d] Copy from %s (%d skills)\n", i+1, d.name, d.skillCount)
	}
	fmt.Printf("  [%d] Start fresh (empty source)\n", len(withSkills)+1)
	fmt.Println()

	fmt.Print("  Enter choice [1]: ")
	var input string
	fmt.Scanln(&input)

	choice := 1
	if input != "" {
		fmt.Sscanf(input, "%d", &choice)
	}

	if choice >= 1 && choice <= len(withSkills) {
		copyFrom = withSkills[choice-1].path
		copyFromName = withSkills[choice-1].name
		ui.Success("Will copy skills from %s", copyFromName)
	} else {
		ui.Info("Starting with empty source")
	}

	return copyFrom, copyFromName
}

func copySkillsToSource(copyFrom, sourcePath string, dryRun bool) {
	entries, err := os.ReadDir(copyFrom)
	if err != nil {
		ui.Warning("Failed to read %s: %v", copyFrom, err)
		return
	}

	if dryRun {
		copyCount := 0
		for _, entry := range entries {
			if entry.IsDir() && !utils.IsHidden(entry.Name()) {
				copyCount++
			}
		}
		ui.Info("Would copy %d skills to %s", copyCount, sourcePath)
		return
	}

	ui.Info("Copying skills to %s...", sourcePath)
	copied := 0
	for _, entry := range entries {
		if !entry.IsDir() || utils.IsHidden(entry.Name()) {
			continue
		}
		srcPath := filepath.Join(copyFrom, entry.Name())
		dstPath := filepath.Join(sourcePath, entry.Name())

		// Skip if already exists
		if _, err := os.Stat(dstPath); err == nil {
			continue
		}

		// Copy directory
		if err := copyDir(srcPath, dstPath); err != nil {
			ui.Warning("Failed to copy %s: %v", entry.Name(), err)
			continue
		}
		copied++
	}
	ui.Success("Copied %d skills to source", copied)
}

func buildTargetsList(detected []detectedDir, copyFrom, copyFromName, targetsArg string, allTargets, noTargets bool) map[string]config.TargetConfig {
	defaultTargets := config.DefaultTargets()
	targets := make(map[string]config.TargetConfig)

	// Non-interactive: --no-targets
	if noTargets {
		ui.Info("Skipping targets (--no-targets)")
		return targets
	}

	// Non-interactive: --all-targets
	if allTargets {
		for _, d := range detected {
			targets[d.name] = defaultTargets[d.name]
		}
		if len(targets) > 0 {
			ui.Success("Added all %d detected targets (--all-targets)", len(targets))
		} else {
			ui.Warning("No CLI skills directories detected")
		}
		return targets
	}

	// Non-interactive: --targets (comma-separated list)
	if targetsArg != "" {
		names := strings.Split(targetsArg, ",")
		added := 0
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}

			// Check if it's a known target name
			if target, ok := defaultTargets[name]; ok {
				targets[name] = target
				added++
			} else {
				ui.Warning("Unknown target: %s (skipped)", name)
			}
		}
		if added > 0 {
			ui.Success("Added %d targets from --targets", added)
		}
		return targets
	}

	// Interactive mode: Build multi-select items from detected directories
	if len(detected) == 0 {
		ui.Warning("No CLI skills directories detected.")
		return targets
	}

	// Build checklist items from detected directories.
	items := make([]checklistItemData, len(detected))
	for i, d := range detected {
		status := ""
		if d.name == "universal" {
			status = "(shared agent directory)"
		} else if d.exists {
			if d.skillCount > 0 {
				status = fmt.Sprintf("(%d skills)", d.skillCount)
			} else {
				status = "(empty)"
			}
		} else {
			status = "(not initialized)"
		}
		items[i] = checklistItemData{
			label:       fmt.Sprintf("%-14s %s  %s", d.name, d.path, status),
			preSelected: d.name == copyFromName,
		}
	}

	selectedIndices, err := runChecklistTUI(checklistConfig{
		title:    "Select targets to sync",
		items:    items,
		itemName: "target",
	})
	if err != nil || selectedIndices == nil {
		return targets // User cancelled
	}

	// Add selected targets
	for _, idx := range selectedIndices {
		name := detected[idx].name
		targets[name] = defaultTargets[name]
	}

	if len(targets) > 0 {
		ui.Success("Added %d target(s): %s", len(targets), joinTargetNames(targets))
	} else {
		ui.Info("No targets selected")
	}

	return targets
}

func joinTargetNames(targets map[string]config.TargetConfig) string {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func summarizeInitConfig(cfg *config.Config) {
	ui.Header("Planned configuration")
	ui.Info("Source: %s", cfg.Source)

	if len(cfg.Targets) == 0 {
		ui.Info("Targets: none")
		return
	}

	ui.Info("Targets: %d", len(cfg.Targets))
	for name, target := range cfg.Targets {
		mode := target.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}
		fmt.Printf("  %-12s %s (%s)\n", name, target.Path, mode)
	}
}

func initGitIfNeeded(sourcePath string, dryRun, initGit, noGit bool) {
	// Non-interactive: --no-git
	if noGit {
		ui.Info("Skipped git initialization (--no-git)")
		ui.Warning("Without git, deleted skills cannot be recovered!")
		return
	}

	gitDir := filepath.Join(sourcePath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		ui.Info("Git already initialized in source directory")
		return
	}

	// Non-interactive: --git flag was set, proceed without prompting
	if initGit {
		if dryRun {
			ui.Info("Dry run - would initialize git in %s (--git)", sourcePath)
			return
		}
		doGitInit(sourcePath)
		return
	}

	// Interactive mode
	ui.Header("Git version control")
	fmt.Println("  Git helps protect your skills from accidental deletion.")
	fmt.Println()
	fmt.Print("  Initialize git in source directory? [Y/n]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))

	if input != "" && input != "y" && input != "yes" {
		if dryRun {
			ui.Info("Dry run - skipped git initialization")
			return
		}
		ui.Info("Skipped git initialization")
		ui.Warning("Without git, deleted skills cannot be recovered!")
		return
	}

	if dryRun {
		ui.Info("Dry run - would initialize git in %s", sourcePath)
		return
	}

	doGitInit(sourcePath)
}

// ensureGitIdentity sets repo-local user.name/email if not configured globally.
func ensureGitIdentity(repoDir string) {
	// Check if user.name is already set (global or local)
	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = repoDir
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return // already configured
	}

	// Set repo-local fallback identity so git commit works
	nameCmd := exec.Command("git", "config", "user.name", "skillshare")
	nameCmd.Dir = repoDir
	nameCmd.Run()

	emailCmd := exec.Command("git", "config", "user.email", "skillshare@local")
	emailCmd.Dir = repoDir
	emailCmd.Run()

	ui.Info("Git identity not configured, using local default")
	ui.Info("  Set yours: git config --global user.name \"Your Name\"")
	ui.Info("             git config --global user.email \"you@example.com\"")
}

func doGitInit(sourcePath string) {
	// Run git init
	cmd := exec.Command("git", "init")
	cmd.Dir = sourcePath
	if err := cmd.Run(); err != nil {
		ui.Warning("Failed to initialize git: %v", err)
		return
	}

	// Create .gitignore
	gitignore := filepath.Join(sourcePath, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		os.WriteFile(gitignore, []byte(".DS_Store\n"), 0644)
	}

	// Ensure git identity is configured (needed for commit).
	ensureGitIdentity(sourcePath)

	ui.Success("Git initialized")
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
	commitCmd.Env = append(os.Environ(), "LC_ALL=C")
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

func setupGitRemote(sourcePath, remoteURL string, dryRun bool) {
	// Check if git is initialized
	gitDir := filepath.Join(sourcePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if remoteURL != "" {
			ui.Warning("Git not initialized in source directory")
			ui.Info("Run: cd %s && git init", sourcePath)
		}
		return
	}

	// Check if remote already exists
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = sourcePath
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		existingRemote := strings.TrimSpace(string(output))
		if existingRemote == remoteURL {
			ui.Info("Git remote already configured: %s", existingRemote)
		} else {
			ui.Warning("Git remote already exists: %s", existingRemote)
			ui.Info("To change: git remote set-url origin %s", remoteURL)
		}
		return
	}

	// If --remote flag provided, use it directly
	if remoteURL != "" {
		if dryRun {
			ui.Info("Would add git remote: %s", remoteURL)
			return
		}
		addRemote(sourcePath, remoteURL)
		return
	}

	// Prompt user
	ui.Header("Cross-machine sync")
	fmt.Println("  Set up a git remote to sync skills across machines.")
	fmt.Println()
	fmt.Print("  Set up git remote? [y/N]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))

	if input != "y" && input != "yes" {
		ui.Info("Skipped remote setup")
		ui.Info("Add later: git remote add origin <url>")
		return
	}

	fmt.Print("  Remote URL (e.g., git@github.com:user/skills.git): ")
	fmt.Scanln(&remoteURL)
	remoteURL = strings.TrimSpace(remoteURL)

	if remoteURL == "" {
		ui.Info("No URL provided, skipped remote setup")
		return
	}

	if dryRun {
		ui.Info("Would add git remote: %s", remoteURL)
		return
	}

	addRemote(sourcePath, remoteURL)
}

func addRemote(sourcePath, remoteURL string) {
	cmd := exec.Command("git", "remote", "add", "origin", remoteURL)
	cmd.Dir = sourcePath
	if err := cmd.Run(); err != nil {
		ui.Warning("Failed to add remote: %v", err)
		return
	}

	ui.Success("Git remote configured: %s", remoteURL)

	// Try to fetch and auto-pull if remote has existing skills
	if !tryPullAfterRemoteSetup(sourcePath, remoteURL) {
		ui.Info("Push your skills: skillshare push")
	}
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
		if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) {
			spinner.Warn("Remote check timed out (will retry on push/pull)")
			return false
		}
		spinner.Warn("Could not reach remote (will retry on push/pull)")
		outStr := strings.TrimSpace(string(output))
		if strings.Contains(outStr, "Could not read from remote") {
			ui.Info("  Check SSH keys: ssh -T git@github.com")
		} else if strings.Contains(outStr, "not found") || strings.Contains(outStr, "does not exist") {
			ui.Info("  Check remote URL: git remote get-url origin")
		}
		return false
	}

	remoteBranch, err := gitops.GetRemoteDefaultBranch(sourcePath)
	if err != nil {
		if errors.Is(err, gitops.ErrNoRemoteBranches) {
			spinner.Success("Remote is empty")
			return false
		}
		spinner.Warn("Could not detect remote default branch (will retry on push/pull)")
		return false
	}

	hasRemoteSkills, err := gitops.HasRemoteSkillDirs(sourcePath, remoteBranch)
	if err != nil {
		spinner.Warn("Could not inspect remote skills (will retry on push/pull)")
		return false
	}
	if !hasRemoteSkills {
		spinner.Success("Remote is empty (no skills found)")
		return false
	}

	hasLocalSkills, err := gitops.HasLocalSkillDirs(sourcePath)
	if err != nil {
		spinner.Warn("Could not inspect local skills (will retry on pull)")
		return true
	}

	if hasLocalSkills {
		spinner.Warn("Remote has existing skills, but local skills also exist")
		ui.Info("  Push local:  skillshare push")
		ui.Info("  Pull remote: skillshare pull  (replaces local with remote)")
		return true
	}

	// Local is empty — reset to remote branch for clean linear history.
	// This is safe because we verified hasLocalSkills is false above.
	spinner.Update("Pulling skills from remote...")

	resetCmd := exec.Command("git", "reset", "--hard", "origin/"+remoteBranch)
	resetCmd.Dir = sourcePath
	if output, err := resetCmd.CombinedOutput(); err != nil {
		spinner.Fail("Failed to pull from remote")
		fmt.Println(string(output))
		ui.Info("  Try manually: cd %s && git reset --hard origin/%s", sourcePath, remoteBranch)
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

	spinner.Success(fmt.Sprintf("Pulled %d skill(s) from remote", skillCount))
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

func installSkillIfNeeded(sourcePath string, dryRun, initSkill, noSkill bool) {
	// Non-interactive: --no-skill
	if noSkill {
		ui.Info("Skipped built-in skill (--no-skill)")
		return
	}

	skillshareSkillFile := filepath.Join(sourcePath, "skillshare", "SKILL.md")
	if _, err := os.Stat(skillshareSkillFile); err == nil {
		ui.Info("Built-in skill already installed")
		return
	}

	// Non-interactive: --skill flag was set, proceed without prompting
	if initSkill {
		createDefaultSkill(sourcePath, dryRun)
		return
	}

	// Interactive mode
	ui.Header("Built-in skill")
	fmt.Println("  Install the skillshare skill for AI integration?")
	fmt.Println()
	fmt.Print("  Install built-in skillshare skill? [y/N]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))

	if input == "y" || input == "yes" {
		createDefaultSkill(sourcePath, dryRun)
		return
	}

	ui.Info("Skipped built-in skill")
	ui.Info("Install later: skillshare upgrade --skill")
}

func createDefaultSkill(sourcePath string, dryRun bool) {
	skillshareSkillDir := filepath.Join(sourcePath, "skillshare")
	skillshareSkillFile := filepath.Join(skillshareSkillDir, "SKILL.md")

	if _, err := os.Stat(skillshareSkillFile); err == nil {
		return
	}

	if dryRun {
		ui.Info("Would download default skill: skillshare")
		return
	}

	ui.Header("Installing skillshare skill")

	// Use spinner for download
	spinner := ui.StartSpinner("Downloading from GitHub...")

	// Try to install from GitHub using install package
	source, err := install.ParseSource(skillshareSkillSource)
	if err == nil {
		source.Name = "skillshare"
		_, err = install.Install(source, skillshareSkillDir, install.InstallOptions{
			Force:  true,
			DryRun: false,
		})
	}

	if err != nil {
		spinner.Warn("Download failed, using fallback version")
		// Fallback to minimal version
		if err := os.MkdirAll(skillshareSkillDir, 0755); err != nil {
			ui.Warning("Failed to create skillshare skill directory: %v", err)
			return
		}
		if err := os.WriteFile(skillshareSkillFile, []byte(fallbackSkillContent), 0644); err != nil {
			ui.Warning("Failed to create skillshare skill: %v", err)
			return
		}
		ui.Success("Created default skill: skillshare (minimal)")
		ui.Info("Run 'skillshare upgrade --skill' to get the full version")
		return
	}
	spinner.Success("Downloaded default skill: skillshare")
}

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

	for name, target := range defaultTargets {
		if _, exists := existingCfg.Targets[name]; exists {
			continue
		}

		parent := filepath.Dir(target.Path)
		if _, err := os.Stat(parent); err != nil {
			continue
		}

		status := getAgentStatus(target.Path)
		newAgents = append(newAgents, agentInfo{
			name:        name,
			path:        target.Path,
			description: status,
		})
	}

	// Auto-include universal if any new agent is found but universal isn't
	// already configured and not yet in the candidate list.
	if len(newAgents) > 0 {
		if _, configured := existingCfg.Targets["universal"]; !configured {
			if !sliceHasName(newAgents, "universal", func(a agentInfo) string { return a.name }) {
				if target, ok := defaultTargets["universal"]; ok {
					newAgents = append(newAgents, agentInfo{
						name:        "universal",
						path:        target.Path,
						description: "(shared agent directory)",
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

// promptAgentSelection shows interactive selection and returns selected agent names
func promptAgentSelection(newAgents []agentInfo) ([]string, error) {
	items := make([]checklistItemData, len(newAgents))
	for i, agent := range newAgents {
		items[i] = checklistItemData{
			label: agent.name,
			desc:  fmt.Sprintf("%s %s", agent.path, agent.description),
		}
	}

	selectedIndices, err := runChecklistTUI(checklistConfig{
		title:    "Select agents to add",
		items:    items,
		itemName: "agent",
	})
	if err != nil || selectedIndices == nil {
		return nil, nil
	}

	var selectedNames []string
	for _, idx := range selectedIndices {
		selectedNames = append(selectedNames, newAgents[idx].name)
	}

	return selectedNames, nil
}

// saveAddedAgents adds agents to config and saves
func saveAddedAgents(cfg *config.Config, names []string, dryRun bool, mode string) error {
	defaultTargets := config.DefaultTargets()

	for _, name := range names {
		if target, ok := defaultTargets[name]; ok {
			target.Mode = modeOverrideForTarget(mode, cfg.Mode)
			cfg.Targets[name] = target
		}
	}

	if dryRun {
		ui.Warning("Dry run - would add %d agent(s) to config", len(names))
		for _, name := range names {
			fmt.Printf("  + %s\n", name)
		}
		return nil
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Added %d agent(s) to config", len(names))
	for _, name := range names {
		fmt.Printf("  + %s\n", name)
	}
	ui.Info("Run 'skillshare sync' to sync skills to new targets")

	return nil
}

// reinitWithDiscover detects new agents and allows user to add them to existing config
func reinitWithDiscover(existingCfg *config.Config, selectArg string, dryRun bool, mode string) error {
	ui.Header("Discovering new agents")

	newAgents := detectNewAgents(existingCfg)
	if len(newAgents) == 0 {
		ui.Info("No new agents detected")
		return nil
	}

	ui.Success("Found %d new agent(s)", len(newAgents))

	// Non-interactive mode with --select
	if selectArg != "" {
		return addSelectedAgentsByName(existingCfg, newAgents, selectArg, dryRun, mode)
	}

	// Interactive mode
	selectedNames, err := promptAgentSelection(newAgents)
	if err != nil {
		return err
	}

	if len(selectedNames) == 0 {
		ui.Info("No agents selected")
		return nil
	}

	return saveAddedAgents(existingCfg, selectedNames, dryRun, mode)
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
				ui.Info("Agent already in config: %s (skipped)", name)
			} else if _, ok := defaultTargets[name]; !ok {
				ui.Warning("Unknown agent: %s (skipped)", name)
			} else {
				ui.Warning("Agent not detected: %s (skipped)", name)
			}
			continue
		}

		// Add to config
		if target, ok := defaultTargets[name]; ok {
			target.Mode = modeOverrideForTarget(mode, existingCfg.Mode)
			existingCfg.Targets[name] = target
			addedNames = append(addedNames, name)
		}
	}

	if len(addedNames) == 0 {
		ui.Info("No new agents added")
		return nil
	}

	if dryRun {
		ui.Warning("Dry run - would add %d agent(s) to config", len(addedNames))
		for _, name := range addedNames {
			fmt.Printf("  + %s\n", name)
		}
		return nil
	}

	if err := existingCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Added %d agent(s) to config", len(addedNames))
	for _, name := range addedNames {
		fmt.Printf("  + %s\n", name)
	}
	ui.Info("Run 'skillshare sync' to sync skills to new targets")

	return nil
}
