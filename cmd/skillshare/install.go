package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"

	"skillshare/internal/audit"
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
	SkipAudit       bool
	AuditVerbose    bool
	AuditThreshold  string
}

type installBatchSummary struct {
	InstalledSkills []string
	FailedSkills    []string
}

var installAuditSeverityOrder = []string{
	audit.SeverityCritical,
	audit.SeverityHigh,
	audit.SeverityMedium,
	audit.SeverityLow,
	audit.SeverityInfo,
}

var installAuditThresholdPattern = regexp.MustCompile(`at/above\s+([A-Z]+)\s+detected`)

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
		case arg == "--skip-audit":
			result.opts.SkipAudit = true
		case arg == "--audit-verbose":
			result.opts.AuditVerbose = true
		case arg == "--audit-threshold" || arg == "--threshold" || arg == "-T":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("%s requires a value", arg)
			}
			i++
			threshold, err := normalizeInstallAuditThreshold(args[i])
			if err != nil {
				return nil, false, err
			}
			result.opts.AuditThreshold = threshold
		case arg == "--track" || arg == "-t":
			result.opts.Track = true
		case arg == "--skill" || arg == "-s":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--skill requires a value")
			}
			i++
			result.opts.Skills = strings.Split(args[i], ",")
		case arg == "--exclude":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--exclude requires a value")
			}
			i++
			result.opts.Exclude = strings.Split(args[i], ",")
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

	// Clean --exclude input
	if len(result.opts.Exclude) > 0 {
		cleaned := make([]string, 0, len(result.opts.Exclude))
		for _, s := range result.opts.Exclude {
			s = strings.TrimSpace(s)
			if s != "" {
				cleaned = append(cleaned, s)
			}
		}
		result.opts.Exclude = cleaned
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

	// When no source is given, only bare "install" is valid — reject incompatible flags
	if result.sourceArg == "" {
		hasSourceFlags := result.opts.Name != "" || result.opts.Into != "" ||
			result.opts.Track || len(result.opts.Skills) > 0 ||
			len(result.opts.Exclude) > 0 || result.opts.All || result.opts.Yes || result.opts.Update
		if hasSourceFlags {
			return nil, false, fmt.Errorf("flags --name, --into, --track, --skill, --exclude, --all, --yes, and --update require a source argument")
		}
		return result, false, nil
	}

	if result.opts.Into != "" {
		if err := validate.IntoPath(result.opts.Into); err != nil {
			return nil, false, err
		}
	}

	return result, false, nil
}

// normalizeInstallAuditThreshold normalizes install threshold values and
// supports shorthand level aliases for CLI ergonomics.
func normalizeInstallAuditThreshold(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "c", "crit":
		v = audit.SeverityCritical
	case "h":
		v = audit.SeverityHigh
	case "m", "med":
		v = audit.SeverityMedium
	case "l":
		v = audit.SeverityLow
	case "i":
		v = audit.SeverityInfo
	}

	threshold, err := audit.NormalizeThreshold(v)
	if err != nil {
		return "", fmt.Errorf("invalid audit threshold %q (use: critical|high|medium|low|info or c|h|m|l|i)", raw)
	}
	return threshold, nil
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
	if parsed.opts.AuditThreshold == "" {
		parsed.opts.AuditThreshold = cfg.Audit.BlockThreshold
	}

	// No source argument: install from global config
	if parsed.sourceArg == "" {
		summary, err := installFromGlobalConfig(cfg, parsed.opts)
		logInstallOp(config.ConfigPath(), rest, start, err, summary)
		return err
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
		Source:         parsed.sourceArg,
		Mode:           "global",
		DryRun:         parsed.opts.DryRun,
		Tracked:        parsed.opts.Track,
		Into:           parsed.opts.Into,
		SkipAudit:      parsed.opts.SkipAudit,
		AuditVerbose:   parsed.opts.AuditVerbose,
		AuditThreshold: parsed.opts.AuditThreshold,
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
		if err == nil && !parsed.opts.DryRun && len(summary.InstalledSkills) > 0 {
			if rErr := config.ReconcileGlobalSkills(cfg); rErr != nil {
				ui.Warning("Failed to reconcile global skills config: %v", rErr)
			}
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
	if err == nil && !parsed.opts.DryRun && len(summary.InstalledSkills) > 0 {
		if rErr := config.ReconcileGlobalSkills(cfg); rErr != nil {
			ui.Warning("Failed to reconcile global skills config: %v", rErr)
		}
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
	if summary.SkipAudit {
		fields["skip_audit"] = true
	}
	if summary.AuditVerbose {
		fields["audit_verbose"] = true
	}
	if summary.AuditThreshold != "" {
		fields["threshold"] = strings.ToUpper(summary.AuditThreshold)
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
		Source:         source.Raw,
		DryRun:         opts.DryRun,
		Tracked:        true,
		Into:           opts.Into,
		SkipAudit:      opts.SkipAudit,
		AuditVerbose:   opts.AuditVerbose,
		AuditThreshold: opts.AuditThreshold,
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
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			treeSpinner.Update(line)
		}
	}

	result, err := install.InstallTrackedRepo(source, cfg.Source, opts)
	if err != nil {
		treeSpinner.Fail("Failed to clone")
		if errors.Is(err, audit.ErrBlocked) {
			return logSummary, renderBlockedAuditError(err)
		}
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
	renderInstallWarnings("", result.Warnings, opts.AuditVerbose)

	if !opts.DryRun {
		logSummary.SkillCount = result.SkillCount
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, result.Skills...)
	}

	// Show next steps
	if !opts.DryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute skills to all targets")
		ui.Info("Run 'skillshare update %s' to update this repo later", result.RepoName)
	}

	return logSummary, nil
}

