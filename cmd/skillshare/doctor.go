package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/skillignore"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
	versioncheck "skillshare/internal/version"
)

// doctorResult tracks issues and warnings
type doctorResult struct {
	errors   int
	warnings int
	checks   []doctorCheck
}

func (r *doctorResult) addError() {
	r.errors++
}

func (r *doctorResult) addWarning() {
	r.warnings++
}

func (r *doctorResult) addCheck(name, status, message string, details []string) {
	r.checks = append(r.checks, doctorCheck{
		Name: name, Status: status, Message: message, Details: details,
	})
}

func (r *doctorResult) addInfo(name, message string) {
	r.addCheck(name, checkInfo, message, nil)
}

func cmdDoctor(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	// Extract --json before checking for unexpected arguments
	var jsonMode bool
	var filtered []string
	for _, arg := range rest {
		if arg == "--json" {
			jsonMode = true
		} else {
			filtered = append(filtered, arg)
		}
	}
	rest = filtered

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

	if len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %v", rest)
	}

	if !jsonMode {
		ui.Logo(version)
	}

	if mode == modeProject {
		return cmdDoctorProject(cwd, jsonMode)
	}
	return cmdDoctorGlobal(jsonMode)
}

func cmdDoctorGlobal(jsonMode bool) error {
	// Start network check early so it overlaps with local I/O
	updateCh := make(chan *versioncheck.CheckResult, 1)
	go func() { updateCh <- fetchDoctorUpdateResult() }()

	var restoreUI func()
	if jsonMode {
		restoreUI = suppressUIToDevnull()
	}

	ui.Header("Checking environment")
	result := &doctorResult{}

	// Check config exists
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		if jsonMode {
			restoreUI()
			return writeJSONError(fmt.Errorf("config not found: run 'skillshare init' first"))
		}
		ui.Error("Config not found: run 'skillshare init' first")
		return nil
	}
	ui.Success("Config: %s", config.ConfigPath())
	ui.Info("Config directory: %s", config.BaseDir())
	ui.Info("Data directory:   %s", config.DataDir())
	ui.Info("State directory:  %s", config.StateDir())
	fmt.Println()

	cfg, err := config.Load()
	if err != nil {
		if jsonMode {
			restoreUI()
			return writeJSONError(fmt.Errorf("config error: %w", err))
		}
		ui.Error("Config error: %v", err)
		return nil
	}

	runDoctorChecks(cfg, result, false)
	checkExtras(cfg.Extras, result, false, cfg.Source, cfg.ExtrasSource, "")
	ui.Header("Storage")
	checkBackupStatus(result, false, backup.BackupDir())
	checkTrashStatus(result, trash.TrashDir())
	checkVersionDoctor(cfg, result)

	if jsonMode {
		return finalizeDoctorJSON(restoreUI, result, updateCh)
	}

	printUpdateAvailable(<-updateCh)
	printDoctorSummary(result)

	return nil
}

func cmdDoctorProject(root string, jsonMode bool) error {
	updateCh := make(chan *versioncheck.CheckResult, 1)
	go func() { updateCh <- fetchDoctorUpdateResult() }()

	var restoreUI func()
	if jsonMode {
		restoreUI = suppressUIToDevnull()
	}

	ui.Header("Checking environment")
	result := &doctorResult{}

	cfgPath := config.ProjectConfigPath(root)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if jsonMode {
			restoreUI()
			return writeJSONError(fmt.Errorf("project config not found: run 'skillshare init -p' first"))
		}
		ui.Error("Project config not found: run 'skillshare init -p' first")
		return nil
	}
	ui.Success("Config: %s", cfgPath)

	rt, err := loadProjectRuntime(root)
	if err != nil {
		if jsonMode {
			restoreUI()
			return writeJSONError(fmt.Errorf("config error: %w", err))
		}
		ui.Error("Config error: %v", err)
		return nil
	}

	cfg := &config.Config{
		Source:  rt.sourcePath,
		Targets: rt.targets,
		Mode:    "merge",
		Audit:   rt.config.Audit,
	}

	runDoctorChecks(cfg, result, true)
	checkExtras(rt.config.Extras, result, true, "", "", root)
	ui.Header("Storage")
	checkBackupStatus(result, true, "")
	checkTrashStatus(result, trash.ProjectTrashDir(root))
	checkVersionDoctor(cfg, result)

	if jsonMode {
		return finalizeDoctorJSON(restoreUI, result, updateCh)
	}

	printUpdateAvailable(<-updateCh)
	printDoctorSummary(result)

	return nil
}

