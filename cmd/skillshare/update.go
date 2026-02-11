package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdUpdate(args []string) error {
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
		err := cmdUpdateProject(rest, cwd)
		logUpdateOp(config.ProjectConfigPath(cwd), rest, start, err)
		return err
	}

	var name string
	var updateAll bool
	var dryRun bool
	var force bool

	// Parse arguments
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		switch {
		case arg == "--all" || arg == "-a":
			updateAll = true
		case arg == "--dry-run" || arg == "-n":
			dryRun = true
		case arg == "--force" || arg == "-f":
			force = true
		case arg == "--help" || arg == "-h":
			printUpdateHelp()
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if name != "" {
				return fmt.Errorf("unexpected argument: %s", arg)
			}
			name = arg
		}
	}

	if name == "" && !updateAll {
		printUpdateHelp()
		return fmt.Errorf("specify a skill or repo name, or use --all")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if updateAll {
		err = updateAllTrackedRepos(cfg, dryRun, force)
		logUpdateOp(config.ConfigPath(), []string{"--all"}, start, err)
		return err
	}

	// Determine if it's a tracked repo or regular skill
	err = updateSkillOrRepo(cfg, name, dryRun, force)
	logUpdateOp(config.ConfigPath(), []string{name}, start, err)
	return err
}

func logUpdateOp(cfgPath string, args []string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("update", statusFromErr(cmdErr), time.Since(start))
	if len(args) > 0 {
		e.Args = map[string]any{"name": args[0]}
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.Write(cfgPath, oplog.OpsFile, e) //nolint:errcheck
}

// updateResult tracks the result of an update operation
type updateResult struct {
	updated int
	skipped int
}

// updateTrackedRepoQuick updates a single tracked repo (for --all mode)
func updateTrackedRepoQuick(repo, repoPath, progress string, dryRun, force bool) (updated bool, err error) {
	// Check for uncommitted changes
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
		if !force {
			ui.ListItem("warning", repo, "has uncommitted changes (use --force)")
			return false, nil
		}
		if !dryRun {
			if err := git.Restore(repoPath); err != nil {
				ui.ListItem("warning", repo, fmt.Sprintf("failed to discard changes: %v", err))
				return false, nil
			}
		}
	}

	if dryRun {
		ui.ListItem("info", repo, "[dry-run] would git pull")
		return false, nil
	}

	spinner := ui.StartSpinner(fmt.Sprintf("%s Updating %s...", progress, repo))

	var info *git.UpdateInfo
	if force {
		info, err = git.ForcePull(repoPath)
	} else {
		info, err = git.Pull(repoPath)
	}
	if err != nil {
		spinner.Warn(fmt.Sprintf("%s %v", repo, err))
		return false, nil
	}

	if info.UpToDate {
		spinner.Success(fmt.Sprintf("%s Already up to date", repo))
	} else {
		spinner.Success(fmt.Sprintf("%s %d commits, %d files", repo, len(info.Commits), info.Stats.FilesChanged))
	}
	return true, nil
}

// updateSkillFromMeta updates a skill using its metadata
func updateSkillFromMeta(skill, skillPath, progress string, dryRun bool) (updated bool) {
	if dryRun {
		ui.ListItem("info", skill, "[dry-run] would reinstall from source")
		return false
	}

	spinner := ui.StartSpinner(fmt.Sprintf("%s Updating %s...", progress, skill))

	meta, _ := install.ReadMeta(skillPath)
	source, err := install.ParseSource(meta.Source)
	if err != nil {
		spinner.Warn(fmt.Sprintf("%s invalid source: %v", skill, err))
		return false
	}

	opts := install.InstallOptions{Force: true, Update: true}
	if _, err = install.Install(source, skillPath, opts); err != nil {
		spinner.Warn(fmt.Sprintf("%s %v", skill, err))
		return false
	}

	spinner.Success(fmt.Sprintf("%s Reinstalled from source", skill))
	return true
}

