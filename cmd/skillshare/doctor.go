package main

import (
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
	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
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
	start    time.Time
	next     []string // command and reason pairs for the closing Next
}

// doctorWidth aligns the labels of doctor's fixed rows.
var doctorWidth = ui.RowWidth("Skillignore")

func (r *doctorResult) addError() {
	r.errors++
}

func (r *doctorResult) addWarning() {
	r.warnings++
}

func (r *doctorResult) addCheck(name, status, message string, details []string) {
	r.addCheckWithSuggestions(name, status, message, details, nil)
}

func (r *doctorResult) addCheckWithSuggestions(name, status, message string, details []string, suggestions []string) {
	r.checks = append(r.checks, doctorCheck{
		Name: name, Status: status, Message: message, Details: details, Suggestions: suggestions,
	})
}

func (r *doctorResult) addInfo(name, message string) {
	r.addCheck(name, checkInfo, message, nil)
}

// suggest adds a command to the closing Next, once each and at most three.
func (r *doctorResult) suggest(cmd, why string) {
	if len(r.next) >= 6 {
		return
	}
	for i := 0; i < len(r.next); i += 2 {
		if r.next[i] == cmd {
			return
		}
	}
	r.next = append(r.next, cmd, why)
}

func cmdDoctor(args []string) error {
	if wantsHelp(args) {
		printDoctorHelp()
		return nil
	}

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

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	if len(rest) > 0 {
		return fmt.Errorf("unexpected arguments: %v", rest)
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
	skillCh := make(chan string, 1)
	go func() { skillCh <- versioncheck.CachedRemoteSkillVersion() }()

	var restoreUI func()
	if jsonMode {
		restoreUI = suppressUIToDevnull()
	}

	fmt.Println(theme.Primary().Bold(true).Render(ui.WithModeLabel("Environment")))
	result := &doctorResult{start: time.Now()}

	// Check config exists
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		if jsonMode {
			restoreUI()
			return writeJSONError(fmt.Errorf("config not found: run 'skillshare init' first"))
		}
		ui.Error("Config not found: run 'skillshare init' first")
		return nil
	}
	ui.Row(ui.MarkOK, "Config", shortenPath(config.ConfigPath()), doctorWidth)
	ui.Row(ui.MarkNone, "Config dir", shortenPath(config.BaseDir()), doctorWidth)
	ui.Row(ui.MarkNone, "Data", shortenPath(config.DataDir()), doctorWidth)
	ui.Row(ui.MarkNone, "State", shortenPath(config.StateDir()), doctorWidth)

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
	checkExtras(cfg.Extras, result, false, cfg.EffectiveSkillsSource(), cfg.EffectiveExtrasSource(), "", "", func() error { return cfg.ValidateExtras() })
	checkNativeResources(cfg, "", result)
	ui.Section("Storage")
	checkBackupStatus(result, false, backup.BackupDir())
	checkTrashStatus(result, trash.TrashDir())
	checkVersionDoctor(cfg, result, false, skillCh)

	if jsonMode {
		return finalizeDoctorJSON(restoreUI, result, updateCh)
	}

	printUpdateAvailable(<-updateCh, result)
	printDoctorSummary(result)

	return nil
}

func cmdDoctorProject(root string, jsonMode bool) error {
	updateCh := make(chan *versioncheck.CheckResult, 1)
	go func() { updateCh <- fetchDoctorUpdateResult() }()
	skillCh := make(chan string, 1)
	go func() { skillCh <- versioncheck.CachedRemoteSkillVersion() }()

	var restoreUI func()
	if jsonMode {
		restoreUI = suppressUIToDevnull()
	}

	fmt.Println(theme.Primary().Bold(true).Render(ui.WithModeLabel("Environment")))
	result := &doctorResult{start: time.Now()}

	cfgPath := config.ProjectConfigPath(root)
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if jsonMode {
			restoreUI()
			return writeJSONError(fmt.Errorf("project config not found: run 'skillshare init -p' first"))
		}
		ui.Error("Project config not found: run 'skillshare init -p' first")
		return nil
	}
	ui.Row(ui.MarkOK, "Config", shortenPath(cfgPath), doctorWidth)

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
		Source:            rt.sourcePath,
		AgentsSource:      rt.agentsSourcePath,
		FollowSourceLinks: rt.config.FollowSourceLinks,
		Targets:           rt.targets,
		Mode:              "merge",
		Audit:             rt.config.Audit,
	}

	runDoctorChecks(cfg, result, true)
	checkExtras(rt.config.Extras, result, true, "", "", root, rt.config.EffectiveExtrasSource(root), func() error { return rt.config.ValidateExtras(root) })
	checkNativeResources(nil, root, result)
	ui.Section("Storage")
	checkBackupStatus(result, true, "")
	checkTrashStatus(result, trash.ProjectTrashDir(root))
	checkVersionDoctor(cfg, result, true, skillCh)

	if jsonMode {
		return finalizeDoctorJSON(restoreUI, result, updateCh)
	}

	printUpdateAvailable(<-updateCh, result)
	printDoctorSummary(result)

	return nil
}