func runDoctorChecks(cfg *config.Config, result *doctorResult, isProject bool) {
	// Single discovery pass for all checks (with .skillignore stats)
	sp := ui.StartSpinner("Discovering skills...")
	discovered, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(cfg.Source)
	if discoverErr != nil {
		discovered = nil
	}
	sp.Stop()

	checkSource(cfg, result, discovered, discoverErr)
	checkSkillignore(result, stats)
	checkSymlinkSupport(result)

	if !isProject {
		checkGitStatus(cfg.Source, result)
	}

	fmt.Println() // visual break before skill validation
	checkSkillsValidity(cfg.Source, result, discovered)
	checkSkillIntegrity(result, discovered)
	checkSkillTargetsField(result, discovered, targetNamesFromConfig(cfg.Targets))
	targetCache := checkTargets(cfg, result)
	printSymlinkCompatHint(cfg.Targets, cfg.Mode, isProject)
	checkSyncDrift(cfg, result, discovered, targetCache)
	checkBrokenSymlinks(cfg, result)
	checkDuplicateSkills(cfg, result, discovered)
}

func printDoctorSummary(result *doctorResult) {
	ui.Header("Summary")
	if result.errors == 0 && result.warnings == 0 {
		ui.Success("All checks passed!")
	} else if result.errors == 0 {
		ui.Warning("%d warning(s)", result.warnings)
	} else {
		ui.Error("%d error(s), %d warning(s)", result.errors, result.warnings)
	}

}

// checkSkillignore reports .skillignore status as an info or pass check.
func checkSkillignore(result *doctorResult, stats *skillignore.IgnoreStats) {
	if stats == nil || !stats.Active() {
		ui.Info("Skillignore: not configured")
		result.addInfo("skillignore", "No .skillignore found — you can create one to hide skills from discovery")
		return
	}

	msg := fmt.Sprintf("%d patterns, %d skills ignored", stats.PatternCount(), stats.IgnoredCount())
	if stats.HasLocal() {
		msg += " (.local active)"
	}
	ui.Success("Skillignore: %s", msg)
	var details []string
	details = append(details, stats.Patterns...)
	if len(stats.IgnoredSkills) > 0 {
		details = append(details, "---")
		details = append(details, stats.IgnoredSkills...)
	}
	result.addCheck("skillignore", checkPass, ".skillignore: "+msg, details)
}

func checkSource(cfg *config.Config, result *doctorResult, discovered []sync.DiscoveredSkill, discoverErr error) {
	info, err := os.Stat(cfg.Source)
	if err != nil {
		ui.Error("Source not found: %s", cfg.Source)
		result.addError()
		result.addCheck("source", checkError, fmt.Sprintf("Source not found: %s", cfg.Source), nil)
		return
	}

	if !info.IsDir() {
		ui.Error("Source is not a directory: %s", cfg.Source)
		result.addError()
		result.addCheck("source", checkError, fmt.Sprintf("Source is not a directory: %s", cfg.Source), nil)
		return
	}

	skillCount := 0
	if discoverErr == nil {
		skillCount = len(discovered)
	} else {
		entries, _ := os.ReadDir(cfg.Source)
		for _, e := range entries {
			if e.IsDir() && !utils.IsHidden(e.Name()) {
				skillCount++
			}
		}
	}
	ui.Success("Source: %s (%d skills)", cfg.Source, skillCount)
	result.addCheck("source", checkPass, fmt.Sprintf("Source: %s (%d skills)", cfg.Source, skillCount), nil)
}

