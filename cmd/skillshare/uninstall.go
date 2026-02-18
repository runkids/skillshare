package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

// uninstallOptions holds parsed arguments for uninstall command
type uninstallOptions struct {
	skillNames []string // positional args (0+)
	groups     []string // --group/-G values (repeatable)
	force      bool
	dryRun     bool
}

// uninstallTarget holds resolved target information
type uninstallTarget struct {
	name          string
	path          string
	isTrackedRepo bool
}

// parseUninstallArgs parses command line arguments
func parseUninstallArgs(args []string) (*uninstallOptions, bool, error) {
	opts := &uninstallOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--force" || arg == "-f":
			opts.force = true
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--group" || arg == "-G":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("--group requires a value")
			}
			opts.groups = append(opts.groups, args[i])
		case arg == "--help" || arg == "-h":
			return nil, true, nil // showHelp = true
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			opts.skillNames = append(opts.skillNames, arg)
		}
	}

	if len(opts.skillNames) == 0 && len(opts.groups) == 0 {
		return nil, true, fmt.Errorf("skill name or --group is required")
	}

	return opts, false, nil
}

// resolveUninstallTarget resolves skill name to path and checks existence.
// Supports short names for nested skills (e.g. "react-best-practices" resolves
// to "frontend/react/react-best-practices").
func resolveUninstallTarget(skillName string, cfg *config.Config) (*uninstallTarget, error) {
	// Normalize _ prefix for tracked repos
	if !strings.HasPrefix(skillName, "_") {
		prefixedPath := filepath.Join(cfg.Source, "_"+skillName)
		if install.IsGitRepo(prefixedPath) {
			skillName = "_" + skillName
		}
	}

	skillPath := filepath.Join(cfg.Source, skillName)
	info, err := os.Stat(skillPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Fallback: search by basename in nested directories
			resolved, resolveErr := resolveNestedSkillDir(cfg.Source, skillName)
			if resolveErr != nil {
				return nil, resolveErr
			}
			skillName = resolved
			skillPath = filepath.Join(cfg.Source, resolved)
		} else {
			return nil, fmt.Errorf("cannot access skill: %w", err)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("'%s' is not a directory", skillName)
	}

	return &uninstallTarget{
		name:          skillName,
		path:          skillPath,
		isTrackedRepo: install.IsGitRepo(skillPath),
	}, nil
}

// resolveGroupSkills finds all skills under a group directory (prefix match).
// Returns uninstallTargets for each skill found.
func resolveGroupSkills(group, sourceDir string) ([]*uninstallTarget, error) {
	group = strings.TrimSuffix(group, "/")
	groupPath := filepath.Join(sourceDir, group)

	info, err := os.Stat(groupPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("group '%s' not found in source", group)
	}

	var targets []*uninstallTarget
	filepath.Walk(groupPath, func(path string, fi os.FileInfo, err error) error { //nolint:errcheck
		if err != nil || path == groupPath || !fi.IsDir() {
			return nil
		}
		if fi.Name() == ".git" {
			return filepath.SkipDir
		}

		// Check if this directory is a skill (has SKILL.md) or tracked repo
		hasSkillMD := false
		if _, statErr := os.Stat(filepath.Join(path, "SKILL.md")); statErr == nil {
			hasSkillMD = true
		}
		isRepo := install.IsGitRepo(path)

		if hasSkillMD || isRepo {
			rel, relErr := filepath.Rel(sourceDir, path)
			if relErr == nil {
				targets = append(targets, &uninstallTarget{
					name:          rel,
					path:          path,
					isTrackedRepo: isRepo,
				})
			}
			return filepath.SkipDir // don't descend into skill dirs
		}
		return nil
	})

	if len(targets) == 0 {
		return nil, fmt.Errorf("no skills found in group '%s'", group)
	}

	return targets, nil
}

// resolveNestedSkillDir searches for a skill directory by basename within
// nested organizational folders. Also matches _name variant for tracked repos.
// Returns the relative path from sourceDir, or an error listing all matches
// when the name is ambiguous.
func resolveNestedSkillDir(sourceDir, name string) (string, error) {
	var matches []string

	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil || path == sourceDir || !info.IsDir() {
			return nil
		}
		if info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.Name() == name || info.Name() == "_"+name {
			if rel, relErr := filepath.Rel(sourceDir, path); relErr == nil && rel != "." {
				matches = append(matches, rel)
			}
			return filepath.SkipDir
		}
		return nil
	})

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("skill '%s' not found in source", name)
	case 1:
		return matches[0], nil
	default:
		lines := []string{fmt.Sprintf("'%s' matches multiple skills:", name)}
		for _, m := range matches {
			lines = append(lines, fmt.Sprintf("  - %s", m))
		}
		lines = append(lines, "Please specify the full path")
		return "", fmt.Errorf("%s", strings.Join(lines, "\n"))
	}
}

