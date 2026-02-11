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
	skillName string
	force     bool
	dryRun    bool
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

	for _, arg := range args {
		switch {
		case arg == "--force" || arg == "-f":
			opts.force = true
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--help" || arg == "-h":
			return nil, true, nil // showHelp = true
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			if opts.skillName != "" {
				return nil, false, fmt.Errorf("unexpected argument: %s", arg)
			}
			opts.skillName = arg
		}
	}

	if opts.skillName == "" {
		return nil, true, fmt.Errorf("skill name is required")
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

	target, err := resolveUninstallTarget(opts.skillName, cfg)
	if err != nil {
		return err
	}

	displayUninstallInfo(target)

	// Check for uncommitted changes (skip in dry-run)
	if !opts.dryRun {
		if err := checkTrackedRepoStatus(target, opts.force); err != nil {
			return err
		}
	}

	// Handle dry-run
	if opts.dryRun {
		ui.Warning("[dry-run] would move to trash: %s", target.path)
		if target.isTrackedRepo {
			ui.Warning("[dry-run] would remove %s from .gitignore", target.name)
		}
		if meta, err := install.ReadMeta(target.path); err == nil && meta != nil && meta.Source != "" {
			ui.Info("[dry-run] Reinstall: skillshare install %s", meta.Source)
		}
		return nil
	}

	// Confirm unless --force
	if !opts.force {
		confirmed, err := confirmUninstall(target)
		if err != nil {
			return err
		}
		if !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	err = performUninstall(target, cfg)
	logUninstallOp(config.ConfigPath(), []string{opts.skillName}, start, err)
	return err
}

func logUninstallOp(cfgPath string, args []string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("uninstall", statusFromErr(cmdErr), time.Since(start))
	if len(args) > 0 {
		e.Args = map[string]any{"name": args[0]}
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
	fmt.Println(`Usage: skillshare uninstall <name> [options]

Remove a skill or tracked repository from the source directory.
Skills are moved to trash and kept for 7 days before automatic cleanup.
If the skill was installed from a remote source, a reinstall command is shown.

For tracked repositories (_repo-name):
  - Checks for uncommitted changes (requires --force to override)
  - Automatically removes the entry from .gitignore
  - The _ prefix is optional (automatically detected)

Options:
  --force, -f     Skip confirmation and ignore uncommitted changes
  --dry-run, -n   Preview without making changes
  --project, -p   Use project-level config in current directory
  --global, -g    Use global config (~/.config/skillshare)
  --help, -h      Show this help

Examples:
  skillshare uninstall my-skill              # Remove a skill (moved to trash)
  skillshare uninstall my-skill --force      # Skip confirmation
  skillshare uninstall _team-repo            # Remove tracked repository
  skillshare uninstall team-repo             # _ prefix is optional
  skillshare uninstall _team-repo --force    # Force remove with uncommitted changes`)
}