func checkSymlinkSupport(result *doctorResult) {
	testLink := filepath.Join(os.TempDir(), "skillshare_symlink_test")
	testTarget := filepath.Join(os.TempDir(), "skillshare_symlink_target")
	os.Remove(testLink)
	os.RemoveAll(testTarget)
	os.MkdirAll(testTarget, 0755)
	defer os.Remove(testLink)
	defer os.RemoveAll(testTarget)

	// Use sync.CreateSymlink which handles Windows junctions
	if err := sync.CreateSymlink(testLink, testTarget); err != nil {
		ui.Error("Link not supported: %v", err)
		result.addError()
		result.addCheck("symlink_support", checkError, fmt.Sprintf("Link not supported: %v", err), nil)
		return
	}

	ui.Success("Link support: OK")
	result.addCheck("symlink_support", checkPass, "Link support: OK", nil)
}

// cachedTargetStatus stores CheckStatusMerge/Copy results so checkSyncDrift
// can reuse them without a second call.
type cachedTargetStatus struct {
	syncedCount int
	mode        string
	status      sync.TargetStatus
}

func checkTargets(cfg *config.Config, result *doctorResult) map[string]cachedTargetStatus {
	ui.Header("Checking targets")
	cache := make(map[string]cachedTargetStatus)

	var details []string
	hasError := false

	for name, target := range cfg.Targets {
		sc := target.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}
		if _, err := sync.FilterSkills(nil, sc.Include, sc.Exclude); err != nil {
			ui.Error("%s [%s]: invalid include/exclude config: %v", name, mode, err)
			result.addError()
			details = append(details, fmt.Sprintf("%s: invalid include/exclude config: %v", name, err))
			hasError = true
			continue
		}
		if mode == "symlink" && (len(sc.Include) > 0 || len(sc.Exclude) > 0) {
			ui.Warning("%s [%s]: include/exclude ignored in symlink mode", name, mode)
			result.addWarning()
		}

		targetIssues := checkTargetIssues(target, cfg.Source)

		if len(targetIssues) > 0 {
			ui.Error("%s [%s]: %s", name, mode, strings.Join(targetIssues, ", "))
			result.addError()
			details = append(details, fmt.Sprintf("%s: %s", name, strings.Join(targetIssues, ", ")))
			hasError = true
		} else {
			cached := displayTargetStatus(name, target, cfg.Source, mode)
			cache[name] = cached
		}
	}

	if hasError {
		result.addCheck("targets", checkError, "Some targets have issues", details)
	} else if len(cfg.Targets) > 0 {
		result.addCheck("targets", checkPass, fmt.Sprintf("All %d target(s) OK", len(cfg.Targets)), nil)
	} else {
		result.addCheck("targets", checkWarning, "No targets configured", nil)
	}

	return cache
}

func checkTargetIssues(target config.TargetConfig, source string) []string {
	sc := target.SkillsConfig()
	var targetIssues []string

	info, err := os.Lstat(sc.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check parent is writable
			parent := filepath.Dir(sc.Path)
			if _, err := os.Stat(parent); err != nil {
				targetIssues = append(targetIssues, "parent directory not found")
			}
		} else {
			targetIssues = append(targetIssues, fmt.Sprintf("access error: %v", err))
		}
		return targetIssues
	}

	// Check if it's a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		link, _ := os.Readlink(sc.Path)
		absLink, _ := filepath.Abs(link)
		absSource, _ := filepath.Abs(source)
		if !utils.PathsEqual(absLink, absSource) {
			targetIssues = append(targetIssues, fmt.Sprintf("symlink points to wrong location: %s", link))
		}
	}

	// Check write permission
	if info.IsDir() {
		testFile := filepath.Join(sc.Path, ".skillshare_write_test")
		if f, err := os.Create(testFile); err != nil {
			targetIssues = append(targetIssues, "not writable")
		} else {
			f.Close()
			os.Remove(testFile)
		}
	}

	return targetIssues
}