func handleGitDiscovery(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source:         source.Raw,
		DryRun:         opts.DryRun,
		Into:           opts.Into,
		SkipAudit:      opts.SkipAudit,
		AuditVerbose:   opts.AuditVerbose,
		AuditThreshold: opts.AuditThreshold,
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
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			treeSpinner.Update(line)
		}
	}

	discovery, err := install.DiscoverFromGitWithProgress(source, opts.OnProgress)
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

	// Apply --exclude early so excluded skills never appear in prompts
	if len(opts.Exclude) > 0 {
		discovery.Skills = applyExclude(discovery.Skills, opts.Exclude)
		if len(discovery.Skills) == 0 {
			ui.Info("All skills were excluded")
			return logSummary, nil
		}
	}

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
		desc := ""
		if skill.License != "" {
			desc = "License: " + skill.License
		}
		ui.SkillBox(skill.Name, desc, loc)

		destPath := destWithInto(cfg.Source, opts, skill.Name)
		if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
			return logSummary, fmt.Errorf("failed to create --into directory: %w", err)
		}
		fmt.Println()

		installSpinner := ui.StartSpinner(fmt.Sprintf("Installing %s...", skill.Name))
		result, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			installSpinner.Fail("Failed to install")
			if errors.Is(err, audit.ErrBlocked) {
				return logSummary, renderBlockedAuditError(err)
			}
			return logSummary, err
		}

		if opts.DryRun {
			installSpinner.Stop()
			ui.Warning("[dry-run] %s", result.Action)
		} else {
			installSpinner.Success(fmt.Sprintf("Installed: %s", skill.Name))
		}

		renderInstallWarnings("", result.Warnings, opts.AuditVerbose)

		if !opts.DryRun {
			ui.SectionLabel("Next Steps")
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
			printSkillListCompact(selected)
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
		printSkillListCompact(discovery.Skills)
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
// Callers are expected to apply --exclude filtering before calling this function.
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

// applyExclude removes skills whose names appear in the exclude list.
func applyExclude(skills []install.SkillInfo, exclude []string) []install.SkillInfo {
	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excludeSet[name] = true
	}
	var excluded []string
	filtered := make([]install.SkillInfo, 0, len(skills))
	for _, s := range skills {
		if excludeSet[s.Name] {
			excluded = append(excluded, s.Name)
			continue
		}
		filtered = append(filtered, s)
	}
	if len(excluded) > 0 {
		ui.Info("Excluded %d skill(s): %s", len(excluded), strings.Join(excluded, ", "))
	}
	return filtered
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
	// Sort by path so skills in the same directory cluster together
	sorted := make([]install.SkillInfo, len(skills))
	copy(sorted, skills)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})
	skills = sorted

	options := make([]string, len(skills))
	for i, skill := range skills {
		dir := filepath.Dir(skill.Path)
		var loc string
		switch {
		case skill.Path == ".":
			loc = "root"
		case dir == ".":
			loc = "root"
		default:
			loc = dir
		}
		label := skill.Name
		if skill.License != "" {
			label += fmt.Sprintf(" (%s)", skill.License)
		}
		options[i] = fmt.Sprintf("%s  \033[90m%s\033[0m", label, loc)
	}

	var selectedIndices []int
	prompt := &survey.MultiSelect{
		Message:  "Select skills to install:",
		Options:  options,
		PageSize: 15,
	}

	err := survey.AskOne(prompt, &selectedIndices,
		survey.WithKeepFilter(true),
		survey.WithIcons(func(icons *survey.IconSet) {
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
	skill    install.SkillInfo
	success  bool
	message  string
	warnings []string
	err      error
}

type installWarningDigest struct {
	findingCounts   map[string]int
	findingByLevel  map[string][]string
	statusLines     []string
	otherAuditLines []string
	nonAuditLines   []string
}

type batchInstallWarningDigest struct {
	totalFindings            int
	findingCounts            map[string]int
	skillsWithFindings       int
	belowThresholdSkillCount int
	aboveThresholdSkillCount int
	scanErrorSkillCount      int
	skippedAuditSkillCount   int
	highCriticalBySkill      map[string]int
	nonAuditLines            []string
	otherAuditLines          []string
}

func parseInstallAuditSeverity(warning string) (string, bool) {
	for _, severity := range installAuditSeverityOrder {
		if strings.HasPrefix(warning, "audit "+severity+":") {
			return severity, true
		}
	}
	return "", false
}

func firstWarningLine(warning string) string {
	trimmed := strings.TrimSpace(warning)
	if i := strings.IndexByte(trimmed, '\n'); i >= 0 {
		return strings.TrimSpace(trimmed[:i])
	}
	return trimmed
}

func digestInstallWarnings(warnings []string) installWarningDigest {
	digest := installWarningDigest{
		findingCounts:  make(map[string]int, len(installAuditSeverityOrder)),
		findingByLevel: make(map[string][]string, len(installAuditSeverityOrder)),
	}

	for _, warning := range warnings {
		if severity, ok := parseInstallAuditSeverity(warning); ok {
			line := firstWarningLine(warning)
			digest.findingCounts[severity]++
			digest.findingByLevel[severity] = append(digest.findingByLevel[severity], line)
			continue
		}

		switch {
		case strings.HasPrefix(warning, "audit findings "):
			digest.statusLines = append(digest.statusLines, firstWarningLine(warning))
		case strings.HasPrefix(warning, "audit scan error"), strings.HasPrefix(warning, "audit skipped"), strings.HasPrefix(warning, "audit "):
			digest.otherAuditLines = append(digest.otherAuditLines, firstWarningLine(warning))
		default:
			digest.nonAuditLines = append(digest.nonAuditLines, warning)
		}
	}

	return digest
}

func formatWarningWithSkill(skillName, warning string) string {
	if skillName == "" {
		return warning
	}
	return fmt.Sprintf("%s: %s", skillName, warning)
}

// auditFindingGroup groups findings with the same severity and message.
type auditFindingGroup struct {
	severity  string
	message   string
	locations []string // e.g., "SKILL.md:3"
}

// installAuditFindingLinePattern parses "audit HIGH: Sudo escalation (SKILL.md:3)"
var installAuditFindingLinePattern = regexp.MustCompile(
	`^audit\s+([A-Z]+):\s+(.+?)\s+\(([^)]+)\)\s*$`,
)

// groupAuditFindings groups finding lines by (severity, message), collecting locations.
func groupAuditFindings(digest installWarningDigest) []auditFindingGroup {
	type groupKey struct {
		severity string
		message  string
	}
	indexMap := make(map[groupKey]int)
	var groups []auditFindingGroup

	for _, severity := range installAuditSeverityOrder {
		for _, line := range digest.findingByLevel[severity] {
			m := installAuditFindingLinePattern.FindStringSubmatch(line)
			if m == nil {
				// Fallback: ungroupable line as its own group
				groups = append(groups, auditFindingGroup{
					severity: severity, message: stripAuditPrefix(line), locations: nil,
				})
				continue
			}
			sev, msg, loc := m[1], m[2], m[3]
			key := groupKey{severity: strings.ToUpper(sev), message: msg}
			if idx, ok := indexMap[key]; ok {
				groups[idx].locations = append(groups[idx].locations, loc)
			} else {
				indexMap[key] = len(groups)
				groups = append(groups, auditFindingGroup{
					severity: strings.ToUpper(sev), message: msg, locations: []string{loc},
				})
			}
		}
	}
	return groups
}

// stripAuditPrefix removes the "audit " prefix from a finding line.
func stripAuditPrefix(line string) string {
	for _, severity := range installAuditSeverityOrder {
		prefix := "audit " + severity + ": "
		if strings.HasPrefix(line, prefix) {
			return severity + ": " + strings.TrimPrefix(line, prefix)
		}
	}
	// Also strip bare "audit " prefix from status lines
	if strings.HasPrefix(line, "audit ") {
		return strings.TrimPrefix(line, "audit ")
	}
	return line
}

// formatFindingGroup formats a grouped finding as a single line.
func formatFindingGroup(g auditFindingGroup) string {
	var sb strings.Builder
	sb.WriteString(g.severity)
	sb.WriteString(": ")
	sb.WriteString(g.message)
	if len(g.locations) > 1 {
		sb.WriteString(fmt.Sprintf(" × %d", len(g.locations)))
	}
	// Show locations (compact)
	if len(g.locations) > 0 {
		const maxLocs = 5
		sb.WriteString(" (")
		for i, loc := range g.locations {
			if i >= maxLocs {
				sb.WriteString(fmt.Sprintf(", +%d more", len(g.locations)-maxLocs))
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(loc)
		}
		sb.WriteString(")")
	}
	return sb.String()
}

// renderInstallWarnings renders audit warnings for a single skill.
// When skillName is empty, it prints a SectionLabel header (single-skill mode).
// When skillName is set, each line is prefixed with the skill name (batch mode).
func renderInstallWarnings(skillName string, warnings []string, auditVerbose bool) {
	renderInstallWarningsWithResult(skillName, warnings, auditVerbose, nil)
}

// renderInstallWarningsWithResult is like renderInstallWarnings but also displays
// the aggregate risk score from the install result when available.
func renderInstallWarningsWithResult(skillName string, warnings []string, auditVerbose bool, result *install.InstallResult) {
	if len(warnings) == 0 {
		renderAuditRiskOnly(skillName, result)
		return
	}

	// Visual separator for single-skill output (batch mode handles its own sections)
	if skillName == "" {
		ui.SectionLabel("Audit Findings")
	}

	digest := digestInstallWarnings(warnings)

	// Non-audit warnings first
	for _, warning := range digest.nonAuditLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, warning))
	}

	// Compute totals
	totalFindings := 0
	countParts := make([]string, 0, len(installAuditSeverityOrder))
	for _, severity := range installAuditSeverityOrder {
		count := digest.findingCounts[severity]
		if count == 0 {
			continue
		}
		totalFindings += count
		countParts = append(countParts, fmt.Sprintf("%s=%d", severity, count))
	}

	// Summary-first: counts + status + risk score
	if totalFindings > 0 {
		summary := strings.Join(countParts, ", ")
		if len(digest.statusLines) > 0 {
			// Append threshold status (strip "audit " prefix)
			status := stripAuditPrefix(digest.statusLines[0])
			summary += " — " + status
		}
		ui.Info("%s", formatWarningWithSkill(skillName, fmt.Sprintf("%d finding(s): %s", totalFindings, summary)))
	} else {
		for _, line := range digest.statusLines {
			ui.Info("%s", formatWarningWithSkill(skillName, stripAuditPrefix(line)))
		}
	}
	renderAuditRiskOnly(skillName, result)
	for _, line := range digest.otherAuditLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, stripAuditPrefix(line)))
	}

	if totalFindings == 0 {
		return
	}

	// Group findings by message
	groups := groupAuditFindings(digest)

	if auditVerbose {
		// Verbose: show all groups with all locations
		for _, g := range groups {
			ui.Warning("%s", formatWarningWithSkill(skillName, formatFindingGroup(g)))
		}
		return
	}

	// Compact: show top groups (by severity order, already sorted)
	const maxGroups = 5
	shown := 0
	for _, g := range groups {
		if shown >= maxGroups {
			break
		}
		ui.Warning("%s", formatWarningWithSkill(skillName, formatFindingGroup(g)))
		shown++
	}

	if remaining := len(groups) - shown; remaining > 0 {
		ui.Info("%s", formatWarningWithSkill(skillName,
			fmt.Sprintf("+%d more finding type(s); use --audit-verbose for full details", remaining)))
	}
}