func runDoctorChecks(cfg *config.Config, result *doctorResult, isProject bool) {
	// Single discovery pass for all checks (with .skillignore stats)
	sp := ui.StartSpinner("Discovering skills...")
	walk := cfg.SkillsWalk()
	discovered, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(cfg.EffectiveSkillsSource(), walk)
	if discoverErr != nil {
		discovered = nil
	}
	sp.Stop()

	checkSource(cfg, walk, result, discovered, discoverErr)
	checkAgentsSource(cfg, result)
	checkSkillignore(result, stats)
	checkUndeclaredSourceLinks(cfg.EffectiveSkillsSource(), walk, result)
	checkSymlinkSupport(result)
	checkTheme(result)

	if !isProject {
		checkGitStatus(cfg.EffectiveSkillsSource(), result)
	}
	checkMissingTrackedRepos(cfg.EffectiveSkillsSource(), result, isProject, walk)

	checkSkillsValidity(cfg.EffectiveSkillsSource(), walk, result, discovered)
	checkSkillIntegrity(result, discovered, walk.Follow)
	checkSkillTargetsField(result, discovered, targetNamesFromConfig(cfg.Targets))
	targetCache := checkTargets(cfg, result, isProject)
	printSymlinkCompatHint(cfg.Targets, cfg.Mode, isProject)
	checkSharedTargetPaths(cfg, result, isProject)
	checkCrossTargetDiscovery(cfg, result, isProject, discovered)
	checkSyncDrift(cfg, result, discovered, targetCache)
	checkBrokenSymlinks(cfg, walk.Follow, result)
	checkDuplicateSkills(cfg, result, discovered)
}

func printDoctorSummary(result *doctorResult) {
	fmt.Println()
	took := time.Since(result.start)
	switch {
	case result.errors == 0 && result.warnings == 0:
		ui.Done(ui.MarkOK, "All checks passed", took)
	case result.errors == 0:
		ui.Done(ui.MarkWarn, plural(result.warnings, "warning"), took)
	case result.warnings == 0:
		ui.Done(ui.MarkFail, plural(result.errors, "error"), took)
	default:
		ui.Done(ui.MarkFail, plural(result.errors, "error")+", "+plural(result.warnings, "warning"), took)
	}
	ui.Next(result.next...)
}

// checkSkillignore reports .skillignore status as an info or pass check.
func checkSkillignore(result *doctorResult, stats *skillignore.IgnoreStats) {
	if stats == nil || !stats.Active() {
		ui.Row(ui.MarkNone, "Skillignore", "not configured", doctorWidth)
		result.addInfo("skillignore", "No .skillignore found — you can create one to hide skills from discovery")
		return
	}

	msg := fmt.Sprintf("%d patterns, %d skills ignored", stats.PatternCount(), stats.IgnoredCount())
	if stats.HasLocal() {
		msg += " (.local active)"
	}
	shown := plural(stats.PatternCount(), "pattern") + ", " + plural(stats.IgnoredCount(), "skill") + " ignored"
	if stats.HasLocal() {
		shown += " (.local active)"
	}
	ui.Row(ui.MarkOK, "Skillignore", shown, doctorWidth)
	var details []string
	details = append(details, stats.Patterns...)
	if len(stats.IgnoredSkills) > 0 {
		details = append(details, "---")
		details = append(details, stats.IgnoredSkills...)
	}
	result.addCheck("skillignore", checkPass, ".skillignore: "+msg, details)
}

// checkUndeclaredSourceLinks reports first-level links by their raw identity:
// not followed while follow_source_links is off; followed, or skipped with the
// reason, while walk follows them.
func checkUndeclaredSourceLinks(source string, walk sourcewalk.Options, result *doctorResult) {
	root := utils.ResolveSymlink(source)
	entries, err := sourcewalk.ReadDir(root, sourcewalk.Options{})
	if err != nil {
		return
	}
	followed := map[string]bool{}
	skipped := map[string]sourcewalk.Skipped{}
	if walk.Follow != nil {
		view, _ := sourcewalk.ReadDir(root, walk)
		for _, e := range view {
			followed[e.Name()] = e.IsDir()
		}
		for _, s := range walk.Follow.Skipped() {
			skipped[s.Name] = s
		}
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := entry.Info()
		if err != nil || !utils.IsLinkMode(path, info.Mode()) {
			continue
		}
		if s, ok := skipped[entry.Name()]; ok {
			message := entry.Name() + ": not followed: " + s.Reason
			ui.Row(ui.MarkWarn, "Source link", message, doctorWidth)
			result.addWarning()
			result.addCheck("undeclared_source_links", checkWarning, message, nil)
			continue
		}
		message := entry.Name() + ": followed as a directory (follow_source_links)"
		if !followed[entry.Name()] {
			if walk.Follow != nil {
				continue // a link to a file is an ordinary entry
			}
			target, err := os.Stat(path)
			message = unfollowedLinkHint(entry.Name(), err != nil || target.IsDir())
		}
		ui.Row(ui.MarkNone, "Source link", message, doctorWidth)
		result.addInfo("undeclared_source_links", message)
	}
}

// unfollowedLinkHint is doctor's line for a first-level source link while
// follow_source_links is off; dir adds how to follow it.
func unfollowedLinkHint(name string, dir bool) string {
	message := name + ": not followed by discovery; its contents are invisible to skillshare"
	if dir {
		message += ". Set follow_source_links: true to follow it"
	}
	return message
}