func displayTargetStatus(name string, target config.TargetConfig, source, mode string) cachedTargetStatus {
	sc := target.SkillsConfig()
	var statusStr string
	var cached cachedTargetStatus
	cached.mode = mode
	needsSync := false

	switch mode {
	case "merge":
		status, linkedCount, localCount := sync.CheckStatusMerge(sc.Path, source)
		cached.status = status
		cached.syncedCount = linkedCount
		switch status {
		case sync.StatusMerged:
			statusStr = fmt.Sprintf("merged (%d shared, %d local)", linkedCount, localCount)
		case sync.StatusLinked:
			statusStr = "linked (needs sync to apply merge mode)"
			needsSync = true
		default:
			statusStr = status.String()
		}
	case "copy":
		status, managedCount, localCount := sync.CheckStatusCopy(sc.Path)
		cached.status = status
		cached.syncedCount = managedCount
		switch status {
		case sync.StatusCopied:
			statusStr = fmt.Sprintf("copied (%d managed, %d local)", managedCount, localCount)
		case sync.StatusLinked:
			statusStr = "linked (needs sync to apply copy mode)"
			needsSync = true
		default:
			statusStr = status.String()
		}
	default:
		status := sync.CheckStatus(sc.Path, source)
		cached.status = status
		statusStr = status.String()
		if status == sync.StatusMerged {
			statusStr = "merged (needs sync to apply symlink mode)"
			needsSync = true
		}
	}

	if needsSync {
		ui.Warning("%s [%s]: %s", name, mode, statusStr)
	} else {
		ui.Success("%s [%s]: %s", name, mode, statusStr)
	}
	return cached
}

func checkSyncDrift(cfg *config.Config, result *doctorResult, discovered []sync.DiscoveredSkill, targetCache map[string]cachedTargetStatus) {
	if discovered == nil {
		result.addCheck("sync_drift", checkPass, "No skills discovered, skipping drift check", nil)
		return
	}

	var driftDetails []string
	for name, target := range cfg.Targets {
		cached, ok := targetCache[name]
		if !ok {
			continue // target had issues, skip drift check
		}
		if cached.mode != "merge" && cached.mode != "copy" {
			continue
		}
		sc := target.SkillsConfig()
		filtered, err := sync.FilterSkills(discovered, sc.Include, sc.Exclude)
		if err != nil {
			ui.Error("%s: invalid include/exclude config: %v", name, err)
			result.addError()
			continue
		}
		filtered = sync.FilterSkillsByTarget(filtered, name)
		expectedCount := len(filtered)
		if expectedCount == 0 {
			continue
		}

		if cached.mode == "copy" {
			if cached.status != sync.StatusCopied {
				continue
			}
			if cached.syncedCount < expectedCount {
				drift := expectedCount - cached.syncedCount
				msg := fmt.Sprintf("%s: %d skill(s) not synced (%d/%d copied)", name, drift, cached.syncedCount, expectedCount)
				ui.Warning("%s: %d skill(s) not synced (%d/%d copied)", name, drift, cached.syncedCount, expectedCount)
				result.addWarning()
				driftDetails = append(driftDetails, msg)
			}
		} else {
			if cached.status != sync.StatusMerged {
				continue
			}
			if cached.syncedCount < expectedCount {
				drift := expectedCount - cached.syncedCount
				msg := fmt.Sprintf("%s: %d skill(s) not synced (%d/%d linked)", name, drift, cached.syncedCount, expectedCount)
				ui.Warning("%s: %d skill(s) not synced (%d/%d linked)", name, drift, cached.syncedCount, expectedCount)
				result.addWarning()
				driftDetails = append(driftDetails, msg)
			}
		}
	}

	if len(driftDetails) > 0 {
		result.addCheck("sync_drift", checkWarning, "Sync drift detected", driftDetails)
	} else {
		result.addCheck("sync_drift", checkPass, "No sync drift detected", nil)
	}
}

