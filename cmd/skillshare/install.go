package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
	"skillshare/internal/validate"
	appversion "skillshare/internal/version"
)

// installArgs holds parsed install command arguments
type installArgs struct {
	sourceArg string
	opts      install.InstallOptions
}

type installLogSummary struct {
	Source          string
	Mode            string
	SkillCount      int
	InstalledSkills []string
	FailedSkills    []string
	DryRun          bool
	Tracked         bool
	Into            string
}

type installBatchSummary struct {
	InstalledSkills []string
	FailedSkills    []string
}

// parseInstallArgs parses install command arguments
func parseInstallArgs(args []string) (*installArgs, bool, error) {
	result := &installArgs{}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--name":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--name requires a value")
			}
			i++
			result.opts.Name = args[i]
		case arg == "--force" || arg == "-f":
			result.opts.Force = true
		case arg == "--update" || arg == "-u":
			result.opts.Update = true
		case arg == "--dry-run" || arg == "-n":
			result.opts.DryRun = true
		case arg == "--track" || arg == "-t":
			result.opts.Track = true
		case arg == "--skill" || arg == "-s":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--skill requires a value")
			}
			i++
			result.opts.Skills = strings.Split(args[i], ",")
		case arg == "--into":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--into requires a value")
			}
			i++
			result.opts.Into = args[i]
		case arg == "--all":
			result.opts.All = true
		case arg == "--yes" || arg == "-y":
			result.opts.Yes = true
		case arg == "--help" || arg == "-h":
			return nil, true, nil // showHelp = true
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			if result.sourceArg != "" {
				return nil, false, fmt.Errorf("unexpected argument: %s", arg)
			}
			result.sourceArg = arg
		}
		i++
	}

	// Clean --skill input
	if len(result.opts.Skills) > 0 {
		cleaned := make([]string, 0, len(result.opts.Skills))
		for _, s := range result.opts.Skills {
			s = strings.TrimSpace(s)
			if s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) == 0 {
			return nil, false, fmt.Errorf("--skill requires at least one skill name")
		}
		result.opts.Skills = cleaned
	}

	// Validate mutual exclusion
	if result.opts.HasSkillFilter() && result.opts.All {
		return nil, false, fmt.Errorf("--skill and --all cannot be used together")
	}
	if result.opts.HasSkillFilter() && result.opts.Yes {
		return nil, false, fmt.Errorf("--skill and --yes cannot be used together")
	}
	if result.opts.HasSkillFilter() && result.opts.Track {
		return nil, false, fmt.Errorf("--skill cannot be used with --track")
	}
	if result.opts.ShouldInstallAll() && result.opts.Track {
		return nil, false, fmt.Errorf("--all/--yes cannot be used with --track")
	}

	if result.sourceArg == "" {
		return nil, true, fmt.Errorf("source is required")
	}

	if result.opts.Into != "" {
		if err := validate.IntoPath(result.opts.Into); err != nil {
			return nil, false, err
		}
	}

	return result, false, nil
}

// destWithInto returns the destination path, prepending opts.Into if set.
func destWithInto(sourceDir string, opts install.InstallOptions, skillName string) string {
	if opts.Into != "" {
		return filepath.Join(sourceDir, opts.Into, skillName)
	}
	return filepath.Join(sourceDir, skillName)
}

// ensureIntoDirExists creates the Into subdirectory if opts.Into is set.
func ensureIntoDirExists(sourceDir string, opts install.InstallOptions) error {
	if opts.Into == "" {
		return nil
	}
	return os.MkdirAll(filepath.Join(sourceDir, opts.Into), 0755)
}

// resolveSkillFromName resolves a skill name to source using metadata
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

// resolveInstallSource parses and resolves the install source
func resolveInstallSource(sourceArg string, opts install.InstallOptions, cfg *config.Config) (*install.Source, bool, error) {
	source, err := install.ParseSource(sourceArg)
	if err == nil {
		return source, false, nil
	}

	// Try resolving from installed skill metadata if update/force
	if opts.Update || opts.Force {
		resolvedSource, resolveErr := resolveSkillFromName(sourceArg, cfg)
		if resolveErr != nil {
			return nil, false, fmt.Errorf("invalid source: %w", err)
		}
		ui.Info("Resolved '%s' from installed skill metadata", sourceArg)
		return resolvedSource, true, nil // resolvedFromMeta = true
	}

	return nil, false, fmt.Errorf("invalid source: %w", err)
}

