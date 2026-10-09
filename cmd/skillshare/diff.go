package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	gosync "sync"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// diffRenderOpts controls diff output behavior.
type diffRenderOpts struct {
	noTUI      bool
	showPatch  bool
	showStat   bool
	jsonOutput bool
	noTargets  bool // the config has no targets at all
}

// diffJSONOutput is the JSON representation for diff --json output.
type diffJSONOutput struct {
	Targets  []diffJSONTarget `json:"targets"`
	Duration string           `json:"duration"`
}

type diffJSONTarget struct {
	Name    string         `json:"name"`
	Mode    string         `json:"mode"`
	Synced  bool           `json:"synced"`
	Error   string         `json:"error,omitempty"`
	Items   []diffJSONItem `json:"items"`
	Include []string       `json:"include"`
	Exclude []string       `json:"exclude"`
}

type diffJSONItem struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	Kind   string `json:"kind,omitempty"` // "skill" or "agent"
	Reason string `json:"reason"`
	IsSync bool   `json:"is_sync"`
}

func cmdDiff(args []string) error {
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

	// Extract kind filter (e.g. "skillshare diff agents"). "all" is the
	// default: skills and agents together, plus extras.
	if len(rest) > 0 && rest[0] == "all" {
		rest = rest[1:]
	}
	kind, rest := parseKindArg(rest)

	scope := "global"
	cfgPath := config.ConfigPath()
	if mode == modeProject {
		scope = "project"
		cfgPath = config.ProjectConfigPath(cwd)
	}

	var targetName string
	var opts diffRenderOpts
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--help", "-h":
			printDiffHelp()
			return nil
		case "--no-tui":
			opts.noTUI = true
		case "--patch":
			opts.showPatch = true
			opts.noTUI = true // --patch implies no TUI
		case "--stat":
			opts.showStat = true
			opts.noTUI = true // --stat implies no TUI
		case "--json":
			opts.jsonOutput = true
			opts.noTUI = true // --json implies no TUI
		default:
			targetName = rest[i]
		}
	}

	var cmdErr error
	if mode == modeProject {
		cmdErr = cmdDiffProject(cwd, targetName, kind, opts, start)
	} else {
		cmdErr = cmdDiffGlobal(targetName, kind, opts, start)
	}
	logDiffOp(cfgPath, targetName, scope, 0, start, cmdErr)
	return cmdErr
}

func logDiffOp(cfgPath string, targetName, scope string, targetsShown int, start time.Time, cmdErr error) {
	e := oplog.NewEntry("diff", statusFromErr(cmdErr), time.Since(start))
	a := map[string]any{"scope": scope}
	if targetName != "" {
		a["target"] = targetName
	}
	if targetsShown > 0 {
		a["targets_shown"] = targetsShown
	}
	e.Args = a
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

// targetDiffResult holds the diff outcome for one target.
type targetDiffResult struct {
	name       string
	mode       string          // "merge", "copy", "symlink"
	items      []copyDiffEntry // reuse existing struct
	syncCount  int
	localCount int
	synced     bool   // true if fully synced
	errMsg     string // non-empty if target inaccessible
	include    []string
	exclude    []string
	srcMtime   time.Time // newest file mtime across source skills
	dstMtime   time.Time // newest file mtime in target dir
}

// inSync reports that sync has nothing to do for the target. Folders only the
// target holds stay listed, but sync never touches them.
func (r targetDiffResult) inSync() bool { return r.errMsg == "" && r.syncCount == 0 }

type copyDiffEntry struct {
	action string // "add", "modify", "remove"
	name   string
	kind   string // "skill" or "agent" (empty defaults to "skill")
	reason string
	isSync bool            // true = needs sync, false = local-only
	keptAt string          // old name a skill stays at while a local folder holds its new one; sync changes nothing, so no dstDir
	files  []fileDiffEntry // file-level diffs (nil until populated)
	srcDir string          // source directory path (for lazy diff)
	dstDir string          // target directory path (for lazy diff)
}

// ensureFiles lazily populates file-level diffs on first access.
// Works for items with srcDir (full diff) or only dstDir (file listing).
func (e *copyDiffEntry) ensureFiles() {
	if e.files != nil {
		return
	}
	if e.srcDir == "" && e.dstDir == "" {
		return
	}
	e.files = diffSkillFiles(e.srcDir, e.dstDir)
}

// label is the item's name, plus the old name a kept skill stays at.
func (e copyDiffEntry) label() string {
	if e.keptAt == "" {
		return e.name
	}
	return e.name + " (stays at " + e.keptAt + ")"
}

// keptCount counts the skills kept under their old name.
func (r targetDiffResult) keptCount() int {
	n := 0
	for _, item := range r.items {
		if item.keptAt != "" {
			n++
		}
	}
	return n
}

// latestMtime returns the newest file mtime under dir.
func latestMtime(dir string) time.Time {
	var latest time.Time
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil || info.IsDir() {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}

// diffProgress displays multi-target scanning progress.
// Target list (spinner/queued/done) + overall progress bar at the bottom.
type diffProgress struct {
	names           []string
	states          []string // "queued", "scanning", "done", "error"
	details         []string
	totalSkills     int
	processedSkills int
	area            *pterm.AreaPrinter
	mu              gosync.Mutex
	stopCh          chan struct{}
	frames          []string
	frame           int
	isTTY           bool
}

// newDiffProgress creates a progress display for diff scanning.
// When showBar is false (no copy-mode targets), the progress bar is hidden
// because merge/symlink diffs are instant.
func newDiffProgress(names []string, totalSkills int, showBar bool) *diffProgress {
	barTotal := totalSkills
	if !showBar {
		barTotal = 0
	}
	dp := &diffProgress{
		names:       names,
		states:      make([]string, len(names)),
		details:     make([]string, len(names)),
		totalSkills: barTotal,
		frames:      []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		isTTY:       ui.IsTTY(),
	}
	for i := range dp.states {
		dp.states[i] = "queued"
	}
	if !dp.isTTY {
		return dp
	}
	area, _ := pterm.DefaultArea.WithRemoveWhenDone(true).Start()
	dp.area = area
	dp.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-dp.stopCh:
				return
			case <-ticker.C:
				dp.mu.Lock()
				dp.frame = (dp.frame + 1) % len(dp.frames)
				dp.render()
				dp.mu.Unlock()
			}
		}
	}()
	dp.render()
	return dp
}