// checkGitStatus checks if source is a git repo and its status
func checkGitStatus(source string, result *doctorResult) {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = source
	if err := cmd.Run(); err != nil {
		ui.Warning("Git: not initialized (recommended for backup)")
		result.addWarning()
		result.addCheck("git_status", checkWarning, "Git: not initialized (recommended for backup)", nil)
		return
	}

	// Check for uncommitted changes
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = source
	output, err := cmd.Output()
	if err != nil {
		ui.Warning("Git: unable to check status")
		result.addWarning()
		result.addCheck("git_status", checkWarning, "Git: unable to check status", nil)
		return
	}

	if len(output) > 0 {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		ui.Warning("Git: %d uncommitted change(s)", len(lines))
		result.addWarning()
		result.addCheck("git_status", checkWarning, fmt.Sprintf("Git: %d uncommitted change(s)", len(lines)), nil)
		return
	}

	// Check for remote
	cmd = exec.Command("git", "remote")
	cmd.Dir = source
	output, err = cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) == 0 {
		ui.Success("Git: initialized (no remote configured)")
		result.addCheck("git_status", checkPass, "Git: initialized (no remote configured)", nil)
	} else {
		ui.Success("Git: initialized with remote")
		result.addCheck("git_status", checkPass, "Git: initialized with remote", nil)
	}
}

// checkSkillsValidity checks if all skills have valid SKILL.md files
func checkSkillsValidity(source string, result *doctorResult, discovered []sync.DiscoveredSkill) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return
	}

	hasNestedSkills := make(map[string]bool)
	for _, skill := range discovered {
		if idx := strings.Index(skill.RelPath, "/"); idx > 0 {
			hasNestedSkills[skill.RelPath[:idx]] = true
		}
	}

	var invalid []string
	for _, entry := range entries {
		if !entry.IsDir() || utils.IsHidden(entry.Name()) {
			continue
		}

		// Tracked repos (_prefix) are container directories for nested skills;
		// they don't need a SKILL.md at the top level.
		if utils.IsTrackedRepoDir(entry.Name()) {
			continue
		}

		// Group containers can intentionally organize nested skills and do not
		// require their own SKILL.md at top-level.
		if hasNestedSkills[entry.Name()] {
			continue
		}

		skillFile := filepath.Join(source, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			invalid = append(invalid, entry.Name())
		}
	}

	if len(invalid) > 0 {
		ui.Warning("Skills without SKILL.md: %s", strings.Join(invalid, ", "))
		result.addWarning()
		result.addCheck("skills_validity", checkWarning, fmt.Sprintf("Skills without SKILL.md: %s", strings.Join(invalid, ", ")), invalid)
	} else {
		result.addCheck("skills_validity", checkPass, "All skills have valid SKILL.md", nil)
	}
}

// checkSkillIntegrity verifies installed skills haven't been tampered with by
// comparing current file hashes against the stored .skillshare-meta.json hashes.
func checkSkillIntegrity(result *doctorResult, discovered []sync.DiscoveredSkill) {
	if discovered == nil {
		return
	}

	// Phase 1: filter to skills that have meta with file hashes (cheap ReadMeta only)
	type verifiable struct {
		name   string
		path   string
		stored map[string]string
	}
	var toVerify []verifiable
	var skippedNames []string

	for _, skill := range discovered {
		meta, err := install.ReadMeta(skill.SourcePath)
		if err != nil {
			continue
		}
		if meta == nil {
			continue // Local skill without meta — expected, skip silently
		}
		if meta.FileHashes == nil {
			skippedNames = append(skippedNames, skill.RelPath)
			continue
		}
		toVerify = append(toVerify, verifiable{
			name:   skill.RelPath,
			path:   skill.SourcePath,
			stored: meta.FileHashes,
		})
	}

	if len(toVerify) == 0 {
		if len(skippedNames) > 0 {
			ui.Warning("Skill integrity: %d skill(s) missing file hashes: %s", len(skippedNames), strings.Join(skippedNames, ", "))
			result.addWarning()
			result.addCheck("skill_integrity", checkWarning, fmt.Sprintf("%d skill(s) missing file hashes", len(skippedNames)), skippedNames)
		} else {
			result.addCheck("skill_integrity", checkPass, "No tracked skills to verify", nil)
		}
		return
	}

	// Phase 2: compute hashes and compare (expensive)
	sp := ui.StartSpinner("Verifying skill integrity...")

	var tampered []string
	verified := 0

	for _, v := range toVerify {
		current, err := install.ComputeFileHashes(v.path)
		if err != nil {
			tampered = append(tampered, fmt.Sprintf("%s: hash error: %v", v.name, err))
			continue
		}

		var modified, missing, added int
		for file, storedHash := range v.stored {
			currentHash, ok := current[file]
			if !ok {
				missing++
			} else if currentHash != storedHash {
				modified++
			}
		}
		for file := range current {
			if _, ok := v.stored[file]; !ok {
				added++
			}
		}

		if modified > 0 || missing > 0 || added > 0 {
			var parts []string
			if modified > 0 {
				parts = append(parts, fmt.Sprintf("%d modified", modified))
			}
			if missing > 0 {
				parts = append(parts, fmt.Sprintf("%d missing", missing))
			}
			if added > 0 {
				parts = append(parts, fmt.Sprintf("%d added", added))
			}
			tampered = append(tampered, fmt.Sprintf("%s: %s", v.name, strings.Join(parts, ", ")))
		} else {
			verified++
		}
	}

	sp.Stop()

	if len(tampered) > 0 {
		for _, t := range tampered {
			ui.Warning(t)
		}
		result.addWarning()
		result.addCheck("skill_integrity", checkWarning,
			fmt.Sprintf("%d skill(s) with integrity issues", len(tampered)), tampered)
	}

	if verified > 0 {
		ui.Success("Skill integrity: %d/%d verified", verified, len(toVerify))
		if len(tampered) == 0 {
			result.addCheck("skill_integrity", checkPass,
				fmt.Sprintf("Skill integrity: %d/%d verified", verified, len(toVerify)), nil)
		}
	}

	if len(skippedNames) > 0 {
		ui.Warning("Skill integrity: %d skill(s) missing file hashes: %s", len(skippedNames), strings.Join(skippedNames, ", "))
		result.addWarning()
	}
}