// dispatchInstall routes to the appropriate install handler
func dispatchInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	if opts.Track {
		return handleTrackedRepoInstall(source, cfg, opts)
	}

	if source.IsGit() {
		if !source.HasSubdir() {
			return handleGitDiscovery(source, cfg, opts)
		}
		return handleGitSubdirInstall(source, cfg, opts)
	}

	return handleDirectInstall(source, cfg, opts)
}

func cmdInstall(args []string) error {
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
		summary, err := cmdInstallProject(rest, cwd)
		if summary.Mode == "" {
			summary.Mode = "project"
		}
		logInstallOp(config.ProjectConfigPath(cwd), rest, start, err, summary)
		return err
	}

	parsed, showHelp, parseErr := parseInstallArgs(rest)
	if showHelp {
		printInstallHelp()
		return parseErr
	}
	if parseErr != nil {
		return parseErr
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	source, resolvedFromMeta, err := resolveInstallSource(parsed.sourceArg, parsed.opts, cfg)
	if err != nil {
		logInstallOp(config.ConfigPath(), rest, start, err, installLogSummary{
			Source: parsed.sourceArg,
			Mode:   "global",
		})
		return err
	}

	summary := installLogSummary{
		Source:  parsed.sourceArg,
		Mode:    "global",
		DryRun:  parsed.opts.DryRun,
		Tracked: parsed.opts.Track,
		Into:    parsed.opts.Into,
	}

	// If resolved from metadata with update/force, go directly to install
	if resolvedFromMeta {
		summary, err = handleDirectInstall(source, cfg, parsed.opts)
		if summary.Mode == "" {
			summary.Mode = "global"
		}
		if summary.Source == "" {
			summary.Source = parsed.sourceArg
		}
		logInstallOp(config.ConfigPath(), rest, start, err, summary)
		return err
	}

	summary, err = dispatchInstall(source, cfg, parsed.opts)
	if summary.Mode == "" {
		summary.Mode = "global"
	}
	if summary.Source == "" {
		summary.Source = parsed.sourceArg
	}
	logInstallOp(config.ConfigPath(), rest, start, err, summary)
	return err
}

func logInstallOp(cfgPath string, args []string, start time.Time, cmdErr error, summary installLogSummary) {
	e := oplog.NewEntry("install", statusFromErr(cmdErr), time.Since(start))
	fields := map[string]any{}
	source := summary.Source
	if len(args) > 0 {
		source = args[0]
	}
	if source != "" {
		fields["source"] = source
	}
	if summary.Mode != "" {
		fields["mode"] = summary.Mode
	}
	if summary.DryRun {
		fields["dry_run"] = true
	}
	if summary.Tracked {
		fields["tracked"] = true
	}
	if summary.Into != "" {
		fields["into"] = summary.Into
	}
	if summary.SkillCount > 0 {
		fields["skill_count"] = summary.SkillCount
	}
	if len(summary.InstalledSkills) > 0 {
		fields["installed_skills"] = summary.InstalledSkills
		if _, ok := fields["skill_count"]; !ok {
			fields["skill_count"] = len(summary.InstalledSkills)
		}
	}
	if len(summary.FailedSkills) > 0 {
		fields["failed_skills"] = summary.FailedSkills
	}
	if len(fields) > 0 {
		e.Args = fields
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.Write(cfgPath, oplog.OpsFile, e) //nolint:errcheck
}

func handleTrackedRepoInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source:  source.Raw,
		DryRun:  opts.DryRun,
		Tracked: true,
		Into:    opts.Into,
	}

	// Show logo with version
	ui.Logo(appversion.Version)

	// Step 1: Show source
	ui.StepStart("Source", source.Raw)
	if opts.Name != "" {
		ui.StepContinue("Name", "_"+opts.Name)
	}
	if opts.Into != "" {
		ui.StepContinue("Into", opts.Into)
	}

	// Step 2: Clone with tree spinner
	treeSpinner := ui.StartTreeSpinner("Cloning repository...", false)

	result, err := install.InstallTrackedRepo(source, cfg.Source, opts)
	if err != nil {
		treeSpinner.Fail("Failed to clone")
		return logSummary, err
	}

	treeSpinner.Success("Cloned")

	// Step 3: Show result
	if opts.DryRun {
		ui.StepEnd("Action", result.Action)
		fmt.Println()
		ui.Warning("[dry-run] Would install tracked repo")
	} else {
		ui.StepEnd("Found", fmt.Sprintf("%d skill(s)", result.SkillCount))

		// Show skill box
		fmt.Println()
		ui.SkillBox(result.RepoName, fmt.Sprintf("Tracked repository with %d skills", result.SkillCount), result.RepoPath)

		// Show skill list if not too many
		if len(result.Skills) > 0 && len(result.Skills) <= 10 {
			fmt.Println()
			for _, skill := range result.Skills {
				ui.SkillBoxCompact(skill, "")
			}
		}
	}

	// Display warnings
	for _, warning := range result.Warnings {
		ui.Warning("%s", warning)
	}

	if !opts.DryRun {
		logSummary.SkillCount = result.SkillCount
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, result.Skills...)
	}

	// Show next steps
	if !opts.DryRun {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute skills to all targets")
		ui.Info("Run 'skillshare update %s' to update this repo later", result.RepoName)
	}

	return logSummary, nil
}