// displayUninstallInfo shows information about the skill to be uninstalled
func displayUninstallInfo(target *uninstallTarget) {
	if target.isTrackedRepo {
		ui.Header("Uninstalling tracked repository")
		ui.Info("Type: tracked repository")
	} else {
		ui.Header("Uninstalling skill")
		if meta, err := install.ReadMeta(target.path); err == nil && meta != nil {
			ui.Info("Source: %s", meta.Source)
			ui.Info("Installed: %s", meta.InstalledAt.Format("2006-01-02 15:04"))
		}
	}
	ui.Info("Name: %s", target.name)
	ui.Info("Path: %s", target.path)
	fmt.Println()
}

// checkTrackedRepoStatus checks for uncommitted changes in tracked repos
func checkTrackedRepoStatus(target *uninstallTarget, force bool) error {
	if !target.isTrackedRepo {
		return nil
	}

	isDirty, err := isRepoDirty(target.path)
	if err != nil {
		ui.Warning("Could not check git status: %v", err)
		return nil
	}

	if !isDirty {
		return nil
	}

	if !force {
		ui.Error("Repository has uncommitted changes!")
		ui.Info("Use --force to uninstall anyway, or commit/stash your changes first")
		return fmt.Errorf("uncommitted changes detected, use --force to override")
	}

	ui.Warning("Repository has uncommitted changes (proceeding with --force)")
	return nil
}

// confirmUninstall prompts user for confirmation
func confirmUninstall(target *uninstallTarget) (bool, error) {
	prompt := "Are you sure you want to uninstall this skill?"
	if target.isTrackedRepo {
		prompt = "Are you sure you want to uninstall this tracked repository?"
	}

	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

// performUninstall moves the skill to trash and cleans up
func performUninstall(target *uninstallTarget, cfg *config.Config) error {
	// Read metadata before moving (for reinstall hint)
	meta, _ := install.ReadMeta(target.path)

	// For tracked repos, clean up .gitignore
	if target.isTrackedRepo {
		if removed, err := install.RemoveFromGitIgnore(cfg.Source, target.name); err != nil {
			ui.Warning("Could not update .gitignore: %v", err)
		} else if removed {
			ui.Info("Removed %s from .gitignore", target.name)
		}
	}

	trashPath, err := trash.MoveToTrash(target.path, target.name, trash.TrashDir())
	if err != nil {
		return fmt.Errorf("failed to move to trash: %w", err)
	}

	if target.isTrackedRepo {
		ui.Success("Uninstalled tracked repository: %s", target.name)
	} else {
		ui.Success("Uninstalled: %s", target.name)
	}
	ui.Info("Moved to trash (7 days): %s", trashPath)
	if meta != nil && meta.Source != "" {
		ui.Info("Reinstall: skillshare install %s", meta.Source)
	}
	fmt.Println()
	ui.Info("Run 'skillshare sync' to update all targets")

	// Opportunistic cleanup of expired trash items
	if n, _ := trash.Cleanup(trash.TrashDir(), 0); n > 0 {
		ui.Info("Cleaned up %d expired trash item(s)", n)
	}

	return nil
}

func cmdUninstall(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	if mode == modeProject {
		err := cmdUninstallProject(rest, cwd)
		logUninstallOp(config.ProjectConfigPath(cwd), rest, start, err)
		return err
	}

	opts, showHelp, parseErr := parseUninstallArgs(rest)
	if showHelp {
		printUninstallHelp()
		return parseErr
	}
	if parseErr != nil {
		return parseErr
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// --- Phase 1: RESOLVE ---
	var targets []*uninstallTarget
	seen := map[string]bool{} // dedup by path
	var resolveWarnings []string

	for _, name := range opts.skillNames {
		t, err := resolveUninstallTarget(name, cfg)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !seen[t.path] {
			seen[t.path] = true
			targets = append(targets, t)
		}
	}

	for _, group := range opts.groups {
		groupTargets, err := resolveGroupSkills(group, cfg.Source)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: %v", group, err))
			continue
		}
		for _, t := range groupTargets {
			if !seen[t.path] {
				seen[t.path] = true
				targets = append(targets, t)
			}
		}
	}

	for _, w := range resolveWarnings {
		ui.Warning("%s", w)
	}

	// --- Phase 2: VALIDATE ---
	if len(targets) == 0 {
		if len(resolveWarnings) > 0 {
			return fmt.Errorf("no valid skills to uninstall")
		}
		return fmt.Errorf("no skills found")
	}

	// --- Phase 3: DISPLAY ---
	single := len(targets) == 1
	if single {
		displayUninstallInfo(targets[0])
	} else {
		ui.Header(fmt.Sprintf("Uninstalling %d skill(s)", len(targets)))
		for _, t := range targets {
			label := t.name
			if t.isTrackedRepo {
				label += " (tracked)"
			}
			fmt.Printf("  - %s\n", label)
		}
		fmt.Println()
	}

	// --- Phase 4: PRE-FLIGHT ---
	if !opts.dryRun {
		var preflight []*uninstallTarget
		for _, t := range targets {
			if err := checkTrackedRepoStatus(t, opts.force); err != nil {
				if single {
					return err
				}
				ui.Warning("Skipping %s: %v", t.name, err)
				continue
			}
			preflight = append(preflight, t)
		}
		targets = preflight

		if len(targets) == 0 {
			return fmt.Errorf("no skills to uninstall after pre-flight checks")
		}
	}

	// --- Phase 5: DRY-RUN or CONFIRM ---
	if opts.dryRun {
		for _, t := range targets {
			ui.Warning("[dry-run] would move to trash: %s", t.path)
			if t.isTrackedRepo {
				ui.Warning("[dry-run] would remove %s from .gitignore", t.name)
			}
			if meta, err := install.ReadMeta(t.path); err == nil && meta != nil && meta.Source != "" {
				ui.Info("[dry-run] Reinstall: skillshare install %s", meta.Source)
			}
		}
		return nil
	}

	if !opts.force {
		if single {
			confirmed, err := confirmUninstall(targets[0])
			if err != nil {
				return err
			}
			if !confirmed {
				ui.Info("Cancelled")
				return nil
			}
		} else {
			fmt.Printf("Uninstall %d skill(s)? [y/N]: ", len(targets))
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				ui.Info("Cancelled")
				return nil
			}
		}
	}

	// --- Phase 6: EXECUTE ---
	var succeeded []*uninstallTarget
	var failed []string
	for _, t := range targets {
		if err := performUninstall(t, cfg); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
			ui.Warning("Failed to uninstall %s: %v", t.name, err)
		} else {
			succeeded = append(succeeded, t)
		}
	}

	// --- Phase 7: FINALIZE ---
	// Batch-remove succeeded skills from config
	if len(succeeded) > 0 && len(cfg.Skills) > 0 {
		removedNames := map[string]bool{}
		for _, t := range succeeded {
			removedNames[t.name] = true
		}
		updated := make([]config.SkillEntry, 0, len(cfg.Skills))
		for _, s := range cfg.Skills {
			if !removedNames[s.FullName()] {
				updated = append(updated, s)
			}
		}
		if len(updated) != len(cfg.Skills) {
			cfg.Skills = updated
			if saveErr := cfg.Save(); saveErr != nil {
				ui.Warning("Failed to update config after uninstall: %v", saveErr)
			}
		}
	}

	// Build names list for oplog
	var opNames []string
	for _, name := range opts.skillNames {
		opNames = append(opNames, name)
	}
	for _, g := range opts.groups {
		opNames = append(opNames, "--group="+g)
	}

	var finalErr error
	if len(failed) > 0 {
		if len(succeeded) == 0 {
			finalErr = fmt.Errorf("all uninstalls failed")
		}
		// Partial failure: report but exit success (skip & continue)
	}

	logUninstallOp(config.ConfigPath(), opNames, start, finalErr)
	return finalErr
}

