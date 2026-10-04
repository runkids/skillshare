package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// uninstallOptions holds parsed arguments for uninstall command
type uninstallOptions struct {
	skillNames []string           // positional args (0+)
	groups     []string           // --group/-G values (repeatable)
	kind       resourceKindFilter // set by positional filter (e.g. "uninstall agents")
	all        bool               // --all: remove ALL skills from source
	force      bool
	dryRun     bool
	jsonOutput bool
}

// uninstallJSONOutput is the JSON representation for uninstall --json output.
type uninstallJSONOutput struct {
	Removed  []string `json:"removed"`
	Failed   []string `json:"failed"`
	Skipped  int      `json:"skipped"`
	DryRun   bool     `json:"dry_run"`
	Duration string   `json:"duration"`
}

// uninstallTarget holds resolved target information
type uninstallTarget struct {
	name          string
	path          string
	isTrackedRepo bool
}

type uninstallTypeSummary struct {
	skills          int
	groups          int
	trackedRepos    int
	groupSkillCount map[string]int // key: target.path
}

// parseUninstallArgs parses command line arguments
func parseUninstallArgs(args []string) (*uninstallOptions, bool, error) {
	opts := &uninstallOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all":
			opts.all = true
		case arg == "--force" || arg == "-f":
			opts.force = true
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--json":
			opts.jsonOutput = true
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

	// --all mutual exclusion checks
	if opts.all {
		if len(opts.skillNames) > 0 {
			return nil, false, fmt.Errorf("--all cannot be used with skill names")
		}
		if len(opts.groups) > 0 {
			return nil, false, fmt.Errorf("--all cannot be used with --group")
		}
		return opts, false, nil
	}

	if len(opts.skillNames) == 0 && len(opts.groups) == 0 {
		return nil, true, fmt.Errorf("skill name or --group is required")
	}

	return opts, false, nil
}

// topLevelDir returns the first path component of a relative path.
// e.g. "frontend/react/hooks" → "frontend", "my-skill" → "my-skill"
func topLevelDir(relPath string) string {
	parts := strings.SplitN(relPath, "/", 2)
	return parts[0]
}

// normalizeUninstallName converts OS-native separators to the slash-separated
// logical names used by metadata and trash entries.
func normalizeUninstallName(name string) string {
	return normalizeUninstallNameForSeparator(name, os.PathSeparator)
}

func normalizeUninstallNameForSeparator(name string, separator uint8) string {
	if separator == '/' {
		return name
	}
	return strings.ReplaceAll(name, string(separator), "/")
}

// looksLikeShellGlob detects when positional args appear to be shell-expanded
// file names rather than real skill names. Heuristic: ≥3 warnings, warnings ≥50%
// of names, and ≥2 names contain a dot (file extension characteristic).
func looksLikeShellGlob(names []string, warnings []string) bool {
	if len(warnings) < 3 || len(warnings)*2 < len(names) {
		return false
	}
	dotCount := 0
	for _, name := range names {
		if strings.Contains(name, ".") {
			dotCount++
		}
	}
	return dotCount >= 2
}