func handleGitDiscovery(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source: source.Raw,
		DryRun: opts.DryRun,
		Into:   opts.Into,
	}

	// Show logo with version
	ui.Logo(appversion.Version)

	// Step 1: Show source
	ui.StepStart("Source", source.Raw)
	if opts.Into != "" {
		ui.StepContinue("Into", opts.Into)
	}

	// Step 2: Clone with tree spinner animation
	treeSpinner := ui.StartTreeSpinner("Cloning repository...", false)

	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		treeSpinner.Fail("Failed to clone")
		return logSummary, err
	}
	defer install.CleanupDiscovery(discovery)

	treeSpinner.Success("Cloned")

	// Step 3: Show found skills
	if len(discovery.Skills) == 0 {
		ui.StepEnd("Found", "No skills (no SKILL.md files)")
		return logSummary, nil
	}

	ui.StepEnd("Found", fmt.Sprintf("%d skill(s)", len(discovery.Skills)))

	if opts.Name != "" && len(discovery.Skills) != 1 {
		return logSummary, fmt.Errorf("--name can only be used when exactly one skill is discovered")
	}

	// Single skill: show detailed box and install directly
	if len(discovery.Skills) == 1 {
		skill := discovery.Skills[0]
		if opts.Name != "" {
			if err := validate.SkillName(opts.Name); err != nil {
				return logSummary, fmt.Errorf("invalid skill name '%s': %w", opts.Name, err)
			}
			skill.Name = opts.Name
		}

		loc := skill.Path
		if loc == "." {
			loc = "root"
		}
		fmt.Println()
		ui.SkillBox(skill.Name, "", loc)

		destPath := destWithInto(cfg.Source, opts, skill.Name)
		if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
			return logSummary, fmt.Errorf("failed to create --into directory: %w", err)
		}
		fmt.Println()

		installSpinner := ui.StartSpinner(fmt.Sprintf("Installing %s...", skill.Name))
		result, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			installSpinner.Fail("Failed to install")
			return logSummary, err
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
			logSummary.InstalledSkills = append(logSummary.InstalledSkills, skill.Name)
			logSummary.SkillCount = len(logSummary.InstalledSkills)
		}

		return logSummary, nil
	}

	// Non-interactive path: --skill or --all/--yes
	if opts.HasSkillFilter() || opts.ShouldInstallAll() {
		selected, err := selectSkills(discovery.Skills, opts)
		if err != nil {
			return logSummary, err
		}

		if opts.DryRun {
			fmt.Println()
			for _, skill := range selected {
				ui.SkillBoxCompact(skill.Name, skill.Path)
			}
			fmt.Println()
			ui.Warning("[dry-run] Would install %d skill(s)", len(selected))
			return logSummary, nil
		}

		fmt.Println()
		batchSummary := installSelectedSkills(selected, discovery, cfg, opts)
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, batchSummary.InstalledSkills...)
		logSummary.FailedSkills = append(logSummary.FailedSkills, batchSummary.FailedSkills...)
		logSummary.SkillCount = len(logSummary.InstalledSkills)
		return logSummary, nil
	}

	if opts.DryRun {
		// Show skill list in dry-run mode
		fmt.Println()
		for _, skill := range discovery.Skills {
			ui.SkillBoxCompact(skill.Name, skill.Path)
		}
		fmt.Println()
		ui.Warning("[dry-run] Would prompt for selection")
		return logSummary, nil
	}

	fmt.Println()

	selected, err := promptSkillSelection(discovery.Skills)
	if err != nil {
		return logSummary, err
	}

	if len(selected) == 0 {
		ui.Info("No skills selected")
		return logSummary, nil
	}

	fmt.Println()
	batchSummary := installSelectedSkills(selected, discovery, cfg, opts)
	logSummary.InstalledSkills = append(logSummary.InstalledSkills, batchSummary.InstalledSkills...)
	logSummary.FailedSkills = append(logSummary.FailedSkills, batchSummary.FailedSkills...)
	logSummary.SkillCount = len(logSummary.InstalledSkills)

	return logSummary, nil
}