func logUninstallOp(cfgPath string, names []string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("uninstall", statusFromErr(cmdErr), time.Since(start))
	if len(names) == 1 {
		e.Args = map[string]any{"name": names[0]}
	} else if len(names) > 1 {
		e.Args = map[string]any{"names": names}
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.Write(cfgPath, oplog.OpsFile, e) //nolint:errcheck
}

// isRepoDirty checks if a git repository has uncommitted changes
func isRepoDirty(repoPath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

func printUninstallHelp() {
	fmt.Println(`Usage: skillshare uninstall <name>... [options]
       skillshare uninstall --group <group> [options]

Remove one or more skills or tracked repositories from the source directory.
Skills are moved to trash and kept for 7 days before automatic cleanup.
If the skill was installed from a remote source, a reinstall command is shown.

For tracked repositories (_repo-name):
  - Checks for uncommitted changes (requires --force to override)
  - Automatically removes the entry from .gitignore
  - The _ prefix is optional (automatically detected)

Options:
  --group, -G <name>  Remove all skills in a group (prefix match, repeatable)
  --force, -f         Skip confirmation and ignore uncommitted changes
  --dry-run, -n       Preview without making changes
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare uninstall my-skill              # Remove a single skill
  skillshare uninstall a b c --force         # Remove multiple skills at once
  skillshare uninstall --group frontend      # Remove all skills in frontend/
  skillshare uninstall --group frontend -n   # Preview group removal
  skillshare uninstall x -G backend --force  # Mix names and groups
  skillshare uninstall _team-repo            # Remove tracked repository
  skillshare uninstall team-repo             # _ prefix is optional`)
}