// checkSkillTargetsField validates that skill-level targets values are known target names
func checkSkillTargetsField(result *doctorResult, discovered []sync.DiscoveredSkill, extraTargetNames []string) {
	if discovered == nil {
		return
	}

	warnings := findUnknownSkillTargets(discovered, extraTargetNames)
	if len(warnings) > 0 {
		for _, w := range warnings {
			ui.Warning("Skill targets: %s", w)
		}
		result.addWarning()
		result.addCheck("skill_targets_field", checkWarning, "Skills reference unknown targets", warnings)
	} else {
		result.addCheck("skill_targets_field", checkPass, "All skill target references are valid", nil)
	}
}

// checkBrokenSymlinks finds broken symlinks in targets
func checkBrokenSymlinks(cfg *config.Config, result *doctorResult) {
	var allBroken []string
	for name, target := range cfg.Targets {
		broken := findBrokenSymlinks(target.SkillsConfig().Path)
		if len(broken) > 0 {
			ui.Error("%s: %d broken symlink(s): %s", name, len(broken), strings.Join(broken, ", "))
			result.addError()
			for _, b := range broken {
				allBroken = append(allBroken, fmt.Sprintf("%s/%s", name, b))
			}
		}
	}
	if len(allBroken) > 0 {
		result.addCheck("broken_symlinks", checkError,
			fmt.Sprintf("%d broken symlink(s) found", len(allBroken)), allBroken)
	} else {
		result.addCheck("broken_symlinks", checkPass, "No broken symlinks", nil)
	}
}

func findBrokenSymlinks(dir string) []string {
	var broken []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return broken
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink, check if target exists
			if _, err := os.Stat(path); os.IsNotExist(err) {
				broken = append(broken, entry.Name())
			}
		}
	}

	return broken
}