// selectSkills routes to the appropriate skill selection method:
// --skill filter, --all/--yes auto-select, or interactive prompt.
func selectSkills(skills []install.SkillInfo, opts install.InstallOptions) ([]install.SkillInfo, error) {
	switch {
	case opts.HasSkillFilter():
		matched, notFound := filterSkillsByName(skills, opts.Skills)
		if len(notFound) > 0 {
			return nil, fmt.Errorf("skills not found: %s\nAvailable: %s",
				strings.Join(notFound, ", "), skillNames(skills))
		}
		return matched, nil
	case opts.ShouldInstallAll():
		return skills, nil
	default:
		return promptSkillSelection(skills)
	}
}

// filterSkillsByName matches requested names against discovered skills.
// It tries exact match first, then falls back to fuzzy matching.
func filterSkillsByName(skills []install.SkillInfo, names []string) (matched []install.SkillInfo, notFound []string) {
	skillNames := make([]string, len(skills))
	for i, s := range skills {
		skillNames[i] = s.Name
	}
	skillByName := make(map[string]install.SkillInfo, len(skills))
	for _, s := range skills {
		skillByName[s.Name] = s
	}

	for _, name := range names {
		// Try exact match first
		if s, ok := skillByName[name]; ok {
			matched = append(matched, s)
			continue
		}

		// Fall back to fuzzy match
		ranks := fuzzy.RankFindNormalizedFold(name, skillNames)
		sort.Sort(ranks)
		if len(ranks) == 1 {
			matched = append(matched, skillByName[ranks[0].Target])
		} else if len(ranks) > 1 {
			suggestions := make([]string, len(ranks))
			for i, r := range ranks {
				suggestions[i] = r.Target
			}
			notFound = append(notFound, fmt.Sprintf("%s (did you mean: %s?)", name, strings.Join(suggestions, ", ")))
		} else {
			notFound = append(notFound, name)
		}
	}
	return
}