// resolveUninstallTarget resolves skill name to path and checks existence.
// Supports short names for nested skills (e.g. "react-best-practices" resolves
// to "frontend/react/react-best-practices"). sourceLabel names sourceDir in
// not-found errors.
func resolveUninstallTarget(skillName, sourceDir, sourceLabel string) (*uninstallTarget, error) {
	skillName = strings.TrimRight(strings.TrimSpace(skillName), `/\`)
	skillName = normalizeUninstallName(skillName)
	if skillName == "" || skillName == "." {
		return nil, fmt.Errorf("invalid skill name: %q", skillName)
	}

	// Normalize _ prefix for tracked repos
	if !strings.HasPrefix(skillName, "_") {
		prefixedPath := filepath.Join(sourceDir, "_"+skillName)
		if install.IsGitRepo(prefixedPath) {
			skillName = "_" + skillName
		}
	}

	skillPath := filepath.Join(sourceDir, skillName)
	info, err := os.Stat(skillPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Fallback: search by basename in nested directories
			resolved, resolveErr := resolveNestedSkillDir(sourceDir, skillName, sourceLabel)
			if resolveErr != nil {
				return nil, resolveErr
			}
			skillName = resolved
			skillPath = filepath.Join(sourceDir, resolved)
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

// resolveUninstallByGlob scans the source directory for top-level entries
// whose names match the given glob pattern (e.g. "core-*", "_team-?").
func resolveUninstallByGlob(pattern, sourceDir string) ([]*uninstallTarget, error) {
	entries, err := sourcewalk.ReadDir(sourceDir, sourcewalk.Options{})
	if err != nil {
		return nil, fmt.Errorf("cannot read source directory: %w", err)
	}

	var targets []*uninstallTarget
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if matchGlob(pattern, e.Name()) {
			skillPath := filepath.Join(sourceDir, e.Name())
			targets = append(targets, &uninstallTarget{
				name:          e.Name(),
				path:          skillPath,
				isTrackedRepo: install.IsGitRepo(skillPath),
			})
		}
	}
	return targets, nil
}

// resolveGroupSkills finds all skills under a group directory (prefix match).
// Returns uninstallTargets for each skill found.
func resolveGroupSkills(group, sourceDir string) ([]*uninstallTarget, error) {
	group = strings.TrimRight(strings.TrimSpace(group), `/\`)
	group = normalizeUninstallName(group)
	if group == "" || group == "." {
		return nil, fmt.Errorf("invalid group name: %q", group)
	}
	groupPath := filepath.Join(sourceDir, group)

	info, err := os.Stat(groupPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("group '%s' not found in source", group)
	}

	walkRoot := utils.ResolveSymlink(groupPath)
	resolvedSourceDir := utils.ResolveSymlink(sourceDir)

	// Guard: walkRoot must be inside resolvedSourceDir to prevent
	// symlinked groups from reaching outside the source tree.
	if srcRel, relErr := filepath.Rel(resolvedSourceDir, walkRoot); relErr != nil || strings.HasPrefix(srcRel, "..") {
		return nil, fmt.Errorf("group '%s' resolves outside source directory", group)
	}

	var targets []*uninstallTarget
	if walkErr := filepath.Walk(walkRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == walkRoot || !fi.IsDir() {
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
			rel, relErr := filepath.Rel(resolvedSourceDir, path)
			if relErr == nil && !strings.HasPrefix(rel, "..") {
				targets = append(targets, &uninstallTarget{
					name:          normalizeUninstallName(rel),
					path:          path,
					isTrackedRepo: isRepo,
				})
			}
			return filepath.SkipDir // don't descend into skill dirs
		}
		return nil
	}); walkErr != nil {
		return nil, fmt.Errorf("failed to walk group '%s': %w", group, walkErr)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no skills found in group '%s'", group)
	}

	return targets, nil
}

// resolveNestedSkillDir searches for a skill directory by basename within
// nested organizational folders. Also matches _name variant for tracked repos.
// Returns the relative path from sourceDir, or an error listing all matches
// when the name is ambiguous.
func resolveNestedSkillDir(sourceDir, name, sourceLabel string) (string, error) {
	var matches []string

	walkRoot := utils.ResolveSymlink(sourceDir)
	if walkErr := sourcewalk.Walk(walkRoot, sourcewalk.Options{}, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == walkRoot || !info.IsDir() {
			return nil
		}
		if info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.Name() == name || info.Name() == "_"+name {
			if rel, relErr := filepath.Rel(walkRoot, path); relErr == nil && rel != "." {
				matches = append(matches, normalizeUninstallName(rel))
			}
			return filepath.SkipDir
		}
		return nil
	}); walkErr != nil {
		return "", fmt.Errorf("failed to search for skill '%s': %w", name, walkErr)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("skill '%s' not found in %s", name, sourceLabel)
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

// countGroupSkills counts sub-skills inside a directory (non-recursive per skill).
// Returns the list of relative skill names found, or nil if not a group.
func countGroupSkills(dir string) []string {
	var names []string
	walkRoot := utils.ResolveSymlink(dir)
	_ = filepath.Walk(walkRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == walkRoot || !fi.IsDir() {
			return nil
		}
		if fi.Name() == ".git" {
			return filepath.SkipDir
		}
		if _, statErr := os.Stat(filepath.Join(path, "SKILL.md")); statErr == nil {
			names = append(names, fi.Name())
			return filepath.SkipDir
		}
		if install.IsGitRepo(path) {
			names = append(names, fi.Name())
			return filepath.SkipDir
		}
		return nil
	})
	return names
}

func summarizeUninstallTargets(targets []*uninstallTarget) uninstallTypeSummary {
	s := uninstallTypeSummary{
		groupSkillCount: make(map[string]int, len(targets)),
	}

	for _, t := range targets {
		if t.isTrackedRepo {
			s.trackedRepos++
			continue
		}

		subSkills := countGroupSkills(t.path)
		if len(subSkills) > 0 {
			s.groups++
			s.groupSkillCount[t.path] = len(subSkills)
			continue
		}

		s.skills++
	}

	return s
}

func (s uninstallTypeSummary) noun() string {
	total := s.skills + s.groups + s.trackedRepos
	switch {
	case s.groups == total:
		return fmt.Sprintf("group%s", pluralS(total))
	case s.skills == total:
		return fmt.Sprintf("skill%s", pluralS(total))
	case s.trackedRepos == total:
		return fmt.Sprintf("tracked repo%s", pluralS(total))
	default:
		return fmt.Sprintf("item%s", pluralS(total))
	}
}

// displayUninstallInfo names the target, where it lives and what it holds.
func displayUninstallInfo(target *uninstallTarget) {
	fmt.Printf("  %s  %s\n", target.name, ui.DimText(utils.FoldHomePath(target.path)+" · "+uninstallTargetKind(target)))
}

// uninstallTargetKind describes what a target holds: a tracked repository,
// a group of skills or a skill's files.
func uninstallTargetKind(target *uninstallTarget) string {
	if target.isTrackedRepo {
		return "tracked repository"
	}
	if n := len(countGroupSkills(target.path)); n > 0 {
		return "group, " + plural(n, "skill")
	}
	files := 0
	filepath.WalkDir(target.path, func(_ string, d os.DirEntry, err error) error { //nolint:errcheck
		if err == nil && !d.IsDir() {
			files++
		}
		return nil
	})
	return plural(files, "file")
}

// gitStatusError marks a tracked repo whose git status could not be read.
type gitStatusError struct{ err error }

func (e *gitStatusError) Error() string { return fmt.Sprintf("failed to check git status: %v", e.err) }
func (e *gitStatusError) Unwrap() error { return e.err }

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

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	// Extract kind filter (e.g. "skillshare uninstall agents myagent").
	kind, rest := parseKindArg(rest)

	if mode == modeProject {
		if kind == kindAgents {
			projectCfg, loadErr := config.LoadProject(cwd)
			if loadErr != nil {
				return fmt.Errorf("failed to load project config: %w", loadErr)
			}
			agentsDir := projectCfg.EffectiveAgentsSource(cwd)
			opts, _, _ := parseUninstallArgs(rest)
			if opts == nil {
				opts = &uninstallOptions{skillNames: rest}
			}
			opts.force = opts.force || opts.jsonOutput
			err := cmdUninstallAgents(agentsDir, opts, config.ProjectConfigPath(cwd), trash.ProjectAgentTrashDir(cwd), start)
			return err
		}
		return cmdUninstallProject(rest, cwd)
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
		if opts.jsonOutput {
			return writeJSONError(fmt.Errorf("failed to load config: %w", err))
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Agent-only uninstall: move .md + sidecar to agent trash, then return.
	if kind == kindAgents {
		opts.force = opts.force || opts.jsonOutput
		agentsDir := cfg.EffectiveAgentsSource()
		err := cmdUninstallAgents(agentsDir, opts, config.ConfigPath(), trash.AgentTrashDir(), start)
		return err
	}

	// Load centralized metadata store for display/reinstall hints.
	sourceDir := cfg.EffectiveSkillsSource()
	skillsStore, _ := install.LoadMetadataWithMigration(sourceDir, "")
	if skillsStore == nil {
		skillsStore = install.NewMetadataStore()
	}

	targetNames := targetNamesFromConfig(cfg.Targets)
	sort.Strings(targetNames)
	skillsMode := &uninstallMode{
		sourceDir:      sourceDir,
		sourceLabel:    "source",
		trashDir:       trash.TrashDir(),
		configPath:     config.ConfigPath(),
		store:          skillsStore,
		emptySourceErr: "no skills found in source",
		globs:          true,
		targetNames:    targetNames,
		dryRunGitignore: func(t *uninstallTarget) string {
			if t.isTrackedRepo {
				return fmt.Sprintf("would remove %s from .gitignore", t.name)
			}
			return ""
		},
		// Only tracked repos are gitignored in the global source.
		gitignoreEntries: func(succeeded []*uninstallTarget) (string, []string) {
			var entries []string
			for _, t := range succeeded {
				if t.isTrackedRepo {
					entries = append(entries, t.name)
				}
			}
			return sourceDir, entries
		},
		preflight: globalUninstallPreflight,
	}
	return runUninstallSkills(opts, skillsMode, rest, start)
}

// uninstallOutputJSON converts uninstall results to JSON and writes to stdout.
func uninstallOutputJSON(removed, failed []string, skipped int, dryRun bool, start time.Time, uninstallErr error) error {
	output := uninstallJSONOutput{
		Removed:  removed,
		Failed:   failed,
		Skipped:  skipped,
		DryRun:   dryRun,
		Duration: formatDuration(start),
	}
	return writeJSONResult(&output, uninstallErr)
}

// uninstallOpNames parses raw args to build a clean oplog names list,
// filtering out flags like --force so only skill names and semantic markers appear.
func uninstallOpNames(args []string) []string {
	opts, _, _ := parseUninstallArgs(args)
	if opts == nil {
		return args // fallback: return raw args if parsing fails
	}
	var names []string
	if opts.all {
		names = append(names, "--all")
	}
	names = append(names, opts.skillNames...)
	for _, g := range opts.groups {
		names = append(names, "--group="+g)
	}
	return names
}

func logUninstallOp(cfgPath string, names []string, succeeded int, start time.Time, cmdErr error) {
	status := statusFromErr(cmdErr)
	if succeeded > 0 && succeeded < len(names) {
		status = "partial"
	}
	e := oplog.NewEntry("uninstall", status, time.Since(start))
	if len(names) == 1 {
		e.Args = map[string]any{"name": names[0]}
	} else if len(names) > 1 {
		e.Args = map[string]any{"names": names}
	}
	if succeeded > 0 && e.Args != nil {
		e.Args["succeeded"] = succeeded
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func printUninstallHelp() {
	printHelp("skillshare uninstall <name>... [options]\n       skillshare uninstall [agents] <name|--all> [options]\n       skillshare uninstall --group <group> [options]\n       skillshare uninstall --all [options]", "Remove one or more skills or tracked repositories from the source directory.\nSkills are moved to trash and kept for 7 days before automatic cleanup.\nIf the skill was installed from a remote source, a reinstall command is shown.\n\nFor tracked repositories (_repo-name):\n  - Checks for uncommitted changes (requires --force to override)\n  - Fails if git status cannot be read (requires --force to override)\n  - Automatically removes the entry from .gitignore\n  - The _ prefix is optional (automatically detected)\n\nSkill names support glob patterns (e.g. \"core-*\", \"test-?\").",
		helpGroup{title: "Options", rows: []helpRow{
			{"--all", "Remove ALL skills from source (requires confirmation)"},
			{"-G, --group <name>", "Remove all skills in a group (prefix match, repeatable)"},
			{"-f, --force", "Skip confirmation and ignore uncommitted changes"},
			{"-n, --dry-run", "Preview without making changes"},
			{"--json", "Global mode: output JSON and skip confirmation"},
			{"-p, --project", "Use project-level config in current directory"},
			{"-g, --global", "Use global config (~/.config/skillshare)"},
		}},
		helpExamples(
			helpRow{"skillshare uninstall my-skill", "Remove a single skill"},
			helpRow{"skillshare uninstall a b c --force", "Remove multiple skills at once"},
			helpRow{"skillshare uninstall \"core-*\"", "Remove all matching a glob pattern"},
			helpRow{"skillshare uninstall --all", "Remove all skills"},
			helpRow{"skillshare uninstall --all --force", "Remove all without confirmation"},
			helpRow{"skillshare uninstall --all -n", "Preview what would be removed"},
			helpRow{"skillshare uninstall --group frontend", "Remove all skills in frontend/"},
			helpRow{"skillshare uninstall --group frontend -n", "Preview group removal"},
			helpRow{"skillshare uninstall x -G backend --force", "Mix names and groups"},
			helpRow{"skillshare uninstall _team-repo", "Remove tracked repository"},
			helpRow{"skillshare uninstall team-repo", "_ prefix is optional"},
			helpRow{"skillshare uninstall agents tutor", "Uninstall an agent"},
			helpRow{"skillshare uninstall agents --all", "Uninstall all agents"},
			helpRow{"skillshare uninstall agents -G demo", "Uninstall all agents in demo/"},
		),
	)
}