func checkSource(cfg *config.Config, walk sourcewalk.Options, result *doctorResult, discovered []sync.DiscoveredSkill, discoverErr error) {
	info, err := os.Stat(cfg.EffectiveSkillsSource())
	if err != nil {
		ui.Row(ui.MarkFail, "Source", "not found: "+shortenPath(cfg.EffectiveSkillsSource()), doctorWidth)
		result.suggest("skillshare init", "create the source")
		result.addError()
		result.addCheck("source", checkError, fmt.Sprintf("Source not found: %s", cfg.EffectiveSkillsSource()), nil)
		return
	}

	if !info.IsDir() {
		ui.Row(ui.MarkFail, "Source", "not a directory: "+shortenPath(cfg.EffectiveSkillsSource()), doctorWidth)
		result.addError()
		result.addCheck("source", checkError, fmt.Sprintf("Source is not a directory: %s", cfg.EffectiveSkillsSource()), nil)
		return
	}

	skillCount := 0
	if discoverErr == nil {
		skillCount = len(discovered)
	} else {
		entries, _ := sourcewalk.ReadDir(cfg.EffectiveSkillsSource(), walk)
		for _, e := range entries {
			if e.IsDir() && !utils.IsHidden(e.Name()) {
				skillCount++
			}
		}
	}
	ui.Row(ui.MarkOK, "Source", shortenPath(cfg.EffectiveSkillsSource())+ui.DimText(" · "+plural(skillCount, "skill")), doctorWidth)
	result.addCheck("source", checkPass, fmt.Sprintf("Source: %s (%d skills)", cfg.EffectiveSkillsSource(), skillCount), nil)
}

func checkAgentsSource(cfg *config.Config, result *doctorResult) {
	agentsSource := cfg.EffectiveAgentsSource()
	info, err := os.Stat(agentsSource)
	if err != nil {
		if os.IsNotExist(err) {
			ui.Row(ui.MarkNone, "Agents", shortenPath(agentsSource)+ui.DimText(" · not created yet"), doctorWidth)
			result.addCheck("agents_source", checkPass, fmt.Sprintf("Agents source: %s (not created yet)", agentsSource), nil)
			return
		}
		ui.Row(ui.MarkFail, "Agents", err.Error(), doctorWidth)
		result.addError()
		result.addCheck("agents_source", checkError, fmt.Sprintf("Agents source error: %v", err), nil)
		return
	}

	if !info.IsDir() {
		ui.Row(ui.MarkFail, "Agents", "not a directory: "+shortenPath(agentsSource), doctorWidth)
		result.addError()
		result.addCheck("agents_source", checkError, fmt.Sprintf("Agents source is not a directory: %s", agentsSource), nil)
		return
	}

	agentCount := 0
	entries, _ := os.ReadDir(agentsSource)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			agentCount++
		}
	}
	ui.Row(ui.MarkOK, "Agents", shortenPath(agentsSource)+ui.DimText(" · "+plural(agentCount, "agent")), doctorWidth)
	result.addCheck("agents_source", checkPass, fmt.Sprintf("Agents source: %s (%d agents)", agentsSource, agentCount), nil)
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
	if err := sync.CreateSymlink(testLink, testTarget, ""); err != nil {
		ui.Row(ui.MarkFail, "Links", fmt.Sprintf("not supported: %v", err), doctorWidth)
		result.addError()
		result.addCheck("symlink_support", checkError, fmt.Sprintf("Link not supported: %v", err), nil)
		return
	}

	ui.Row(ui.MarkOK, "Links", "supported", doctorWidth)
	result.addCheck("symlink_support", checkPass, "Link support: OK", nil)
}

// checkTheme reports the resolved theme mode and source. Emits a warning
// status when the theme fell back to dark due to probe failure or non-TTY
// environments — users can resolve by setting SKILLSHARE_THEME explicitly.
func checkTheme(result *doctorResult) {
	tm := theme.Get()

	var status string
	switch tm.Source {
	case "env", "detected", "no-color":
		status = checkPass
	case "fallback-dark-no-tty", "fallback-dark-probe-failed":
		status = checkWarning
	default:
		status = checkInfo
	}

	// Build user-friendly message based on how the theme was resolved.
	var msg string
	switch {
	case tm.NoColor:
		msg = "Theme: colors disabled (NO_COLOR is set)"
	case tm.Source == "env":
		msg = fmt.Sprintf("Theme: %s (set via SKILLSHARE_THEME)", tm.Mode)
	case tm.Source == "detected":
		msg = fmt.Sprintf("Theme: %s (auto-detected from terminal)", tm.Mode)
	case tm.Source == "fallback-dark-no-tty":
		msg = fmt.Sprintf("Theme: %s (non-interactive terminal, defaulted to dark)", tm.Mode)
	case tm.Source == "fallback-dark-probe-failed":
		msg = fmt.Sprintf("Theme: %s (terminal detection failed, defaulted to dark)", tm.Mode)
	default:
		msg = fmt.Sprintf("Theme: %s", tm.Mode)
	}

	// Details: actionable guidance only, not raw internals.
	var details []string
	envVal := envOrDefault("SKILLSHARE_THEME", "")
	if envVal != "" {
		details = append(details, fmt.Sprintf("SKILLSHARE_THEME is set to \"%s\"", envVal))
	}
	if status == checkWarning {
		details = append(details, "Tip: set SKILLSHARE_THEME=light or SKILLSHARE_THEME=dark to choose your theme")
	}
	if tm.NoColor {
		details = append(details, "Unset NO_COLOR to re-enable colors")
	}

	mark := ui.MarkNone
	switch status {
	case checkPass:
		mark = ui.MarkOK
	case checkWarning:
		mark = ui.MarkWarn
	}
	ui.Row(mark, "Theme", strings.TrimPrefix(msg, "Theme: "), doctorWidth)

	if status == checkWarning {
		result.addWarning()
	}
	result.addCheck("theme", status, msg, details)
}