// skillNames returns a comma-separated list of skill names for error messages.
func skillNames(skills []install.SkillInfo) string {
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

func promptSkillSelection(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	// Check for orchestrator structure (root + children)
	var rootSkill *install.SkillInfo
	var childSkills []install.SkillInfo
	for i := range skills {
		if skills[i].Path == "." {
			rootSkill = &skills[i]
		} else {
			childSkills = append(childSkills, skills[i])
		}
	}

	// If orchestrator structure detected, use two-stage selection
	if rootSkill != nil && len(childSkills) > 0 {
		return promptOrchestratorSelection(*rootSkill, childSkills)
	}

	// Otherwise, use standard multi-select
	return promptMultiSelect(skills)
}

func promptOrchestratorSelection(rootSkill install.SkillInfo, childSkills []install.SkillInfo) ([]install.SkillInfo, error) {
	// Stage 1: Choose install mode
	options := []string{
		fmt.Sprintf("Install entire pack  \033[90m%s + %d children\033[0m", rootSkill.Name, len(childSkills)),
		"Select individual skills",
	}

	var modeIdx int
	prompt := &survey.Select{
		Message:  "Install mode:",
		Options:  options,
		PageSize: 5,
	}

	err := survey.AskOne(prompt, &modeIdx, survey.WithIcons(func(icons *survey.IconSet) {
		icons.SelectFocus.Text = "▸"
		icons.SelectFocus.Format = "yellow"
	}))
	if err != nil {
		return nil, nil
	}

	// If "entire pack" selected, return all skills
	if modeIdx == 0 {
		allSkills := make([]install.SkillInfo, 0, len(childSkills)+1)
		allSkills = append(allSkills, rootSkill)
		allSkills = append(allSkills, childSkills...)
		return allSkills, nil
	}

	// Stage 2: Select individual skills (children only, no root)
	return promptMultiSelect(childSkills)
}

func promptMultiSelect(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	options := make([]string, len(skills))
	for i, skill := range skills {
		loc := skill.Path
		if skill.Path == "." {
			loc = "root"
		}
		options[i] = fmt.Sprintf("%s  \033[90m%s\033[0m", skill.Name, loc)
	}

	var selectedIndices []int
	prompt := &survey.MultiSelect{
		Message:  "Select skills to install:",
		Options:  options,
		PageSize: 15,
	}

	err := survey.AskOne(prompt, &selectedIndices, survey.WithIcons(func(icons *survey.IconSet) {
		icons.UnmarkedOption.Text = " "
		icons.MarkedOption.Text = "✓"
		icons.MarkedOption.Format = "green"
		icons.SelectFocus.Text = "▸"
		icons.SelectFocus.Format = "yellow"
	}))
	if err != nil {
		return nil, nil
	}

	selected := make([]install.SkillInfo, len(selectedIndices))
	for i, idx := range selectedIndices {
		selected[i] = skills[idx]
	}

	return selected, nil
}

// skillInstallResult holds the result of installing a single skill
type skillInstallResult struct {
	skill   install.SkillInfo
	success bool
	message string
}

// installSelectedSkills installs multiple skills with progress display
func installSelectedSkills(selected []install.SkillInfo, discovery *install.DiscoveryResult, cfg *config.Config, opts install.InstallOptions) installBatchSummary {
	results := make([]skillInstallResult, 0, len(selected))
	installSpinner := ui.StartSpinnerWithSteps("Installing...", len(selected))

	// Ensure Into directory exists for batch installs
	if opts.Into != "" {
		if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
			installSpinner.Fail("Failed to create --into directory")
			return installBatchSummary{}
		}
	}

	// Detect orchestrator: if root skill (path=".") is selected, children nest under it
	var parentName string
	var rootIdx = -1
	for i, skill := range selected {
		if skill.Path == "." {
			parentName = skill.Name
			rootIdx = i
			break
		}
	}

	// Reorder: install root skill first so children can nest under it
	orderedSkills := selected
	if rootIdx > 0 {
		orderedSkills = make([]install.SkillInfo, 0, len(selected))
		orderedSkills = append(orderedSkills, selected[rootIdx])
		orderedSkills = append(orderedSkills, selected[:rootIdx]...)
		orderedSkills = append(orderedSkills, selected[rootIdx+1:]...)
	}

	// Track if root was installed (children are already included in root)
	rootInstalled := false

	for i, skill := range orderedSkills {
		installSpinner.NextStep(fmt.Sprintf("Installing %s...", skill.Name))
		if i == 0 {
			installSpinner.Update(fmt.Sprintf("Installing %s...", skill.Name))
		}

		// Determine destination path
		var destPath string
		if skill.Path == "." {
			// Root skill - install directly
			destPath = destWithInto(cfg.Source, opts, skill.Name)
		} else if parentName != "" {
			// Child skill with parent selected - nest under parent
			destPath = destWithInto(cfg.Source, opts, filepath.Join(parentName, skill.Name))
		} else {
			// Standalone child skill - install to root
			destPath = destWithInto(cfg.Source, opts, skill.Name)
		}

		// If root was installed, children are already included - skip reinstall
		if rootInstalled && skill.Path != "." {
			results = append(results, skillInstallResult{skill: skill, success: true, message: fmt.Sprintf("included in %s", parentName)})
			continue
		}

		_, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			results = append(results, skillInstallResult{skill: skill, success: false, message: err.Error()})
			continue
		}

		if skill.Path == "." {
			rootInstalled = true
		}
		results = append(results, skillInstallResult{skill: skill, success: true, message: "installed"})
	}

	displayInstallResults(results, installSpinner)

	summary := installBatchSummary{
		InstalledSkills: make([]string, 0, len(results)),
		FailedSkills:    make([]string, 0, len(results)),
	}
	for _, r := range results {
		if r.success {
			summary.InstalledSkills = append(summary.InstalledSkills, r.skill.Name)
			continue
		}
		summary.FailedSkills = append(summary.FailedSkills, r.skill.Name)
	}
	return summary
}

