package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/cache"
	"skillshare/internal/config"
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
}

func (r *doctorResult) addError() {
	r.errors++
}

func (r *doctorResult) addWarning() {
	r.warnings++
}

func cmdDoctor(args []string) error {
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

	if len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %v", rest)
	}

	ui.Logo(version)

	if mode == modeProject {
		return cmdDoctorProject(cwd)
	}
	return cmdDoctorGlobal()
}

func cmdDoctorGlobal() error {
	// Start network check early so it overlaps with local I/O
	updateCh := make(chan *versioncheck.CheckResult, 1)
	go func() { updateCh <- fetchDoctorUpdateResult() }()

	ui.Header("Checking environment")
	result := &doctorResult{}

	// Check config exists
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		ui.Error("Config not found: run 'skillshare init' first")
		return nil
	}
	ui.Success("Config: %s", config.ConfigPath())
	ui.Info("Config directory: %s", config.BaseDir())
	ui.Info("Data directory:   %s", config.DataDir())
	ui.Info("State directory:  %s", config.StateDir())

	cfg, err := config.Load()
	if err != nil {
		ui.Error("Config error: %v", err)
		return nil
	}

	runDoctorChecks(cfg, result, false)
	checkBackupStatus(false, backup.BackupDir())
	checkTrashStatus(trash.TrashDir())
	checkCacheStatus()
	checkVersionDoctor(cfg)
	printUpdateAvailable(<-updateCh)
	printDoctorSummary(result)

	return nil
}

func cmdDoctorProject(root string) error {
	updateCh := make(chan *versioncheck.CheckResult, 1)
	go func() { updateCh <- fetchDoctorUpdateResult() }()

	ui.Header("Checking environment")
	result := &doctorResult{}

	cfgPath := config.ProjectConfigPath(root)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		ui.Error("Project config not found: run 'skillshare init -p' first")
		return nil
	}
	ui.Success("Config: %s", cfgPath)

	rt, err := loadProjectRuntime(root)
	if err != nil {
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
	checkBackupStatus(true, "")
	checkTrashStatus(trash.ProjectTrashDir(root))
	checkCacheStatus()
	checkVersionDoctor(cfg)
	printUpdateAvailable(<-updateCh)
	printDoctorSummary(result)

	return nil
}

func runDoctorChecks(cfg *config.Config, result *doctorResult, isProject bool) {
	// Single discovery pass for all checks
	sp := ui.StartSpinner("Discovering skills...")
	discovered, discoverErr := discoveryCache.Discover(cfg.Source)
	if discoverErr != nil {
		discovered = nil
	}
	sp.Stop()

	checkSource(cfg, result, discovered, discoverErr)
	checkSymlinkSupport(result)

	if !isProject {
		checkGitStatus(cfg.Source, result)
	}

	checkSkillsValidity(cfg.Source, result, discovered)
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

func checkSource(cfg *config.Config, result *doctorResult, discovered []sync.DiscoveredSkill, discoverErr error) {
	info, err := os.Stat(cfg.Source)
	if err != nil {
		ui.Error("Source not found: %s", cfg.Source)
		result.addError()
		return
	}

	if !info.IsDir() {
		ui.Error("Source is not a directory: %s", cfg.Source)
		result.addError()
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
		return
	}

	ui.Success("Link support: OK")
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

	for name, target := range cfg.Targets {
		mode := target.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}
		if _, err := sync.FilterSkills(nil, target.Include, target.Exclude); err != nil {
			ui.Error("%s [%s]: invalid include/exclude config: %v", name, mode, err)
			result.addError()
			continue
		}
		if mode == "symlink" && (len(target.Include) > 0 || len(target.Exclude) > 0) {
			ui.Warning("%s [%s]: include/exclude ignored in symlink mode", name, mode)
			result.addWarning()
		}

		targetIssues := checkTargetIssues(target, cfg.Source)

		if len(targetIssues) > 0 {
			ui.Error("%s [%s]: %s", name, mode, strings.Join(targetIssues, ", "))
			result.addError()
		} else {
			cached := displayTargetStatus(name, target, cfg.Source, mode)
			cache[name] = cached
		}
	}
	return cache
}

func checkTargetIssues(target config.TargetConfig, source string) []string {
	var targetIssues []string

	info, err := os.Lstat(target.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check parent is writable
			parent := filepath.Dir(target.Path)
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
		link, _ := os.Readlink(target.Path)
		absLink, _ := filepath.Abs(link)
		absSource, _ := filepath.Abs(source)
		if !utils.PathsEqual(absLink, absSource) {
			targetIssues = append(targetIssues, fmt.Sprintf("symlink points to wrong location: %s", link))
		}
	}

	// Check write permission
	if info.IsDir() {
		testFile := filepath.Join(target.Path, ".skillshare_write_test")
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
	var statusStr string
	var cached cachedTargetStatus
	cached.mode = mode
	needsSync := false

	switch mode {
	case "merge":
		status, linkedCount, localCount := sync.CheckStatusMerge(target.Path, source)
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
		status, managedCount, localCount := sync.CheckStatusCopy(target.Path)
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
		status := sync.CheckStatus(target.Path, source)
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
		return
	}

	for name, target := range cfg.Targets {
		cached, ok := targetCache[name]
		if !ok {
			continue // target had issues, skip drift check
		}
		if cached.mode != "merge" && cached.mode != "copy" {
			continue
		}
		filtered, err := sync.FilterSkills(discovered, target.Include, target.Exclude)
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
				ui.Warning("%s: %d skill(s) not synced (%d/%d copied)", name, drift, cached.syncedCount, expectedCount)
				result.addWarning()
			}
		} else {
			if cached.status != sync.StatusMerged {
				continue
			}
			if cached.syncedCount < expectedCount {
				drift := expectedCount - cached.syncedCount
				ui.Warning("%s: %d skill(s) not synced (%d/%d linked)", name, drift, cached.syncedCount, expectedCount)
				result.addWarning()
			}
		}
	}
}