func (dp *diffProgress) render() {
	if dp.area == nil {
		return
	}
	var lines []string
	for i, name := range dp.names {
		var line string
		switch dp.states[i] {
		case "queued":
			line = fmt.Sprintf("  %s  %s", ui.DimText(name), ui.DimText("queued"))
		case "scanning":
			spin := pterm.Cyan(dp.frames[dp.frame])
			line = fmt.Sprintf("  %s %s  %s", spin, pterm.Cyan(name), ui.DimText(dp.details[i]))
		case "done":
			line = fmt.Sprintf("  %s %s  %s", pterm.Green("✓"), name, ui.DimText(dp.details[i]))
		case "error":
			line = fmt.Sprintf("  %s %s  %s", pterm.Red("✗"), name, ui.DimText(dp.details[i]))
		}
		lines = append(lines, line)
	}
	// Progress bar at bottom (with blank line separator)
	if dp.totalSkills > 0 {
		lines = append(lines, "", "  "+dp.renderBar())
	}
	dp.area.Update(strings.Join(lines, "\n"))
}

func (dp *diffProgress) renderBar() string {
	return ui.RenderInlineBar(dp.processedSkills, dp.totalSkills)
}

func (dp *diffProgress) startTarget(name string) {
	if dp == nil {
		return
	}
	dp.mu.Lock()
	defer dp.mu.Unlock()
	for i, n := range dp.names {
		if n == name {
			dp.states[i] = "scanning"
			dp.details[i] = "comparing..."
			break
		}
	}
}

func (dp *diffProgress) update(targetName, skillName string) {
	if dp == nil {
		return
	}
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.processedSkills++
	for i, n := range dp.names {
		if n == targetName {
			dp.details[i] = skillName
			break
		}
	}
}

func (dp *diffProgress) add(n int) {
	if dp == nil {
		return
	}
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.processedSkills += n
}

func (dp *diffProgress) doneTarget(name string, r targetDiffResult) {
	if dp == nil {
		return
	}
	dp.mu.Lock()
	defer dp.mu.Unlock()
	for i, n := range dp.names {
		if n != name {
			continue
		}
		if r.errMsg != "" {
			dp.states[i] = "error"
			dp.details[i] = r.errMsg
		} else if r.synced {
			dp.states[i] = "done"
			dp.details[i] = "fully synced"
		} else {
			dp.states[i] = "done"
			dp.details[i] = fmt.Sprintf("%d difference(s)", len(r.items))
		}
		break
	}
}

func (dp *diffProgress) stop() {
	if dp == nil {
		return
	}
	if dp.stopCh != nil {
		close(dp.stopCh)
	}
	if dp.area != nil {
		dp.area.Stop() //nolint:errcheck
	}
}