// checkDuplicateSkills finds skills with same name in multiple locations.
// Merge mode is skipped because local skills are intentional.
func checkDuplicateSkills(cfg *config.Config, result *doctorResult, discovered []sync.DiscoveredSkill) {
	skillLocations := make(map[string][]string)

	if discovered == nil {
		result.addCheck("duplicate_skills", checkInfo, "Duplicate skill check skipped (source discovery unavailable)", nil)
		return
	}

	// Collect from non-merge targets.
	for name, target := range cfg.Targets {
		sc := target.SkillsConfig()
		// Determine effective mode
		mode := sc.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}

		// Skip merge mode - local skills are intentional
		if mode == "merge" {
			continue
		}

		resolution, err := sync.ResolveTargetSkillsForTarget(name, target.SkillsConfig(), discovered)
		if err != nil {
			continue
		}
		sourceNames := resolution.ValidTargetNames()

		manifestManaged := map[string]string{}
		if mode == "copy" {
			if manifest, err := sync.ReadManifest(sc.Path); err == nil && manifest != nil {
				manifestManaged = manifest.Managed
			}
		}

		entries, err := os.ReadDir(sc.Path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() || utils.IsHidden(entry.Name()) {
				continue
			}

			// In copy mode, managed entries are expected source mirrors, not duplicates.
			if mode == "copy" {
				if _, isManaged := manifestManaged[entry.Name()]; isManaged {
					continue
				}
			}

			// Check if it's a local skill (not a symlink to source)
			path := filepath.Join(sc.Path, entry.Name())
			info, err := os.Lstat(path)
			if err != nil {
				continue
			}

			if info.Mode()&os.ModeSymlink == 0 {
				// It's a real directory, not a symlink
				if sourceNames[entry.Name()] {
					if !slices.Contains(skillLocations[entry.Name()], "source") {
						skillLocations[entry.Name()] = append(skillLocations[entry.Name()], "source")
					}
					skillLocations[entry.Name()] = append(skillLocations[entry.Name()], name)
				}
			}
		}
	}

	// Find duplicates
	var duplicates []string
	for skill, locations := range skillLocations {
		if len(locations) > 1 {
			duplicates = append(duplicates, fmt.Sprintf("%s (%s)", skill, strings.Join(locations, ", ")))
		}
	}

	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		ui.Warning("Duplicate skills: %s", strings.Join(duplicates, "; "))
		ui.Info("  These exist in both source and target as separate copies.")
		ui.Info("  Fix: manually delete target copies, then run 'skillshare sync'")
		result.addWarning()
		result.addCheck("duplicate_skills", checkWarning, "Duplicate skills found", duplicates)
	} else {
		result.addCheck("duplicate_skills", checkPass, "No duplicate skills", nil)
	}
}

// checkExtras verifies extras source directories exist and targets are reachable.
func checkExtras(extras []config.ExtraConfig, result *doctorResult, isProject bool, source, extrasSource, projectRoot string) {
	if len(extras) == 0 {
		return
	}

	ui.Header("Extras")

	var details []string
	hasIssue := false

	for _, extra := range extras {
		var sourceDir string
		if isProject {
			sourceDir = config.ExtrasSourceDirProject(projectRoot, extra.Name)
		} else {
			sourceDir = config.ResolveExtrasSourceDir(extra, extrasSource, source)
		}

		files, err := sync.DiscoverExtraFiles(sourceDir)
		if err != nil {
			result.addError()
			ui.Error("%s: source directory missing (%s)", extra.Name, sourceDir)
			details = append(details, fmt.Sprintf("%s: source directory missing", extra.Name))
			hasIssue = true
			continue
		}
		ui.Success("%s: source exists (%d files)", extra.Name, len(files))

		reachable := 0
		for _, t := range extra.Targets {
			targetPath := config.ExpandPath(t.Path)
			if isProject && !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(projectRoot, targetPath)
			}
			if _, err := os.Stat(filepath.Dir(targetPath)); err == nil {
				reachable++
			} else {
				ui.Warning("%s: target %s not reachable (parent dir missing: %s)", extra.Name, t.Path, filepath.Dir(targetPath))
			}
		}
		if reachable == len(extra.Targets) {
			ui.Success("%s: all targets reachable (%d/%d)", extra.Name, reachable, len(extra.Targets))
		} else {
			result.addWarning()
			ui.Warning("%s: some targets unreachable (%d/%d)", extra.Name, reachable, len(extra.Targets))
			details = append(details, fmt.Sprintf("%s: %d/%d targets unreachable", extra.Name, len(extra.Targets)-reachable, len(extra.Targets)))
			hasIssue = true
		}
	}

	if hasIssue {
		result.addCheck("extras", checkWarning, "Some extras have issues", details)
	} else {
		result.addCheck("extras", checkPass, fmt.Sprintf("All %d extra(s) OK", len(extras)), nil)
	}
}

