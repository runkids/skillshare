package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
	"skillshare/internal/validate"
)

func resolveSkillFromName(skillName string, cfg *config.Config) (*install.Source, error) {
	skillPath := filepath.Join(cfg.Source, skillName)

	meta, err := install.ReadMeta(skillPath)
	if err != nil {
		return nil, fmt.Errorf("skill '%s' not found or has no metadata", skillName)
	}
	if meta == nil {
		return nil, fmt.Errorf("skill '%s' has no metadata, cannot update", skillName)
	}

	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return nil, fmt.Errorf("invalid source in metadata: %w", err)
	}

	source.Name = skillName
	return source, nil
}

func cmdInstall(args []string) error {
	opts := install.InstallOptions{}
	var sourceArg string

	// Parse arguments
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--name":
			if i+1 >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			i++
			opts.Name = args[i]
		case arg == "--force" || arg == "-f":
			opts.Force = true
		case arg == "--update" || arg == "-u":
			opts.Update = true
		case arg == "--dry-run" || arg == "-n":
			opts.DryRun = true
		case arg == "--track" || arg == "-t":
			opts.Track = true
		case arg == "--help" || arg == "-h":
			printInstallHelp()
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if sourceArg != "" {
				return fmt.Errorf("unexpected argument: %s", arg)
			}
			sourceArg = arg
		}
		i++
	}

	if sourceArg == "" {
		printInstallHelp()
		return fmt.Errorf("source is required")
	}

	// Load config to get source directory
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Parse the source
	source, err := install.ParseSource(sourceArg)
	if err != nil {
		if opts.Update || opts.Force {
			resolvedSource, resolveErr := resolveSkillFromName(sourceArg, cfg)
			if resolveErr != nil {
				return fmt.Errorf("invalid source: %w", err)
			}
			source = resolvedSource
			ui.Info("Resolved '%s' from installed skill metadata", sourceArg)
		} else {
			return fmt.Errorf("invalid source: %w", err)
		}
	}

	if (opts.Update || opts.Force) && source.Raw != sourceArg {
		return handleDirectInstall(source, cfg, opts)
	}

	// Handle --track mode: install entire repo as tracked repository
	if opts.Track {
		return handleTrackedRepoInstall(source, cfg, opts)
	}

	// Handle git sources with discovery
	if source.IsGit() {
		if !source.HasSubdir() {
			// Whole repo - discover all skills
			return handleGitDiscovery(source, cfg, opts)
		}
		// Subdir specified - check for multiple skills within subdir
		return handleGitSubdirInstall(source, cfg, opts)
	}

	// Local path - direct installation
	return handleDirectInstall(source, cfg, opts)
}

func handleTrackedRepoInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) error {
	// Header box
	subtitle := fmt.Sprintf("Source: %s", source.Raw)
	if opts.Name != "" {
		subtitle += fmt.Sprintf("\nName override: _%s", opts.Name)
	}
	ui.HeaderBox("skillshare install --track", subtitle)
	fmt.Println()

	spinner := ui.StartSpinner("Cloning repository...")

	result, err := install.InstallTrackedRepo(source, cfg.Source, opts)
	if err != nil {
		spinner.Fail("Failed to install")
		return err
	}

	// Display result
	if opts.DryRun {
		spinner.Stop()
		ui.Warning("[dry-run] %s", result.Action)
	} else {
		spinner.Success(fmt.Sprintf("Installed tracked repo: %s", result.RepoName))
		fmt.Println()

		// Show info box
		lines := []string{
			"",
			fmt.Sprintf("  Path:    %s", result.RepoPath),
			fmt.Sprintf("  Skills:  %d found", result.SkillCount),
			"",
		}
		if len(result.Skills) > 0 && len(result.Skills) <= 10 {
			for _, skill := range result.Skills {
				lines = append(lines, fmt.Sprintf("    - %s", skill))
			}
			lines = append(lines, "")
		}
		ui.Box("Installed", lines...)
	}

	// Display warnings
	for _, warning := range result.Warnings {
		ui.Warning("%s", warning)
	}

	// Show next steps
	if !opts.DryRun {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute skills to all targets")
		ui.Info("Run 'skillshare update %s' to update this repo later", result.RepoName)
	}

	return nil
}