// envOrDefault returns the value of env var name or the given default.
func envOrDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// cachedTargetStatus stores CheckStatusMerge/Copy results so checkSyncDrift
// can reuse them without a second call.
type cachedTargetStatus struct {
	syncedCount int
	mode        string
	status      sync.TargetStatus
	needsSync   bool
}

func checkTargets(cfg *config.Config, result *doctorResult, isProject bool) map[string]cachedTargetStatus {
	ui.Section("Targets")
	cache := make(map[string]cachedTargetStatus)

	// Prepare agent context for per-target agent checks
	agentsSource := cfg.EffectiveAgentsSource()
	agentsExist := dirExists(agentsSource)
	var discoveredAgents []resource.DiscoveredResource
	if agentsExist {
		all, _ := resource.AgentKind{}.Discover(agentsSource)
		discoveredAgents = resource.ActiveAgents(all)
	}
	builtinAgents := config.DefaultAgentTargets()
	if isProject {
		builtinAgents = config.ProjectAgentTargets()
	}

	var details []string
	hasError := false

	names := targetNamesFromConfig(cfg.Targets)
	sort.Strings(names)
	width := ui.RowWidth(names...)
	for _, name := range names {
		target := cfg.Targets[name]
		sc := target.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}
		// The name labels the target's first row only.
		label := name
		row := func(mark, kind, text string) {
			ui.Row(mark, label, fmt.Sprintf("%-6s  %s", kind, text), width)
			label = ""
		}
		// Skills off: the folder is not managed, so it has nothing to check.
		if !sc.IsEnabled() {
			row(ui.MarkNone, "skills", ui.DimText(skillsOffSummary))
			if agentsExist {
				checkAgentTargetInline(name, target, builtinAgents, discoveredAgents, result, row)
			}
			continue
		}
		if _, err := sync.FilterSkills(nil, sc.Include, sc.Exclude); err != nil {
			row(ui.MarkFail, "skills", fmt.Sprintf("invalid include/exclude config: %v", err))
			result.addError()
			details = append(details, fmt.Sprintf("%s: invalid include/exclude config: %v", name, err))
			hasError = true
			continue
		}
		if mode == "symlink" && (len(sc.Include) > 0 || len(sc.Exclude) > 0) {
			row(ui.MarkWarn, "skills", "include/exclude ignored in symlink mode")
			result.addWarning()
		}

		targetIssues := checkTargetIssues(target, cfg.EffectiveSkillsSource(), mode)

		if len(targetIssues) > 0 {
			row(ui.MarkFail, "skills", strings.Join(targetIssues, ", ")+ui.DimText(" · "+mode))
			result.addError()
			details = append(details, fmt.Sprintf("%s: %s", name, strings.Join(targetIssues, ", ")))
			hasError = true
		} else {
			cached := displayTargetStatus(target, cfg.EffectiveSkillsSource(), mode, row)
			cache[name] = cached
			if cached.needsSync {
				result.suggest("skillshare sync", "bring the targets up to date")
			}
		}

		// Agent sub-check for this target
		if agentsExist {
			checkAgentTargetInline(name, target, builtinAgents, discoveredAgents, result, row)
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

func checkTargetIssues(target config.TargetConfig, source, mode string) []string {
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

	// Check if it's a symlink. Relative links must resolve against the link's
	// parent directory, not the current working directory.
	if mode == "symlink" && utils.IsLinkMode(sc.Path, info.Mode()) {
		link, _ := os.Readlink(sc.Path)
		absLink, err := utils.ResolveLinkTarget(sc.Path)
		absSource, _ := filepath.Abs(source)
		if err != nil || !utils.PathsEqual(absLink, absSource) {
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

func displayTargetStatus(target config.TargetConfig, source, mode string, row func(mark, kind, text string)) cachedTargetStatus {
	sc := target.SkillsConfig()
	var statusWord, detail string
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
			statusWord = "merged"
			detail = joinAgentCounts(linkedCount, "shared", localCount)
		case sync.StatusLinked:
			statusWord = "linked"
			detail = "needs sync"
			needsSync = true
		default:
			statusWord = status.String()
		}
	case "copy":
		status, managedCount, localCount := sync.CheckStatusCopy(sc.Path)
		cached.status = status
		cached.syncedCount = managedCount
		switch status {
		case sync.StatusCopied:
			statusWord = "copied"
			detail = joinAgentCounts(managedCount, "managed", localCount)
		case sync.StatusLinked:
			statusWord = "linked"
			detail = "needs sync"
			needsSync = true
		default:
			statusWord = status.String()
		}
	default:
		status := sync.CheckStatus(sc.Path, source)
		cached.status = status
		statusWord = status.String()
		if status == sync.StatusMerged {
			statusWord = "merged"
			detail = "needs sync"
			needsSync = true
		}
	}

	mark := ui.MarkOK
	if needsSync {
		mark = ui.MarkWarn
	}
	cached.needsSync = needsSync
	info := mode
	if detail != "" {
		info += " · " + detail
	}
	row(mark, "skills", statusWord+ui.DimText(" · "+info))
	return cached
}

func checkSyncDrift(cfg *config.Config, result *doctorResult, discovered []sync.DiscoveredSkill, targetCache map[string]cachedTargetStatus) {
	if discovered == nil {
		result.addCheck("sync_drift", checkPass, "No skills discovered, skipping drift check", nil)
		return
	}

	var driftDetails []string
	names := targetNamesFromConfig(cfg.Targets)
	sort.Strings(names)
	width := ui.RowWidth(names...)
	for _, name := range names {
		target := cfg.Targets[name]
		cached, ok := targetCache[name]
		if !ok {
			continue // target had issues, skip drift check
		}
		if cached.mode != "merge" && cached.mode != "copy" {
			continue
		}
		sc := target.SkillsConfig()
		expectedCount, err := sync.ExpectedSkillCount(name, sc, discovered)
		if err != nil {
			ui.Row(ui.MarkFail, name, fmt.Sprintf("invalid include/exclude config: %v", err), width)
			result.addError()
			continue
		}
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
				ui.Row(ui.MarkWarn, name, plural(drift, "skill")+" not synced"+ui.DimText(fmt.Sprintf(" · %d/%d copied", cached.syncedCount, expectedCount)), width)
				result.suggest("skillshare sync", "bring the targets up to date")
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
				ui.Row(ui.MarkWarn, name, plural(drift, "skill")+" not synced"+ui.DimText(fmt.Sprintf(" · %d/%d linked", cached.syncedCount, expectedCount)), width)
				result.suggest("skillshare sync", "bring the targets up to date")
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
		ui.Row(ui.MarkWarn, "Git", "not initialized (recommended for backup)", doctorWidth)
		result.addWarning()
		result.addCheck("git_status", checkWarning, "Git: not initialized (recommended for backup)", nil)
		return
	}

	// Check for uncommitted changes
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = source
	output, err := cmd.Output()
	if err != nil {
		ui.Row(ui.MarkWarn, "Git", "unable to check status", doctorWidth)
		result.addWarning()
		result.addCheck("git_status", checkWarning, "Git: unable to check status", nil)
		return
	}

	if len(output) > 0 {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		ui.Row(ui.MarkWarn, "Git", plural(len(lines), "uncommitted change"), doctorWidth)
		result.addWarning()
		result.addCheck("git_status", checkWarning, fmt.Sprintf("Git: %d uncommitted change(s)", len(lines)), nil)
		return
	}

	// Check for remote
	cmd = exec.Command("git", "remote")
	cmd.Dir = source
	output, err = cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) == 0 {
		ui.Row(ui.MarkOK, "Git", "initialized, no remote configured", doctorWidth)
		result.addCheck("git_status", checkPass, "Git: initialized (no remote configured)", nil)
	} else {
		ui.Row(ui.MarkOK, "Git", "initialized with remote", doctorWidth)
		result.addCheck("git_status", checkPass, "Git: initialized with remote", nil)
	}
}

// checkSkillsValidity checks if all skills have valid SKILL.md files
func checkMissingTrackedRepos(source string, result *doctorResult, isProject bool, walks ...sourcewalk.Options) {
	missingRepos, err := install.GetMissingTrackedRepos(source, walks...)
	if err != nil || len(missingRepos) == 0 {
		return
	}

	details := make([]string, 0, len(missingRepos))
	for _, repo := range missingRepos {
		detail := repo.Name
		if repo.Source != "" {
			detail += " ← " + repo.Source
		}
		details = append(details, detail)
	}

	suggestion := "Run 'skillshare install' to rehydrate tracked repositories from metadata."
	if isProject {
		suggestion = "Run 'skillshare install -p' to rehydrate tracked repositories from project metadata."
	}

	result.addWarning()
	result.addCheckWithSuggestions(
		"tracked_repos",
		checkWarning,
		fmt.Sprintf("%d tracked repository clone(s) are missing", len(missingRepos)),
		details,
		[]string{suggestion},
	)
}

func checkSkillsValidity(source string, walk sourcewalk.Options, result *doctorResult, discovered []sync.DiscoveredSkill) {
	entries, err := sourcewalk.ReadDir(source, walk)
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
		ui.Row(ui.MarkWarn, "Skills", fmt.Sprintf("%d without SKILL.md: %s", len(invalid), strings.Join(invalid, ", ")), doctorWidth)
		result.addWarning()
		result.addCheck("skills_validity", checkWarning, fmt.Sprintf("Skills without SKILL.md: %s", strings.Join(invalid, ", ")), invalid)
	} else {
		result.addCheck("skills_validity", checkPass, "All skills have valid SKILL.md", nil)
	}
}

// checkSkillIntegrity verifies installed skills haven't been tampered with by
// comparing current file hashes against the stored .skillshare-meta.json hashes.
func checkSkillIntegrity(result *doctorResult, discovered []sync.DiscoveredSkill, follow ...*sourcewalk.Follow) {
	if discovered == nil {
		return
	}

	// Phase 1: filter to skills that have meta with file hashes
	type verifiable struct {
		name   string
		path   string
		stored map[string]string
	}
	var toVerify []verifiable
	var skippedNames []string

	store := install.NewMetadataStore()
	if len(discovered) > 0 {
		sourceDir := strings.TrimSuffix(discovered[0].SourcePath, discovered[0].RelPath)
		sourceDir = strings.TrimRight(sourceDir, `/\`)
		store = install.LoadMetadataOrNew(sourceDir)
	}

	for _, skill := range discovered {
		entry := store.GetByPath(skill.RelPath)
		if entry == nil {
			continue // Local skill without meta — expected, skip silently
		}
		if entry.FileHashes == nil {
			skippedNames = append(skippedNames, skill.RelPath)
			continue
		}
		toVerify = append(toVerify, verifiable{
			name:   skill.RelPath,
			path:   skill.SourcePath,
			stored: entry.FileHashes,
		})
	}

	if len(toVerify) == 0 {
		if len(skippedNames) > 0 {
			ui.Row(ui.MarkWarn, "Integrity", plural(len(skippedNames), "skill")+" missing file hashes: "+strings.Join(skippedNames, ", "), doctorWidth)
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
		current, err := install.ComputeFileHashes(v.path, follow...)
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

	mark := ui.MarkOK
	if len(tampered)+len(skippedNames) > 0 {
		mark = ui.MarkWarn
	}
	ui.Row(mark, "Integrity", fmt.Sprintf("%d/%d skills verified", verified, len(toVerify)), doctorWidth)
	for _, t := range tampered {
		ui.Row(ui.MarkWarn, "", t, doctorWidth)
	}

	if len(tampered) > 0 {
		result.addWarning()
		result.addCheck("skill_integrity", checkWarning,
			fmt.Sprintf("%d skill(s) with integrity issues", len(tampered)), tampered)
	}

	if verified > 0 {
		if len(tampered) == 0 {
			result.addCheck("skill_integrity", checkPass,
				fmt.Sprintf("Skill integrity: %d/%d verified", verified, len(toVerify)), nil)
		}
	}

	if len(skippedNames) > 0 {
		ui.Row(ui.MarkWarn, "", plural(len(skippedNames), "skill")+" missing file hashes: "+strings.Join(skippedNames, ", "), doctorWidth)
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
		label := "Skills"
		for _, w := range warnings {
			ui.Row(ui.MarkWarn, label, w, doctorWidth)
			label = ""
		}
		result.addWarning()
		result.addCheck("skill_targets_field", checkWarning, "Skills reference unknown targets", warnings)
	} else {
		result.addCheck("skill_targets_field", checkPass, "All skill target references are valid", nil)
	}
}

// checkBrokenSymlinks finds broken symlinks in targets. Links to skills behind
// a source link whose target is away (an unmounted drive) are expected to be
// broken and are kept by sync, so they are reported as waiting, not as errors.
func checkBrokenSymlinks(cfg *config.Config, follow *sourcewalk.Follow, result *doctorResult) {
	var allBroken, allWaiting []string
	unavailable := follow.Unavailable()
	names := targetNamesFromConfig(cfg.Targets)
	sort.Strings(names)
	width := ui.RowWidth(names...)
	for _, name := range names {
		target := cfg.Targets[name]
		if !target.SkillsConfig().IsEnabled() {
			continue
		}
		var broken, waiting []string
		for _, b := range findBrokenSymlinks(target.SkillsConfig().Path) {
			if behindUnavailableLink(filepath.Join(target.SkillsConfig().Path, b), cfg.EffectiveSkillsSource(), unavailable) {
				waiting = append(waiting, b)
			} else {
				broken = append(broken, b)
			}
		}
		if len(broken) > 0 {
			ui.Row(ui.MarkFail, name, plural(len(broken), "broken symlink")+": "+strings.Join(broken, ", "), width)
			result.suggest("skillshare sync", "prune the broken links")
			result.addError()
			for _, b := range broken {
				allBroken = append(allBroken, fmt.Sprintf("%s/%s", name, b))
			}
		}
		if len(waiting) > 0 {
			ui.Row(ui.MarkWarn, name, plural(len(waiting), "link")+" behind an unavailable source link, kept until it is back: "+strings.Join(waiting, ", "), width)
			result.addWarning()
			for _, b := range waiting {
				allWaiting = append(allWaiting, fmt.Sprintf("%s/%s", name, b))
			}
		}
	}
	switch {
	case len(allBroken) > 0:
		result.addCheck("broken_symlinks", checkError,
			fmt.Sprintf("%d broken symlink(s) found", len(allBroken)), allBroken)
	case len(allWaiting) > 0:
		result.addCheck("broken_symlinks", checkWarning,
			fmt.Sprintf("%d link(s) behind an unavailable source link", len(allWaiting)), allWaiting)
	default:
		result.addCheck("broken_symlinks", checkPass, "No broken symlinks", nil)
	}
}

// behindUnavailableLink classifies broken links by their stored destination,
// so frontmatter-based target names work as well as flattened names.
func behindUnavailableLink(path, source string, unavailable []string) bool {
	if destination, err := utils.ResolveLinkTarget(path); err == nil {
		destination = sourcewalk.Canonical(destination)
		for _, name := range unavailable {
			root := sourcewalk.Canonical(filepath.Join(source, name))
			if utils.PathsEqual(destination, root) || utils.PathHasPrefix(destination, root+string(filepath.Separator)) {
				return true
			}
		}
		return false
	}
	// Some link types cannot expose their target; retain the flattened-name fallback.
	entry := filepath.Base(path)
	for _, name := range unavailable {
		if entry == name || strings.HasPrefix(entry, name+"__") {
			return true
		}
	}
	return false
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

		if utils.IsLinkMode(path, info.Mode()) {
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

		// Skip merge mode - local skills are intentional; skip targets with skills off.
		if mode == "merge" || !sc.IsEnabled() {
			continue
		}

		// The target dir is itself a link (symlink mode after sync), so its
		// entries are the source itself, not target-local copies.
		if mode == "symlink" && utils.IsSymlinkOrJunction(sc.Path) {
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

			if !utils.IsLinkMode(path, info.Mode()) {
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
		ui.Note("These exist in both source and target as separate copies.")
		ui.Note("Fix: delete the target copies, then run skillshare sync")
		result.addWarning()
		result.addCheck("duplicate_skills", checkWarning, "Duplicate skills found", duplicates)
	} else {
		result.addCheck("duplicate_skills", checkPass, "No duplicate skills", nil)
	}
}

// checkExtras verifies extras source directories exist and targets are reachable.
func checkExtras(extras []config.ExtraConfig, result *doctorResult, isProject bool, source, extrasSource, projectRoot, projectExtrasParent string, validate func() error) {
	if len(extras) == 0 {
		return
	}

	ui.Section("Extras")
	names := make([]string, len(extras))
	for i, extra := range extras {
		names[i] = extra.Name
	}
	width := ui.RowWidth(names...)

	var details []string
	hasIssue, hasError := false, false

	for _, extra := range extras {
		if err := config.ValidateExtraConfig(extra); err != nil {
			result.addError()
			ui.Row(ui.MarkFail, extra.Name, err.Error(), width)
			details = append(details, fmt.Sprintf("%s: %v", extra.Name, err))
			hasIssue, hasError = true, true
			continue
		}

		var sourceDir string
		if isProject {
			sourceDir = config.ResolveExtrasSourceDirProject(extra, projectExtrasParent, projectRoot)
		} else {
			sourceDir = config.ResolveExtrasSourceDir(extra, extrasSource, source)
		}

		files, err := sync.DiscoverExtraSource(sourceDir, extra.File)
		if err != nil {
			result.addError()
			ui.Row(ui.MarkFail, extra.Name, "source missing · "+shortenPath(sourceDir), width)
			details = append(details, fmt.Sprintf("%s: source directory missing", extra.Name))
			hasIssue = true
			continue
		}

		reachable := 0
		var unreachableTargets []string
		var targetErrors, targetWarnings []string
		for _, t := range extra.Targets {
			targetPath := resolveDoctorExtraTarget(t.Path, isProject, projectRoot)
			if _, err := os.Stat(filepath.Dir(targetPath)); err == nil {
				reachable++
				errs, warns := extraTargetFindings(extra, t, sourceDir, targetPath)
				targetErrors = append(targetErrors, errs...)
				targetWarnings = append(targetWarnings, warns...)
			} else {
				unreachableTargets = append(unreachableTargets, t.Path)
			}
		}
		// Findings name the extra unless a row above already did.
		findingsLabel := ""
		if reachable == len(extra.Targets) && len(targetErrors)+len(targetWarnings) == 0 {
			ui.Row(ui.MarkOK, extra.Name, fmt.Sprintf("%s · %d/%d targets OK", plural(len(files), "file"), reachable, len(extra.Targets)), width)
		} else if reachable < len(extra.Targets) {
			result.addWarning()
			ui.Row(ui.MarkWarn, extra.Name, fmt.Sprintf("%s · %d/%d targets unreachable", plural(len(files), "file"), len(extra.Targets)-reachable, len(extra.Targets)), width)
			for _, t := range unreachableTargets {
				ui.Note(t + " (parent dir missing)")
			}
			details = append(details, fmt.Sprintf("%s: %d/%d targets unreachable", extra.Name, len(extra.Targets)-reachable, len(extra.Targets)))
			hasIssue = true
		}
		shownErrors, shownWarnings := targetErrors, targetWarnings
		if reachable == len(extra.Targets) {
			// The row label names the extra, so the lines start at the arrow.
			findingsLabel = extra.Name
			shownErrors = trimFindingsPrefix(targetErrors, extra.Name+" ")
			shownWarnings = trimFindingsPrefix(targetWarnings, extra.Name+" ")
		}
		printResourceFindings(findingsLabel, width, shownErrors, shownWarnings)
		for range targetErrors {
			result.addError()
		}
		for range targetWarnings {
			result.addWarning()
		}
		details = append(append(details, targetErrors...), targetWarnings...)
		hasIssue = hasIssue || len(targetErrors)+len(targetWarnings) > 0
		hasError = hasError || len(targetErrors) > 0
	}

	if !hasError {
		if err := validate(); err != nil {
			result.addError()
			ui.Error("%v", err)
			details = append(details, err.Error())
			hasIssue, hasError = true, true
		}
	}

	switch {
	case hasError:
		result.addCheck("extras", checkError, "Some extras have errors", details)
	case hasIssue:
		result.addCheck("extras", checkWarning, "Some extras have issues", details)
	default:
		result.addCheck("extras", checkPass, fmt.Sprintf("All %d extra(s) OK", len(extras)), nil)
	}
}

// checkBackupStatus shows last backup time
func checkBackupStatus(result *doctorResult, isProject bool, backupDir string) {
	if isProject {
		ui.Row(ui.MarkNone, "Backups", "not used in project mode", doctorWidth)
		result.addCheck("backup", checkPass, "Backups: not used in project mode", nil)
		return
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		ui.Row(ui.MarkNone, "Backups", "none found", doctorWidth)
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
		ui.Row(ui.MarkNone, "Backups", "last "+latest+ui.DimText(" · "+timeAgo(latestTime)), doctorWidth)
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
		ui.Row(ui.MarkNone, "Trash", "empty", doctorWidth)
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
	oldestStr := "under a day"
	if days > 0 {
		oldestStr = plural(days, "day")
	}
	ui.Row(ui.MarkNone, "Trash", fmt.Sprintf("%s, %s", plural(len(items), "item"), sizeStr)+ui.DimText(" · oldest "+oldestStr), doctorWidth)
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
// remoteSkill delivers the latest published skill version, or "" offline;
// it is read only once a local version is known.
func checkVersionDoctor(cfg *config.Config, result *doctorResult, isProject bool, remoteSkill <-chan string) {
	ui.Section("Version")

	// CLI version
	ui.Row(ui.MarkOK, "CLI", version, doctorWidth)
	result.addCheck("cli_version", checkPass, fmt.Sprintf("CLI: %s", version), nil)

	// Skill version: try SKILL.md frontmatter first, then metadata store
	localVersion := versioncheck.ReadLocalSkillVersion(cfg.EffectiveSkillsSource())
	if localVersion == "" {
		// Try metadata store (tracks installed version even without metadata.version in SKILL.md)
		store := install.LoadMetadataOrNew(cfg.EffectiveSkillsSource())
		if entry := store.Get("skillshare"); entry != nil && entry.Version != "" {
			localVersion = strings.TrimPrefix(entry.Version, "v")
		}
	}

	if localVersion == "" {
		skillFile := filepath.Join(cfg.EffectiveSkillsSource(), "skillshare", "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			if isProject {
				ui.Row(ui.MarkNone, "Skill", "not installed", doctorWidth)
				result.addCheck("skill_version", checkInfo, "Skill: not installed in project", nil)
			} else {
				ui.Row(ui.MarkWarn, "Skill", "not found", doctorWidth)
				result.suggest("skillshare upgrade --skill", "install the built-in skill")
				result.addCheck("skill_version", checkWarning, "Skill: not found", nil)
				result.addWarning()
			}
		} else {
			ui.Row(ui.MarkWarn, "Skill", "missing version", doctorWidth)
			result.addCheck("skill_version", checkWarning, "Skill: missing version", nil)
			result.addWarning()
		}
		return
	}

	if remote := <-remoteSkill; versioncheck.SkillOutdated(localVersion, remote) {
		ui.Row(ui.MarkWarn, "Skill", localVersion+" → "+remote+" available", doctorWidth)
		result.suggest("skillshare upgrade --skill", "update the built-in skill")
		result.addCheck("skill_version", checkWarning, fmt.Sprintf("Skill: %s (%s available)", localVersion, remote), nil)
		result.addWarning()
		return
	}
	ui.Row(ui.MarkOK, "Skill", localVersion, doctorWidth)
	result.addCheck("skill_version", checkPass, fmt.Sprintf("Skill: %s", localVersion), nil)
}

// fetchDoctorUpdateResult checks if a newer version is available.
// Uses the shared versioncheck.Check which handles caching, auth, and
// proper semver comparison. Safe to call from a goroutine.
func fetchDoctorUpdateResult() *versioncheck.CheckResult {
	method := detectInstallMethod()
	if version == "" || version == "dev" {
		return &versioncheck.CheckResult{
			CurrentVersion:  "dev",
			LatestVersion:   "dev-ui-flow",
			UpdateAvailable: true,
			InstallMethod:   method,
		}
	}
	return versioncheck.Check(version, method)
}

func printUpdateAvailable(update *versioncheck.CheckResult, result *doctorResult) {
	if update == nil || !update.UpdateAvailable {
		return
	}
	ui.Row(ui.MarkNone, "Update", ui.VersionLabel(update.CurrentVersion)+" → "+ui.VersionLabel(update.LatestVersion)+" available", doctorWidth)
	result.suggest(update.InstallMethod.UpgradeCommand(), "update to "+ui.VersionLabel(update.LatestVersion))
}

func printDoctorHelp() {
	printHelp("skillshare doctor [options]", "Check environment and diagnose issues.",
		helpGroup{title: "Options", rows: []helpRow{
			{"--json", "Output results as JSON"},
			{"-p, --project", "Use project-level config"},
			{"-g, --global", "Use global config"},
		}},
		helpExamples(
			helpRow{"skillshare doctor", "Run diagnostics"},
			helpRow{"skillshare doctor --json", "Output as JSON"},
			helpRow{"skillshare doctor -p", "Check project config"},
		),
	)
}

// trimFindingsPrefix drops prefix from each finding for display.
func trimFindingsPrefix(findings []string, prefix string) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = strings.TrimPrefix(f, prefix)
	}
	return out
}