// checkBackupStatus shows last backup time
func checkBackupStatus(result *doctorResult, isProject bool, backupDir string) {
	if isProject {
		ui.Info("Backups: not used in project mode")
		result.addCheck("backup", checkPass, "Backups: not used in project mode", nil)
		return
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		ui.Info("Backups: none found")
		result.addCheck("backup", checkPass, "Backups: none found", nil)
		return
	}

	// Find most recent backup
	var latest string
	var latestTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = entry.Name()
		}
	}

	if latest != "" {
		age := time.Since(latestTime)
		var ageStr string
		switch {
		case age < time.Hour:
			ageStr = fmt.Sprintf("%d minutes ago", int(age.Minutes()))
		case age < 24*time.Hour:
			ageStr = fmt.Sprintf("%d hours ago", int(age.Hours()))
		default:
			ageStr = fmt.Sprintf("%d days ago", int(age.Hours()/24))
		}
		ui.Info("Backups: last backup %s (%s)", latest, ageStr)
		result.addCheck("backup", checkPass, fmt.Sprintf("Backups: last backup %s (%s)", latest, ageStr), nil)
	} else {
		result.addCheck("backup", checkPass, "Backups: none found", nil)
	}
}

// checkTrashStatus shows trash directory status
func checkTrashStatus(result *doctorResult, trashBase string) {
	if trashBase == "" {
		result.addCheck("trash", checkPass, "Trash: not configured", nil)
		return
	}

	items := trash.List(trashBase)
	if len(items) == 0 {
		ui.Info("Trash: empty")
		result.addCheck("trash", checkPass, "Trash: empty", nil)
		return
	}

	totalSize := trash.TotalSize(trashBase)
	sizeStr := formatBytes(totalSize)

	// Find oldest item age
	oldest := items[len(items)-1] // List is sorted newest-first
	age := time.Since(oldest.Date)
	days := int(age.Hours() / 24)

	var msg string
	if days > 0 {
		msg = fmt.Sprintf("Trash: %d item(s) (%s), oldest %d day(s)", len(items), sizeStr, days)
	} else {
		msg = fmt.Sprintf("Trash: %d item(s) (%s), oldest <1 day", len(items), sizeStr)
	}
	ui.Info("%s", msg)
	result.addCheck("trash", checkPass, msg, nil)
}

// formatBytes formats bytes into a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
	)
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// checkVersionDoctor checks CLI and skill versions
func checkVersionDoctor(cfg *config.Config, result *doctorResult) {
	ui.Header("Version")

	// CLI version
	ui.Success("CLI: %s", version)
	result.addCheck("cli_version", checkPass, fmt.Sprintf("CLI: %s", version), nil)

	// Skill version
	skillFile := filepath.Join(cfg.Source, "skillshare", "SKILL.md")

	file, err := os.Open(skillFile)
	if err != nil {
		ui.Warning("Skill: not found")
		ui.Info("  Run: skillshare upgrade --skill")
		result.addCheck("skill_version", checkWarning, "Skill: not found", nil)
		return
	}
	defer file.Close()

	var localVersion string
	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}
		if inFrontmatter && strings.HasPrefix(line, "version:") {
			localVersion = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
			break
		}
	}

	if localVersion == "" {
		ui.Warning("Skill: missing version")
		result.addCheck("skill_version", checkWarning, "Skill: missing version", nil)
		return
	}

	ui.Success("Skill: %s", localVersion)
	result.addCheck("skill_version", checkPass, fmt.Sprintf("Skill: %s", localVersion), nil)
}

// fetchDoctorUpdateResult checks if a newer version is available.
// Uses the shared versioncheck.Check which handles caching, auth, and
// proper semver comparison. Safe to call from a goroutine.
func fetchDoctorUpdateResult() *versioncheck.CheckResult {
	method := detectInstallMethod()
	return versioncheck.Check(version, method)
}

func printUpdateAvailable(result *versioncheck.CheckResult) {
	if result == nil || !result.UpdateAvailable {
		return
	}
	ui.Info("Update available: %s -> %s", result.CurrentVersion, result.LatestVersion)
	ui.Info("  Run: %s", result.InstallMethod.UpgradeCommand())
}