func cmdDiffGlobal(targetName string, kind resourceKindFilter, opts diffRenderOpts, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Agent-only diff
	if kind == kindAgents {
		return diffGlobalAgents(cfg, targetName, opts, start)
	}

	var spinner *ui.Spinner
	if !opts.jsonOutput {
		spinner = ui.StartSpinner("Discovering skills")
	}
	discovered, discoverErr := sync.DiscoverSourceSkills(cfg.EffectiveSkillsSource(), cfg.SkillsWalk())
	if discoverErr != nil {
		if spinner != nil {
			spinner.Fail("Discovery failed")
		}
		return fmt.Errorf("failed to discover skills: %w", discoverErr)
	}
	if spinner != nil {
		spinner.Stop()
	}

	targets := cfg.Targets
	if targetName != "" {
		if t, exists := cfg.Targets[targetName]; exists {
			targets = map[string]config.TargetConfig{targetName: t}
		} else {
			return fmt.Errorf("target '%s' not found", targetName)
		}
	}

	// Build sorted target list for deterministic progress display
	type targetEntry struct {
		name   string
		target config.TargetConfig
		mode   string
	}
	var entries []targetEntry
	for name, target := range targets {
		// Skills off: nothing is synced, so there is nothing to compare.
		if !target.SkillsConfig().IsEnabled() {
			continue
		}
		mode := target.SkillsConfig().Mode
		if mode == "" {
			mode = cfg.Mode
			if mode == "" {
				mode = "merge"
			}
		}
		entries = append(entries, targetEntry{name, target, mode})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	// Pre-filter skills per target and compute total for progress bar
	type filteredEntry struct {
		targetEntry
		filtered []sync.DiscoveredSkill
	}
	var fentries []filteredEntry
	totalSkills := 0
	hasCopyMode := false
	for _, e := range entries {
		sc := e.target.SkillsConfig()
		filtered, err := sync.SelectTargetSkills(discovered, e.name, sc)
		if err != nil {
			return fmt.Errorf("target %s has invalid include/exclude config: %w", e.name, err)
		}
		fentries = append(fentries, filteredEntry{e, filtered})
		totalSkills += len(filtered)
		if e.mode == "copy" {
			hasCopyMode = true
		}
	}

	names := make([]string, len(fentries))
	for i, fe := range fentries {
		names[i] = fe.name
	}

	var progress *diffProgress
	if !opts.jsonOutput {
		progress = newDiffProgress(names, totalSkills, hasCopyMode)
	}

	results := make([]targetDiffResult, len(fentries))
	sem := make(chan struct{}, 8)
	var wg gosync.WaitGroup
	for i, fe := range fentries {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, fe filteredEntry) {
			defer wg.Done()
			defer func() { <-sem }()
			progress.startTarget(fe.name)
			r := collectTargetDiff(fe.name, fe.target, cfg.EffectiveSkillsSource(), fe.mode, fe.filtered, sync.EffectiveFileIgnorePatterns(cfg.Ignore), progress)
			progress.doneTarget(fe.name, r)
			results[idx] = r
		}(i, fe)
	}
	wg.Wait()

	if progress != nil {
		progress.stop()
	}

	// Extras diff (always included when extras are configured)
	var extrasResults []extraDiffResult
	if len(cfg.Extras) > 0 {
		extrasResults = collectExtrasDiff(cfg.Extras, func(extra config.ExtraConfig) string {
			return config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())
		})
	}

	// Merge agent diffs into skill results so they appear together
	results = mergeAgentDiffsGlobal(cfg, results, targetName)

	if opts.jsonOutput {
		return diffOutputJSONWithExtras(results, extrasResults, start)
	}
	if shouldLaunchTUI(opts.noTUI, cfg) && len(results) > 0 {
		return runDiffTUI(results, extrasResults)
	}
	opts.noTargets = len(cfg.Targets) == 0
	renderGroupedDiffs(results, extrasResults, opts)
	return nil
}

func diffItemToJSON(item copyDiffEntry) diffJSONItem {
	k := item.kind
	if k == "" {
		k = "skill"
	}
	action := item.action
	if item.keptAt != "" {
		action = "kept" // as the Web API reports it
	}
	return diffJSONItem{
		Action: action,
		Name:   item.name,
		Kind:   k,
		Reason: item.reason,
		IsSync: item.isSync,
	}
}

func diffOutputJSON(results []targetDiffResult, start time.Time) error {
	output := diffJSONOutput{
		Duration: formatDuration(start),
	}
	for _, r := range results {
		jt := diffJSONTarget{
			Name:    r.name,
			Mode:    r.mode,
			Synced:  r.inSync(),
			Error:   r.errMsg,
			Include: r.include,
			Exclude: r.exclude,
		}
		for _, item := range r.items {
			jt.Items = append(jt.Items, diffItemToJSON(item))
		}
		output.Targets = append(output.Targets, jt)
	}
	return writeJSON(&output)
}

func diffOutputJSONWithExtras(results []targetDiffResult, extrasResults []extraDiffResult, start time.Time) error {
	type outputWithExtras struct {
		Targets  []diffJSONTarget     `json:"targets"`
		Extras   []extraDiffJSONEntry `json:"extras,omitempty"`
		Duration string               `json:"duration"`
	}
	o := outputWithExtras{
		Duration: formatDuration(start),
		Extras:   extrasDiffToJSON(extrasResults),
	}
	for _, r := range results {
		jt := diffJSONTarget{
			Name:    r.name,
			Mode:    r.mode,
			Synced:  r.inSync(),
			Error:   r.errMsg,
			Include: r.include,
			Exclude: r.exclude,
		}
		for _, item := range r.items {
			jt.Items = append(jt.Items, diffItemToJSON(item))
		}
		o.Targets = append(o.Targets, jt)
	}
	return writeJSON(&o)
}