// renderAuditRiskOnly prints the aggregate risk score if available.
func renderAuditRiskOnly(skillName string, result *install.InstallResult) {
	if result == nil || result.AuditSkipped || result.AuditRiskLabel == "" {
		return
	}
	label := strings.ToUpper(result.AuditRiskLabel)
	if result.AuditRiskScore > 0 {
		ui.Info("%s", formatWarningWithSkill(skillName,
			fmt.Sprintf("risk: %s (%d/100)", label, result.AuditRiskScore)))
	} else {
		ui.Info("%s", formatWarningWithSkill(skillName,
			fmt.Sprintf("risk: %s", label)))
	}
}

// renderBlockedAuditError displays a structured audit-blocked error and returns
// a short error for main.go to print as a one-line summary.
func renderBlockedAuditError(err error) error {
	msg := err.Error()

	// Extract finding detail lines (lines starting with a severity level)
	var detailLines []string
	for _, rawLine := range strings.Split(msg, "\n") {
		line := strings.TrimSpace(rawLine)
		if isAuditSeverityLine(line) {
			detailLines = append(detailLines, "  "+line)
		}
	}

	// Build summary from parsed digest
	digest := parseAuditBlockedFailure(msg)
	summaryParts := []string{"security audit"}
	if digest.threshold != "" && digest.findingCount > 0 {
		suffix := "findings"
		if digest.findingCount == 1 {
			suffix = "finding"
		}
		summaryParts = append(summaryParts, fmt.Sprintf("blocked — %d %s at/above %s", digest.findingCount, suffix, digest.threshold))
	} else {
		summaryParts = append(summaryParts, "blocked")
	}

	ui.SectionLabel("Audit Findings")
	ui.StepFail(strings.Join(summaryParts, ": "), "")
	for _, line := range detailLines {
		fmt.Println(line)
	}

	// Check for cleanup failure or rollback info
	if strings.Contains(msg, "Automatic cleanup failed") || strings.Contains(msg, "Manual removal is required") {
		fmt.Println()
		ui.Warning("automatic cleanup failed — manual removal may be required")
	}
	if strings.Contains(msg, "rollback also failed") {
		fmt.Println()
		ui.Warning("rollback also failed — malicious content may remain")
	}

	return fmt.Errorf("blocked by security audit (use --force to override)")
}

