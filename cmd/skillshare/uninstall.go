package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

// uninstallOptions holds parsed arguments for uninstall command
type uninstallOptions struct {
	skillNames []string // positional args (0+)
	groups     []string // --group/-G values (repeatable)
	all        bool     // --all: remove ALL skills from source
	force      bool
	dryRun     bool
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
// to "frontend/react/react-best-practices").
func resolveUninstallTarget(skillName string, cfg *config.Config) (*uninstallTarget, error) {
	skillName = strings.TrimRight(strings.TrimSpace(skillName), `/\`)
	if skillName == "" || skillName == "." {
		return nil, fmt.Errorf("invalid skill name: %q", skillName)
	}

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
	group = strings.TrimRight(strings.TrimSpace(group), `/\`)
	if group == "" || group == "." {
		return nil, fmt.Errorf("invalid group name: %q", group)
	}
	groupPath := filepath.Join(sourceDir, group)

	info, err := os.Stat(groupPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("group '%s' not found in source", group)
	}

	var targets []*uninstallTarget
	if walkErr := filepath.Walk(groupPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == groupPath || !fi.IsDir() {
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
func resolveNestedSkillDir(sourceDir, name string) (string, error) {
	var matches []string

	if walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == sourceDir || !info.IsDir() {
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
	}); walkErr != nil {
		return "", fmt.Errorf("failed to search for skill '%s': %w", name, walkErr)
	}

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

// countGroupSkills counts sub-skills inside a directory (non-recursive per skill).
// Returns the list of relative skill names found, or nil if not a group.
func countGroupSkills(dir string) []string {
	var names []string
	_ = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dir || !fi.IsDir() {
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
		return fmt.Sprintf("target%s", pluralS(total))
	}
}

func (s uninstallTypeSummary) isMixed() bool {
	types := 0
	if s.skills > 0 {
		types++
	}
	if s.groups > 0 {
		types++
	}
	if s.trackedRepos > 0 {
		types++
	}
	return types > 1
}

func (s uninstallTypeSummary) details() string {
	var parts []string
	if s.skills > 0 {
		parts = append(parts, fmt.Sprintf("%d skill%s", s.skills, pluralS(s.skills)))
	}
	if s.groups > 0 {
		parts = append(parts, fmt.Sprintf("%d group%s", s.groups, pluralS(s.groups)))
	}
	if s.trackedRepos > 0 {
		parts = append(parts, fmt.Sprintf("%d tracked repo%s", s.trackedRepos, pluralS(s.trackedRepos)))
	}
	return strings.Join(parts, ", ")
}

// displayUninstallInfo shows information about the skill to be uninstalled
func displayUninstallInfo(target *uninstallTarget) {
	if target.isTrackedRepo {
		ui.Header("Uninstalling tracked repository")
		ui.Info("Type: tracked repository")
	} else {
		// Check if this is a group directory containing sub-skills
		subSkills := countGroupSkills(target.path)
		if len(subSkills) > 0 {
			ui.Header(fmt.Sprintf("Uninstalling group (%d skills)", len(subSkills)))
			for _, s := range subSkills {
				fmt.Printf("  - %s\n", s)
			}
		} else {
			ui.Header("Uninstalling skill")
		}
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
	} else if len(countGroupSkills(target.path)) > 0 {
		prompt = "Are you sure you want to uninstall this group?"
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

// performUninstallQuiet moves the skill to trash and cleans up without printing output.
// Used by batch mode; returns the type label for StepDone display.
func performUninstallQuiet(target *uninstallTarget, cfg *config.Config) (typeLabel string, err error) {
	groupSkillCount := 0
	if !target.isTrackedRepo {
		groupSkillCount = len(countGroupSkills(target.path))
	}

	// For tracked repos, clean up .gitignore
	if target.isTrackedRepo {
		install.RemoveFromGitIgnore(cfg.Source, target.name) //nolint:errcheck
	}

	if _, err := trash.MoveToTrash(target.path, target.name, trash.TrashDir()); err != nil {
		return "", fmt.Errorf("failed to move to trash: %w", err)
	}

	if target.isTrackedRepo {
		return "tracked repo", nil
	}
	if groupSkillCount > 0 {
		return fmt.Sprintf("group, %d skill%s", groupSkillCount, pluralS(groupSkillCount)), nil
	}
	return "skill", nil
}

// performUninstall moves the skill to trash and cleans up (verbose single-target output)
func performUninstall(target *uninstallTarget, cfg *config.Config) error {
	// Read metadata before moving (for reinstall hint)
	meta, _ := install.ReadMeta(target.path)
	groupSkillCount := 0
	if !target.isTrackedRepo {
		groupSkillCount = len(countGroupSkills(target.path))
	}

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
	} else if groupSkillCount > 0 {
		ui.Success("Uninstalled group: %s", target.name)
	} else {
		ui.Success("Uninstalled skill: %s", target.name)
	}
	ui.Info("Moved to trash (7 days): %s", trashPath)
	if meta != nil && meta.Source != "" {
		ui.Info("Reinstall: skillshare install %s", meta.Source)
	}
	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to update all targets")

	// Opportunistic cleanup of expired trash items
	if n, _ := trash.Cleanup(trash.TrashDir(), 0); n > 0 {
		ui.Info("Cleaned up %d expired trash item%s", n, pluralS(n))
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
		logUninstallOp(config.ProjectConfigPath(cwd), uninstallOpNames(rest), 0, start, err)
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

	if opts.all {
		sp := ui.StartSpinner("Discovering skills...")
		discovered, _, err := sync.DiscoverSourceSkillsLite(cfg.Source)
		if err != nil {
			sp.Fail("Discovery failed")
			return fmt.Errorf("failed to discover skills: %w", err)
		}
		sp.Success(fmt.Sprintf("Found %d skills", len(discovered)))
		if len(discovered) == 0 {
			return fmt.Errorf("no skills found in source")
		}
		// Collect unique top-level directories to avoid nested skill duplication
		topDirs := map[string]bool{}
		for _, d := range discovered {
			topDirs[topLevelDir(d.RelPath)] = true
		}
		for dir := range topDirs {
			skillPath := filepath.Join(cfg.Source, dir)
			targets = append(targets, &uninstallTarget{
				name:          dir,
				path:          skillPath,
				isTrackedRepo: install.IsGitRepo(skillPath),
			})
			seen[skillPath] = true
		}
	}

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

	// Shell glob detection: if positional args look like shell-expanded filenames,
	// intercept early and suggest --all instead
	if !opts.all && looksLikeShellGlob(opts.skillNames, resolveWarnings) {
		ui.Warning("It looks like '*' was expanded by your shell into file names.")
		ui.Info("To uninstall all skills, use: skillshare uninstall --all")
		return fmt.Errorf("shell glob expansion detected")
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
	summary := summarizeUninstallTargets(targets)
	if single {
		displayUninstallInfo(targets[0])
	} else {
		ui.Header(fmt.Sprintf("Uninstalling %d %s", len(targets), summary.noun()))
		if len(targets) > 20 {
			// Compressed: only list non-skill items (groups, tracked repos) individually
			ui.Info("Includes: %s", summary.details())
			for _, t := range targets {
				if t.isTrackedRepo {
					fmt.Printf("  - %s (tracked repository)\n", t.name)
				} else if c := summary.groupSkillCount[t.path]; c > 0 {
					fmt.Printf("  - %s (group, %d skill%s)\n", t.name, c, pluralS(c))
				}
			}
			if summary.skills > 0 {
				fmt.Printf("  ... and %d skill%s\n", summary.skills, pluralS(summary.skills))
			}
		} else {
			if summary.isMixed() {
				ui.Info("Includes: %s", summary.details())
			}
			for _, t := range targets {
				label := t.name
				if t.isTrackedRepo {
					label += " (tracked repository)"
				} else if c := summary.groupSkillCount[t.path]; c > 0 {
					label += fmt.Sprintf(" (group, %d skill%s)", c, pluralS(c))
				} else {
					label += " (skill)"
				}
				fmt.Printf("  - %s\n", label)
			}
		}
		fmt.Println()
	}

	// --- Phase 4: PRE-FLIGHT ---
	var preflightSkipped int
	if !opts.dryRun {
		// Parallel git dirty checks for tracked repos
		type dirtyResult struct {
			dirty bool
			err   error
		}
		dirtyResults := make(map[int]dirtyResult) // index → result

		// Collect tracked repo indices
		var trackedIndices []int
		for i, t := range targets {
			if t.isTrackedRepo {
				trackedIndices = append(trackedIndices, i)
			}
		}

		if len(trackedIndices) > 0 {
			const maxDirtyWorkers = 8
			results := make([]dirtyResult, len(trackedIndices))
			sem := make(chan struct{}, maxDirtyWorkers)
			var wg gosync.WaitGroup

			for j, idx := range trackedIndices {
				wg.Add(1)
				sem <- struct{}{}
				go func(slot int, t *uninstallTarget) {
					defer wg.Done()
					defer func() { <-sem }()
					dirty, err := isRepoDirty(t.path)
					results[slot] = dirtyResult{dirty: dirty, err: err}
				}(j, targets[idx])
			}
			wg.Wait()

			for j, idx := range trackedIndices {
				dirtyResults[idx] = results[j]
			}
		}

		var preflight []*uninstallTarget
		for i, t := range targets {
			if !t.isTrackedRepo {
				preflight = append(preflight, t)
				continue
			}
			dr := dirtyResults[i]
			if dr.err != nil {
				ui.Warning("Could not check git status for %s: %v", t.name, dr.err)
				preflight = append(preflight, t)
				continue
			}
			if !dr.dirty {
				preflight = append(preflight, t)
				continue
			}
			// Repo is dirty
			if !opts.force {
				if single {
					ui.Error("Repository has uncommitted changes!")
					ui.Info("Use --force to uninstall anyway, or commit/stash your changes first")
					return fmt.Errorf("uncommitted changes detected, use --force to override")
				}
				ui.StepSkip(t.name, "uncommitted changes, use --force")
				continue
			}
			ui.Warning("Repository %s has uncommitted changes (proceeding with --force)", t.name)
			preflight = append(preflight, t)
		}
		preflightSkipped = len(targets) - len(preflight)
		targets = preflight
		summary = summarizeUninstallTargets(targets)

		if preflightSkipped > 0 {
			ui.Info("%d tracked repo%s skipped, %d remaining", preflightSkipped, pluralS(preflightSkipped), len(targets))
			fmt.Println()
		}

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
			confirmSummary := summarizeUninstallTargets(targets)
			fmt.Printf("Uninstall %d %s? [y/N]: ", len(targets), confirmSummary.noun())
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
	batch := len(targets) > 1
	type batchResult struct {
		target    *uninstallTarget
		typeLabel string
		errMsg    string
	}

	var succeeded []*uninstallTarget
	var failed []string

	if batch {
		sp := ui.StartSpinner(fmt.Sprintf("Uninstalling %d %s", len(targets), summary.noun()))
		var results []batchResult

		for _, t := range targets {
			typeLabel, err := performUninstallQuiet(t, cfg)
			if err != nil {
				results = append(results, batchResult{target: t, errMsg: err.Error()})
				failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
			} else {
				results = append(results, batchResult{target: t, typeLabel: typeLabel})
				succeeded = append(succeeded, t)
			}
		}

		// Spinner end state
		if len(failed) > 0 && len(succeeded) == 0 {
			sp.Fail(fmt.Sprintf("Failed to uninstall %d %s", len(failed), summary.noun()))
		} else if len(failed) > 0 {
			sp.Warn(fmt.Sprintf("Uninstalled %d, failed %d", len(succeeded), len(failed)))
		} else {
			sp.Success(fmt.Sprintf("Uninstalled %d %s", len(succeeded), summary.noun()))
		}

		// Failures always shown individually
		var successes []batchResult
		var failures []batchResult
		for _, r := range results {
			if r.errMsg != "" {
				failures = append(failures, r)
			} else {
				successes = append(successes, r)
			}
		}

		if len(failures) > 0 {
			ui.SectionLabel("Failed")
			for _, r := range failures {
				ui.StepFail(r.target.name, r.errMsg)
			}
		}

		// Successes: condensed when many
		if len(successes) > 0 {
			ui.SectionLabel("Removed")
			switch {
			case len(successes) > 50:
				ui.StepDone(fmt.Sprintf("%d uninstalled", len(successes)), "")
			case len(successes) > 10:
				const maxShown = 10
				names := make([]string, 0, maxShown)
				for i := 0; i < maxShown && i < len(successes); i++ {
					names = append(names, successes[i].target.name)
				}
				detail := strings.Join(names, ", ")
				if len(successes) > maxShown {
					detail = fmt.Sprintf("%s ... +%d more", detail, len(successes)-maxShown)
				}
				ui.StepDone(fmt.Sprintf("%d uninstalled", len(successes)), detail)
			default:
				for _, r := range successes {
					ui.StepDone(r.target.name, r.typeLabel)
				}
			}
		}

		// Batch summary
		ui.OperationSummary("Uninstall", time.Since(start),
			ui.Metric{Label: "removed", Count: len(succeeded), HighlightColor: pterm.Green},
			ui.Metric{Label: "skipped", Count: preflightSkipped, HighlightColor: pterm.Yellow},
			ui.Metric{Label: "failed", Count: len(failed), HighlightColor: pterm.Red},
		)

		ui.SectionLabel("Next Steps")
		ui.Info("Moved to trash (7 days).")
		ui.Info("Run 'skillshare sync' to update all targets")

		// Opportunistic cleanup of expired trash items
		if n, _ := trash.Cleanup(trash.TrashDir(), 0); n > 0 {
			ui.Info("Cleaned up %d expired trash item%s", n, pluralS(n))
		}
	} else {
		for _, t := range targets {
			if err := performUninstall(t, cfg); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
			} else {
				succeeded = append(succeeded, t)
			}
		}
	}

	// --- Phase 7: FINALIZE ---
	// Batch-remove succeeded skills from registry
	if len(succeeded) > 0 {
		regDir := filepath.Dir(config.ConfigPath())
		reg, regErr := config.LoadRegistry(regDir)
		if regErr != nil {
			ui.Warning("Failed to load registry: %v", regErr)
		} else if len(reg.Skills) > 0 {
			removedNames := map[string]bool{}
			for _, t := range succeeded {
				removedNames[t.name] = true
			}
			updated := make([]config.SkillEntry, 0, len(reg.Skills))
			for _, s := range reg.Skills {
				fullName := s.FullName()
				if removedNames[fullName] {
					continue
				}
				// When a group directory is uninstalled, also remove its member skills
				memberOfRemoved := false
				for name := range removedNames {
					if strings.HasPrefix(fullName, name+"/") {
						memberOfRemoved = true
						break
					}
				}
				if memberOfRemoved {
					continue
				}
				updated = append(updated, s)
			}
			if len(updated) != len(reg.Skills) {
				reg.Skills = updated
				if saveErr := reg.Save(regDir); saveErr != nil {
					ui.Warning("Failed to update registry after uninstall: %v", saveErr)
				}
			}
		}
	}

	opNames := uninstallOpNames(rest)

	var finalErr error
	if len(failed) > 0 {
		if len(succeeded) == 0 {
			finalErr = fmt.Errorf("all uninstalls failed")
		}
		// Partial failure: report but exit success (skip & continue)
	}

	logUninstallOp(config.ConfigPath(), opNames, len(succeeded), start, finalErr)
	return finalErr
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
       skillshare uninstall --all [options]

Remove one or more skills or tracked repositories from the source directory.
Skills are moved to trash and kept for 7 days before automatic cleanup.
If the skill was installed from a remote source, a reinstall command is shown.

For tracked repositories (_repo-name):
  - Checks for uncommitted changes (requires --force to override)
  - Automatically removes the entry from .gitignore
  - The _ prefix is optional (automatically detected)

Options:
  --all               Remove ALL skills from source (requires confirmation)
  --group, -G <name>  Remove all skills in a group (prefix match, repeatable)
  --force, -f         Skip confirmation and ignore uncommitted changes
  --dry-run, -n       Preview without making changes
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare uninstall my-skill              # Remove a single skill
  skillshare uninstall a b c --force         # Remove multiple skills at once
  skillshare uninstall --all                 # Remove all skills
  skillshare uninstall --all --force         # Remove all without confirmation
  skillshare uninstall --all -n              # Preview what would be removed
  skillshare uninstall --group frontend      # Remove all skills in frontend/
  skillshare uninstall --group frontend -n   # Preview group removal
  skillshare uninstall x -G backend --force  # Mix names and groups
  skillshare uninstall _team-repo            # Remove tracked repository
  skillshare uninstall team-repo             # _ prefix is optional`)
}