func collectTargetDiff(name string, target config.TargetConfig, source, mode string, filtered []sync.DiscoveredSkill, ignorePatterns []string, dp *diffProgress) targetDiffResult {
	sc := target.SkillsConfig()
	r := targetDiffResult{
		name:    name,
		mode:    mode,
		include: sc.Include,
		exclude: sc.Exclude,
	}

	// Check if target is accessible
	_, err := os.Lstat(sc.Path)
	if err != nil {
		r.errMsg = fmt.Sprintf("Cannot access target: %v", err)
		dp.add(len(filtered))
		return r
	}

	resolution, err := sync.ResolveTargetSkillsForTarget(name, config.ResourceTargetConfig{
		Path:         sc.Path,
		TargetNaming: sc.TargetNaming,
	}, filtered)
	if err != nil {
		r.errMsg = err.Error()
		dp.add(len(filtered))
		return r
	}

	sourceSkills := resolution.ValidTargetNames()
	sourceMap := make(map[string]string, len(resolution.Skills))
	for _, resolved := range resolution.Skills {
		sourceMap[resolved.TargetName] = resolved.Skill.SourcePath
	}

	if utils.IsSymlinkOrJunction(sc.Path) {
		r.mode = "symlink"
		collectSymlinkDiff(&r, sc.Path, source)
		dp.add(len(filtered))
		return r
	}

	var manifest *sync.Manifest // only copy-mode legacy lookups read it
	if mode == "copy" {
		manifest, _ = sync.ReadManifest(sc.Path)
	}
	legacyNames := resolution.LegacyNames(mode, sc.Path, manifest)
	if mode == "copy" {
		collectCopyDiff(&r, name, sc.Path, resolution.Skills, resolution.Naming, sourceSkills, legacyNames, manifest, ignorePatterns, dp)
	} else {
		// Merge mode (instant)
		collectMergeDiff(&r, sc.Path, sourceSkills, sourceMap, legacyNames)
		dp.add(len(filtered))
	}

	// Compute mtime only for copy mode (merge uses symlinks, mtime is misleading)
	if mode == "copy" {
		r.dstMtime = latestMtime(sc.Path)
		var srcLatest time.Time
		for _, resolved := range resolution.Skills {
			mt := latestMtime(resolved.Skill.SourcePath)
			if mt.After(srcLatest) {
				srcLatest = mt
			}
		}
		r.srcMtime = srcLatest
	}

	return r
}

func collectSymlinkDiff(r *targetDiffResult, targetPath, source string) {
	absLink, err := utils.ResolveLinkTarget(targetPath)
	if err != nil {
		r.errMsg = fmt.Sprintf("Unable to resolve symlink target: %v", err)
		return
	}
	absSource, _ := filepath.Abs(source)
	if utils.PathsEqual(absLink, absSource) {
		r.synced = true
	} else {
		r.errMsg = fmt.Sprintf("Symlink points to different location: %s", absLink)
	}
}