func updateAllTrackedRepos(cfg *config.Config, dryRun, force bool) error {
	repos, err := install.GetTrackedRepos(cfg.Source)
	if err != nil {
		return fmt.Errorf("failed to get tracked repos: %w", err)
	}

	skills, err := install.GetUpdatableSkills(cfg.Source)
	if err != nil {
		return fmt.Errorf("failed to get updatable skills: %w", err)
	}

	if len(repos) == 0 && len(skills) == 0 {
		ui.Info("No tracked repositories or updatable skills found")
		ui.Info("Use 'skillshare install <repo> --track' to add a tracked repository")
		return nil
	}

	total := len(repos) + len(skills)
	ui.HeaderBox("skillshare update --all",
		fmt.Sprintf("Updating %d tracked repos + %d skills", len(repos), len(skills)))
	fmt.Println()

	var result updateResult

	// Update tracked repos
	for i, repo := range repos {
		repoPath := filepath.Join(cfg.Source, repo)
		progress := fmt.Sprintf("[%d/%d]", i+1, total)
		if updated, _ := updateTrackedRepoQuick(repo, repoPath, progress, dryRun, force); updated {
			result.updated++
		} else {
			result.skipped++
		}
	}

	// Update regular skills
	for i, skill := range skills {
		skillPath := filepath.Join(cfg.Source, skill)
		progress := fmt.Sprintf("[%d/%d]", len(repos)+i+1, total)
		if updateSkillFromMeta(skill, skillPath, progress, dryRun) {
			result.updated++
		} else {
			result.skipped++
		}
	}

	if !dryRun {
		fmt.Println()
		ui.Box("Summary", "",
			fmt.Sprintf("  Total:    %d", total),
			fmt.Sprintf("  Updated:  %d", result.updated),
			fmt.Sprintf("  Skipped:  %d", result.skipped), "")
	}

	if result.updated > 0 {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	return nil
}

func updateSkillOrRepo(cfg *config.Config, name string, dryRun, force bool) error {
	// Try tracked repo first (with _ prefix)
	repoName := name
	if !strings.HasPrefix(repoName, "_") {
		repoName = "_" + name
	}
	repoPath := filepath.Join(cfg.Source, repoName)

	if install.IsGitRepo(repoPath) {
		return updateTrackedRepo(cfg, repoName, dryRun, force)
	}

	// Try as regular skill (exact path)
	skillPath := filepath.Join(cfg.Source, name)
	if meta, err := install.ReadMeta(skillPath); err == nil && meta != nil {
		return updateRegularSkill(cfg, name, dryRun, force)
	}

	// Check if it's a nested path that exists as git repo
	if install.IsGitRepo(skillPath) {
		return updateTrackedRepo(cfg, name, dryRun, force)
	}

	// Fallback: search by basename in nested skills and repos
	if match, err := resolveByBasename(cfg.Source, name); err == nil {
		if match.isRepo {
			return updateTrackedRepo(cfg, match.relPath, dryRun, force)
		}
		return updateRegularSkill(cfg, match.relPath, dryRun, force)
	} else {
		return err
	}
}

type resolvedMatch struct {
	relPath string
	isRepo  bool
}

// resolveByBasename searches nested skills and tracked repos by their
// directory basename. Returns an error when zero or multiple matches found.
func resolveByBasename(sourceDir, name string) (resolvedMatch, error) {
	var matches []resolvedMatch

	// Search tracked repos
	repos, _ := install.GetTrackedRepos(sourceDir)
	for _, r := range repos {
		if filepath.Base(r) == "_"+name || filepath.Base(r) == name {
			matches = append(matches, resolvedMatch{relPath: r, isRepo: true})
		}
	}

	// Search updatable skills
	skills, _ := install.GetUpdatableSkills(sourceDir)
	for _, s := range skills {
		if filepath.Base(s) == name {
			matches = append(matches, resolvedMatch{relPath: s, isRepo: false})
		}
	}

	if len(matches) == 0 {
		return resolvedMatch{}, fmt.Errorf("'%s' not found as tracked repo or skill with metadata", name)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	// Ambiguous: list all matches
	lines := []string{fmt.Sprintf("'%s' matches multiple items:", name)}
	for _, m := range matches {
		lines = append(lines, fmt.Sprintf("  - %s", m.relPath))
	}
	lines = append(lines, "Please specify the full path")
	return resolvedMatch{}, fmt.Errorf("%s", strings.Join(lines, "\n"))
}

func updateTrackedRepo(cfg *config.Config, repoName string, dryRun, force bool) error {
	repoPath := filepath.Join(cfg.Source, repoName)

	// Header box
	ui.HeaderBox("skillshare update", fmt.Sprintf("Updating: %s", repoName))
	fmt.Println()

	// Check for uncommitted changes
	spinner := ui.StartSpinner("Checking repository status...")

	isDirty, _ := git.IsDirty(repoPath)
	if isDirty {
		spinner.Stop()
		files, _ := git.GetDirtyFiles(repoPath)

		if !force {
			lines := []string{
				"",
				"Repository has uncommitted changes:",
				"",
			}
			lines = append(lines, files...)
			lines = append(lines, "", "Use --force to discard changes and update", "")

			ui.WarningBox("Warning", lines...)
			fmt.Println()
			ui.ErrorMsg("Update aborted")
			return fmt.Errorf("uncommitted changes in repository")
		}

		ui.Warning("Discarding local changes (--force)")
		if !dryRun {
			if err := git.Restore(repoPath); err != nil {
				return fmt.Errorf("failed to discard changes: %w", err)
			}
		}
		spinner = ui.StartSpinner("Fetching from origin...")
	}

	if dryRun {
		spinner.Stop()
		ui.Warning("[dry-run] Would run: git pull")
		return nil
	}

	spinner.Update("Fetching from origin...")

	// Use ForcePull if --force to handle force push
	var info *git.UpdateInfo
	var err error
	if force {
		info, err = git.ForcePull(repoPath)
	} else {
		info, err = git.Pull(repoPath)
	}
	if err != nil {
		spinner.Fail("Failed to update")
		return fmt.Errorf("git pull failed: %w", err)
	}

	if info.UpToDate {
		spinner.Success("Already up to date")
		return nil
	}

	spinner.Stop()
	fmt.Println()

	// Show changes box
	lines := []string{
		"",
		fmt.Sprintf("  Commits:  %d new", len(info.Commits)),
		fmt.Sprintf("  Files:    %d changed (+%d / -%d)",
			info.Stats.FilesChanged, info.Stats.Insertions, info.Stats.Deletions),
		"",
	}

	// Show up to 5 commits
	maxCommits := 5
	for i, c := range info.Commits {
		if i >= maxCommits {
			lines = append(lines, fmt.Sprintf("  ... and %d more", len(info.Commits)-maxCommits))
			break
		}
		lines = append(lines, fmt.Sprintf("  %s  %s", c.Hash, truncateString(c.Message, 40)))
	}
	lines = append(lines, "")

	ui.Box("Changes", lines...)
	fmt.Println()

	ui.SuccessMsg("Updated %s", repoName)
	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute changes")

	return nil
}

func updateRegularSkill(cfg *config.Config, skillName string, dryRun, force bool) error {
	skillPath := filepath.Join(cfg.Source, skillName)

	// Read metadata to get source
	meta, err := install.ReadMeta(skillPath)
	if err != nil {
		return fmt.Errorf("cannot read metadata for '%s': %w", skillName, err)
	}
	if meta == nil || meta.Source == "" {
		return fmt.Errorf("skill '%s' has no source metadata, cannot update", skillName)
	}

	// Header box
	ui.HeaderBox("skillshare update",
		fmt.Sprintf("Updating: %s\nSource: %s", skillName, meta.Source))
	fmt.Println()

	if dryRun {
		ui.Warning("[dry-run] Would reinstall from: %s", meta.Source)
		return nil
	}

	// Parse source and reinstall
	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return fmt.Errorf("invalid source in metadata: %w", err)
	}

	spinner := ui.StartSpinner("Cloning source repository...")

	opts := install.InstallOptions{
		Force:  true,
		Update: true,
	}

	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		spinner.Fail("Failed to update")
		return fmt.Errorf("update failed: %w", err)
	}

	spinner.Success(fmt.Sprintf("Updated %s", skillName))

	for _, warning := range result.Warnings {
		ui.Warning("%s", warning)
	}

	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute changes")

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func printUpdateHelp() {
	fmt.Println(`Usage: skillshare update <name> [options]
       skillshare update --all [options]

Update a skill or tracked repository.

For tracked repos (_repo-name): runs git pull
For regular skills: reinstalls from stored source metadata

Safety: Tracked repos with uncommitted changes are skipped by default.
Use --force to discard local changes and update.

Arguments:
  name                Skill name or tracked repo name

Options:
  --all, -a           Update all tracked repos + skills with metadata
  --force, -f         Discard local changes and force update
  --dry-run, -n       Preview without making changes
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare update my-skill              # Update regular skill from source
  skillshare update _team-skills          # Update tracked repo (git pull)
  skillshare update team-skills           # _ prefix is optional for repos
  skillshare update --all                 # Update all tracked repos + skills
  skillshare update --all --dry-run       # Preview updates
  skillshare update _team --force         # Discard changes and update`)
}