func appendUniqueLimited(lines []string, line string, limit int) []string {
	for _, existing := range lines {
		if existing == line {
			return lines
		}
	}
	if len(lines) >= limit {
		return lines
	}
	return append(lines, line)
}

func summarizeBatchInstallWarnings(results []skillInstallResult) batchInstallWarningDigest {
	summary := batchInstallWarningDigest{
		findingCounts:       make(map[string]int, len(installAuditSeverityOrder)),
		highCriticalBySkill: make(map[string]int),
	}

	for _, result := range results {
		if len(result.warnings) == 0 {
			continue
		}

		digest := digestInstallWarnings(result.warnings)
		skillHasFindings := false

		for _, severity := range installAuditSeverityOrder {
			count := digest.findingCounts[severity]
			if count == 0 {
				continue
			}
			summary.totalFindings += count
			summary.findingCounts[severity] += count
			skillHasFindings = true
		}

		if skillHasFindings {
			summary.skillsWithFindings++
			highCritical := digest.findingCounts[audit.SeverityCritical] + digest.findingCounts[audit.SeverityHigh]
			if highCritical > 0 {
				summary.highCriticalBySkill[result.skill.Name] = highCritical
			}
		}

		for _, line := range digest.statusLines {
			switch {
			case strings.HasPrefix(line, "audit findings detected, but none at/above block threshold"):
				summary.belowThresholdSkillCount++
			case strings.HasPrefix(line, "audit findings at/above block threshold"):
				summary.aboveThresholdSkillCount++
			default:
				summary.otherAuditLines = appendUniqueLimited(
					summary.otherAuditLines,
					formatWarningWithSkill(result.skill.Name, line),
					5,
				)
			}
		}

		for _, line := range digest.otherAuditLines {
			switch {
			case strings.HasPrefix(line, "audit scan error"):
				summary.scanErrorSkillCount++
			case strings.HasPrefix(line, "audit skipped"):
				summary.skippedAuditSkillCount++
			default:
				summary.otherAuditLines = appendUniqueLimited(
					summary.otherAuditLines,
					formatWarningWithSkill(result.skill.Name, line),
					5,
				)
			}
		}

		for _, line := range digest.nonAuditLines {
			summary.nonAuditLines = append(summary.nonAuditLines, formatWarningWithSkill(result.skill.Name, line))
		}
	}

	return summary
}

func formatInstallSeverityCounts(counts map[string]int) string {
	parts := make([]string, 0, len(installAuditSeverityOrder))
	for _, severity := range installAuditSeverityOrder {
		if count := counts[severity]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", severity, count))
		}
	}
	return strings.Join(parts, ", ")
}

func topHighCriticalSkillsByCount(scoreBySkill map[string]int, limit int) []string {
	type skillScore struct {
		name  string
		score int
	}

	if limit <= 0 || len(scoreBySkill) == 0 {
		return nil
	}

	scores := make([]skillScore, 0, len(scoreBySkill))
	for name, score := range scoreBySkill {
		scores = append(scores, skillScore{name: name, score: score})
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].name < scores[j].name
		}
		return scores[i].score > scores[j].score
	})

	if len(scores) > limit {
		scores = scores[:limit]
	}

	top := make([]string, 0, len(scores))
	for _, item := range scores {
		top = append(top, fmt.Sprintf("%s(%d)", item.name, item.score))
	}
	return top
}