func collectCopyDiff(r *targetDiffResult, targetName, targetPath string, filtered []sync.ResolvedTargetSkill, naming string, sourceSkills map[string]bool, legacyNames map[string]sync.ResolvedTargetSkill, manifest *sync.Manifest, ignorePatterns []string, dp *diffProgress) {
	renamedFrom := sync.RenamedFrom(legacyNames)
	for _, resolved := range filtered {
		skill := resolved.Skill
		dp.update(targetName, resolved.TargetName)
		oldChecksum, isManaged := manifest.Managed[resolved.TargetName]
		targetSkillPath := filepath.Join(targetPath, resolved.TargetName)
		srcDir := skill.SourcePath
		dstDir := targetSkillPath
		if !isManaged {
			if info, err := os.Stat(targetSkillPath); err == nil {
				if old, ok := renamedFrom[resolved.TargetName]; ok {
					r.items = append(r.items, copyDiffEntry{action: "remove", name: resolved.TargetName, reason: sync.KeptLegacyReason(old), keptAt: old})
				} else if info.IsDir() {
					r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: "local copy (sync --force to replace)", isSync: true, srcDir: srcDir, dstDir: dstDir})
				} else {
					r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: "target entry is not a directory", isSync: true, srcDir: srcDir, dstDir: dstDir})
				}
			} else if old, ok := renamedFrom[resolved.TargetName]; ok && os.IsNotExist(err) {
				r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: sync.RenameReason(old), isSync: true, srcDir: srcDir, dstDir: filepath.Join(targetPath, old)})
			} else if os.IsNotExist(err) {
				r.items = append(r.items, copyDiffEntry{action: "add", name: resolved.TargetName, reason: "source only", isSync: true, srcDir: srcDir, dstDir: dstDir})
			} else {
				r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: "cannot access target entry", isSync: true, srcDir: srcDir, dstDir: dstDir})
			}
			continue
		}
		targetInfo, err := os.Stat(targetSkillPath)
		if os.IsNotExist(err) {
			r.items = append(r.items, copyDiffEntry{action: "add", name: resolved.TargetName, reason: "deleted from target", isSync: true, srcDir: srcDir, dstDir: dstDir})
			continue
		}
		if err != nil {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: "cannot access target entry", isSync: true, srcDir: srcDir, dstDir: dstDir})
			continue
		}
		if !targetInfo.IsDir() {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: "target entry is not a directory", isSync: true, srcDir: srcDir, dstDir: dstDir})
			continue
		}
		// A copy made under another naming carries another name:, so sync re-copies it.
		if recorded := manifest.Naming[resolved.TargetName]; recorded != "" && recorded != naming {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: sync.NamingChangedReason, isSync: true, srcDir: srcDir, dstDir: dstDir})
			continue
		}
		// mtime fast-path
		oldMtime := manifest.Mtimes[resolved.TargetName]
		currentMtime, mtimeErr := sync.DirMaxMtimeWithIgnore(skill.SourcePath, ignorePatterns)
		if mtimeErr == nil && oldMtime > 0 && currentMtime == oldMtime {
			continue
		}
		srcChecksum, err := sync.DirChecksumWithIgnore(skill.SourcePath, ignorePatterns)
		if err != nil {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: "cannot compute checksum", isSync: true, srcDir: srcDir, dstDir: dstDir})
			continue
		}
		if srcChecksum != oldChecksum {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: resolved.TargetName, reason: "content changed", isSync: true, srcDir: srcDir, dstDir: dstDir})
		}
	}

	// Orphan managed copies
	for name := range manifest.Managed {
		if _, keepLegacy := legacyNames[name]; keepLegacy {
			continue
		}
		if !sourceSkills[name] {
			r.items = append(r.items, copyDiffEntry{action: "remove", name: name, reason: "orphan (will be pruned)", isSync: true, dstDir: filepath.Join(targetPath, name)})
		}
	}

	// Local directories
	entries, _ := os.ReadDir(targetPath)
	for _, e := range entries {
		if utils.IsHidden(e.Name()) || !e.IsDir() {
			continue
		}
		if sourceSkills[e.Name()] {
			continue
		}
		if _, keepLegacy := legacyNames[e.Name()]; keepLegacy {
			continue
		}
		if _, isManaged := manifest.Managed[e.Name()]; isManaged {
			continue
		}
		r.items = append(r.items, copyDiffEntry{action: "remove", name: e.Name(), reason: "local only", isSync: false, dstDir: filepath.Join(targetPath, e.Name())})
	}

	// Compute counts. A kept legacy entry is shown but counts as neither:
	// sync changes nothing until the user moves the folder.
	for _, item := range r.items {
		if item.isSync {
			r.syncCount++
		} else if item.keptAt == "" {
			r.localCount++
		}
	}
	r.synced = len(r.items) == 0
}

func collectMergeDiff(r *targetDiffResult, targetPath string, sourceSkills map[string]bool, sourceMap map[string]string, legacyNames map[string]sync.ResolvedTargetSkill) {
	targetSkills := make(map[string]bool)
	targetSymlinks := make(map[string]bool)
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		r.errMsg = fmt.Sprintf("Cannot read target: %v", err)
		return
	}

	manifest, _ := sync.ReadManifest(targetPath)
	for _, e := range entries {
		if manifest.SkipsHidden(e.Name()) {
			continue
		}
		skillPath := filepath.Join(targetPath, e.Name())
		if utils.IsSymlinkOrJunction(skillPath) {
			targetSymlinks[e.Name()] = true
		}
		targetSkills[e.Name()] = true
	}

	// Skills only in source (not synced)
	renamedFrom := sync.RenamedFrom(legacyNames)
	for skill := range sourceSkills {
		srcDir := sourceMap[skill]
		dstDir := filepath.Join(targetPath, skill)
		if old, ok := renamedFrom[skill]; ok && !targetSkills[skill] {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: skill, reason: sync.RenameReason(old), isSync: true, srcDir: srcDir, dstDir: filepath.Join(targetPath, old)})
			r.syncCount++
		} else if !targetSkills[skill] {
			r.items = append(r.items, copyDiffEntry{action: "add", name: skill, reason: "source only", isSync: true, srcDir: srcDir, dstDir: dstDir})
			r.syncCount++
		} else if old, ok := renamedFrom[skill]; ok && !targetSymlinks[skill] {
			r.items = append(r.items, copyDiffEntry{action: "remove", name: skill, reason: sync.KeptLegacyReason(old), keptAt: old})
		} else if !targetSymlinks[skill] {
			r.items = append(r.items, copyDiffEntry{action: "modify", name: skill, reason: "local copy (sync --force to replace)", isSync: true, srcDir: srcDir, dstDir: dstDir})
			r.syncCount++
		}
	}

	// Skills only in target (local only)
	for skill := range targetSkills {
		if _, keepLegacy := legacyNames[skill]; keepLegacy {
			continue
		}
		if !sourceSkills[skill] && !targetSymlinks[skill] {
			r.items = append(r.items, copyDiffEntry{action: "remove", name: skill, reason: "local only", isSync: false, dstDir: filepath.Join(targetPath, skill)})
			r.localCount++
		}
	}

	r.synced = len(r.items) == 0
}