// displayInstallResults shows the final install results
func displayInstallResults(results []skillInstallResult, spinner *ui.Spinner) {
	var successes, failures []skillInstallResult
	for _, r := range results {
		if r.success {
			successes = append(successes, r)
		} else {
			failures = append(failures, r)
		}
	}

	installed := len(successes)
	failed := len(failures)

	if failed > 0 && installed == 0 {
		spinner.Fail(fmt.Sprintf("Failed to install %d skill(s)", failed))
	} else if failed > 0 {
		spinner.Warn(fmt.Sprintf("Installed %d, failed %d", installed, failed))
	} else {
		spinner.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	}

	// Show failures first with details
	if failed > 0 {
		fmt.Println()
		for _, r := range failures {
			ui.StepFail(r.skill.Name, r.message)
		}
	}

	// Show successes — condensed when many
	if installed > 0 {
		fmt.Println()
		if installed > 10 {
			names := make([]string, installed)
			for i, r := range successes {
				names[i] = r.skill.Name
			}
			ui.StepDone(fmt.Sprintf("%d skills installed", installed), strings.Join(names, ", "))
		} else {
			for _, r := range successes {
				ui.StepDone(r.skill.Name, r.message)
			}
		}
	}

	if installed > 0 {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}
}

func handleGitSubdirInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source: source.Raw,
		DryRun: opts.DryRun,
		Into:   opts.Into,
	}

	// Show logo with version
	ui.Logo(appversion.Version)

	// Step 1: Show source
	ui.StepStart("Source", source.Raw)
	ui.StepContinue("Subdir", source.Subdir)
	if opts.Into != "" {
		ui.StepContinue("Into", opts.Into)
	}

	// Step 2: Clone with tree spinner
	treeSpinner := ui.StartTreeSpinner("Cloning repository...", false)

	// Discover skills in subdir
	discovery, err := install.DiscoverFromGitSubdir(source)
	if err != nil {
		treeSpinner.Fail("Failed to clone")
		return logSummary, err
	}
	defer install.CleanupDiscovery(discovery)

	treeSpinner.Success("Cloned")

	// If only one skill found, install directly
	if len(discovery.Skills) == 1 {
		skill := discovery.Skills[0]
		if opts.Name != "" {
			if err := validate.SkillName(opts.Name); err != nil {
				return logSummary, fmt.Errorf("invalid skill name '%s': %w", opts.Name, err)
			}
			skill.Name = opts.Name
		}
		ui.StepEnd("Found", fmt.Sprintf("1 skill: %s", skill.Name))

		destPath := destWithInto(cfg.Source, opts, skill.Name)
		if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
			return logSummary, fmt.Errorf("failed to create --into directory: %w", err)
		}

		fmt.Println()
		installSpinner := ui.StartSpinner(fmt.Sprintf("Installing %s...", skill.Name))

		result, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			installSpinner.Fail("Failed to install")
			return logSummary, err
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
			logSummary.InstalledSkills = append(logSummary.InstalledSkills, skill.Name)
			logSummary.SkillCount = len(logSummary.InstalledSkills)
		}
		return logSummary, nil
	}

	// Multiple skills found - enter discovery mode
	if len(discovery.Skills) == 0 {
		ui.StepEnd("Found", "No skills (no SKILL.md files)")
		return logSummary, nil
	}

	ui.StepEnd("Found", fmt.Sprintf("%d skill(s)", len(discovery.Skills)))

	if opts.Name != "" {
		return logSummary, fmt.Errorf("--name can only be used when exactly one skill is discovered")
	}

	// Non-interactive path: --skill or --all/--yes
	if opts.HasSkillFilter() || opts.ShouldInstallAll() {
		selected, err := selectSkills(discovery.Skills, opts)
		if err != nil {
			return logSummary, err
		}

		if opts.DryRun {
			fmt.Println()
			for _, skill := range selected {
				ui.SkillBoxCompact(skill.Name, skill.Path)
			}
			fmt.Println()
			ui.Warning("[dry-run] Would install %d skill(s)", len(selected))
			return logSummary, nil
		}

		fmt.Println()
		batchSummary := installSelectedSkills(selected, discovery, cfg, opts)
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, batchSummary.InstalledSkills...)
		logSummary.FailedSkills = append(logSummary.FailedSkills, batchSummary.FailedSkills...)
		logSummary.SkillCount = len(logSummary.InstalledSkills)
		return logSummary, nil
	}

	if opts.DryRun {
		fmt.Println()
		for _, skill := range discovery.Skills {
			ui.SkillBoxCompact(skill.Name, skill.Path)
		}
		fmt.Println()
		ui.Warning("[dry-run] Would prompt for selection")
		return logSummary, nil
	}

	fmt.Println()

	selected, err := promptSkillSelection(discovery.Skills)
	if err != nil {
		return logSummary, err
	}

	if len(selected) == 0 {
		ui.Info("No skills selected")
		return logSummary, nil
	}

	fmt.Println()
	batchSummary := installSelectedSkills(selected, discovery, cfg, opts)
	logSummary.InstalledSkills = append(logSummary.InstalledSkills, batchSummary.InstalledSkills...)
	logSummary.FailedSkills = append(logSummary.FailedSkills, batchSummary.FailedSkills...)
	logSummary.SkillCount = len(logSummary.InstalledSkills)

	return logSummary, nil
}

func handleDirectInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source: source.Raw,
		DryRun: opts.DryRun,
		Into:   opts.Into,
	}

	// Determine skill name
	skillName := source.Name
	if opts.Name != "" {
		skillName = opts.Name
	}

	// Validate skill name
	if err := validate.SkillName(skillName); err != nil {
		return logSummary, fmt.Errorf("invalid skill name '%s': %w", skillName, err)
	}

	// Set the name in source for display
	source.Name = skillName

	// Determine destination path
	destPath := destWithInto(cfg.Source, opts, skillName)

	// Ensure Into directory exists
	if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
		return logSummary, fmt.Errorf("failed to create --into directory: %w", err)
	}

	// Show logo with version
	ui.Logo(appversion.Version)

	// Step 1: Show source info
	ui.StepStart("Source", source.Raw)
	ui.StepContinue("Name", skillName)
	if opts.Into != "" {
		ui.StepContinue("Into", opts.Into)
	}
	if source.HasSubdir() {
		ui.StepContinue("Subdir", source.Subdir)
	}

	// Step 2: Clone/copy with tree spinner
	var actionMsg string
	if source.IsGit() {
		actionMsg = "Cloning repository..."
	} else {
		actionMsg = "Copying files..."
	}
	treeSpinner := ui.StartTreeSpinner(actionMsg, true)

	// Execute installation
	result, err := install.Install(source, destPath, opts)
	if err != nil {
		treeSpinner.Fail("Failed to install")
		return logSummary, err
	}

	// Display result
	if opts.DryRun {
		treeSpinner.Success("Ready")
		fmt.Println()
		ui.Warning("[dry-run] %s", result.Action)
	} else {
		treeSpinner.Success(fmt.Sprintf("Installed: %s", skillName))
	}

	// Display warnings
	for _, warning := range result.Warnings {
		ui.Warning("%s", warning)
	}

	// Show next steps
	if !opts.DryRun {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute to all targets")
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, skillName)
		logSummary.SkillCount = len(logSummary.InstalledSkills)
	}

	return logSummary, nil
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
  --name <name>       Override installed name when exactly one skill is installed
  --into <dir>        Install into subdirectory (e.g. "frontend" or "frontend/react")
  --force, -f         Overwrite if skill already exists
  --update, -u        Update existing (git pull if possible, else reinstall)
  --track, -t         Install as tracked repo (preserves .git for updates)
  --skill, -s <names> Select specific skills from multi-skill repo (comma-separated)
  --all               Install all discovered skills without prompting
  --yes, -y           Auto-accept all prompts (equivalent to --all for multi-skill repos)
  --dry-run, -n       Preview the installation without making changes
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare install anthropics/skills
  skillshare install anthropics/skills/skills/pdf
  skillshare install ComposioHQ/awesome-claude-skills
  skillshare install ~/my-skill
  skillshare install github.com/user/repo --force

Selective install (non-interactive):
  skillshare install anthropics/skills -s pdf,commit     # Specific skills
  skillshare install anthropics/skills --all             # All skills
  skillshare install anthropics/skills -y                # Auto-accept
  skillshare install anthropics/skills -s pdf --dry-run  # Preview selection

Organize into subdirectories:
  skillshare install anthropics/skills -s pdf --into frontend
  skillshare install user/repo --track --into devops
  skillshare install ~/my-skill --into frontend/react

Tracked repositories (Team Edition):
  skillshare install team/shared-skills --track   # Clone as _shared-skills
  skillshare install _shared-skills --update      # Update tracked repo

Update existing skills:
  skillshare install my-skill --update       # Update using stored source
  skillshare install my-skill --force        # Reinstall using stored source
  skillshare install my-skill --update -n    # Preview update`)
}