func renderBatchInstallWarningsCompact(results []skillInstallResult, totalWarnings int, hints ...string) {
	ui.Warning("%d warning(s) detected during install (compact batch view)", totalWarnings)

	summary := summarizeBatchInstallWarnings(results)

	for _, line := range summary.nonAuditLines {
		ui.Warning("%s", line)
	}

	if summary.totalFindings > 0 {
		ui.Warning("audit findings across %d skill(s): %s",
			summary.skillsWithFindings,
			formatInstallSeverityCounts(summary.findingCounts),
		)
	}
	if summary.belowThresholdSkillCount > 0 {
		ui.Warning("%d skill(s) had findings below the active block threshold", summary.belowThresholdSkillCount)
	}
	if summary.aboveThresholdSkillCount > 0 {
		ui.Warning("%d skill(s) had findings at/above threshold and continued due to --force", summary.aboveThresholdSkillCount)
	}
	if summary.scanErrorSkillCount > 0 {
		ui.Warning("%d skill(s) had audit scan errors", summary.scanErrorSkillCount)
	}
	if summary.skippedAuditSkillCount > 0 {
		ui.Warning("%d skill(s) skipped audit (--skip-audit)", summary.skippedAuditSkillCount)
	}

	if skillsWithHighCritical := len(summary.highCriticalBySkill); skillsWithHighCritical > 0 {
		ui.Warning("skills with HIGH/CRITICAL findings: %d", skillsWithHighCritical)
		top := topHighCriticalSkillsByCount(summary.highCriticalBySkill, 10)
		if len(top) > 5 {
			for _, item := range top {
				ui.Info("  %s", item)
			}
		} else if len(top) > 0 {
			ui.Info("top HIGH/CRITICAL: %s", strings.Join(top, ", "))
		}
	}

	for _, line := range summary.otherAuditLines {
		ui.Warning("%s", line)
	}

	if summary.totalFindings > 0 {
		hint := "suppressed %d audit finding line(s); re-run with --audit-verbose for full details"
		if len(hints) > 0 {
			hint = hints[0]
		}
		ui.Info(hint, summary.totalFindings)
	}
}

// printSkillListCompact prints a list of skills with compression for large lists.
// ≤20 skills: print each with SkillBoxCompact. >20: first 10 + "... and N more".
func printSkillListCompact(skills []install.SkillInfo) {
	const threshold = 20
	const showCount = 10

	if len(skills) <= threshold {
		for _, skill := range skills {
			ui.SkillBoxCompact(skill.Name, skill.Path)
		}
		return
	}

	for i := 0; i < showCount; i++ {
		ui.SkillBoxCompact(skills[i].Name, skills[i].Path)
	}
	ui.Info("... and %d more skill(s)", len(skills)-showCount)
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

		installResult, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			results = append(results, skillInstallResult{skill: skill, success: false, message: err.Error(), err: err})
			continue
		}

		if skill.Path == "." {
			rootInstalled = true
		}
		message := "installed"
		if len(installResult.Warnings) > 0 {
			message = fmt.Sprintf("installed (%d warning(s))", len(installResult.Warnings))
		}
		results = append(results, skillInstallResult{
			skill:    skill,
			success:  true,
			message:  message,
			warnings: installResult.Warnings,
		})
	}

	displayInstallResults(results, installSpinner, opts.AuditVerbose)

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

type auditBlockedFailureDigest struct {
	threshold    string
	findingCount int
	firstFinding string
}

func isAuditSeverityLine(line string) bool {
	for _, severity := range installAuditSeverityOrder {
		if strings.HasPrefix(line, severity+":") {
			return true
		}
	}
	return false
}

func parseAuditBlockedFailure(message string) auditBlockedFailureDigest {
	digest := auditBlockedFailureDigest{}
	if matches := installAuditThresholdPattern.FindStringSubmatch(message); len(matches) == 2 {
		digest.threshold = matches[1]
	}

	for _, rawLine := range strings.Split(message, "\n") {
		line := strings.TrimSpace(rawLine)
		if !isAuditSeverityLine(line) {
			continue
		}
		digest.findingCount++
		if digest.firstFinding == "" {
			digest.firstFinding = line
		}
	}

	return digest
}

func summarizeBlockedThreshold(failures []skillInstallResult) string {
	thresholds := map[string]bool{}
	for _, failure := range failures {
		digest := parseAuditBlockedFailure(failure.message)
		if digest.threshold != "" {
			thresholds[digest.threshold] = true
		}
	}

	if len(thresholds) == 0 {
		return "configured"
	}
	if len(thresholds) == 1 {
		for threshold := range thresholds {
			return threshold
		}
	}
	return "mixed"
}