// diffFingerprint generates a grouping key from diff items.
// Results with the same fingerprint are displayed together.
func diffFingerprint(items []copyDiffEntry) string {
	sorted := make([]copyDiffEntry, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].name != sorted[j].name {
			return sorted[i].name < sorted[j].name
		}
		return sorted[i].action < sorted[j].action
	})
	var b strings.Builder
	for _, item := range sorted {
		fmt.Fprintf(&b, "%s|%s|%s\n", item.action, item.name, item.reason)
	}
	return b.String()
}

// actionCategory groups diff items by the user action needed.
type actionCategory struct {
	kind   string // "new", "modified", "restore", "override", "kept", "orphan", "local", "warn"
	label  string // e.g. "New", "Modified", "Local Override"
	names  []string
	expand bool // true = list skill names
}

// categorizeItems maps raw diff items to action-oriented categories.
func categorizeItems(items []copyDiffEntry) []actionCategory {
	type bucket struct {
		kind  string
		label string
		names []string
	}
	buckets := map[string]*bucket{}
	var order []string

	add := func(key, kind, label, name string) {
		if b, ok := buckets[key]; ok {
			b.names = append(b.names, name)
		} else {
			buckets[key] = &bucket{kind: kind, label: label, names: []string{name}}
			order = append(order, key)
		}
	}

	for _, item := range items {
		switch {
		case item.reason == "source only" || item.reason == "not in target":
			add("new", "new", "New", item.name)
		case item.reason == "deleted from target":
			add("restore", "new", "Restore", item.name)
		case item.reason == "content changed" || item.reason == sync.NamingChangedReason:
			add("modified", "modified", "Modified", item.name)
		case strings.HasPrefix(item.reason, "renamed from "):
			add("renamed", "modified", "Renamed", item.name)
		case strings.Contains(item.reason, "local copy"):
			add("override", "override", "Local Override", item.name)
		case strings.Contains(item.reason, "orphan"):
			add("orphan", "orphan", "Orphan", item.name)
		case item.keptAt != "":
			add("kept", "kept", "Local only, skill kept under old name", item.name)
		case item.reason == "local only" || item.reason == "not in source" || item.reason == "local file":
			add("local", "local", "Local Only", item.name)
		default:
			add("warn", "warn", item.reason, item.name)
		}
	}

	var cats []actionCategory
	for _, key := range order {
		b := buckets[key]
		expand := len(b.names) <= 15
		cats = append(cats, actionCategory{
			kind:   b.kind,
			label:  b.label,
			names:  b.names,
			expand: expand,
		})
	}
	return cats
}