// checkGitStatus checks if source is a git repo and its status
func checkGitStatus(source string, result *doctorResult) {
	gitDir := filepath.Join(source, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		ui.Warning("Git: not initialized (recommended for backup)")
		result.addWarning()
		return
	}

	// Check for uncommitted changes
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = source
	output, err := cmd.Output()
	if err != nil {
		ui.Warning("Git: unable to check status")
		result.addWarning()
		return
	}

	if len(output) > 0 {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		ui.Warning("Git: %d uncommitted change(s)", len(lines))
		result.addWarning()
		return
	}

	// Check for remote
	cmd = exec.Command("git", "remote")
	cmd.Dir = source
	output, err = cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) == 0 {
		ui.Success("Git: initialized (no remote configured)")
	} else {
		ui.Success("Git: initialized with remote")
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
	}
}

// checkBrokenSymlinks finds broken symlinks in targets
func checkBrokenSymlinks(cfg *config.Config, result *doctorResult) {
	for name, target := range cfg.Targets {
		broken := findBrokenSymlinks(target.Path)
		if len(broken) > 0 {
			ui.Error("%s: %d broken symlink(s): %s", name, len(broken), strings.Join(broken, ", "))
			result.addError()
		}
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

	// Collect from source
	if discovered != nil {
		for _, skill := range discovered {
			skillLocations[skill.FlatName] = append(skillLocations[skill.FlatName], "source")
		}
	} else {
		entries, _ := os.ReadDir(cfg.Source)
		for _, entry := range entries {
			if entry.IsDir() && !utils.IsHidden(entry.Name()) {
				skillLocations[entry.Name()] = append(skillLocations[entry.Name()], "source")
			}
		}
	}

	// Collect from non-merge targets.
	for name, target := range cfg.Targets {
		// Determine effective mode
		mode := target.Mode
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

		manifestManaged := map[string]string{}
		if mode == "copy" {
			if manifest, err := sync.ReadManifest(target.Path); err == nil && manifest != nil {
				manifestManaged = manifest.Managed
			}
		}

		entries, err := os.ReadDir(target.Path)
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
			path := filepath.Join(target.Path, entry.Name())
			info, err := os.Lstat(path)
			if err != nil {
				continue
			}

			if info.Mode()&os.ModeSymlink == 0 {
				// It's a real directory, not a symlink
				skillLocations[entry.Name()] = append(skillLocations[entry.Name()], name)
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
	}
}

// checkBackupStatus shows last backup time
func checkBackupStatus(isProject bool, backupDir string) {
	if isProject {
		ui.Info("Backups: not used in project mode")
		return
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		ui.Info("Backups: none found")
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
	}
}

// checkTrashStatus shows trash directory status
func checkTrashStatus(trashBase string) {
	if trashBase == "" {
		return
	}

	items := trash.List(trashBase)
	if len(items) == 0 {
		ui.Info("Trash: empty")
		return
	}

	totalSize := trash.TotalSize(trashBase)
	sizeStr := formatBytes(totalSize)

	// Find oldest item age
	oldest := items[len(items)-1] // List is sorted newest-first
	age := time.Since(oldest.Date)
	days := int(age.Hours() / 24)

	if days > 0 {
		ui.Info("Trash: %d item(s) (%s), oldest %d day(s)", len(items), sizeStr, days)
	} else {
		ui.Info("Trash: %d item(s) (%s), oldest <1 day", len(items), sizeStr)
	}
}

// checkCacheStatus shows a one-line cache summary (discovery + UI).
// Cache is per-machine (not mode-specific), so both global and project call this.
func checkCacheStatus() {
	cacheDir := config.CacheDir()
	items := cache.ListDiskCaches(cacheDir)
	uiVersions := listUIVersions(cacheDir)

	if len(items) == 0 && len(uiVersions) == 0 {
		ui.Info("Cache: empty")
		return
	}

	var parts []string

	if len(items) > 0 {
		var totalSize int64
		orphanCount := 0
		for _, item := range items {
			totalSize += item.Size
			if item.Orphan {
				orphanCount++
			}
		}
		s := fmt.Sprintf("%d discovery file(s) (%s)", len(items), formatBytes(totalSize))
		if orphanCount > 0 {
			s += fmt.Sprintf(", %d orphan", orphanCount)
		}
		parts = append(parts, s)
	}

	if len(uiVersions) > 0 {
		// Show the first (usually only) UI version with size
		v := uiVersions[0]
		parts = append(parts, fmt.Sprintf("UI %s (%s)", v.name, formatBytes(v.size)))
	} else {
		parts = append(parts, "no UI cache")
	}

	ui.Info("Cache: %s", strings.Join(parts, " · "))
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
func checkVersionDoctor(cfg *config.Config) {
	ui.Header("Version")

	// CLI version
	ui.Success("CLI: %s", version)

	// Skill version
	skillFile := filepath.Join(cfg.Source, "skillshare", "SKILL.md")

	file, err := os.Open(skillFile)
	if err != nil {
		ui.Warning("Skill: not found")
		ui.Info("  Run: skillshare upgrade --skill")
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
		return
	}

	ui.Success("Skill: %s", localVersion)
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