func truncateForInstallSummary(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func blockedSkillLabel(name, threshold string) string {
	if !ui.IsTTY() {
		return name
	}
	color := riskColor(strings.ToLower(strings.TrimSpace(threshold)))
	if color == "" {
		color = ui.Red
	}
	return ui.Bold + color + name + ui.Reset
}

func formatBlockedThresholdLabel(threshold string) string {
	threshold = strings.TrimSpace(threshold)
	if threshold == "" {
		return "configured"
	}
	if threshold == "mixed" {
		return threshold
	}
	return formatSeverity(threshold)
}

func compactInstallFailureMessage(result skillInstallResult) string {
	if result.err != nil && errors.Is(result.err, audit.ErrBlocked) {
		digest := parseAuditBlockedFailure(result.message)

		parts := []string{"blocked by security audit"}
		if digest.threshold != "" && digest.findingCount > 0 {
			suffix := "findings"
			if digest.findingCount == 1 {
				suffix = "finding"
			}
			parts = append(parts, fmt.Sprintf("(%s, %d %s)", digest.threshold, digest.findingCount, suffix))
		} else if digest.threshold != "" {
			parts = append(parts, "("+digest.threshold+")")
		}

		if digest.firstFinding != "" {
			first := digest.firstFinding
			for _, severity := range installAuditSeverityOrder {
				prefix := severity + ": "
				if strings.HasPrefix(first, prefix) {
					first = strings.TrimPrefix(first, prefix)
					break
				}
			}
			parts = append(parts, truncateForInstallSummary(first, 110))
		}

		return strings.Join(parts, " ")
	}

	return firstWarningLine(result.message)
}

// renderInstallWarningsHighCriticalOnly prints only HIGH/CRITICAL findings
// verbosely, and summarizes remaining findings in one line.
func renderInstallWarningsHighCriticalOnly(skillName string, warnings []string) {
	digest := digestInstallWarnings(warnings)

	// Print non-audit warnings as-is
	for _, warning := range digest.nonAuditLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, warning))
	}

	// Print HIGH and CRITICAL findings verbosely
	highCritCount := 0
	for _, severity := range []string{audit.SeverityCritical, audit.SeverityHigh} {
		for _, line := range digest.findingByLevel[severity] {
			ui.Warning("%s", formatWarningWithSkill(skillName, line))
			highCritCount++
		}
	}

	// Summarize remaining findings (MEDIUM/LOW/INFO) in one line
	otherParts := make([]string, 0, 3)
	for _, severity := range []string{audit.SeverityMedium, audit.SeverityLow, audit.SeverityInfo} {
		if count := digest.findingCounts[severity]; count > 0 {
			otherParts = append(otherParts, fmt.Sprintf("%s=%d", severity, count))
		}
	}
	if len(otherParts) > 0 {
		ui.Info("%s", formatWarningWithSkill(skillName,
			fmt.Sprintf("also: %s (use 'skillshare check %s' for details)", strings.Join(otherParts, ", "), skillName)))
	}

	// Print status/other audit lines
	for _, line := range digest.statusLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, line))
	}
	for _, line := range digest.otherAuditLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, line))
	}
}

// countSkillsWithWarnings counts how many results have at least one warning.
func countSkillsWithWarnings(results []skillInstallResult) int {
	n := 0
	for _, r := range results {
		if len(r.warnings) > 0 {
			n++
		}
	}
	return n
}

// hasHighCriticalWarnings checks if a result contains HIGH or CRITICAL audit findings.
func hasHighCriticalWarnings(r skillInstallResult) bool {
	for _, w := range r.warnings {
		if strings.HasPrefix(w, "audit CRITICAL:") || strings.HasPrefix(w, "audit HIGH:") {
			return true
		}
	}
	return false
}

// sortResultsByHighCritical returns a copy sorted by HIGH/CRITICAL finding count descending.
func sortResultsByHighCritical(results []skillInstallResult) []skillInstallResult {
	sorted := make([]skillInstallResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		ci := countHighCritical(sorted[i])
		cj := countHighCritical(sorted[j])
		if ci != cj {
			return ci > cj
		}
		return sorted[i].skill.Name < sorted[j].skill.Name
	})
	return sorted
}

func countHighCritical(r skillInstallResult) int {
	n := 0
	for _, w := range r.warnings {
		if strings.HasPrefix(w, "audit CRITICAL:") || strings.HasPrefix(w, "audit HIGH:") {
			n++
		}
	}
	return n
}

// displayInstallResults shows the final install results
func displayInstallResults(results []skillInstallResult, spinner *ui.Spinner, auditVerbose bool) {
	var successes, failures []skillInstallResult
	totalWarnings := 0
	for _, r := range results {
		if r.success {
			successes = append(successes, r)
		} else {
			failures = append(failures, r)
		}
		totalWarnings += len(r.warnings)
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
		var blockedFailures, otherFailures []skillInstallResult
		for _, r := range failures {
			if r.err != nil && errors.Is(r.err, audit.ErrBlocked) {
				blockedFailures = append(blockedFailures, r)
				continue
			}
			otherFailures = append(otherFailures, r)
		}

		ui.SectionLabel("Blocked / Failed")
		if len(blockedFailures) > 0 && !auditVerbose {
			threshold := summarizeBlockedThreshold(blockedFailures)
			ui.Warning("%d skill(s) blocked by security audit (%s threshold)", len(blockedFailures), formatBlockedThresholdLabel(threshold))
			ui.Info("Use --force to continue blocked installs, or --skip-audit to bypass scanning for this run")
		}

		const blockedVerboseLimit = 20
		if auditVerbose && len(blockedFailures) > blockedVerboseLimit {
			// Large batch: summary line + first N verbose + rest compact
			threshold := summarizeBlockedThreshold(blockedFailures)
			ui.Warning("%d skill(s) blocked by security audit (%s threshold)", len(blockedFailures), formatBlockedThresholdLabel(threshold))
			ui.Info("Use --force to continue blocked installs, or --skip-audit to bypass scanning for this run")
			for i, r := range blockedFailures {
				digest := parseAuditBlockedFailure(r.message)
				if i < blockedVerboseLimit {
					ui.StepFail(blockedSkillLabel(r.skill.Name, digest.threshold), r.message)
				} else {
					ui.StepFail(blockedSkillLabel(r.skill.Name, digest.threshold), compactInstallFailureMessage(r))
				}
			}
			remaining := len(blockedFailures) - blockedVerboseLimit
			if remaining > 0 {
				ui.Info("%d more blocked skill(s) shown in compact form above", remaining)
			}
		} else {
			for _, r := range blockedFailures {
				digest := parseAuditBlockedFailure(r.message)
				msg := r.message
				if !auditVerbose {
					msg = compactInstallFailureMessage(r)
				}
				ui.StepFail(blockedSkillLabel(r.skill.Name, digest.threshold), msg)
			}
		}
		for _, r := range otherFailures {
			msg := r.message
			if !auditVerbose {
				msg = compactInstallFailureMessage(r)
			}
			ui.StepFail(r.skill.Name, msg)
		}
	}

	// Show successes — condensed when many
	if installed > 0 {
		ui.SectionLabel("Installed")
		switch {
		case installed > 50:
			ui.StepDone(fmt.Sprintf("%d skills installed", installed), "")
		case installed > 10:
			maxShown := 10
			names := make([]string, 0, maxShown)
			for i, r := range successes {
				if i >= maxShown {
					break
				}
				names = append(names, r.skill.Name)
			}
			detail := strings.Join(names, ", ")
			if installed > maxShown {
				detail = fmt.Sprintf("%s ... +%d more", detail, installed-maxShown)
			}
			ui.StepDone(fmt.Sprintf("%d skills installed", installed), detail)
		default:
			for _, r := range successes {
				ui.StepDone(r.skill.Name, r.message)
			}
		}
	}

	if totalWarnings > 0 {
		ui.SectionLabel("Audit Warnings")
		if auditVerbose {
			skillsWithWarnings := countSkillsWithWarnings(results)
			if skillsWithWarnings <= 20 {
				// Small batch: show full verbose detail per skill
				ui.Warning("%d warning(s) detected during install", totalWarnings)
				for _, r := range results {
					renderInstallWarnings(r.skill.Name, r.warnings, true)
				}
			} else {
				// Large batch: compact summary + only HIGH/CRITICAL findings from top skills
				renderBatchInstallWarningsCompact(results, totalWarnings,
					"%d audit finding line(s) across all skills; HIGH/CRITICAL detail expanded below")
				fmt.Println()
				ui.Warning("HIGH/CRITICAL detail (top skills):")
				shown := 0
				for _, r := range sortResultsByHighCritical(results) {
					if shown >= 20 || !hasHighCriticalWarnings(r) {
						break
					}
					renderInstallWarningsHighCriticalOnly(r.skill.Name, r.warnings)
					shown++
				}
				remaining := skillsWithWarnings - shown
				if remaining > 0 {
					ui.Info("%d more skill(s) with findings; use 'skillshare check <name>' for details", remaining)
				}
			}
		} else {
			renderBatchInstallWarningsCompact(results, totalWarnings)
		}
	}

	if installed > 0 {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}
}

func handleGitSubdirInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source:         source.Raw,
		DryRun:         opts.DryRun,
		Into:           opts.Into,
		SkipAudit:      opts.SkipAudit,
		AuditVerbose:   opts.AuditVerbose,
		AuditThreshold: opts.AuditThreshold,
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
	progressMsg := "Cloning repository..."
	if source.GitHubOwner() != "" && source.GitHubRepo() != "" {
		progressMsg = "Downloading via GitHub API..."
	}
	treeSpinner := ui.StartTreeSpinner(progressMsg, false)
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			treeSpinner.Update(line)
		}
	}

	// Discover skills in subdir
	discovery, err := install.DiscoverFromGitSubdirWithProgress(source, opts.OnProgress)
	if err != nil {
		treeSpinner.Fail("Failed to clone")
		return logSummary, err
	}
	defer install.CleanupDiscovery(discovery)

	treeSpinner.Success("Cloned")
	for _, w := range discovery.Warnings {
		ui.Warning("%s", w)
	}

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

		loc := skill.Path
		if loc == "." {
			loc = "root"
		}
		fmt.Println()
		desc := ""
		if skill.License != "" {
			desc = "License: " + skill.License
		}
		ui.SkillBox(skill.Name, desc, loc)

		destPath := destWithInto(cfg.Source, opts, skill.Name)
		if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
			return logSummary, fmt.Errorf("failed to create --into directory: %w", err)
		}

		fmt.Println()
		installSpinner := ui.StartSpinner(fmt.Sprintf("Installing %s...", skill.Name))

		result, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			installSpinner.Fail("Failed to install")
			if errors.Is(err, audit.ErrBlocked) {
				return logSummary, renderBlockedAuditError(err)
			}
			return logSummary, err
		}

		if opts.DryRun {
			installSpinner.Stop()
			ui.Warning("[dry-run] %s", result.Action)
		} else {
			installSpinner.Success(fmt.Sprintf("Installed: %s", skill.Name))
		}

		renderInstallWarnings("", result.Warnings, opts.AuditVerbose)

		if !opts.DryRun {
			ui.SectionLabel("Next Steps")
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

	// Apply --exclude early so excluded skills never appear in prompts
	if len(opts.Exclude) > 0 {
		discovery.Skills = applyExclude(discovery.Skills, opts.Exclude)
		if len(discovery.Skills) == 0 {
			ui.Info("All skills were excluded")
			return logSummary, nil
		}
	}

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
			printSkillListCompact(selected)
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
		printSkillListCompact(discovery.Skills)
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
		Source:         source.Raw,
		DryRun:         opts.DryRun,
		Into:           opts.Into,
		SkipAudit:      opts.SkipAudit,
		AuditVerbose:   opts.AuditVerbose,
		AuditThreshold: opts.AuditThreshold,
	}

	// Warn about inapplicable flags
	if len(opts.Exclude) > 0 {
		ui.Warning("--exclude is only supported for multi-skill repos; ignored for direct install")
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
	if source.IsGit() && ui.IsTTY() {
		opts.OnProgress = func(line string) {
			treeSpinner.Update(line)
		}
	}

	// Execute installation
	result, err := install.Install(source, destPath, opts)
	if err != nil {
		treeSpinner.Fail("Failed to install")
		if errors.Is(err, audit.ErrBlocked) {
			return logSummary, renderBlockedAuditError(err)
		}
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
	renderInstallWarnings("", result.Warnings, opts.AuditVerbose)

	// Show next steps
	if !opts.DryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute to all targets")
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, skillName)
		logSummary.SkillCount = len(logSummary.InstalledSkills)
	}

	return logSummary, nil
}