// renderGroupedDiffs prints each group of targets with identical
// differences, the targets that are in sync or unreadable, the extras, and
// a closing line with what to run next. Targets with errors are always
// shown individually.
func renderGroupedDiffs(results []targetDiffResult, extras []extraDiffResult, opts diffRenderOpts) {
	if len(results) == 0 && len(extras) == 0 {
		if opts.noTargets {
			ui.Done(ui.MarkNone, "No targets configured", 0)
			ui.Next("skillshare target add <name> <path>", "add one")
		} else {
			ui.Done(ui.MarkNone, "Nothing to compare: no target takes skills", 0)
		}
		return
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})

	var errorResults []targetDiffResult
	var syncedNames []string
	localOnly := 0 // in sync, but listed for the folders only the target holds
	type diffGroup struct {
		names  []string
		result targetDiffResult
	}
	groups := make(map[string]*diffGroup)
	var groupOrder []string

	for _, r := range results {
		if r.errMsg != "" {
			errorResults = append(errorResults, r)
			continue
		}
		if r.synced {
			syncedNames = append(syncedNames, r.name)
			continue
		}
		fp := diffFingerprint(r.items)
		if g, exists := groups[fp]; exists {
			g.names = append(g.names, r.name)
		} else {
			groups[fp] = &diffGroup{names: []string{r.name}, result: r}
			groupOrder = append(groupOrder, fp)
		}
	}

	// One label width for every group, so values line up across sections.
	var groupLabels []string
	for _, fp := range groupOrder {
		for _, cat := range categorizeItems(groups[fp].result.items) {
			groupLabels = append(groupLabels, sentenceCase(cat.label))
		}
	}
	groupWidth := ui.RowWidth(groupLabels...)

	out := &diffOutput{}
	var next diffNext
	needCount, keptCount := 0, 0
	for _, fp := range groupOrder {
		g := groups[fp]
		sort.Strings(g.names)
		// Kept rows need the user to move a folder first, so they are neither
		// in sync nor something sync can apply.
		switch {
		case g.result.syncCount == 0 && g.result.keptCount() > 0:
			keptCount += len(g.names)
		case g.result.inSync():
			localOnly += len(g.names)
		default:
			needCount += len(g.names)
		}
		out.section(strings.Join(g.names, ", "))
		renderDiffGroup(g.result, opts, &next, groupWidth)
	}

	// Unreadable targets, then the ones in sync on a single row
	syncedLabel := strings.Join(syncedNames, ", ")
	var statusNames []string
	for _, r := range errorResults {
		statusNames = append(statusNames, r.name)
	}
	if syncedLabel != "" {
		statusNames = append(statusNames, syncedLabel)
	}
	if len(statusNames) > 0 {
		out.gap()
		width := ui.RowWidth(statusNames...)
		for _, r := range errorResults {
			ui.Row(ui.MarkFail, r.name, r.errMsg, width)
		}
		if syncedLabel != "" {
			ui.Row(ui.MarkOK, syncedLabel, "in sync", width)
		}
	}

	extrasNeed := 0
	if len(extras) > 0 {
		out.section("Extras")
		extrasNeed = renderExtrasDiffPlain(extras)
		if extrasNeed > 0 {
			next.extras = true
		}
	}

	if out.printed {
		fmt.Println()
	}
	total := len(results)
	if needCount == 0 && keptCount == 0 && len(errorResults) == 0 {
		text := "No differences"
		switch {
		case total == 1:
			text = results[0].name + " is in sync"
		case total > 1:
			text = fmt.Sprintf("All %d targets in sync", total)
		}
		if extrasNeed == 0 {
			ui.Done(ui.MarkOK, text, 0)
			ui.Next(next.pairs()...)
			return
		}
		if total == 0 {
			text = ""
		} else {
			text += " · "
		}
		ui.Done(ui.MarkWarn, text+plural(extrasNeed, "extra")+" to sync", 0)
		ui.Next(next.pairs()...)
		return
	}
	var parts []string
	if needCount > 0 {
		parts = append(parts, fmt.Sprintf("%d to sync", needCount))
	}
	if keptCount > 0 {
		parts = append(parts, fmt.Sprintf("%d kept under old name", keptCount))
	}
	if len(errorResults) > 0 {
		parts = append(parts, fmt.Sprintf("%d unreadable", len(errorResults)))
	}
	if n := len(syncedNames) + localOnly; n > 0 {
		parts = append(parts, fmt.Sprintf("%d in sync", n))
	}
	text := plural(total, "target")
	if len(parts) > 0 {
		text += ": " + strings.Join(parts, ", ")
	}
	if extrasNeed > 0 {
		text += " · " + plural(extrasNeed, "extra") + " to sync"
	}
	mark := ui.MarkWarn
	if len(errorResults) > 0 {
		mark = ui.MarkFail
	}
	ui.Done(mark, text, 0)
	ui.Next(next.pairs()...)
}

// diffOutput tracks whether anything was printed, so the first section
// starts at the top and later ones are set off by a blank line.
type diffOutput struct{ printed bool }

func (o *diffOutput) gap() {
	if o.printed {
		fmt.Println()
	}
	o.printed = true
}

func (o *diffOutput) section(name string) {
	o.gap()
	fmt.Println(ui.Bold + name + ui.Reset)
}

// keptSyncHint says when sync moves a skill kept under its old name.
const keptSyncHint = "after renaming or removing the local folders kept under the old name"

// diffNext collects which commands would resolve the differences shown.
type diffNext struct {
	skills, agents, extras bool // something to sync
	force                  bool // local copies sync would only replace with --force
	kept                   bool // local folders holding a skill's new name
	collectSkills          bool // local-only skills
	collectAgents          bool // local-only agents
}

func (n diffNext) pairs() []string {
	var pairs []string
	switch {
	case n.skills && n.agents:
		pairs = append(pairs, "skillshare sync --all", "apply the changes")
	case n.skills:
		pairs = append(pairs, "skillshare sync", "apply the changes")
	case n.agents:
		pairs = append(pairs, "skillshare sync agents", "apply the changes")
	}
	if n.extras && !(n.skills && n.agents) {
		pairs = append(pairs, "skillshare sync extras", "update the extras")
	}
	if n.force {
		pairs = append(pairs, "skillshare sync --force", "also replace local copies")
	}
	if n.kept {
		pairs = append(pairs, "skillshare sync", keptSyncHint)
	}
	if n.collectSkills {
		pairs = append(pairs, "skillshare collect", "copy local-only skills into source")
	}
	if n.collectAgents {
		pairs = append(pairs, "skillshare collect agents", "copy local-only agents into source")
	}
	if len(pairs) > 6 {
		pairs = pairs[:6]
	}
	return pairs
}