func handleGitDiscovery(source *install.Source, cfg *config.Config, opts install.InstallOptions) error {
	// Header box
	ui.HeaderBox("skillshare install", fmt.Sprintf("Source: %s", source.Raw))
	fmt.Println()

	spinner := ui.StartSpinner("Cloning repository...")

	// Discover skills
	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		spinner.Fail("Failed to clone")
		return err
	}
	defer install.CleanupDiscovery(discovery)

	if len(discovery.Skills) == 0 {
		spinner.Fail("No skills found")
		ui.Warning("No skills found in repository (no SKILL.md files)")
		return nil
	}

	spinner.Success(fmt.Sprintf("Found %d skill(s)", len(discovery.Skills)))
	fmt.Println()

	if opts.DryRun {
		// Show skill list in dry-run mode
		for _, skill := range discovery.Skills {
			ui.ListItem("info", skill.Name, skill.Path)
		}
		fmt.Println()
		ui.Warning("[dry-run] Would prompt for selection")
		return nil
	}

	// Prompt for selection
	selected, err := promptSkillSelection(discovery.Skills)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		ui.Info("No skills selected")
		return nil
	}

	// Install selected skills
	fmt.Println()

	type installResult struct {
		skill   install.SkillInfo
		success bool
		message string
	}
	results := make([]installResult, 0, len(selected))

	spinner = ui.StartSpinner(fmt.Sprintf("Installing %d skill(s)...", len(selected)))

	for i, skill := range selected {
		spinner.Update(fmt.Sprintf("Installing %s... (%d/%d)", skill.Name, i+1, len(selected)))
		destPath := filepath.Join(cfg.Source, skill.Name)

		_, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			results = append(results, installResult{skill: skill, success: false, message: err.Error()})
			continue
		}

		results = append(results, installResult{skill: skill, success: true, message: "installed"})
	}

	// Count results
	var installed, failed int
	for _, r := range results {
		if r.success {
			installed++
		} else {
			failed++
		}
	}

	if failed > 0 && installed == 0 {
		spinner.Fail(fmt.Sprintf("Failed to install %d skill(s)", failed))
	} else if failed > 0 {
		spinner.Success(fmt.Sprintf("Installed %d skill(s), %d failed", installed, failed))
	} else {
		spinner.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	}

	// Show results
	fmt.Println()
	for _, r := range results {
		if r.success {
			ui.ListItem("success", r.skill.Name, r.message)
		} else {
			ui.ListItem("error", r.skill.Name, r.message)
		}
	}

	if installed > 0 {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}

	return nil
}

func promptSkillSelection(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	// Build options list with skill name and path
	// Show "root" for skills with Path="."
	options := make([]string, len(skills))
	for i, skill := range skills {
		label := skill.Path
		if skill.Path == "." {
			label = "root"
		}
		options[i] = fmt.Sprintf("%s  (%s)", skill.Name, label)
	}

	var selectedIndices []int
	prompt := &survey.MultiSelect{
		Message:  "Select skills to install:",
		Options:  options,
		PageSize: 15,
		Help:     "Use arrow keys to navigate, space to select, enter to confirm",
	}

	err := survey.AskOne(prompt, &selectedIndices)
	if err != nil {
		return nil, nil // User cancelled (Ctrl+C)
	}

	// Map indices back to skills
	selected := make([]install.SkillInfo, len(selectedIndices))
	for i, idx := range selectedIndices {
		selected[i] = skills[idx]
	}

	return selected, nil
}

func handleGitSubdirInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) error {
	// Header box
	ui.HeaderBox("skillshare install", fmt.Sprintf("Source: %s\nSubdirectory: %s", source.Raw, source.Subdir))
	fmt.Println()

	spinner := ui.StartSpinner("Cloning repository...")

	// Discover skills in subdir
	discovery, err := install.DiscoverFromGitSubdir(source)
	if err != nil {
		spinner.Fail("Failed to clone")
		return err
	}
	defer install.CleanupDiscovery(discovery)

	// If only one skill found, install directly
	if len(discovery.Skills) == 1 {
		skill := discovery.Skills[0]
		spinner.Success(fmt.Sprintf("Found 1 skill: %s", skill.Name))
		fmt.Println()

		destPath := filepath.Join(cfg.Source, skill.Name)
		if opts.Name != "" {
			destPath = filepath.Join(cfg.Source, opts.Name)
		}

		installSpinner := ui.StartSpinner(fmt.Sprintf("Installing %s...", skill.Name))

		result, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			installSpinner.Fail("Failed to install")
			return err
		}

		if opts.DryRun {
			installSpinner.Stop()
			ui.Warning("[dry-run] %s", result.Action)
		} else {
			installSpinner.Success(fmt.Sprintf("Installed: %s", skill.Name))
		}

		for _, warning := range result.Warnings {
			ui.Warning("%s", warning)
		}

		if !opts.DryRun {
			fmt.Println()
			ui.Info("Run 'skillshare sync' to distribute to all targets")
		}
		return nil
	}

	// Multiple skills found - enter discovery mode
	if len(discovery.Skills) == 0 {
		spinner.Fail("No skills found")
		ui.Warning("No skills found in subdirectory (no SKILL.md files)")
		return nil
	}

	spinner.Success(fmt.Sprintf("Found %d skill(s) in subdirectory", len(discovery.Skills)))
	fmt.Println()

	if opts.DryRun {
		for _, skill := range discovery.Skills {
			label := skill.Path
			if skill.Path == "." {
				label = "root"
			}
			ui.ListItem("info", skill.Name, label)
		}
		fmt.Println()
		ui.Warning("[dry-run] Would prompt for selection")
		return nil
	}

	// Prompt for selection
	selected, err := promptSkillSelection(discovery.Skills)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		ui.Info("No skills selected")
		return nil
	}

	// Install selected skills
	fmt.Println()

	type installResult struct {
		skill   install.SkillInfo
		success bool
		message string
	}
	results := make([]installResult, 0, len(selected))

	spinner = ui.StartSpinner(fmt.Sprintf("Installing %d skill(s)...", len(selected)))

	for i, skill := range selected {
		spinner.Update(fmt.Sprintf("Installing %s... (%d/%d)", skill.Name, i+1, len(selected)))
		destPath := filepath.Join(cfg.Source, skill.Name)

		_, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			results = append(results, installResult{skill: skill, success: false, message: err.Error()})
			continue
		}

		results = append(results, installResult{skill: skill, success: true, message: "installed"})
	}

	// Count results
	var installed, failed int
	for _, r := range results {
		if r.success {
			installed++
		} else {
			failed++
		}
	}

	if failed > 0 && installed == 0 {
		spinner.Fail(fmt.Sprintf("Failed to install %d skill(s)", failed))
	} else if failed > 0 {
		spinner.Success(fmt.Sprintf("Installed %d skill(s), %d failed", installed, failed))
	} else {
		spinner.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	}

	// Show results
	fmt.Println()
	for _, r := range results {
		if r.success {
			ui.ListItem("success", r.skill.Name, r.message)
		} else {
			ui.ListItem("error", r.skill.Name, r.message)
		}
	}

	if installed > 0 {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}

	return nil
}

func handleDirectInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) error {
	// Determine skill name
	skillName := source.Name
	if opts.Name != "" {
		skillName = opts.Name
	}

	// Validate skill name
	if err := validate.SkillName(skillName); err != nil {
		return fmt.Errorf("invalid skill name '%s': %w", skillName, err)
	}

	// Set the name in source for display
	source.Name = skillName

	// Determine destination path
	destPath := filepath.Join(cfg.Source, skillName)

	// Header box
	subtitle := fmt.Sprintf("Source: %s\nName: %s", source.Raw, skillName)
	if source.HasSubdir() {
		subtitle += fmt.Sprintf("\nSubdirectory: %s", source.Subdir)
	}
	ui.HeaderBox("skillshare install", subtitle)
	fmt.Println()

	var spinner *ui.Spinner
	if source.IsGit() {
		spinner = ui.StartSpinner("Cloning repository...")
	} else {
		spinner = ui.StartSpinner("Copying files...")
	}

	// Execute installation
	result, err := install.Install(source, destPath, opts)
	if err != nil {
		spinner.Fail("Failed to install")
		return err
	}

	// Display result
	if opts.DryRun {
		spinner.Stop()
		ui.Warning("[dry-run] %s", result.Action)
	} else {
		spinner.Success(fmt.Sprintf("Installed: %s", skillName))
	}

	// Display warnings
	for _, warning := range result.Warnings {
		ui.Warning("%s", warning)
	}

	// Show next steps
	if !opts.DryRun {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}

	return nil
}

func printInstallHelp() {
	fmt.Println(`Usage: skillshare install <source|skill-name> [options]

Install a skill from a local path or git repository.
When using --update or --force with a skill name, skillshare uses stored metadata to resolve the source.

Sources:
  user/repo                  GitHub shorthand (expands to github.com/user/repo)
  user/repo/path/to/skill    GitHub shorthand with subdirectory
  github.com/user/repo       Full GitHub URL (discovers skills)
  github.com/user/repo/path  Subdirectory in GitHub repo (direct install)
  https://github.com/...     HTTPS git URL
  git@github.com:...         SSH git URL
  ~/path/to/skill            Local directory

Options:
  --name <name>       Override the skill name (only for direct install)
  --force, -f         Overwrite if skill already exists
  --update, -u        Update existing (git pull if possible, else reinstall)
  --track, -t         Install as tracked repo (preserves .git for updates)
  --dry-run, -n       Preview the installation without making changes
  --help, -h          Show this help

Examples:
  skillshare install anthropics/skills
  skillshare install anthropics/skills/skills/pdf
  skillshare install ComposioHQ/awesome-claude-skills
  skillshare install ~/my-skill
  skillshare install github.com/user/repo --force

Tracked repositories (Team Edition):
  skillshare install team/shared-skills --track   # Clone as _shared-skills
  skillshare install _shared-skills --update      # Update tracked repo

Update existing skills:
  skillshare install my-skill --update       # Update using stored source
  skillshare install my-skill --force        # Reinstall using stored source
  skillshare install my-skill --update -n    # Preview update`)
}