func installFromGlobalConfig(cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	summary := installLogSummary{
		Mode:         "global",
		Source:       "global-config",
		DryRun:       opts.DryRun,
		AuditVerbose: opts.AuditVerbose,
	}

	if len(cfg.Skills) == 0 {
		ui.Info("No remote skills defined in config.yaml")
		ui.Info("Install a skill first: skillshare install <source>")
		return summary, nil
	}

	ui.Logo(appversion.Version)

	total := len(cfg.Skills)
	spinner := ui.StartSpinner(fmt.Sprintf("Installing %d skill(s) from config...", total))

	installed := 0

	for _, skill := range cfg.Skills {
		groupDir, bareName := skill.EffectiveParts()
		if strings.TrimSpace(bareName) == "" {
			continue
		}

		displayName := skill.FullName()
		destPath := filepath.Join(cfg.Source, filepath.FromSlash(displayName))
		if _, err := os.Stat(destPath); err == nil {
			ui.StepDone(displayName, "skipped (already exists)")
			continue
		}

		source, err := install.ParseSource(skill.Source)
		if err != nil {
			ui.StepFail(displayName, fmt.Sprintf("invalid source: %v", err))
			continue
		}

		source.Name = bareName

		if skill.Tracked {
			trackOpts := opts
			if groupDir != "" {
				trackOpts.Into = groupDir
			}
			trackedResult, err := install.InstallTrackedRepo(source, cfg.Source, trackOpts)
			if err != nil {
				ui.StepFail(displayName, err.Error())
				continue
			}
			if opts.DryRun {
				ui.StepDone(displayName, trackedResult.Action)
				continue
			}
			ui.StepDone(displayName, fmt.Sprintf("installed (tracked, %d skills)", trackedResult.SkillCount))
			if len(trackedResult.Skills) > 0 {
				summary.InstalledSkills = append(summary.InstalledSkills, trackedResult.Skills...)
			} else {
				summary.InstalledSkills = append(summary.InstalledSkills, displayName)
			}
		} else {
			if err := validate.SkillName(bareName); err != nil {
				ui.StepFail(displayName, fmt.Sprintf("invalid name: %v", err))
				continue
			}
			// Ensure group directory exists
			if groupDir != "" {
				if err := os.MkdirAll(filepath.Join(cfg.Source, filepath.FromSlash(groupDir)), 0755); err != nil {
					ui.StepFail(displayName, fmt.Sprintf("failed to create group directory: %v", err))
					continue
				}
			}
			result, err := install.Install(source, destPath, opts)
			if err != nil {
				ui.StepFail(displayName, err.Error())
				continue
			}
			if opts.DryRun {
				ui.StepDone(displayName, result.Action)
				continue
			}
			ui.StepDone(displayName, "installed")
			summary.InstalledSkills = append(summary.InstalledSkills, displayName)
		}

		installed++
	}

	if opts.DryRun {
		spinner.Stop()
		summary.SkillCount = len(summary.InstalledSkills)
		return summary, nil
	}

	spinner.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to distribute to all targets")
	summary.SkillCount = len(summary.InstalledSkills)

	if installed > 0 {
		if err := config.ReconcileGlobalSkills(cfg); err != nil {
			return summary, err
		}
	}

	return summary, nil
}

func printInstallHelp() {
	fmt.Println(`Usage: skillshare install [source|skill-name] [options]

Install skills from a local path, git repository, or global config.
When run with no arguments, installs all skills listed in config.yaml.
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
  --force, -f         Overwrite existing skill; also continue if audit would block
  --update, -u        Update existing (git pull if possible, else reinstall)
  --track, -t         Install as tracked repo (preserves .git for updates)
  --skill, -s <names> Select specific skills from multi-skill repo (comma-separated)
  --exclude <names>   Skip specific skills during install (comma-separated)
  --all               Install all discovered skills without prompting
  --yes, -y           Auto-accept all prompts (equivalent to --all for multi-skill repos)
  --dry-run, -n       Preview the installation without making changes
  --skip-audit        Skip security audit entirely for this install
  --audit-verbose     Show full audit finding lines (default: compact summary)
  --audit-threshold, --threshold, -T <t>
                      Block install by severity at/above: critical|high|medium|low|info
                      (also supports c|h|m|l|i)
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare install anthropics/skills
  skillshare install anthropics/skills/skills/pdf
  skillshare install ComposioHQ/awesome-claude-skills
  skillshare install ~/my-skill
  skillshare install github.com/user/repo --force
  skillshare install ~/my-skill --skip-audit     # Bypass scan (no findings generated)
  skillshare install user/repo --all --audit-verbose
  skillshare install ~/my-skill -T high          # Override block threshold for this run

Selective install (non-interactive):
  skillshare install anthropics/skills -s pdf,commit     # Specific skills
  skillshare install anthropics/skills --all             # All skills
  skillshare install anthropics/skills -y                # Auto-accept
  skillshare install anthropics/skills -s pdf --dry-run  # Preview selection
  skillshare install repo --all --exclude cli-sentry     # All except specific

Organize into subdirectories:
  skillshare install anthropics/skills -s pdf --into frontend
  skillshare install user/repo --track --into devops
  skillshare install ~/my-skill --into frontend/react

Tracked repositories (Team Edition):
  skillshare install team/shared-skills --track   # Clone as _shared-skills
  skillshare install _shared-skills --update      # Update tracked repo

Install from config (no arguments):
  skillshare install                         # Install all skills from config.yaml
  skillshare install --dry-run               # Preview config-based install

Update existing skills:
  skillshare install my-skill --update       # Update using stored source
  skillshare install my-skill --force        # Reinstall using stored source
  skillshare install my-skill --update -n    # Preview update`)
}