// renderDiffGroup prints one row per kind of change, such as "New  a, b".
// With --stat or --patch, and for modified items, each item gets its own
// row followed by its file changes.
func renderDiffGroup(r targetDiffResult, opts diffRenderOpts, next *diffNext, width int) {
	items := make([]copyDiffEntry, len(r.items))
	copy(items, r.items)
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})

	cats := categorizeItems(items)
	labels := make([]string, len(cats))
	for i, cat := range cats {
		labels[i] = sentenceCase(cat.label)
	}
	fileIndent := strings.Repeat(" ", width+4)

	for i, cat := range cats {
		for _, name := range cat.names {
			agent := false
			if item := findDiffItem(items, name); item != nil && item.kind == "agent" {
				agent = true
			}
			switch {
			case cat.kind == "override":
				next.force = true
			case cat.kind == "kept":
				next.kept = true
			case cat.kind == "local" && agent:
				next.collectAgents = true
			case cat.kind == "local":
				next.collectSkills = true
			case agent:
				next.agents = true
			default:
				next.skills = true
			}
		}

		if !cat.expand {
			ui.Row(ui.MarkNone, labels[i], plural(len(cat.names), "item"), width)
			continue
		}
		shown := cat.names
		if cat.kind == "kept" {
			shown = make([]string, len(cat.names))
			for j, name := range cat.names {
				shown[j] = findDiffItem(items, name).label()
			}
		}
		if !opts.showStat && !opts.showPatch && cat.kind != "modified" {
			ui.Row(ui.MarkNone, labels[i], strings.Join(shown, ", "), width)
			continue
		}
		for j, name := range cat.names {
			label := ""
			if j == 0 {
				label = labels[i]
			}
			ui.Row(ui.MarkNone, label, shown[j], width)
			item := findDiffItem(items, name)
			if item == nil {
				continue
			}
			item.ensureFiles()
			for _, line := range strings.Split(strings.TrimRight(renderFileStat(item.files), "\n"), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					fmt.Println(fileIndent + ui.DimText(line))
				}
			}
			if !opts.showPatch {
				continue
			}
			for _, f := range item.files {
				if f.Action != "modify" || item.srcDir == "" {
					continue
				}
				diffText := generateUnifiedDiff(filepath.Join(item.srcDir, f.RelPath), filepath.Join(item.dstDir, f.RelPath))
				if diffText == "" {
					continue
				}
				fmt.Println(fileIndent + "--- " + f.RelPath)
				for _, line := range strings.Split(strings.TrimRight(colorizePlainDiff(diffText), "\n"), "\n") {
					fmt.Println(fileIndent + line)
				}
			}
		}
	}

	var times []string
	if !r.srcMtime.IsZero() {
		times = append(times, "source changed "+r.srcMtime.Format("2006-01-02 15:04"))
	}
	if !r.dstMtime.IsZero() {
		times = append(times, "target changed "+r.dstMtime.Format("2006-01-02 15:04"))
	}
	if len(times) > 0 {
		ui.Note(strings.Join(times, " · "))
	}
}

// sentenceCase turns "Local Only" into "Local only".
func sentenceCase(s string) string {
	if s == "" {
		return s
	}
	return s[:1] + strings.ToLower(s[1:])
}

func findDiffItem(items []copyDiffEntry, name string) *copyDiffEntry {
	for i := range items {
		if items[i].name == name {
			return &items[i]
		}
	}
	return nil
}

func colorizePlainDiff(diff string) string {
	if !ui.IsTTY() {
		return diff
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "+ "):
			b.WriteString(ui.Green + line + ui.Reset + "\n")
		case strings.HasPrefix(line, "- "):
			b.WriteString(ui.Red + line + ui.Reset + "\n")
		default:
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func printDiffHelp() {
	printHelp("skillshare diff [agents|all] [target] [options]", "Show differences between source skills and target directories.\nPreviews what 'sync' would change without modifying anything.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"target", "Target name to diff (optional; diffs all if omitted)"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"-p, --project", "Diff project-level skills (.skillshare/)"},
			{"-g, --global", "Diff global skills (~/.config/skillshare)"},
			{"--stat", "Show file-level changes (implies --no-tui)"},
			{"--patch", "Show full unified diff (implies --no-tui)"},
			{"--json", "Output results as JSON"},
			{"--no-tui", "Plain text output (skip interactive TUI)"},
		}},
		helpExamples(
			helpRow{"skillshare diff", "Diff all targets"},
			helpRow{"skillshare diff claude", "Diff a single target"},
			helpRow{"skillshare diff -p", "Diff project-mode targets"},
			helpRow{"skillshare diff --stat", "Show file-level stat"},
			helpRow{"skillshare diff --patch", "Show full text diff"},
			helpRow{"skillshare diff agents", "Diff agents targets only"},
		),
	)
}
